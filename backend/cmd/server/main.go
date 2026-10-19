package main

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/lambdawalker/birthday.invitation/backend/internal/app"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	key := os.Getenv("ADMIN_KEY")
	if len(key) < 32 {
		log.Fatal("Set ADMIN_KEY to at least 32 characters")
	}
	origin := os.Getenv("SITE_URL")
	if origin == "" {
		origin = "http://localhost:8080"
	}
	var st app.Store = app.NewMemoryStore()
	if table := os.Getenv("TABLE_NAME"); table != "" {
		cfg, e := config.LoadDefaultConfig(context.Background())
		if e != nil {
			log.Fatal(e)
		}
		st = &app.DynamoStore{Client: dynamodb.NewFromConfig(cfg), Table: table}
	} else {
		_ = st.Create(context.Background(), app.Family{ID: "0123456789abcdef0123456789abcdef", Name: "Familia de ejemplo", Phone: "+526560000000", Revision: 1, RSVP: "pending", Status: "pending"})
		log.Print("Demo memory database: changes disappear at restart")
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", app.NewServer(st, key, origin))
	web := os.Getenv("WEB_DIR")
	if web == "" {
		web = "../web"
	}
	mux.Handle("/", http.FileServer(http.Dir(web)))
	srv := http.Server{Addr: "127.0.0.1:8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Print("Open ", origin, "/?i=0123456789abcdef0123456789abcdef")
	log.Fatal(srv.ListenAndServe())
}
