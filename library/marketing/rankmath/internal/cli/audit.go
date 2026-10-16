// Copyright 2026 ElizabethcStone and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/seohead"
	"github.com/spf13/cobra"
)

type auditItem struct {
	URL    string `json:"url"`
	PostID string `json:"post_id"`
}

type auditIssue struct {
	URL         string   `json:"url"`
	PostID      string   `json:"post_id"`
	Type        string   `json:"type"`
	Issue       string   `json:"issue"`
	Detail      string   `json:"detail"`
	DuplicateOf []string `json:"duplicate_of"`
}

type auditFailure struct {
	URL   string `json:"url"`
	Error string `json:"error"`
}

type auditResult struct {
	SiteItems     int            `json:"site_items"`
	ScannedURLs   int            `json:"scanned_urls"`
	Fetched       int            `json:"fetched"`
	UncheckedURLs int            `json:"unchecked_urls"`
	Issues        []auditIssue   `json:"issues"`
	Summary       map[string]int `json:"summary"`
	FetchFailures []auditFailure `json:"fetch_failures"`
}

var auditChecks = []string{"missing_title", "missing_description", "duplicate_title", "duplicate_description", "long_title", "long_description", "missing_canonical", "canonical_mismatch", "noindex", "missing_schema", "http_error", "changed"}

func newNovelAuditCmd(flags *rootFlags) *cobra.Command {
	var dbPath, checkCSV, issueFilter string
	var maxFetch, concurrency, limit int
	var rps float64
	var noFetch bool
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Audit on-page SEO tags across synced posts and pages",
		Long:  "Use this command to audit on-page SEO tags across all synced posts. Do NOT use this command for internal-link audits; use 'link-genius' audit commands instead. Do NOT use it for Google index status; use 'coverage' instead.",
		Example: "  rankmath-pp-cli sync --resources posts,pages\n" +
			"  rankmath-pp-cli audit --agent --select issues.url,issues.issue",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "--max-fetch=3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "audit")
			}
			checks, err := parseAuditChecks(checkCSV)
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cmd.SetContext(ctx)
			db, found, err := openMirrorOrHint(cmd, flags, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			if !found {
				return printJSONFiltered(cmd.OutOrStdout(), emptyAuditResult(), flags)
			}
			defer db.Close()
			if err := db.EnsureSEOSnapshots(ctx); err != nil {
				return err
			}
			inventory, err := loadPublishedInventory(db)
			if err != nil {
				return err
			}
			items := make([]auditItem, 0, len(inventory))
			for _, item := range inventory {
				items = append(items, auditItem{URL: item.URL, PostID: strconv.Itoa(item.PostID)})
			}
			latestRows, err := db.LatestSEOSnapshots(ctx)
			if err != nil {
				return err
			}
			latest := make(map[string]seohead.Head, len(latestRows))
			for _, h := range latestRows {
				latest[h.URL] = h
			}
			fetchLimit := scanCap(maxFetch, 5)
			result := emptyAuditResult()
			result.SiteItems = len(items)
			headMap := make(map[string]seohead.Head, len(items))
			prev := make(map[string]seohead.Head)
			previous := make(map[string]seohead.Head)
			if checks["changed"] {
				previous, err = db.PreviousSEOSnapshots(ctx)
				if err != nil {
					return err
				}
			}
			toFetch := make([]auditItem, 0)
			for _, item := range items {
				if !noFetch && len(toFetch) < fetchLimit {
					toFetch = append(toFetch, item)
					continue
				}
				if h, ok := latest[item.URL]; ok {
					headMap[item.URL] = h
				} else {
					result.UncheckedURLs++
				}
			}
			fetchedSnapshots := make([]seohead.Head, 0)
			if len(toFetch) > 0 {
				fetched, failures := fetchAuditItems(ctx, toFetch, concurrency, rps)
				result.Fetched = len(fetched)
				result.FetchFailures = failures
				for _, item := range toFetch {
					if h, ok := fetched[item.URL]; ok {
						headMap[item.URL] = h
						if h.Status < 400 {
							fetchedSnapshots = append(fetchedSnapshots, h)
						}
					} else if h, ok := latest[item.URL]; ok {
						headMap[item.URL] = h
					} else {
						result.UncheckedURLs++
					}
				}
				if len(failures) > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d fetches failed\n", len(failures), len(toFetch))
				}
			}
			if checks["changed"] {
				refetched := make(map[string]bool, len(fetchedSnapshots))
				for _, h := range fetchedSnapshots {
					refetched[h.URL] = true
				}
				for _, item := range items {
					if refetched[item.URL] {
						if old, ok := latest[item.URL]; ok {
							prev[item.URL] = old
						}
					} else if old, ok := previous[item.URL]; ok {
						prev[item.URL] = old
					}
				}
			}
			if err := db.InsertSEOSnapshots(ctx, fetchedSnapshots); err != nil {
				return fmt.Errorf("saving SEO snapshots: %w", err)
			}
			result.ScannedURLs = len(headMap)
			result.Issues = analyzeSEO(items, headMap, prev, checks)
			result.Summary = summarizeAudit(result.Issues)
			filtered := make([]auditIssue, 0, len(result.Issues))
			for _, v := range result.Issues {
				if issueFilter == "" || v.Issue == issueFilter {
					filtered = append(filtered, v)
				}
			}
			if limit > 0 && len(filtered) > limit {
				filtered = filtered[:limit]
			}
			result.Issues = filtered
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				return renderAuditHuman(cmd.OutOrStdout(), result)
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	cmd.Flags().StringVar(&checkCSV, "check", strings.Join(auditChecks, ","), "Comma-separated SEO checks")
	cmd.Flags().StringVar(&issueFilter, "issue", "", "Return only one issue type")
	cmd.Flags().IntVar(&maxFetch, "max-fetch", 200, "Maximum number of pages to fetch")
	cmd.Flags().IntVar(&concurrency, "concurrency", 4, "Maximum concurrent page fetches")
	cmd.Flags().Float64Var(&rps, "rps", 2, "Maximum page fetches per second")
	cmd.Flags().BoolVar(&noFetch, "no-fetch", false, "Use only the latest stored SEO snapshots")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum issues to return; 0 returns all")
	return cmd
}

func renderAuditHuman(out io.Writer, result auditResult) error {
	if _, err := fmt.Fprintf(out, "Checked %d of %d URLs (%d fetched; %d unchecked", result.ScannedURLs, result.SiteItems, result.Fetched, result.UncheckedURLs); err != nil {
		return err
	}
	if result.UncheckedURLs > 0 {
		if _, err := fmt.Fprint(out, "; raise --max-fetch"); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(out, ")"); err != nil {
		return err
	}
	if len(result.Issues) == 0 {
		message := "No on-page SEO issues found."
		if result.UncheckedURLs > 0 {
			message = "No on-page SEO issues found among checked URLs."
		}
		_, err := fmt.Fprintln(out, message)
		return err
	}
	if _, err := fmt.Fprintln(out, "URL\tPost ID\tIssue\tDetail"); err != nil {
		return err
	}
	for _, v := range result.Issues {
		if _, err := fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", v.URL, v.PostID, v.Issue, v.Detail); err != nil {
			return err
		}
	}
	return nil
}

func emptyAuditResult() auditResult {
	return auditResult{Issues: make([]auditIssue, 0), Summary: map[string]int{}, FetchFailures: make([]auditFailure, 0)}
}

func parseAuditChecks(csv string) (map[string]bool, error) {
	allowed := make(map[string]struct{}, len(auditChecks))
	for _, name := range auditChecks {
		allowed[name] = struct{}{}
	}
	checks := make(map[string]bool)
	for _, name := range cliutil.SplitCSV(csv) {
		if name == "" {
			continue
		}
		if _, ok := allowed[name]; !ok {
			return nil, fmt.Errorf("unknown check %q (valid: %s)", name, strings.Join(auditChecks, ","))
		}
		checks[name] = true
	}
	return checks, nil
}

type auditFetchResult struct {
	item auditItem
	head seohead.Head
	err  error
}

func fetchAuditItems(ctx context.Context, items []auditItem, concurrency int, rps float64) (map[string]seohead.Head, []auditFailure) {
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > len(items) {
		concurrency = len(items)
	}
	fetcher := seohead.New(20*time.Second, rps)
	jobs := make(chan auditItem)
	results := make(chan auditFetchResult, len(items))
	var wg sync.WaitGroup
	for n := 0; n < concurrency; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				h, err := fetcher.Fetch(ctx, item.URL)
				results <- auditFetchResult{item: item, head: h, err: err}
			}
		}()
	}
	go func() {
		for _, item := range items {
			jobs <- item
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()
	heads := make(map[string]seohead.Head, len(items))
	failures := make([]auditFailure, 0)
	for v := range results {
		if v.err == nil || isGone(v.head.Status) {
			heads[v.item.URL] = v.head
		}
		if v.err != nil && !isGone(v.head.Status) {
			failures = append(failures, auditFailure{URL: v.item.URL, Error: v.err.Error()})
		}
	}
	sort.Slice(failures, func(i, j int) bool { return failures[i].URL < failures[j].URL })
	return heads, failures
}

func analyzeSEO(items []auditItem, heads map[string]seohead.Head, prev map[string]seohead.Head, checks map[string]bool) []auditIssue {
	issues := make([]auditIssue, 0)
	add := func(item auditItem, name, detail string, dup []string) {
		if dup == nil {
			dup = make([]string, 0)
		}
		issues = append(issues, auditIssue{URL: item.URL, PostID: item.PostID, Type: "warning", Issue: name, Detail: detail, DuplicateOf: dup})
	}
	duplicates := map[string]map[string][]string{"duplicate_title": {}, "duplicate_description": {}}
	for _, item := range items {
		h, ok := heads[item.URL]
		if !ok {
			continue
		}
		if isGone(h.Status) {
			continue
		}
		for name, value := range map[string]string{"duplicate_title": h.Title, "duplicate_description": h.Description} {
			key := strings.ToLower(strings.Join(strings.Fields(value), " "))
			if key != "" {
				duplicates[name][key] = append(duplicates[name][key], item.URL)
			}
		}
	}
	for _, item := range items {
		h, ok := heads[item.URL]
		if !ok {
			continue
		}
		if h.Status >= 400 {
			if isGone(h.Status) {
				if checks["http_error"] {
					add(item, "http_error", fmt.Sprintf("HTTP status %d", h.Status), nil)
				}
			}
			continue
		}
		if checks["missing_title"] && strings.TrimSpace(h.Title) == "" {
			add(item, "missing_title", "Title is missing", nil)
		}
		if checks["missing_description"] && strings.TrimSpace(h.Description) == "" {
			add(item, "missing_description", "Meta description is missing", nil)
		}
		if checks["long_title"] && len([]rune(h.Title)) > 60 {
			add(item, "long_title", fmt.Sprintf("Title is %d characters (max 60)", len([]rune(h.Title))), nil)
		}
		if checks["long_description"] && len([]rune(h.Description)) > 160 {
			add(item, "long_description", fmt.Sprintf("Description is %d characters (max 160)", len([]rune(h.Description))), nil)
		}
		if checks["missing_canonical"] && strings.TrimSpace(h.Canonical) == "" {
			add(item, "missing_canonical", "Canonical URL is missing", nil)
		}
		if checks["canonical_mismatch"] && h.Canonical != "" && !samePagePath(h.Canonical, finalURL(h)) {
			add(item, "canonical_mismatch", "Canonical differs from final URL", nil)
		}
		if checks["noindex"] && h.Noindex {
			add(item, "noindex", "Page is marked noindex", nil)
		}
		if checks["missing_schema"] && len(h.SchemaTypes) == 0 {
			add(item, "missing_schema", "No JSON-LD schema types found", nil)
		}
		if old, yes := prev[item.URL]; yes {
			if checks["changed"] && (old.Title != h.Title || old.Robots != h.Robots || old.Canonical != h.Canonical) {
				add(item, "changed", fmt.Sprintf("title %q -> %q; robots %q -> %q; canonical %q -> %q", old.Title, h.Title, old.Robots, h.Robots, old.Canonical, h.Canonical), nil)
			}
		}
	}
	itemsByURL := make(map[string]auditItem, len(items))
	for _, item := range items {
		itemsByURL[item.URL] = item
	}
	for _, check := range []string{"duplicate_title", "duplicate_description"} {
		if !checks[check] {
			continue
		}
		for _, urls := range duplicates[check] {
			if len(urls) < 2 {
				continue
			}
			sort.Strings(urls)
			for _, u := range urls {
				item, ok := itemsByURL[u]
				if !ok {
					continue
				}
				others := make([]string, 0, len(urls)-1)
				for _, other := range urls {
					if other != u {
						others = append(others, other)
					}
				}
				add(item, check, fmt.Sprintf("Duplicate %s", strings.TrimPrefix(check, "duplicate_")), others)
			}
		}
	}
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Issue == issues[j].Issue {
			return issues[i].URL < issues[j].URL
		}
		return issues[i].Issue < issues[j].Issue
	})
	return issues
}

func finalURL(h seohead.Head) string {
	if h.FinalURL != "" {
		return h.FinalURL
	}
	return h.URL
}
func summarizeAudit(issues []auditIssue) map[string]int {
	out := map[string]int{}
	for _, v := range issues {
		out[v.Issue]++
	}
	return out
}

func isGone(status int) bool { return status == 404 || status == 410 }
