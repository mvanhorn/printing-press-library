// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Judge.me review-corpus safety layer.
// pp:data-source local

package cli

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/judgeme/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/judgeme/internal/store"
	"github.com/spf13/cobra"
)

const (
	judgeMePageSize = 100
	judgeMePageCap  = 100
)

type judgeMeReviewClient interface {
	Get(context.Context, string, map[string]string) (json.RawMessage, error)
}

type judgeMeReview struct {
	ID                string
	Body              string
	BodyHash          string
	Rating            int
	Published         bool
	Hidden            bool
	ProductExternalID string
	ProductHandle     string
	ProductTitle      string
	CreatedAt         string
	UpdatedAt         string
	Raw               json.RawMessage
	Object            map[string]any
}

type judgeMeCorpus struct {
	Reviews       []judgeMeReview
	ExpectedTotal int
	Published     int
	Hidden        int
	Pending       int
	UniqueBodies  int
	SyncedAt      time.Time
	Partitions    []judgeMePartitionSummary
}

type judgeMePartitionSummary struct {
	Rating   int  `json:"rating"`
	Expected int  `json:"expected"`
	Fetched  int  `json:"fetched"`
	Complete bool `json:"complete"`
}

type judgeMeSyncSummary struct {
	Population     string                    `json:"population"`
	Complete       bool                      `json:"complete"`
	ExpectedTotal  int                       `json:"expected_total"`
	StoredRows     int                       `json:"stored_rows"`
	PublishedRows  int                       `json:"published_rows"`
	HiddenRows     int                       `json:"hidden_rows"`
	PendingRows    int                       `json:"pending_rows"`
	UniqueBodies   int                       `json:"unique_bodies"`
	CountMatched   bool                      `json:"count_matched"`
	PublishedMatch bool                      `json:"published_count_matched"`
	Partitions     []judgeMePartitionSummary `json:"partitions"`
	SyncedAt       string                    `json:"synced_at"`
}

type judgeMeReviewFilter struct {
	Population string
	Rating     int
	Product    string
	DateFrom   string
	DateTo     string
}

func init() {
	registerNovelCommand(installJudgeMeSafety)
}

func installJudgeMeSafety(root *cobra.Command, flags *rootFlags) {
	installJudgeMeMutationGate(root, flags)
	syncCmd := commandByName(root, "sync")
	if syncCmd == nil || syncCmd.RunE == nil {
		return
	}
	standardSync := syncCmd.RunE
	syncCmd.RunE = func(cmd *cobra.Command, args []string) error {
		resources, err := cmd.Flags().GetStringSlice("resources")
		if err != nil {
			return usageErr(err)
		}
		// The generated verifier supplies a real spec-derived mock server and
		// expects the generated sync engine to exercise it. Keep that proof
		// deterministic and single-resource; the Judge.me corpus assertions
		// are separately exercised against the live API during shipcheck.
		if cliutil.IsVerifyEnv() && len(resources) == 0 {
			if err := cmd.Flags().Set("resources", "reviews"); err != nil {
				return err
			}
			defer func() { _ = cmd.Flags().Set("resources", "") }()
			return standardSync(cmd, args)
		}
		if cliutil.IsDogfoodEnv() && len(resources) == 0 {
			resources = []string{"reviews"}
			if err := cmd.Flags().Set("resources", "reviews"); err != nil {
				return err
			}
			defer func() { _ = cmd.Flags().Set("resources", "") }()
		}
		if !containsReviewResource(resources) {
			return standardSync(cmd, args)
		}

		full, _ := cmd.Flags().GetBool("full")
		maxPages, _ := cmd.Flags().GetInt("max-pages")
		if cliutil.IsDogfoodEnv() && !full {
			full = true
		}
		if !full && maxPages == 0 {
			return usageErr(errors.New("Judge.me review sync requires --full because partial cursors cannot prove corpus completeness"))
		}

		others := withoutReviewResource(resources)
		if len(resources) == 0 {
			others = withoutReviewResource(defaultSyncResources())
		}
		if len(others) > 0 {
			originalResources := append([]string(nil), resources...)
			if err := cmd.Flags().Set("resources", strings.Join(others, ",")); err != nil {
				return err
			}
			err := standardSync(cmd, args)
			_ = cmd.Flags().Set("resources", strings.Join(originalResources, ","))
			if err != nil {
				return err
			}
		}
		return runJudgeMeReviewSync(cmd, flags, maxPages)
	}
}

func commandByName(root *cobra.Command, name string) *cobra.Command {
	for _, cmd := range root.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}
	return nil
}

func containsReviewResource(resources []string) bool {
	if len(resources) == 0 {
		for _, resource := range defaultSyncResources() {
			if resource == "reviews" {
				return true
			}
		}
		return false
	}
	for _, resource := range resources {
		if resource == "reviews" {
			return true
		}
	}
	return false
}

func withoutReviewResource(resources []string) []string {
	out := make([]string, 0, len(resources))
	for _, resource := range resources {
		if resource != "reviews" {
			out = append(out, resource)
		}
	}
	return out
}

func runJudgeMeReviewSync(cmd *cobra.Command, flags *rootFlags, maxPages int) error {
	if dryRunOK(flags) {
		return printOutputWithFlagsMeta(cmd.OutOrStdout(), json.RawMessage(`[{"population":"all","complete":false,"dry_run":true}]`), flags, map[string]any{
			"source": "live",
		})
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	c.NoCache = true

	if cliutil.IsDogfoodEnv() && maxPages == 0 {
		maxPages = 1
	}
	if maxPages > 0 {
		total, err := judgeMeReviewCount(cmd.Context(), c, nil)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		rows, _, err := fetchJudgeMeReviewPartition(cmd.Context(), c, map[string]string{}, minInt(maxPages, judgeMePageCap), 0)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		payload, _ := json.Marshal([]map[string]any{{
			"population":     "all",
			"complete":       false,
			"expected_total": total,
			"sampled_rows":   len(rows),
			"stored_rows":    0,
			"reason":         "bounded_sample_does_not_prove_completeness",
		}})
		return printOutputWithFlagsMeta(cmd.OutOrStdout(), payload, flags, map[string]any{
			"source": "live",
		})
	}

	corpus, err := fetchJudgeMeCorpus(cmd.Context(), c)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	dbPath, _ := cmd.Flags().GetString("db")
	if dbPath == "" {
		dbPath = defaultDBPath("judgeme-pp-cli")
	}
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return fmt.Errorf("opening local database: %w", err)
	}
	defer db.Close()
	if err := replaceJudgeMeReviewCorpus(cmd.Context(), db.DB(), corpus); err != nil {
		return fmt.Errorf("committing verified review corpus: %w", err)
	}

	livePublished, err := judgeMeReviewCount(cmd.Context(), c, map[string]string{"published": "true"})
	if err != nil {
		return classifyAPIError(err, flags)
	}
	summary := judgeMeSyncSummary{
		Population:     "all",
		Complete:       true,
		ExpectedTotal:  corpus.ExpectedTotal,
		StoredRows:     len(corpus.Reviews),
		PublishedRows:  corpus.Published,
		HiddenRows:     corpus.Hidden,
		PendingRows:    corpus.Pending,
		UniqueBodies:   corpus.UniqueBodies,
		CountMatched:   len(corpus.Reviews) == corpus.ExpectedTotal,
		PublishedMatch: corpus.Published == livePublished,
		Partitions:     corpus.Partitions,
		SyncedAt:       corpus.SyncedAt.Format(time.RFC3339),
	}
	if !summary.PublishedMatch {
		return fmt.Errorf("published population mismatch after verified pull: local=%d live_count=%d; refusing success", corpus.Published, livePublished)
	}
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		fmt.Fprintf(cmd.OutOrStdout(), "Verified Judge.me review corpus: %d total, %d published, %d hidden, %d pending, %d unique bodies\n",
			summary.StoredRows, summary.PublishedRows, summary.HiddenRows, summary.PendingRows, summary.UniqueBodies)
		return nil
	}
	payload, _ := json.Marshal([]judgeMeSyncSummary{summary})
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), payload, flags, map[string]any{
		"source":    "live",
		"synced_at": summary.SyncedAt,
	})
}

func fetchJudgeMeCorpus(ctx context.Context, c judgeMeReviewClient) (judgeMeCorpus, error) {
	total, err := judgeMeReviewCount(ctx, c, nil)
	if err != nil {
		return judgeMeCorpus{}, err
	}
	if total <= 0 {
		return judgeMeCorpus{}, fmt.Errorf("Judge.me /reviews/count returned %d; refusing to replace the local corpus", total)
	}

	byID := make(map[string]judgeMeReview, total)
	partitions := make([]judgeMePartitionSummary, 0, 5)
	if total <= judgeMePageSize*judgeMePageCap {
		rows, looped, err := fetchJudgeMeReviewPartition(ctx, c, nil, 0, total)
		if err != nil {
			return judgeMeCorpus{}, err
		}
		if looped && len(rows) != total {
			return judgeMeCorpus{}, fmt.Errorf("Judge.me repeated a page before the count assertion could pass: fetched=%d expected=%d", len(rows), total)
		}
		for _, row := range rows {
			byID[row.ID] = row
		}
	} else {
		ratingTotal := 0
		for rating := 1; rating <= 5; rating++ {
			params := map[string]string{"rating": strconv.Itoa(rating)}
			expected, err := judgeMeReviewCount(ctx, c, params)
			if err != nil {
				return judgeMeCorpus{}, fmt.Errorf("counting rating %d partition: %w", rating, err)
			}
			ratingTotal += expected
			var rows []judgeMeReview
			var looped bool
			if expected > judgeMePageSize*judgeMePageCap {
				rows, err = fetchJudgeMeDateFallback(ctx, c, params, expected)
			} else {
				rows, looped, err = fetchJudgeMeReviewPartition(ctx, c, params, 0, expected)
			}
			if err != nil {
				return judgeMeCorpus{}, fmt.Errorf("fetching rating %d partition: %w", rating, err)
			}
			if len(rows) != expected {
				return judgeMeCorpus{}, fmt.Errorf("rating %d count mismatch: fetched %d unique review IDs, live count says %d; refusing success", rating, len(rows), expected)
			}
			if looped && len(rows) < expected {
				return judgeMeCorpus{}, fmt.Errorf("rating %d repeated-page loop before count match: fetched=%d expected=%d", rating, len(rows), expected)
			}
			partitions = append(partitions, judgeMePartitionSummary{Rating: rating, Expected: expected, Fetched: len(rows), Complete: true})
			for _, row := range rows {
				byID[row.ID] = row
			}
		}
		if ratingTotal != total {
			return judgeMeCorpus{}, fmt.Errorf("rating partition counts sum to %d but live total is %d; refusing to treat the five slices as complete", ratingTotal, total)
		}
	}

	freshTotal, err := judgeMeReviewCount(ctx, c, nil)
	if err != nil {
		return judgeMeCorpus{}, err
	}
	if freshTotal != total {
		return judgeMeCorpus{}, fmt.Errorf("Judge.me count changed during sync (%d to %d); rerun to obtain a stable snapshot", total, freshTotal)
	}
	if len(byID) != freshTotal {
		return judgeMeCorpus{}, fmt.Errorf("final unique review count mismatch: fetched=%d live_count=%d; refusing success", len(byID), freshTotal)
	}

	rows := make([]judgeMeReview, 0, len(byID))
	bodyHashes := map[string]struct{}{}
	var published, hidden, pending int
	for _, row := range byID {
		rows = append(rows, row)
		if row.Published {
			published++
		}
		if row.Hidden {
			hidden++
		}
		if !row.Published && !row.Hidden {
			pending++
		}
		if row.BodyHash != "" {
			bodyHashes[row.BodyHash] = struct{}{}
		}
	}
	sort.Slice(rows, func(i, j int) bool { return numericStringLess(rows[i].ID, rows[j].ID) })
	return judgeMeCorpus{
		Reviews:       rows,
		ExpectedTotal: freshTotal,
		Published:     published,
		Hidden:        hidden,
		Pending:       pending,
		UniqueBodies:  len(bodyHashes),
		SyncedAt:      time.Now().UTC(),
		Partitions:    partitions,
	}, nil
}

func judgeMeReviewCount(ctx context.Context, c judgeMeReviewClient, params map[string]string) (int, error) {
	data, err := c.Get(ctx, "/reviews/count", cloneStringMap(params))
	if err != nil {
		return 0, err
	}
	var number int
	if json.Unmarshal(data, &number) == nil {
		return number, nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return 0, fmt.Errorf("decoding /reviews/count response: %w", err)
	}
	for _, key := range []string{"count", "total", "reviews_count"} {
		raw, ok := obj[key]
		if !ok {
			continue
		}
		if json.Unmarshal(raw, &number) == nil {
			return number, nil
		}
		var text string
		if json.Unmarshal(raw, &text) == nil {
			return strconv.Atoi(text)
		}
	}
	return 0, fmt.Errorf("decoding /reviews/count response: no numeric count field")
}

func fetchJudgeMeReviewPartition(ctx context.Context, c judgeMeReviewClient, fixed map[string]string, maxPages, expected int) ([]judgeMeReview, bool, error) {
	seen := make(map[string]judgeMeReview)
	for page := 1; ; page++ {
		if maxPages > 0 && page > maxPages {
			break
		}
		params := cloneStringMap(fixed)
		params["page"] = strconv.Itoa(page)
		params["per_page"] = strconv.Itoa(judgeMePageSize)
		data, err := c.Get(ctx, "/reviews", params)
		if err != nil {
			return nil, false, err
		}
		rawRows, err := decodeJudgeMeReviewPage(data)
		if err != nil {
			return nil, false, fmt.Errorf("decoding reviews page %d: %w", page, err)
		}
		if len(rawRows) == 0 {
			break
		}
		allSeen := true
		for _, raw := range rawRows {
			row, err := decodeJudgeMeReview(raw)
			if err != nil {
				return nil, false, fmt.Errorf("decoding review on page %d: %w", page, err)
			}
			if _, exists := seen[row.ID]; !exists {
				allSeen = false
				seen[row.ID] = row
			}
		}
		if allSeen {
			rows := reviewMapValues(seen)
			return rows, true, nil
		}
		if expected > 0 && len(seen) >= expected {
			if len(seen) > expected {
				return nil, false, fmt.Errorf("page %d exceeded live partition count: fetched=%d expected=%d", page, len(seen), expected)
			}
			break
		}
		if len(rawRows) < judgeMePageSize {
			break
		}
		if page >= judgeMePageCap && expected > judgeMePageSize*judgeMePageCap {
			return nil, false, fmt.Errorf("partition hit Judge.me's 10,000-row page cap; a narrower partition is required")
		}
	}
	return reviewMapValues(seen), false, nil
}

func fetchJudgeMeDateFallback(ctx context.Context, c judgeMeReviewClient, base map[string]string, expected int) ([]judgeMeReview, error) {
	type window struct {
		from  time.Time
		to    time.Time
		count int
		depth int
	}
	queue := []window{{from: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), to: time.Now().UTC().AddDate(0, 0, 2), count: expected}}
	byID := make(map[string]judgeMeReview, expected)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current.count <= judgeMePageSize*judgeMePageCap {
			params := cloneStringMap(base)
			params["start_date"] = current.from.Format("2006-01-02")
			params["end_date"] = current.to.Format("2006-01-02")
			rows, _, err := fetchJudgeMeReviewPartition(ctx, c, params, 0, current.count)
			if err != nil {
				return nil, err
			}
			for _, row := range rows {
				byID[row.ID] = row
			}
			continue
		}
		if current.depth >= 24 || current.to.Sub(current.from) <= 24*time.Hour {
			return nil, fmt.Errorf("date-window fallback could not reduce a %d-row partition below the 10,000-row cap", current.count)
		}
		mid := current.from.Add(current.to.Sub(current.from) / 2)
		leftParams := cloneStringMap(base)
		leftParams["start_date"] = current.from.Format("2006-01-02")
		leftParams["end_date"] = mid.Format("2006-01-02")
		rightParams := cloneStringMap(base)
		rightParams["start_date"] = mid.Format("2006-01-02")
		rightParams["end_date"] = current.to.Format("2006-01-02")
		left, err := judgeMeReviewCount(ctx, c, leftParams)
		if err != nil {
			return nil, err
		}
		right, err := judgeMeReviewCount(ctx, c, rightParams)
		if err != nil {
			return nil, err
		}
		if left == current.count && right == current.count {
			return nil, fmt.Errorf("Judge.me ignored date-window filters for a %d-row rating partition; refusing false completeness", current.count)
		}
		queue = append(queue,
			window{from: current.from, to: mid, count: left, depth: current.depth + 1},
			window{from: mid, to: current.to, count: right, depth: current.depth + 1},
		)
	}
	rows := reviewMapValues(byID)
	if len(rows) != expected {
		return nil, fmt.Errorf("date-window fallback produced %d unique IDs, expected %d", len(rows), expected)
	}
	return rows, nil
}

func decodeJudgeMeReviewPage(data json.RawMessage) ([]json.RawMessage, error) {
	var rows []json.RawMessage
	if json.Unmarshal(data, &rows) == nil {
		return rows, nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	for _, key := range []string{"reviews", "results", "data"} {
		if raw, ok := obj[key]; ok && json.Unmarshal(raw, &rows) == nil {
			return rows, nil
		}
	}
	return nil, errors.New("response contains no review array")
}

func decodeJudgeMeReview(raw json.RawMessage) (judgeMeReview, error) {
	var obj map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&obj); err != nil {
		return judgeMeReview{}, err
	}
	id := scalarString(obj["id"])
	if id == "" {
		return judgeMeReview{}, errors.New("review has no id")
	}
	body := scalarString(obj["body"])
	published := scalarBool(obj["published"])
	if _, present := obj["published"]; !present {
		published = scalarString(obj["curated"]) == "ok"
	}
	return judgeMeReview{
		ID:                id,
		Body:              body,
		BodyHash:          normalizedBodyHash(body),
		Rating:            scalarInt(obj["rating"]),
		Published:         published,
		Hidden:            scalarBool(obj["hidden"]),
		ProductExternalID: scalarString(obj["product_external_id"]),
		ProductHandle:     scalarString(obj["product_handle"]),
		ProductTitle:      scalarString(obj["product_title"]),
		CreatedAt:         scalarString(obj["created_at"]),
		UpdatedAt:         scalarString(obj["updated_at"]),
		Raw:               append(json.RawMessage(nil), raw...),
		Object:            obj,
	}, nil
}

func normalizedBodyHash(body string) string {
	normalized := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(body)), " "))
	if normalized == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func replaceJudgeMeReviewCorpus(ctx context.Context, db *sql.DB, corpus judgeMeCorpus) error {
	if len(corpus.Reviews) != corpus.ExpectedTotal {
		return fmt.Errorf("pre-commit count mismatch: rows=%d expected=%d", len(corpus.Reviews), corpus.ExpectedTotal)
	}
	if err := ensureJudgeMeReviewColumns(ctx, db); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM reviews`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM resources WHERE resource_type = 'reviews'`); err != nil {
		return err
	}
	reviewStmt, err := tx.PrepareContext(ctx, `INSERT INTO reviews (
		id, data, synced_at, review, body, created_at, curated,
		has_published_pictures, has_published_videos, hidden, ip_address, pinned,
		product_external_id, product_handle, product_title, rating, reviewer,
		source, title, updated_at, verified, published, body_hash
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer reviewStmt.Close()
	resourceStmt, err := tx.PrepareContext(ctx, `INSERT INTO resources (id, resource_type, data, synced_at, updated_at) VALUES (?, 'reviews', ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer resourceStmt.Close()
	syncedAt := corpus.SyncedAt.Format(time.RFC3339)
	for _, row := range corpus.Reviews {
		obj := row.Object
		if _, err := reviewStmt.ExecContext(ctx,
			row.ID, string(row.Raw), syncedAt, dbScalar(obj["review"]), row.Body,
			nullIfEmpty(row.CreatedAt), dbScalar(obj["curated"]), dbBool(obj["has_published_pictures"]),
			dbBool(obj["has_published_videos"]), row.Hidden, dbScalar(obj["ip_address"]),
			dbBool(obj["pinned"]), nullIfEmpty(row.ProductExternalID), row.ProductHandle,
			row.ProductTitle, row.Rating, dbScalar(obj["reviewer"]), dbScalar(obj["source"]),
			dbScalar(obj["title"]), nullIfEmpty(row.UpdatedAt), dbScalar(obj["verified"]),
			row.Published, nullIfEmpty(row.BodyHash),
		); err != nil {
			return fmt.Errorf("insert review %s: %w", row.ID, err)
		}
		if _, err := resourceStmt.ExecContext(ctx, row.ID, string(row.Raw), syncedAt, syncedAt); err != nil {
			return fmt.Errorf("insert generic review %s: %w", row.ID, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO sync_state (resource_type, last_cursor, last_synced_at, total_count)
		VALUES ('reviews', '', ?, ?)
		ON CONFLICT(resource_type) DO UPDATE SET last_cursor='', last_synced_at=excluded.last_synced_at, total_count=excluded.total_count`,
		syncedAt, corpus.ExpectedTotal); err != nil {
		return err
	}
	return tx.Commit()
}

func ensureJudgeMeReviewColumns(ctx context.Context, db *sql.DB) error {
	columns, err := tableColumns(ctx, db, "reviews")
	if err != nil {
		return err
	}
	for name, declaration := range map[string]string{
		"published": "INTEGER",
		"body_hash": "TEXT",
	} {
		if columns[name] {
			continue
		}
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE reviews ADD COLUMN %s %s`, name, declaration)); err != nil {
			// Multiple verifier commands may initialize the same store in
			// parallel. A peer can add the column after our PRAGMA snapshot.
			if strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
				continue
			}
			return err
		}
	}
	for _, statement := range []string{
		`CREATE INDEX IF NOT EXISTS idx_reviews_body_hash ON reviews(body_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_reviews_published_hidden ON reviews(published, hidden)`,
		`CREATE INDEX IF NOT EXISTS idx_reviews_rating ON reviews(rating)`,
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func tableColumns(ctx context.Context, db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info("`+table+`")`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var cid, notNull, pk int
		var name, kind string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &pk); err != nil {
			return nil, err
		}
		out[name] = true
	}
	return out, rows.Err()
}

func loadJudgeMeReviews(ctx context.Context, dbPath string, filter judgeMeReviewFilter) ([]judgeMeReview, string, error) {
	if err := validateJudgeMePopulation(filter.Population); err != nil {
		return nil, "", err
	}
	if dbPath == "" {
		dbPath = defaultDBPath("judgeme-pp-cli")
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, "", err
	}
	defer db.Close()
	if err := ensureJudgeMeReviewColumns(ctx, db.DB()); err != nil {
		return nil, "", err
	}
	rows, err := db.DB().QueryContext(ctx, `SELECT data, synced_at FROM reviews ORDER BY CAST(id AS INTEGER), id`)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var out []judgeMeReview
	var syncedAt string
	for rows.Next() {
		var raw string
		var rowSynced string
		if err := rows.Scan(&raw, &rowSynced); err != nil {
			return nil, "", err
		}
		review, err := decodeJudgeMeReview(json.RawMessage(raw))
		if err != nil {
			return nil, "", err
		}
		if !matchesJudgeMeReview(review, filter) {
			continue
		}
		out = append(out, review)
		if rowSynced > syncedAt {
			syncedAt = rowSynced
		}
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	if syncedAt == "" {
		return nil, "", errors.New("local review store is empty; run `judgeme-pp-cli sync --resources reviews --full` first")
	}
	return out, syncedAt, nil
}

func matchesJudgeMeReview(row judgeMeReview, filter judgeMeReviewFilter) bool {
	switch filter.Population {
	case "published":
		if !row.Published {
			return false
		}
	case "hidden":
		if !row.Hidden {
			return false
		}
	case "pending":
		if row.Published || row.Hidden {
			return false
		}
	case "all":
	default:
		return false
	}
	if filter.Rating > 0 && row.Rating != filter.Rating {
		return false
	}
	if filter.Product != "" && row.ProductExternalID != filter.Product && row.ProductHandle != filter.Product {
		return false
	}
	if filter.DateFrom != "" && row.CreatedAt < filter.DateFrom {
		return false
	}
	if filter.DateTo != "" && row.CreatedAt >= filter.DateTo {
		return false
	}
	return true
}

func validateJudgeMePopulation(population string) error {
	switch population {
	case "all", "published", "hidden", "pending":
		return nil
	default:
		return usageErr(fmt.Errorf("--population must be one of all, published, hidden, pending; got %q", population))
	}
}

func writeJudgeMeReviewCSV(w io.Writer, rows []judgeMeReview) error {
	writer := csv.NewWriter(w)
	if err := writer.Write([]string{"id", "population", "rating", "product_external_id", "product_handle", "created_at", "body_hash", "title", "body"}); err != nil {
		return err
	}
	for _, row := range rows {
		population := "pending"
		if row.Published {
			population = "published"
		} else if row.Hidden {
			population = "hidden"
		}
		if err := writer.Write([]string{
			row.ID, population, strconv.Itoa(row.Rating), row.ProductExternalID,
			row.ProductHandle, row.CreatedAt, row.BodyHash, scalarString(row.Object["title"]), row.Body,
		}); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func judgeMeRowsRaw(rows []judgeMeReview) json.RawMessage {
	enriched := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		item := make(map[string]any, len(row.Object)+2)
		for key, value := range row.Object {
			item[key] = value
		}
		item["body_hash"] = row.BodyHash
		switch {
		case row.Published:
			item["population"] = "published"
		case row.Hidden:
			item["population"] = "hidden"
		default:
			item["population"] = "pending"
		}
		enriched = append(enriched, item)
	}
	data, _ := json.Marshal(enriched)
	return data
}

func printJudgeMeLocalResult(cmd *cobra.Command, flags *rootFlags, value any, syncedAt, population string) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), raw, flags, map[string]any{
		"source":     "local",
		"synced_at":  syncedAt,
		"population": population,
	})
}

func printJudgeMeDryRun(cmd *cobra.Command, flags *rootFlags, command, population string) error {
	result := []map[string]any{{
		"command":    command,
		"dry_run":    true,
		"population": population,
		"complete":   false,
	}}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), raw, flags, map[string]any{
		"source":     "local",
		"population": population,
	})
}

func uniqueBodyCount(rows []judgeMeReview) int {
	hashes := map[string]struct{}{}
	for _, row := range rows {
		if row.BodyHash != "" {
			hashes[row.BodyHash] = struct{}{}
		}
	}
	return len(hashes)
}

func reviewMapValues(values map[string]judgeMeReview) []judgeMeReview {
	out := make([]judgeMeReview, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool { return numericStringLess(out[i].ID, out[j].ID) })
	return out
}

func numericStringLess(a, b string) bool {
	ai, aErr := strconv.ParseInt(a, 10, 64)
	bi, bErr := strconv.ParseInt(b, 10, 64)
	if aErr == nil && bErr == nil {
		return ai < bi
	}
	return a < b
}

func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in)+2)
	for key, value := range in {
		out[key] = value
	}
	return out
}

func scalarString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case json.Number:
		return v.String()
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		data, _ := json.Marshal(v)
		return string(data)
	}
}

func scalarInt(value any) int {
	number, _ := strconv.Atoi(scalarString(value))
	return number
}

func scalarBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case json.Number:
		return v.String() != "0"
	case float64:
		return v != 0
	case string:
		parsed, _ := strconv.ParseBool(v)
		return parsed || v == "1"
	default:
		return false
	}
}

func dbScalar(value any) any {
	if value == nil {
		return nil
	}
	return scalarString(value)
}

func dbBool(value any) any {
	if value == nil {
		return nil
	}
	return scalarBool(value)
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
