// Copyright 2026 nicolas-correa. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/iqos/internal/store"
)

// Sitemap XML types
type sitemapURL struct {
	Loc     string `xml:"loc"`
	Lastmod string `xml:"lastmod"`
}

type sitemapIndex struct {
	URLs []sitemapURL `xml:"url"`
}

// iqosCountryMap maps commonly used country codes to their default language.
var iqosCountryMap = map[string]string{
	"us": "en", "gb": "en", "de": "de", "fr": "fr", "it": "it",
	"es": "es", "pt": "pt", "nl": "nl", "be": "fr", "ch": "de",
	"at": "de", "pl": "pl", "cz": "cs", "sk": "sk", "hu": "hu",
	"ro": "ro", "bg": "bg", "gr": "el", "rs": "sr", "hr": "hr",
	"ua": "uk", "kz": "ru", "jp": "ja", "kr": "ko", "ph": "en",
	"my": "en", "th": "th", "nz": "en", "au": "en", "za": "en",
	"ae": "en", "il": "he", "tr": "tr", "mx": "es", "ca": "en",
}

// AllIQOSCountries returns the list of all known IQOS market country codes.
func allIQOSCountries() []string {
	countries := make([]string, 0, len(iqosCountryMap))
	for cc := range iqosCountryMap {
		countries = append(countries, cc)
	}
	return countries
}

// langForCountry returns the default language for a country code.
func langForCountry(country string) string {
	if lang, ok := iqosCountryMap[country]; ok {
		return lang
	}
	return "en"
}

// fetchSitemap fetches and parses a product sitemap for a given country/lang.
func fetchSitemap(ctx context.Context, baseURL, country, lang, sitemapType string) ([]sitemapURL, error) {
	sitemapPath := fmt.Sprintf("/%s/%s.sitemap_%s.xml", country, lang, sitemapType)
	fullURL := strings.TrimRight(baseURL, "/") + sitemapPath

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "iqos-pp-cli/1.0 (catalog sync)")
	req.Header.Set("Accept", "application/xml,text/xml,*/*")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching sitemap %s: %w", fullURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // market doesn't have this sitemap type
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sitemap %s returned HTTP %d", sitemapPath, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10 MB max
	if err != nil {
		return nil, fmt.Errorf("reading sitemap body: %w", err)
	}

	var sm sitemapIndex
	if err := xml.Unmarshal(body, &sm); err != nil {
		return nil, fmt.Errorf("parsing sitemap XML: %w", err)
	}
	return sm.URLs, nil
}

// extractSlugFromURL pulls the product slug from a full IQOS product URL.
// e.g. "https://www.iqos.com/us/en/shop/heets-pack-amber.html" → "heets-pack-amber"
func extractSlugFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	path := strings.TrimSuffix(u.Path, ".html")
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// extractCategoryFromURL guesses product category from its URL path.
func extractCategoryFromURL(rawURL string) string {
	lower := strings.ToLower(rawURL)
	switch {
	case strings.Contains(lower, "/shop/heets") || strings.Contains(lower, "/heets-"):
		return "heets"
	case strings.Contains(lower, "/shop/terea") || strings.Contains(lower, "/terea-"):
		return "terea"
	case strings.Contains(lower, "/levia"):
		return "levia"
	case strings.Contains(lower, "/veev") || strings.Contains(lower, "/vaping"):
		return "veev"
	case strings.Contains(lower, "/zyn"):
		return "zyn"
	case strings.Contains(lower, "/iluma") || strings.Contains(lower, "/originals") ||
		strings.Contains(lower, "/iqos-3") || strings.Contains(lower, "/iqos-2"):
		return "devices"
	case strings.Contains(lower, "/accessories") || strings.Contains(lower, "/accessory"):
		return "accessories"
	default:
		return "other"
	}
}

// schemaOrgProduct represents the schema.org Product JSON-LD found on product pages.
type schemaOrgProduct struct {
	Type        string `json:"@type"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Offers      struct {
		Price    interface{} `json:"price"`
		Currency string      `json:"priceCurrency"`
	} `json:"offers"`
}

// fetchProductPage retrieves a product page and extracts schema.org Product data.
// Returns an empty struct (no error) when the page has no JSON-LD.
func fetchProductPage(ctx context.Context, productURL string) (schemaOrgProduct, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, productURL, nil)
	if err != nil {
		return schemaOrgProduct{}, err
	}
	req.Header.Set("User-Agent", "iqos-pp-cli/1.0 (catalog sync)")

	c := &http.Client{Timeout: 20 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return schemaOrgProduct{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return schemaOrgProduct{}, err
	}

	return extractSchemaOrgProduct(string(body)), nil
}

// extractSchemaOrgProduct parses schema.org/Product from HTML.
func extractSchemaOrgProduct(html string) schemaOrgProduct {
	// Find application/ld+json script blocks
	const start = `application/ld+json`
	remaining := html
	for {
		idx := strings.Index(remaining, start)
		if idx < 0 {
			break
		}
		rest := remaining[idx:]
		open := strings.Index(rest, ">")
		if open < 0 {
			break
		}
		rest = rest[open+1:]
		close := strings.Index(rest, "</script>")
		if close < 0 {
			break
		}
		jsonText := strings.TrimSpace(rest[:close])

		var raw interface{}
		if err := json.Unmarshal([]byte(jsonText), &raw); err != nil {
			remaining = rest[close:]
			continue
		}

		// Handle both single object and @graph array
		candidates := []json.RawMessage{}
		switch v := raw.(type) {
		case map[string]interface{}:
			if graph, ok := v["@graph"]; ok {
				if arr, err := json.Marshal(graph); err == nil {
					var items []json.RawMessage
					if json.Unmarshal(arr, &items) == nil {
						candidates = items
					}
				}
			} else {
				if b, err := json.Marshal(v); err == nil {
					candidates = append(candidates, b)
				}
			}
		case []interface{}:
			for _, item := range v {
				if b, err := json.Marshal(item); err == nil {
					candidates = append(candidates, b)
				}
			}
		}

		for _, c := range candidates {
			var p schemaOrgProduct
			if err := json.Unmarshal(c, &p); err == nil && p.Type == "Product" && p.Name != "" {
				return p
			}
		}
		remaining = rest[close:]
	}
	return schemaOrgProduct{}
}

// IQOSProductRow is the denormalized row type saved per product.
type IQOSProductRow struct {
	Slug    string `json:"slug"`
	Country string `json:"country"`
	Name    string `json:"name"`
	URL     string `json:"url"`
	Category string `json:"category"`
	Price   string `json:"price"`
}

// saveIQOSProducts upserts a batch of products into iqos_products.
func saveIQOSProducts(ctx context.Context, db *store.Store, rows []IQOSProductRow) error {
	rawDB := db.DB()
	tx, err := rawDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO iqos_products (slug, country, name, url, category, price, data, synced_at)
		VALUES (?, ?, ?, ?, ?, ?, '{}', CURRENT_TIMESTAMP)
		ON CONFLICT(slug, country) DO UPDATE SET
			name=excluded.name, url=excluded.url, category=excluded.category,
			price=excluded.price, synced_at=excluded.synced_at
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, r := range rows {
		if _, err := stmt.ExecContext(ctx, r.Slug, r.Country, r.Name, r.URL, r.Category, r.Price); err != nil {
			return fmt.Errorf("saving product %s/%s: %w", r.Country, r.Slug, err)
		}
	}
	return tx.Commit()
}

// saveIQOSSnapshot records a snapshot for change detection.
func saveIQOSSnapshot(ctx context.Context, db *store.Store, country string, rows []IQOSProductRow) error {
	rawDB := db.DB()
	tx, err := rawDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	snapshotAt := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO iqos_snapshots (slug, country, price, present, snapshot_at)
		VALUES (?, ?, ?, 1, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, r := range rows {
		if _, err := stmt.ExecContext(ctx, r.Slug, r.Country, r.Price, snapshotAt); err != nil {
			return fmt.Errorf("saving snapshot %s/%s: %w", r.Country, r.Slug, err)
		}
	}
	return tx.Commit()
}
