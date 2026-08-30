// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written `undo`: delta-based reversal of a ledgered apply (grill
// R1-C6, R3-C3). Per id it verifies the current live state still carries
// the introduced delta; ids whose state changed since are SKIPPED and
// reported as conflicts — never forced.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"sync"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"
)

// undoConflict is one skipped id and why.
type undoConflict struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// undoResult is undo's JSON envelope.
type undoResult struct {
	LedgerID  string         `json:"ledger_id"`
	Account   string         `json:"account"`
	Total     int            `json:"total"`
	Undone    int            `json:"undone"`
	Conflicts []undoConflict `json:"conflicts"`
	Failed    int            `json:"failed"`
	FailedIDs []string       `json:"failed_ids,omitempty"`
}

func newUndoCmd(flags *rootFlags) *cobra.Command {
	var ledgerID string

	cmd := &cobra.Command{
		Use:   "undo",
		Short: "Reverse a ledgered apply delta-by-delta; ids whose state changed since are skipped as conflicts, never forced",
		Long: `Reverse exactly the deltas a previous apply (or label rename) ledgered.

Per id, the CURRENT live state must still carry the introduced delta:
  trash entries   TRASH must still be present -> untrash, THEN re-add any
                  recorded pre-placement labels (INBOX/CATEGORY_* the plan
                  snapshotted) that are now missing;
  label entries   every added label must still be present and every removed
                  label still absent -> the exact add/remove lists are
                  reversed via messages.modify;
  rename entries  the label's current name must still equal the recorded
                  new name -> renamed back to the recorded old name.

Anything that changed since the apply is SKIPPED and reported as a
conflict — undo never forces a state it cannot prove it created. Entries
already undone report as conflicts too ("already undone").

Same discipline as apply: advisory flock (busy = exit 7), identity
preflight (mismatch = exit 4), 8-worker pool, 30s/5-attempt call budget.

Typed exits: 0 complete (conflicts allowed) / 2 usage / 4 refused
(unknown ledger, identity) / 5 auth or API failure / 7 lock busy.`,
		Example: `  # ledger ids are printed by 'cleanup apply' and 'labels rename'
  gmail-pp-cli undo --ledger 3f9c01ab52de77c0

  # See what a ledger holds before undoing (local read, no network)
  gmail-pp-cli undo --ledger 3f9c01ab52de77c0 --dry-run`,
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,4,5,7"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if ledgerID == "" {
				return usageErr(fmt.Errorf("--ledger <id> is required (printed by 'cleanup apply' / 'labels rename')"))
			}
			ctx := cmd.Context()
			db, err := store.OpenWithContext(ctx, defaultDBPath("gmail-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			ledger, err := db.GetMailLedger(ledgerID)
			if errors.Is(err, sql.ErrNoRows) {
				return refusedErr(fmt.Errorf("no ledger %q in the local store — nothing to undo", ledgerID))
			}
			if err != nil {
				return err
			}
			entries, err := db.ListMailLedgerEntries(ledgerID)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				return refusedErr(fmt.Errorf("ledger %q holds no entries — nothing to undo", ledgerID))
			}
			if flags.account != "" && flags.account != ledger.Account {
				return refusedErr(fmt.Errorf("--account %q does not match the ledger's account %q", flags.account, ledger.Account))
			}
			flags.account = ledger.Account
			if _, err := gauthProfile(flags, ledger.Account); err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}

			authDir := gauthConfigDirFrom(flags.authDir)
			release, err := acquireApplyLock(authDir)
			if err != nil {
				return err
			}
			defer release()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true
			if err := verifyLiveIdentity(ctx, c, flags, ledger.Account); err != nil {
				return classifyEngineAPIError(err)
			}

			res := undoLedgerEntries(ctx, c, db, ledger, entries)
			if perr := printJSONFiltered(cmd.OutOrStdout(), res, flags); perr != nil {
				return perr
			}
			if res.Failed > 0 {
				return partialApplyErr(fmt.Errorf("undo finished partial: %d undone, %d conflicts skipped, %d failed — re-run to retry the failures", res.Undone, len(res.Conflicts), res.Failed))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&ledgerID, "ledger", "", "Ledger id to reverse (from 'cleanup apply' / 'labels rename' output)")
	return cmd
}

// undoLedgerEntries reverses each entry through the worker pool (renames
// run inline — a ledger holds at most one). Conflict = skip + report.
func undoLedgerEntries(ctx context.Context, c *client.Client, db *store.Store, ledger store.MailLedger, entries []store.MailLedgerEntry) undoResult {
	res := undoResult{LedgerID: ledger.LedgerID, Account: ledger.Account, Total: len(entries), Conflicts: []undoConflict{}}

	type entryResult struct {
		id       string
		undone   bool
		conflict string
		err      error
	}
	results := make(chan entryResult, len(entries))
	work := make(chan store.MailLedgerEntry)
	var wg sync.WaitGroup
	workers := cleanupWorkers
	if workers > len(entries) {
		workers = len(entries)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for e := range work {
				id, undone, conflict, err := undoOneEntry(ctx, c, e)
				results <- entryResult{id: id, undone: undone, conflict: conflict, err: err}
			}
		}()
	}
	for _, e := range entries {
		work <- e
	}
	close(work)
	wg.Wait()
	close(results)

	perID := map[string]entryResult{}
	for r := range results {
		perID[r.id] = r
	}
	// Deterministic reporting + durable per-entry outcome stamps.
	ids := make([]string, 0, len(perID))
	for id := range perID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		r := perID[id]
		switch {
		case r.undone:
			res.Undone++
			_ = db.SetMailLedgerEntryUndone(ledger.LedgerID, id, "undone")
		case r.conflict != "":
			res.Conflicts = append(res.Conflicts, undoConflict{ID: id, Reason: r.conflict})
			_ = db.SetMailLedgerEntryUndone(ledger.LedgerID, id, "conflict")
		default:
			res.Failed++
			res.FailedIDs = append(res.FailedIDs, id)
		}
	}
	return res
}

// undoOneEntry reverses one ledger entry. Returns (id, undone, conflictReason, err).
func undoOneEntry(ctx context.Context, c *client.Client, e store.MailLedgerEntry) (string, bool, string, error) {
	if e.Undone == "undone" {
		return e.ID, false, "already undone by a previous undo run", nil
	}
	switch e.Kind {
	case "trash":
		return undoTrashEntry(ctx, c, e)
	case "label":
		return undoLabelEntry(ctx, c, e)
	case "label_rename":
		return undoRenameEntry(ctx, c, e)
	default:
		return e.ID, false, fmt.Sprintf("unknown ledger entry kind %q", e.Kind), nil
	}
}

func undoTrashEntry(ctx context.Context, c *client.Client, e store.MailLedgerEntry) (string, bool, string, error) {
	labels, found, err := fetchMessageLabelIDs(ctx, c, e.ID)
	if err != nil {
		return e.ID, false, "", err
	}
	if !found {
		return e.ID, false, "message no longer exists (purged or deleted since the apply)", nil
	}
	if !hasLabel(labels, "TRASH") {
		return e.ID, false, "TRASH is no longer present — the message was already untrashed or moved since the apply", nil
	}
	if err := engineCall(ctx, func(cctx context.Context) error {
		_, _, perr := c.Post(cctx, mailMetadataFetchPath+url.PathEscape(e.ID)+"/untrash", struct{}{})
		return perr
	}); err != nil {
		return e.ID, false, "", err
	}
	// Restore recorded pre-placement labels now missing (grill R3-C3):
	// untrash removes TRASH but does NOT restore INBOX/CATEGORY_*.
	post := map[string]bool{}
	for _, l := range labels {
		if l != "TRASH" {
			post[l] = true
		}
	}
	var missing []string
	for _, l := range e.PrePlacement {
		if !post[l] {
			missing = append(missing, l)
		}
	}
	if len(missing) > 0 {
		if err := engineCall(ctx, func(cctx context.Context) error {
			_, _, perr := c.Post(cctx, mailMetadataFetchPath+url.PathEscape(e.ID)+"/modify",
				map[string]any{"addLabelIds": missing})
			return perr
		}); err != nil {
			return e.ID, false, "", fmt.Errorf("untrashed, but restoring placement labels failed: %w", err)
		}
	}
	return e.ID, true, "", nil
}

func undoLabelEntry(ctx context.Context, c *client.Client, e store.MailLedgerEntry) (string, bool, string, error) {
	labels, found, err := fetchMessageLabelIDs(ctx, c, e.ID)
	if err != nil {
		return e.ID, false, "", err
	}
	if !found {
		return e.ID, false, "message no longer exists (purged or deleted since the apply)", nil
	}
	for _, a := range e.DeltaAdd {
		if !hasLabel(labels, a) {
			return e.ID, false, fmt.Sprintf("introduced label %s is no longer present — state changed since the apply", a), nil
		}
	}
	for _, r := range e.DeltaRemove {
		if hasLabel(labels, r) {
			return e.ID, false, fmt.Sprintf("removed label %s was re-added since the apply", r), nil
		}
	}
	body := map[string]any{}
	if len(e.DeltaRemove) > 0 {
		body["addLabelIds"] = e.DeltaRemove
	}
	if len(e.DeltaAdd) > 0 {
		body["removeLabelIds"] = e.DeltaAdd
	}
	if err := engineCall(ctx, func(cctx context.Context) error {
		_, _, perr := c.Post(cctx, mailMetadataFetchPath+url.PathEscape(e.ID)+"/modify", body)
		return perr
	}); err != nil {
		return e.ID, false, "", err
	}
	return e.ID, true, "", nil
}

func undoRenameEntry(ctx context.Context, c *client.Client, e store.MailLedgerEntry) (string, bool, string, error) {
	var current struct {
		Name string `json:"name"`
	}
	err := engineCall(ctx, func(cctx context.Context) error {
		data, gerr := c.GetWithHeadersNoCache(cctx, "/gmail/v1/users/me/labels/"+url.PathEscape(e.ID), nil, nil)
		if gerr != nil {
			return gerr
		}
		return json.Unmarshal(data, &current)
	})
	if err != nil {
		if apiStatus(err) == 404 {
			return e.ID, false, "label no longer exists", nil
		}
		return e.ID, false, "", err
	}
	if current.Name != e.NewName {
		return e.ID, false, fmt.Sprintf("label is now named %q, not the recorded %q — renamed again since; not forcing", current.Name, e.NewName), nil
	}
	if err := engineCall(ctx, func(cctx context.Context) error {
		_, _, perr := c.Patch(cctx, "/gmail/v1/users/me/labels/"+url.PathEscape(e.ID), map[string]any{"name": e.OldName})
		return perr
	}); err != nil {
		return e.ID, false, "", err
	}
	return e.ID, true, "", nil
}
