// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored. `styles list` surfaces Artistly's style catalog, which the app
// only exposes through Inertia shared props embedded in authenticated pages.
//
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type styleEntry struct {
	Label  string `json:"label"`
	Style  string `json:"style"`
	Prefix string `json:"prefix"`
	Suffix string `json:"suffix"`
}

func newStylesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "styles",
		Short:       "Browse Artistly's style catalog",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newStylesListCmd(flags))
	return cmd
}

func newStylesListCmd(flags *rootFlags) *cobra.Command {
	var match string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available illustrator styles (optionally filtered with --match)",
		Long: trimLong(`
List the style catalog Artistly exposes on its generation pages. Use --match to
fuzzy-filter by label so you can resolve a human term (e.g. "watercolor") to the
style value 'generate --style' expects.`),
		Example:     "  artistly-pp-cli styles list --match watercolor --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			props, err := sharedProps(ctx, c)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var styles []styleEntry
			if raw, ok := props["illustratorStyles"]; ok {
				_ = json.Unmarshal(raw, &styles)
			}
			if match != "" {
				filtered := styles[:0:0]
				for _, s := range styles {
					if strings.Contains(strings.ToLower(s.Label), strings.ToLower(match)) ||
						strings.Contains(strings.ToLower(s.Style), strings.ToLower(match)) {
						filtered = append(filtered, s)
					}
				}
				styles = filtered
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), styles, flags)
			}
			if len(styles) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No styles found.")
				return nil
			}
			for _, s := range styles {
				fmt.Fprintf(cmd.OutOrStdout(), "%-28s %s\n", s.Label, s.Style)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&match, "match", "", "Filter styles whose label or value contains this text")
	return cmd
}
