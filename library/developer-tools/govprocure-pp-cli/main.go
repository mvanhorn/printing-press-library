package main

import (
	"os"

	"github.com/silentnw/govprocure-pp-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
