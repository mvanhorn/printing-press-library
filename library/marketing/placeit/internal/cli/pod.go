// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

// pod returns Printify-ready mockups for a query with deep links — a POD
// listing pipeline filter the Placeit UI never exposes.
// pp:data-source local

package cli

import (
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/placeit/internal/algolia"
	"github.com/spf13/cobra"
)

func newNovelPodCmd(flags *rootFlags) *cobra.Command {
	var device string
	var limit int

	cmd := &cobra.Command{
		Use:   "pod [query]",
		Short: "Return only Printify-compatible (POD-ready) mockups for a query, with deep links.",
		Long: strings.Trim(`
Filter the cached catalog to Printify-compatible mockups (is_printify) for a
query, ranked by purchase count, with stage and editor deep links ready for a
print-on-demand listing pipeline. Run 'sync' first to populate the mirror.

Use for finding Printify-compatible mockups for a POD listing pipeline. To
rank any mockup by popularity regardless of POD, use 'top'.`, "\n"),
		Example: strings.Trim(`
  placeit-pp-cli pod "t-shirt" --agent --select name,stage_link,purchases
  placeit-pp-cli pod "hoodie" --limit 20`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.Join(args, " ")

			// Live-first: filter Algolia to Printify mockups, ranked best-selling.
			// No sync needed. --data-source local reads the offline mirror.
			if flags.dataSource != "local" {
				ff, _, ferr := buildFacetFilters("", "", device, nil)
				if ferr != nil {
					_ = cmd.Usage()
					return usageErr(ferr)
				}
				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()
				ac := algolia.New(flags.timeout)
				res, serr := ac.Search(ctx, algolia.SearchParams{
					Index:        algolia.IndexBestSelling,
					Query:        query,
					HitsPerPage:  limit,
					FacetFilters: ff,
					Filters:      "is_printify=1",
				})
				if serr != nil {
					if flags.dataSource == "live" {
						return apiErr(serr)
					}
					// auto: fall through to local mirror below
				} else {
					out := make([]map[string]any, 0, len(res.Hits))
					for _, h := range res.Hits {
						m, cerr := cleanStage(h)
						if cerr != nil {
							continue
						}
						p := projectStage(m)
						editor, _ := m["editor_link"].(string)
						if editor != "" && !strings.HasPrefix(editor, "http") {
							editor = "https://placeit.net" + editor
						}
						p["editor_link"] = editor
						out = append(out, p)
					}
					printProvenance(cmd, len(out), DataProvenance{Source: "live", ResourceType: templateResource})
					return flags.printJSON(cmd, out)
				}
			}

			db, ok, err := openCatalogStore(cmd, flags)
			if err != nil || !ok {
				return err
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, templateResource) {
				hintIfStale(cmd, db, templateResource, flags.maxAge)
			}
			all, err := loadCatalog(db)
			if err != nil {
				return apiErr(err)
			}
			type row struct {
				m         map[string]any
				purchases float64
			}
			matches := make([]row, 0)
			for _, m := range all {
				if !truthy(m["is_printify"]) {
					continue
				}
				if device != "" && !containsTag(m, "device_tags", device) {
					continue
				}
				if !matchesQuery(m, query) {
					continue
				}
				matches = append(matches, row{m: m, purchases: asFloat(m["purchases"])})
			}
			sort.SliceStable(matches, func(i, j int) bool { return matches[i].purchases > matches[j].purchases })
			if len(matches) > limit {
				matches = matches[:limit]
			}
			out := make([]map[string]any, 0, len(matches))
			for _, r := range matches {
				p := projectStage(r.m)
				editor, _ := r.m["editor_link"].(string)
				if editor != "" && !strings.HasPrefix(editor, "http") {
					editor = "https://placeit.net" + editor
				}
				p["editor_link"] = editor
				out = append(out, p)
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&device, "device", "", "Filter by device tag (e.g. 'T-Shirt', 'Hoodie')")
	cmd.Flags().IntVar(&limit, "limit", 15, "Maximum results to return")
	return cmd
}
