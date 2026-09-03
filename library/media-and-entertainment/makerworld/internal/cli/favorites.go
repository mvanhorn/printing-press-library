// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored absorbed command: list the authenticated user's liked designs.
// Requires a Bambu Cloud JWT in MAKERWORLD_TOKEN (account-tier feature).

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source live

const makerworldTokenEnv = "MAKERWORLD_TOKEN"

// requireToken returns the Bambu Cloud JWT, or false after printing a hint and
// an empty result. A missing token is a graceful empty state (exit 0), not an
// error: the public read commands need no auth and most users have no token.
func requireToken(cmd *cobra.Command, flags *rootFlags) (string, bool) {
	tok := os.Getenv(makerworldTokenEnv)
	if tok == "" {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"%s is not set; this account command needs a Bambu Cloud token.\n"+
				"export %s=<jwt> (from a logged-in Bambu Handy / Bambu Studio session)\n",
			makerworldTokenEnv, makerworldTokenEnv)
		if flags.asJSON || flags.agent {
			fmt.Fprintln(cmd.OutOrStdout(), "[]")
		}
		return "", false
	}
	return tok, true
}

func newNovelFavoritesCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	var flagOffset int

	cmd := &cobra.Command{
		Use:   "favorites",
		Short: "List the designs you have liked (requires MAKERWORLD_TOKEN)",
		Long: "Lists the designs the authenticated MakerWorld user has liked. Requires a Bambu " +
			"Cloud JWT in the MAKERWORLD_TOKEN environment variable; the public read commands " +
			"(search, designs, discover, tags) need no auth.",
		Example:     strings.Trim("\n  makerworld-pp-cli favorites --limit 20 --agent", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list your liked designs")
				return nil
			}
			if flags.dataSource == "local" {
				return usageErr(fmt.Errorf("favorites reads the live account API and has no local mode"))
			}
			tok, ok := requireToken(cmd, flags)
			if !ok {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GetWithHeaders(ctx, "/design-service/my/design/like", map[string]string{
				"offset": strconv.Itoa(flagOffset),
				"limit":  strconv.Itoa(flagLimit),
			}, map[string]string{"Authorization": "Bearer " + tok})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var env struct {
				Hits json.RawMessage `json:"hits"`
			}
			out := data
			if json.Unmarshal(data, &env) == nil && len(env.Hits) > 0 {
				out = env.Hits
			}
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Page size")
	cmd.Flags().IntVar(&flagOffset, "offset", 0, "Pagination offset")
	return cmd
}
