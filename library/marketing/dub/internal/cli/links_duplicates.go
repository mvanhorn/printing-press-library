// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

// links_duplicates groups synced links by destination URL and returns clusters
// where two or more short links point to the same target. Reads from the local
// /store cache via store.Open — no API call.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type dupGroup struct {
	URL        string   `json:"url"`
	Count      int      `json:"count"`
	ShortLinks []string `json:"short_links"`
	IDs        []string `json:"ids"`
}

func newLinksDuplicatesCmd(flags *rootFlags) *cobra.Command {
	var ignoreUTM bool

	cmd := &cobra.Command{
		Use:     "duplicates",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Aliases: []string{"dups"},
		Short:   "Find links pointing to the same destination URL",
		Long: `Group every link in the local store by its destination URL and surface every
URL that appears more than once. Useful for consolidation campaigns where the
same landing page accidentally got multiple short links.`,
		Example: `  # Find all duplicate destinations
  dub-pp-cli links duplicates --json

  # Treat utm_*-only differences as the same URL
  dub-pp-cli links duplicates --ignore-utm --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStoreForRead("dub-pp-cli")
			if err != nil {
				return apiErr(fmt.Errorf("open local store: %w", err))
			}
			if db == nil {
				return notFoundErr(fmt.Errorf("no local store found. run `dub-pp-cli sync --full` first"))
			}
			defer db.Close()

			rows, err := db.DB().Query(`SELECT id, data FROM links WHERE archived = 0 OR archived IS NULL`)
			if err != nil {
				return apiErr(fmt.Errorf("query links: %w", err))
			}
			defer rows.Close()

			groups := make(map[string]*dupGroup)
			for rows.Next() {
				var id, raw string
				if err := rows.Scan(&id, &raw); err != nil {
					continue
				}
				var obj map[string]any
				if err := json.Unmarshal([]byte(raw), &obj); err != nil {
					continue
				}
				url := stringField(obj, "url")
				if url == "" {
					continue
				}
				key := normalizeURL(url, ignoreUTM)
				short := stringField(obj, "shortLink")
				if g, ok := groups[key]; ok {
					g.Count++
					g.ShortLinks = append(g.ShortLinks, short)
					g.IDs = append(g.IDs, id)
				} else {
					groups[key] = &dupGroup{URL: url, Count: 1, ShortLinks: []string{short}, IDs: []string{id}}
				}
			}

			out := make([]dupGroup, 0)
			for _, g := range groups {
				if g.Count > 1 {
					out = append(out, *g)
				}
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, out)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no duplicate destinations found")
				return nil
			}
			headers := []string{"COUNT", "URL", "SHORT_LINKS"}
			rowsTbl := make([][]string, 0, len(out))
			for _, g := range out {
				url := g.URL
				if len(url) > 60 {
					url = url[:57] + "..."
				}
				rowsTbl = append(rowsTbl, []string{fmt.Sprintf("%d", g.Count), url, strings.Join(g.ShortLinks, ", ")})
			}
			return flags.printTable(cmd, headers, rowsTbl)
		},
	}

	cmd.Flags().BoolVar(&ignoreUTM, "ignore-utm", false, "Strip utm_* params before comparing destination URLs")
	return cmd
}

// normalizeURL collapses utm_* params (and trims trailing slashes) when ignoreUTM is set.
// Pure logic; tested in links_duplicates_test.go.
func normalizeURL(raw string, ignoreUTM bool) string {
	if !ignoreUTM {
		return raw
	}
	idx := strings.Index(raw, "?")
	if idx < 0 {
		return strings.TrimRight(raw, "/")
	}
	base := strings.TrimRight(raw[:idx], "/")
	params := strings.Split(raw[idx+1:], "&")
	keep := make([]string, 0, len(params))
	for _, p := range params {
		if strings.HasPrefix(strings.ToLower(p), "utm_") {
			continue
		}
		keep = append(keep, p)
	}
	if len(keep) == 0 {
		return base
	}
	sort.Strings(keep)
	return base + "?" + strings.Join(keep, "&")
}
