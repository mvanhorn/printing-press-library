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

type redirectFinding struct {
	RedirectID string `json:"redirectId,omitempty"`
	FromURL    string `json:"fromUrl"`
	ToURL      string `json:"toUrl,omitempty"`
	Issue      string `json:"issue"`
	Severity   string `json:"severity"`
	Detail     string `json:"detail"`
}

type redirectsAuditView struct {
	SiteID         string            `json:"siteId,omitempty"`
	RulesAudited   int               `json:"rulesAudited"`
	KnownPaths     int               `json:"knownPaths"`
	Findings       []redirectFinding `json:"findings"`
	Counts         map[string]int    `json:"countsBySeverity"`
	TargetsChecked bool              `json:"targetsChecked"`
	Source         string            `json:"source,omitempty"`
	Note           string            `json:"note,omitempty"`
}

func newNovelRedirectsAuditCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "audit [site-id]",
		Short: "Validate a site's redirect table against its known page and item slugs",
		Long: strings.Trim(`
Use this command to validate a site's redirect table against its known page and
CMS item slugs.

Do NOT use this command to create, edit, or list redirects; use the generated
'sites redirects' commands instead. Do NOT use it for page SEO metadata
problems; use 'seo audit' instead.

Checks four things: a redirect whose source shadows a real page, a redirect
whose target matches no known page or item, a redirect that points at itself or
forms a loop, and two rules sharing the same source.

Redirects are fetched from the API (generated sync cannot mirror them). Page
slugs come from the local mirror when present, so run
'webflow-pp-cli sync --resources sites' first to enable the target and
shadowed-page checks.
`, "\n"),
		Example: strings.Trim(`
  webflow-pp-cli redirects audit 580e63e98c9a982ac9b8b741 --agent
  webflow-pp-cli redirects audit 580e63e98c9a982ac9b8b741 --json --select findings.fromUrl,findings.issue
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
				return emitDryRun(cmd, flags, "would validate the redirect table against known page and item slugs")
			}

			lq, cleanup, ok, err := openLocalMirror(cmd, flags, dbPath, "sites,redirects,items")
			if err != nil {
				return err
			}
			defer cleanup()
			if !ok {
				return nil
			}
			siteID := resolveSiteID(args)

			var redirectRows []rawRow
			source := "local-mirror"
			if lq.hasTable("redirects") {
				redirQ := `SELECT "id", "sites_id", "data" FROM "redirects"`
				var redirArgs []any
				if siteID != "" {
					redirQ += ` WHERE "sites_id" = ?`
					redirArgs = append(redirArgs, siteID)
				}
				if redirectRows, err = lq.selectRaw(redirQ, redirArgs...); err != nil {
					return err
				}
			}
			// Generated sync cannot reach `redirects` in this Press version
			// (verified: `sync --resources redirects` fails), so the mirror is
			// empty in practice. One GET per site is cheap; fetch it.
			if len(redirectRows) == 0 && flags.dataSource != "local" {
				if siteID == "" {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("a site id is required when the local mirror has no redirects"))
				}
				c, cerr := flags.newClient()
				if cerr != nil {
					return cerr
				}
				if redirectRows, err = lq.fetchOneList(c,
					"/sites/"+escapeSeg(siteID)+"/redirects", "redirects", siteID); err != nil {
					if note, degrade := liveFallbackErr(err, "this site's redirects"); degrade {
						return emitEmptyLocal(cmd, flags, dbPath, "sites", redirectsAuditView{
							SiteID: siteID, Findings: make([]redirectFinding, 0),
							Counts: map[string]int{}, Source: "unavailable", Note: note,
						})
					}
					return fmt.Errorf("fetching site redirects: %w", err)
				}
				source = "api"
			}

			var pageRows, itemRows []rawRow
			if lq.hasTable("sites_pages") {
				pq := `SELECT "id", "sites_id", "data" FROM "sites_pages"`
				var pargs []any
				if siteID != "" {
					pq += ` WHERE "sites_id" = ?`
					pargs = append(pargs, siteID)
				}
				if pageRows, err = lq.selectRaw(pq, pargs...); err != nil {
					return err
				}
			}
			if lq.hasTable("items") {
				if itemRows, err = lq.selectRaw(`SELECT "id", "collections_id", "data" FROM "items"`); err != nil {
					return err
				}
			}

			view := auditRedirects(redirectRows, pageRows, itemRows, siteID)
			view.Source = source

			return emitLocalResult(cmd, flags, view, func() {
				if len(view.Findings) == 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "no redirect findings across %d rules\n", view.RulesAudited)
					return
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%d findings across %d redirect rules\n\n",
					len(view.Findings), view.RulesAudited)
				for _, f := range view.Findings {
					fmt.Fprintf(cmd.OutOrStdout(), "%-8s %-32s %s\n", f.Severity, f.FromURL, f.Detail)
				}
			})
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Local mirror path (defaults to the standard data directory)")
	return cmd
}

// auditRedirects cross-checks the redirect table against the site's real
// paths. Split out so it is testable without a store.
func auditRedirects(redirectRows, pageRows, itemRows []rawRow, siteID string) redirectsAuditView {
	view := redirectsAuditView{
		SiteID:   siteID,
		Findings: make([]redirectFinding, 0, 8),
		Counts:   map[string]int{},
	}

	// Known paths: every live page slug, plus every CMS item slug. Item slugs
	// are matched suffix-wise because an item's public URL is
	// /<collection-slug>/<item-slug> and the collection prefix is not on the
	// item record.
	pagePaths := map[string]bool{}
	for _, p := range decodeRows[wfPage](pageRows) {
		if p.isArchived() {
			continue
		}
		// Register the page's real public path. Registering the bare slug too
		// would make a page published at /company/about also claim /about, so
		// the canonical post-restructure redirect /about -> /company/about
		// would be flagged as shadowing a live page.
		candidate := p.PublishedPath
		if strings.TrimSpace(candidate) == "" {
			candidate = p.Slug
		}
		if n := normalizePath(candidate); n != "" {
			pagePaths[n] = true
		}
	}
	itemSlugs := map[string]bool{}
	for _, it := range decodeRows[wfItem](itemRows) {
		if it.archived() {
			continue
		}
		if s := normalizePath(it.slug()); s != "" {
			itemSlugs[s] = true
		}
	}
	view.KnownPaths = len(pagePaths) + len(itemSlugs) // distinct known paths; a path in both sets is rare and counted once per set
	// Only claim a target is unknown when we actually have a path universe to
	// check against. Without synced pages, every target would look broken.
	view.TargetsChecked = len(pagePaths) > 0
	if !view.TargetsChecked {
		view.Note = "pages were not synced, so redirect targets were not checked; run 'webflow-pp-cli sync --resources sites' to enable that check"
	}

	add := func(r wfRedirect, issue, severity, detail string) {
		view.Findings = append(view.Findings, redirectFinding{
			RedirectID: r.ID, FromURL: r.FromURL, ToURL: r.ToURL,
			Issue: issue, Severity: severity, Detail: detail,
		})
		view.Counts[severity]++
	}

	knownTarget := func(p string) bool {
		if pagePaths[p] {
			return true
		}
		if itemSlugs[p] {
			return true
		}
		// /<collection>/<item-slug> shape: match on the trailing segment.
		if i := strings.LastIndex(p, "/"); i > 0 {
			if itemSlugs[p[i:]] {
				return true
			}
		}
		return false
	}

	seenFrom := map[string][]string{}
	redirects := decodeRows[wfRedirect](redirectRows)
	for _, r := range redirects {
		view.RulesAudited++
		from := normalizePath(r.FromURL)
		to := normalizePath(r.ToURL)

		if from == "" {
			add(r, "empty-source", "error", "redirect has no source path")
			continue
		}
		seenFrom[from] = append(seenFrom[from], r.ID)

		if to == "" {
			add(r, "empty-target", "error", "redirect has no target path")
			continue
		}
		if from == to {
			add(r, "self-redirect", "error", "source and target are the same path")
			continue
		}
		if pagePaths[from] {
			add(r, "shadows-live-page", "error",
				fmt.Sprintf("a live page already serves %s, so this rule may never fire or may hide the page", from))
		}
		// Only flag unknown targets when the target is same-origin. External
		// targets are legitimate and unverifiable from the mirror.
		if view.TargetsChecked && !strings.Contains(r.ToURL, "://") && !knownTarget(to) {
			add(r, "unknown-target", "warning",
				fmt.Sprintf("target %s matches no known page or CMS item slug", to))
		}
	}

	// Loop detection over the normalized rule graph.
	next := map[string]string{}
	for _, r := range redirects {
		from, to := normalizePath(r.FromURL), normalizePath(r.ToURL)
		if from != "" && to != "" && !strings.Contains(r.ToURL, "://") {
			next[from] = to
		}
	}
	for _, r := range redirects {
		from := normalizePath(r.FromURL)
		if from == "" {
			continue
		}
		seen := map[string]bool{from: true}
		cur, steps := from, 0
		for steps < 16 {
			n, ok := next[cur]
			if !ok {
				break
			}
			if seen[n] {
				add(r, "redirect-loop", "error",
					fmt.Sprintf("following %s leads back to a path already in the chain (%s)", from, n))
				break
			}
			seen[n] = true
			cur = n
			steps++
		}
	}

	dupKeys := make([]string, 0, len(seenFrom))
	for k, ids := range seenFrom {
		if len(ids) > 1 {
			dupKeys = append(dupKeys, k)
		}
	}
	sort.Strings(dupKeys)
	for _, k := range dupKeys {
		view.Findings = append(view.Findings, redirectFinding{
			FromURL:  k,
			Issue:    "duplicate-source",
			Severity: "error",
			Detail:   fmt.Sprintf("%d rules share the source %s; only one can win", len(seenFrom[k]), k),
		})
		view.Counts["error"]++
	}

	sort.SliceStable(view.Findings, func(i, j int) bool {
		a, b := view.Findings[i], view.Findings[j]
		if seoSeverityRank[a.Severity] != seoSeverityRank[b.Severity] {
			return seoSeverityRank[a.Severity] < seoSeverityRank[b.Severity]
		}
		if a.FromURL != b.FromURL {
			return a.FromURL < b.FromURL
		}
		return a.Issue < b.Issue
	})
	return view
}
