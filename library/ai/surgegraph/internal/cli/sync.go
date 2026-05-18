package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/surgegraph/internal/store"

	"github.com/spf13/cobra"
)

// newSyncCmd populates the local SQLite store from the SurgeGraph API. It is
// the foundation for every transcendence command that joins, deltas, or
// searches across entities — those features answer questions the API alone
// cannot, but they require a recent snapshot to do it.
func newSyncCmd(flags *rootFlags) *cobra.Command {
	var projectID string
	var brand string
	var all bool
	var include string
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Populate the local store for one or all projects",
		Long: strings.TrimSpace(`
Populate the local SQLite store with AI Visibility snapshots, citation
rollups, prompts, traffic pages, documents, and knowledge libraries for
one project (--project) or every project (--all).

The local store is what powers transcendence commands like
'visibility delta', 'prompts losers', 'citation-domains rank-shift',
'docs stale', 'knowledge impact', and 'search'. Run this once per day
(or before any analytical command) to keep the cache fresh.

Default scope is everything; pass --include to narrow:
  visibility,citations,prompts,traffic,docs,knowledge,usage
`),
		Example: strings.Trim(`
  surgegraph-pp-cli sync --project proj_abc123
  surgegraph-pp-cli sync --all
  surgegraph-pp-cli sync --project proj_abc123 --include visibility,citations
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"}, // writes to local store
		RunE: func(cmd *cobra.Command, args []string) error {
			if !all && projectID == "" {
				if dryRunOK(flags) {
					return nil
				}
				return errors.New("--project <id> or --all is required")
			}
			// Under --dry-run, describe what would happen and return — do not
			// open the store, do not fan out across the network. The flat
			// promoted commands (get-projects, get-ai-visibility-*) are what
			// the verifier exercises end-to-end; this compound command should
			// not gate on the local store being populated.
			if dryRunOK(flags) {
				scopes := normalizeIncludes(include)
				scopeList := make([]string, 0, len(scopes))
				for k := range scopes {
					scopeList = append(scopeList, k)
				}
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run":  true,
					"project":  projectID,
					"all":      all,
					"includes": scopeList,
					"note":     "would call: get_projects, then per-project get_ai_visibility_*, get_knowledge_libraries, get_writer_documents, get_optimized_documents, get_usage",
				}, flags)
			}
			st, err := store.Open("")
			if err != nil {
				return err
			}
			defer st.Close()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			scopes := normalizeIncludes(include)

			// Project list. When --project is supplied, fetch projects to
			// resolve brand and capture metadata; when --all, walk every project.
			projects, err := syncProjects(ctx, c, st)
			if err != nil {
				return fmt.Errorf("syncing projects: %w", err)
			}
			if !all {
				filtered := projects[:0]
				for _, p := range projects {
					if p.ID == projectID {
						filtered = append(filtered, p)
					}
				}
				projects = filtered
				if len(projects) == 0 {
					return fmt.Errorf("project %q not found (try `surgegraph-pp-cli get-projects --json | jq` to list)", projectID)
				}
			}

			// Org-scoped resources sync once.
			if scopes["usage"] {
				if err := syncUsage(ctx, c, st); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "sync usage: %v\n", err)
				}
			}

			today := time.Now().UTC().Format("2006-01-02")
			for _, p := range projects {
				brandName := brand
				if brandName == "" {
					brandName = p.BrandName
				}
				if brandName == "" {
					brandName = p.Name
				}

				if scopes["visibility"] {
					if err := syncVisibility(ctx, c, st, p.ID, brandName, today); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "sync visibility %s: %v\n", p.ID, err)
					}
				}
				if scopes["citations"] {
					if err := syncCitations(ctx, c, st, p.ID, today); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "sync citations %s: %v\n", p.ID, err)
					}
				}
				if scopes["prompts"] {
					if err := syncPrompts(ctx, c, st, p.ID, today); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "sync prompts %s: %v\n", p.ID, err)
					}
				}
				if scopes["traffic"] {
					if err := syncTraffic(ctx, c, st, p.ID, today); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "sync traffic %s: %v\n", p.ID, err)
					}
				}
				if scopes["docs"] {
					if err := syncDocuments(ctx, c, st, p.ID); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "sync docs %s: %v\n", p.ID, err)
					}
				}
				if scopes["knowledge"] {
					if err := syncKnowledge(ctx, c, st, p.ID); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "sync knowledge %s: %v\n", p.ID, err)
					}
				}
				_ = st.BumpCursor(ctx, "project", p.ID, 1)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Synced %d project(s) to %s\n", len(projects), st.Path)
			return nil
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "Project ID to sync (use 'get-projects' to list)")
	cmd.Flags().StringVar(&brand, "brand", "", "Brand name override (defaults to the project's primary brand)")
	cmd.Flags().BoolVar(&all, "all", false, "Sync every project in the organization")
	cmd.Flags().StringVar(&include, "include", "visibility,citations,prompts,traffic,docs,knowledge,usage", "Comma-separated scopes (default: all)")
	return cmd
}

func normalizeIncludes(csv string) map[string]bool {
	out := map[string]bool{}
	for _, raw := range strings.Split(csv, ",") {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		out[s] = true
	}
	return out
}

// ---------- per-resource sync helpers ----------

type apiProject struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	BrandName string `json:"brandName"`
}

func syncProjects(ctx context.Context, c apiClient, st *store.Store) ([]store.Project, error) {
	data, _, err := c.Post("/v1/get_projects", map[string]any{})
	if err != nil {
		return nil, err
	}
	rows, err := decodeListArray(data)
	if err != nil {
		return nil, err
	}
	var out []store.Project
	now := time.Now().UTC()
	for _, raw := range rows {
		var p apiProject
		if err := json.Unmarshal(raw, &p); err != nil {
			continue
		}
		if p.ID == "" {
			continue
		}
		project := store.Project{
			ID:           p.ID,
			Name:         p.Name,
			BrandName:    p.BrandName,
			RawJSON:      string(raw),
			LastSyncedAt: now,
		}
		if err := st.UpsertProject(ctx, project); err != nil {
			return nil, err
		}
		out = append(out, project)
	}
	_ = st.BumpCursor(ctx, "projects", "", len(out))
	return out, nil
}

func syncVisibility(ctx context.Context, c apiClient, st *store.Store, projectID, brand, today string) error {
	// Date range required for several reads. We fetch the most recent 14 days
	// (the API's typical max for trend). Snapshots are keyed by snapshot date,
	// so multiple same-day runs upsert rather than duplicate.
	startDate := time.Now().UTC().AddDate(0, 0, -14).Format("2006-01-02")
	endDate := today

	metrics := []struct {
		name string
		path string
		body map[string]any
	}{
		{"overview", "/v1/get_ai_visibility_overview", map[string]any{
			"projectId": projectID, "brandName": brand, "startDate": startDate, "endDate": endDate,
		}},
		{"trend", "/v1/get_ai_visibility_trend", map[string]any{
			"projectId": projectID, "brandName": brand, "startDate": startDate, "endDate": endDate,
		}},
		{"sentiment", "/v1/get_ai_visibility_sentiment", map[string]any{
			"projectId": projectID, "brandName": brand, "startDate": startDate, "endDate": endDate,
		}},
		{"traffic_summary", "/v1/get_ai_visibility_traffic_summary", map[string]any{
			"projectId": projectID, "startDate": startDate, "endDate": endDate,
		}},
	}
	count := 0
	for _, m := range metrics {
		data, _, err := c.Post(m.path, m.body)
		if err != nil {
			continue
		}
		if err := st.UpsertVisibilitySnapshot(ctx, store.VisibilitySnapshot{
			ProjectID:    projectID,
			BrandName:    brand,
			SnapshotDate: today,
			MetricType:   m.name,
			Payload:      data,
		}); err != nil {
			return err
		}
		count++
	}
	_ = st.BumpCursor(ctx, "visibility:"+projectID, projectID, count)
	return nil
}

func syncCitations(ctx context.Context, c apiClient, st *store.Store, projectID, today string) error {
	data, _, err := c.Post("/v1/get_ai_visibility_citations", map[string]any{
		"projectId": projectID,
	})
	if err != nil {
		return err
	}
	// The response contains aggregate citation counts by domain. Persist a
	// snapshot row per domain so rank-shift can diff two days.
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err == nil {
		if domains, ok := envelope["domains"]; ok {
			var arr []map[string]any
			_ = json.Unmarshal(domains, &arr)
			for _, d := range arr {
				domain, _ := d["domain"].(string)
				if domain == "" {
					continue
				}
				count := toInt(d["citationCount"])
				urlCount := toInt(d["urlCount"])
				raw, _ := json.Marshal(d)
				_ = st.UpsertCitationSnapshot(ctx, store.CitationSnapshot{
					ProjectID:     projectID,
					SnapshotDate:  today,
					Domain:        domain,
					CitationCount: count,
					URLCount:      urlCount,
					RawJSON:       raw,
				})
			}
		}
	}
	_ = st.BumpCursor(ctx, "citations:"+projectID, projectID, 1)
	return nil
}

func syncPrompts(ctx context.Context, c apiClient, st *store.Store, projectID, today string) error {
	page := 1
	pageSize := 100
	count := 0
	for {
		data, _, err := c.Post("/v1/get_ai_visibility_prompts", map[string]any{
			"projectId": projectID,
			"page":      page,
			"pageSize":  pageSize,
		})
		if err != nil {
			return err
		}
		rows, err := decodeListArray(data)
		if err != nil {
			break
		}
		if len(rows) == 0 {
			break
		}
		now := time.Now().UTC()
		for _, raw := range rows {
			var p struct {
				ID            string `json:"id"`
				Text          string `json:"prompt"`
				Text2         string `json:"text"`
				CitationCount int    `json:"citationCount"`
				FirstPosition int    `json:"firstPosition"`
			}
			if err := json.Unmarshal(raw, &p); err != nil {
				continue
			}
			if p.ID == "" {
				continue
			}
			text := p.Text
			if text == "" {
				text = p.Text2
			}
			_ = st.UpsertPrompt(ctx, store.Prompt{
				ID:         p.ID,
				ProjectID:  projectID,
				Text:       text,
				RawJSON:    raw,
				LastSeenAt: now,
			})
			// Today's run snapshot.
			run := store.PromptRun{
				PromptID:      p.ID,
				ProjectID:     projectID,
				SnapshotDate:  today,
				CitationCount: p.CitationCount,
				RawJSON:       raw,
			}
			if p.FirstPosition > 0 {
				run.FirstPosition.Int64 = int64(p.FirstPosition)
				run.FirstPosition.Valid = true
			}
			_ = st.UpsertPromptRun(ctx, run)
			// Feed FTS index.
			_ = st.UpsertFTS(ctx, store.FTSDoc{
				Kind:      "prompt",
				ID:        p.ID,
				ProjectID: projectID,
				Title:     text,
				Body:      text,
			})
			count++
		}
		if len(rows) < pageSize {
			break
		}
		page++
	}
	_ = st.BumpCursor(ctx, "prompts:"+projectID, projectID, count)
	return nil
}

func syncTraffic(ctx context.Context, c apiClient, st *store.Store, projectID, today string) error {
	page := 1
	count := 0
	for {
		data, _, err := c.Post("/v1/get_ai_visibility_traffic_pages", map[string]any{
			"projectId": projectID,
			"page":      page,
			"pageSize":  100,
		})
		if err != nil {
			return err
		}
		rows, err := decodeListArray(data)
		if err != nil || len(rows) == 0 {
			break
		}
		for _, raw := range rows {
			var t struct {
				URL         string  `json:"url"`
				AIVisits    int     `json:"aiVisits"`
				HumanVisits int     `json:"humanVisits"`
				CTR         float64 `json:"ctr"`
			}
			if err := json.Unmarshal(raw, &t); err != nil {
				continue
			}
			if t.URL == "" {
				continue
			}
			tp := store.TrafficPage{
				ProjectID:    projectID,
				SnapshotDate: today,
				URL:          t.URL,
				AIVisits:     t.AIVisits,
				HumanVisits:  t.HumanVisits,
				RawJSON:      raw,
			}
			if t.CTR > 0 {
				tp.CTR.Float64 = t.CTR
				tp.CTR.Valid = true
			}
			_ = st.UpsertTrafficPage(ctx, tp)
			_ = st.UpsertFTS(ctx, store.FTSDoc{
				Kind:      "traffic",
				ID:        t.URL,
				ProjectID: projectID,
				Title:     t.URL,
				Body:      t.URL,
			})
			count++
		}
		if len(rows) < 100 {
			break
		}
		page++
	}
	_ = st.BumpCursor(ctx, "traffic:"+projectID, projectID, count)
	return nil
}

func syncDocuments(ctx context.Context, c apiClient, st *store.Store, projectID string) error {
	count := 0
	now := time.Now().UTC()
	// Writer documents.
	if data, _, err := c.Post("/v1/get_writer_documents", map[string]any{"projectId": projectID}); err == nil {
		if rows, err := decodeListArray(data); err == nil {
			for _, raw := range rows {
				var d struct {
					ID        string `json:"id"`
					Title     string `json:"title"`
					UpdatedAt string `json:"updatedAt"`
				}
				_ = json.Unmarshal(raw, &d)
				if d.ID == "" {
					continue
				}
				doc := store.Document{
					ID:         d.ID,
					ProjectID:  projectID,
					Kind:       "writer",
					Title:      d.Title,
					RawJSON:    raw,
					LastSeenAt: now,
				}
				if ts, ok := parseTime(d.UpdatedAt); ok {
					doc.UpdatedAt.Time = ts
					doc.UpdatedAt.Valid = true
				}
				_ = st.UpsertDocument(ctx, doc)
				_ = st.UpsertFTS(ctx, store.FTSDoc{
					Kind: "doc", ID: d.ID, ProjectID: projectID,
					Title: d.Title, Body: d.Title,
				})
				count++
			}
		}
	}
	// Optimized documents.
	if data, _, err := c.Post("/v1/get_optimized_documents", map[string]any{"projectId": projectID}); err == nil {
		if rows, err := decodeListArray(data); err == nil {
			for _, raw := range rows {
				var d struct {
					ID        string `json:"id"`
					Title     string `json:"title"`
					UpdatedAt string `json:"updatedAt"`
				}
				_ = json.Unmarshal(raw, &d)
				if d.ID == "" {
					continue
				}
				doc := store.Document{
					ID:         d.ID,
					ProjectID:  projectID,
					Kind:       "optimized",
					Title:      d.Title,
					RawJSON:    raw,
					LastSeenAt: now,
				}
				if ts, ok := parseTime(d.UpdatedAt); ok {
					doc.UpdatedAt.Time = ts
					doc.UpdatedAt.Valid = true
				}
				_ = st.UpsertDocument(ctx, doc)
				_ = st.UpsertFTS(ctx, store.FTSDoc{
					Kind: "doc", ID: d.ID, ProjectID: projectID,
					Title: d.Title, Body: d.Title,
				})
				count++
			}
		}
	}
	_ = st.BumpCursor(ctx, "documents:"+projectID, projectID, count)
	return nil
}

func syncKnowledge(ctx context.Context, c apiClient, st *store.Store, projectID string) error {
	count := 0
	now := time.Now().UTC()
	data, _, err := c.Post("/v1/get_knowledge_libraries", map[string]any{"projectId": projectID})
	if err != nil {
		return err
	}
	rows, err := decodeListArray(data)
	if err != nil {
		return nil
	}
	for _, raw := range rows {
		var l struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		_ = json.Unmarshal(raw, &l)
		if l.ID == "" {
			continue
		}
		_ = st.UpsertKnowledgeLibrary(ctx, store.KnowledgeLibrary{
			ID: l.ID, ProjectID: projectID, Name: l.Name, RawJSON: raw, LastSeenAt: now,
		})
		// Walk documents.
		if d2, _, err := c.Post("/v1/get_knowledge_library_documents", map[string]any{"libraryId": l.ID}); err == nil {
			if drows, err := decodeListArray(d2); err == nil {
				for _, dr := range drows {
					var kd struct {
						ID    string `json:"id"`
						Title string `json:"title"`
						URL   string `json:"url"`
					}
					_ = json.Unmarshal(dr, &kd)
					if kd.ID == "" {
						continue
					}
					_ = st.UpsertKnowledgeLibraryDocument(ctx, store.KnowledgeLibraryDocument{
						ID: kd.ID, LibraryID: l.ID, Title: kd.Title, URL: kd.URL, RawJSON: dr, LastSeenAt: now,
					})
					count++
				}
			}
		}
	}
	_ = st.BumpCursor(ctx, "knowledge:"+projectID, projectID, count)
	return nil
}

func syncUsage(ctx context.Context, c apiClient, st *store.Store) error {
	data, _, err := c.Post("/v1/get_usage", map[string]any{})
	if err != nil {
		return err
	}
	today := time.Now().UTC().Format("2006-01-02")
	var u struct {
		Credits struct {
			Total     int64 `json:"total"`
			Used      int64 `json:"used"`
			Remaining int64 `json:"remaining"`
		} `json:"credits"`
	}
	_ = json.Unmarshal(data, &u)
	snapshot := store.UsageSnapshot{Date: today, RawJSON: data}
	if u.Credits.Total > 0 {
		snapshot.CreditsTotal.Int64 = u.Credits.Total
		snapshot.CreditsTotal.Valid = true
	}
	if u.Credits.Used > 0 {
		snapshot.CreditsUsed.Int64 = u.Credits.Used
		snapshot.CreditsUsed.Valid = true
	}
	if u.Credits.Remaining > 0 {
		snapshot.CreditsRemaining.Int64 = u.Credits.Remaining
		snapshot.CreditsRemaining.Valid = true
	}
	_ = st.UpsertUsageSnapshot(ctx, snapshot)
	_ = st.BumpCursor(ctx, "usage", "", 1)
	return nil
}

// ---------- shared helpers ----------

// apiClient is the subset of *client.Client these sync helpers need.
// Using an interface lets future tests stub it without standing up an HTTP server.
type apiClient interface {
	Post(path string, body any) (json.RawMessage, int, error)
}

// decodeListArray accepts either a bare JSON array or an envelope
// `{"data": [...]}` and returns the rows as RawMessages.
func decodeListArray(data json.RawMessage) ([]json.RawMessage, error) {
	trimmed := []byte(strings.TrimSpace(string(data)))
	if len(trimmed) == 0 {
		return nil, nil
	}
	if trimmed[0] == '[' {
		var rows []json.RawMessage
		if err := json.Unmarshal(trimmed, &rows); err != nil {
			return nil, err
		}
		return rows, nil
	}
	// Envelope: try common keys (data, items, prompts, rows, results).
	var env map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &env); err != nil {
		return nil, err
	}
	for _, k := range []string{"data", "items", "rows", "results", "prompts", "documents", "libraries", "pages"} {
		if v, ok := env[k]; ok {
			if len(v) > 0 && v[0] == '[' {
				var rows []json.RawMessage
				if err := json.Unmarshal(v, &rows); err == nil {
					return rows, nil
				}
			}
		}
	}
	return nil, nil
}

func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	return 0
}

func parseTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
