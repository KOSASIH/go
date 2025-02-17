package main

import (
	"fmt"
	"os"

	galexie "github.com/stellar/go/services/galexie/internal"
)

func main() {
	err := galexie.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
