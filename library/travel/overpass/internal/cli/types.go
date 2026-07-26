// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source computed
//
// `types` prints the built-in photographic-vocabulary -> OpenStreetMap tag
// taxonomy. It is the CLI's own mapping table, not API data: no request is
// made and no store is read, so there is nothing to sync or cache.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/overpass/internal/subjects"

	"github.com/spf13/cobra"
)

func newNovelTypesCmd(flags *rootFlags) *cobra.Command {
	var (
		group string
		typ   string
	)
	cmd := &cobra.Command{
		Use:   "types",
		Short: "Lists every subject type this CLI can find, with the OpenStreetMap tags behind each",
		Long: strings.Trim(`
What can be searched, and what it actually queries.

The mapping from photographic vocabulary to OpenStreetMap tags is the thing
this CLI adds, so it is printed rather than hidden. If a search comes back
empty, checking the tags here is the first thing worth doing: coverage in
OpenStreetMap is volunteer-contributed and very uneven between subjects.
`, "\n"),
		Example: strings.Trim(`
  overpass-pp-cli types
  overpass-pp-cli types --group architecture
  overpass-pp-cli types --type water_tower --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			var list []subjects.Type
			switch {
			case typ != "":
				t, err := subjects.Lookup(typ)
				if err != nil {
					return usageErr(err)
				}
				list = []subjects.Type{t}
			case group != "":
				list = subjects.InGroup(group)
				if len(list) == 0 {
					return usageErr(fmt.Errorf("no group %q; known groups: %s",
						group, strings.Join(subjects.Groups(), ", ")))
				}
			default:
				list = subjects.All()
			}

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"groups": subjects.Groups(), "types": list,
				})
			}

			out := cmd.OutOrStdout()
			rows := make([][]string, 0, len(list))
			for _, t := range list {
				sel := make([]string, 0, len(t.Tags))
				for _, tg := range t.Tags {
					sel = append(sel, tg.Selector())
				}
				rows = append(rows, []string{t.Group, t.Name, t.Description, strings.Join(sel, " OR ")})
			}
			if err := flags.printTable(cmd, []string{"GROUP", "TYPE", "WHAT IT IS", "OSM TAGS"}, rows); err != nil {
				return err
			}
			var noted bool
			for _, t := range list {
				if t.Note != "" {
					if !noted {
						fmt.Fprintln(out, "")
						noted = true
					}
					fmt.Fprintf(out, "note (%s): %s\n", t.Name, t.Note)
				}
			}
			if typ == "" && group == "" {
				fmt.Fprintf(out, "\ngroups: %s\n", strings.Join(subjects.Groups(), ", "))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&group, "group", "", "Show only one group, e.g. architecture, industrial, coastal")
	cmd.Flags().StringVar(&typ, "type", "", "Show only one subject type")
	return cmd
}
