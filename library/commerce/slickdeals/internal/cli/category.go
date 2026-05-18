// Copyright 2026 david. Licensed under Apache-2.0. See LICENSE.
// Hand-written for slickdeals-pp-cli v0.2 (rss-browse engineer).

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/slickdeals/internal/rss"
	"github.com/spf13/cobra"
)

func newCategoryCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var listFlag bool

	cmd := &cobra.Command{
		Use:   "category [id|name]",
		Short: "Browse deals by Slickdeals forum category",
		Long: `Fetch live RSS deals for a Slickdeals forum category.

Pass a numeric forum ID or a friendly name from the built-in map.
Use --list to print all known category names and their forum IDs.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  # Browse tech deals (forum 25)
  slickdeals-pp-cli category tech --limit 5

  # Browse by numeric forum ID
  slickdeals-pp-cli category 25 --json

  # List all known category names
  slickdeals-pp-cli category --list`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			// --list: print category map and exit (no API call)
			if listFlag {
				return printCategoryList(cmd, flags)
			}

			if len(args) == 0 {
				return cmd.Help()
			}

			if dryRunOK(flags) {
				return nil
			}

			forumID, err := rss.ResolveCategory(args[0])
			if err != nil {
				return usageErr(err)
			}

			items, err := rss.LiveCategory(cmd.Context(), nil, forumID, limit)
			if err != nil {
				return apiErr(err)
			}

			prov := DataProvenance{
				Source:       "live",
				ResourceType: "category",
			}
			printProvenance(cmd, len(items), prov)

			raw, err := json.Marshal(items)
			if err != nil {
				return err
			}
			wrapped, err := wrapWithProvenance(raw, prov)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), json.RawMessage(wrapped), flags)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 25, "Max number of deals to return")
	cmd.Flags().BoolVar(&listFlag, "list", false, "Print built-in category name→ID map and exit")

	return cmd
}

// printCategoryList prints the CategoryMap in a human/JSON-friendly form.
func printCategoryList(cmd *cobra.Command, flags *rootFlags) error {
	// Deduplicate: one entry per unique forum ID with all aliases.
	idToNames := map[int][]string{}
	for name, id := range rss.CategoryMap {
		idToNames[id] = append(idToNames[id], name)
	}

	type entry struct {
		ForumID int      `json:"forum_id"`
		Names   []string `json:"names"`
	}
	entries := make([]entry, 0, len(idToNames))
	for id, names := range idToNames {
		sort.Strings(names)
		entries = append(entries, entry{ForumID: id, Names: names})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ForumID < entries[j].ForumID
	})

	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		raw, err := json.Marshal(entries)
		if err != nil {
			return err
		}
		return printJSONFiltered(cmd.OutOrStdout(), json.RawMessage(raw), flags)
	}

	tw := newTabWriter(cmd.OutOrStdout())
	fmt.Fprintln(tw, "FORUM ID\tNAMES")
	for _, e := range entries {
		fmt.Fprintf(tw, "%d\t%s\n", e.ForumID, strings.Join(e.Names, ", "))
	}
	return tw.Flush()
}
