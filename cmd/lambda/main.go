package main

import (
	"context"
	"encoding/base64"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/lambdawalker/birthday.invitation/internal/app"
	"log"
	"net/http/httptest"
	"os"
	"strings"
)

func main() {
	cfg, e := config.LoadDefaultConfig(context.Background())
	if e != nil {
		log.Fatal(e)
	}
	s := app.NewServer(&app.DynamoStore{Client: dynamodb.NewFromConfig(cfg), Table: os.Getenv("TABLE_NAME")}, os.Getenv("ADMIN_KEY"), os.Getenv("SITE_URL"))
	lambda.Start(func(ctx context.Context, event events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
		body := event.Body
		if event.IsBase64Encoded {
			b, e := base64.StdEncoding.DecodeString(body)
			if e != nil {
				return events.APIGatewayV2HTTPResponse{StatusCode: 400}, nil
			}
			body = string(b)
		}
		url := event.RawPath
		if event.RawQueryString != "" {
			url += "?" + event.RawQueryString
		}
		r := httptest.NewRequest(event.RequestContext.HTTP.Method, url, strings.NewReader(body)).WithContext(ctx)
		for k, v := range event.Headers {
			r.Header.Set(k, v)
		}
		if len(event.Cookies) > 0 {
			r.Header.Set("Cookie", strings.Join(event.Cookies, "; "))
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		out := events.APIGatewayV2HTTPResponse{StatusCode: w.Code, Body: w.Body.String(), Headers: map[string]string{}}
		for k, v := range w.Header() {
			if strings.EqualFold(k, "Set-Cookie") {
				out.Cookies = v
			} else {
				out.Headers[k] = strings.Join(v, ",")
			}
		}
		return out, nil
	})
}
