// HTTP requests in this file go through doWithLimiter (see ratelimit.go),
// which paces calls with cliutil.AdaptiveLimiter and surfaces a typed
// *cliutil.RateLimitError when 429 retries are exhausted. Empty-on-throttle
// is indistinguishable from "no data exists" — callers must treat the
// limiter's typed error as a hard failure rather than swallow it.
package food52

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// TypesenseRecipeCollection is the Food52-deployed search collection name.
// It does not change with each deploy — the cluster ID and search-only key
// do (and we rotate those via discovery), but the collection slug has been
// stable for the life of the search backend.
const TypesenseRecipeCollection = "recipes_production_food52_current"

// SearchRecipesParams is the shape for a recipes search call.
type SearchRecipesParams struct {
	Query   string
	Tag     string // optional: filter_by tags
	Page    int    // 1-indexed
	PerPage int    // typesense max 250; default 36
	Sort    string // optional: e.g. "popularity:desc", "publishedAt:desc"
}

// SearchRecipesResponse is the agent-friendly shape we return from
// `recipes search`. We project the Typesense hit shape into RecipeSummary
// records to keep field names consistent with `recipes browse`.
type SearchRecipesResponse struct {
	Query      string          `json:"query"`
	Page       int             `json:"page"`
	PerPage    int             `json:"per_page"`
	Found      int             `json:"found"`
	OutOf      int             `json:"out_of"`
	SearchTime int             `json:"search_time_ms"`
	Hits       []RecipeSummary `json:"hits"`
}

// SearchRecipes runs a Typesense search against Food52's recipes collection.
// It uses the discovered host + key (do not call this without a populated
// Discovery — pass a fresh one from LoadDiscovery).
//
// On 401/403 the caller should InvalidateDiscovery() and retry once: that
// covers the case where Food52 rotated the search-only key.
func SearchRecipes(httpc HTTPClient, d *Discovery, p SearchRecipesParams) (*SearchRecipesResponse, error) {
	if d == nil || d.TypesenseHost == "" || d.TypesenseAPIKey == "" {
		return nil, fmt.Errorf("food52 search: discovery not loaded (need typesense host + key)")
	}
	if strings.TrimSpace(p.Query) == "" {
		return nil, fmt.Errorf("food52 search: query is required")
	}
	per := p.PerPage
	if per <= 0 {
		per = 36
	}
	if per > 250 {
		per = 250
	}
	page := p.Page
	if page <= 0 {
		page = 1
	}

	q := url.Values{}
	q.Set("q", p.Query)
	q.Set("query_by", "title,metaDescription,tags")
	q.Set("per_page", strconv.Itoa(per))
	q.Set("page", strconv.Itoa(page))
	q.Set("facet_by", "tags")
	if p.Tag != "" {
		q.Set("filter_by", "tagSlugs:="+p.Tag)
	}
	if p.Sort != "" {
		q.Set("sort_by", p.Sort)
	}

	full := fmt.Sprintf("https://%s/collections/%s/documents/search?%s", d.TypesenseHost, TypesenseRecipeCollection, q.Encode())
	req, err := http.NewRequest("GET", full, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-TYPESENSE-API-KEY", d.TypesenseAPIKey)
	req.Header.Set("Accept", "application/json")

	resp, body, err := doWithLimiter(httpc, req)
	if err != nil {
		return nil, fmt.Errorf("food52 search: %w", err)
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return nil, ErrTypesenseAuth
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("food52 search: typesense HTTP %d: %s", resp.StatusCode, truncateForErr(body))
	}

	var raw typesenseRawResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("food52 search: parsing typesense response: %w", err)
	}

	out := &SearchRecipesResponse{
		Query:      p.Query,
		Page:       page,
		PerPage:    per,
		Found:      raw.Found,
		OutOf:      raw.OutOf,
		SearchTime: raw.SearchTimeMs,
	}
	for _, h := range raw.Hits {
		doc := h.Document
		slug := stringField(doc, "slug")
		recURL := ""
		if slug != "" {
			recURL = "https://food52.com/recipes/" + slug
		}
		out.Hits = append(out.Hits, RecipeSummary{
			ID:                  stringField(doc, "id"),
			Slug:                slug,
			Title:               stringField(doc, "title"),
			URL:                 recURL,
			Description:         stringField(doc, "metaDescription"),
			FeaturedImageURL:    stringField(doc, "featuredImageUrl"),
			AverageRating:       floatField(doc, "rating"),
			RatingCount:         intField(doc, "ratingCount"),
			TestKitchenApproved: boolField(doc, "testKitchenApproved"),
			Tags:                stringSliceFromAny(doc["tags"]),
			PublishedAt:         publishedAtToISO(floatField(doc, "publishedAt")),
		})
	}
	return out, nil
}

// ErrTypesenseAuth is returned when the search-only key is rejected. Callers
// should InvalidateDiscovery() and retry once.
var ErrTypesenseAuth = fmt.Errorf("food52 search: typesense rejected the search-only key (likely rotated)")

type typesenseRawResponse struct {
	Found        int `json:"found"`
	OutOf        int `json:"out_of"`
	SearchTimeMs int `json:"search_time_ms"`
	Hits         []struct {
		Document map[string]any `json:"document"`
	} `json:"hits"`
}

func stringSliceFromAny(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, x := range arr {
		if s, ok := x.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// publishedAtToISO converts the millisecond epoch Typesense returns to an
// RFC3339 string so it matches the SSR `publishedAt` field shape.
func publishedAtToISO(ms float64) string {
	if ms <= 0 {
		return ""
	}
	// Use time.UnixMilli for a clean conversion.
	return unixMilliToRFC3339(int64(ms))
}

func truncateForErr(b []byte) string {
	const max = 200
	s := string(b)
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
