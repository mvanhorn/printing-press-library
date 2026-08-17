// Copyright 2026 Vikas and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: guide-centric index over the harvested corpus.

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/shodhganga/internal/dspace"
)

type guideResult struct {
	Guide  string          `json:"guide"`
	Count  int             `json:"count"`
	Theses []dspace.Thesis `json:"theses"`
}

// pp:data-source local
func newNovelGuideCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "guide <name>",
		Short: "List every thesis supervised by a given research guide across the harvested corpus.",
		Long: "Search the local store for theses whose guide (DC.contributor) matches <name>.\n" +
			"Populate the store first with: shodhganga-pp-cli harvest <query>.",
		Example:     "  shodhganga-pp-cli guide \"Ghosh, Sushant G\" --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "name=Ghosh", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a guide name is required"))
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
			res := guideResult{Guide: name, Theses: []dspace.Thesis{}}
			for _, t := range theses {
				for _, g := range t.Guides {
					if containsFold(g, name) {
						res.Theses = append(res.Theses, t)
						break
					}
				}
			}
			sort.Slice(res.Theses, func(i, j int) bool {
				return res.Theses[i].Title < res.Theses[j].Title
			})
			res.Count = len(res.Theses)

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			if res.Count == 0 {
				fmt.Fprintf(cmd.OutOrStdout(),
					"No theses in the local store match guide %q. Harvest more with: shodhganga-pp-cli harvest <query>\n", name)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d thesis/theses guided by %q:\n", res.Count, name)
			for _, t := range res.Theses {
				fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s — %s (%s)\n", t.Handle, t.Title, t.Researcher, t.CompletedDate)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "database path (default: standard cache location)")
	return cmd
}
