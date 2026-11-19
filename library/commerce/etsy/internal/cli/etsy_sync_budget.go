package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
	"github.com/spf13/cobra"
)

// etsy-ext: Reserve calls under a lock; in-flight requests may finish after a stop.
var errEtsySyncMaxCalls = errors.New("Etsy sync maximum calls reached")

type etsySyncClient interface {
	Get(context.Context, string, map[string]string) (json.RawMessage, error)
	RateLimit() float64
}
type etsySyncGuard struct {
	inner    etsySyncClient
	maxCalls int
	mu       sync.Mutex
	calls    int64
	stop     error
}

func (g *etsySyncGuard) Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error) {
	g.mu.Lock()
	if g.stop == nil && g.maxCalls > 0 && g.calls >= int64(g.maxCalls) {
		g.stop = errEtsySyncMaxCalls
	}
	if g.stop != nil {
		err := g.stop
		g.mu.Unlock()
		return nil, err
	}
	g.calls++
	g.mu.Unlock()
	data, err := g.inner.Get(ctx, path, params)
	if isDailyQuotaErr(err) {
		g.mu.Lock()
		if g.stop == nil {
			g.stop = err
		}
		g.mu.Unlock()
	}
	return data, err
}
func (g *etsySyncGuard) RateLimit() float64 { return g.inner.RateLimit() }
func (g *etsySyncGuard) CurrentTemplateVars() map[string]string {
	return syncClientTemplateVars(g.inner)
}
func (g *etsySyncGuard) IsDryRun() bool {
	c, ok := g.inner.(interface{ IsDryRun() bool })
	return ok && c.IsDryRun()
}
func (g *etsySyncGuard) Calls() int64       { g.mu.Lock(); defer g.mu.Unlock(); return g.calls }
func (g *etsySyncGuard) SyncStopped() error { g.mu.Lock(); defer g.mu.Unlock(); return g.stop }
func syncStopError(c any) error {
	if g, ok := c.(interface{ SyncStopped() error }); ok {
		return g.SyncStopped()
	}
	return nil
}
func isSyncStopError(err error) bool {
	return isDailyQuotaErr(err) || errors.Is(err, errEtsySyncMaxCalls)
}

// etsy-ext: When checked is true, payment-walk estimates use the check ledger;
// the zero value conservatively counts every parent.
type etsySyncPlan struct{ checked bool }

const etsyReceiptStampKey = "etsy_receipt_updated"

// etsy-ext: F1 walks exclude forbidden rows except one oldest probe appended last,
// with input-order ties; incremental selection preserves normal receipt order,
// while full selection prioritizes unchecked receipts then oldest checks stably.
func etsyPaymentParentsToWalk(db *store.Store, receiptIDs []string, full bool) (walk []string, stamps map[string]int64, err error) {
	updated := map[string]int64{}
	rows, err := db.Query(`SELECT id, CAST(json_extract(data,'$.receipt_id') AS TEXT), CAST(json_extract(data,'$.updated_timestamp') AS TEXT) FROM resources WHERE resource_type = 'receipts'`)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var id string
		var receipt, stamp sql.NullString
		if err := rows.Scan(&id, &receipt, &stamp); err != nil {
			_ = rows.Close()
			return nil, nil, err
		}
		if n, err := strconv.ParseInt(stamp.String, 10, 64); stamp.Valid && err == nil {
			updated[store.BareResourceID(id)] = n
			if receipt.Valid {
				updated[receipt.String] = n
			}
		}
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, nil, err
	}
	checks, err := db.ReadPaymentChecks()
	if err != nil {
		return nil, nil, err
	}
	etsyRest, etsyProbe, etsyHasProbe := etsySplitForbiddenProbe(receiptIDs, checks)
	if full {
		walk = append([]string(nil), etsyRest...)
		sort.SliceStable(walk, func(i, j int) bool {
			etsyLeft, etsyLeftChecked := checks[walk[i]]
			etsyRight, etsyRightChecked := checks[walk[j]]
			if etsyLeftChecked != etsyRightChecked {
				return !etsyLeftChecked
			}
			return etsyLeftChecked && etsyLeft.CheckedAt < etsyRight.CheckedAt
		})
		if etsyHasProbe {
			walk = append(walk, etsyProbe)
		}
		return walk, updated, nil
	}
	walk = make([]string, 0, len(receiptIDs))
	for _, id := range etsyRest {
		stamp, known := updated[id]
		check, checked := checks[id]
		if checked && check.Denied {
			if known && (!check.ReceiptUpdated.Valid || stamp > check.ReceiptUpdated.Int64) {
				walk = append(walk, id)
			}
			continue
		}
		if !checked || !known || !check.ReceiptUpdated.Valid || stamp > check.ReceiptUpdated.Int64 {
			walk = append(walk, id)
		}
	}
	if etsyHasProbe {
		walk = append(walk, etsyProbe)
	}
	return walk, updated, nil
}

// etsy-ext: Only input receipts compete for the single forbidden probe; ties keep input order.
func etsySplitForbiddenProbe(etsyReceiptIDs []string, etsyChecks map[string]store.PaymentCheck) (etsyRest []string, etsyProbe string, etsyOK bool) {
	for _, etsyID := range etsyReceiptIDs {
		etsyCheck := etsyChecks[etsyID]
		if !etsyCheck.Forbidden {
			etsyRest = append(etsyRest, etsyID)
			continue
		}
		if !etsyOK || etsyCheck.CheckedAt < etsyChecks[etsyProbe].CheckedAt {
			etsyProbe, etsyOK = etsyID, true
		}
	}
	return
}

// etsy-ext: Carry the payment allowance and durable progress through dependent workers.
type etsyPaymentsCap struct {
	allow        int
	total        int
	capped       bool
	checked      atomic.Int64
	denied       atomic.Int64
	forbidden    atomic.Int64
	scopeLost    atomic.Bool
	ok           atomic.Int64
	walked       sync.Map
	forbiddenIDs sync.Map
}
type etsyPaymentsCapKey struct{}

func etsyWithPaymentsCap(ctx context.Context, cs *etsyPaymentsCap) context.Context {
	return context.WithValue(ctx, etsyPaymentsCapKey{}, cs)
}
func etsyPaymentsCapFrom(ctx context.Context) *etsyPaymentsCap {
	cs, _ := ctx.Value(etsyPaymentsCapKey{}).(*etsyPaymentsCap)
	return cs
}

// etsy-ext: Share the payment reserve floor between allowance and boundary refusal.
func etsyPaymentsReserve(q client.EtsyQuota) int {
	reserve := q.RemainingToday / 20
	if q.RemainingToday%20 != 0 {
		reserve++
	}
	if q.LimitPerDay > 0 {
		reserve = max(reserve, (q.LimitPerDay-1)/20+1)
	}
	return reserve
}

// etsy-ext: Reserve five percent of remaining quota or the smallest accepted daily balance, whichever is greater.
func etsyPaymentsAllowance(maxCalls int, used int64, others int, ignoreBudget bool, key string) (allow int, bounded bool) {
	if maxCalls > 0 {
		allow, bounded = maxCalls-int(used)-others, true
	}
	if q, known := client.LoadEtsyQuota(key); known && !ignoreBudget {
		reserve := etsyPaymentsReserve(q)
		quotaAllow := q.RemainingToday - reserve - others
		if !bounded || quotaAllow < allow {
			allow = quotaAllow
		}
		bounded = true
	}
	return
}

// etsy-ext: Estimate head walks using stored populations, including selected fan-out.
func estimateSyncCalls(db *store.Store, flat []string, parentFilter []string, maxPages int, latestOnly bool) int {
	return estimateSyncCallsPlan(db, flat, parentFilter, maxPages, latestOnly, etsySyncPlan{})
}

// etsy-ext: Ledger-aware estimates share the receipt selection used by execution.
func estimateSyncCallsPlan(db *store.Store, flat, parentFilter []string, maxPages int, latestOnly bool, plan etsySyncPlan) int {
	// etsy-ext: Preserve the total estimate while exposing independently budgeted phases.
	flatCalls, others, payments := estimateSyncCallsSplit(db, flat, parentFilter, maxPages, latestOnly, plan)
	return flatCalls + others + payments
}

// etsy-ext: Split flat calls, other fan-out, and receipt payment checks.
func estimateSyncCallsSplit(db *store.Store, flat, parentFilter []string, maxPages int, latestOnly bool, plan etsySyncPlan) (flatCalls, otherDeps, payments int) {
	for _, resource := range flat {
		n, _ := db.Count(resource)
		limit := determinePaginationDefaults(resource).limit
		if limit < 1 {
			limit = 1
		}
		pages := n/limit + 1
		if latestOnly {
			pages = 1
		}
		if maxPages > 0 && pages > maxPages {
			pages = maxPages
		}
		flatCalls += pages
		if resource == "listings" {
			flatCalls += 4
		}
	}
	allow := map[string]bool{}
	for _, r := range parentFilter {
		allow[r] = true
	}
	for _, dep := range dependentResourceDefs() {
		if etsyDefaultSkippedDependents[dep.Name] && !allow[dep.Name] {
			continue
		}
		if len(allow) > 0 && !allow[dep.ParentTable] && !allow[dep.Name] {
			continue
		}
		n, _ := db.Count(dep.ParentTable)
		// etsy-ext: On lookup failure retain the conservative full-parent estimate.
		if dep.Name == "shops_receipts_payments" && plan.checked {
			ids, idsErr := db.ListField(dep.ParentTable, "receipt_id")
			if idsErr == nil {
				if walk, _, err := etsyPaymentParentsToWalk(db, ids, false); err == nil {
					n = len(walk)
				}
			}
		}
		if dep.Name == "shops_receipts_payments" {
			payments += n
		} else {
			otherDeps += n
		}
	}
	return
}

// etsy-ext: Bound large plans until an Etsy response establishes the daily quota.
const etsyUnknownQuotaMaxPlan = 1000

// etsy-ext: MCP caps and overrides are fixed by the host.
func etsyBudgetHint(base string) string {
	if cliutil.IsMCPChildEnv() {
		return "narrow --resources"
	}
	return base
}

// etsy-ext: Keep the unknown-quota decision independent of stored quota state.
func etsyUnknownQuotaRefusal(planned, maxCalls int, ignoreBudget, quotaKnown bool) error {
	if !ignoreBudget && maxCalls == 0 && planned > etsyUnknownQuotaMaxPlan && !quotaKnown {
		return rateLimitErr(fmt.Errorf("planned ~%d Etsy calls and today's quota is unknown (no Etsy response observed yet); %s", planned, etsyBudgetHint("pass --max-calls N to cap the run or --ignore-budget to override")))
	}
	return nil
}

// etsy-ext: Both planning gates apply the same quota rules after their call-cap check.
func etsySyncQuotaCheck(cmd *cobra.Command, unknownPlanned, budgetPlanned, maxCalls int, ignoreBudget bool) error {
	_, known := client.LoadEtsyQuota(etsyQuotaKey(cmd))
	if err := etsyUnknownQuotaRefusal(unknownPlanned, maxCalls, ignoreBudget, known); err != nil {
		return err
	}
	return checkBudget(cmd, budgetPlanned, ignoreBudget)
}

func etsySyncPreflight(cmd *cobra.Command, db *store.Store, flat, parentFilter []string, maxPages int, latestOnly bool, maxCalls int, ignoreBudget bool, plan etsySyncPlan) (int, error) {
	// etsy-ext: Preflight uses the same incremental ledger as execution.
	flatCalls, others, payments := estimateSyncCallsSplit(db, flat, parentFilter, maxPages, latestOnly, plan)
	planned := flatCalls + others + payments
	// etsy-ext: Positive payment allowances cap execution instead of refusing the full fan-out.
	refusalCount := flatCalls + others
	if payments > 0 {
		allow, bounded := etsyPaymentsAllowance(maxCalls, 0, refusalCount, ignoreBudget, etsyQuotaKey(cmd))
		if bounded && allow <= 0 {
			refusalCount += payments
		}
	}
	if maxCalls > 0 && refusalCount > maxCalls {
		return planned, rateLimitErr(fmt.Errorf("planned ~%d Etsy calls exceeds --max-calls %d; %s", refusalCount, maxCalls, etsyBudgetHint("narrow --resources or increase the cap")))
	}
	if err := etsySyncQuotaCheck(cmd, flatCalls, refusalCount, maxCalls, ignoreBudget); err != nil {
		return planned, err
	}
	if humanFriendly {
		fmt.Fprintf(cmd.ErrOrStderr(), "planned ~%d Etsy calls\n", planned)
	}
	return planned, nil
}

// etsy-ext: etsySyncBoundaryCheck re-runs the budget gate before dependents using current parent counts.
// used is the number of calls the run has already made through the guard.
func etsySyncBoundaryCheck(cmd *cobra.Command, db *store.Store, parentFilter []string, maxCalls int, used int64, ignoreBudget bool, plan etsySyncPlan) (planned int, paymentsCap int, err error) {
	// etsy-ext: Other dependents retain refusal semantics; payments consume only their allowance.
	_, others, pay := estimateSyncCallsSplit(db, nil, parentFilter, 0, false, plan)
	planned, paymentsCap = others+pay, -1
	if planned == 0 {
		return
	}
	allow, bounded := etsyPaymentsAllowance(maxCalls, used, others, ignoreBudget, etsyQuotaKey(cmd))
	payCount, budgetCount := pay, others+pay
	if pay > 0 && bounded && allow > 0 {
		paymentsCap = allow
		payCount = min(pay, allow)
		// etsy-ext: A capped payment walk still calls Etsy and must run the budget gate.
		budgetCount = max(others, 1)
	}
	refusalCount := others + payCount
	if maxCalls > 0 && int64(refusalCount) > int64(maxCalls)-used {
		return planned, paymentsCap, rateLimitErr(fmt.Errorf("planned ~%d more Etsy calls exceeds --max-calls %d remaining allowance %d; %s", refusalCount, maxCalls, int64(maxCalls)-used, etsyBudgetHint("narrow --resources or increase the cap")))
	}
	// etsy-ext: Refuse an exhausted payment reserve without completing an empty walk.
	if pay > 0 && bounded && allow <= 0 && !ignoreBudget {
		if q, known := client.LoadEtsyQuota(etsyQuotaKey(cmd)); known {
			return planned, paymentsCap, rateLimitErr(fmt.Errorf("payments: daily reserve reached (%d left, floor %d); %s", q.RemainingToday, etsyPaymentsReserve(q), etsyBudgetHint("re-run when quota is available in the rolling 24h window, pass --ignore-budget, or narrow --resources to skip shops_receipts_payments")))
		}
	}
	if err = etsySyncQuotaCheck(cmd, planned, budgetCount, maxCalls, ignoreBudget); err != nil {
		return
	}
	if humanFriendly {
		fmt.Fprintf(cmd.ErrOrStderr(), "planned ~%d more Etsy calls for dependents\n", refusalCount)
	}
	return
}

// etsy-ext: Explain full-walk rotation while preserving the resume guidance.
func etsyPaymentsCappedMessage(full bool, checked, total int64) string {
	if full {
		return fmt.Sprintf("payments: %d of %d receipts checked this run (--full checks never-checked receipts first, then the oldest-checked); re-run to continue (progress is saved)", checked, total)
	}
	return fmt.Sprintf("payments: %d of %d receipts checked; re-run to continue (progress is saved)", checked, total)
}

func etsyPaymentsDeniedWarning(err error) (*accessWarning, bool) {
	var etsyAPIError *client.APIError
	if errors.As(err, &etsyAPIError) {
		etsyReason := ""
		switch etsyAPIError.StatusCode {
		case 403:
			etsyReason = "forbidden"
		case 404:
			etsyReason = "not_found"
		}
		if etsyReason != "" {
			return &accessWarning{Status: etsyAPIError.StatusCode, Reason: etsyReason, Message: etsyAPIError.Body}, true
		}
	}
	return nil, false
}

const etsyPaymentsForbiddenHint = "Etsy returned 403 for payments: re-authorize with the transactions_r scope (`etsy-pp-cli auth login --scopes \"… transactions_r\"`)"

var errEtsyPaymentsForbidden = errors.New("payments: " + etsyPaymentsForbiddenHint)

func etsyPaymentsForbiddenMessage(n int) string {
	return fmt.Sprintf("%d receipt(s) are currently marked forbidden (403) — usually receipts this token can't see (stale or another shop's); they are retried next sync.", n)
}

func etsyPaymentsForbiddenScopeMessage(n int) string {
	return etsyPaymentsForbiddenMessage(n) + " " + etsyPaymentsForbiddenHint
}

// etsyPaymentsCapExit attributes failures only to payments and reports durable progress.
func etsyPaymentsCapExit(full bool, checked, total int64, failedResources []string) error {
	var etsyPaymentsFailed []string
	for _, etsyResource := range failedResources {
		if etsyResource == "shops_receipts_payments" {
			etsyPaymentsFailed = append(etsyPaymentsFailed, etsyResource)
		}
	}
	if len(etsyPaymentsFailed) > 0 {
		return fmt.Errorf("payments: %s", describeFailedResources(len(etsyPaymentsFailed), etsyPaymentsFailed))
	}
	if checked == 0 {
		return rateLimitErr(fmt.Errorf("payments: 0 of %d receipts checked; no progress was made", total))
	}
	return rateLimitErr(errors.New(etsyPaymentsCappedMessage(full, checked, total)))
}

func etsyPaymentsCapFailed(err error) bool {
	var ce *cliError
	return !errors.As(err, &ce) || ce.code != 7
}

// etsyMaxCallsMessage explains the durable progress available on a re-run.
func etsyMaxCallsMessage(maxCalls int, flatPhase bool) string {
	etsyHint := "payments and other dependent progress is saved, so a re-run resumes where this stopped"
	if flatPhase {
		etsyHint = "a re-run restarts capped listings walks from the first page, so raise --max-calls or narrow --resources to make progress"
	}
	return fmt.Sprintf("--max-calls %d reached; data may be incomplete; re-run sync to continue (%s)", maxCalls, etsyHint)
}

// etsyPaymentsDecision keeps cap stops at exit 7 and classifies run-wide scope problems.
// scopeLost is the final run classification: mixed ok/403 is false; ambiguous
// ever-ok denials remain true unless the single bounded peer verification succeeds.
// checked includes durable 404s, so checked > 0 alone cannot prove token scope.
func etsyPaymentsDecision(capped, full bool, checked, total, forbidden int64, failed []string, scopeLost, anySuccess, strict bool) error {
	scopeProblem := forbidden > 0 && (scopeLost || !anySuccess)
	if capped && checked == 0 && scopeProblem {
		return errEtsyPaymentsForbidden
	}
	if capped {
		return etsyPaymentsCapExit(full, checked, total, failed)
	}
	if forbidden > 0 && strict {
		return fmt.Errorf("payments: %d receipt(s) forbidden under --strict (see the payments_forbidden warning)", forbidden)
	}
	if forbidden > 0 && scopeProblem {
		return errEtsyPaymentsForbidden
	}
	return nil
}
