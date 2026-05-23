// Copyright 2026 servosity. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/cloud/servosity-msp/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/cloud/servosity-msp/internal/snapshot"
	"github.com/mvanhorn/printing-press-library/library/cloud/servosity-msp/internal/store"
)

// attentionCompany is one company row in the attention rollup.
type attentionCompany struct {
	CompanyID        int64  `json:"company_id"`
	CompanyName      string `json:"company_name"`
	Score            int    `json:"score"`
	OpenIssues       int    `json:"open_issues"`
	StaleBackups     int    `json:"stale_backups"`
	DRBackupInFlight int    `json:"drbackup_in_flight"`
}

// attentionResult is the full envelope written both to stdout and the snapshot.
type attentionResult struct {
	TakenAt   time.Time          `json:"taken_at"`
	Companies []attentionCompany `json:"companies"`
	Totals    map[string]int     `json:"totals"`
}

// newAttentionCmd builds the morning fleet-sweep command. It merges open
// issues + stale backup sets (+ in-flight DR events when available) into a
// per-company ranked view, persists the result as a snapshot for `drift`,
// and emits a JSON envelope on stdout.
//
// v1 ranking:  score = (open_issues * 2) + (stale_backups * 3)
//
// Restore-queue weighting is deferred to v0.2 (per-company iteration is too
// expensive without a synced table; see TODO below).
func newAttentionCmd(flags *rootFlags) *cobra.Command {
	var refresh bool
	var since string
	var topN int

	cmd := &cobra.Command{
		Use:   "attention",
		Short: "Morning fleet sweep: rank companies by open issues + stale backups",
		Long: `Merge open issues, stale backup sets, and in-flight DR events into a
per-company ranked view of where your attention is needed today.

By default reads from the local store (run 'sync' first). Pass --refresh to
pull live from the API. Results are snapshotted to pp_snapshots so future
runs of 'drift' can compute day-over-day changes.`,
		Example: `  # Use local store
  servosity-msp-cli attention

  # Pull live, only items newer than 12h, top 5
  servosity-msp-cli attention --refresh --since 12h --top 5`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate --since up front; bail with a usage error before
			// any IO or work that might mask the real issue.
			sinceDur, err := time.ParseDuration(since)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since value %q: %w", since, err))
			}
			if topN <= 0 {
				return usageErr(fmt.Errorf("--top must be > 0, got %d", topN))
			}

			// --dry-run short-circuits BEFORE any IO so verify probes don't
			// touch the store or hit the API.
			if dryRunOK(flags) {
				return nil
			}

			ctx := cmd.Context()
			cutoff := time.Now().Add(-sinceDur)

			// Per-company aggregate, keyed by company id.
			agg := map[int64]*attentionCompany{}

			// ---- 1. Open issues ----
			if refresh {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				// Partner tokens cannot hit /issues/ globally — that
				// endpoint is admin-only and returns 403. Resolve the
				// caller's reseller ID and hit /resellers/{id}/issues/.
				resellerID, err := cliutil.ResolveResellerID(cmd.Context(), c)
				if err != nil {
					return fmt.Errorf("resolving reseller ID: %w", err)
				}
				data, err := c.Get(fmt.Sprintf("/resellers/%d/issues/", resellerID), nil)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				countIssues(data, cutoff, agg)
			} else {
				db, err := store.Open(defaultDBPath("servosity-msp-cli"))
				if err != nil {
					return fmt.Errorf("opening local store: %w\nRun 'servosity-msp-cli sync' first, or pass --refresh.", err)
				}
				if err := countIssuesFromStore(db, cutoff, agg); err != nil {
					db.Close()
					return err
				}
				db.Close()
			}

			// ---- 2. Stale backup sets ----
			// No dedicated store table for this report — always fetch from
			// the API when --refresh, otherwise check the most-recent
			// snapshot we kept under metric "stale-backup-sets". If neither
			// exists the section is empty (not an error).
			if refresh {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				data, err := c.Get("/reports/stale-backup-sets/", nil)
				if err != nil {
					// Non-fatal: /reports/stale-backup-sets/ is admin-only;
					// partner-scoped tokens get 403 here. Log to stderr and
					// continue with empty stale data — open-issues alone
					// still produces a useful attention score.
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: skipping stale-backup section (%v)\n", err)
				} else {
					countStale(data, agg)
				}
			} else {
				db, err := store.Open(defaultDBPath("servosity-msp-cli"))
				if err == nil {
					if snap, _ := snapshot.Latest(ctx, db.DB(), "stale-backup-sets"); snap != nil {
						countStale(snap.Data, agg)
					}
					db.Close()
				}
			}

			// ---- 3. DR backup in-flight ----
			// TODO(v0.2): /companies/{id}/restore-queues/ requires per-company
			// iteration; skip in v1 until a synced restore_queues table exists.
			// All drbackup_in_flight values stay 0 for now.

			// ---- Rank & truncate ----
			companies := make([]attentionCompany, 0, len(agg))
			for _, row := range agg {
				row.Score = row.OpenIssues*2 + row.StaleBackups*3
				companies = append(companies, *row)
			}
			sort.Slice(companies, func(i, j int) bool {
				if companies[i].Score != companies[j].Score {
					return companies[i].Score > companies[j].Score
				}
				// Stable secondary by name then id so output is deterministic
				// even when two companies score identically.
				if companies[i].CompanyName != companies[j].CompanyName {
					return companies[i].CompanyName < companies[j].CompanyName
				}
				return companies[i].CompanyID < companies[j].CompanyID
			})

			totalIssues := 0
			totalStale := 0
			for _, c := range companies {
				totalIssues += c.OpenIssues
				totalStale += c.StaleBackups
			}
			totalCompanies := len(companies)

			if len(companies) > topN {
				companies = companies[:topN]
			}

			result := attentionResult{
				TakenAt:   time.Now().UTC(),
				Companies: companies,
				Totals: map[string]int{
					"companies": totalCompanies,
					"issues":    totalIssues,
					"stale":     totalStale,
				},
			}

			// ---- Persist snapshot for `drift` ----
			payload, err := json.Marshal(result)
			if err != nil {
				return fmt.Errorf("marshal attention result: %w", err)
			}
			if db, err := store.Open(defaultDBPath("servosity-msp-cli")); err == nil {
				if serr := snapshot.Save(ctx, db.DB(), "attention", result.TakenAt, json.RawMessage(payload)); serr != nil {
					// Snapshot save is best-effort; print to stderr but don't
					// fail the user's read. Drift will warn on a missing
					// anchor; that's a clearer signal than an attention exit.
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: snapshot save failed: %v\n", serr)
				}
				db.Close()
			}

			// ---- Emit ----
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			return renderAttentionTable(cmd, result)
		},
	}

	cmd.Flags().BoolVar(&refresh, "refresh", false, "Pull live from the API instead of the local store")
	cmd.Flags().StringVar(&since, "since", "24h", "Only count items newer than this duration (Go duration syntax: 24h, 7d... )")
	cmd.Flags().IntVar(&topN, "top", 10, "Limit output to top N companies by score")

	return cmd
}

// countIssues walks a raw /issues/ payload (paginated envelope or a bare
// array) and increments per-company open-issue counts in agg for rows whose
// created_at is at-or-after cutoff.
func countIssues(data json.RawMessage, cutoff time.Time, agg map[int64]*attentionCompany) {
	for _, item := range unwrapList(data) {
		var row struct {
			Company     any    `json:"company"`
			CompanyName string `json:"company_name"`
			CreatedAt   string `json:"created_at"`
			State       string `json:"state"`
		}
		if err := json.Unmarshal(item, &row); err != nil {
			continue
		}
		// Closed/resolved/archived issues never demand morning attention.
		switch row.State {
		case "closed", "resolved", "archived", "ignored":
			continue
		}
		if row.CreatedAt != "" && !cutoff.IsZero() {
			if t, err := time.Parse(time.RFC3339, row.CreatedAt); err == nil && t.Before(cutoff) {
				continue
			}
		}
		id := coerceID(row.Company)
		if id == 0 {
			continue
		}
		bumpIssues(agg, id, row.CompanyName)
	}
}

// countIssuesFromStore reads the local `issues` table directly. Uses the
// dedicated columns where they exist (company, company_name, created_at,
// state) so the predicate stays an index-friendly SQL filter rather than a
// json_extract scan.
func countIssuesFromStore(db *store.Store, cutoff time.Time, agg map[int64]*attentionCompany) error {
	rows, err := db.DB().Query(`
		SELECT COALESCE(company, 0), COALESCE(company_name, ''), COALESCE(state, '')
		  FROM issues
		 WHERE (created_at IS NULL OR created_at >= ?)
		   AND COALESCE(state,'') NOT IN ('closed','resolved','archived','ignored')`,
		cutoff.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("query issues from store: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var companyID int64
		var companyName, state string
		if err := rows.Scan(&companyID, &companyName, &state); err != nil {
			continue
		}
		if companyID == 0 {
			continue
		}
		bumpIssues(agg, companyID, companyName)
	}
	return rows.Err()
}

// countStale walks a /reports/stale-backup-sets/ payload and increments
// stale_backups per company.
func countStale(data json.RawMessage, agg map[int64]*attentionCompany) {
	for _, item := range unwrapList(data) {
		var row struct {
			Company     any    `json:"company"`
			CompanyID   any    `json:"company_id"`
			CompanyName string `json:"company_name"`
		}
		if err := json.Unmarshal(item, &row); err != nil {
			continue
		}
		id := coerceID(row.Company)
		if id == 0 {
			id = coerceID(row.CompanyID)
		}
		if id == 0 {
			continue
		}
		bumpStale(agg, id, row.CompanyName)
	}
}

// unwrapList normalises a paginated envelope ({"results":[...]} or
// {"items":[...]}) and a bare top-level array into a slice of raw items.
// Returns nil for shapes it can't recognise so callers see an empty rollup
// section rather than a panic.
func unwrapList(data json.RawMessage) []json.RawMessage {
	if len(data) == 0 {
		return nil
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err == nil {
		return arr
	}
	var env struct {
		Results []json.RawMessage `json:"results"`
		Items   []json.RawMessage `json:"items"`
		Data    []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(data, &env); err == nil {
		if len(env.Results) > 0 {
			return env.Results
		}
		if len(env.Items) > 0 {
			return env.Items
		}
		if len(env.Data) > 0 {
			return env.Data
		}
	}
	return nil
}

// coerceID accepts either a JSON number (decoded as float64) or a numeric
// string and returns it as int64; everything else becomes 0.
func coerceID(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	case string:
		var n int64
		_, _ = fmt.Sscanf(t, "%d", &n)
		return n
	}
	return 0
}

func bumpIssues(agg map[int64]*attentionCompany, id int64, name string) {
	row, ok := agg[id]
	if !ok {
		row = &attentionCompany{CompanyID: id, CompanyName: name}
		agg[id] = row
	}
	if row.CompanyName == "" && name != "" {
		row.CompanyName = name
	}
	row.OpenIssues++
}

func bumpStale(agg map[int64]*attentionCompany, id int64, name string) {
	row, ok := agg[id]
	if !ok {
		row = &attentionCompany{CompanyID: id, CompanyName: name}
		agg[id] = row
	}
	if row.CompanyName == "" && name != "" {
		row.CompanyName = name
	}
	row.StaleBackups++
}

func renderAttentionTable(cmd *cobra.Command, r attentionResult) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Attention as of %s — %d companies, %d issues, %d stale backups\n\n",
		r.TakenAt.Format(time.RFC3339), r.Totals["companies"], r.Totals["issues"], r.Totals["stale"])
	if len(r.Companies) == 0 {
		fmt.Fprintln(w, "No companies need attention right now.")
		return nil
	}
	headers := []string{"SCORE", "COMPANY", "ISSUES", "STALE", "DR"}
	rows := make([][]string, 0, len(r.Companies))
	for _, c := range r.Companies {
		name := c.CompanyName
		if name == "" {
			name = fmt.Sprintf("company:%d", c.CompanyID)
		}
		rows = append(rows, []string{
			fmt.Sprintf("%d", c.Score),
			name,
			fmt.Sprintf("%d", c.OpenIssues),
			fmt.Sprintf("%d", c.StaleBackups),
			fmt.Sprintf("%d", c.DRBackupInFlight),
		})
	}
	// Reuse rootFlags.printTable — but it requires a *rootFlags, which we
	// don't have in scope here; emit via a simple tabwriter-less format to
	// keep this file dependency-light. The JSON path handles agent output.
	for i, h := range headers {
		if i > 0 {
			fmt.Fprint(w, "\t")
		}
		fmt.Fprint(w, h)
	}
	fmt.Fprintln(w)
	for _, row := range rows {
		for i, cell := range row {
			if i > 0 {
				fmt.Fprint(w, "\t")
			}
			fmt.Fprint(w, cell)
		}
		fmt.Fprintln(w)
	}
	return nil
}
