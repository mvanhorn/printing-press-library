// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored shared helpers for the Extron literature catalog commands.
// The catalog lives in the generated `resources` table under
// resource_type='literature'; each row's data is an extron.Doc JSON object.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/extron/internal/extron"
	"github.com/mvanhorn/printing-press-library/library/devices/extron/internal/store"
	"github.com/spf13/cobra"
)

const catalogResource = "literature"

// resolveCatalogDB returns the effective SQLite path for the catalog.
func resolveCatalogDB(flags *rootFlags, dbPath string) string {
	if dbPath != "" {
		return dbPath
	}
	return defaultDBPath("extron-pp-cli")
}

// openCatalogReadOnly opens the store read-only after applying the
// missing-mirror guard: when the local mirror does not exist yet, it prints
// the sync hint to stderr and returns ok=false so the caller can emit an
// empty result set instead of a SQLite open failure.
func openCatalogReadOnly(cmd *cobra.Command, dbPath string) (*store.Store, bool) {
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: extron-pp-cli catalog sync --db %s\n", dbPath, dbPath)
		return nil, false
	}
	db, err := store.OpenReadOnlyContext(context.Background(), dbPath)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "opening local catalog: %v\n", err)
		return nil, false
	}
	return db, true
}

// loadCatalogDocs loads every catalog row from the store.
func loadCatalogDocs(ctx context.Context, db *store.Store) ([]extron.Doc, error) {
	raws, err := db.List(catalogResource, 0)
	if err != nil {
		return nil, fmt.Errorf("loading local catalog: %w", err)
	}
	docs := make([]extron.Doc, 0, len(raws))
	for _, raw := range raws {
		var d extron.Doc
		if err := json.Unmarshal(raw, &d); err != nil {
			return nil, fmt.Errorf("decoding catalog row: %w", err)
		}
		docs = append(docs, d)
	}
	return docs, nil
}

// matchCategory reports whether a doc belongs to the given category filter.
// Empty filter matches everything; otherwise the filter matches the exact
// category name case-insensitively, or any category containing the filter.
func matchCategory(doc extron.Doc, filter string) bool {
	if strings.TrimSpace(filter) == "" {
		return true
	}
	f := strings.ToLower(strings.TrimSpace(filter))
	c := strings.ToLower(doc.Category)
	if c == f || strings.Contains(c, f) {
		return true
	}
	return false
}

// matchLetter reports whether a doc belongs to the alphabetical letter
// bucket. Bucket "0" covers digit/non-alpha titles; A-Z match the first
// alphabetic character of the title.
func matchLetter(doc extron.Doc, letter string) bool {
	if strings.TrimSpace(letter) == "" {
		return true
	}
	title := strings.TrimSpace(doc.Title)
	l := strings.ToUpper(strings.TrimSpace(letter))
	if title == "" {
		return l == "0"
	}
	first := title[0]
	isAlpha := (first >= 'A' && first <= 'Z') || (first >= 'a' && first <= 'z')
	if !isAlpha {
		return l == "0"
	}
	return strings.EqualFold(string(first), l)
}

// parseCatalogDate parses the site's date formats into a sortable time.
// Unparseable dates return the zero time (sorted last in descending order).
func parseCatalogDate(s string) time.Time {
	for _, layout := range []string{"Jan. 2, 2006", "Jan 2, 2006", "2006-01-02", "1/2/2006"} {
		if t, err := time.Parse(layout, strings.TrimSpace(s)); err == nil {
			return t
		}
	}
	return time.Time{}
}

func sortDocsByDateDesc(docs []extron.Doc) {
	sort.SliceStable(docs, func(i, j int) bool {
		ti, tj := parseCatalogDate(docs[i].Date), parseCatalogDate(docs[j].Date)
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return strings.ToLower(docs[i].Title) < strings.ToLower(docs[j].Title)
	})
}

// curated family prefixes used by `literature family` and model resolution.
var familyPrefixes = []string{
	"DTP", "MAV", "IPL", "DVS", "SMD", "PCS", "MLC", "SMX", "MGP", "MDA",
	"Matrix", "Annotator", "MediaLink", "IN", "TLP", "TLS", "TouchLink",
}

// variantAfterPrefix returns 0 when the title continues the (lowercased)
// query prefix with a letter, 1 when it continues with a digit, and 2
// otherwise. Lower ranks first: family/series docs outrank model variants.
func variantAfterPrefix(title, q string) int {
	t := strings.ToLower(title)
	if !strings.HasPrefix(t, q) || len(t) == len(q) {
		return 2
	}
	i := len(q)
	for i < len(t) && (t[i] == ' ' || t[i] == '\t') {
		i++
	}
	if i >= len(t) {
		return 2
	}
	if t[i] >= '0' && t[i] <= '9' {
		return 1
	}
	return 0
}

// hintIfCatalogIncomplete warns when the local catalog exists but was synced
// only partially (bounded sync or --max-pages cap), so a partial catalog is
// never silently treated as authoritative.
func hintIfCatalogIncomplete(cmd *cobra.Command, db *store.Store) {
	if cmd == nil || db == nil {
		return
	}
	cursor, _, _, err := db.GetSyncState(catalogResource)
	if err != nil || cursor == "" {
		return
	}
	if cursor != "full" {
		fmt.Fprintf(cmd.ErrOrStderr(), "hint: the local catalog is partial (cursor %q); run 'extron-pp-cli catalog sync --full' for the complete catalog.\n", cursor)
	}
}

// docsToTable maps docs into rows for the human table renderer.
func docsToTable(docs []extron.Doc) []map[string]any {
	rows := make([]map[string]any, 0, len(docs))
	for _, d := range docs {
		rows = append(rows, map[string]any{
			"title":    d.Title,
			"category": d.Category,
			"rev":      d.Rev,
			"date":     d.Date,
			"size":     d.Size,
			"type":     d.Type,
			"url":      d.URL,
		})
	}
	return rows
}

// resolveDocs scores every catalog doc against a model query and returns the
// best matches (exact title > title prefix > title substring > url match).
func resolveDocs(docs []extron.Doc, query string, limit int) []extron.Doc {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	type scored struct {
		doc   extron.Doc
		score int
	}
	var matches []scored
	for _, d := range docs {
		title := strings.ToLower(d.Title)
		url := strings.ToLower(d.URL)
		switch {
		case title == q:
			matches = append(matches, scored{d, 100})
		case strings.HasPrefix(title, q):
			matches = append(matches, scored{d, 80})
		case strings.Contains(title, q):
			matches = append(matches, scored{d, 60})
		case strings.Contains(url, q):
			matches = append(matches, scored{d, 40})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		// Same score: prefer the title that continues the query with a letter
		// (family/series doc, e.g. "MAV Plus Series") over one that continues
		// with a digit (specific model variant, e.g. "MAV Plus 328 A").
		if vi, vj := variantAfterPrefix(matches[i].doc.Title, q), variantAfterPrefix(matches[j].doc.Title, q); vi != vj {
			return vi < vj
		}
		if len(matches[i].doc.Title) != len(matches[j].doc.Title) {
			return len(matches[i].doc.Title) < len(matches[j].doc.Title)
		}
		return matches[i].doc.Title < matches[j].doc.Title
	})
	if limit <= 0 {
		limit = len(matches)
	}
	out := make([]extron.Doc, 0, len(matches))
	for i, m := range matches {
		if i >= limit {
			break
		}
		out = append(out, m.doc)
	}
	return out
}
