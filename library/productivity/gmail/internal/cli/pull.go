// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: mailbox mirror. messages.list returns only ids, so the
// framework sync cannot hydrate a useful local store for Gmail; pull lists ids,
// batch-fetches full messages, flattens headers + decoded body text, and
// upserts them locally. Incremental runs ride history.list cursors (lieer's
// niche) and fall back to a windowed resync when the cursor expires (~1 week).
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/gmailmail"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newPullCmd(flags))
	})
}

type pullResult struct {
	Mode          string `json:"mode"` // full | incremental
	Messages      int    `json:"messages_stored"`
	Labels        int    `json:"labels_stored"`
	Skipped       int    `json:"skipped"`
	Deleted       int    `json:"deleted,omitempty"`
	HistoryID     string `json:"history_id"`
	WindowQuery   string `json:"window_query,omitempty"`
	CursorExpired bool   `json:"cursor_expired,omitempty"`
}

// pullStoreMessages hydrates ids (full format: headers + body for FTS) and
// upserts flattened docs. Returns stored and skipped counts.
func pullStoreMessages(cmd *cobra.Command, c *client.Client, db *store.Store, ids []string, metadataOnly bool) (int, int, error) {
	if len(ids) == 0 {
		return 0, 0, nil
	}
	format := "full"
	if metadataOnly {
		format = "metadata"
	}
	msgs, skipped, err := gmailmail.BatchGetMessages(cmd.Context(), c, ids, format, gmailmail.DefaultMetadataHeaders)
	if err != nil {
		return 0, skipped, err
	}
	var skipReason error
	if skipped > 0 {
		skipReason = fmt.Errorf("%d message(s) were not returned by the batch endpoint", skipped)
	}
	stored := 0
	for i := range msgs {
		m := &msgs[i]
		body := ""
		if !metadataOnly {
			body = m.BodyText()
			const maxBody = 64 * 1024 // keep FTS useful without bloating the DB
			if len(body) > maxBody {
				body = body[:maxBody]
			}
		}
		doc, err := m.Flatten(body)
		if err != nil {
			skipped++
			firstSkipReason(&skipReason, fmt.Errorf("flatten %s: %w", m.ID, err))
			continue
		}
		if err := db.UpsertMessages(doc); err != nil {
			skipped++
			firstSkipReason(&skipReason, fmt.Errorf("store %s: %w", m.ID, err))
			continue
		}
		stored++
	}
	// A silent skip count is indistinguishable from "nothing was wrong";
	// surface the first cause so the number is actionable.
	if skipReason != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "note: %d message(s) skipped; first cause: %v\n", skipped, skipReason)
	}
	return stored, skipped, nil
}

func firstSkipReason(dst *error, err error) {
	if *dst == nil {
		*dst = err
	}
}

// pullLabels mirrors the label list (id, name, counts) for name resolution.
func pullLabels(cmd *cobra.Command, c *client.Client, db *store.Store) (int, error) {
	data, err := c.Get(cmd.Context(), "/gmail/v1/users/me/labels", nil)
	if err != nil {
		return 0, err
	}
	var page struct {
		Labels []json.RawMessage `json:"labels"`
	}
	if err := json.Unmarshal(data, &page); err != nil {
		return 0, fmt.Errorf("parsing labels.list: %w", err)
	}
	stored := 0
	for _, l := range page.Labels {
		if err := db.UpsertLabels(l); err == nil {
			stored++
		}
	}
	return stored, nil
}

// historyChangedIDs walks history.list from cursor, returning changed message
// ids, deleted ids, and the newest historyId. expired=true means HTTP 404:
// the cursor is too old and a windowed resync is required.
func historyChangedIDs(cmd *cobra.Command, c *client.Client, cursor string) (changed, deleted []string, newest string, expired bool, err error) {
	seen := map[string]bool{}
	gone := map[string]bool{}
	pageToken := ""
	newest = cursor
	for {
		params := map[string]string{"startHistoryId": cursor, "maxResults": "500"}
		if pageToken != "" {
			params["pageToken"] = pageToken
		}
		data, err := c.Get(cmd.Context(), "/gmail/v1/users/me/history", params)
		if err != nil {
			// 404: cursor older than history retention (~1 week). 400: cursor
			// is not a valid historyId (e.g. another tool overwrote the sync
			// state). Both mean "start over with a windowed resync". Match on
			// the typed status, not the message text, so ids or byte counts
			// containing "400" cannot trigger a spurious full resync.
			var apiErr *client.APIError
			if errors.As(err, &apiErr) && (apiErr.StatusCode == 404 || apiErr.StatusCode == 400) {
				return nil, nil, cursor, true, nil
			}
			return nil, nil, cursor, false, err
		}
		var page struct {
			History []struct {
				Messages []struct {
					ID string `json:"id"`
				} `json:"messages"`
				MessagesDeleted []struct {
					Message struct {
						ID string `json:"id"`
					} `json:"message"`
				} `json:"messagesDeleted"`
			} `json:"history"`
			NextPageToken string `json:"nextPageToken"`
			HistoryID     string `json:"historyId"`
		}
		if err := json.Unmarshal(data, &page); err != nil {
			return nil, nil, cursor, false, fmt.Errorf("parsing history.list: %w", err)
		}
		if page.HistoryID != "" {
			newest = page.HistoryID
		}
		for _, h := range page.History {
			for _, m := range h.Messages {
				seen[m.ID] = true
			}
			for _, d := range h.MessagesDeleted {
				gone[d.Message.ID] = true
			}
		}
		if page.NextPageToken == "" {
			break
		}
		pageToken = page.NextPageToken
	}
	for id := range seen {
		if !gone[id] {
			changed = append(changed, id)
		}
	}
	for id := range gone {
		deleted = append(deleted, id)
	}
	return changed, deleted, newest, false, nil
}

func newPullCmd(flags *rootFlags) *cobra.Command {
	var since string
	var query string
	var limit int
	var full bool
	var metadataOnly bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Mirror the mailbox into local SQLite (headers, labels, decoded bodies)",
		Long: `First run: lists messages in the --since window, batch-fetches each one,
and stores flattened headers plus decoded body text locally. Later runs use
the saved history cursor for cheap incremental updates (2 quota units per
page) and fall back to a windowed resync when the cursor expires.

The local mirror powers search, senders, storage, followups, and unsub.`,
		Example: strings.Trim(`
  gmail-pp-cli pull --since 30d
  gmail-pp-cli pull --query "category:primary" --since 90d --limit 2000
  gmail-pp-cli pull --full --since 7d`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "mirror mailbox messages into the local store")
			}
			return runMailboxPull(cmd, flags, mailboxPullOptions{
				Since:        since,
				Query:        query,
				Limit:        limit,
				Full:         full,
				MetadataOnly: metadataOnly,
				DBPath:       dbPath,
			})
		},
	}
	cmd.Flags().StringVar(&since, "since", "30d", "Window for the first/full sync (e.g. 7d, 24h, 12w)")
	cmd.Flags().StringVar(&query, "query", "", "Extra Gmail query to scope the full sync (e.g. category:primary)")
	cmd.Flags().IntVar(&limit, "limit", 5000, "Maximum messages to mirror in one run")
	cmd.Flags().BoolVar(&full, "full", false, "Force a windowed resync even when an incremental cursor exists")
	cmd.Flags().BoolVar(&metadataOnly, "metadata-only", false, "Skip body download (faster; disables body full-text search)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: resolved data directory data.db)")
	return cmd
}

// mailboxPullOptions parameterizes the mailbox mirror flow shared by the
// pull command and the bare `sync` delegation.
type mailboxPullOptions struct {
	Since        string
	Query        string
	Limit        int
	Full         bool
	MetadataOnly bool
	DBPath       string
}

// runMailboxPull is the mailbox mirror core: list ids in the window (or walk
// the history cursor incrementally), batch-hydrate full messages, and upsert
// flattened docs plus labels into the local store. Extracted from the pull
// command so a bare `sync` (Gmail's list endpoints return id stubs only, so
// the generic resource sync cannot hydrate a useful mirror) can delegate here.
func runMailboxPull(cmd *cobra.Command, flags *rootFlags, opts mailboxPullOptions) error {
	since, query, limit := opts.Since, opts.Query, opts.Limit
	full, metadataOnly, dbPath := opts.Full, opts.MetadataOnly, opts.DBPath
	{
		d, err := cliutil.ParseDurationLoose(since)
		if err != nil {
			_ = cmd.Usage()
			return usageErr(fmt.Errorf("invalid --since duration %q: %w", since, err))
		}
		if cliutil.IsDogfoodEnv() && limit > 25 {
			limit = 25 // curtail live-matrix work; a real pull uses the full window
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		db, err := openGmailStore(cmd.Context(), dbPath)
		if err != nil {
			return fmt.Errorf("opening database: %w", err)
		}
		defer db.Close()

		res := pullResult{}
		cursor, _, _, _ := db.GetSyncState("messages")

		if cursor != "" && !full {
			changed, deleted, newest, expired, err := historyChangedIDs(cmd, c, cursor)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if !expired {
				res.Mode = "incremental"
				stored, skipped, err := pullStoreMessages(cmd, c, db, changed, metadataOnly)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				res.Messages, res.Skipped = stored, skipped
				// Drop messages deleted upstream so they stop inflating
				// senders/storage/unsub/followups results.
				for _, id := range deleted {
					if _, err := db.DB().ExecContext(cmd.Context(), `DELETE FROM messages WHERE id = ?`, id); err == nil {
						res.Deleted++
					}
				}
				res.HistoryID = newest
				if n, err := pullLabels(cmd, c, db); err == nil {
					res.Labels = n
				}
				if err := db.SaveSyncState("messages", newest, stored); err != nil {
					return fmt.Errorf("saving sync cursor: %w", err)
				}
				return pullReport(cmd, flags, res)
			}
			res.CursorExpired = true
		}

		// Full / windowed sync. Capture the history cursor BEFORE listing:
		// a message arriving mid-listing would otherwise be missed by this
		// run and skipped by later incrementals starting from a newer
		// cursor. Taking it first means the next incremental replays it.
		res.Mode = "full"
		_, historyID, err := gmailProfile(cmd.Context(), c)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		days := int(math.Ceil(d.Hours() / 24))
		if days < 1 {
			days = 1
		}
		windowQuery := fmt.Sprintf("newer_than:%dd", days)
		if query != "" {
			windowQuery = query + " " + windowQuery
		}
		res.WindowQuery = windowQuery
		ids, err := gmailListIDs(cmd.Context(), c, windowQuery, limit)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		stored, skipped, err := pullStoreMessages(cmd, c, db, ids, metadataOnly)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		res.Messages, res.Skipped = stored, skipped
		if n, err := pullLabels(cmd, c, db); err == nil {
			res.Labels = n
		}
		res.HistoryID = historyID
		if err := db.SaveSyncState("messages", historyID, stored); err != nil {
			return fmt.Errorf("saving sync cursor: %w", err)
		}
		return pullReport(cmd, flags, res)
	}
}

func pullReport(cmd *cobra.Command, flags *rootFlags, res pullResult) error {
	if wantsJSONOutput(cmd, flags) {
		return printJSONFiltered(cmd.OutOrStdout(), res, flags)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "pull (%s): %d messages stored, %d labels, %d skipped (history %s)\n",
		res.Mode, res.Messages, res.Labels, res.Skipped, res.HistoryID)
	if res.CursorExpired {
		fmt.Fprintln(cmd.ErrOrStderr(), "note: history cursor had expired; ran a windowed resync")
	}
	return nil
}
