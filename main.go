package main

import (
	"log"

	"github.com/dkooll/issuor/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
