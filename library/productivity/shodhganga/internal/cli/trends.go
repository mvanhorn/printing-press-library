// Copyright 2026 Vikas and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: subject/year trend analysis over the harvested corpus.

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/shodhganga/internal/dspace"
)

type yearCount struct {
	Year  string `json:"year"`
	Count int    `json:"count"`
}

type trendsResult struct {
	Subject string      `json:"subject,omitempty"`
	Total   int         `json:"total"`
	ByYear  []yearCount `json:"by_year"`
}

// pp:data-source local
func newNovelTrendsCmd(flags *rootFlags) *cobra.Command {
	var (
		flagSubject string
		dbPath      string
	)
	cmd := &cobra.Command{
		Use:   "trends",
		Short: "Show how thesis counts change over completion year across the harvested corpus.",
		Long: "Group harvested theses by completion year. With --subject, only theses whose\n" +
			"keywords match the subject are counted. Populate the store first with:\n" +
			"shodhganga-pp-cli harvest <query>.",
		Example:     "  shodhganga-pp-cli trends --subject Physics --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, ok, err := openThesisStoreRead(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			defer s.Close()

			theses, err := loadTheses(s)
			if err != nil {
				return err
			}
			counts := map[string]int{}
			total := 0
			for _, t := range theses {
				if flagSubject != "" && !thesisMatchesSubject(t, flagSubject) {
					continue
				}
				y := yearOf(t.CompletedDate)
				if y == "" {
					y = "unknown"
				}
				counts[y]++
				total++
			}
			res := trendsResult{Subject: flagSubject, Total: total, ByYear: []yearCount{}}
			for y, c := range counts {
				res.ByYear = append(res.ByYear, yearCount{Year: y, Count: c})
			}
			sort.Slice(res.ByYear, func(i, j int) bool {
				return res.ByYear[i].Year < res.ByYear[j].Year
			})

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			if total == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matching theses in the local store. Harvest more with: shodhganga-pp-cli harvest <query>")
				return nil
			}
			label := "all subjects"
			if flagSubject != "" {
				label = flagSubject
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Thesis completion trend for %s (%d total):\n", label, total)
			for _, yc := range res.ByYear {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %d\n", yc.Year, yc.Count)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSubject, "subject", "", "restrict to theses whose keywords match this subject")
	cmd.Flags().StringVar(&dbPath, "db", "", "database path (default: standard cache location)")
	return cmd
}

func thesisMatchesSubject(t dspace.Thesis, subject string) bool {
	for _, k := range t.Keywords {
		if containsFold(k, subject) {
			return true
		}
	}
	return false
}
