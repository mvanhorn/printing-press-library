// Copyright 2026 prisco-faiella. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/diretta/internal/parser"
	"github.com/spf13/cobra"
)

func newExportCmd(flags *rootFlags) *cobra.Command {
	var flagFormat string
	var flagOutput string

	cmd := &cobra.Command{
		Use:   "export <tournamentId>",
		Short: "Export all matches and stats for a tournament as CSV or JSON.",
		Long: `Fetches today, yesterday, and tomorrow feeds filtered to a specific
tournament and exports as CSV or JSON. For season-wide exports, combine with
--since on the sync command first.`,
		Example:     "  diretta-pp-cli export naYhNOaA --format csv --output serie-a.csv\n  diretta-pp-cli export naYhNOaA --format json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usageErr(fmt.Errorf("tournamentId is required\nUsage: %s <tournamentId>", cmd.CommandPath()))
			}
			if dryRunOK(flags) {
				return nil
			}
			tournamentID := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Fetch yesterday, today, tomorrow feeds
			paths := []string{
				"/x/feed/f_1_-1_3_it_1",
				"/x/feed/f_1_0_3_it_1",
				"/x/feed/f_1_1_3_it_1",
			}

			var allMatches []map[string]any
			seen := map[string]bool{}
			for _, path := range paths {
				raw, _, ferr := resolveRead(cmd.Context(), c, flags, "matches", false, path, map[string]string{}, nil)
				if ferr != nil {
					continue
				}
				for _, m := range parser.ParseMatches([]byte(raw)) {
					id, _ := m["id"].(string)
					tid, _ := m["tournament_id"].(string)
					if tid != tournamentID {
						continue
					}
					if seen[id] {
						continue
					}
					seen[id] = true
					allMatches = append(allMatches, m)
				}
			}

			if len(allMatches) == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"tournament_id": tournamentID,
						"matches":       []any{},
						"count":         0,
					}, flags)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "No matches found for tournament %s\n", tournamentID)
				return nil
			}

			if flagFormat == "json" {
				data, _ := json.MarshalIndent(allMatches, "", "  ")
				if flagOutput != "" {
					return os.WriteFile(flagOutput, data, 0o644)
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}

			// CSV output
			var w *csv.Writer
			if flagOutput != "" {
				f, ferr := os.Create(flagOutput)
				if ferr != nil {
					return fmt.Errorf("creating output file: %w", ferr)
				}
				defer f.Close()
				w = csv.NewWriter(f)
			} else {
				w = csv.NewWriter(cmd.OutOrStdout())
			}

			// Collect all keys
			keySet := map[string]bool{}
			for _, m := range allMatches {
				for k := range m {
					keySet[k] = true
				}
			}
			var keys []string
			for k := range keySet {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			w.Write(keys)
			for _, m := range allMatches {
				row := make([]string, len(keys))
				for i, k := range keys {
					v := m[k]
					if v == nil {
						row[i] = ""
					} else {
						row[i] = strings.TrimSpace(fmt.Sprintf("%v", v))
					}
				}
				w.Write(row)
			}
			w.Flush()
			if flagOutput != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "Exported %d matches to %s\n", len(allMatches), flagOutput)
			}
			return w.Error()
		},
	}
	cmd.Flags().StringVar(&flagFormat, "format", "csv", "Output format: csv or json")
	cmd.Flags().StringVar(&flagOutput, "output", "", "Write to file instead of stdout")
	return cmd
}
