// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type overviewSite struct {
	SiteID           string `json:"siteId"`
	DisplayName      string `json:"displayName"`
	ShortName        string `json:"shortName,omitempty"`
	Pages            int    `json:"pages"`
	DraftPages       int    `json:"draftPages"`
	Collections      *int   `json:"collections"`
	UnpublishedItems *int   `json:"unpublishedItems"`
	SEOFindings      int    `json:"seoFindings"`
	Redirects        *int   `json:"redirects"`
	LastPublished    string `json:"lastPublished,omitempty"`
	DaysSincePublish int    `json:"daysSincePublish"`
}

type overviewView struct {
	Sites            []overviewSite `json:"sites"`
	SitesTotal       int            `json:"sitesTotal"`
	UnpublishedTotal *int           `json:"unpublishedItemsTotal"`
	FindingsTotal    int            `json:"seoFindingsTotal"`
	Note             string         `json:"note,omitempty"`
}

func newNovelOverviewCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "overview",
		Short: "One row per synced site: pages, SEO findings, and days since publish",
		Long: strings.Trim(`
Use this command for a one-row-per-site rollup across every site in the local
mirror.

Do NOT use this command to inspect what a single site's next publish would
change; use 'publish preview' instead. Do NOT use it for page-level SEO
findings on one site; use 'seo audit' instead.

Reads the local mirror only. Run
'webflow-pp-cli sync --resources sites,items' first.
`, "\n"),
		Example: strings.Trim(`
  webflow-pp-cli overview --agent
  webflow-pp-cli overview --json --select sites.displayName,sites.unpublishedItems
  webflow-pp-cli overview --csv
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "would roll up every synced site from the local mirror")
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("overview has no live equivalent; it reads the local mirror. Run 'webflow-pp-cli sync --resources sites,items' then retry"))
			}

			lq, cleanup, ok, err := openLocalMirror(cmd, flags, dbPath, "sites,items")
			if err != nil {
				return err
			}
			defer cleanup()
			if !ok {
				return nil
			}
			if !lq.hasTable("sites") {
				return emitEmptyLocal(cmd, flags, dbPath, "sites,items", overviewView{
					Sites: make([]overviewSite, 0),
					Note:  missingTableNote(dbPath, "site"),
				})
			}
			if !hintIfUnsynced(cmd, lq.db, "sites") {
				hintIfStale(cmd, lq.db, "sites", flags.maxAge)
			}

			// Every result set is drained before the next query runs.
			siteRows, err := lq.selectRaw(`SELECT "id", "id", "data" FROM "sites"`)
			if err != nil {
				return err
			}
			var pageRows, collRows, itemRows, redirectRows []rawRow
			if lq.hasTable("sites_pages") {
				if pageRows, err = lq.selectRaw(`SELECT "id", "sites_id", "data" FROM "sites_pages"`); err != nil {
					return err
				}
			}
			if lq.hasTable("sites_collections") {
				if collRows, err = lq.selectRaw(`SELECT "id", "sites_id", "data" FROM "sites_collections"`); err != nil {
					return err
				}
			}
			if lq.hasTable("items") {
				if itemRows, err = lq.selectRaw(`SELECT "id", "collections_id", "data" FROM "items"`); err != nil {
					return err
				}
			}
			if lq.hasTable("redirects") {
				if redirectRows, err = lq.selectRaw(`SELECT "id", "sites_id", "data" FROM "redirects"`); err != nil {
					return err
				}
			}

			view := buildOverview(siteRows, pageRows, collRows, itemRows, redirectRows)
			if view.SitesTotal == 0 {
				view.Note = "no sites in the local mirror; run 'webflow-pp-cli sync --resources sites' first"
			}

			return emitLocalResult(cmd, flags, view, func() {
				if len(view.Sites) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), view.Note)
					return
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-32s %6s %6s %8s %8s %8s\n",
					"SITE", "PAGES", "COLLS", "UNPUB", "SEO", "DAYS")
				cell := func(p *int) string {
					if p == nil {
						return "-"
					}
					return fmt.Sprintf("%d", *p)
				}
				for _, s := range view.Sites {
					fmt.Fprintf(cmd.OutOrStdout(), "%-32s %6d %6s %8s %8d %8d\n",
						truncate(s.DisplayName, 32), s.Pages, cell(s.Collections),
						cell(s.UnpublishedItems), s.SEOFindings, s.DaysSincePublish)
				}
				if view.Note != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", view.Note)
				}
			})
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Local mirror path (defaults to the standard data directory)")
	return cmd
}

// buildOverview aggregates every synced table into one row per site. Split out
// so it is testable without a store.
func buildOverview(siteRows, pageRows, collRows, itemRows, redirectRows []rawRow) overviewView {
	view := overviewView{Sites: make([]overviewSite, 0, len(siteRows))}
	haveCollections := len(collRows) > 0
	haveItems := len(itemRows) > 0
	haveRedirects := len(redirectRows) > 0
	unpublishedTotal := 0
	haveUnpublishedTotal := false

	rowsBySite := func(rows []rawRow) map[string][]rawRow {
		m := map[string][]rawRow{}
		for _, r := range rows {
			m[r.Scope] = append(m[r.Scope], r)
		}
		return m
	}
	pagesBySite := rowsBySite(pageRows)
	collsBySite := rowsBySite(collRows)
	redirectsBySite := rowsBySite(redirectRows)

	itemsByColl := map[string][]rawRow{}
	for _, r := range itemRows {
		itemsByColl[r.Scope] = append(itemsByColl[r.Scope], r)
	}

	for i, site := range decodeRows[wfSite](siteRows) {
		id := site.ID
		if id == "" && i < len(siteRows) {
			id = siteRows[i].ID
		}
		row := overviewSite{
			SiteID:        id,
			DisplayName:   site.DisplayName,
			ShortName:     site.ShortName,
			LastPublished: site.LastPublished,
		}
		// null, not 0: generated sync cannot mirror redirects, collections, or
		// items, so reporting 0 here would contradict `publish preview` on the
		// same mirror with no explanation.
		if haveRedirects {
			n := len(redirectsBySite[id])
			row.Redirects = &n
		}
		if n, ok := daysSince(site.LastPublished); ok {
			row.DaysSincePublish = n
		}

		pages := decodeRows[wfPage](pagesBySite[id])
		row.Pages = len(pages)
		for _, p := range pages {
			if p.isDraft() {
				row.DraftPages++
			}
		}
		audit := auditPages(pages, id, seoThresholds{
			titleMax: defaultSEOTitleMax,
			descMax:  defaultSEODescMax,
			descMin:  defaultSEODescMin,
		})
		row.SEOFindings = len(audit.Findings)

		collIDs := make([]string, 0, len(collsBySite[id]))
		for j, c := range decodeRows[wfCollection](collsBySite[id]) {
			cid := c.ID
			if cid == "" && j < len(collsBySite[id]) {
				cid = collsBySite[id][j].ID
			}
			if cid != "" {
				collIDs = append(collIDs, cid)
			}
		}
		if haveCollections {
			n := len(collIDs)
			row.Collections = &n
		}
		if haveCollections && haveItems {
			unpub := 0
			for _, cid := range collIDs {
				for _, it := range decodeRows[wfItem](itemsByColl[cid]) {
					if it.archived() || it.draft() {
						continue
					}
					if hasUnpublishedChanges(it) {
						unpub++
					}
				}
			}
			row.UnpublishedItems = &unpub
		}

		view.Sites = append(view.Sites, row)
		view.SitesTotal++
		if row.UnpublishedItems != nil {
			unpublishedTotal += *row.UnpublishedItems
			haveUnpublishedTotal = true
		}
		view.FindingsTotal += row.SEOFindings
	}

	if haveUnpublishedTotal {
		view.UnpublishedTotal = &unpublishedTotal
	}
	if !haveCollections || !haveItems || !haveRedirects {
		view.Note = "collections, CMS items, and redirects are not reachable by this CLI's sync, so those columns are reported as null rather than zero. Use 'publish preview <site-id>' for a live per-site answer."
	}
	sort.SliceStable(view.Sites, func(i, j int) bool {
		a, b := 0, 0
		if view.Sites[i].UnpublishedItems != nil {
			a = *view.Sites[i].UnpublishedItems
		}
		if view.Sites[j].UnpublishedItems != nil {
			b = *view.Sites[j].UnpublishedItems
		}
		if a != b {
			return a > b
		}
		return view.Sites[i].DisplayName < view.Sites[j].DisplayName
	})
	return view
}
