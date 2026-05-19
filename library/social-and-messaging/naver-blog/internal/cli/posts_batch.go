// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

// Hand-written `batch` command. Reads a file of Naver Blog URLs (CSV
// with a `url` column, JSON array of strings, or newline-delimited
// URLs), canonicalizes each, runs the same per-post fetch pipeline
// as `posts get`, and emits one row per input URL in the original
// input order.
//
// Concurrency: per-URL fetches fan out via the press's standard
// FanoutRun helper, capped by --concurrency (default 5). Engagement
// enrichment uses the shared primitive so reaction likes are batched
// and cbox/PostView lookups share the same concurrency cap.

package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/engagement"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/naverurl"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/lib/postparse"
)

// batchInput is one parsed entry from the file: the raw string the
// user provided, plus the canonicalized (blog_id, log_no) pair if
// parsing succeeded. ParseErr is non-empty for unparseable inputs.
type batchInput struct {
	Raw      string
	BlogID   string
	LogNo    string
	URL      string
	ParseErr string
}

// batchRow is one element of the output array, in input order.
type batchRow struct {
	InputURL       string    `json:"input_url"`
	URL            string    `json:"url,omitempty"`
	BlogID         string    `json:"blog_id,omitempty"`
	LogNo          string    `json:"log_no,omitempty"`
	Title          string    `json:"title,omitempty"`
	Likes          *int      `json:"likes,omitempty"`
	Comments       *int      `json:"comments,omitempty"`
	Tags           []string  `json:"tags,omitempty"`
	PublishedAtUTC time.Time `json:"published_at_utc,omitempty"`
	Error          string    `json:"error,omitempty"`
	ErrorCode      string    `json:"error_code,omitempty"`
}

func newBatchCmd(flags *rootFlags) *cobra.Command {
	var (
		flagFromFile string
		flagConc     int
		flagPacing   time.Duration
		flagEnrich   bool
	)

	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Backfill engagement for a file of Naver Blog URLs (CSV, JSON, or newline-delimited).",
		Long: `Read a file of Naver Blog post URLs, canonicalize each, fetch each post (title, tags, publish date, likes, comments), and emit one row per input URL in input order.

Supported file shapes (auto-detected via extension + first-byte sniff):
  - CSV (.csv): a column named "url" carries the URL (case-insensitive). First row may be a header.
  - JSON (.json): an array of strings, e.g. ["https://m.blog.naver.com/...","..."]
  - Anything else: one URL per line. '#' starts a comment.

Each URL is normalized via naverurl.CanonicalKey, which accepts any of the three Naver post URL shapes (m.blog.naver.com, blog.naver.com short, PostView.naver). Unparseable inputs appear in the output as a row with input_url + error + error_code="invalid_url" so the caller sees exactly which entries were skipped.

--concurrency caps the per-URL fetch fanout (default 5; the reaction API call is a single batched request regardless).
--pacing inserts a delay between fetches inside each worker for politeness.
--enrich-engagement controls whether likes/comments are fetched (default true; pass --enrich-engagement=false for a fast URL-canonicalize-only pass).`,
		Example: `  naver-blog-pp-cli batch --from-file urls.csv
  naver-blog-pp-cli batch --from-file urls.json --concurrency 3 --pacing 2s
  naver-blog-pp-cli batch --from-file urls.txt --enrich-engagement=false`,
		Annotations: map[string]string{
			"pp:endpoint":   "posts.batch",
			"pp:method":     "GET",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Honor --dry-run before required-flag validation so verify
			// dry-run probes succeed without forcing a sample input file.
			if dryRunOK(flags) {
				return nil
			}
			if strings.TrimSpace(flagFromFile) == "" {
				if flags.asJSON {
					_ = printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"error": "from-file is required",
						"usage": fmt.Sprintf("%s --from-file <path>", cmd.CommandPath()),
					}, flags)
				}
				return usageErr(fmt.Errorf("required flag --from-file not set"))
			}
			if flagConc <= 0 {
				flagConc = 5
			}
			inputs, err := loadBatchURLs(flagFromFile)
			if err != nil {
				return usageErr(err)
			}
			if len(inputs) == 0 {
				return usageErr(fmt.Errorf("no URLs parsed from %q", flagFromFile))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			rows := runBatchFetch(ctx, c, inputs, flagEnrich, flagConc, flagPacing)
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&flagFromFile, "from-file", "", "Path to a CSV / JSON / newline-delimited file of post URLs. Required.")
	cmd.Flags().IntVar(&flagConc, "concurrency", 5, "Max concurrent per-URL fetches")
	cmd.Flags().DurationVar(&flagPacing, "pacing", time.Second, "Sleep between fetches inside each worker (politeness pacing)")
	cmd.Flags().BoolVar(&flagEnrich, "enrich-engagement", true, "Fetch likes/comments per URL. Pass --enrich-engagement=false to skip.")
	return cmd
}

// loadBatchURLs auto-detects file shape and returns one batchInput per
// entry, preserving the file's order. Parse errors are recorded on the
// individual entry rather than failing the whole load so partially
// invalid files still produce a usable per-row output.
func loadBatchURLs(path string) ([]batchInput, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".csv"):
		return parseBatchCSV(data)
	case strings.HasSuffix(lower, ".json"):
		return parseBatchJSON(data)
	}
	// Sniff: if the trimmed content starts with '[' it's JSON.
	//  strips a leading UTF-8 byte-order mark; some CSV/JSON
	// exports prepend it and the strict json.Unmarshal would reject
	// the document otherwise.
	trimmed := strings.TrimLeft(string(data), " \t\r\n\ufeff")
	if strings.HasPrefix(trimmed, "[") {
		return parseBatchJSON(data)
	}
	// Default: newline-delimited URLs.
	return parseBatchLines(data), nil
}

// parseBatchCSV reads a CSV file and pulls the column named "url"
// (case-insensitive). If no header row is present, the first column
// is taken as the URL.
func parseBatchCSV(data []byte) ([]batchInput, error) {
	cr := csv.NewReader(strings.NewReader(string(data)))
	cr.FieldsPerRecord = -1
	records, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parsing CSV: %w", err)
	}
	if len(records) == 0 {
		return nil, nil
	}
	urlCol := -1
	startRow := 0
	for i, cell := range records[0] {
		if strings.EqualFold(strings.TrimSpace(cell), "url") {
			urlCol = i
			startRow = 1
			break
		}
	}
	if urlCol == -1 {
		urlCol = 0 // no header — take first column
	}
	out := make([]batchInput, 0, len(records)-startRow)
	for _, rec := range records[startRow:] {
		if urlCol >= len(rec) {
			continue
		}
		raw := strings.TrimSpace(rec[urlCol])
		if raw == "" {
			continue
		}
		out = append(out, makeBatchInput(raw))
	}
	return out, nil
}

// parseBatchJSON expects an array of strings.
func parseBatchJSON(data []byte) ([]batchInput, error) {
	var urls []string
	if err := json.Unmarshal(data, &urls); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w (expected array of strings)", err)
	}
	out := make([]batchInput, 0, len(urls))
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		out = append(out, makeBatchInput(u))
	}
	return out, nil
}

// parseBatchLines reads one URL per line, skipping blanks and '#'
// comments.
func parseBatchLines(data []byte) []batchInput {
	out := make([]batchInput, 0)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, makeBatchInput(line))
	}
	return out
}

// makeBatchInput runs canonicalization once at file-load time so the
// per-URL fanout doesn't repeat the work and so unparseable rows can
// be surfaced before any HTTP traffic.
func makeBatchInput(raw string) batchInput {
	blogID, logNo, ok := naverurl.CanonicalKey(raw)
	bi := batchInput{Raw: raw}
	if !ok {
		bi.ParseErr = "could not canonicalize as Naver Blog URL"
		return bi
	}
	bi.BlogID = blogID
	bi.LogNo = logNo
	bi.URL = naverurl.MobileURL(blogID, logNo)
	return bi
}

// runBatchFetch fans out the per-URL fetches and returns rows in input
// order. Engagement is batched after successful mobile HTML fetches
// so deleted/private posts don't pollute the engagement request set.
func runBatchFetch(ctx context.Context, c *client.Client, inputs []batchInput, enrich bool, concurrency int, pacing time.Duration) []batchRow {
	rows := make([]batchRow, len(inputs))

	// First: seed rows for unparseable inputs and collect indices that
	// need real fetches.
	type fetchSlot struct {
		idx   int
		input batchInput
	}
	pending := make([]fetchSlot, 0, len(inputs))
	for i, in := range inputs {
		rows[i].InputURL = in.Raw
		if in.ParseErr != "" {
			rows[i].Error = in.ParseErr
			rows[i].ErrorCode = "invalid_url"
			continue
		}
		rows[i].URL = in.URL
		rows[i].BlogID = in.BlogID
		rows[i].LogNo = in.LogNo
		pending = append(pending, fetchSlot{idx: i, input: in})
	}
	if len(pending) == 0 {
		return rows
	}

	// Per-URL HTML fetch + parse (no reaction API yet — that's batched
	// after the fanout).
	_, errs := cliutil.FanoutRun(
		ctx,
		pending,
		func(s fetchSlot) string { return s.input.URL },
		func(ctx context.Context, s fetchSlot) (struct{}, error) {
			if pacing > 0 {
				select {
				case <-ctx.Done():
					return struct{}{}, ctx.Err()
				case <-time.After(pacing):
				}
			}
			mobileHTML, err := getHTMLBytes(c, s.input.URL)
			if err != nil {
				rows[s.idx].Error = "fetch_mobile: " + err.Error()
				rows[s.idx].ErrorCode = "fetch_failed"
				return struct{}{}, err
			}
			meta, err := postparse.ParseMobilePost(mobileHTML)
			if err != nil {
				rows[s.idx].Error = "parse_mobile: " + err.Error()
				rows[s.idx].ErrorCode = "parse_failed"
				return struct{}{}, err
			}
			rows[s.idx].Title = meta.Title
			rows[s.idx].Tags = meta.Tags
			return struct{}{}, nil
		},
		cliutil.WithConcurrency(concurrency),
	)
	cliutil.FanoutReportErrors(os.Stderr, errs)

	if enrich {
		keys := make([]engagement.BatchKey, 0, len(pending))
		slotForKey := make([]fetchSlot, 0, len(pending))
		for _, s := range pending {
			if rows[s.idx].ErrorCode != "" {
				continue
			}
			keys = append(keys, engagement.BatchKey{BlogID: s.input.BlogID, LogNo: s.input.LogNo})
			slotForKey = append(slotForKey, s)
		}
		if len(keys) > 0 {
			snaps := engagement.FetchBatch(ctx, c, keys, concurrency)
			for i, snap := range snaps {
				s := slotForKey[i]
				if snap.Likes != nil {
					rows[s.idx].Likes = snap.Likes
				}
				if snap.CommentsSource != "" {
					n := snap.Comments
					rows[s.idx].Comments = &n
				}
				if !snap.PublishedAtUTC.IsZero() {
					rows[s.idx].PublishedAtUTC = snap.PublishedAtUTC
				}
				for _, err := range snap.Errors {
					fmt.Fprintf(os.Stderr, "warn: batch engagement failed for %s: %v\n", rows[s.idx].URL, err)
				}
			}
		}
	}
	return rows
}

// silence unused-import linters for generated builds that trim helpers.
var _ io.Reader = (*strings.Reader)(nil)
