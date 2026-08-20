// Copyright 2026 Vikas and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: aggregate a university's theses into a research profile.

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type subjectCount struct {
	Subject string `json:"subject"`
	Count   int    `json:"count"`
}

type universityStatsResult struct {
	University  string         `json:"university"`
	ThesisCount int            `json:"thesis_count"`
	YearMin     string         `json:"year_min,omitempty"`
	YearMax     string         `json:"year_max,omitempty"`
	Departments []subjectCount `json:"departments"`
	TopSubjects []subjectCount `json:"top_subjects"`
}

// pp:data-source local
func newNovelUniversityStatsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "stats <name>",
		Short: "Aggregate a university's theses in the local store into a profile: count, subjects, year range.",
		Long: "Profile a university's doctoral output from the harvested corpus: thesis count,\n" +
			"completion-year range, departments, and top subjects. Populate the store first\n" +
			"with: shodhganga-pp-cli harvest <query>.",
		Example:     "  shodhganga-pp-cli university stats \"Jamia Millia Islamia\" --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "name=University", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a university name is required"))
			}
			name := args[0]

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
			res := universityStatsResult{
				University:  name,
				Departments: []subjectCount{},
				TopSubjects: []subjectCount{},
			}
			deptCounts := map[string]int{}
			subjCounts := map[string]int{}
			for _, t := range theses {
				if t.University == "" || !containsFold(t.University, name) {
					continue
				}
				res.ThesisCount++
				if y := yearOf(t.CompletedDate); y != "" {
					if res.YearMin == "" || y < res.YearMin {
						res.YearMin = y
					}
					if res.YearMax == "" || y > res.YearMax {
						res.YearMax = y
					}
				}
				if t.Department != "" {
					deptCounts[t.Department]++
				}
				for _, k := range t.Keywords {
					subjCounts[k]++
				}
			}
			res.Departments = topCounts(deptCounts, 0)
			res.TopSubjects = topCounts(subjCounts, 10)

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			if res.ThesisCount == 0 {
				fmt.Fprintf(cmd.OutOrStdout(),
					"No theses in the local store match university %q. Harvest more with: shodhganga-pp-cli harvest <query>\n", name)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s — %d theses", name, res.ThesisCount)
			if res.YearMin != "" {
				fmt.Fprintf(cmd.OutOrStdout(), " (%s–%s)", res.YearMin, res.YearMax)
			}
			fmt.Fprintln(cmd.OutOrStdout())
			if len(res.TopSubjects) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Top subjects:")
				for _, sc := range res.TopSubjects {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-40s %d\n", sc.Subject, sc.Count)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "database path (default: standard cache location)")
	return cmd
}

// topCounts returns count entries sorted by count desc then name; limit<=0 returns all.
func topCounts(m map[string]int, limit int) []subjectCount {
	out := make([]subjectCount, 0, len(m))
	for k, v := range m {
		out = append(out, subjectCount{Subject: k, Count: v})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Subject < out[j].Subject
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
