// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source auto

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type previewPage struct {
	PageID      string `json:"pageId"`
	Slug        string `json:"slug"`
	Title       string `json:"title,omitempty"`
	LastUpdated string `json:"lastUpdated,omitempty"`
	Reason      string `json:"reason"`
}

type previewCollection struct {
	CollectionID   string `json:"collectionId"`
	Name           string `json:"name,omitempty"`
	UnpublishedIDs int    `json:"unpublishedItems"`
	Drafts         int    `json:"draftItems"`
}

type publishPreviewView struct {
	SiteID            string              `json:"siteId"`
	SiteName          string              `json:"siteName,omitempty"`
	LastPublished     string              `json:"lastPublished,omitempty"`
	DaysSincePublish  int                 `json:"daysSincePublish,omitempty"`
	CustomDomains     []string            `json:"customDomains"`
	PagesChanged      []previewPage       `json:"pagesChangedSincePublish"`
	PagesChangedTotal int                 `json:"pagesChangedTotal"`
	ItemSource        string              `json:"itemSource,omitempty"`
	DraftPages        []previewPage       `json:"draftPages"`
	Collections       []previewCollection `json:"collectionsWithUnpublishedItems"`
	TotalUnpublished  int                 `json:"totalUnpublishedItems"`
	RedirectCount     int                 `json:"redirectRules"`
	WouldChangeAnythg bool                `json:"wouldChangeAnything"`
	Note              string              `json:"note,omitempty"`
}

func newNovelPublishPreviewCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	var maxScanPages int
	var maxCollections int

	cmd := &cobra.Command{
		Use:   "preview [site-id]",
		Short: "Show everything a publish would change, before you trigger the build",
		Long: strings.Trim(`
Use this command to see everything that would change if you published this site
right now.

Do NOT use this command for a field-level comparison of one collection's staged
and live items; use 'drift' instead. Do NOT use it for a rollup across every
site you sync; use 'overview' instead.

Sites and pages come from the local mirror, so run
'webflow-pp-cli sync --resources sites' first. Collections and CMS items are
fetched from the API because generated sync cannot mirror them; --max-scan-pages
bounds that fan-out.
`, "\n"),
		Example: strings.Trim(`
  webflow-pp-cli publish preview 580e63e98c9a982ac9b8b741 --agent
  webflow-pp-cli publish preview 580e63e98c9a982ac9b8b741 --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			// A wrong id and an unreachable data source are indistinguishable
			// here: both yield zero rows. Returning non-zero for the second
			// would make an un-synced, un-credentialed run look like a crash.
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return emitDryRun(cmd, flags, "would summarize pending publish changes")
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
				return emitEmptyLocal(cmd, flags, dbPath, "sites,items", publishPreviewView{
					CustomDomains: []string{}, PagesChanged: []previewPage{},
					DraftPages: []previewPage{}, Collections: []previewCollection{},
					Note: missingTableNote(dbPath, "site"),
				})
			}
			if !hintIfUnsynced(cmd, lq.db, "sites") {
				hintIfStale(cmd, lq.db, "sites", flags.maxAge)
			}

			siteID := resolveSiteID(args)
			notes := make([]string, 0, 2)

			// Drain every result set fully before running the next query so the
			// read pool stays free and query ordering is explicit.
			siteQ := `SELECT "id", "id", "data" FROM "sites"`
			var siteArgs []any
			if siteID != "" {
				siteQ += ` WHERE "id" = ?`
				siteArgs = append(siteArgs, siteID)
			}
			siteRows, err := lq.selectRaw(siteQ, siteArgs...)
			if err != nil {
				return err
			}
			sites := decodeRows[wfSite](siteRows)
			if len(sites) == 0 {
				view := publishPreviewView{
					SiteID:        siteID,
					CustomDomains: []string{},
					PagesChanged:  []previewPage{},
					DraftPages:    []previewPage{},
					Collections:   []previewCollection{},
					Note:          "site not found in the local mirror; run 'webflow-pp-cli sync --resources sites' first",
					ItemSource:    "none",
				}
				return emitLocalResult(cmd, flags, view, func() {
					fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				})
			}
			site := sites[0]

			var pageRows, itemRows, redirectRows, collRows []rawRow
			if lq.hasTable("sites_pages") {
				pageRows, err = lq.selectRaw(
					`SELECT "id", "sites_id", "data" FROM "sites_pages" WHERE "sites_id" = ?`, site.ID)
				if err != nil {
					return err
				}
			}
			if lq.hasTable("sites_collections") {
				collRows, err = lq.selectRaw(
					`SELECT "id", "sites_id", "data" FROM "sites_collections" WHERE "sites_id" = ?`, site.ID)
				if err != nil {
					return err
				}
			}
			if lq.hasTable("items") {
				itemRows, err = lq.selectRaw(`SELECT "id", "collections_id", "data" FROM "items"`)
				if err != nil {
					return err
				}
			}
			// Collections are not reachable by generated sync, so fetch the
			// site's collections and then each one's items. Bounded by
			// --max-collections and --max-scan-pages; this is the only novel
			// command that fans out, and it is scoped to a single site.
			itemSource := "local-mirror"
			if len(collRows) == 0 && flags.dataSource != "local" && maxCollections > 0 {
				c, cerr := flags.newClient()
				if cerr != nil {
					return cerr
				}
				if collRows, err = lq.fetchPaged(c, "/sites/"+escapeSeg(site.ID)+"/collections",
					"collections", site.ID, 2); err != nil {
					if note, degrade := liveFallbackErr(err, "this site's collections"); !degrade {
						return fmt.Errorf("fetching site collections: %w", err)
					} else {
						// Pages still audit fine from the mirror; report the
						// CMS half as unavailable rather than failing outright.
						notes = append(notes, note)
						collRows = nil
						maxCollections = 0
					}
				}
				fetched := make([]rawRow, 0, 64)
				scanned := 0
				for _, cr := range collRows {
					if scanned >= maxCollections {
						break
					}
					scanned++
					got, ferr := lq.fetchPaged(c, "/collections/"+escapeSeg(cr.ID)+"/items",
						"items", cr.ID, maxScanPages)
					if ferr != nil {
						if note, degrade := liveFallbackErr(ferr, "collection items"); degrade {
							notes = append(notes, note)
							break
						}
						return fmt.Errorf("fetching items for collection %s: %w", cr.ID, ferr)
					}
					fetched = append(fetched, got...)
				}
				itemRows = fetched
				itemSource = "api"
				if len(collRows) > maxCollections {
					notes = append(notes, fmt.Sprintf(
						"scanned %d of %d collections; raise --max-collections to cover the rest",
						maxCollections, len(collRows)))
				}
			}
			if lq.hasTable("redirects") {
				redirectRows, err = lq.selectRaw(
					`SELECT "id", "sites_id", "data" FROM "redirects" WHERE "sites_id" = ?`, site.ID)
				if err != nil {
					return err
				}
			}

			view := buildPublishPreview(site, pageRows, collRows, itemRows, len(redirectRows), limit)
			view.ItemSource = itemSource
			if view.Note != "" {
				notes = append(notes, view.Note)
			}
			view.Note = strings.Join(notes, "; ")

			return emitLocalResult(cmd, flags, view, func() {
				if !view.WouldChangeAnythg {
					fmt.Fprintf(cmd.OutOrStdout(), "%s: nothing pending; last published %s\n", view.SiteName, view.LastPublished)
					return
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s — last published %s (%d days ago)\n\n",
					view.SiteName, view.LastPublished, view.DaysSincePublish)
				fmt.Fprintf(cmd.OutOrStdout(), "  pages changed since publish : %d\n", view.PagesChangedTotal)
				if view.PagesChangedTotal > len(view.PagesChanged) {
					fmt.Fprintf(cmd.OutOrStdout(), "    (listing %d; raise --limit for the rest)\n", len(view.PagesChanged))
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  draft pages (will not ship) : %d\n", len(view.DraftPages))
				fmt.Fprintf(cmd.OutOrStdout(), "  unpublished CMS items       : %d\n", view.TotalUnpublished)
				fmt.Fprintf(cmd.OutOrStdout(), "  redirect rules in place     : %d\n", view.RedirectCount)
				for _, p := range view.PagesChanged {
					fmt.Fprintf(cmd.OutOrStdout(), "\n  page  %-30s %s", p.Slug, p.Reason)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			})
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Local mirror path (defaults to the standard data directory)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum changed pages to list (pagesChangedTotal still reflects every page scanned)")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 5, "When fetching items from the API, maximum 100-item pages per collection")
	cmd.Flags().IntVar(&maxCollections, "max-collections", 10, "When fetching items from the API, maximum collections to scan for this site")
	return cmd
}

// buildPublishPreview joins sites, pages, collections, and items into the
// pre-flight answer. Split out so it is testable without a store.
func buildPublishPreview(site wfSite, pageRows, collRows, itemRows []rawRow, redirectCount, limit int) publishPreviewView {
	view := publishPreviewView{
		SiteID:        site.ID,
		SiteName:      site.DisplayName,
		LastPublished: site.LastPublished,
		CustomDomains: make([]string, 0, len(site.CustomDomains)),
		PagesChanged:  make([]previewPage, 0, 8),
		DraftPages:    make([]previewPage, 0, 4),
		Collections:   make([]previewCollection, 0, 4),
		RedirectCount: redirectCount,
	}
	for _, d := range site.CustomDomains {
		if d.URL != "" {
			view.CustomDomains = append(view.CustomDomains, d.URL)
		}
	}
	if n, ok := daysSince(site.LastPublished); ok {
		view.DaysSincePublish = n
	}

	published, publishedOK := parseWFTime(site.LastPublished)
	for _, p := range decodeRows[wfPage](pageRows) {
		if p.isArchived() {
			continue
		}
		if p.isDraft() {
			view.DraftPages = append(view.DraftPages, previewPage{
				PageID: p.ID, Slug: p.label(), Title: p.Title,
				LastUpdated: p.LastUpdated,
				Reason:      "page is a draft and will not go live on publish",
			})
			continue
		}
		updated, updatedOK := parseWFTime(p.LastUpdated)
		switch {
		case !publishedOK:
			view.PagesChanged = append(view.PagesChanged, previewPage{
				PageID: p.ID, Slug: p.label(), Title: p.Title,
				LastUpdated: p.LastUpdated,
				Reason:      "site has never been published",
			})
		case updatedOK && updated.After(published):
			view.PagesChanged = append(view.PagesChanged, previewPage{
				PageID: p.ID, Slug: p.label(), Title: p.Title,
				LastUpdated: p.LastUpdated,
				Reason:      "page edited after the last site publish",
			})
		}
	}

	// Collection names come from the site's collections; item counts come from
	// the flat items table keyed by collections_id.
	names := map[string]string{}
	inSite := map[string]bool{}
	for i, c := range decodeRows[wfCollection](collRows) {
		id := c.ID
		if id == "" && i < len(collRows) {
			id = collRows[i].ID
		}
		if id == "" {
			continue
		}
		inSite[id] = true
		names[id] = c.DisplayName
	}

	perColl := map[string]*previewCollection{}
	for _, r := range itemRows {
		coll := r.Scope
		// When the mirror holds items for several sites, only count the ones
		// belonging to a collection on this site. If collections were not
		// synced we cannot scope, so fall back to counting everything and say
		// so in the note.
		if len(inSite) > 0 && !inSite[coll] {
			continue
		}
		decoded := decodeRows[wfItem]([]rawRow{r})
		if len(decoded) == 0 {
			continue
		}
		it := decoded[0]
		if it.archived() {
			continue
		}
		entry, ok := perColl[coll]
		if !ok {
			entry = &previewCollection{CollectionID: coll, Name: names[coll]}
			perColl[coll] = entry
		}
		if it.draft() {
			entry.Drafts++
			continue
		}
		if hasUnpublishedChanges(it) {
			entry.UnpublishedIDs++
			view.TotalUnpublished++
		}
	}
	if len(inSite) == 0 && len(itemRows) > 0 {
		view.Note = "collections were not synced, so CMS item counts span every collection in the mirror; run 'webflow-pp-cli sync --resources sites' to scope them to this site"
	}

	for _, c := range perColl {
		if c.UnpublishedIDs > 0 || c.Drafts > 0 {
			view.Collections = append(view.Collections, *c)
		}
	}
	sort.SliceStable(view.Collections, func(i, j int) bool {
		if view.Collections[i].UnpublishedIDs != view.Collections[j].UnpublishedIDs {
			return view.Collections[i].UnpublishedIDs > view.Collections[j].UnpublishedIDs
		}
		return view.Collections[i].CollectionID < view.Collections[j].CollectionID
	})

	sort.SliceStable(view.PagesChanged, func(i, j int) bool {
		return view.PagesChanged[i].Slug < view.PagesChanged[j].Slug
	})
	view.PagesChangedTotal = len(view.PagesChanged)
	if limit > 0 && len(view.PagesChanged) > limit {
		view.PagesChanged = view.PagesChanged[:limit]
	}

	view.WouldChangeAnythg = len(view.PagesChanged) > 0 || view.TotalUnpublished > 0
	return view
}
