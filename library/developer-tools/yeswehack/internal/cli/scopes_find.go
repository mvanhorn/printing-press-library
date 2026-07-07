// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: scopes find - regex search across every synced program scope.

package cli

import (
	"fmt"
	"regexp"

	"github.com/spf13/cobra"
)

type scopeFindHit struct {
	Asset   string `json:"asset"`
	Program string `json:"program"`
}

func newScopesFindCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "find <pattern>",
		Short: "Regex-search every synced program scope for an asset pattern",
		Long: `Compiles the argument as a Go regexp and matches it against every asset
across every synced program. Useful when you have a candidate finding on an
asset and want to know which program(s) cover it.`,
		Example:     "  yeswehack-pp-cli scopes find 'api-v[0-9]+\\.example\\.com' --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			pat, err := regexp.Compile(args[0])
			if err != nil {
				return fmt.Errorf("invalid regex: %w", err)
			}
			db, err := openDefaultStore()
			if err != nil {
				return err
			}
			defer db.Close()
			programs, err := loadResourceObjects(db, "programs")
			if err != nil {
				return err
			}
			hits := []scopeFindHit{}
			for _, p := range programs {
				slug := programSlug(p)
				for _, s := range scopesFromProgram(p) {
					if pat.MatchString(s.Asset) {
						hits = append(hits, scopeFindHit{Asset: s.Asset, Program: slug})
					}
				}
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), hits, flags)
			}
			if len(hits) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matching assets. Run 'sync programs' first if your local store is empty.")
				return nil
			}
			for _, h := range hits {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", h.Program, h.Asset)
			}
			return nil
		},
	}
	return cmd
}
