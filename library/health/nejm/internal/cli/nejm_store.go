// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.
// Shared local-store helpers for the article corpus: the canonical record
// shape, query/print plumbing, store opening, and the article detail-page
// meta extraction used by --enrich. Relocated from the retired nejm_feeds.go
// when the RSS transport was replaced by the OpenAlex sync source
// (nejm_openalex_sync.go); nothing here is RSS-specific.

package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/nejm/internal/client"
	"github.com/mvanhorn/printing-press-library/library/health/nejm/internal/store"
	xhtml "golang.org/x/net/html"
)

// nejmBaseURL is the origin article pages are fetched from for --enrich. It is
// a var rather than a const so tests can point it at an httptest server.
var nejmBaseURL = "https://www.nejm.org"

// nejmArticleRecord is the canonical shape stored in SQLite.
// The "id" field drives UpsertArticle's primary-key lookup.
type nejmArticleRecord struct {
	ID          string `json:"id"`
	DOI         string `json:"doi"`
	Title       string `json:"title"`
	Authors     string `json:"authors"`
	Abstract    string `json:"abstract"`
	ArticleType string `json:"article_type"`
	Specialties string `json:"specialties"`
	Date        string `json:"date"`
	IsFree      bool   `json:"is_free"`
	URL         string `json:"url"`
	Volume      string `json:"volume,omitempty"`
	Issue       string `json:"issue,omitempty"`
	Pages       string `json:"pages,omitempty"`
	CoverDate   string `json:"cover_date,omitempty"`
	Feed        string `json:"feed,omitempty"`
}

// nejmCurrentIssueWhere scopes article queries to the most recent publication
// week in the corpus. The retired RSS transport partitioned rows by feed
// ('etoc' marked the current issue); OpenAlex carries no issue partition, so
// "current issue" is approximated as everything published within six days of
// the newest article. NEJM publishes weekly, so the window tracks one issue.
const nejmCurrentIssueWhere = `date >= (SELECT date(MAX(date), '-6 days') FROM "article")`

// --- Article detail page extraction ---

// nejmFetchAndParseArticle fetches an article detail page via the Surf client
// and extracts NEJM-specific meta tags (abstract, article_type, specialties, isFree).
// Merges results into an existing nejmArticleRecord (DOI already known).
func nejmFetchAndParseArticle(ctx context.Context, c *client.Client, doi string) (nejmArticleRecord, error) {
	path := "/doi/full/" + doi
	raw, err := c.GetWithHeaders(ctx, path, nil, nil)
	if err != nil {
		return nejmArticleRecord{}, fmt.Errorf("fetching article %s: %w", doi, err)
	}

	meta := nejmParseArticleMeta(raw)

	articleURL := nejmBaseURL + "/doi/full/" + doi

	rec := nejmArticleRecord{
		ID:          doi,
		DOI:         doi,
		Title:       meta["dc.Title"],
		Authors:     meta["dc.Creator"],
		Abstract:    firstNonEmpty(meta["description"], meta["shortAbstract"], meta["og:description"]),
		ArticleType: firstNonEmpty(meta["articleType"], meta["articleCategory"]),
		Specialties: meta["Specialties"],
		Date:        meta["dc.Date"],
		IsFree:      strings.EqualFold(meta["isFree"], "true"),
		URL:         articleURL,
	}
	return rec, nil
}

// 🔧 JAVÍTOTT: nejmParseArticleMeta - bytes.NewReader használata
func nejmParseArticleMeta(htmlBytes []byte) map[string]string {
	meta := make(map[string]string)

	doc, err := xhtml.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		return meta
	}

	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode {
			tag := strings.ToLower(n.Data)
			if tag == "meta" {
				name := attrValue(n, "name")
				if name == "" {
					name = attrValue(n, "property")
				}
				content := attrValue(n, "content")
				if name != "" && content != "" {
					meta[name] = content
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	return meta
}

// firstNonEmpty returns the first non-empty string from the arguments.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// 🔧 JAVÍTOTT: nejmQueryArticles - SQL NULL kezelés minden oszlopnál
func nejmQueryArticles(db *store.Store, where string, args []any, limit int) ([]map[string]any, error) {
	q := `SELECT doi, title, authors, abstract, article_type, specialties, date, is_free, url, feed, synced_at FROM "article"`
	if where != "" {
		q += " WHERE " + where
	}
	q += " ORDER BY date DESC, synced_at DESC"
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var doi string
		var title, authors, abstract, articleType, specialties, date, url, syncedAt sql.NullString
		var feed *string
		var isFree bool

		if err := rows.Scan(&doi, &title, &authors, &abstract, &articleType, &specialties, &date, &isFree, &url, &feed, &syncedAt); err != nil {
			continue
		}

		// NULL értékek kezelése - ha NULL, üres stringet használunk
		titleStr := ""
		if title.Valid {
			titleStr = title.String
		}
		authorsStr := ""
		if authors.Valid {
			authorsStr = authors.String
		}
		abstractStr := ""
		if abstract.Valid {
			abstractStr = abstract.String
		}
		articleTypeStr := ""
		if articleType.Valid {
			articleTypeStr = articleType.String
		}
		specialtiesStr := ""
		if specialties.Valid {
			specialtiesStr = specialties.String
		}
		dateStr := ""
		if date.Valid {
			dateStr = date.String
		}
		urlStr := ""
		if url.Valid {
			urlStr = url.String
		}
		syncedAtStr := ""
		if syncedAt.Valid {
			syncedAtStr = syncedAt.String
		}
		feedVal := ""
		if feed != nil {
			feedVal = *feed
		}

		results = append(results, map[string]any{
			"doi":          doi,
			"title":        titleStr,
			"authors":      authorsStr,
			"abstract":     abstractStr,
			"article_type": articleTypeStr,
			"specialties":  specialtiesStr,
			"date":         dateStr,
			"is_free":      isFree,
			"url":          urlStr,
			"feed":         feedVal,
			"synced_at":    syncedAtStr,
		})
	}
	return results, rows.Err()
}

// nejmOpenStore opens the store for reading, returning a clear error if not synced.
// openStoreForRead opens the DB read-write (WAL), so the idempotent feed-column
// migration runs here too — guaranteeing `current` and friends never hit a
// "no such column: feed" error even if the user has not re-synced since upgrading.
func nejmOpenStore(ctx context.Context) (*store.Store, error) {
	db, err := openStoreForRead(ctx, "nejm-pp-cli")
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w\nRun 'nejm-pp-cli sync' first", err)
	}
	if db == nil {
		return nil, fmt.Errorf("no local data. Run 'nejm-pp-cli sync' first to populate the corpus")
	}
	if err := nejmEnsureFeedColumn(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrating local database: %w", err)
	}
	return db, nil
}

// nejmPrintArticles marshals a []map[string]any slice and writes it to the command.
func nejmPrintArticles(cmd interface {
	OutOrStdout() io.Writer
	ErrOrStderr() io.Writer
}, items []map[string]any, flags *rootFlags) error {
	if len(items) == 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "hint: no articles found. Run 'nejm-pp-cli sync' to populate the corpus.")
		if flags.asJSON {
			return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage("[]"), flags)
		}
		return nil
	}
	data, err := json.Marshal(items)
	if err != nil {
		return err
	}
	return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(data), flags)
}

// parseSinceDuration converts strings like "48h", "7d", "2w" into a time.Time cutoff.
// Duplicated here (also exists in sync.go) to keep the store helpers self-contained.
func nejmParseSinceDuration(s string) (time.Time, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return time.Time{}, fmt.Errorf("empty duration")
	}
	unit := s[len(s)-1]
	numStr := s[:len(s)-1]
	switch unit {
	case 'h':
		var n int
		if _, err := fmt.Sscanf(numStr, "%d", &n); err != nil {
			return time.Time{}, fmt.Errorf("invalid duration %q", s)
		}
		return time.Now().Add(-time.Duration(n) * time.Hour), nil
	case 'd':
		var n int
		if _, err := fmt.Sscanf(numStr, "%d", &n); err != nil {
			return time.Time{}, fmt.Errorf("invalid duration %q", s)
		}
		return time.Now().AddDate(0, 0, -n), nil
	case 'w':
		var n int
		if _, err := fmt.Sscanf(numStr, "%d", &n); err != nil {
			return time.Time{}, fmt.Errorf("invalid duration %q", s)
		}
		return time.Now().AddDate(0, 0, -n*7), nil
	default:
		// Try standard Go duration
		d, err := time.ParseDuration(s)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid duration %q: use e.g. 48h, 7d, 2w", s)
		}
		return time.Now().Add(-d), nil
	}
}
