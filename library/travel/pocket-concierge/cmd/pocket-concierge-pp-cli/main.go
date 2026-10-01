package main

import (
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/travel/pocket-concierge/internal/cli"
	"github.com/mvanhorn/printing-press-library/library/travel/pocket-concierge/internal/pocket"
	"os"
	"strings"
)

func main() {
	if err := cli.Execute(); err != nil {
		if strings.HasPrefix(err.Error(), "unknown command") {
			err = pocket.Fail("usage", err.Error())
		}
		_ = json.NewEncoder(os.Stderr).Encode(map[string]any{"error": pocket.ErrorJSON(err)})
		os.Exit(cli.ExitCode(err))
	}
}
