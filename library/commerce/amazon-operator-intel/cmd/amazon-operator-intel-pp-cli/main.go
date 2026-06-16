package main

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-operator-intel/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
