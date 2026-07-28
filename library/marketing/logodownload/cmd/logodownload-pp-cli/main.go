package main

import (
	"github.com/mvanhorn/printing-press-library/library/marketing/logodownload/internal/cli"
	"os"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
