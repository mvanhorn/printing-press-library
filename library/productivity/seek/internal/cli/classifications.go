// Copyright 2026 Paul Siola and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: SEEK classification-taxonomy resolver.
// pp:data-source computed

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type classificationMatch struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Parent     string            `json:"parent_id,omitempty"`
	ParentName string            `json:"parent_name,omitempty"`
	Kind       string            `json:"kind"` // "classification" | "subclassification"
	Subclasses map[string]string `json:"subclasses,omitempty"`
}

func newNovelClassificationsCmd(flags *rootFlags) *cobra.Command {
	var flagSub bool

	cmd := &cobra.Command{
		Use:   "classifications [term]",
		Short: "Browse or resolve SEEK's numeric classification and subclassification IDs",
		Long: "Use 'classifications' to discover the numeric IDs for 'listings search --classification'\n" +
			"and 'salary --classification'. With no argument it lists every top-level classification;\n" +
			"with a term it returns matching classifications and (with --sub) subclassifications.\n" +
			"Work-type, salary and date filters are resolved inline by 'listings search'.",
		Example: strings.Trim(`
  seek-pp-cli classifications
  seek-pp-cli classifications software --sub
  seek-pp-cli classifications 6281 --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "computed", "pp:no-error-path-probe": "true", "pp:happy-args": "term=software"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "classifications")
			}
			if flags.dataSource == "local" || flags.dataSource == "live" {
				return fmt.Errorf("classifications is a computed reference command; --data-source %q does not apply", flags.dataSource)
			}

			// Accept an optional leading verb ("resolve <term>" / "list") so the
			// manifest's `classifications [list | resolve <term>]` shape works.
			if len(args) > 0 {
				switch strings.ToLower(args[0]) {
				case "resolve", "list", "search", "find":
					args = args[1:]
				}
			}
			term := strings.ToLower(strings.TrimSpace(strings.Join(args, " ")))
			matches := make([]classificationMatch, 0)

			for _, cl := range seekClassifications {
				clMatch := term == "" ||
					cl.ID == term ||
					strings.Contains(strings.ToLower(cl.Name), term)
				if clMatch {
					m := classificationMatch{ID: cl.ID, Name: cl.Name, Kind: "classification"}
					if flagSub || term == "" {
						m.Subclasses = cl.Subclasses
					}
					matches = append(matches, m)
				}
				if term == "" {
					continue
				}
				for sid, sname := range cl.Subclasses {
					if sid == term || strings.Contains(strings.ToLower(sname), term) {
						matches = append(matches, classificationMatch{
							ID: sid, Name: sname, Kind: "subclassification",
							Parent: cl.ID, ParentName: cl.Name,
						})
					}
				}
			}
			sort.Slice(matches, func(i, j int) bool {
				if matches[i].Kind != matches[j].Kind {
					return matches[i].Kind == "classification"
				}
				return matches[i].Name < matches[j].Name
			})

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), matches, flags)
			}
			w := cmd.OutOrStdout()
			if len(matches) == 0 {
				fmt.Fprintf(w, "No SEEK classification matches %q.\n", term)
				return nil
			}
			for _, m := range matches {
				if m.Kind == "subclassification" {
					fmt.Fprintf(w, "  %-7s %s  (under %s %s)\n", m.ID, m.Name, m.Parent, m.ParentName)
					continue
				}
				fmt.Fprintf(w, "%-7s %s\n", m.ID, m.Name)
				if len(m.Subclasses) > 0 {
					sids := make([]string, 0, len(m.Subclasses))
					for sid := range m.Subclasses {
						sids = append(sids, sid)
					}
					sort.Strings(sids)
					for _, sid := range sids {
						fmt.Fprintf(w, "    %-7s %s\n", sid, m.Subclasses[sid])
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagSub, "sub", false, "Include subclassifications for matched classifications")
	return cmd
}
