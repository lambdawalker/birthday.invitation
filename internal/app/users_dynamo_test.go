package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func TestUserDynamoConditionalWrites(t *testing.T) {
	var body struct {
		ConditionExpression       string
		Item                      map[string]map[string]string
		ExpressionAttributeValues map[string]map[string]string
	}
	conflict := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/x-amz-json-1.0")
		if conflict {
			w.WriteHeader(400)
			w.Write([]byte(`{"__type":"com.amazonaws.dynamodb.v20120810#ConditionalCheckFailedException","message":"conflict"}`))
			return
		}
		w.Write([]byte("{}"))
	}))
	defer server.Close()
	client := dynamodb.New(dynamodb.Options{Region: "us-east-1", BaseEndpoint: aws.String(server.URL),
		Credentials: aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
			return aws.Credentials{AccessKeyID: "test", SecretAccessKey: "test"}, nil
		}),
	})
	store := DynamoStore{Client: client, Table: "test"}
	u := User{ID: "user-id", Username: "david", Revision: 1, PINHash: "keyed-hash"}
	if err := store.SaveUser(t.Context(), u, 0); err != nil {
		t.Fatal(err)
	}
	if body.ConditionExpression != "attribute_not_exists(pk)" || body.Item["pk"]["S"] != "U#user-id" {
		t.Fatal(body)
	}
	u.Revision = 2
	if err := store.SaveUser(t.Context(), u, 1); err != nil {
		t.Fatal(err)
	}
	if body.ConditionExpression != "revision = :rev" || body.ExpressionAttributeValues[":rev"]["N"] != "1" || body.Item["revision"]["N"] != "2" {
		t.Fatal(body)
	}
	conflict = true
	if err := store.SaveUser(t.Context(), u, 1); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
}
