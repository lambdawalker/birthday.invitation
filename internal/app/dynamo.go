package app

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"strconv"
)

type DynamoStore struct {
	Client *dynamodb.Client
	Table  string
}

func (d *DynamoStore) get(ctx context.Context, key string, out any) error {
	r, e := d.Client.GetItem(ctx, &dynamodb.GetItemInput{TableName: aws.String(d.Table), Key: map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: key}}, ConsistentRead: aws.Bool(true)})
	if e != nil {
		return e
	}
	if len(r.Item) == 0 {
		return ErrNotFound
	}
	v, ok := r.Item["data"].(*types.AttributeValueMemberS)
	if !ok {
		return errors.New("invalid record")
	}
	return json.Unmarshal([]byte(v.Value), out)
}
func (d *DynamoStore) put(ctx context.Context, key string, v any, rev int, condition string, expected int) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	input := &dynamodb.PutItemInput{TableName: aws.String(d.Table), Item: map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: key}, "data": &types.AttributeValueMemberS{Value: string(b)}, "revision": &types.AttributeValueMemberN{Value: strconv.Itoa(rev)}}}
	if condition != "" {
		input.ConditionExpression = aws.String(condition)
	}
	if expected >= 0 {
		input.ExpressionAttributeValues = map[string]types.AttributeValue{":rev": &types.AttributeValueMemberN{Value: strconv.Itoa(expected)}}
	}
	_, e = d.Client.PutItem(ctx, input)
	var conflict *types.ConditionalCheckFailedException
	if errors.As(e, &conflict) {
		return ErrConflict
	}
	return e
}
func (d *DynamoStore) scan(ctx context.Context, prefix string) ([]string, error) {
	out := []string{}
	var start map[string]types.AttributeValue
	for {
		r, e := d.Client.Scan(ctx, &dynamodb.ScanInput{TableName: aws.String(d.Table), ExclusiveStartKey: start, ConsistentRead: aws.Bool(true), FilterExpression: aws.String("begins_with(pk, :prefix)"), ExpressionAttributeValues: map[string]types.AttributeValue{":prefix": &types.AttributeValueMemberS{Value: prefix}}})
		if e != nil {
			return nil, e
		}
		for _, item := range r.Items {
			if v, ok := item["data"].(*types.AttributeValueMemberS); ok {
				out = append(out, v.Value)
			}
		}
		start = r.LastEvaluatedKey
		if len(start) == 0 {
			break
		}
	}
	return out, nil
}
func (d *DynamoStore) Get(c context.Context, id string) (f Family, e error) {
	e = d.get(c, "F#"+id, &f)
	return
}
func (d *DynamoStore) List(c context.Context) ([]Family, error) {
	rows, e := d.scan(c, "F#")
	if e != nil {
		return nil, e
	}
	fs := []Family{}
	for _, row := range rows {
		var f Family
		if e = json.Unmarshal([]byte(row), &f); e != nil {
			return nil, e
		}
		fs = append(fs, f)
	}
	return fs, nil
}
func (d *DynamoStore) Create(c context.Context, f Family) error {
	phone, err := NormalizePhone(f.Phone)
	if err != nil {
		return err
	}
	f.Phone = phone
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}
	// Claim the phone and create its family atomically. A failed request cannot
	// leave an orphan reservation or allow two app users to add the same number.
	_, err = d.Client.TransactWriteItems(c, &dynamodb.TransactWriteItemsInput{
		ClientRequestToken: aws.String(RandomID()),
		TransactItems: []types.TransactWriteItem{
			{Put: &types.Put{TableName: aws.String(d.Table), ConditionExpression: aws.String("attribute_not_exists(pk)"),
				Item: map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: "F#" + f.ID}, "data": &types.AttributeValueMemberS{Value: string(data)}, "revision": &types.AttributeValueMemberN{Value: strconv.Itoa(f.Revision)}}}},
			{Put: &types.Put{TableName: aws.String(d.Table), ConditionExpression: aws.String("attribute_not_exists(pk)"),
				Item: map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: "P#" + HashToken(phone)}, "familyId": &types.AttributeValueMemberS{Value: f.ID}}}},
		},
	})
	var cancelled *types.TransactionCanceledException
	if errors.As(err, &cancelled) {
		for _, reason := range cancelled.CancellationReasons {
			if aws.ToString(reason.Code) == "ConditionalCheckFailed" {
				return ErrConflict
			}
		}
	}
	return err
}
func (d *DynamoStore) Save(c context.Context, f Family, rev int) error {
	return d.put(c, "F#"+f.ID, f, f.Revision, "revision = :rev", rev)
}
func (d *DynamoStore) PutDevice(c context.Context, v Device) error {
	return d.put(c, "D#"+v.ID, v, 1, "", -1)
}
func (d *DynamoStore) GetDevice(c context.Context, id string) (v Device, e error) {
	e = d.get(c, "D#"+id, &v)
	return
}
func (d *DynamoStore) Devices(c context.Context) ([]Device, error) {
	rows, e := d.scan(c, "D#")
	if e != nil {
		return nil, e
	}
	ds := []Device{}
	for _, row := range rows {
		var v Device
		if e = json.Unmarshal([]byte(row), &v); e != nil {
			return nil, e
		}
		ds = append(ds, v)
	}
	return ds, nil
}

func (d *DynamoStore) GetUser(c context.Context, id string) (u User, err error) {
	err = d.get(c, "U#"+id, &u)
	return
}
func (d *DynamoStore) Users(c context.Context) ([]User, error) {
	rows, err := d.scan(c, "U#")
	if err != nil {
		return nil, err
	}
	out := []User{}
	for _, row := range rows {
		var u User
		if err = json.Unmarshal([]byte(row), &u); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, nil
}
func (d *DynamoStore) SaveUser(c context.Context, u User, rev int) error {
	if rev == 0 {
		return d.put(c, "U#"+u.ID, u, u.Revision, "attribute_not_exists(pk)", -1)
	}
	return d.put(c, "U#"+u.ID, u, u.Revision, "revision = :rev", rev)
}

// Delete the family and its phone reservation together. Revision and ownership
// conditions prevent a concurrent update or another family's claim being removed.
func (d *DynamoStore) Delete(c context.Context, f Family) error {
	phone, err := NormalizePhone(f.Phone)
	if err != nil {
		return err
	}
	_, err = d.Client.TransactWriteItems(c, &dynamodb.TransactWriteItemsInput{
		ClientRequestToken: aws.String(RandomID()),
		TransactItems: []types.TransactWriteItem{
			{Delete: &types.Delete{TableName: aws.String(d.Table), Key: familyDeleteKey("F#" + f.ID), ConditionExpression: aws.String("revision = :rev"), ExpressionAttributeValues: map[string]types.AttributeValue{":rev": &types.AttributeValueMemberN{Value: strconv.Itoa(f.Revision)}}}},
			{Delete: &types.Delete{TableName: aws.String(d.Table), Key: familyDeleteKey("P#" + HashToken(phone)), ConditionExpression: aws.String("attribute_not_exists(pk) OR familyId = :id"), ExpressionAttributeValues: map[string]types.AttributeValue{":id": &types.AttributeValueMemberS{Value: f.ID}}}},
		},
	})
	var cancelled *types.TransactionCanceledException
	if errors.As(err, &cancelled) {
		for _, reason := range cancelled.CancellationReasons {
			if aws.ToString(reason.Code) == "ConditionalCheckFailed" {
				return ErrConflict
			}
		}
	}
	return err
}

func familyDeleteKey(pk string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{"pk": &types.AttributeValueMemberS{Value: pk}}
}
