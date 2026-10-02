// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/yamato/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/yamato/internal/yamato"
	"github.com/spf13/cobra"
	"strings"
)

func luggageCmd(cmd *cobra.Command, short, example, source string, f *rootFlags) *cobra.Command {
	tokens := strings.Fields(example)
	var fixtures []string
	for i := 0; i < len(tokens); i++ {
		if tokens[i] == "--agent" {
			continue
		}
		if i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "--") {
			fixtures = append(fixtures, tokens[i]+"="+tokens[i+1])
			i++
		} else {
			fixtures = append(fixtures, tokens[i])
		}
	}
	cmd.Short = short
	cmd.Example = "  yamato-pp-cli " + cmd.Name() + " " + example
	cmd.Annotations = map[string]string{"mcp:read-only": "true", "pp:data-source": source, "pp:happy-args": strings.Join(fixtures, ";")}
	return cmd
}
func luggageClient(f *rootFlags) *yamato.Client { return yamato.NewClient(f.timeout) }
func luggageError(e error) error {
	var rl *cliutil.RateLimitError
	if errors.As(e, &rl) {
		return rateLimitErr(e)
	}
	return apiErr(e)
}
func luggageOutput(cmd *cobra.Command, f *rootFlags, c *yamato.Client, v any) error {
	if len(c.Failures) > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d optional source checks failed; inspect meta.fetch_failures for unavailable evidence\n", len(c.Failures))
	}
	return f.printJSON(cmd, map[string]any{"meta": c.Meta(), "results": v})
}
func luggageLimit(limit, offset int) error {
	if limit < 1 || limit > 50 || offset < 0 || offset > 10000 {
		return usageErr(fmt.Errorf("--limit must be 1..50 and --offset 0..10000"))
	}
	return nil
}
func noLuggageArgs(args []string) error {
	if len(args) > 0 {
		return usageErr(fmt.Errorf("this command takes flags only; see --help"))
	}
	return nil
}
