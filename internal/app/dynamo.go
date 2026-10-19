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
	return d.put(c, "F#"+f.ID, f, f.Revision, "attribute_not_exists(pk)", -1)
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
