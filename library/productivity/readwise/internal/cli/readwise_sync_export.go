package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/store"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

const (
	readwiseExportRatePerSecond = 2.5
	readwiseV3RatePerSecond     = 1.0 / 3.1
	readwiseBooksListPageSize   = 1000
)

func useReadwiseExportSync() bool {
	return true
}

func readwiseSyncRateForResource(resource string) float64 {
	switch resource {
	case "books", "highlights", "readwise-export":
		return readwiseExportRatePerSecond
	case "readwise-books-list":
		return readwiseV3RatePerSecond
	case "documents", "reader-tags":
		return readwiseV3RatePerSecond
	default:
		return 0
	}
}

type readwiseGetter interface {
	Get(context.Context, string, map[string]string) (json.RawMessage, error)
	RateLimit() float64
}

type fixedPacedClient struct {
	inner readwiseGetter
	rate  float64

	mu          sync.Mutex
	nextAllowed time.Time
}

func newFixedPacedClient(inner readwiseGetter, rate float64) *fixedPacedClient {
	return &fixedPacedClient{inner: inner, rate: rate}
}

func (c *fixedPacedClient) Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error) {
	if err := c.wait(ctx); err != nil {
		return nil, err
	}
	return c.inner.Get(ctx, path, params)
}

func (c *fixedPacedClient) RateLimit() float64 {
	if c == nil {
		return 0
	}
	return c.rate
}

func (c *fixedPacedClient) wait(ctx context.Context) error {
	if c == nil || c.rate <= 0 {
		return nil
	}
	interval := time.Duration(float64(time.Second) / c.rate)
	c.mu.Lock()
	now := time.Now()
	waitUntil := c.nextAllowed
	if waitUntil.IsZero() || !now.Before(waitUntil) {
		c.nextAllowed = now.Add(interval)
		c.mu.Unlock()
		return nil
	}
	c.nextAllowed = waitUntil.Add(interval)
	c.mu.Unlock()

	timer := time.NewTimer(time.Until(waitUntil))
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func runReadwiseExportSyncCmd(cmd *cobra.Command, flags *rootFlags, resources []string, full bool, since string, concurrency int, dbPath string, maxPages int, latestOnly bool, strict bool, paramFlags, resourceParamFlags, globalParamFlags []string) error {
	_ = concurrency
	userParams, err := parseSyncUserParams(paramFlags, resourceParamFlags, globalParamFlags)
	if err != nil {
		return usageErr(err)
	}
	if len(resources) == 0 {
		resources = defaultSyncResources()
	}
	if err := userParams.validateResourceNames(knownSyncResourceNames()); err != nil {
		return usageErr(err)
	}
	if dbPath == "" {
		dbPath = defaultDBPath("readwise-pp-cli")
	}
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return fmt.Errorf("opening local database: %w", err)
	}
	defer db.Close()

	sinceTS := ""
	if since != "" {
		ts, err := parseSinceDuration(since)
		if err != nil {
			return fmt.Errorf("invalid --since value %q: %w", since, err)
		}
		sinceTS = ts.Format(time.RFC3339)
	}

	if full && !(latestOnly && since == "") {
		for _, resource := range resources {
			if err := db.ResetReadwiseSyncState(resource); err != nil {
				return err
			}
		}
		if err := db.ResetReadwiseSyncState("readwise-export"); err != nil {
			return err
		}
	}
	if latestOnly && since == "" {
		maxPages = 1
	}

	if cliutil.IsDogfoodEnv() && maxPages == 0 {
		// Curtail hard under live-dogfood: the 20/min endpoints are paced at
		// ~0.32 req/s, so even a few pages per resource would blow the flat
		// 30s per-command timeout. One page per resource proves the path works.
		maxPages = 1
	}

	started := readwiseRunStart(cmd.Context())
	syncEventWriter := cmd.OutOrStdout()
	results := runReadwiseSyncResources(cmd.Context(), flags, db, normalizeReadwiseSyncResources(resources), sinceTS, full, maxPages, latestOnly && since == "", userParams, syncEventWriter, readwisePruneAllowed(cmd))
	return finishReadwiseSyncResults(results, strict, started, syncEventWriter)
}

func normalizeReadwiseSyncResources(resources []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(resources))
	for _, resource := range resources {
		if resource == "highlights" {
			resource = "books"
		}
		if !seen[resource] {
			seen[resource] = true
			out = append(out, resource)
		}
	}
	return out
}

func runReadwiseSyncResources(ctx context.Context, flags *rootFlags, db *store.Store, resources []string, sinceTS string, full bool, maxPages int, latestOnly bool, userParams *syncUserParams, syncEvents io.Writer, pruneAllowed bool) []syncResult {
	results := make([]syncResult, 0, len(resources)+1)
	for _, resource := range resources {
		switch resource {
		case "books":
			exportClient, err := newReadwiseSyncClient(flags, readwiseSyncRateForResource("readwise-export"))
			if err != nil {
				results = append(results, syncResult{Resource: "books", Err: err})
				continue
			}
			booksListClient, err := newReadwiseSyncClient(flags, readwiseSyncRateForResource("readwise-books-list"))
			if err != nil {
				results = append(results, syncResult{Resource: "books", Err: err})
				continue
			}
			exportResults := syncReadwiseExport(ctx, exportClient, booksListClient, db, sinceTS, full, maxPages, latestOnly, userParams, syncEvents)
			results = append(results, exportResults...)
		case "documents":
			v3Client, err := newReadwiseSyncClient(flags, readwiseSyncRateForResource(resource))
			if err != nil {
				results = append(results, syncResult{Resource: resource, Err: err})
				continue
			}
			results = append(results, syncReadwiseCursorResource(ctx, v3Client, db, "documents", "https://readwise.io/api/v3/list/", sinceTS, full, maxPages, latestOnly, userParams, syncEvents, pruneAllowed))
		case "reader-tags":
			v3Client, err := newReadwiseSyncClient(flags, readwiseSyncRateForResource(resource))
			if err != nil {
				results = append(results, syncResult{Resource: resource, Err: err})
				continue
			}
			results = append(results, syncReadwiseCursorResource(ctx, v3Client, db, "reader-tags", "https://readwise.io/api/v3/tags/", "", full, maxPages, latestOnly, userParams, syncEvents, false))
		default:
			results = append(results, syncResult{Resource: resource, Err: fmt.Errorf("unknown sync resource %q", resource)})
		}
	}
	return results
}

func newReadwiseSyncClient(flags *rootFlags, rate float64) (readwiseGetter, error) {
	// Share TLS validation, explicit timeout, verified platform binding and hooks.
	syncFlags := *flags
	syncFlags.rateLimit = 0 // Fixed pacing below owns sync request spacing.
	syncFlags.noCache = true
	c, err := syncFlags.newClient()
	if err != nil {
		return nil, err
	}
	return newFixedPacedClient(c, rate), nil
}

type readwiseExportEnvelope struct {
	Count          int               `json:"count"`
	NextPageCursor flexCursor        `json:"nextPageCursor"`
	Results        []json.RawMessage `json:"results"`
}

type readwiseCursorEnvelope struct {
	Count          int               `json:"count"`
	NextPageCursor flexCursor        `json:"nextPageCursor"`
	Results        []json.RawMessage `json:"results"`
}

type readwiseBooksListEnvelope struct {
	Count    int               `json:"count"`
	Next     string            `json:"next"`
	Previous string            `json:"previous"`
	Results  []json.RawMessage `json:"results"`
}

type flexCursor string

func (c *flexCursor) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		*c = ""
		return nil
	}
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*c = flexCursor(s)
		return nil
	}

	var n json.Number
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&n); err != nil {
		return fmt.Errorf("decoding cursor: %w", err)
	}
	*c = flexCursor(n.String())
	return nil
}

func (c flexCursor) String() string {
	return string(c)
}

func syncReadwiseExport(ctx context.Context, c readwiseGetter, booksListClient readwiseGetter, db *store.Store, sinceTS string, full bool, maxPages int, latestOnly bool, userParams *syncUserParams, syncEvents io.Writer) []syncResult {
	started := readwiseRunStart(ctx)
	if syncEvents == nil {
		syncEvents = io.Discard
	}
	emitSyncStart(syncEvents, "books")
	emitSyncStart(syncEvents, "highlights")

	filtered := readwiseWalkFiltered(userParams, "books")
	// Head refreshes share the read-only checkpoint path with filtered walks.
	if latestOnly {
		filtered = true
		maxPages = 1
	}
	if latestOnly {
		fmt.Fprintln(os.Stderr, "books: head refresh — checkpoint not advanced")
	} else if filtered {
		fmt.Fprintln(os.Stderr, "books: filtered walk — checkpoint not advanced")
	}

	if full && !filtered {
		if err := db.ClearReadwiseBooksList(); err != nil {
			return []syncResult{{Resource: "books", Err: err}}
		}
	}
	listCursor, _, _, _ := db.GetSyncState("readwise-books-list")
	listScope, validListChain, listErr := db.ReadwiseBooksListResumeScope()
	if listErr != nil {
		return []syncResult{{Resource: "books", Err: listErr}}
	}
	resumeList := booksListClient != nil && !full && !filtered && listCursor != "" && validListChain
	if !full && !filtered && listCursor != "" && !validListChain {
		if err := db.ClearReadwiseBooksList(); err != nil {
			return []syncResult{{Resource: "books", Err: err}}
		}
	}
	existingCursor, lastSynced, savedExportCount, _ := db.GetSyncState("readwise-export")
	_, _, savedBookCount, _ := db.GetSyncState("books")
	_, _, savedHighlightCount, _ := db.GetSyncState("highlights")
	if full || filtered {
		existingCursor = ""
		lastSynced = time.Time{}
	}
	effectiveSince := sinceTS
	if full {
		effectiveSince = ""
	}
	if effectiveSince == "" && !lastSynced.IsZero() && !full {
		effectiveSince = lastSynced.Format(time.RFC3339)
	}

	if existingCursor != "" || resumeList {
		if saved, err := db.ReadwiseSyncSince("readwise-export"); err == nil {
			effectiveSince = saved
		}
	}

	if resumeList {
		if sinceTS != "" {
			fmt.Fprintln(os.Stderr, "resuming capped books-list chain; --since ignored until it completes")
		}
		effectiveSince = listScope
	}

	chainStarted, chainErr := readwiseWalkChainStart(db, filtered, "readwise-export", existingCursor != "" || resumeList, started)
	if chainErr != nil {
		return []syncResult{{Resource: "books", Err: chainErr}}
	}
	progressCount := 0
	if resumeList {
		progressCount = savedExportCount
	}
	if err := readwiseWalkProgress(db, filtered, "readwise-export", existingCursor, progressCount, effectiveSince); err != nil {
		emitSyncError(syncEvents, "books", err)
		return []syncResult{{Resource: "books", Err: err}}
	}
	var bookCount, highlightCount, deletedCount, pages int
	if resumeList {
		bookCount = savedBookCount
		highlightCount = savedHighlightCount
	}
	cursor := existingCursor
	capHit := false
	for !resumeList {
		params := map[string]string{}
		if cursor != "" {
			params["pageCursor"] = cursor
		}
		if effectiveSince != "" {
			params["updatedAfter"] = effectiveSince
		}
		if latestOnly {
			delete(params, "pageCursor")
			delete(params, "updatedAfter")
		}
		userParams.applyTo("books", params, false)
		params["includeDeleted"] = "true"

		data, err := c.Get(ctx, "/export/", params)
		if err != nil {
			emitSyncError(syncEvents, "books", err)
			return []syncResult{{Resource: "books", Count: bookCount, Err: fmt.Errorf("fetching readwise export: %w", err), Duration: time.Since(started)}}
		}
		if isDryRunResponseForClient(c, data) {
			emitSyncDryRun(syncEvents, "books")
			emitSyncDryRun(syncEvents, "highlights")
			return []syncResult{{Resource: "books"}, {Resource: "highlights"}}
		}

		var env readwiseExportEnvelope
		if err := json.Unmarshal(data, &env); err != nil {
			emitSyncError(syncEvents, "books", err)
			return []syncResult{{Resource: "books", Count: bookCount, Err: fmt.Errorf("decoding readwise export: %w", err), Duration: time.Since(started)}}
		}

		bookItems, highlightItems, skippedMalformed, err := normalizeReadwiseExportPage(env.Results)
		if err != nil {
			emitSyncError(syncEvents, "books", err)
			return []syncResult{{Resource: "books", Count: bookCount, Err: err, Duration: time.Since(started)}}
		}
		if skippedMalformed > 0 {
			fmt.Fprintf(os.Stderr, "warning: skipped %d malformed readwise export book(s)\n", skippedMalformed)
		}
		var deadBooks, deadHighlights []string
		liveBooks := make([]json.RawMessage, 0, len(bookItems))
		liveHighlights := make([]json.RawMessage, 0, len(highlightItems))
		for _, partition := range []struct {
			items []json.RawMessage
			dead  *[]string
			live  *[]json.RawMessage
		}{{bookItems, &deadBooks, &liveBooks}, {highlightItems, &deadHighlights, &liveHighlights}} {
			for _, raw := range partition.items {
				obj, err := store.DecodeJSONObject(raw)
				if err != nil {
					emitSyncError(syncEvents, "highlights", err)
					return []syncResult{{Resource: "highlights", Err: err}}
				}
				if deleted, _ := obj["is_deleted"].(bool); deleted {
					*partition.dead = append(*partition.dead, store.ResourceIDString(obj["id"]))
				} else {
					*partition.live = append(*partition.live, raw)
				}
			}
		}
		if len(deadBooks)+len(deadHighlights) > 0 {
			n, err := db.DeleteReadwiseTombstones(ctx, deadBooks, deadHighlights)
			if err != nil {
				emitSyncError(syncEvents, "highlights", err)
				return []syncResult{{Resource: "highlights", Err: err}}
			}
			deletedCount += n
		}
		bookItems = liveBooks
		if len(bookItems) > 0 {
			stored, _, _, err := upsertResourceBatch(db, "books", bookItems)
			if err != nil {
				emitSyncError(syncEvents, "books", err)
				return []syncResult{{Resource: "books", Count: bookCount, Err: fmt.Errorf("upserting export books: %w", err), Duration: time.Since(started)}}
			}
			bookCount += stored
		}
		highlightItems = liveHighlights
		if len(highlightItems) > 0 {
			stored, _, _, err := upsertResourceBatch(db, "highlights", highlightItems)
			if err != nil {
				emitSyncError(syncEvents, "highlights", err)
				return []syncResult{{Resource: "books", Count: bookCount, Duration: time.Since(started)}, {Resource: "highlights", Count: highlightCount, Err: fmt.Errorf("upserting export highlights: %w", err), Duration: time.Since(started)}}
			}
			highlightCount += stored
		}

		emitSyncProgress(syncEvents, c, "books", bookCount)
		emitSyncProgress(syncEvents, c, "highlights", highlightCount)
		pages++
		next := env.NextPageCursor.String()
		if !filtered {
			if err := db.SaveReadwiseExportCheckpoint(next, effectiveSince, bookCount+highlightCount, bookCount, highlightCount, chainStarted, false); err != nil {
				emitSyncError(syncEvents, "books", err)
				return []syncResult{{Resource: "books", Err: err}}
			}
		}
		if maxPages > 0 && pages >= maxPages {
			capHit = next != ""
			cursor = next
			break
		}
		if next == "" {
			break
		}
		cursor = next
	}

	finalCursor := ""
	if capHit {
		finalCursor = cursor
	}
	exportTotalCount := bookCount + highlightCount
	if resumeList {
		exportTotalCount = savedExportCount
	}
	if booksListClient != nil && (!capHit || latestOnly) {
		listCount, listCap, err := syncReadwiseBooksList(ctx, booksListClient, db, effectiveSince, maxPages, userParams, syncEvents, latestOnly)
		if err != nil {
			emitSyncError(syncEvents, "books", err)
			return []syncResult{
				{Resource: "books", Count: bookCount, Err: err, Duration: time.Since(started)},
				{Resource: "highlights", Count: highlightCount, Deleted: deletedCount, Duration: time.Since(started)},
			}
		}
		bookCount += listCount
		capHit = capHit || listCap
	}
	if !filtered {
		if !capHit {
			if booksListClient == nil {
				return []syncResult{{Resource: "books", Err: fmt.Errorf("books-list client is required to complete export sync")}}
			}
			// Completion reports cached books, not overlap or restart fetches.
			var err error
			bookCount, err = db.Count("books")
			if err != nil {
				return []syncResult{{Resource: "books", Err: err}}
			}
		}
		if err := db.SaveReadwiseExportCheckpoint(finalCursor, effectiveSince, exportTotalCount, bookCount, highlightCount, chainStarted, !capHit); err != nil {
			emitSyncError(syncEvents, "books", err)
			return []syncResult{{Resource: "books", Err: err}}
		}
	}
	emitSyncComplete(syncEvents, "books", bookCount, started)
	emitSyncComplete(syncEvents, "highlights", highlightCount, started)
	return []syncResult{
		{Resource: "books", Count: bookCount, Duration: time.Since(started)},
		{Resource: "highlights", Count: highlightCount, Deleted: deletedCount, Duration: time.Since(started)},
	}
}

func syncReadwiseBooksList(ctx context.Context, c readwiseGetter, db *store.Store, effectiveSince string, maxPages int, userParams *syncUserParams, syncEvents io.Writer, latestOnly bool) (int, bool, error) {
	page := 1
	filtered := readwiseWalkFiltered(userParams, "books") || latestOnly
	if latestOnly {
		maxPages = 1
		effectiveSince = ""
	}
	sizeParams := map[string]string{"page_size": strconv.Itoa(readwiseBooksListPageSize)}
	userParams.applyTo("books", sizeParams, false)
	pageSize := sizeParams["page_size"]
	resumed := false
	requestedMaxPages := maxPages
	if !filtered {
		cursor, _, _, err := db.GetSyncState("readwise-books-list")
		if err != nil {
			return 0, false, err
		}
		savedPage, savedSize, ok := strings.Cut(cursor, ":")
		if ok && savedSize == pageSize {
			if saved, err := strconv.Atoi(savedPage); err == nil && saved > 0 {
				page = saved
				resumed = true
				// The overlap is extra work so even a one-page cap makes forward progress.
				if maxPages > 0 {
					maxPages++
				}
			}
		}
	}

	var total, pages int
	for {
		params := map[string]string{
			"page":      fmt.Sprintf("%d", page),
			"page_size": fmt.Sprintf("%d", readwiseBooksListPageSize),
		}
		if effectiveSince != "" {
			params["updated__gt"] = effectiveSince
		}
		if latestOnly {
			params["page"] = "1"
			delete(params, "updated__gt")
		}
		userParams.applyTo("books", params, false)

		data, err := c.Get(ctx, "/books/", params)
		if err != nil {
			var apiErr *client.APIError
			if resumed && page > 1 && errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
				page = 1
				maxPages = requestedMaxPages
				resumed = false
				continue
			}
			return total, false, fmt.Errorf("fetching readwise books list: %w", err)
		}
		if isDryRunResponseForClient(c, data) {
			emitSyncDryRun(syncEvents, "books")
			return total, false, nil
		}

		var env readwiseBooksListEnvelope
		if err := json.Unmarshal(data, &env); err != nil {
			return total, false, fmt.Errorf("decoding readwise books list: %w", err)
		}

		resumed = false

		// Empty pages are terminal even when the API advertises a next page.
		if len(env.Results) == 0 {
			env.Next = ""
		}

		stored, _, _, err := upsertResourceBatch(db, "books", env.Results)
		if err != nil {
			return total, false, fmt.Errorf("upserting readwise books list: %w", err)
		}
		total += stored
		emitSyncProgress(syncEvents, c, "books", total)

		pages++
		if !filtered {
			if env.Next != "" {
				if err := db.SaveReadwiseSyncProgress("readwise-books-list", strconv.Itoa(page)+":"+pageSize, total, effectiveSince); err != nil {
					return total, false, err
				}
			}
		}
		if maxPages > 0 && pages >= maxPages {
			return total, env.Next != "", nil
		}
		if env.Next == "" {
			return total, false, nil
		}
		page++
	}
}

func syncReadwiseCursorResource(ctx context.Context, c readwiseGetter, db *store.Store, resource, path, sinceTS string, full bool, maxPages int, latestOnly bool, userParams *syncUserParams, syncEvents io.Writer, pruneAllowed bool) syncResult {
	filtered := readwiseWalkFiltered(userParams, resource) || latestOnly
	if latestOnly {
		maxPages = 1
	}
	if latestOnly {
		fmt.Fprintf(os.Stderr, "%s: head refresh — checkpoint not advanced\n", resource)
	} else if filtered {
		fmt.Fprintf(os.Stderr, "%s: filtered walk — checkpoint not advanced\n", resource)
	}
	started := readwiseRunStart(ctx)
	emitSyncStart(syncEvents, resource)
	existingCursor, lastSynced, _, _ := db.GetSyncState(resource)
	if full || filtered {
		existingCursor = ""
		lastSynced = time.Time{}
	}
	effectiveSince := sinceTS
	if resource == "documents" && effectiveSince == "" && !lastSynced.IsZero() && !full {
		effectiveSince = lastSynced.Format(time.RFC3339)
	}

	if existingCursor != "" {
		if saved, err := db.ReadwiseSyncSince(resource); err == nil {
			effectiveSince = saved
		}
	}

	chainStarted, chainErr := readwiseWalkChainStart(db, filtered, resource, existingCursor != "", started)
	if chainErr != nil {
		return syncResult{Resource: resource, Err: chainErr}
	}
	if err := readwiseWalkProgress(db, filtered, resource, existingCursor, 0, effectiveSince); err != nil {
		emitSyncError(syncEvents, resource, err)
		return syncResult{Resource: resource, Err: err}
	}
	cursor := existingCursor
	missingResults := false
	sawChild := false
	reportedCount := 0
	seen := map[string]bool{}
	prune := resource == "documents" && full && sinceTS == "" && pruneAllowed && !filtered
	baseline, baselineErr := db.List("documents", 0)
	if baselineErr != nil {
		return syncResult{Resource: resource, Err: baselineErr}
	}
	var total, pages int
	capHit := false
	for {
		params := map[string]string{}
		if resource == "documents" {
			params["limit"] = "100"
		}
		if cursor != "" {
			params["pageCursor"] = cursor
		}
		if resource == "documents" && effectiveSince != "" {
			params["updatedAfter"] = effectiveSince
		}
		if latestOnly {
			delete(params, "pageCursor")
			delete(params, "updatedAfter")
		}
		userParams.applyTo(resource, params, false)
		for key := range params {
			if key != "limit" && key != "pageCursor" {
				prune = false
			}
		}

		data, err := c.Get(ctx, path, params)
		if err != nil {
			emitSyncError(syncEvents, resource, err)
			return syncResult{Resource: resource, Count: total, Err: fmt.Errorf("fetching %s: %w", resource, err), Duration: time.Since(started)}
		}
		if isDryRunResponseForClient(c, data) {
			emitSyncDryRun(syncEvents, resource)
			return syncResult{Resource: resource, Duration: time.Since(started)}
		}
		var env readwiseCursorEnvelope
		if err := json.Unmarshal(data, &env); err != nil {
			emitSyncError(syncEvents, resource, err)
			return syncResult{Resource: resource, Count: total, Err: fmt.Errorf("decoding %s: %w", resource, err), Duration: time.Since(started)}
		}
		if env.Results == nil {
			missingResults = true
		}
		if env.Count > reportedCount {
			reportedCount = env.Count
		}
		for _, raw := range env.Results {
			obj, err := store.DecodeJSONObject(raw)
			if err != nil {
				prune = false
				continue
			}
			id := store.ResourceIDString(obj["id"])
			if id == "" {
				prune = false
			}
			seen[id] = true
			if store.ResourceIDString(obj["parent_id"]) != "" {
				sawChild = true
			}
		}
		if len(env.Results) > 0 {
			stored, _, _, err := upsertResourceBatch(db, resource, env.Results)
			if err != nil {
				emitSyncError(syncEvents, resource, err)
				return syncResult{Resource: resource, Count: total, Err: fmt.Errorf("upserting %s: %w", resource, err), Duration: time.Since(started)}
			}
			total += stored
		}
		emitSyncProgress(syncEvents, c, resource, total)
		pages++
		next := env.NextPageCursor.String()
		if next != "" {
			if err := readwiseWalkProgress(db, filtered, resource, next, total, effectiveSince); err != nil {
				emitSyncError(syncEvents, resource, err)
				return syncResult{Resource: resource, Err: err}
			}
		}
		if maxPages > 0 && pages >= maxPages {
			capHit = next != ""
			cursor = next
			break
		}
		if next == "" {
			break
		}
		cursor = next
	}
	finalCursor := ""
	if capHit {
		finalCursor = cursor
	}
	if latestOnly {
		finalCursor = ""
	}
	if prune && !capHit {
		reason, err := readwisePruneRefusal(baseline, seen, missingResults, reportedCount, ctx.Value(readwiseMassPruneKey{}) == true)
		if err != nil {
			return syncResult{Resource: resource, Err: err}
		}
		if reason != "" {
			fmt.Fprintf(syncEvents, "{\"event\":\"sync_warning\",\"resource\":\"documents\",\"reason\":%q}\n", reason)
			prune = false
		}
	}
	if prune && !capHit && !sawChild {
		fmt.Fprintln(syncEvents, `{"event":"sync_warning","resource":"documents","reason":"child_pruning_skipped_no_children_returned"}`)
	}
	if prune && !capHit {
		if err := db.PruneReadwiseDocuments(ctx, seen, sawChild); err != nil {
			emitSyncError(syncEvents, resource, err)
			return syncResult{Resource: resource, Err: err}
		}
	}
	var checkpointErr error
	if capHit {
		checkpointErr = readwiseWalkProgress(db, filtered, resource, finalCursor, total, effectiveSince)
	} else {
		checkpointErr = readwiseWalkComplete(db, filtered, resource, finalCursor, total, chainStarted)
	}
	if checkpointErr != nil {
		emitSyncError(syncEvents, resource, checkpointErr)
		return syncResult{Resource: resource, Err: checkpointErr}
	}
	emitSyncComplete(syncEvents, resource, total, started)
	return syncResult{Resource: resource, Count: total, Duration: time.Since(started)}
}

func finishReadwiseSyncResults(results []syncResult, strict bool, started time.Time, syncEventWriter io.Writer) error {
	var totalSynced, totalDeleted, errCount, warnCount, successCount int
	var firstErr error
	for _, res := range results {
		if res.Err != nil {
			if humanFriendly {
				fmt.Fprintf(os.Stderr, "  %s: error: %s\n", res.Resource, safeSyncErrorMessage(res.Err))
			}
			errCount++
			if firstErr == nil {
				firstErr = res.Err
			}
			continue
		}
		if res.Warn != nil {
			if humanFriendly {
				fmt.Fprintf(os.Stderr, "  %s: warning: %v\n", res.Resource, res.Warn)
			}
			warnCount++
			continue
		}
		if humanFriendly {
			fmt.Fprintf(os.Stderr, "  %s: %d synced, %d deleted (done)\n", res.Resource, res.Count, res.Deleted)
		}
		totalSynced += res.Count
		totalDeleted += res.Deleted
		successCount++
	}
	elapsed := time.Since(started)
	totalResources := successCount + warnCount + errCount
	if humanFriendly {
		fmt.Fprintf(os.Stderr, "Sync complete: %d records, %d deleted across %d resources (%.1fs)\n", totalSynced, totalDeleted, totalResources, elapsed.Seconds())
	} else {
		fmt.Fprintf(syncEventWriter, `{"event":"sync_summary","total_records":%d,"deleted":%d,"resources":%d,"success":%d,"warned":%d,"errored":%d,"duration_ms":%d}`+"\n",
			totalSynced, totalDeleted, totalResources, successCount, warnCount, errCount, elapsed.Milliseconds())
	}
	for _, res := range results {
		if errors.Is(res.Err, context.Canceled) || errors.Is(res.Err, context.DeadlineExceeded) {
			return redactedSyncFailure("sync interrupted", res.Err)
		}
	}
	if firstErr != nil && strict {
		return redactedSyncFailure(fmt.Sprintf("%d resource(s) failed to sync", errCount), firstErr)
	}
	if successCount == 0 {
		if firstErr != nil {
			return redactedSyncFailure(fmt.Sprintf("%d resource(s) failed to sync", errCount), firstErr)
		}
		if warnCount > 0 {
			return fmt.Errorf("%d resource(s) skipped due to insufficient access", warnCount)
		}
	}
	if errCount > 0 && !strict && successCount > 0 {
		fmt.Fprintf(syncEventWriter, `{"event":"sync_warning","reason":"exit_policy_default_changed","errored":%d,"message":"Some resources failed; pass --strict for non-zero exit on any failure."}`+"\n", errCount)
	}
	return nil
}

func normalizeReadwiseExportPage(books []json.RawMessage) ([]json.RawMessage, []json.RawMessage, int, error) {
	bookItems := make([]json.RawMessage, 0, len(books))
	var highlightItems []json.RawMessage
	var skippedMalformed int
	for _, raw := range books {
		book, highlights, err := normalizeReadwiseExportBook(raw)
		if err != nil {
			skippedMalformed++
			continue
		}
		bookItems = append(bookItems, book)
		highlightItems = append(highlightItems, highlights...)
	}
	return bookItems, highlightItems, skippedMalformed, nil
}

func normalizeReadwiseExportBook(raw json.RawMessage) (json.RawMessage, []json.RawMessage, error) {
	obj, err := store.DecodeJSONObject(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("decoding export book: %w", err)
	}
	bookID := store.ResourceIDString(obj["user_book_id"])
	if bookID == "" {
		bookID = store.ResourceIDString(obj["id"])
	}
	if bookID == "" {
		return nil, nil, fmt.Errorf("export book missing user_book_id")
	}
	obj["id"] = bookID
	if _, ok := obj["updated"]; !ok {
		if v, ok := newestExportTimestamp(obj); ok {
			obj["updated"] = v
		}
	}
	highlightsRaw, _ := obj["highlights"].([]any)
	// F a book tombstone must not resurrect nested highlights.
	if dead, _ := obj["is_deleted"].(bool); dead {
		highlightsRaw = nil
	}
	delete(obj, "highlights")

	bookJSON, err := json.Marshal(obj)
	if err != nil {
		return nil, nil, err
	}

	highlights := make([]json.RawMessage, 0, len(highlightsRaw))
	for _, h := range highlightsRaw {
		hObj, ok := h.(map[string]any)
		if !ok {
			continue
		}
		if store.ResourceIDString(hObj["book_id"]) == "" {
			hObj["book_id"] = bookID
		}
		if _, ok := hObj["updated"]; !ok {
			if v := store.ResourceIDString(hObj["updated_at"]); v != "" {
				hObj["updated"] = v
			}
		}
		hJSON, err := json.Marshal(hObj)
		if err != nil {
			return nil, nil, err
		}
		highlights = append(highlights, hJSON)
	}
	return bookJSON, highlights, nil
}

func newestExportTimestamp(obj map[string]any) (string, bool) {
	var values []string
	for _, key := range []string{"updated_at", "last_highlight_at"} {
		if value := store.ResourceIDString(obj[key]); value != "" {
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return "", false
	}
	sort.Strings(values)
	return values[len(values)-1], true
}

func emitSyncStart(w io.Writer, resource string) {
	if !humanFriendly {
		fmt.Fprintf(w, `{"event":"sync_start","resource":"%s"}`+"\n", resource)
	}
}

func emitSyncProgress(w io.Writer, c readwiseGetter, resource string, count int) {
	if humanFriendly {
		if c.RateLimit() > 0 {
			fmt.Fprintf(os.Stderr, "\r  %s: %d synced [%.2f req/s]", resource, count, c.RateLimit())
		} else {
			fmt.Fprintf(os.Stderr, "\r  %s: %d synced", resource, count)
		}
		return
	}
	fmt.Fprintf(w, `{"event":"sync_progress","resource":"%s","fetched":%d,"rate_rps":%.2f}`+"\n", resource, count, c.RateLimit())
}

func emitSyncComplete(w io.Writer, resource string, count int, started time.Time) {
	if !humanFriendly {
		fmt.Fprintf(w, `{"event":"sync_complete","resource":"%s","total":%d,"duration_ms":%d}`+"\n", resource, count, time.Since(started).Milliseconds())
	}
}

func emitSyncDryRun(w io.Writer, resource string) {
	if !humanFriendly {
		fmt.Fprintf(w, `{"event":"sync_dryrun","resource":"%s"}`+"\n", resource)
	}
}

func emitSyncError(w io.Writer, resource string, err error) {
	if !humanFriendly {
		fmt.Fprintln(w, syncErrorJSON(resource, "", err))
	}
}

func readwisePruneAllowed(cmd *cobra.Command) bool {
	disabled, _ := cmd.Flags().GetBool("no-prune")
	return !disabled
}

// readwiseMassPruneKey is set only by sync's explicit override flag.
type readwiseMassPruneKey struct{}

func readwisePruneRefusal(baseline []json.RawMessage, seen map[string]bool, missing bool, count int, allowMass bool) (string, error) {
	if missing {
		return "prune_missing_results", nil
	}
	if len(seen) == 0 && len(baseline) > 0 {
		return "prune_empty_snapshot", nil
	}
	if len(seen) < count {
		return "prune_incomplete_count", nil
	}
	top, removed := 0, 0
	for _, raw := range baseline {
		obj, err := store.DecodeJSONObject(raw)
		if err != nil {
			return "", err
		}
		if store.ResourceIDString(obj["parent_id"]) == "" {
			top++
			if !seen[store.ResourceIDString(obj["id"])] {
				removed++
			}
		}
	}
	if !allowMass && removed*2 > top {
		return "prune_mass_removal_refused", nil
	}
	return "", nil
}

type readwiseRunStartKey struct{}

func readwiseRunStart(ctx context.Context) time.Time {
	if at, ok := ctx.Value(readwiseRunStartKey{}).(time.Time); ok {
		return at
	}
	return time.Now()
}

type safeReadwiseSyncError struct {
	message string
	cause   error
}

func (e *safeReadwiseSyncError) Error() string { return e.message }
func (e *safeReadwiseSyncError) Unwrap() error { return e.cause }
func redactedSyncFailure(prefix string, err error) error {
	return &safeReadwiseSyncError{message: prefix + ": " + safeSyncErrorMessage(err), cause: err}
}

// readwiseWalkFiltered keeps partitioned reads out of shared checkpoint state.
func readwiseWalkFiltered(p *syncUserParams, resource string) bool {
	if resource == "readwise-export" || resource == "highlights" {
		resource = "books"
	}
	params := map[string]string{}
	p.applyTo(resource, params, false)
	for key := range params {
		switch key {
		case "limit", "page_size", "withHtmlContent", "withRawSourceUrl":
		default:
			return true
		}
	}
	return false
}
func readwiseWalkChainStart(db *store.Store, filtered bool, resource string, resume bool, now time.Time) (time.Time, error) {
	if filtered {
		return now, nil
	}
	return db.ReadwiseChainStart(resource, resume, now)
}
func readwiseWalkProgress(db *store.Store, filtered bool, resource, cursor string, count int, since string) error {
	if filtered {
		return nil
	}
	return db.SaveReadwiseSyncProgress(resource, cursor, count, since)
}
func readwiseWalkComplete(db *store.Store, filtered bool, resource, cursor string, count int, at time.Time) error {
	if filtered {
		return nil
	}
	return db.SaveSyncStateAt(resource, cursor, count, at)
}
