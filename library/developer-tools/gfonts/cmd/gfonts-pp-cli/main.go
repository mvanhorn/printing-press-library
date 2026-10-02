package main

import (
	"os"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/gfonts/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
