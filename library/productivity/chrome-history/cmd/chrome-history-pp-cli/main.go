package main

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/productivity/chrome-history/internal/cli"
)

func main() {
	cmd := cli.NewRootCmd()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(cli.ExitCodeForError(err))
	}
}
