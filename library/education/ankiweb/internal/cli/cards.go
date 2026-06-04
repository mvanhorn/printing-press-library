// Copyright 2026 paul-bockewitz. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"net/http"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/svc"
	"github.com/spf13/cobra"
)

// newCardsCmd is the `cards` parent command grouping card commands
// (currently `cards search`).
func newCardsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cards",
		Short: "Search the cards in your collection (search)",
		Long:  "Commands for the cards in your AnkiWeb collection. See 'cards search'.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCardsSearchCmd(flags))
	return cmd
}

func newCardsSearchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "search [query]",
		Short:       "Search the cards in your collection (requires login)",
		Long:        "Search your own cards via /svc/search/search using AnkiWeb's search syntax (e.g. a word, \"deck:Spanish\", or \"tag:french\"). Returns matching card ids and a text snippet. Requires an AnkiWeb session cookie.",
		Example:     "  ankiweb-pp-cli cards search \"casa\" --json",
		Annotations: map[string]string{"pp:endpoint": "search.search", "pp:method": "POST", "pp:path": "/svc/search/search", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), []svc.Card{}, flags)
			}
			c, cfg, err := flags.newSvcClient()
			if err != nil {
				return err
			}
			if strings.TrimSpace(cfg.AnkiwebCookies) == "" {
				return authErr(errAuth())
			}
			data, status, err := c.PostBytes(cmd.Context(), "/svc/search/search", svc.BuildSearchRequest(query))
			if err != nil {
				if status == http.StatusForbidden || status == http.StatusUnauthorized {
					return authErr(errAuth())
				}
				return classifyAPIError(err, flags)
			}
			cards, err := svc.DecodeSearchResults(data)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), cards, flags)
		},
	}
	return cmd
}
