// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

// industry-map joins Placeit's 152-entry industry taxonomy against the local
// catalog mirror to report template counts per industry — a taxonomy view with
// volume the flat Placeit UI never shows.
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/placeit/internal/algolia"
	"github.com/spf13/cobra"
)

func newNovelIndustryMapCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "industry-map [industry]",
		Short: "Map Placeit's 152-entry industry taxonomy with template counts from your mirror.",
		Long: strings.Trim(`
Fetch Placeit's industry taxonomy and join it against your local mirror to show
how many cached templates match each industry — sized volume the flat Placeit
UI never surfaces. Run 'sync' first. Pass an industry name to drill into one
industry with sample templates.

Use to navigate the industry taxonomy with counts. The 'industries' command
lists entries flat with no counts.`, "\n"),
		Example: strings.Trim(`
  placeit-pp-cli industry-map --agent
  placeit-pp-cli industry-map "coffee shop"`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			query := strings.Join(args, " ")

			// Live drill-down: for a single industry, count and sample directly
			// from Algolia — no sync needed. Full-map mode still uses the mirror.
			if query != "" && flags.dataSource != "local" {
				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()
				ac := algolia.New(flags.timeout)
				label := query
				var tags []string
				// Canonicalize the industry name/tags when the taxonomy matches,
				// but don't require it — the catalog count below uses the label.
				if ind, ierr := ac.Search(ctx, algolia.SearchParams{Index: algolia.IndexIndustries, Query: query, HitsPerPage: 1}); ierr == nil && len(ind.Hits) > 0 {
					var im map[string]any
					if json.Unmarshal(ind.Hits[0], &im) == nil {
						if name, _ := im["industry"].(string); name != "" {
							label = name
						}
						tags = tagValues(im, "tags")
					}
				}
				cnt, cerr := ac.Search(ctx, algolia.SearchParams{Index: algolia.IndexMain, Query: label, HitsPerPage: 10})
				if cerr == nil {
					samples := make([]map[string]any, 0, len(cnt.Hits))
					for _, h := range cnt.Hits {
						if m, e := cleanStage(h); e == nil {
							samples = append(samples, projectStage(m))
						}
					}
					if len(tags) > 5 {
						tags = tags[:5]
					}
					return flags.printJSON(cmd, map[string]any{
						"industry":       label,
						"match_tags":     tags,
						"template_count": cnt.NbHits,
						"samples":        samples,
					})
				}
				if flags.dataSource == "live" {
					return apiErr(cerr)
				}
				// auto: fall through to mirror path below
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
			// Precompute one lowercase haystack per template.
			haystacks := make([]string, len(all))
			for i, m := range all {
				haystacks[i] = strings.ToLower(fmt.Sprint(m["name"]) + " " +
					strings.Join(tagValues(m, "device_tags"), " ") + " " +
					strings.Join(tagValues(m, "stage_tags"), " ") + " " +
					strings.Join(tagValues(m, "bundle_tags"), " "))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			ac := algolia.New(flags.timeout)
			res, err := ac.Search(ctx, algolia.SearchParams{
				Index:       algolia.IndexIndustries,
				Query:       query,
				HitsPerPage: 200,
			})
			if err != nil {
				return apiErr(err)
			}

			type indEntry struct {
				Name  string   `json:"industry"`
				Tags  []string `json:"match_tags"`
				Count int      `json:"template_count"`
			}
			entries := make([]indEntry, 0, len(res.Hits))
			for _, h := range res.Hits {
				var im map[string]any
				if json.Unmarshal(h, &im) != nil {
					continue
				}
				name, _ := im["industry"].(string)
				if name == "" {
					continue
				}
				needles := []string{strings.ToLower(name)}
				for i, t := range tagValues(im, "tags") {
					if i >= 5 {
						break
					}
					needles = append(needles, strings.ToLower(t))
				}
				count := 0
				for _, hs := range haystacks {
					for _, n := range needles {
						if n != "" && strings.Contains(hs, n) {
							count++
							break
						}
					}
				}
				entries = append(entries, indEntry{Name: name, Tags: needles[1:], Count: count})
			}
			sort.SliceStable(entries, func(i, j int) bool { return entries[i].Count > entries[j].Count })

			// Single-industry drill-down: include top sample templates.
			if query != "" && len(entries) > 0 {
				top := entries[0]
				needles := append([]string{strings.ToLower(top.Name)}, top.Tags...)
				type samp struct {
					m         map[string]any
					purchases float64
				}
				samples := make([]samp, 0)
				for i, m := range all {
					for _, n := range needles {
						if n != "" && strings.Contains(haystacks[i], n) {
							samples = append(samples, samp{m: m, purchases: asFloat(m["purchases"])})
							break
						}
					}
				}
				sort.SliceStable(samples, func(i, j int) bool { return samples[i].purchases > samples[j].purchases })
				out := make([]map[string]any, 0, 10)
				for i, s := range samples {
					if i >= 10 {
						break
					}
					out = append(out, projectStage(s.m))
				}
				return flags.printJSON(cmd, map[string]any{
					"industry":       top.Name,
					"match_tags":     top.Tags,
					"template_count": top.Count,
					"samples":        out,
				})
			}

			if len(entries) > limit {
				entries = entries[:limit]
			}
			return flags.printJSON(cmd, entries)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum industries to return (full-map mode)")
	return cmd
}
