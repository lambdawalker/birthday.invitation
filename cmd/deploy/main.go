package main

import (
	"flag"
	"fmt"
	"github.com/lambdawalker/birthday.invitation/internal/deploytools"
	"os"
)

func main() {
	check := flag.Bool("check", false, "Check Go, AWS CLI and Pulumi availability without changing cloud resources")
	reconfigure := flag.Bool("reconfigure", false, "Review saved deployment settings and update private values")
	flag.Parse()
	var err error
	if *check {
		err = deploytools.CheckTools()
		if err == nil {
			fmt.Println("Go, AWS CLI and Pulumi are available.")
		}
	} else {
		var root string
		root, err = deploytools.FindRoot(".")
		if err == nil {
			err = deploytools.DeployWithOptions(root, *reconfigure)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Deployment did not complete:", err)
		os.Exit(1)
	}
}
