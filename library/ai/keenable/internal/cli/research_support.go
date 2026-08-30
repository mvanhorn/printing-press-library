package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/keenable/internal/store"
)

type researchResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
	Snippet     string `json:"snippet,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
	AcquiredAt  string `json:"acquired_at,omitempty"`
	Rank        int    `json:"rank,omitempty"`
}

type researchPage struct {
	URL         string `json:"url"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Author      string `json:"author,omitempty"`
	Content     string `json:"content"`
	PublishedAt any    `json:"published_at,omitempty"`
	ContentHash string `json:"content_hash,omitempty"`
}

type researchSnapshot struct {
	ID              string `json:"id"`
	Query           string `json:"query"`
	Site            string `json:"site,omitempty"`
	AcquiredAfter   string `json:"acquired_after,omitempty"`
	AcquiredBefore  string `json:"acquired_before,omitempty"`
	PublishedAfter  string `json:"published_after,omitempty"`
	PublishedBefore string `json:"published_before,omitempty"`
	QueryTime       string `json:"query_time,omitempty"`
	CreatedAt       string `json:"created_at"`
	AuthMode        string `json:"auth_mode"`
	ResultCount     int    `json:"result_count"`
	FetchedCount    int    `json:"fetched_count"`
}

type researchSearchRequest struct {
	Query           string
	Site            string
	AcquiredAfter   string
	AcquiredBefore  string
	PublishedAfter  string
	PublishedBefore string
	QueryTime       string
	MaxResults      int
	SnippetMax      int
	Authenticated   bool
}

func researchDBPath() string { return defaultDBPath("keenable-pp-cli") }

func openResearchStore(ctx context.Context) (*store.Store, error) {
	return store.OpenWithContext(ctx, researchDBPath())
}

func researchHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func newResearchSnapshotID(query string) string {
	now := time.Now().UTC()
	return now.Format("20060102T150405Z") + "-" + researchHash(query+now.String())[:10]
}

func searchBody(req researchSearchRequest) map[string]any {
	body := map[string]any{"query": req.Query}
	if req.Site != "" {
		body["site"] = req.Site
	}
	if req.AcquiredAfter != "" {
		body["acquired_after"] = req.AcquiredAfter
	}
	if req.AcquiredBefore != "" {
		body["acquired_before"] = req.AcquiredBefore
	}
	if req.PublishedAfter != "" {
		body["published_after"] = req.PublishedAfter
	}
	if req.PublishedBefore != "" {
		body["published_before"] = req.PublishedBefore
	}
	if req.QueryTime != "" {
		body["query_time"] = req.QueryTime
	}
	if req.MaxResults > 0 {
		body["max_results"] = req.MaxResults
	}
	if req.SnippetMax > 0 {
		body["snippet_max_length"] = req.SnippetMax
	}
	return body
}

func researchSearch(ctx context.Context, flags *rootFlags, req researchSearchRequest) ([]researchResult, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	path := "/v1/search"
	headers := map[string]string{}
	if !req.Authenticated {
		path = "/v1/search/public"
		headers["X-Keenable-Title"] = "Keenable Research CLI"
	}
	data, _, err := c.PostQueryWithParamsAndHeaders(ctx, path, nil, searchBody(req), headers)
	if err != nil {
		return nil, err
	}
	var response struct {
		Results []researchResult `json:"results"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}
	for i := range response.Results {
		response.Results[i].Rank = i + 1
	}
	return response.Results, nil
}

func researchFetch(ctx context.Context, flags *rootFlags, rawURL string, maxChars int, live bool, prompt string, authenticated bool) (researchPage, error) {
	c, err := flags.newClient()
	if err != nil {
		return researchPage{}, err
	}
	path := "/v1/fetch"
	headers := map[string]string{}
	if !authenticated {
		path = "/v1/fetch/public"
		headers["X-Keenable-Title"] = "Keenable Research CLI"
	}
	params := map[string]string{"url": rawURL}
	if maxChars > 0 {
		params["max_chars"] = fmt.Sprintf("%d", maxChars)
	}
	if live {
		params["live"] = "true"
	}
	if prompt != "" {
		params["prompt"] = prompt
	}
	data, err := c.GetWithHeaders(ctx, path, params, headers)
	if err != nil {
		return researchPage{}, err
	}
	var page researchPage
	if err := json.Unmarshal(data, &page); err != nil {
		return researchPage{}, fmt.Errorf("decode fetch response: %w", err)
	}
	page.ContentHash = researchHash(page.Content)
	return page, nil
}

func persistResearchSnapshot(s *store.Store, snap researchSnapshot, results []researchResult, pages []researchPage) error {
	writes := make([]store.ResourceWrite, 0, 1+len(results)+len(pages))
	writes = append(writes, store.ResourceWrite{
		ResourceType: "research_snapshots",
		ID:           snap.ID,
		Data:         mustJSON(snap),
	})
	for _, result := range results {
		writes = append(writes, store.ResourceWrite{
			ResourceType: "research_results",
			ID:           snap.ID + ":" + researchHash(result.URL),
			Data:         mustJSON(map[string]any{"snapshot_id": snap.ID, "result": result}),
		})
	}
	for _, page := range pages {
		writes = append(writes, store.ResourceWrite{
			ResourceType: "research_pages",
			ID:           snap.ID + ":" + researchHash(page.URL),
			Data:         mustJSON(map[string]any{"snapshot_id": snap.ID, "page": page}),
		})
	}
	return s.UpsertMany(writes)
}

func mustJSON(value any) json.RawMessage {
	data, _ := json.Marshal(value)
	return data
}

func loadResearchSnapshot(s *store.Store, id string) (researchSnapshot, error) {
	if id == "latest" || id == "" {
		items, err := s.List("research_snapshots", 1)
		if err != nil {
			return researchSnapshot{}, err
		}
		if len(items) == 0 {
			return researchSnapshot{ID: "latest", AuthMode: "unknown"}, nil
		}
		var snap researchSnapshot
		if err := json.Unmarshal(items[0], &snap); err != nil {
			return snap, err
		}
		return snap, nil
	}
	data, err := s.Get("research_snapshots", id)
	if err != nil {
		return researchSnapshot{}, fmt.Errorf("snapshot %q not found: %w", id, err)
	}
	var snap researchSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return snap, err
	}
	return snap, nil
}

func loadSnapshotResults(s *store.Store, id string) ([]researchResult, error) {
	items, err := s.List("research_results", 0)
	if err != nil {
		return nil, err
	}
	results := make([]researchResult, 0)
	for _, item := range items {
		var row struct {
			SnapshotID string         `json:"snapshot_id"`
			Result     researchResult `json:"result"`
		}
		if json.Unmarshal(item, &row) == nil && row.SnapshotID == id {
			results = append(results, row.Result)
		}
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Rank < results[j].Rank })
	return results, nil
}

func loadSnapshotPages(s *store.Store, id string) ([]researchPage, error) {
	items, err := s.List("research_pages", 0)
	if err != nil {
		return nil, err
	}
	pages := make([]researchPage, 0)
	for _, item := range items {
		var row struct {
			SnapshotID string       `json:"snapshot_id"`
			Page       researchPage `json:"page"`
		}
		if json.Unmarshal(item, &row) == nil && row.SnapshotID == id {
			pages = append(pages, row.Page)
		}
	}
	return pages, nil
}

func saveLiveSnapshot(ctx context.Context, flags *rootFlags, req researchSearchRequest, fetchTop int, maxChars int, live bool, prompt string) (researchSnapshot, []researchResult, []researchPage, error) {
	results, err := researchSearch(ctx, flags, req)
	if err != nil {
		return researchSnapshot{}, nil, nil, err
	}
	if fetchTop < 0 {
		fetchTop = 0
	}
	if fetchTop > len(results) {
		fetchTop = len(results)
	}
	pages := make([]researchPage, 0, fetchTop)
	for _, result := range results[:fetchTop] {
		page, fetchErr := researchFetch(ctx, flags, result.URL, maxChars, live, prompt, req.Authenticated)
		if fetchErr != nil {
			continue
		}
		pages = append(pages, page)
	}
	snap := researchSnapshot{ID: newResearchSnapshotID(req.Query), Query: req.Query, Site: req.Site, AcquiredAfter: req.AcquiredAfter, AcquiredBefore: req.AcquiredBefore, PublishedAfter: req.PublishedAfter, PublishedBefore: req.PublishedBefore, QueryTime: req.QueryTime, CreatedAt: time.Now().UTC().Format(time.RFC3339), AuthMode: map[bool]string{true: "authenticated", false: "public"}[req.Authenticated], ResultCount: len(results), FetchedCount: len(pages)}
	s, err := openResearchStore(ctx)
	if err != nil {
		return snap, nil, nil, err
	}
	defer s.Close()
	if err := persistResearchSnapshot(s, snap, results, pages); err != nil {
		return snap, nil, nil, fmt.Errorf("persist snapshot: %w", err)
	}
	return snap, results, pages, nil
}

func fetchMany(ctx context.Context, flags *rootFlags, urls []string, maxChars, concurrency int, live, authenticated bool) ([]researchPage, []map[string]any) {
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > 8 {
		concurrency = 8
	}
	type job struct {
		index  int
		rawURL string
	}
	type result struct {
		index int
		page  researchPage
		err   error
	}
	jobs := make(chan job)
	results := make(chan result, len(urls))
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				page, err := researchFetch(ctx, flags, item.rawURL, maxChars, live, "", authenticated)
				results <- result{index: item.index, page: page, err: err}
			}
		}()
	}
	go func() {
		for i, rawURL := range urls {
			jobs <- job{index: i, rawURL: rawURL}
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()
	ordered := make([]result, len(urls))
	for item := range results {
		ordered[item.index] = item
	}
	pages := make([]researchPage, 0, len(urls))
	failures := make([]map[string]any, 0)
	for _, item := range ordered {
		if item.err != nil {
			failures = append(failures, map[string]any{"url": urls[item.index], "error": item.err.Error()})
			continue
		}
		pages = append(pages, item.page)
	}
	return pages, failures
}

func researchDomain(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" {
		return "unknown"
	}
	return strings.ToLower(parsed.Hostname())
}
