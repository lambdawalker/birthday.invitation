package app

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestContactDynamoAtomicPhoneClaim(t *testing.T) {
	var body struct {
		ClientRequestToken string
		TransactItems      []struct {
			Put struct {
				ConditionExpression string
				Item                map[string]map[string]string
			}
		}
	}
	reason := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Amz-Target") != "DynamoDB_20120810.TransactWriteItems" {
			t.Error("not transactional")
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/x-amz-json-1.0")
		if reason != "" {
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]any{"__type": "com.amazonaws.dynamodb.v20120810#TransactionCanceledException", "CancellationReasons": []any{map[string]string{"Code": "None"}, map[string]string{"Code": reason}}})
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
	st := DynamoStore{Client: client, Table: "test"}
	f := Family{ID: RandomID(), Name: "Family", Phone: "+526561234567", Revision: 1}
	if err := st.Create(t.Context(), f); err != nil {
		t.Fatal(err)
	}
	if len(body.TransactItems) != 2 || len(body.ClientRequestToken) != 32 {
		t.Fatal(body)
	}
	if body.TransactItems[0].Put.Item["pk"]["S"] != "F#"+f.ID || body.TransactItems[1].Put.Item["pk"]["S"] != "P#"+HashToken(f.Phone) || body.TransactItems[1].Put.Item["familyId"]["S"] != f.ID {
		t.Fatal(body)
	}
	for _, item := range body.TransactItems {
		if item.Put.ConditionExpression != "attribute_not_exists(pk)" {
			t.Fatal(item)
		}
	}
	reason = "ConditionalCheckFailed"
	if err := st.Create(t.Context(), f); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	reason = "TransactionConflict"
	if err := st.Create(t.Context(), f); err == nil || errors.Is(err, ErrConflict) {
		t.Fatal("retryable contention must not be reported as existing", err)
	}
}
