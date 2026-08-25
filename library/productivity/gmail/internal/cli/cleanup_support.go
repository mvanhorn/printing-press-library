// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written shared machinery for the cleanup engine: typed exits,
// the live-identity preflight (grill R2-C1), the apply flock (R3-C4),
// the per-call retry/timeout budget (R2-C6), immutable plan files with
// sha256 identity (R1-C4/C5), one-time nonces, and the watermark/drift
// helpers (R3-C1). Command RunEs live in cleanup_plan.go /
// cleanup_apply.go / undo.go; this file has no cobra wiring.

package cli

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/gauth"
	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/store"
)

// Engine budgets and sizes. See each command's Long text for the
// user-facing documentation of these numbers.
const (
	cleanupPlanSchemaVersion = 1
	cleanupPlanDefaultLimit  = 2000
	cleanupNonceTTL          = 10 * time.Minute
	cleanupTrashChunkSize    = 100 // per-id POSTs, durably chunked for recovery
	cleanupLabelChunkSize    = 500 // ids per batchModify call
	cleanupWorkers           = 8
	cleanupCallTimeout       = 30 * time.Second
	cleanupMaxAttempts       = 5 // per call, incl. the first
	cleanupBackoffCap        = 32 * time.Second
	cleanupChunkRetryCeiling = 3 // execution passes per chunk
	cleanupApplySoftDeadline = 15 * time.Minute
)

// Typed exits for the mutation engine:
//
//	0 complete / 3 partial / 4 refused (identity, plan-sha, nonce, drift,
//	conflict guards) / 5 auth or API failure / 7 lock busy.
//
// The numeric values intentionally reuse the framework's cliError codes
// (3, 4, 5, 7) so ExitCode() needs no changes; these constructors exist so
// engine call sites read as their engine meaning, not the generic one.
func refusedErr(err error) error      { return &cliError{code: 4, err: err} }
func partialApplyErr(err error) error { return &cliError{code: 3, err: err} }
func lockBusyErr(err error) error     { return &cliError{code: 7, err: err} }

// classifyEngineAPIError maps any transport/API failure inside the engine
// to the engine's auth/API exit (5). Typed cliErrors (refusals, partials,
// usage) pass through untouched.
func classifyEngineAPIError(err error) error {
	if err == nil {
		return nil
	}
	var typed *cliError
	if errors.As(err, &typed) {
		return err
	}
	var apiE *client.APIError
	if errors.As(err, &apiE) && (apiE.StatusCode == 401 || apiE.StatusCode == 403) {
		return &cliError{code: 5, err: fmt.Errorf("%w\nhint: run 'gmail-pp-cli accounts auth --account <name>' to re-authenticate", err)}
	}
	return &cliError{code: 5, err: err}
}

// verifyLiveIdentity is the identity preflight (grill R2-C1): fetch the
// live mailbox's users.me profile and compare its emailAddress
// (case-insensitively) to the email configured for the resolved profile in
// profiles.yaml. Any mismatch is a typed refusal (exit 4) naming both
// addresses. Called at the top of every mutating command and of sync.
// The profile GET bypasses the response cache so a stale cached identity
// can never satisfy the check.
func verifyLiveIdentity(ctx context.Context, c *client.Client, flags *rootFlags, account string) error {
	prof, err := gauth.Get(gauth.ConfigDir(flags.authDir), account)
	if err != nil {
		return err
	}
	data, err := c.GetWithHeadersNoCache(ctx, "/gmail/v1/users/me/profile", nil, nil)
	if err != nil {
		return err
	}
	var p struct {
		EmailAddress string `json:"emailAddress"`
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("identity preflight: parsing live profile: %w", err)
	}
	if strings.TrimSpace(p.EmailAddress) == "" {
		return refusedErr(fmt.Errorf("identity preflight: live profile returned no emailAddress — refusing to proceed"))
	}
	if !strings.EqualFold(p.EmailAddress, prof.Email) {
		return refusedErr(fmt.Errorf(
			"identity preflight: the live mailbox is %q but profile %q is configured for %q — refusing to touch this mailbox (fix profiles.yaml or re-run: accounts auth --account %s)",
			p.EmailAddress, account, prof.Email, account))
	}
	return nil
}

// acquireApplyLock takes the advisory apply lock at <auth-dir>/apply.lock
// via flock(LOCK_EX|LOCK_NB). Busy is a typed exit 7. No timestamp or
// staleness break-in logic on purpose (grill R3-C4): the OS releases the
// lock automatically when the holding process dies, so "stale lock"
// cannot exist and breaking a live one must be impossible.
func acquireApplyLock(authDir string) (func(), error) {
	if err := os.MkdirAll(authDir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(authDir, "apply.lock")
	f, err := os.OpenFile(filepath.Clean(path), os.O_CREATE|os.O_RDWR, 0o600) // #nosec G304 -- app-derived auth-dir lock path.
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, lockBusyErr(fmt.Errorf(
				"another apply/recover/undo currently holds %s — wait for it to finish (the OS releases the lock the moment that process exits, even on a crash)", path))
		}
		return nil, fmt.Errorf("acquiring apply lock %s: %w", path, err)
	}
	release := func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}
	return release, nil
}

// engineCall runs one mutating/read call under the engine's budget
// (grill R2-C6): 30s per attempt, up to 5 attempts total, exponential
// 1..32s between retries, retrying only transient failures (429, 5xx,
// network). The transport client additionally honors Retry-After on 429
// inside each attempt. Non-retryable errors (4xx, refusals) return
// immediately.
func engineCall(ctx context.Context, fn func(ctx context.Context) error) error {
	var lastErr error
	for attempt := 0; attempt < cleanupMaxAttempts; attempt++ {
		if attempt > 0 {
			wait := time.Duration(1<<uint(attempt-1)) * time.Second
			if wait > cleanupBackoffCap {
				wait = cleanupBackoffCap
			}
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		callCtx, cancel := context.WithTimeout(ctx, cleanupCallTimeout)
		err := fn(callCtx)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		if !engineRetryable(err) {
			return err
		}
	}
	return lastErr
}

// engineRetryable reports whether an error is worth another engine
// attempt: 429 and 5xx API responses and transport-level failures are;
// every other API status (and every non-network typed error) is not.
func engineRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	var apiE *client.APIError
	if errors.As(err, &apiE) {
		return apiE.StatusCode == 429 || apiE.StatusCode >= 500
	}
	var typed *cliError
	if errors.As(err, &typed) {
		return false
	}
	var refusal *client.TransportRefusedError
	if errors.As(err, &refusal) {
		return false
	}
	// Non-API, non-typed: network-level (timeout, reset, DNS). Retry.
	return true
}

// isEngineAuthError reports a 401/403 API response — the engine aborts the
// whole run on these (exit 5), leaving chunk states recoverable.
func isEngineAuthError(err error) bool {
	var apiE *client.APIError
	return errors.As(err, &apiE) && (apiE.StatusCode == 401 || apiE.StatusCode == 403)
}

// ---------------------------------------------------------------------------
// Plan files
// ---------------------------------------------------------------------------

// cleanupPlanAction is one frozen action: trash, or a label delta.
type cleanupPlanAction struct {
	Type   string   `json:"type"` // trash | label
	Add    []string `json:"add,omitempty"`
	Remove []string `json:"remove,omitempty"`
}

// cleanupPlanGroup is one rule's (or the direct plan's) frozen id set.
// Sender and UnsubURL are populated only by `unsub plan` (action type
// "unsub"): the frozen sender identity and the exact https one-click URL
// `unsub run` is authorized to POST to. omitempty keeps trash/label plan
// bytes (and therefore their shas) identical to before the unsub engine
// existed.
type cleanupPlanGroup struct {
	Rule     string            `json:"rule"` // "" for a direct `cleanup plan`
	Query    string            `json:"query,omitempty"`
	Action   cleanupPlanAction `json:"action"`
	IDs      []string          `json:"ids"`
	Count    int               `json:"count"`
	Sender   string            `json:"sender,omitempty"`
	UnsubURL string            `json:"unsub_url,omitempty"`
}

// cleanupPlanSnapshot is the per-id labelIds snapshot the store showed at
// plan time — the source of undo's pre_placement (grill R3-C3).
type cleanupPlanSnapshot struct {
	ID       string   `json:"id"`
	ThreadID string   `json:"thread_id,omitempty"`
	LabelIDs []string `json:"label_ids"`
}

// cleanupPlanDelta is one id's intended delta.
type cleanupPlanDelta struct {
	ID     string   `json:"id"`
	Add    []string `json:"add,omitempty"`
	Remove []string `json:"remove,omitempty"`
}

// cleanupPlanSample is one digest row (first 10 frozen ids).
type cleanupPlanSample struct {
	ID      string `json:"id"`
	From    string `json:"from"`
	Subject string `json:"subject"`
	Date    string `json:"date"`
}

// cleanupPlanWatermark records where the store stood when the plan froze.
type cleanupPlanWatermark struct {
	HistoryID string `json:"history_id"`
	Timestamp string `json:"timestamp"`
}

// cleanupPlanCounts summarizes the frozen plan.
type cleanupPlanCounts struct {
	Total  int `json:"total"`
	Groups int `json:"groups"`
}

// cleanupPlan is the immutable plan-file content. Its canonical JSON
// marshaling IS the identity: the file name is the sha256 of the exact
// bytes written, and apply re-hashes the bytes before trusting anything
// inside (grill R1-C4, R2-C4). All slices are deterministically ordered
// before marshaling.
type cleanupPlan struct {
	SchemaVersion int                   `json:"schema_version"`
	CreatedAt     string                `json:"created_at"`
	Source        string                `json:"source"` // "cleanup plan" | "rules run"
	Account       string                `json:"account"`
	AccountEmail  string                `json:"account_email"`
	Groups        []cleanupPlanGroup    `json:"groups"`
	Snapshots     []cleanupPlanSnapshot `json:"snapshots"`
	Deltas        []cleanupPlanDelta    `json:"deltas"`
	Samples       []cleanupPlanSample   `json:"samples"`
	Counts        cleanupPlanCounts     `json:"counts"`
	Watermark     cleanupPlanWatermark  `json:"watermark"`
}

// allIDs returns every frozen id across groups (already deduplicated at
// build time), in group order.
func (p *cleanupPlan) allIDs() []string {
	var ids []string
	for _, g := range p.Groups {
		ids = append(ids, g.IDs...)
	}
	return ids
}

// snapshotLabels returns the plan-time labelIds for one id (” set when
// the id has no snapshot — cannot happen for plans this binary wrote).
func (p *cleanupPlan) snapshotLabels(id string) []string {
	for i := range p.Snapshots {
		if p.Snapshots[i].ID == id {
			return p.Snapshots[i].LabelIDs
		}
	}
	return nil
}

// prePlacementFor derives undo's pre_placement for one id: the INBOX and
// CATEGORY_* labels the store showed at plan time (grill R3-C3).
func (p *cleanupPlan) prePlacementFor(id string) []string {
	var out []string
	for _, l := range p.snapshotLabels(id) {
		if l == "INBOX" || strings.HasPrefix(l, "CATEGORY_") {
			out = append(out, l)
		}
	}
	return out
}

// actionSummary renders a short human label for the plan's action mix.
func (p *cleanupPlan) actionSummary() string {
	kinds := map[string]bool{}
	for _, g := range p.Groups {
		kinds[g.Action.Type] = true
	}
	var parts []string
	for _, k := range []string{"trash", "label", "unsub"} {
		if kinds[k] {
			parts = append(parts, k)
		}
	}
	return strings.Join(parts, "+")
}

// planIsMailboxMutation reports whether every group carries a mailbox
// action (trash/label) — the only plans `cleanup apply` may execute. An
// unsub plan authorizes external HTTPS POSTs, never mailbox mutations, and
// the two engines refuse each other's plans (grill R2-C3).
func (p *cleanupPlan) planIsMailboxMutation() bool {
	for _, g := range p.Groups {
		if g.Action.Type != "trash" && g.Action.Type != "label" {
			return false
		}
	}
	return true
}

// planIsUnsub reports whether every group carries the unsub action.
func (p *cleanupPlan) planIsUnsub() bool {
	for _, g := range p.Groups {
		if g.Action.Type != "unsub" {
			return false
		}
	}
	return len(p.Groups) > 0
}

// cleanupPlansDir returns (creating if needed) <auth-dir>/plans.
func cleanupPlansDir(authDir string) (string, error) {
	dir := filepath.Join(authDir, "plans")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating plans dir: %w", err)
	}
	return dir, nil
}

// writePlanFile marshals the plan canonically, names the file by the
// sha256 of those exact bytes, and writes it 0600. Returns (sha, path).
func writePlanFile(authDir string, plan *cleanupPlan) (string, string, error) {
	dir, err := cleanupPlansDir(authDir)
	if err != nil {
		return "", "", err
	}
	b, err := json.Marshal(plan)
	if err != nil {
		return "", "", fmt.Errorf("marshaling plan: %w", err)
	}
	sum := sha256.Sum256(b)
	sha := hex.EncodeToString(sum[:])
	path := filepath.Join(dir, sha+".json")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return "", "", fmt.Errorf("writing plan file: %w", err)
	}
	return sha, path, nil
}

var planShaPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// loadPlanFile reads <auth-dir>/plans/<sha>.json, re-hashes the raw bytes,
// and refuses (exit 4) on any mismatch — an edited plan can never apply
// (grill R2-C4). Returns the parsed plan.
func loadPlanFile(authDir, shaArg string) (*cleanupPlan, error) {
	sha := strings.ToLower(strings.TrimSpace(shaArg))
	if !planShaPattern.MatchString(sha) {
		return nil, refusedErr(fmt.Errorf("--plan %q is not a plan sha (expected the 64-hex sha256 printed by 'cleanup plan' / 'rules run')", shaArg))
	}
	path := filepath.Join(authDir, "plans", sha+".json")
	b, err := os.ReadFile(filepath.Clean(path)) // #nosec G304 -- sha-named file under the app auth dir.
	if err != nil {
		return nil, refusedErr(fmt.Errorf("plan %s not found under %s — re-run 'cleanup plan' (plans are immutable and never regenerated in place): %v", sha, filepath.Join(authDir, "plans"), err))
	}
	sum := sha256.Sum256(b)
	if hex.EncodeToString(sum[:]) != sha {
		return nil, refusedErr(fmt.Errorf("plan file %s does not hash to its own name — the file was edited after it was written; refusing to apply. Re-plan.", path))
	}
	var p cleanupPlan
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, refusedErr(fmt.Errorf("plan file %s is not parseable: %v", path, err))
	}
	if p.SchemaVersion != cleanupPlanSchemaVersion {
		return nil, refusedErr(fmt.Errorf("plan file %s has schema_version %d; this binary writes and applies version %d", path, p.SchemaVersion, cleanupPlanSchemaVersion))
	}
	if len(p.Groups) == 0 || p.Account == "" {
		return nil, refusedErr(fmt.Errorf("plan file %s carries no groups or no account — refusing", path))
	}
	return &p, nil
}

// mintHex returns n cryptographically random bytes as lowercase hex.
func mintHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("minting random token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// ---------------------------------------------------------------------------
// Watermark + drift
// ---------------------------------------------------------------------------

// runPlanSync runs the same sync machinery the `sync` command uses
// (incremental from the stored cursor when one exists, else full) and
// returns the post-sync watermark the plan freezes against. The cursor is
// persisted exactly as `sync` would persist it.
func runPlanSync(ctx context.Context, c *client.Client, db *store.Store, account string, errW io.Writer) (cleanupPlanWatermark, error) {
	st, err := db.GetMailSyncState(account)
	if err != nil {
		return cleanupPlanWatermark{}, fmt.Errorf("reading sync state: %w", err)
	}
	limit := mailSyncDefaultLimit
	if cliutil.IsDogfoodEnv() {
		limit = 50
	}
	mode := "full"
	if st.HistoryID != "" {
		mode = "incremental"
	}
	var res mailSyncResult
	if mode == "incremental" {
		incRes, expired, incErr := mailIncrementalSync(ctx, c, db, account, st.HistoryID, limit, errW)
		switch {
		case expired:
			mailSyncNotice(errW, "history_expired",
				fmt.Sprintf("history cursor %s expired on Gmail's side; falling back to a full sync", st.HistoryID))
			mode = "full"
		case incErr != nil:
			return cleanupPlanWatermark{}, incErr
		default:
			res = incRes
		}
	}
	if mode == "full" {
		fullRes, fullErr := mailFullSync(ctx, c, db, account, "", limit, errW)
		if fullErr != nil {
			return cleanupPlanWatermark{}, fullErr
		}
		res = fullRes
	}
	if res.HistoryID == "" {
		res.HistoryID = st.HistoryID
	}
	if res.HistoryID == "" {
		return cleanupPlanWatermark{}, fmt.Errorf("sync produced no history cursor; cannot watermark a plan")
	}
	if err := db.SaveMailSyncState(account, res.HistoryID, mode); err != nil {
		return cleanupPlanWatermark{}, fmt.Errorf("saving sync state: %w", err)
	}
	return cleanupPlanWatermark{
		HistoryID: res.HistoryID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// mailHistoryTouchedIDs walks users.history.list from startHistoryID and
// collects every message id any record touched (added, deleted, labels
// changed). expired=true means Gmail no longer holds that cursor (HTTP
// 404) — the caller must refuse, not guess (grill R3-C1). Read-only: never
// moves the stored sync cursor.
func mailHistoryTouchedIDs(ctx context.Context, c *client.Client, startHistoryID string) (map[string]bool, bool, error) {
	touched := map[string]bool{}
	pageToken := ""
	for {
		params := url.Values{}
		params.Set("startHistoryId", startHistoryID)
		params.Set("maxResults", "500")
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}
		data, err := c.GetWithHeadersNoCacheValues(ctx, "/gmail/v1/users/me/history", params, nil)
		if err != nil {
			var apiErr *client.APIError
			if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
				return nil, true, nil
			}
			return nil, false, fmt.Errorf("listing history for drift check: %w", err)
		}
		var page struct {
			History       []gmailHistoryRecord `json:"history"`
			NextPageToken string               `json:"nextPageToken"`
		}
		if err := json.Unmarshal(data, &page); err != nil {
			return nil, false, fmt.Errorf("parsing history page: %w", err)
		}
		for _, rec := range page.History {
			for _, ma := range rec.MessagesAdded {
				if ma.Message.ID != "" {
					touched[ma.Message.ID] = true
				}
			}
			for _, md := range rec.MessagesDeleted {
				if md.Message.ID != "" {
					touched[md.Message.ID] = true
				}
			}
			for _, la := range rec.LabelsAdded {
				if la.Message.ID != "" {
					touched[la.Message.ID] = true
				}
			}
			for _, lr := range rec.LabelsRemoved {
				if lr.Message.ID != "" {
					touched[lr.Message.ID] = true
				}
			}
		}
		if page.NextPageToken == "" {
			break
		}
		pageToken = page.NextPageToken
	}
	return touched, false, nil
}

// fetchMessageLabelIDs fetches one message's current labelIds live
// (format=minimal, cache bypassed). found=false means Gmail returned 404.
func fetchMessageLabelIDs(ctx context.Context, c *client.Client, id string) ([]string, bool, error) {
	params := url.Values{}
	params.Set("format", "minimal")
	data, err := c.GetWithHeadersNoCacheValues(ctx, mailMetadataFetchPath+url.PathEscape(id), params, nil)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return nil, false, nil
		}
		return nil, false, err
	}
	var msg struct {
		LabelIDs []string `json:"labelIds"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, false, fmt.Errorf("parsing message %s: %w", id, err)
	}
	return msg.LabelIDs, true, nil
}

// sortedCopy returns a sorted copy of xs (never mutates the input).
func sortedCopy(xs []string) []string {
	out := append([]string(nil), xs...)
	sort.Strings(out)
	return out
}

// undoHint renders the copy-pasteable undo command for a ledger.
func undoHint(ledgerID string) string {
	return fmt.Sprintf("gmail-pp-cli undo --ledger %s", ledgerID)
}

// gauthConfigDirFrom resolves the auth dir exactly like every gauth caller.
func gauthConfigDirFrom(override string) string {
	return gauth.ConfigDir(override)
}

// gauthProfile loads one profile from the resolved auth dir.
func gauthProfile(flags *rootFlags, account string) (gauth.Profile, error) {
	return gauth.Get(gauth.ConfigDir(flags.authDir), account)
}
