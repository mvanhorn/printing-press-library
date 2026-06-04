// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// send_queue.go implements the local --undo queue plus the `unsend` command.
//
// KD6 design choice (from plan 2026-05-14-002): the CLI does NOT use the
// server-side `delay` field in /messages/send because there is no
// documented endpoint to abort an in-flight send. Instead, the CLI:
//
//  1. Executes steps 1 + 2 (writeMessage + send/log) immediately.
//  2. Persists the OutgoingMessage envelope to a local queue file.
//  3. Blocks the foreground process for the --undo duration showing a
//     countdown timer on stderr.
//  4. On expiry, fires step 3 (POST /messages/send) and exits.
//  5. On Ctrl-C, marks the entry cancelled and exits without firing.
//
// V1 limitation: closing the terminal cancels the send (process exit
// short-circuits the timer). This is the trade-off accepted in KD6 — a
// background daemon would survive terminal close but adds operational
// surface for a v1 ship. Documented in --help so the user can't be
// surprised.
//
// `unsend [queue-id]` marks a pending entry cancelled. Without an arg, the
// most-recently-enqueued pending entry is cancelled. Cancellation is a soft
// marker on the queue file — the running send process polls the file and
// short-circuits if its own queue id has transitioned to cancelled.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/auth"
	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/client"
)

// sendQueueFilename is the on-disk basename of the queue file. Lives next to
// tokens.json under ~/.config/superhuman-pp-cli/.
const sendQueueFilename = "send-queue.json"

// queueStatusPending is the initial status for every enqueued send. Two
// terminal statuses replace it: "fired" after step 3 succeeds, "cancelled"
// after `unsend` (or Ctrl-C) marks it dead.
const (
	queueStatusPending   = "pending"
	queueStatusFired     = "fired"
	queueStatusCancelled = "cancelled"
)

// sendQueueEntry is one row in the queue file. OutgoingMessage is the full
// JSON envelope captured at enqueue time so the eventual fire path doesn't
// have to re-resolve any sender state.
type sendQueueEntry struct {
	QueueID         string          `json:"queue_id"`
	Account         string          `json:"account"`
	OutgoingMessage outgoingMessage `json:"outgoing_message"`
	FireAtEpochMs   int64           `json:"fire_at_epoch_ms"`
	EnqueuedAtMs    int64           `json:"enqueued_at_ms"`
	Status          string          `json:"status"`
}

// sendQueueFile is the top-level on-disk shape. A single file is shared
// across accounts — the per-entry Account field disambiguates.
type sendQueueFile struct {
	Entries []sendQueueEntry `json:"entries"`
}

// queueMu serializes in-process queue mutations. Cross-process safety relies
// on rename(2) atomicity at save time; concurrent saves from the same process
// are coordinated by this mutex.
var queueMu sync.Mutex

// sendQueuePath returns the on-disk path for the queue file. Honors
// $XDG_CONFIG_HOME and falls back to ~/.config (same convention as tokens.json).
func sendQueuePath() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "superhuman-pp-cli", sendQueueFilename), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("send queue: resolve home: %w", err)
	}
	return filepath.Join(home, ".config", "superhuman-pp-cli", sendQueueFilename), nil
}

// loadSendQueue reads the queue file. Missing file is not an error — return
// an empty queue so the first enqueue can proceed without an explicit init.
func loadSendQueue(path string) (*sendQueueFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &sendQueueFile{Entries: []sendQueueEntry{}}, nil
		}
		return nil, fmt.Errorf("send queue: read %s: %w", path, err)
	}
	var q sendQueueFile
	if err := json.Unmarshal(data, &q); err != nil {
		return nil, fmt.Errorf("send queue: parse %s: %w", path, err)
	}
	if q.Entries == nil {
		q.Entries = []sendQueueEntry{}
	}
	return &q, nil
}

// saveSendQueue atomically writes the queue file. Same write pattern as the
// token store: tmp file + rename, mode 0600.
func saveSendQueue(path string, q *sendQueueFile) error {
	queueMu.Lock()
	defer queueMu.Unlock()

	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return fmt.Errorf("send queue: mkdir %s: %w", parent, err)
	}
	data, err := json.MarshalIndent(q, "", "  ")
	if err != nil {
		return fmt.Errorf("send queue: marshal: %w", err)
	}
	tmp, err := os.CreateTemp(parent, sendQueueFilename+".*.tmp")
	if err != nil {
		return fmt.Errorf("send queue: create tmp: %w", err)
	}
	tmpPath := tmp.Name()
	if _, werr := tmp.Write(data); werr != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("send queue: write tmp: %w", werr)
	}
	if cerr := tmp.Close(); cerr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("send queue: close tmp: %w", cerr)
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("send queue: chmod tmp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("send queue: rename: %w", err)
	}
	return nil
}

// enqueueWithUndo persists a pending entry, blocks foreground for the undo
// duration showing a countdown on stderr, and either fires step 3 on expiry
// or short-circuits if the entry transitioned to cancelled mid-wait.
//
// The fire/cancel decision polls the queue file every 250ms during the wait
// so an out-of-process `unsend` lands quickly. The same call also installs a
// SIGINT/SIGTERM handler so Ctrl-C cleanly marks the entry cancelled before
// exit — without it, Ctrl-C would leave a "pending" entry stranded.
//
// Delivery uses Gmail API (sendViaGmailAPI), not Superhuman's /messages/send
// (see send.go's runSend docstring for why). The caller passes the OAuth
// access token + the precomposed sendInputs envelope used to assemble the
// RFC822 body.
func enqueueWithUndo(cmd *cobra.Command, c *client.Client, account, googleID, accessToken, fromDisplay string, store *auth.Store, in sendInputs, om outgoingMessage, undo time.Duration) error {
	now := time.Now()
	entry := sendQueueEntry{
		QueueID:         uuid.NewString(),
		Account:         account,
		OutgoingMessage: om,
		FireAtEpochMs:   now.Add(undo).UnixMilli(),
		EnqueuedAtMs:    now.UnixMilli(),
		Status:          queueStatusPending,
	}

	path, err := sendQueuePath()
	if err != nil {
		return err
	}
	q, err := loadSendQueue(path)
	if err != nil {
		return err
	}
	q.Entries = append(q.Entries, entry)
	if err := saveSendQueue(path, q); err != nil {
		return err
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Queued for send in %s (queueId=%s). Run 'superhuman-pp-cli unsend %s' or Ctrl-C to abort.\n",
		undo, entry.QueueID, entry.QueueID)

	// Install signal handler so Ctrl-C transitions the entry to cancelled
	// rather than leaving a stranded pending row. The handler runs only for
	// the duration of this wait; Stop() detaches it on return.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	fireAt := time.UnixMilli(entry.FireAtEpochMs)

	for {
		select {
		case <-sigCh:
			// Ctrl-C / SIGTERM: mark cancelled and exit cleanly.
			if uerr := updateQueueStatus(path, entry.QueueID, queueStatusCancelled); uerr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to mark queue entry cancelled: %v\n", uerr)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "\nSend cancelled (queueId=%s)\n", entry.QueueID)
			return nil
		case <-ticker.C:
			// Reload the queue to detect an out-of-process `unsend`.
			fresh, lerr := loadSendQueue(path)
			if lerr == nil {
				if status := findStatus(fresh, entry.QueueID); status == queueStatusCancelled {
					fmt.Fprintf(cmd.ErrOrStderr(), "\nSend cancelled by 'unsend' (queueId=%s)\n", entry.QueueID)
					return nil
				}
			}
			remaining := time.Until(fireAt)
			if remaining <= 0 {
				// Time to fire. Re-check status once more under the lock to
				// race-resolve a cancellation that lands at the exact same
				// instant the timer fires.
				if uerr := updateQueueStatus(path, entry.QueueID, queueStatusFired); uerr != nil {
					return apiErr(fmt.Errorf("send: mark fired: %w", uerr))
				}
				ctx := cmd.Context()
				if ctx == nil {
					ctx = context.Background()
				}
				gmailID, serr := sendGmailWithRefresh(ctx, cmd.ErrOrStderr(), store, account, googleID, accessToken, fromDisplay, in)
				if serr != nil {
					return apiErr(fmt.Errorf("send step 3 (gmail api): %w", serr))
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Sent. send_at=%d, draftId=%s, gmailId=%s\n", time.Now().Unix(), om.MessageID, gmailID)
				return nil
			}
			// Render a coarse countdown on stderr (1-second resolution; the
			// 250ms tick is for cancel-detection responsiveness).
			fmt.Fprintf(cmd.ErrOrStderr(), "\rSending in %ds...   ", int(remaining.Seconds())+1)
		}
	}
}

// findStatus returns the Status field for the entry with the given queue id,
// or "" if not present.
func findStatus(q *sendQueueFile, queueID string) string {
	for _, e := range q.Entries {
		if e.QueueID == queueID {
			return e.Status
		}
	}
	return ""
}

// updateQueueStatus loads, mutates one entry's Status, and saves. No-op if
// the entry has already transitioned (e.g., a duplicate Ctrl-C after fire).
func updateQueueStatus(path, queueID, status string) error {
	q, err := loadSendQueue(path)
	if err != nil {
		return err
	}
	for i := range q.Entries {
		if q.Entries[i].QueueID == queueID {
			// Don't overwrite a terminal status (fired/cancelled) with a
			// later state — first-writer-wins keeps the queue file truthful
			// even if two paths race to update.
			cur := q.Entries[i].Status
			if cur == queueStatusFired || cur == queueStatusCancelled {
				return nil
			}
			q.Entries[i].Status = status
			return saveSendQueue(path, q)
		}
	}
	return fmt.Errorf("send queue: entry %s not found", queueID)
}

// newUnsendCmd registers `superhuman-pp-cli unsend [queue-id]`. With no
// argument, cancels the most-recently-enqueued pending entry.
func newUnsendCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unsend [queue-id]",
		Short: "Cancel a pending send (the most recent if no id is given)",
		Long: `Cancel a pending send queued by 'send --undo <duration>'.

With no argument, the most-recently-enqueued pending entry is cancelled.
With a queue id, that specific entry is cancelled.

Note: the running 'send' process polls the queue file every 250ms, so a
cancellation lands within ~250ms — well under any realistic undo window.`,
		Example: `  superhuman-pp-cli unsend
  superhuman-pp-cli unsend 0d27e9b1-1c3a-4e2f-9d62-3c8d2a7e9b1f`,
		Annotations: map[string]string{
			"pp:typed-exit-codes": "0,2,3,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUnsend(cmd, args)
		},
	}
	return cmd
}

// runUnsend is the verifiable RunE body.
func runUnsend(cmd *cobra.Command, args []string) error {
	path, err := sendQueuePath()
	if err != nil {
		return apiErr(err)
	}
	q, err := loadSendQueue(path)
	if err != nil {
		return apiErr(err)
	}
	if len(q.Entries) == 0 {
		return notFoundErr(fmt.Errorf("unsend: no entries in queue"))
	}

	var target string
	if len(args) == 1 {
		target = args[0]
	} else {
		// Pick the most-recently-enqueued PENDING entry. Sorting by
		// EnqueuedAtMs desc means index 0 wins.
		pending := make([]sendQueueEntry, 0, len(q.Entries))
		for _, e := range q.Entries {
			if e.Status == queueStatusPending {
				pending = append(pending, e)
			}
		}
		if len(pending) == 0 {
			return notFoundErr(fmt.Errorf("unsend: no pending entries (all fired or cancelled)"))
		}
		sort.Slice(pending, func(i, j int) bool {
			return pending[i].EnqueuedAtMs > pending[j].EnqueuedAtMs
		})
		target = pending[0].QueueID
	}

	if err := updateQueueStatus(path, target, queueStatusCancelled); err != nil {
		return notFoundErr(fmt.Errorf("unsend: %w", err))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Cancelled queueId=%s\n", target)
	return nil
}
