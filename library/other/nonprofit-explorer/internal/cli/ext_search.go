// Copyright 2026 Sean Fannan and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored command extension for the Nonprofit Explorer CLI (recorded in
// .printing-press-patches/ext-nonprofit-commands.md). Shared helpers live in
// ext_nonprofit.go; registration is in root.go.

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newNPSearchCmd(flags *rootFlags) *cobra.Command {
	var state, page string
	var ntee, ccode, limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search US nonprofits by name/keyword (ranked); filter by state, NTEE group, 501(c) code",
		Long: "Search ProPublica's Nonprofit Explorer for organizations by name or keyword.\n" +
			"Returns a ranked list with EIN, name, city/state and cause area.\n\n" +
			"Filters:\n" +
			"  --state    two-letter US state (e.g. CA)\n" +
			"  --ntee     NTEE major group 1-10\n" +
			"  --c-code   501(c) sub-code (3 = 501(c)(3))",
		Example: "  nonprofit-explorer-pp-cli search \"marcella foundation\"\n" +
			"  nonprofit-explorer-pp-cli search \"food bank\" --state CA\n" +
			"  nonprofit-explorer-pp-cli search \"education\" --state NV --c-code 3 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			q := strings.TrimSpace(strings.Join(args, " "))
			// Bare `search` with no query and no filter flags would dump the
			// unfiltered full registry page — show usage instead.
			if q == "" && !hasChangedLocalFlags(cmd) {
				return cmd.Help()
			}
			pageN := 0
			if page != "" {
				pageN, _ = strconv.Atoi(page)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := fetchSearch(cmd.Context(), c, q, state, ntee, ccode, pageN)
			if err != nil {
				// ProPublica returns HTTP 404 (not 200 + empty array) when a
				// query matches nothing. A zero-match search is a successful
				// search: render "0 results" and exit 0.
				if strings.Contains(err.Error(), "HTTP 404") {
					resp = &npSearchResp{}
				} else {
					return classifyAPIError(err, flags)
				}
			}
			orgs := resp.Organizations
			if limit > 0 && len(orgs) > limit {
				orgs = orgs[:limit]
			}
			if flags.asJSON {
				out := map[string]any{
					"query":         q,
					"total_results": resp.TotalResults,
					"num_pages":     resp.NumPages,
					"count":         len(orgs),
					"organizations": orgs,
				}
				return printJSONLive(cmd, flags, out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d results", resp.TotalResults)
			if resp.NumPages > 1 {
				fmt.Fprintf(cmd.OutOrStdout(), " across %d pages (showing page %d)", resp.NumPages, pageN)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n\n")
			rows := make([][]string, 0, len(orgs))
			for _, o := range orgs {
				cat := nteeName(o.NteeCode)
				loc := strings.TrimSpace(o.City + ", " + o.State)
				rows = append(rows, []string{o.StrEIN, o.Name, loc, o.NteeCode, cat})
			}
			return flags.printTable(cmd, []string{"EIN", "NAME", "LOCATION", "NTEE", "CATEGORY"}, rows)
		},
	}
	cmd.Flags().StringVar(&state, "state", "", "Two-letter US state filter (e.g. CA)")
	cmd.Flags().IntVar(&ntee, "ntee", 0, "NTEE major group filter (1-10)")
	cmd.Flags().IntVar(&ccode, "c-code", 0, "501(c) sub-code filter (3 = 501(c)(3))")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max results to show (0 = all on page)")
	cmd.Flags().StringVar(&page, "page", "", "Zero-based page number")
	return cmd
}
