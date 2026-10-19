package app

import (
	"context"
	"encoding/json"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeleteDynamoTransaction(t *testing.T) {
	var body struct {
		TransactItems []struct {
			Delete struct {
				Key                       map[string]map[string]string
				ConditionExpression       string
				ExpressionAttributeValues map[string]map[string]string
			}
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Amz-Target") != "DynamoDB_20120810.TransactWriteItems" {
			t.Error("not transactional")
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/x-amz-json-1.0")
		w.Write([]byte("{}"))
	}))
	defer srv.Close()
	client := dynamodb.New(dynamodb.Options{Region: "us-east-1", BaseEndpoint: aws.String(srv.URL), Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{AccessKeyID: "test", SecretAccessKey: "test"}, nil
	})})
	st := DynamoStore{Client: client, Table: "test"}
	f := Family{ID: RandomID(), Phone: "+526561234567", Revision: 7}
	if err := st.Delete(t.Context(), f); err != nil {
		t.Fatal(err)
	}
	if len(body.TransactItems) != 2 {
		t.Fatal(body)
	}
	a, b := body.TransactItems[0].Delete, body.TransactItems[1].Delete
	if a.Key["pk"]["S"] != "F#"+f.ID || a.ConditionExpression != "revision = :rev" || a.ExpressionAttributeValues[":rev"]["N"] != "7" {
		t.Fatal(a)
	}
	if b.Key["pk"]["S"] != "P#"+HashToken(f.Phone) || b.ConditionExpression != "attribute_not_exists(pk) OR familyId = :id" || b.ExpressionAttributeValues[":id"]["S"] != f.ID {
		t.Fatal(b)
	}
}
