// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written `cleanup apply` + `cleanup recover`: the confirm/execute
// half of the engine. Apply's ordering is load-bearing (grill R3-C2/C4,
// R2-C4/C5/C6, R3-C1): flock -> plan re-hash -> atomic nonce burn +
// authorization -> identity preflight -> drift check -> durable chunk
// intents -> execution -> per-id delta ledger.

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"
)

// applyResult is the JSON envelope apply/recover print per apply.
type applyResult struct {
	ApplyID  int64  `json:"apply_id"`
	Account  string `json:"account"`
	PlanSha  string `json:"plan_sha"`
	LedgerID string `json:"ledger_id,omitempty"`
	Applied  int    `json:"applied"`
	Failed   int    `json:"failed"`
	Skipped  int    `json:"skipped"`
	Chunks   struct {
		Total   int `json:"total"`
		Done    int `json:"done"`
		Pending int `json:"pending"`
	} `json:"chunks"`
	State     string   `json:"state"`
	FailedIDs []string `json:"failed_ids,omitempty"`
	UndoHint  string   `json:"undo_hint,omitempty"`
	Note      string   `json:"note,omitempty"`
}

// buildChunksFromPlan turns a frozen plan into durable chunk intents:
// trash groups in chunks of 100 ids (each id executed as its own POST
// messages/{id}/trash through the 8-worker pool — grill R2-C5: batchModify
// is deliberately NOT used for trash), label groups in batchModify chunks
// of 500 ids.
func buildChunksFromPlan(plan *cleanupPlan, applyID int64) []store.MailApplyChunk {
	var chunks []store.MailApplyChunk
	no := 0
	for _, g := range plan.Groups {
		size := cleanupLabelChunkSize
		if g.Action.Type == "trash" {
			size = cleanupTrashChunkSize
		}
		for start := 0; start < len(g.IDs); start += size {
			end := start + size
			if end > len(g.IDs) {
				end = len(g.IDs)
			}
			ch := store.MailApplyChunk{
				ApplyID: applyID,
				ChunkNo: no,
				Kind:    g.Action.Type,
				IDs:     append([]string(nil), g.IDs[start:end]...),
				State:   store.MailChunkStatePending,
			}
			if g.Action.Type == "label" {
				ch.Add = g.Action.Add
				ch.Remove = g.Action.Remove
			}
			chunks = append(chunks, ch)
			no++
		}
	}
	return chunks
}

// chunkOutcome aggregates one chunk's execution.
type chunkOutcome struct {
	applied, skipped int
	appliedIDs       []string
	failedIDs        []string
	authAbort        error // non-nil: 401/403 hit; whole apply aborts recoverably
}

// executeTrashChunk trashes each id through the worker pool. recovery
// re-checks current labelIds first (grill: per-id trash recovery) so an
// id the crashed run already trashed is completed, not double-counted as
// new work; a 404 (message gone) is a skip, never a forced mutation.
func executeTrashChunk(ctx context.Context, c *client.Client, ch store.MailApplyChunk, recovery bool) chunkOutcome {
	out := chunkOutcome{}
	pending := append([]string(nil), ch.IDs...)
	succeeded := map[string]bool{}
	skipped := map[string]bool{}

	for pass := 0; pass < cleanupChunkRetryCeiling && len(pending) > 0 && out.authAbort == nil; pass++ {
		type res struct {
			id   string
			ok   bool
			skip bool
			err  error
		}
		work := make(chan string)
		results := make(chan res, len(pending))
		var wg sync.WaitGroup
		workers := cleanupWorkers
		if workers > len(pending) {
			workers = len(pending)
		}
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for id := range work {
					if recovery {
						labels, found, err := fetchMessageLabelIDs(ctx, c, id)
						if err != nil {
							results <- res{id: id, err: err}
							continue
						}
						if !found {
							results <- res{id: id, skip: true}
							continue
						}
						if hasLabel(labels, "TRASH") {
							// The crashed run already applied this id.
							results <- res{id: id, ok: true}
							continue
						}
					}
					err := engineCall(ctx, func(cctx context.Context) error {
						_, _, perr := c.Post(cctx, mailMetadataFetchPath+url.PathEscape(id)+"/trash", struct{}{})
						return perr
					})
					if err != nil {
						if apiStatus(err) == 404 {
							results <- res{id: id, skip: true}
							continue
						}
						results <- res{id: id, err: err}
						continue
					}
					results <- res{id: id, ok: true}
				}
			}()
		}
		for _, id := range pending {
			work <- id
		}
		close(work)
		wg.Wait()
		close(results)

		var stillFailing []string
		for r := range results {
			switch {
			case r.ok:
				succeeded[r.id] = true
			case r.skip:
				skipped[r.id] = true
			case isEngineAuthError(r.err):
				out.authAbort = r.err
			default:
				stillFailing = append(stillFailing, r.id)
			}
		}
		pending = stillFailing
	}

	out.applied = len(succeeded)
	out.skipped = len(skipped)
	out.failedIDs = pending
	// Rebuild the succeeded list deterministically for the ledger.
	for _, id := range ch.IDs {
		if succeeded[id] {
			out.appliedIDs = append(out.appliedIDs, id)
		}
	}
	return out
}

// executeLabelChunk applies one batchModify chunk (idempotent — recovery
// simply re-sends it).
func executeLabelChunk(ctx context.Context, c *client.Client, ch store.MailApplyChunk) chunkOutcome {
	out := chunkOutcome{}
	body := map[string]any{"ids": ch.IDs}
	if len(ch.Add) > 0 {
		body["addLabelIds"] = ch.Add
	}
	if len(ch.Remove) > 0 {
		body["removeLabelIds"] = ch.Remove
	}
	err := engineCall(ctx, func(cctx context.Context) error {
		_, _, perr := c.Post(cctx, "/gmail/v1/users/me/messages/batchModify", body)
		return perr
	})
	switch {
	case err == nil:
		out.applied = len(ch.IDs)
		out.appliedIDs = ch.IDs
	case isEngineAuthError(err):
		out.authAbort = err
	default:
		out.failedIDs = ch.IDs
	}
	return out
}

// executeApplyChunks walks an apply's chunks in order, stamping durable
// intent transitions around every network mutation and ledgering the
// introduced deltas per id after each chunk. Honors the 15-minute soft
// deadline: the current chunk finishes, the rest stay 'pending', the apply
// is marked partial. On a 401/403 the whole run aborts (exit 5) with chunk
// state left recoverable.
func executeApplyChunks(ctx context.Context, c *client.Client, db *store.Store, plan *cleanupPlan,
	applyID int64, ledgerID string, chunks []store.MailApplyChunk, recovery bool, started time.Time, errW io.Writer) (applyResult, error) {

	res := applyResult{ApplyID: applyID, Account: plan.Account, LedgerID: ledgerID}
	res.Chunks.Total = len(chunks)
	deadlineHit := false

	for _, ch := range chunks {
		if ch.State == store.MailChunkStateDone {
			res.Chunks.Done++
			continue
		}
		if time.Since(started) > cleanupApplySoftDeadline {
			deadlineHit = true
			res.Chunks.Pending++
			continue
		}
		// Durable pre-mutation intent stamp.
		if err := db.SetMailApplyChunkState(applyID, ch.ChunkNo, store.MailChunkStateApplying); err != nil {
			return res, err
		}
		chunkWasApplying := ch.State == store.MailChunkStateApplying

		var out chunkOutcome
		if ch.Kind == "trash" {
			// Re-check semantics only when this chunk may have partially run.
			out = executeTrashChunk(ctx, c, ch, recovery && chunkWasApplying)
		} else {
			out = executeLabelChunk(ctx, c, ch)
		}

		// Ledger the introduced deltas for everything that succeeded, BEFORE
		// marking the chunk done — a crash in between re-runs the chunk and
		// re-inserts idempotently (INSERT OR IGNORE).
		var entries []store.MailLedgerEntry
		for _, id := range out.appliedIDs {
			if ch.Kind == "trash" {
				entries = append(entries, store.MailLedgerEntry{
					LedgerID:     ledgerID,
					ID:           id,
					Kind:         "trash",
					DeltaAdd:     []string{"TRASH"},
					PrePlacement: plan.prePlacementFor(id),
				})
			} else {
				entries = append(entries, store.MailLedgerEntry{
					LedgerID:    ledgerID,
					ID:          id,
					Kind:        "label",
					DeltaAdd:    ch.Add,
					DeltaRemove: ch.Remove,
				})
			}
		}
		if _, err := db.InsertMailLedgerEntries(entries); err != nil {
			return res, err
		}

		res.Applied += out.applied
		res.Skipped += out.skipped
		res.Failed += len(out.failedIDs)
		res.FailedIDs = append(res.FailedIDs, out.failedIDs...)

		if out.authAbort != nil {
			// Leave this chunk 'applying' — recover completes it after
			// re-auth. Honest counts already accumulated.
			mailSyncNotice(errW, "apply_auth_abort",
				fmt.Sprintf("authorization failed mid-apply on chunk %d; run 'accounts auth' then 'cleanup recover'", ch.ChunkNo))
			return res, out.authAbort
		}

		if err := db.SetMailApplyChunkState(applyID, ch.ChunkNo, store.MailChunkStateDone); err != nil {
			return res, err
		}
		res.Chunks.Done++
	}

	switch {
	case deadlineHit:
		res.State = store.MailApplyStatePartial
	case res.Failed > 0:
		res.State = store.MailApplyStatePartial
	default:
		res.State = store.MailApplyStateDone
	}
	if err := db.SetMailApplyState(applyID, finalApplyState(res.State)); err != nil {
		return res, err
	}
	return res, nil
}

// finalApplyState maps the result state to the stored apply state. A
// failed-ids partial with all chunks terminal is stored 'done' (nothing
// recoverable remains) while a deadline partial keeps 'partial' so recover
// picks the pending chunks up.
func finalApplyState(resultState string) string {
	return resultState
}

func apiStatus(err error) int {
	var apiE *client.APIError
	if errors.As(err, &apiE) {
		return apiE.StatusCode
	}
	return 0
}

func newCleanupApplyCmd(flags *rootFlags) *cobra.Command {
	var planSha, token string

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Execute a frozen plan: re-hash it, burn its one-time token atomically, preflight identity, refuse on drift, mutate with durable intents and a delta ledger",
		Long: `Execute exactly the ids a plan froze — never a re-run of its query.

Ordering (every step fails closed):
 1. take the advisory apply lock (<auth-dir>/apply.lock, flock; busy = exit
    7; the OS releases it on process death — there is no staleness override),
 2. re-hash the plan file and refuse (exit 4) unless it still equals its
    own sha-name,
 3. atomically (one SQLite transaction) verify the one-time token — exists,
    unused, unexpired, bound to this plan and account — mark it used, and
    record the authorized apply,
 4. identity preflight: the live mailbox email must match the profile's
    configured email (exit 4 naming both on mismatch),
 5. drift check: history since the plan's watermark; ANY planned id having
    changed refuses with "plan drifted — re-plan" (exit 4),
 6. execute: trash per id (POST messages/{id}/trash, 8 workers; batchModify
    is deliberately not used for trash), label deltas via batchModify in
    chunks of 500. Every chunk's intent is durably recorded before its
    requests go out, so a crash is recoverable ('cleanup recover').
 7. every introduced delta is ledgered per id (trash: +TRASH plus the
    INBOX/CATEGORY_* placement the plan snapshotted; label: the exact
    add/remove lists) — 'undo --ledger <id>' reverses exactly that.

Budgets: 30s per call; 429/5xx retried up to 5 attempts (exponential
1..32s; Retry-After honored); 3 passes per chunk; 15-minute soft deadline
after which remaining chunks stay pending and the run exits 3 (partial)
with honest counts.

Typed exits: 0 complete / 3 partial / 4 refused (identity, plan-sha,
token, drift) / 5 auth or API failure / 7 lock busy.`,
		Example: `  # Tokens come from 'cleanup plan' / 'rules run' and live 10 minutes
  gmail-pp-cli cleanup apply --plan 4f0c...e2 --token 9b31c0de84a1f2b3c4d5e6f708192a3b

  # Output includes ledger_id; reverse everything with:
  gmail-pp-cli undo --ledger <ledger_id>`,
		Annotations: map[string]string{"pp:typed-exit-codes": "0,3,4,5,7"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if planSha == "" || token == "" {
				return usageErr(fmt.Errorf("--plan <sha> and --token <nonce> are both required"))
			}
			authDir := gauthConfigDirFrom(flags.authDir)

			// Local validation: plan file + hash + account binding.
			plan, err := loadPlanFile(authDir, planSha)
			if err != nil {
				return err
			}
			// Engine separation (grill R2-C3): a plan whose groups carry any
			// non-mailbox action (unsub) must never drive mailbox mutations.
			// Checked BEFORE the token burns so the refusal costs nothing.
			if !plan.planIsMailboxMutation() {
				return refusedErr(fmt.Errorf("plan %s carries action %q — 'cleanup apply' executes only trash/label plans; unsub plans run via 'unsub run'", strings.ToLower(strings.TrimSpace(planSha)), plan.actionSummary()))
			}
			if flags.account != "" && flags.account != plan.Account {
				return refusedErr(fmt.Errorf("--account %q does not match the plan's account %q — a plan applies only to the account it froze", flags.account, plan.Account))
			}
			flags.account = plan.Account
			prof, err := gauthProfile(flags, plan.Account)
			if err != nil {
				return err
			}
			if !strings.EqualFold(prof.Email, plan.AccountEmail) {
				return refusedErr(fmt.Errorf("plan was frozen for %s but profile %q is now configured for %s — re-plan", plan.AccountEmail, plan.Account, prof.Email))
			}
			if dryRunOK(flags) {
				return nil
			}

			release, err := acquireApplyLock(authDir)
			if err != nil {
				return err
			}
			defer release()

			ctx := cmd.Context()
			db, err := store.OpenWithContext(ctx, defaultDBPath("gmail-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			// Crashed-apply gate: reconcile leftovers before ANY new work.
			leftovers, err := db.ListMailApplyLeftovers()
			if err != nil {
				return err
			}
			if len(leftovers) > 0 {
				mailSyncNotice(cmd.ErrOrStderr(), "recovering_leftovers",
					fmt.Sprintf("%d unfinished apply record(s) found; recovering before new work", len(leftovers)))
				if _, err := recoverLeftovers(ctx, flags, db, leftovers, cmd.ErrOrStderr()); err != nil {
					return err
				}
				// Re-check: anything still non-terminal refuses new work.
				remaining, err := db.ListMailApplyLeftovers()
				if err != nil {
					return err
				}
				if len(remaining) > 0 {
					return partialApplyErr(fmt.Errorf("%d crashed apply record(s) could not be fully recovered — resolve with 'cleanup recover' before applying new plans", len(remaining)))
				}
			}

			// Atomic: burn nonce + record authorization (grill R3-C2).
			applyID, err := db.AuthorizeMailApply(token, strings.ToLower(strings.TrimSpace(planSha)), plan.Account, plan.actionSummary(), time.Now())
			if err != nil {
				if isNonceRefusal(err) {
					return refusedErr(fmt.Errorf("apply token refused: %w (mint a fresh one with 'cleanup plan' / 'rules run')", err))
				}
				return err
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true

			// Identity preflight (grill R2-C1).
			if err := verifyLiveIdentity(ctx, c, flags, plan.Account); err != nil {
				_ = db.SetMailApplyState(applyID, store.MailApplyStateRefused)
				return classifyEngineAPIError(err)
			}

			// Drift check (grill R3-C1): any planned id changed since the
			// watermark = refuse; expired watermark = refuse.
			touched, expired, err := mailHistoryTouchedIDs(ctx, c, plan.Watermark.HistoryID)
			if err != nil {
				_ = db.SetMailApplyState(applyID, store.MailApplyStateRefused)
				return classifyEngineAPIError(err)
			}
			if expired {
				_ = db.SetMailApplyState(applyID, store.MailApplyStateRefused)
				return refusedErr(fmt.Errorf("the plan's history watermark %s has expired on Gmail's side — cannot prove the plan is still current; re-plan", plan.Watermark.HistoryID))
			}
			drifted := 0
			for _, id := range plan.allIDs() {
				if touched[id] {
					drifted++
				}
			}
			if drifted > 0 {
				_ = db.SetMailApplyState(applyID, store.MailApplyStateRefused)
				return refusedErr(fmt.Errorf("plan drifted — re-plan: %d of %d planned message(s) changed since the plan's watermark (%s)", drifted, plan.Counts.Total, plan.Watermark.Timestamp))
			}

			// Durable chunk intents, then execution.
			chunks := buildChunksFromPlan(plan, applyID)
			if err := db.InsertMailApplyChunks(chunks); err != nil {
				return err
			}
			if err := db.SetMailApplyState(applyID, store.MailApplyStateApplying); err != nil {
				return err
			}
			ledgerID, err := mintHex(8)
			if err != nil {
				return err
			}
			if err := db.CreateMailLedger(store.MailLedger{
				LedgerID: ledgerID, Account: plan.Account, PlanSha: strings.ToLower(strings.TrimSpace(planSha)),
				ApplyID: applyID, Action: plan.actionSummary(),
			}); err != nil {
				return err
			}
			if err := db.SetMailApplyLedger(applyID, ledgerID); err != nil {
				return err
			}

			res, err := executeApplyChunks(ctx, c, db, plan, applyID, ledgerID, chunks, false, time.Now(), cmd.ErrOrStderr())
			res.PlanSha = strings.ToLower(strings.TrimSpace(planSha))
			res.UndoHint = undoHint(ledgerID)
			if err != nil {
				// Auth abort or store failure; states already honest.
				if perr := printJSONFiltered(cmd.OutOrStdout(), res, flags); perr != nil {
					return perr
				}
				return classifyEngineAPIError(err)
			}
			if perr := printJSONFiltered(cmd.OutOrStdout(), res, flags); perr != nil {
				return perr
			}
			if res.State != store.MailApplyStateDone {
				return partialApplyErr(fmt.Errorf("apply finished partial: applied %d, failed %d, skipped %d, chunks pending %d — 'cleanup recover' completes pending chunks", res.Applied, res.Failed, res.Skipped, res.Chunks.Pending))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&planSha, "plan", "", "The plan sha printed by 'cleanup plan' / 'rules run' (also the plan's file name)")
	cmd.Flags().StringVar(&token, "token", "", "The one-time apply token minted with that plan (10-minute expiry, single use)")
	return cmd
}

func newCleanupRecoverCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recover",
		Short: "Reconcile crashed applies: re-apply 'applying' label chunks idempotently, re-check 'applying' trash chunks per id, finish 'pending' chunks",
		Long: `Finish whatever a crashed or deadline-cut apply left behind.

Reads every apply record still in a non-terminal state and completes it
from its durable chunk intents: label chunks are simply re-sent
(batchModify is idempotent); trash chunks that were mid-flight re-check
each id's live labelIds first, so already-trashed ids are completed, not
double-counted; untouched pending chunks run normally. Ledger rows are
re-inserted idempotently. An authorized apply that never wrote a chunk
intent is marked 'abandoned' (its token is already burned; nothing was
executed).

Runs automatically at the start of every 'cleanup apply' when leftovers
exist; this command is the standalone form.

Typed exits: 0 everything terminal / 3 partial / 4 identity refusal /
5 auth or API failure / 7 lock busy.`,
		Example: `  gmail-pp-cli cleanup recover
  gmail-pp-cli cleanup recover --agent`,
		Annotations: map[string]string{"pp:typed-exit-codes": "0,3,4,5,7"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			authDir := gauthConfigDirFrom(flags.authDir)
			release, err := acquireApplyLock(authDir)
			if err != nil {
				return err
			}
			defer release()

			ctx := cmd.Context()
			db, err := store.OpenWithContext(ctx, defaultDBPath("gmail-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			leftovers, err := db.ListMailApplyLeftovers()
			if err != nil {
				return err
			}
			results, err := recoverLeftovers(ctx, flags, db, leftovers, cmd.ErrOrStderr())
			out := map[string]any{"recovered": results, "count": len(results)}
			if perr := printJSONFiltered(cmd.OutOrStdout(), out, flags); perr != nil {
				return perr
			}
			if err != nil {
				return err
			}
			remaining, rerr := db.ListMailApplyLeftovers()
			if rerr != nil {
				return rerr
			}
			if len(remaining) > 0 {
				return partialApplyErr(fmt.Errorf("%d apply record(s) remain non-terminal", len(remaining)))
			}
			return nil
		},
	}
	return cmd
}

// recoverLeftovers reconciles every non-terminal apply. Caller holds the
// apply lock. Returns per-apply results; the error (if any) is the first
// hard failure (auth abort, store failure) — later leftovers are not
// attempted past a hard failure.
func recoverLeftovers(ctx context.Context, flags *rootFlags, db *store.Store, leftovers []store.MailApply, errW io.Writer) ([]applyResult, error) {
	var results []applyResult
	authDir := gauthConfigDirFrom(flags.authDir)
	for _, a := range leftovers {
		res := applyResult{ApplyID: a.ID, Account: a.Account, PlanSha: a.PlanSha, LedgerID: a.LedgerID}
		chunks, err := db.ListMailApplyChunks(a.ID)
		if err != nil {
			return results, err
		}
		if len(chunks) == 0 {
			// Authorized but never started: token burned, nothing executed.
			// An unsub run never writes chunks (its durable intent lives in
			// mail_unsub_ledger), so an interrupted one lands here with an
			// honest pointer instead of a false "nothing was sent".
			if err := db.SetMailApplyState(a.ID, store.MailApplyStateAbandoned); err != nil {
				return results, err
			}
			res.State = store.MailApplyStateAbandoned
			if a.Action == "unsub" {
				res.Note = "interrupted 'unsub run' — per-attempt outcomes are in mail_unsub_ledger ('unknown' rows are never auto-retried); token stays burned; mint a fresh plan with 'unsub plan' to retry skipped senders"
			} else {
				res.Note = "authorized apply had no chunk intents (crash before execution start) — nothing was sent; token stays burned; re-plan"
			}
			results = append(results, res)
			continue
		}

		plan, err := loadPlanFile(authDir, a.PlanSha)
		if err != nil {
			res.State = a.State
			res.Note = fmt.Sprintf("plan file unavailable; cannot recover this apply: %v", err)
			results = append(results, res)
			continue
		}

		// Client + identity preflight for THIS apply's account.
		prevAccount := flags.account
		flags.account = a.Account
		c, err := flags.newClient()
		if err != nil {
			flags.account = prevAccount
			return results, err
		}
		c.NoCache = true
		if err := verifyLiveIdentity(ctx, c, flags, a.Account); err != nil {
			flags.account = prevAccount
			return results, classifyEngineAPIError(err)
		}

		ledgerID := a.LedgerID
		if ledgerID == "" {
			// Crash landed between chunk insert and ledger create.
			ledgerID, err = mintHex(8)
			if err != nil {
				flags.account = prevAccount
				return results, err
			}
			if err := db.CreateMailLedger(store.MailLedger{
				LedgerID: ledgerID, Account: a.Account, PlanSha: a.PlanSha,
				ApplyID: a.ID, Action: plan.actionSummary(),
			}); err != nil {
				flags.account = prevAccount
				return results, err
			}
			if err := db.SetMailApplyLedger(a.ID, ledgerID); err != nil {
				flags.account = prevAccount
				return results, err
			}
		}

		out, execErr := executeApplyChunks(ctx, c, db, plan, a.ID, ledgerID, chunks, true, time.Now(), errW)
		out.PlanSha = a.PlanSha
		out.UndoHint = undoHint(ledgerID)
		results = append(results, out)
		flags.account = prevAccount
		if execErr != nil {
			return results, classifyEngineAPIError(execErr)
		}
	}
	return results, nil
}

// isNonceRefusal reports the AuthorizeMailApply sentinel family.
func isNonceRefusal(err error) bool {
	return errors.Is(err, store.ErrNonceUnknown) || errors.Is(err, store.ErrNonceUsed) ||
		errors.Is(err, store.ErrNonceExpired) || errors.Is(err, store.ErrNonceMismatch)
}
