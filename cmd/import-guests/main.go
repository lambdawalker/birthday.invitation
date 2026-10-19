package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/lambdawalker/birthday.invitation/internal/app"
	"log"
	"os"
)

func main() {
	file := flag.String("file", "", "CSV with name,phone,email and optional id")
	apply := flag.Bool("apply", false, "Create missing guests in DynamoDB")
	flag.Parse()
	if *file == "" {
		log.Fatal("Use -file guests.csv [-apply]")
	}
	f, e := os.Open(*file)
	if e != nil {
		log.Fatal(e)
	}
	defer f.Close()
	fs, e := app.ReadGuests(f, os.Getenv("IMPORT_NAMESPACE"))
	if e != nil {
		log.Fatal(e)
	}
	fmt.Printf("Validated %d families. Emails are optional.\n", len(fs))
	if !*apply {
		fmt.Println("Preview only. Set TABLE_NAME, AWS_PROFILE and IMPORT_NAMESPACE, then add -apply.")
		return
	}
	table := os.Getenv("TABLE_NAME")
	if table == "" {
		log.Fatal("Set TABLE_NAME")
	}
	cfg, e := config.LoadDefaultConfig(context.Background())
	if e != nil {
		log.Fatal(e)
	}
	st := &app.DynamoStore{Client: dynamodb.NewFromConfig(cfg), Table: table}
	created, skipped := 0, 0
	for _, f := range fs {
		e = st.Create(context.Background(), f)
		if errors.Is(e, app.ErrConflict) {
			skipped++
			continue
		}
		if e != nil {
			log.Fatal(e)
		}
		created++
	}
	fmt.Printf("Created %d; preserved %d existing families.\n", created, skipped)
}
