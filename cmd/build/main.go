package main

import (
	"fmt"
	"github.com/lambdawalker/birthday.invitation/internal/deploytools"
	"os"
)

func main() {
	root, err := deploytools.FindRoot(".")
	if err == nil {
		err = deploytools.Build(root)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Lambda archive ready: build/lambda.zip")
}
