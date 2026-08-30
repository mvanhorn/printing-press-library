// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written Gmail metadata sync. Replaces the generated generic-resource
// sync registration for this CLI: mailbox-cleanup intelligence needs typed
// per-account rows (mail_meta) with parsed From headers, derived categories,
// and List-Unsubscribe capture — not raw JSON blobs in the generic resources
// table. The generated framework sync (sync.go) remains in-tree because
// channel_workflow.go reuses its syncResource machinery.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"
)

const (
	mailSyncPageSize      = 500
	mailSyncDefaultLimit  = 5000
	mailSyncWorkers       = 8
	mailMetadataFetchPath = "/gmail/v1/users/me/messages/"
)

// mailSyncResult is the JSON envelope sync prints on success.
type mailSyncResult struct {
	Account    string `json:"account"`
	Mode       string `json:"mode"` // full | incremental
	Fetched    int    `json:"fetched"`
	Upserted   int    `json:"upserted"`
	Deleted    int    `json:"deleted"`
	HistoryID  string `json:"history_id"`
	DurationMs int64  `json:"duration_ms"`
}

func newMailSyncCmd(flags *rootFlags) *cobra.Command {
	var full bool
	var limit int
	var q string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync Gmail message metadata into the local mail_meta store (full or historyId-incremental)",
		Long: `Sync message metadata for one account into the local SQLite store.

Full sync pages users.messages.list (newest first), then fetches each id with
format=metadata (From, Subject, List-Unsubscribe, List-Unsubscribe-Post,
Authentication-Results) through a bounded worker pool and upserts into
mail_meta + its FTS index.
The account's current historyId (from users.getProfile) is recorded so the
next run can be incremental.

Incremental sync replays users.history.list from the stored historyId:
messagesAdded are fetched and upserted, messagesDeleted rows are removed,
and labelsAdded/labelsRemoved update label_ids, unread, and category in
place (label updates count toward "upserted"). When Gmail reports the
cursor expired (HTTP 404) an honest notice is printed and a full sync runs
automatically.

--q runs an ad-hoc filtered backfill (full Gmail search syntax). Because a
filtered listing proves nothing about the rest of the mailbox, it never
moves the incremental historyId cursor.

Local-only writes: this command reads Gmail and writes SQLite; it never
mutates the mailbox.`,
		Example: `  # First run: full sync of the newest 5000 messages
  gmail-pp-cli sync --account personal

  # Afterwards: incremental from the stored historyId cursor
  gmail-pp-cli sync --account personal

  # Force a full re-walk, larger window, agent-friendly output
  gmail-pp-cli sync --account ads --full --limit 20000 --agent

  # Ad-hoc backfill of promotions older than a year (cursor untouched)
  gmail-pp-cli sync --account ads --q "category:promotions older_than:1y"`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Account resolution runs before the dry-run gate on purpose:
			// a typo'd --account (or an unconfigured gauth dir) must fail
			// with the available profiles, not silently no-op.
			account, err := resolveGauthAccount(flags)
			if err != nil {
				return err
			}
			if limit <= 0 {
				limit = mailSyncDefaultLimit
			}
			// Dogfood runs cap the window so a first exercise can't walk
			// a whole mailbox; an explicit --limit wins.
			if cliutil.IsDogfoodEnv() && !cmd.Flags().Changed("limit") {
				limit = 50
			}
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true

			// Identity preflight (grill R2-C1): synced rows are keyed by
			// account name, and every cleanup plan snapshots from them.
			// Writing mailbox B's rows under account A's key would poison
			// downstream plans, so a live-vs-configured email mismatch is a
			// typed refusal (exit 4) before any row is written.
			if err := verifyLiveIdentity(cmd.Context(), c, flags, account); err != nil {
				return classifyAPIError(err, flags)
			}

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("gmail-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			st, err := db.GetMailSyncState(account)
			if err != nil {
				return fmt.Errorf("reading sync state: %w", err)
			}

			started := time.Now()
			errW := cmd.ErrOrStderr()
			mode := "full"
			if !full && q == "" && st.HistoryID != "" {
				mode = "incremental"
			}

			var res mailSyncResult
			if mode == "incremental" {
				incRes, expired, incErr := mailIncrementalSync(cmd.Context(), c, db, account, st.HistoryID, limit, errW)
				switch {
				case expired:
					mailSyncNotice(errW, "history_expired",
						fmt.Sprintf("history cursor %s expired on Gmail's side; falling back to a full sync", st.HistoryID))
					mode = "full"
				case incErr != nil:
					return classifyAPIError(incErr, flags)
				default:
					res = incRes
				}
			}
			if mode == "full" {
				fullRes, fullErr := mailFullSync(cmd.Context(), c, db, account, q, limit, errW)
				if fullErr != nil {
					return classifyAPIError(fullErr, flags)
				}
				res = fullRes
			}

			res.Account = account
			res.Mode = mode
			res.DurationMs = time.Since(started).Milliseconds()

			// Persist the cursor. --q backfills never move it: a filtered
			// listing proves nothing about the rest of the mailbox.
			switch {
			case q != "":
				res.HistoryID = st.HistoryID
			case res.HistoryID != "":
				if err := db.SaveMailSyncState(account, res.HistoryID, mode); err != nil {
					return fmt.Errorf("saving sync state: %w", err)
				}
			default:
				res.HistoryID = st.HistoryID
			}

			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}

	cmd.Flags().BoolVar(&full, "full", false, "Force a full re-walk even when an incremental historyId cursor exists")
	cmd.Flags().IntVar(&limit, "limit", mailSyncDefaultLimit, "Maximum messages to fetch this run (newest first)")
	cmd.Flags().StringVar(&q, "q", "", "Gmail search query for an ad-hoc filtered backfill (full search syntax; does not move the incremental cursor)")
	return cmd
}

// mailSyncNotice prints an operator-visible sync notice to stderr, honoring
// the NDJSON-by-default / prose-under---human-friendly house convention.
func mailSyncNotice(w io.Writer, reason, message string) {
	if humanFriendly {
		fmt.Fprintf(w, "notice: %s\n", message)
		return
	}
	payload, _ := json.Marshal(map[string]string{
		"event":   "mail_sync_notice",
		"reason":  reason,
		"message": message,
	})
	fmt.Fprintln(w, string(payload))
}

func mailSyncProgress(w io.Writer, stage string, n int) {
	if humanFriendly {
		fmt.Fprintf(w, "  %s: %d\n", stage, n)
		return
	}
	fmt.Fprintf(w, `{"event":"mail_sync_progress","stage":"%s","count":%d}`+"\n", stage, n)
}

// mailFullSync pages users.messages.list, fetches metadata for every listed
// id, upserts, and records the profile's current historyId.
func mailFullSync(ctx context.Context, c *client.Client, db *store.Store, account, q string, limit int, errW io.Writer) (mailSyncResult, error) {
	res := mailSyncResult{}

	ids, err := mailListMessageIDs(ctx, c, q, limit)
	if err != nil {
		return res, fmt.Errorf("listing messages: %w", err)
	}
	mailSyncProgress(errW, "listed", len(ids))

	metas, fetched, err := mailFetchMetadata(ctx, c, account, ids, errW)
	if err != nil {
		return res, err
	}
	res.Fetched = fetched

	upserted, err := db.UpsertMailMeta(metas)
	if err != nil {
		return res, fmt.Errorf("upserting mail_meta: %w", err)
	}
	res.Upserted = upserted
	mailSyncProgress(errW, "upserted", upserted)

	// Record where "now" is so the next run can be incremental. Fetched
	// AFTER the listing walk: a historyId minted before the walk could miss
	// nothing, one minted after can only replay changes we may already
	// hold — replays are idempotent upserts.
	profile, err := c.Get(ctx, "/gmail/v1/users/me/profile", nil)
	if err != nil {
		return res, fmt.Errorf("fetching profile historyId: %w", err)
	}
	var prof struct {
		HistoryID string `json:"historyId"`
	}
	if err := json.Unmarshal(profile, &prof); err != nil {
		return res, fmt.Errorf("parsing profile: %w", err)
	}
	res.HistoryID = prof.HistoryID
	return res, nil
}

// mailListMessageIDs pages users.messages.list (newest first, the API's
// default ordering) collecting up to limit ids.
func mailListMessageIDs(ctx context.Context, c *client.Client, q string, limit int) ([]string, error) {
	var ids []string
	pageToken := ""
	for len(ids) < limit {
		want := limit - len(ids)
		if want > mailSyncPageSize {
			want = mailSyncPageSize
		}
		params := url.Values{}
		params.Set("maxResults", strconv.Itoa(want))
		if q != "" {
			params.Set("q", q)
		}
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}
		data, err := c.GetWithHeadersValues(ctx, "/gmail/v1/users/me/messages", params, nil)
		if err != nil {
			return nil, err
		}
		var page struct {
			Messages []struct {
				ID string `json:"id"`
			} `json:"messages"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err := json.Unmarshal(data, &page); err != nil {
			return nil, fmt.Errorf("parsing messages.list page: %w", err)
		}
		for _, m := range page.Messages {
			if m.ID != "" {
				ids = append(ids, m.ID)
			}
		}
		if page.NextPageToken == "" || len(page.Messages) == 0 {
			break
		}
		pageToken = page.NextPageToken
	}
	return ids, nil
}

// mailFetchMetadata fetches format=metadata for each id through a bounded
// worker pool (all requests share the client's adaptive limiter and 429
// retry). Individual fetch failures are counted and warned; the run only
// fails when nothing could be fetched at all.
func mailFetchMetadata(ctx context.Context, c *client.Client, account string, ids []string, errW io.Writer) ([]store.MailMeta, int, error) {
	if len(ids) == 0 {
		return nil, 0, nil
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	work := make(chan string)
	results := make(chan store.MailMeta, len(ids))
	errs := make(chan error, len(ids))

	var wg sync.WaitGroup
	workers := mailSyncWorkers
	if workers > len(ids) {
		workers = len(ids)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range work {
				meta, err := mailFetchOneMetadata(ctx, c, account, id)
				if err != nil {
					errs <- fmt.Errorf("message %s: %w", id, err)
					continue
				}
				results <- meta
			}
		}()
	}

	go func() {
		defer close(work)
		for _, id := range ids {
			select {
			case work <- id:
			case <-ctx.Done():
				return
			}
		}
	}()

	wg.Wait()
	close(results)
	close(errs)

	metas := make([]store.MailMeta, 0, len(ids))
	for m := range results {
		metas = append(metas, m)
	}
	var firstErr error
	failed := 0
	for e := range errs {
		failed++
		if firstErr == nil {
			firstErr = e
		}
	}
	if failed > 0 {
		mailSyncNotice(errW, "metadata_fetch_failures",
			fmt.Sprintf("%d of %d metadata fetches failed (first: %v)", failed, len(ids), firstErr))
	}
	if len(metas) == 0 && firstErr != nil {
		return nil, 0, fmt.Errorf("all %d metadata fetches failed: %w", len(ids), firstErr)
	}
	return metas, len(metas), nil
}

func mailFetchOneMetadata(ctx context.Context, c *client.Client, account, id string) (store.MailMeta, error) {
	params := url.Values{}
	params.Set("format", "metadata")
	for _, h := range []string{"From", "Subject", "List-Unsubscribe", "List-Unsubscribe-Post", "Authentication-Results"} {
		params.Add("metadataHeaders", h)
	}
	data, err := c.GetWithHeadersValues(ctx, mailMetadataFetchPath+url.PathEscape(id), params, nil)
	if err != nil {
		return store.MailMeta{}, err
	}
	var msg gmailMessageMeta
	if err := json.Unmarshal(data, &msg); err != nil {
		return store.MailMeta{}, fmt.Errorf("parsing metadata: %w", err)
	}
	if msg.ID == "" {
		msg.ID = id
	}
	return mailMetaFromMessage(account, &msg), nil
}

// Gmail history.list shapes (only the fields the sync consumes).
type gmailHistoryMessage struct {
	ID       string   `json:"id"`
	ThreadID string   `json:"threadId"`
	LabelIDs []string `json:"labelIds"`
}

type gmailHistoryRecord struct {
	MessagesAdded []struct {
		Message gmailHistoryMessage `json:"message"`
	} `json:"messagesAdded"`
	MessagesDeleted []struct {
		Message gmailHistoryMessage `json:"message"`
	} `json:"messagesDeleted"`
	LabelsAdded []struct {
		Message  gmailHistoryMessage `json:"message"`
		LabelIDs []string            `json:"labelIds"`
	} `json:"labelsAdded"`
	LabelsRemoved []struct {
		Message  gmailHistoryMessage `json:"message"`
		LabelIDs []string            `json:"labelIds"`
	} `json:"labelsRemoved"`
}

// mailLabelEvent is one in-order label mutation observed in the history
// stream. full carries the message's resulting label set when Gmail included
// it on the record; add/remove are the per-record deltas used as fallback.
type mailLabelEvent struct {
	id          string
	full        []string
	add, remove []string
}

// mailIncrementalSync replays users.history.list from startHistoryID.
// Returns expired=true when Gmail reports the cursor is gone (HTTP 404) so
// the caller can fall back to a full sync.
func mailIncrementalSync(ctx context.Context, c *client.Client, db *store.Store, account, startHistoryID string, limit int, errW io.Writer) (mailSyncResult, bool, error) {
	res := mailSyncResult{}

	added := map[string]bool{}
	deleted := map[string]bool{}
	var events []mailLabelEvent
	latestHistoryID := ""

	pageToken := ""
	for {
		params := url.Values{}
		params.Set("startHistoryId", startHistoryID)
		params.Set("maxResults", strconv.Itoa(mailSyncPageSize))
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}
		data, err := c.GetWithHeadersValues(ctx, "/gmail/v1/users/me/history", params, nil)
		if err != nil {
			var apiErr *client.APIError
			if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
				return res, true, nil
			}
			return res, false, fmt.Errorf("listing history: %w", err)
		}
		var page struct {
			History       []gmailHistoryRecord `json:"history"`
			NextPageToken string               `json:"nextPageToken"`
			HistoryID     string               `json:"historyId"`
		}
		if err := json.Unmarshal(data, &page); err != nil {
			return res, false, fmt.Errorf("parsing history page: %w", err)
		}
		if page.HistoryID != "" {
			latestHistoryID = page.HistoryID
		}
		for _, rec := range page.History {
			for _, ma := range rec.MessagesAdded {
				if ma.Message.ID == "" {
					continue
				}
				added[ma.Message.ID] = true
				delete(deleted, ma.Message.ID)
			}
			for _, md := range rec.MessagesDeleted {
				if md.Message.ID == "" {
					continue
				}
				deleted[md.Message.ID] = true
				delete(added, md.Message.ID)
			}
			for _, la := range rec.LabelsAdded {
				if la.Message.ID == "" {
					continue
				}
				events = append(events, mailLabelEvent{id: la.Message.ID, full: la.Message.LabelIDs, add: la.LabelIDs})
			}
			for _, lr := range rec.LabelsRemoved {
				if lr.Message.ID == "" {
					continue
				}
				events = append(events, mailLabelEvent{id: lr.Message.ID, full: lr.Message.LabelIDs, remove: lr.LabelIDs})
			}
		}
		if page.NextPageToken == "" {
			break
		}
		pageToken = page.NextPageToken
	}

	// 1. Deletions: drop local rows (and their FTS shadows).
	if len(deleted) > 0 {
		ids := make([]string, 0, len(deleted))
		for id := range deleted {
			ids = append(ids, id)
		}
		n, err := db.DeleteMailMeta(account, ids)
		if err != nil {
			return res, false, fmt.Errorf("applying deletions: %w", err)
		}
		res.Deleted = n
		mailSyncProgress(errW, "deleted", n)
	}

	// 2. Additions: fetch metadata like the full path does.
	if len(added) > 0 {
		ids := make([]string, 0, len(added))
		for id := range added {
			ids = append(ids, id)
		}
		if len(ids) > limit {
			mailSyncNotice(errW, "limit_truncated",
				fmt.Sprintf("history reported %d new messages; fetching only --limit %d", len(ids), limit))
			ids = ids[:limit]
		}
		metas, fetched, err := mailFetchMetadata(ctx, c, account, ids, errW)
		if err != nil {
			return res, false, err
		}
		res.Fetched = fetched
		n, err := db.UpsertMailMeta(metas)
		if err != nil {
			return res, false, fmt.Errorf("upserting added messages: %w", err)
		}
		res.Upserted += n
		mailSyncProgress(errW, "upserted", n)
	}

	// 3. Label changes on rows we already hold. Events replay in stream
	// order; the record's full label set wins when present, otherwise the
	// add/remove delta is applied to the latest known set. Messages the
	// local window never held are skipped (they are outside the synced
	// window, not an error).
	labelState := map[string][]string{}
	missing := map[string]bool{}
	for _, ev := range events {
		if added[ev.id] || deleted[ev.id] || missing[ev.id] {
			continue
		}
		cur, ok := labelState[ev.id]
		if !ok {
			row, err := db.GetMailMeta(account, ev.id)
			if err != nil {
				missing[ev.id] = true
				continue
			}
			cur = row.LabelIDs
		}
		if ev.full != nil {
			cur = ev.full
		} else {
			cur = applyLabelDelta(cur, ev.add, ev.remove)
		}
		labelState[ev.id] = cur
	}
	labelUpdates := 0
	for id, labels := range labelState {
		found, err := db.UpdateMailLabels(account, id, labels, hasLabel(labels, "UNREAD"), deriveCategory(labels))
		if err != nil {
			return res, false, fmt.Errorf("applying label update for %s: %w", id, err)
		}
		if found {
			labelUpdates++
		}
	}
	if labelUpdates > 0 {
		res.Upserted += labelUpdates
		mailSyncProgress(errW, "labels_updated", labelUpdates)
	}
	if len(missing) > 0 {
		mailSyncNotice(errW, "labels_outside_window",
			fmt.Sprintf("%d label change(s) referenced messages outside the synced window; skipped", len(missing)))
	}

	if latestHistoryID != "" {
		res.HistoryID = latestHistoryID
	} else {
		res.HistoryID = startHistoryID
	}
	return res, false, nil
}

// applyLabelDelta returns cur + add - remove, preserving order and dropping
// duplicates.
func applyLabelDelta(cur, add, remove []string) []string {
	removeSet := make(map[string]bool, len(remove))
	for _, r := range remove {
		removeSet[r] = true
	}
	out := make([]string, 0, len(cur)+len(add))
	seen := map[string]bool{}
	for _, l := range cur {
		if removeSet[l] || seen[l] {
			continue
		}
		seen[l] = true
		out = append(out, l)
	}
	for _, l := range add {
		if removeSet[l] || seen[l] {
			continue
		}
		seen[l] = true
		out = append(out, l)
	}
	return out
}
