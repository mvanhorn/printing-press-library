// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Shared reader for the support.qsys.com corpus (qsys_support), used by
// `fault`, `bom risks`, and `qds`.
//
// The vendor's only classification is the sitemap category, so every filter
// here keys off it rather than guessing from the article text.

// riskCategories are the categories that carry deployment risk for an
// equipment list. faq/application-notes/tips are deliberately excluded: they
// are how-to content, and folding them in would bury the four categories that
// actually describe something going wrong.
var riskCategories = []string{"known-issues", "awareness", "troubleshooting", "errorstatus-messages"}

// faultCategories are the categories whose articles are titled with the
// literal string Q-SYS Designer displays.
var faultCategories = []string{"errorstatus-messages", "troubleshooting"}

// releaseCategories are the categories that describe a Designer release rather
// than a device.
var releaseCategories = []string{"known-issues", "awareness"}

// supportRow is one stored knowledge-base article.
type supportRow struct {
	URL      string
	Category string
	Slug     string
	Title    string
	Body     string
}

// supportRef is the article shape returned to callers. Models and
// DesignerVersions are the locally-derived joins, not vendor metadata.
type supportRef struct {
	Title            string   `json:"title"`
	Category         string   `json:"category"`
	URL              string   `json:"url"`
	Models           []string `json:"models,omitempty"`
	DesignerVersions []string `json:"designer_versions,omitempty"`
	Excerpt          string   `json:"excerpt,omitempty"`
}

// loadSupportArticles reads every article in the given categories into memory.
//
// Drain-first by construction: the store holds a single SQLite connection, so
// the parent rows are fully scanned and closed before any caller runs the
// follow-up product queries these results get joined against. Category counts
// are small (known-issues 16, errorstatus-messages 38, troubleshooting 122,
// awareness 128), so the whole working set is a few hundred rows at most.
func loadSupportArticles(ctx context.Context, db *sql.DB, categories []string) ([]supportRow, error) {
	out := make([]supportRow, 0, 256)
	if len(categories) == 0 {
		return out, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(categories)), ",")
	args := make([]any, 0, len(categories))
	for _, c := range categories {
		args = append(args, c)
	}
	rows, err := db.QueryContext(ctx, `
		SELECT url,
		       COALESCE(category, ''),
		       COALESCE(slug, ''),
		       COALESCE(title, ''),
		       COALESCE(body, '')
		FROM qsys_support WHERE category IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("reading support articles: %w", err)
	}
	for rows.Next() {
		var r supportRow
		if err := rows.Scan(&r.URL, &r.Category, &r.Slug, &r.Title, &r.Body); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning support article: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterating support articles: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return out, nil
}

// supportHarvested reports how many support articles are stored. Zero means
// the third source was never harvested, which is a different answer from "no
// article matched" and every caller reports it differently.
func supportHarvested(ctx context.Context, db *sql.DB) (int, error) {
	n, err := countRows(ctx, db, `SELECT COUNT(*) FROM qsys_support`)
	if err != nil {
		return 0, fmt.Errorf("counting support articles: %w", err)
	}
	return n, nil
}

const supportHarvestHint = "no support articles in the local corpus; run `qsys-pp-cli harvest --only support`"

// ---------- fault-string normalization ----------

// normalizeFaultString folds a fault or status string into the slug shape QSC
// publishes its articles under.
//
// This is the whole reason `fault` works. Designer displays
//
//	Fault - LAN A Streaming Error - Not Connected
//
// and the vendor files that article at
//
//	/en_US/errorstatus-messages/error-fault-lan-a-streaming-error-not-connected
//
// Case, spacing, and every separator differ, so a full-text query on the raw
// string matches the wrong article or nothing at all. Folding both sides to
// lowercase alphanumerics joined by single hyphens makes them comparable.
func normalizeFaultString(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevDash := false
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if !prevDash && b.Len() > 0 {
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// faultNoisePrefixes are the classification words QSC prepends to error and
// status article slugs and titles. The user pastes what Designer showed them,
// which may carry none, one, or both of these, so they are stripped from both
// sides rather than being pattern-matched on one.
var faultNoisePrefixes = []string{"error-", "fault-", "status-", "warning-", "notice-"}

// faultKey is normalizeFaultString plus repeated noise-prefix removal, so
// "Fault - LAN A Streaming Error - Not Connected",
// "LAN A Streaming Error - Not Connected", and the article slug
// "error-fault-lan-a-streaming-error-not-connected" all reduce to the same key.
func faultKey(s string) string {
	key := normalizeFaultString(s)
	for {
		stripped := false
		for _, p := range faultNoisePrefixes {
			if rest, ok := strings.CutPrefix(key, p); ok && rest != "" {
				key, stripped = rest, true
				break
			}
		}
		if !stripped {
			return key
		}
	}
}

const (
	// faultMinQueryKey rejects a query too short to identify anything. Without
	// it a two-character query substring-matches most of the knowledge base.
	faultMinQueryKey = 4
	// faultMinArticleKey guards the reverse-containment rule, where the
	// article key is found inside a longer pasted string.
	faultMinArticleKey = 8
)

// faultRank scores one article against an already-keyed query. Returns -1 for
// no match; matching is deliberately strict so a nonsense string comes back
// empty rather than dragging in an unrelated article.
func faultRank(queryKey, slug, title string) int {
	if len(queryKey) < faultMinQueryKey {
		return -1
	}
	slugKey, titleKey := faultKey(slug), faultKey(title)
	switch {
	case queryKey == slugKey || queryKey == titleKey:
		return 300
	case slugKey != "" && strings.Contains(slugKey, queryKey):
		return 200
	case titleKey != "" && strings.Contains(titleKey, queryKey):
		return 190
	case len(slugKey) >= faultMinArticleKey && strings.Contains(queryKey, slugKey):
		return 120
	case len(titleKey) >= faultMinArticleKey && strings.Contains(queryKey, titleKey):
		return 110
	}
	return -1
}

// ---------- model mentions ----------

// mentionsModel reports whether lowerText names model as a standalone token.
//
// A bare substring test is wrong here: "NC-90" would hit inside "NC-900", and
// short series names would match half the knowledge base. Requiring a
// non-alphanumeric character on both sides keeps "CX-Q 8K8" and "CX-Q-8K8" as
// hits while rejecting "CX-Qx". This is a relevance heuristic; an empty result
// means "nothing matched", never "nothing exists".
func mentionsModel(lowerText, model string) bool {
	needle := strings.ToLower(strings.TrimSpace(model))
	if len(needle) < 3 || lowerText == "" {
		return false
	}
	for i := 0; i+len(needle) <= len(lowerText); {
		j := strings.Index(lowerText[i:], needle)
		if j < 0 {
			return false
		}
		start := i + j
		end := start + len(needle)
		if !alnumByteAt(lowerText, start-1) && !alnumByteAt(lowerText, end) {
			return true
		}
		i = start + 1
	}
	return false
}

func alnumByteAt(s string, i int) bool {
	if i < 0 || i >= len(s) {
		return false
	}
	c := s[i]
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// ---------- Designer release relevance ----------

// designerVersionRE finds Designer versions an article names. It requires the
// product word before the number: support articles are full of firmware
// revisions, port numbers, and model digits, and a bare \d+\.\d+ pattern
// classifies most of them as release notes.
var designerVersionRE = regexp.MustCompile(`(?i)\b(?:q-?sys\s+)?(?:designer(?:\s+software)?|qds)\s*(?:version\s*)?v?(\d+\.\d+(?:\.\d+)?)`)

// articleVersions returns the Designer versions an article names, deduped and
// version-sorted. An empty result means the article named none, which is
// treated as "applies to any release" rather than "applies to none".
func articleVersions(text string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 4)
	for _, m := range designerVersionRE.FindAllStringSubmatch(text, -1) {
		v := m[1]
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return versionLess(out[i], out[j]) })
	return out
}

// versionSeriesMatch reports whether two Designer versions name the same
// release line: 10.0 matches 10.0.2, and 10.0.2 matches 10.0, but 10.0 does
// not match 10.1.
func versionSeriesMatch(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return a == b || strings.HasPrefix(a, b+".") || strings.HasPrefix(b, a+".")
}

// versionRelevant reports whether an article naming `have` applies to target.
// Articles that name no version are kept: most troubleshooting content is
// release-agnostic, and dropping it would hide the majority of real risk.
func versionRelevant(target string, have []string) bool {
	if strings.TrimSpace(target) == "" || len(have) == 0 {
		return true
	}
	for _, v := range have {
		if versionSeriesMatch(target, v) {
			return true
		}
	}
	return false
}

// ---------- shared formatting ----------

// excerpt returns a bounded slice of article text for agent payloads. Support
// article bodies run to thousands of characters and returning them whole burns
// context for no benefit when the caller wanted a URL and a title.
func excerpt(s string, n int) string {
	s = strings.TrimSpace(s)
	if n <= 0 || len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n]) + "…"
}

// printSupportRefs renders a human table of articles.
func printSupportRefs(w interface{ Write([]byte) (int, error) }, label string, refs []supportRef) {
	if len(refs) == 0 {
		return
	}
	fmt.Fprintf(w, "\n%s\n", label)
	for _, r := range refs {
		fmt.Fprintf(w, "  %-52s %s\n", trimTo(r.Title, 52), r.URL)
		if len(r.Models) > 0 {
			fmt.Fprintf(w, "  %-52s models: %s\n", "", strings.Join(r.Models, ", "))
		}
	}
}
