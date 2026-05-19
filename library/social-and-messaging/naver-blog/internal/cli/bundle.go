// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

// Hand-written `bundle` command. Reads a YAML/JSONL list of search
// queries (kind=keyword or kind=hashtag), runs each through the
// existing SERP path, dedupes by canonical URL across queries, and
// emits one row per unique post. Optional --enrich-engagement
// fetches likes, cbox comment counts, and publish dates for each
// unique URL using the shared engagement primitive.
//
// The command is a pure data operation. Scheduling, cron expressions,
// and integration with external runners (Hermes, launchd, GitHub
// Actions) live outside the CLI — wrap this command in whatever
// scheduler you use. The legacy alias `cron` is kept for backward
// compatibility with older callers.

package cli

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/engagement"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/serpparse"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/store"
)

// bundleQuery is one entry in the queries file. RequireAll only
// applies to kind=hashtag.
type bundleQuery struct {
	Query      string `json:"query" yaml:"query"`
	Kind       string `json:"kind" yaml:"kind"`
	RequireAll bool   `json:"require_all" yaml:"require_all"`
}

// bundleRow is one output row per unique URL.
type bundleRow struct {
	URL            string   `json:"url"`
	BlogID         string   `json:"blog_id"`
	LogNo          string   `json:"log_no"`
	Title          string   `json:"title,omitempty"`
	Snippet        string   `json:"snippet,omitempty"`
	MatchedQueries []string `json:"matched_queries"`
	Likes          *int     `json:"likes,omitempty"`
	Comments       *int     `json:"comments,omitempty"`
	// PublishedAtUTC is the post's publish date as RFC3339, populated
	// from PostView.naver under --enrich-engagement. Empty when
	// enrichment didn't run or the page lacked a parseable date span.
	// Downstream sheet writers convert to a local-time M/D and YYYY-MM.
	PublishedAtUTC string `json:"published_at_utc,omitempty"`
}

func newBundleCmd(flags *rootFlags) *cobra.Command {
	var (
		flagFormat string
		flagEnrich bool
	)

	cmd := &cobra.Command{
		Use:     "bundle <queries-file>",
		Aliases: []string{"cron"},
		Short:   "Run a YAML/JSONL list of keyword + hashtag queries, dedupe, optionally enrich.",
		Long: `Run a list of queries against Naver's mobile SERP and emit one row per unique post (canonical URL). The queries file may be YAML (extension .yaml/.yml) or JSONL (default).

File shape (YAML):
  queries:
    - query: "칠리 협찬"
      kind: keyword
    - query: "칠리,여성청결제"
      kind: hashtag
      require_all: true

File shape (JSONL):
  {"query": "칠리 협찬", "kind": "keyword"}
  {"query": "칠리,여성청결제", "kind": "hashtag", "require_all": true}

Each query is run via the same SERP path used by 'find posts' (kind=keyword) or 'find hashtag' (kind=hashtag). Results are deduplicated by canonical URL — a single URL matching multiple queries appears once with matched_queries listing all of them.

--enrich-engagement adds the like/comment counts and publish date for every unique URL (extra HTTP calls — reaction API for likes, cbox for comments, PostView for publish date). Default off so the dedupe + URL list is cheap.

--format selects the output stream: jsonl (default), tsv, csv.

Scheduling, cron expressions, and integration with external runners (Hermes, launchd, GitHub Actions) live outside the CLI. Wrap this command in whatever scheduler you use. The legacy alias 'cron' is kept for backward compatibility.`,
		Example: `  naver-blog-pp-cli bundle queries.yaml
  naver-blog-pp-cli bundle queries.jsonl --enrich-engagement --format tsv`,
		Annotations: map[string]string{
			"pp:endpoint":   "bundle.run",
			"pp:method":     "GET",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Honor --dry-run before any arg/flag validation so verify
			// dry-run probes succeed without forcing a sample queries file.
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			// Validate --format before any IO; this catches typos cheaply.
			switch strings.ToLower(flagFormat) {
			case "", "jsonl", "tsv", "csv":
				// valid
			default:
				return usageErr(fmt.Errorf("invalid --format %q: expected jsonl|tsv|csv", flagFormat))
			}
			path := args[0]
			queries, err := loadBundleQueries(path)
			if err != nil {
				return usageErr(err)
			}
			if len(queries) == 0 {
				return usageErr(fmt.Errorf("no queries parsed from %q", path))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			rows, err := runBundleQueries(ctx, c, queries, flagEnrich)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return emitBundleRows(cmd.OutOrStdout(), rows, flagFormat, flags)
		},
	}
	cmd.Flags().StringVar(&flagFormat, "format", "jsonl", "Output format: jsonl (default), tsv, csv")
	cmd.Flags().BoolVar(&flagEnrich, "enrich-engagement", false, "Add likes/comments/publish-date per URL (extra HTTP calls)")
	return cmd
}

// loadBundleQueries dispatches on file extension. YAML is parsed via a
// purpose-built reader (we don't pull yaml.v3 just for this); JSONL
// uses encoding/json line-by-line.
func loadBundleQueries(path string) ([]bundleQuery, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml") {
		return parseBundleYAML(data)
	}
	return parseBundleJSONL(data)
}

// parseBundleJSONL parses one JSON object per line. Blank lines and
// '#' comment lines are skipped.
func parseBundleJSONL(data []byte) ([]bundleQuery, error) {
	out := make([]bundleQuery, 0)
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var q bundleQuery
		if err := json.Unmarshal([]byte(line), &q); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
		}
		if q.Kind == "" {
			q.Kind = "keyword"
		}
		out = append(out, q)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scanning JSONL: %w", err)
	}
	return out, nil
}

// parseBundleYAML is a minimal hand-rolled YAML reader for the exact
// shape this command accepts. We deliberately don't take a yaml.v3
// dependency for a 6-line schema.
func parseBundleYAML(data []byte) ([]bundleQuery, error) {
	out := make([]bundleQuery, 0)
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	inQueries := false
	var cur *bundleQuery
	flush := func() {
		if cur != nil && cur.Query != "" {
			if cur.Kind == "" {
				cur.Kind = "keyword"
			}
			out = append(out, *cur)
		}
		cur = nil
	}
	lineNo := 0
	for sc.Scan() {
		lineNo++
		raw := sc.Text()
		// Strip whole-line comments. We don't try to handle inline
		// comments (those would need quote-aware parsing).
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !inQueries {
			if strings.HasPrefix(trimmed, "queries:") {
				inQueries = true
			}
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			flush()
			cur = &bundleQuery{}
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			if trimmed == "" {
				continue
			}
		}
		if cur == nil {
			return nil, fmt.Errorf("line %d: stray key outside a list item: %q", lineNo, raw)
		}
		key, val, ok := strings.Cut(trimmed, ":")
		if !ok {
			return nil, fmt.Errorf("line %d: expected key:value, got %q", lineNo, raw)
		}
		key = strings.TrimSpace(key)
		val = unquoteYAMLValue(val)
		switch key {
		case "query":
			cur.Query = val
		case "kind":
			cur.Kind = val
		case "require_all":
			cur.RequireAll = strings.EqualFold(val, "true") || val == "1" || strings.EqualFold(val, "yes")
		default:
			// Tolerate unknown keys to keep the parser forward-compat.
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scanning YAML: %w", err)
	}
	flush()
	return out, nil
}

// unquoteYAMLValue trims surrounding quotes and whitespace.
func unquoteYAMLValue(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}
	return v
}

// runBundleQueries executes each query, merges by canonical URL, and
// optionally enriches the unique URLs with engagement.
func runBundleQueries(ctx context.Context, c *client.Client, queries []bundleQuery, enrich bool) ([]bundleRow, error) {
	type urlEntry struct {
		row     bundleRow
		queries map[string]bool
	}
	state := make(map[string]*urlEntry)
	order := make([]string, 0)
	for _, q := range queries {
		hits, err := runOneBundleQuery(ctx, c, q)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: query %q (%s) failed: %v\n", q.Query, q.Kind, err)
			continue
		}
		label := q.Kind + ":" + q.Query
		for _, h := range hits {
			canonURL := h.URL
			if e, ok := state[canonURL]; ok {
				e.queries[label] = true
				continue
			}
			state[canonURL] = &urlEntry{
				row: bundleRow{
					URL:     canonURL,
					BlogID:  h.BlogID,
					LogNo:   h.LogNo,
					Title:   h.Title,
					Snippet: h.Snippet,
				},
				queries: map[string]bool{label: true},
			}
			order = append(order, canonURL)
		}
	}
	rows := make([]bundleRow, 0, len(order))
	for _, u := range order {
		e := state[u]
		matched := make([]string, 0, len(e.queries))
		for q := range e.queries {
			matched = append(matched, q)
		}
		sort.Strings(matched)
		e.row.MatchedQueries = matched
		rows = append(rows, e.row)
	}
	if enrich {
		enrichBundleEngagement(ctx, c, rows)
	}
	return rows, nil
}

// runOneBundleQuery dispatches to the appropriate SERP path. Hashtag
// queries with comma-separated tags + require_all use the same
// fetch-and-confirm path as `find hashtag --require-all`; otherwise
// they fall through to the union path.
func runOneBundleQuery(ctx context.Context, c *client.Client, q bundleQuery) ([]serpparse.SearchResult, error) {
	switch strings.ToLower(q.Kind) {
	case "", "keyword":
		return runSERPSearch(ctx, c, q.Query, "m_view", 0)
	case "hashtag":
		tags := splitTags(q.Query)
		if len(tags) == 0 {
			return nil, fmt.Errorf("hashtag query %q parsed to zero tags", q.Query)
		}
		if q.RequireAll {
			return runHashtagIntersection(ctx, c, tags, "m_view", 0)
		}
		return runHashtagUnion(ctx, c, tags, "m_view", 0)
	default:
		return nil, fmt.Errorf("unknown query kind %q (want keyword|hashtag)", q.Kind)
	}
}

// enrichBundleEngagement uses the shared engagement primitive to fetch
// likes, cbox comment counts, and PostView publish dates. All failures
// are warnings to stderr — the row keeps url/title/etc.
func enrichBundleEngagement(ctx context.Context, c *client.Client, rows []bundleRow) {
	if len(rows) == 0 {
		return
	}
	var cacheDB *store.Store
	if !cliutil.IsVerifyEnv() {
		db, err := store.OpenWithContext(ctx, defaultDBPath("naver-blog-pp-cli"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: bundle cache hydration skipped: opening db: %v\n", err)
		} else {
			cacheDB = db
			defer cacheDB.Close()
		}
	}
	keys := make([]engagement.BatchKey, 0, len(rows))
	for _, r := range rows {
		keys = append(keys, engagement.BatchKey{BlogID: r.BlogID, LogNo: r.LogNo})
	}
	snaps := engagement.FetchBatch(ctx, c, keys, 5)
	for i := range rows {
		snap := snaps[i]
		if snap.Likes != nil {
			rows[i].Likes = snap.Likes
		}
		if snap.CommentsSource != "" {
			n := snap.Comments
			rows[i].Comments = &n
		}
		if !snap.PublishedAtUTC.IsZero() {
			rows[i].PublishedAtUTC = snap.PublishedAtUTC.Format(time.RFC3339)
		}
		for _, err := range snap.Errors {
			fmt.Fprintf(os.Stderr, "warn: bundle engagement failed for %s: %v\n", rows[i].URL, err)
		}
	}
	if cacheDB != nil {
		for i := range rows {
			if err := cacheBundleRow(cacheDB, rows[i]); err != nil {
				fmt.Fprintf(os.Stderr, "warn: bundle cache hydration failed for %s: %v\n", rows[i].URL, err)
			}
		}
	}
}

func cacheBundleRow(db *store.Store, row bundleRow) error {
	if strings.TrimSpace(row.Title) == "" {
		return nil
	}
	obj := map[string]any{
		"id":        row.URL,
		"url":       row.URL,
		"blog_id":   row.BlogID,
		"log_no":    row.LogNo,
		"title":     row.Title,
		"body_text": row.Snippet,
	}
	if tags := tagsFromBundleMatches(row.MatchedQueries); tags != "" {
		obj["tags"] = tags
	}
	if row.Likes != nil {
		obj["likes"] = *row.Likes
	}
	if row.Comments != nil {
		obj["comments"] = *row.Comments
	}
	if row.PublishedAtUTC != "" {
		obj["published_at_utc"] = row.PublishedAtUTC
	}
	return upsertPostCacheObject(db, obj)
}

func tagsFromBundleMatches(matches []string) string {
	var tags []string
	for _, match := range matches {
		kind, query, ok := strings.Cut(match, ":")
		if !ok || !strings.EqualFold(kind, "hashtag") {
			continue
		}
		tags = append(tags, splitTags(query)...)
	}
	return joinPostTags(tags)
}

// emitBundleRows writes the rows in the requested format.
//
// --format jsonl emits one JSON object per line (true JSON Lines) so
// streaming consumers (jq -c ., awk per-line) can parse incrementally.
// --json (the global flag) overrides format and emits a single pretty
// JSON array — this is the agent-friendly shape that honors
// --select / --compact via printJSONFiltered.
// TSV/CSV emit a fixed-column shape suitable for Google Sheets import.
func emitBundleRows(w io.Writer, rows []bundleRow, format string, flags *rootFlags) error {
	if flags.asJSON {
		return printJSONFiltered(w, rows, flags)
	}
	if format == "" || strings.EqualFold(format, "jsonl") {
		return writeBundleJSONL(w, rows)
	}
	if strings.EqualFold(format, "tsv") {
		return writeBundleTSV(w, rows)
	}
	if strings.EqualFold(format, "csv") {
		return writeBundleCSV(w, rows)
	}
	return fmt.Errorf("unknown format %q", format)
}

// writeBundleJSONL emits one JSON object per line. Distinct from the
// printJSONFiltered path used by --json: this one is for downstream
// pipelines (jq -c, awk, sheets-import scripts) that read line-by-line
// and don't want a wrapping `[ … ]`. Documented shape: `--format jsonl`.
func writeBundleJSONL(w io.Writer, rows []bundleRow) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for i := range rows {
		if err := enc.Encode(rows[i]); err != nil {
			return err
		}
	}
	return nil
}

// writeBundleTSV writes a TSV with a fixed header row.
func writeBundleTSV(w io.Writer, rows []bundleRow) error {
	if _, err := io.WriteString(w, "url\tblog_id\tlog_no\ttitle\tsnippet\tmatched_queries\tlikes\tcomments\tpublished_at_utc\n"); err != nil {
		return err
	}
	for _, r := range rows {
		line := strings.Join([]string{
			r.URL,
			r.BlogID,
			r.LogNo,
			sanitizeTSV(r.Title),
			sanitizeTSV(r.Snippet),
			sanitizeTSV(strings.Join(r.MatchedQueries, "|")),
			intPtrStr(r.Likes),
			intPtrStr(r.Comments),
			r.PublishedAtUTC,
		}, "\t") + "\n"
		if _, err := io.WriteString(w, line); err != nil {
			return err
		}
	}
	return nil
}

// writeBundleCSV writes a CSV with the same column shape as the TSV
// emitter, properly quoting values that contain commas/newlines.
func writeBundleCSV(w io.Writer, rows []bundleRow) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"url", "blog_id", "log_no", "title", "snippet", "matched_queries", "likes", "comments", "published_at_utc"}); err != nil {
		return err
	}
	for _, r := range rows {
		row := []string{
			r.URL,
			r.BlogID,
			r.LogNo,
			r.Title,
			r.Snippet,
			strings.Join(r.MatchedQueries, "|"),
			intPtrStr(r.Likes),
			intPtrStr(r.Comments),
			r.PublishedAtUTC,
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// sanitizeTSV strips characters that break a one-row-per-line TSV
// stream: newlines and embedded tabs.
func sanitizeTSV(s string) string {
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

// intPtrStr renders a nullable int as "" or the int formatted.
func intPtrStr(p *int) string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%d", *p)
}
