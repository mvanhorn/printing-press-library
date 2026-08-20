package crestronstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
)

// ReleaseHit is a full-text search hit against release notes and change logs.
type ReleaseHit struct {
	Release
	Snippet string `json:"snippet,omitempty"`
}

// releaseDateSortKey builds a sortable YYYYMMDD key from the site's
// "Mon DD, YYYY" date text, which is uniform across every synced release.
// Ordering by the raw column sorts alphabetically, putting "Sep 29, 2008"
// ahead of "Nov 01, 2022"; that scrambled result order and made --limit
// truncate an arbitrary slice rather than the newest N.
const releaseDateSortKey = `(substr(r.date,9,4) || CASE substr(r.date,1,3)
		WHEN 'Jan' THEN '01' WHEN 'Feb' THEN '02' WHEN 'Mar' THEN '03'
		WHEN 'Apr' THEN '04' WHEN 'May' THEN '05' WHEN 'Jun' THEN '06'
		WHEN 'Jul' THEN '07' WHEN 'Aug' THEN '08' WHEN 'Sep' THEN '09'
		WHEN 'Oct' THEN '10' WHEN 'Nov' THEN '11' WHEN 'Dec' THEN '12'
		ELSE '00' END || substr(r.date,5,2))`

// siteBaseURL is the origin the mirror's relative release paths hang off. It
// mirrors config.Default().BaseURL, which this package cannot import.
const siteBaseURL = "https://www.crestron.com"

// absoluteURL turns a stored path into a full permalink. Product rows are
// stored absolute and release rows relative, so the same CLI emitted two link
// shapes; callers could paste a product URL but not a release URL.
func absoluteURL(u string) string {
	if u == "" || strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u
	}
	if !strings.HasPrefix(u, "/") {
		u = "/" + u
	}
	return siteBaseURL + u
}

// SearchReleases runs full-text search over release titles, notes, and change
// logs. This is the search Crestron.com cannot offer: release notes live on
// per-version pages behind a sign-in and have never been searchable across
// versions.
func (s *Store) SearchReleases(ctx context.Context, query string, limit int) ([]ReleaseHit, error) {
	out := make([]ReleaseHit, 0)
	if strings.TrimSpace(query) == "" {
		return out, nil
	}
	if limit <= 0 {
		limit = 25
	}
	match := ftsQuery(query)
	if match == "" {
		return out, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.url, r.title, r.version, r.date, r.type, r.models,
		       snippet(crestron_releases_fts, 3, '', '', ' … ', 12)
		FROM crestron_releases_fts f
		JOIN crestron_releases r ON r.rowid = f.rowid
		WHERE crestron_releases_fts MATCH ?
		ORDER BY `+releaseDateSortKey+` DESC
		LIMIT ?`, match, limit)
	if err != nil {
		return out, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var h ReleaseHit
		// Every selected column can be NULL for a row written before a later
		// migration, so scan through sql.Null* rather than bare strings.
		var url, title, version, date, typ, models, snip sql.NullString
		if err := rows.Scan(&url, &title, &version, &date, &typ, &models, &snip); err != nil {
			return out, err
		}
		h.URL, h.Title, h.Version = absoluteURL(url.String), title.String, version.String
		h.Date, h.Type, h.Snippet = date.String, typ.String, snip.String
		if models.String != "" {
			_ = json.Unmarshal([]byte(models.String), &h.Models)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// SearchProducts runs full-text search over product descriptions.
func (s *Store) SearchProducts(ctx context.Context, query string, limit int) ([]Product, error) {
	out := make([]Product, 0)
	if strings.TrimSpace(query) == "" {
		return out, nil
	}
	if limit <= 0 {
		limit = 25
	}
	match := ftsQuery(query)
	if match == "" {
		return out, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.model, p.description, p.url, p.document_id, p.category_path,
		       p.sku, p.image_url, p.discontinued
		FROM crestron_products_fts f
		JOIN crestron_products p ON p.rowid = f.rowid
		WHERE crestron_products_fts MATCH ?
		LIMIT ?`, match, limit)
	if err != nil {
		return out, err
	}
	defer func() { _ = rows.Close() }()
	return scanProducts(rows, out)
}

// FindProduct looks a model up exactly, then case-insensitively, then by a
// normalized form that ignores punctuation so "dm nvx 360" finds "DM-NVX-360".
func (s *Store) FindProduct(ctx context.Context, model string) (Product, bool, error) {
	var p Product
	row := s.db.QueryRowContext(ctx, `
		SELECT model, description, url, document_id, category_path, sku, image_url, discontinued
		FROM crestron_products
		WHERE model = ? COLLATE NOCASE
		   OR REPLACE(REPLACE(REPLACE(UPPER(model),'-',''),' ',''),'_','')
		      = REPLACE(REPLACE(REPLACE(UPPER(?),'-',''),' ',''),'_','')
		LIMIT 1`, model, model)
	var desc, url, docID, cat, sku, img sql.NullString
	var disc sql.NullInt64
	var m sql.NullString
	if err := row.Scan(&m, &desc, &url, &docID, &cat, &sku, &img, &disc); err != nil {
		if err == sql.ErrNoRows {
			return p, false, nil
		}
		return p, false, err
	}
	p = Product{
		Model: m.String, Description: desc.String, URL: url.String,
		DocumentID: docID.String, CategoryPath: cat.String, SKU: sku.String,
		ImageURL: img.String, Discontinued: disc.Int64 == 1,
	}
	return p, true, nil
}

// ReleasesForModel returns every release covering a model, newest first, using
// the many-to-many join built at sync time.
func (s *Store) ReleasesForModel(ctx context.Context, model string) ([]Release, error) {
	out := make([]Release, 0)
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.url, r.title, r.version, r.date, r.type, r.models, r.notes, r.change_log
		FROM crestron_release_models rm
		JOIN crestron_releases r ON r.url = rm.release_url
		WHERE rm.model = ? COLLATE NOCASE
		   OR REPLACE(REPLACE(REPLACE(UPPER(rm.model),'-',''),' ',''),'_','')
		      = REPLACE(REPLACE(REPLACE(UPPER(?),'-',''),' ',''),'_','')
		ORDER BY `+releaseDateSortKey+` DESC`, model, model)
	if err != nil {
		return out, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var r Release
		var url, title, version, date, typ, models, notes, changelog sql.NullString
		if err := rows.Scan(&url, &title, &version, &date, &typ, &models, &notes, &changelog); err != nil {
			return out, err
		}
		r.URL, r.Title, r.Version = absoluteURL(url.String), title.String, version.String
		r.Date, r.Type = date.String, typ.String
		r.Notes, r.ChangeLog = notes.String, changelog.String
		if models.String != "" {
			_ = json.Unmarshal([]byte(models.String), &r.Models)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListCategories returns every synced category.
func (s *Store) ListCategories(ctx context.Context) ([]Category, error) {
	out := make([]Category, 0)
	rows, err := s.db.QueryContext(ctx, `
		SELECT path, document_id, node_id, product_count
		FROM crestron_categories ORDER BY path`)
	if err != nil {
		return out, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var c Category
		var path, doc, node sql.NullString
		var count sql.NullInt64
		if err := rows.Scan(&path, &doc, &node, &count); err != nil {
			return out, err
		}
		c.Path, c.DocumentID, c.NodeID = path.String, doc.String, node.String
		c.ProductCount = int(count.Int64)
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListDiscontinued returns discontinued products, optionally limited.
func (s *Store) ListDiscontinued(ctx context.Context, limit int) ([]Product, error) {
	out := make([]Product, 0)
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT model, description, url, document_id, category_path, sku, image_url, discontinued
		FROM crestron_products WHERE discontinued = 1 ORDER BY model LIMIT ?`, limit)
	if err != nil {
		return out, err
	}
	defer func() { _ = rows.Close() }()
	return scanProducts(rows, out)
}

func scanProducts(rows *sql.Rows, out []Product) ([]Product, error) {
	for rows.Next() {
		var p Product
		var m, desc, url, docID, cat, sku, img sql.NullString
		var disc sql.NullInt64
		if err := rows.Scan(&m, &desc, &url, &docID, &cat, &sku, &img, &disc); err != nil {
			return out, err
		}
		p = Product{
			Model: m.String, Description: desc.String, URL: url.String,
			DocumentID: docID.String, CategoryPath: cat.String, SKU: sku.String,
			ImageURL: img.String, Discontinued: disc.Int64 == 1,
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ftsQuery makes a user's search text safe for FTS5.
//
// FTS5 treats a bare query as an expression: "-" is NOT, ":" is a column
// filter, and bare words can be read as column names. A model number like
// "DM-NVX" therefore fails with "no such column: NVX". Wrapping each term in
// double quotes turns them into literal phrases, which is what a user typing a
// model number means. Terms are ANDed, matching normal search expectations.
func ftsQuery(raw string) string {
	fields := strings.Fields(raw)
	quoted := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.ReplaceAll(f, `"`, `""`)
		if f == "" {
			continue
		}
		quoted = append(quoted, `"`+f+`"`)
	}
	if len(quoted) == 0 {
		return ""
	}
	return strings.Join(quoted, " AND ")
}
