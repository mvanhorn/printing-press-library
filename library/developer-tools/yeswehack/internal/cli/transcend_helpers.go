// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: shared helpers and types for transcendence commands.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/yeswehack/internal/store"
)

// -----------------------------------------------------------------------------
// Types
// -----------------------------------------------------------------------------

type scopeAsset struct {
	Asset       string         `json:"asset"`
	ProgramSlug string         `json:"program_slug"`
	Raw         map[string]any `json:"raw,omitempty"`
}

// -----------------------------------------------------------------------------
// Store
// -----------------------------------------------------------------------------

func openResearchStore() (*store.Store, error) {
	return store.Open(defaultDBPath("yeswehack-pp-cli"))
}

func openDefaultStore() (*store.Store, error) {
	return openResearchStore()
}

func listResourceObjects(db *store.Store, resourceType string, limit int) ([]map[string]any, error) {
	rows, err := db.Query(`SELECT data FROM resources WHERE resource_type = ? ORDER BY updated_at DESC LIMIT ?`, resourceType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(data), &obj); err == nil {
			out = append(out, obj)
		}
	}
	return out, rows.Err()
}

func loadResourceObjects(db *store.Store, resourceType string) ([]map[string]any, error) {
	return listResourceObjects(db, resourceType, 2000)
}

func selectedPrograms(db *store.Store, slug string) ([]map[string]any, error) {
	all, err := loadResourceObjects(db, "programs")
	if err != nil {
		return nil, err
	}
	if slug == "" {
		return all, nil
	}
	out := make([]map[string]any, 0, 1)
	for _, p := range all {
		if programSlug(p) == slug {
			out = append(out, p)
		}
	}
	return out, nil
}

// -----------------------------------------------------------------------------
// Field extraction
// -----------------------------------------------------------------------------

func stringField(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		cur := any(obj)
		ok := true
		for _, part := range strings.Split(key, ".") {
			m, isMap := cur.(map[string]any)
			if !isMap {
				ok = false
				break
			}
			cur, ok = m[part]
			if !ok {
				break
			}
		}
		if ok {
			switch v := cur.(type) {
			case string:
				if v != "" {
					return v
				}
			case float64:
				return strconv.FormatFloat(v, 'f', -1, 64)
			case bool:
				return strconv.FormatBool(v)
			}
		}
	}
	return ""
}

func floatField(obj map[string]any, keys ...string) float64 {
	for _, key := range keys {
		cur := any(obj)
		ok := true
		for _, part := range strings.Split(key, ".") {
			m, isMap := cur.(map[string]any)
			if !isMap {
				ok = false
				break
			}
			cur, ok = m[part]
			if !ok {
				break
			}
		}
		if !ok {
			continue
		}
		switch v := cur.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case json.Number:
			f, _ := v.Float64()
			return f
		case string:
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return f
			}
		}
	}
	return 0
}

func arrayField(obj map[string]any, keys ...string) []any {
	for _, key := range keys {
		cur := any(obj)
		ok := true
		for _, part := range strings.Split(key, ".") {
			m, isMap := cur.(map[string]any)
			if !isMap {
				ok = false
				break
			}
			cur, ok = m[part]
			if !ok {
				break
			}
		}
		if arr, ok := cur.([]any); ok {
			return arr
		}
	}
	return nil
}

func stringAt(obj map[string]any, paths ...string) string {
	return stringField(obj, paths...)
}

func timeAt(obj map[string]any, paths ...string) time.Time {
	return parseFlexibleTime(stringField(obj, paths...))
}

// -----------------------------------------------------------------------------
// Domain extractors
// -----------------------------------------------------------------------------

func programSlug(obj map[string]any) string {
	if slug := stringField(obj, "slug", "program.slug", "data.program.slug", "program_slug"); slug != "" {
		return slug
	}
	return stringField(obj, "id")
}

func programName(obj map[string]any) string {
	if t := stringField(obj, "title", "name"); t != "" {
		return t
	}
	return programSlug(obj)
}

func rewardMax(obj map[string]any) float64 {
	return floatField(obj, "bounty_reward_max", "reward.max", "reward_max", "max_reward")
}

func extractScopesFromProgram(obj map[string]any) []scopeAsset {
	slug := programSlug(obj)
	rows := []scopeAsset{}
	for _, item := range append(arrayField(obj, "scopes"), arrayField(obj, "data.scopes", "data.in-scopes", "in-scopes")...) {
		if m, ok := item.(map[string]any); ok {
			asset := stringField(m, "asset", "value", "target", "endpoint", "url", "scope")
			if asset == "" {
				asset = fmt.Sprint(item)
			}
			rows = append(rows, scopeAsset{Asset: asset, ProgramSlug: slug, Raw: m})
		}
	}
	return rows
}

func scopesFromProgram(obj map[string]any) []scopeAsset {
	return extractScopesFromProgram(obj)
}

// PATCH(asset-in-scope-helper): wildcard-aware scope matcher used by report submission
// and any future in-scope checks. The previous strings.Contains-based check matched
// only exact substrings, so a wildcard scope like *.example.com never matched a
// concrete asset like api.example.com.
func assetInScope(asset, scope string) bool {
	a := strings.ToLower(strings.TrimSpace(asset))
	s := strings.ToLower(strings.TrimSpace(scope))
	if a == "" || s == "" {
		return false
	}
	if a == s {
		return true
	}
	if strings.HasPrefix(s, "*.") {
		bare := s[2:]
		if bare == "" {
			return false
		}
		// Wildcards cover both the apex (example.com matches *.example.com)
		// and any subdomain (api.example.com matches *.example.com).
		if a == bare {
			return true
		}
		return strings.HasSuffix(a, "."+bare)
	}
	return false
}

func extractScopesFromSnapshot(raw json.RawMessage) []scopeAsset {
	var obj map[string]any
	if json.Unmarshal(raw, &obj) != nil {
		return nil
	}
	slug := stringField(obj, "program_slug")
	scopes := arrayField(obj, "scopes", "scopes.data", "scopes.items")
	rows := []scopeAsset{}
	for _, item := range scopes {
		if m, ok := item.(map[string]any); ok {
			asset := stringField(m, "asset", "value", "target", "endpoint", "url", "scope")
			if asset == "" {
				asset = fmt.Sprint(item)
			}
			rows = append(rows, scopeAsset{Asset: asset, ProgramSlug: slug, Raw: m})
		}
	}
	return rows
}

// -----------------------------------------------------------------------------
// Hacktivity helpers
// -----------------------------------------------------------------------------

func hacktivityProgramSlug(obj map[string]any) string {
	if s := stringField(obj, "program.slug", "program_slug", "data.program.slug"); s != "" {
		return s
	}
	return programSlug(obj)
}

func hacktivityTitle(obj map[string]any) string {
	return stringField(obj, "title", "name", "report_title")
}

func bountyAmount(obj map[string]any) float64 {
	return floatField(obj, "bounty_amount", "reward.amount", "reward_amount", "bounty")
}

func severityValue(obj map[string]any) float64 {
	return floatField(obj, "cvss.base_score", "severity_score", "severity")
}

func normalizeCWE(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	if !strings.HasPrefix(s, "CWE-") {
		if _, err := fmt.Sscanf(s, "%d", new(int)); err == nil {
			s = "CWE-" + s
		}
	}
	return s
}

func cweFromReport(obj map[string]any) string {
	return normalizeCWE(stringField(obj, "vulnerable_part.code", "vulnerable_part_code", "cwe", "category.code"))
}

// -----------------------------------------------------------------------------
// Math + IO helpers
// -----------------------------------------------------------------------------

func uniqueSorted(vals map[string]bool) []string {
	out := make([]string, 0, len(vals))
	for v := range vals {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func tokenSet(s string) map[string]bool {
	re := regexp.MustCompile(`[a-z0-9]+`)
	out := map[string]bool{}
	for _, tok := range re.FindAllString(strings.ToLower(s), -1) {
		out[tok] = true
	}
	return out
}

func jaccard(a, b string) float64 {
	as, bs := tokenSet(a), tokenSet(b)
	if len(as) == 0 || len(bs) == 0 {
		return 0
	}
	inter := 0
	for k := range as {
		if bs[k] {
			inter++
		}
	}
	union := len(as) + len(bs) - inter
	return float64(inter) / float64(union)
}

func medianFloat(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	tmp := append([]float64(nil), vals...)
	sort.Float64s(tmp)
	mid := len(tmp) / 2
	if len(tmp)%2 == 0 {
		return (tmp[mid-1] + tmp[mid]) / 2
	}
	return tmp[mid]
}

func parseFlexibleTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02 15:04:05", "2006-01-02"} {
		if ts, err := time.Parse(layout, s); err == nil {
			return ts
		}
	}
	return time.Time{}
}

func sqlRowsToObjects(rows *sql.Rows) ([]map[string]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, c := range cols {
			row[c] = vals[i]
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func printNovelOutput(w io.Writer, v any, flags *rootFlags) error {
	return printJSONFiltered(w, v, flags)
}

// -----------------------------------------------------------------------------
// reportDedupeCollisions — used by report submit auto-dedupe path
// -----------------------------------------------------------------------------

func reportDedupeCollisions(db *store.Store, title, asset, cwe string, limit int) ([]map[string]any, error) {
	query := strings.TrimSpace(strings.Join([]string{title, asset, cwe}, " "))
	// Empty query has no FTS5 candidates and Jaccard-scoring 20k items
	// against an empty title is wasted I/O — return nothing.
	if query == "" {
		return nil, nil
	}
	var hits []map[string]any
	for _, source := range []struct {
		rtype string
		name  string
	}{{"hacktivity", "hacktivity"}, {"user-reports", "self"}} {
		// PATCH: FTS5-narrowed candidates instead of a 10k full-table
		// scan per source. The BM25-ordered results are then re-scored
		// with Jaccard + asset/CWE overlap for the final ranking. Falls
		// back to listResourceObjects only if FTS5 returns no candidates
		// (e.g., a brand-new sync with no rows in resources_fts yet).
		var items []map[string]any
		rawHits, err := db.SearchByType(query, source.rtype, limit*5)
		if err != nil {
			return nil, err
		}
		if len(rawHits) == 0 {
			items, err = listResourceObjects(db, source.rtype, 10000)
			if err != nil {
				return nil, err
			}
		} else {
			items = make([]map[string]any, 0, len(rawHits))
			for _, raw := range rawHits {
				var obj map[string]any
				if jerr := json.Unmarshal(raw, &obj); jerr != nil {
					continue
				}
				items = append(items, obj)
			}
		}
		for _, obj := range items {
			title2 := stringField(obj, "title", "report_title", "name", "summary")
			text := strings.Join([]string{title2, stringField(obj, "program.slug", "data.program.slug", "program"), fmt.Sprint(obj)}, " ")
			score := 0.0
			if title != "" && title2 != "" {
				score += jaccard(title, title2) * 0.5
			}
			if asset != "" && strings.Contains(strings.ToLower(text), strings.ToLower(asset)) {
				score += 0.3
			}
			if cwe != "" && normalizeCWE(cwe) == cweFromReport(obj) {
				score += 0.3
			}
			if score > 1 {
				score = 1
			}
			if score > 0 {
				hits = append(hits, map[string]any{
					"source":           source.name,
					"id":               stringField(obj, "id", "report_id", "local_id", "uuid"),
					"title":            title2,
					"program":          stringField(obj, "program.slug", "data.program.slug", "program"),
					"score":            score,
					"url_if_disclosed": stringField(obj, "url", "link", "writeup_link", "disclosed_url"),
				})
			}
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		si, _ := hits[i]["score"].(float64)
		sj, _ := hits[j]["score"].(float64)
		return si > sj
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, nil
}
