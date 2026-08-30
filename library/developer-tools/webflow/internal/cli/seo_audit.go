// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"
)

// Google truncates around these lengths in a desktop SERP. They are the
// conventional working limits an SEO consultant audits against, not a Webflow
// API constraint, so they are flags rather than fixed rules.
const (
	defaultSEOTitleMax = 60
	defaultSEODescMax  = 160
	defaultSEODescMin  = 50
)

type seoFinding struct {
	PageID   string `json:"pageId,omitempty"`
	PageSlug string `json:"pageSlug"`
	Title    string `json:"pageTitle,omitempty"`
	Issue    string `json:"issue"`
	Severity string `json:"severity"`
	Detail   string `json:"detail,omitempty"`
}

type seoAuditView struct {
	SiteID           string         `json:"siteId,omitempty"`
	PagesAudited     int            `json:"pagesAudited"`
	PagesUndecodable int            `json:"pagesUndecodable"`
	Findings         []seoFinding   `json:"findings"`
	Counts           map[string]int `json:"countsBySeverity"`
	Note             string         `json:"note,omitempty"`
}

// seoSeverityRank orders output so a caller reading top-down sees errors first
// regardless of the order findings were appended.
var seoSeverityRank = map[string]int{"error": 0, "warning": 1, "info": 2}

func newNovelSeoAuditCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var titleMax, descMax, descMin int
	var includeDrafts bool

	cmd := &cobra.Command{
		Use:   "audit [site-id]",
		Short: "Score every page's SEO metadata for missing, duplicated, and over-length values",
		Long: strings.Trim(`
Use this command to score every page's SEO metadata on one site, including
missing, duplicated, and over-length seo.title and seo.description.

Do NOT use this command for a pre-publish summary of what would change; use
'publish preview' instead. Do NOT use it for CMS collection field gaps; use
'collections completeness' instead.

Reads the local mirror only. Run 'webflow-pp-cli sync --resources sites' first.
`, "\n"),
		Example: strings.Trim(`
  webflow-pp-cli seo audit 580e63e98c9a982ac9b8b741 --agent
  webflow-pp-cli seo audit 580e63e98c9a982ac9b8b741 --json --select findings.pageSlug,findings.issue
  webflow-pp-cli seo audit --title-max 55 --desc-max 155
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
				return emitDryRun(cmd, flags, "would audit page SEO metadata from the local mirror")
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("seo audit has no live equivalent; it reads the local mirror. Run 'webflow-pp-cli sync --resources sites' then retry"))
			}

			lq, cleanup, ok, err := openLocalMirror(cmd, flags, dbPath, "sites")
			if err != nil {
				return err
			}
			defer cleanup()
			if !ok {
				return nil
			}
			if !lq.hasTable("sites_pages") {
				return emitEmptyLocal(cmd, flags, dbPath, "sites", seoAuditView{
					Findings: make([]seoFinding, 0),
					Counts:   map[string]int{},
					Note:     missingTableNote(dbPath, "page"),
				})
			}
			if !hintIfUnsynced(cmd, lq.db, "sites_pages") {
				hintIfStale(cmd, lq.db, "sites_pages", flags.maxAge)
			}

			siteID := resolveSiteID(args)
			query := `SELECT "id", "sites_id", "data" FROM "sites_pages"`
			var qargs []any
			if siteID != "" {
				query += ` WHERE "sites_id" = ?`
				qargs = append(qargs, siteID)
			}
			rows, err := lq.selectRaw(query, qargs...)
			if err != nil {
				return err
			}
			pages := decodeRows[wfPage](rows)

			view := auditPages(pages, siteID, seoThresholds{
				titleMax:      titleMax,
				descMax:       descMax,
				descMin:       descMin,
				includeDrafts: includeDrafts,
			})
			view.PagesUndecodable = len(rows) - len(pages)
			if len(pages) == 0 {
				view.Note = "no pages available for this site: the local mirror has none and no credential was usable. Run 'webflow-pp-cli sync --resources sites', or check the site id."
			}

			return emitLocalResult(cmd, flags, view, func() {
				if len(view.Findings) == 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "no SEO findings across %d pages\n", view.PagesAudited)
					return
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%d findings across %d pages\n\n", len(view.Findings), view.PagesAudited)
				for _, f := range view.Findings {
					fmt.Fprintf(cmd.OutOrStdout(), "%-8s %-28s %s\n", f.Severity, f.PageSlug, f.Detail)
				}
			})
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Local mirror path (defaults to the standard data directory)")
	cmd.Flags().IntVar(&titleMax, "title-max", defaultSEOTitleMax, "Characters before an SEO title is flagged as over-length")
	cmd.Flags().IntVar(&descMax, "desc-max", defaultSEODescMax, "Characters before an SEO description is flagged as over-length")
	cmd.Flags().IntVar(&descMin, "desc-min", defaultSEODescMin, "Characters below which an SEO description is flagged as thin")
	cmd.Flags().BoolVar(&includeDrafts, "include-drafts", false, "Audit draft and archived pages too (skipped by default)")
	return cmd
}

type seoThresholds struct {
	titleMax      int
	descMax       int
	descMin       int
	includeDrafts bool
}

// auditPages holds the whole rule set, separated from the store so it is
// testable without a database.
func auditPages(pages []wfPage, siteID string, th seoThresholds) seoAuditView {
	view := seoAuditView{
		SiteID:   siteID,
		Findings: make([]seoFinding, 0, 16),
		Counts:   map[string]int{},
	}

	titleSeen := map[string][]string{}
	descSeen := map[string][]string{}
	slugSeen := map[string][]string{}

	add := func(p wfPage, issue, severity, detail string) {
		view.Findings = append(view.Findings, seoFinding{
			PageID:   p.ID,
			PageSlug: p.label(),
			Title:    p.Title,
			Issue:    issue,
			Severity: severity,
			Detail:   detail,
		})
		view.Counts[severity]++
	}

	for _, p := range pages {
		if !th.includeDrafts && (p.isDraft() || p.isArchived()) {
			continue
		}
		view.PagesAudited++

		title := p.seoTitle()
		desc := p.seoDescription()

		switch {
		case title == "":
			add(p, "missing-seo-title", "error", "no SEO title set")
		case th.titleMax > 0 && utf8.RuneCountInString(title) > th.titleMax:
			add(p, "long-seo-title", "warning",
				fmt.Sprintf("SEO title is %d characters (limit %d)", utf8.RuneCountInString(title), th.titleMax))
		}

		switch {
		case desc == "":
			add(p, "missing-seo-description", "error", "no SEO description set")
		case th.descMax > 0 && utf8.RuneCountInString(desc) > th.descMax:
			add(p, "long-seo-description", "warning",
				fmt.Sprintf("SEO description is %d characters (limit %d)", utf8.RuneCountInString(desc), th.descMax))
		case th.descMin > 0 && utf8.RuneCountInString(desc) < th.descMin:
			add(p, "thin-seo-description", "info",
				fmt.Sprintf("SEO description is only %d characters (under %d)", utf8.RuneCountInString(desc), th.descMin))
		}

		if p.ogTitle() == "" && title == "" {
			add(p, "missing-open-graph-title", "warning", "no Open Graph title and no SEO title to inherit from")
		}
		if p.ogDescription() == "" && desc == "" {
			add(p, "missing-open-graph-description", "warning", "no Open Graph description and no SEO description to inherit from")
		}

		if title != "" {
			k := strings.ToLower(title)
			titleSeen[k] = append(titleSeen[k], p.label()+"\u0000"+p.ID)
		}
		if desc != "" {
			k := strings.ToLower(desc)
			descSeen[k] = append(descSeen[k], p.label()+"\u0000"+p.ID)
		}
		// Webflow slugs are unique per folder, not per site, so key on the
		// page's public path. Keying on the bare slug flags /blog/index and
		// /docs/index as duplicates of each other.
		pubPath := p.PublishedPath
		if strings.TrimSpace(pubPath) == "" {
			pubPath = p.Slug
		}
		if s := normalizePath(pubPath); s != "" && s != "/" {
			slugSeen[s] = append(slugSeen[s], p.label()+"\u0000"+p.ID)
		}
	}

	// Duplicate detection is the self-join the API cannot do: it needs every
	// page of the site in one place, which only the local mirror provides.
	addDupes := func(seen map[string][]string, issue, noun string) {
		// Entries may carry a NUL-separated page id so duplicate findings can
		// name which page each one is, rather than repeating the same label.
		keys := make([]string, 0, len(seen))
		for k, v := range seen {
			if len(v) > 1 {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			labels := append([]string(nil), seen[k]...)
			sort.Strings(labels)
			display := make([]string, 0, len(labels))
			for _, l := range labels {
				name, _, _ := strings.Cut(l, "\u0000")
				display = append(display, name)
			}
			for _, label := range labels {
				name, id, _ := strings.Cut(label, "\u0000")
				view.Findings = append(view.Findings, seoFinding{
					PageID:   id,
					PageSlug: name,
					Issue:    issue,
					Severity: "error",
					Detail: fmt.Sprintf("%s shared with %d other page(s): %s",
						noun, len(labels)-1, strings.Join(display, ", ")),
				})
				view.Counts["error"]++
			}
		}
	}
	addDupes(titleSeen, "duplicate-seo-title", "SEO title")
	addDupes(descSeen, "duplicate-seo-description", "SEO description")
	addDupes(slugSeen, "duplicate-slug", "slug")

	sort.SliceStable(view.Findings, func(i, j int) bool {
		a, b := view.Findings[i], view.Findings[j]
		if seoSeverityRank[a.Severity] != seoSeverityRank[b.Severity] {
			return seoSeverityRank[a.Severity] < seoSeverityRank[b.Severity]
		}
		if a.PageSlug != b.PageSlug {
			return a.PageSlug < b.PageSlug
		}
		return a.Issue < b.Issue
	})
	return view
}
