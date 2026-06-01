// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/revenuecat/internal/client"
	"github.com/mvanhorn/printing-press-library/library/payments/revenuecat/internal/store"
	"github.com/spf13/cobra"
)

// snapshotMetric is one named overview metric with its value and unit.
type snapshotMetric struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
	Delta float64 `json:"delta_vs_last"`
}

type revenueSnapshotView struct {
	ProjectID    string           `json:"project_id"`
	CapturedAt   string           `json:"captured_at"`
	Metrics      []snapshotMetric `json:"metrics"`
	MRR          float64          `json:"mrr"`
	ARR          float64          `json:"arr"`
	ActiveSubs   float64          `json:"active_subscriptions"`
	ActiveTrials float64          `json:"active_trials"`
	Revenue      float64          `json:"revenue"`
	HasPrior     bool             `json:"has_prior_snapshot"`
	PriorAt      string           `json:"prior_captured_at,omitempty"`
	Note         string           `json:"note,omitempty"`
}

func newNovelRevenueSnapshotCmd(flags *rootFlags) *cobra.Command {
	var projectFlag string
	var dbPath string
	var flagCurrency string
	cmd := &cobra.Command{
		Use:   "revenue-snapshot",
		Short: "Point-in-time MRR / ARR / active subs / trials / revenue with a diff vs the prior snapshot",
		Long: `Captures the current overview metrics (MRR, ARR, active subscriptions, active
trials, revenue) for a project via the live /metrics/overview endpoint, persists
the run to a local 'rc_snapshots' history table, and reports the delta against
the previous run.

Use this command for the current-moment revenue rollup and its run-over-run
delta. Do NOT use it for the MRR-over-time line and movement breakdown; use
'mrr-trend' instead.

Data source: live (overview metrics are live-only; the rc_snapshots table is
this command's own history ledger).`,
		Example: "  revenuecat-pp-cli revenue-snapshot --project proj1ab2c3d4 --json",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would capture overview metrics and diff against the prior snapshot")
				return nil
			}
			projectID, err := resolveProjectID(projectFlag)
			if err != nil {
				return err
			}
			if dbPath == "" {
				dbPath = defaultDBPath("revenuecat-pp-cli")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			view, err := runRevenueSnapshot(cmd.Context(), c, db, projectID, flagCurrency)
			if err != nil {
				return apiErr(err)
			}
			return emitRevenueSnapshot(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&projectFlag, "project", "", "RevenueCat project id (or set REVENUECAT_PROJECT_ID)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite database path")
	cmd.Flags().StringVar(&flagCurrency, "currency", "", "Currency for metrics (USD, EUR, GBP, AUD, CAD, JPY, BRL, KRW, CNY, MXN, SEK, PLN)")
	return cmd
}

func emitRevenueSnapshot(cmd *cobra.Command, flags *rootFlags, view revenueSnapshotView) error {
	if len(view.Metrics) > 0 && wantsHumanTable(cmd.OutOrStdout(), flags) {
		items := make([]map[string]any, 0, len(view.Metrics))
		for _, m := range view.Metrics {
			items = append(items, map[string]any{
				"metric":        m.Name,
				"id":            m.ID,
				"value":         fmt.Sprintf("%.2f", m.Value),
				"unit":          m.Unit,
				"delta_vs_last": fmt.Sprintf("%+.2f", m.Delta),
			})
		}
		if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\nProject %s snapshot at %s", view.ProjectID, view.CapturedAt)
		if view.HasPrior {
			fmt.Fprintf(cmd.OutOrStdout(), " (diff vs %s)", view.PriorAt)
		}
		fmt.Fprintln(cmd.OutOrStdout())
		if view.Note != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Note: %s\n", view.Note)
		}
		return nil
	}
	return flags.printJSON(cmd, view)
}

// runRevenueSnapshot fetches the live overview metrics, diffs them against the
// prior persisted snapshot, then persists this run.
//
// TODO(verify): confirm exact metric ids (mrr/arr/active_subscriptions/
// active_trials/revenue) against live /metrics/overview data.
func runRevenueSnapshot(ctx context.Context, c *client.Client, db *store.Store, projectID, currency string) (revenueSnapshotView, error) {
	now := time.Now().UTC()
	view := revenueSnapshotView{
		ProjectID:  projectID,
		CapturedAt: now.Format(time.RFC3339),
		Metrics:    []snapshotMetric{},
	}

	path := replacePathParam("/projects/{project_id}/metrics/overview", "project_id", projectID)
	var params map[string]string
	if currency != "" {
		params = map[string]string{"currency": currency}
	}
	raw, err := c.Get(ctx, path, params)
	if err != nil {
		return view, fmt.Errorf("fetching overview metrics: %w", err)
	}
	var resp struct {
		Metrics []struct {
			ID    string  `json:"id"`
			Name  string  `json:"name"`
			Value float64 `json:"value"`
			Unit  string  `json:"unit"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return view, fmt.Errorf("parsing overview metrics: %w", err)
	}

	// Pull prior snapshot (if any) to compute deltas before we persist this run.
	prior, priorAt, hasPrior, priorErr := loadPriorSnapshot(ctx, db, projectID)
	view.HasPrior = hasPrior
	if hasPrior {
		view.PriorAt = priorAt
	}

	for _, m := range resp.Metrics {
		sm := snapshotMetric{ID: m.ID, Name: m.Name, Value: m.Value, Unit: m.Unit}
		if hasPrior {
			if pv, ok := prior[m.ID]; ok {
				sm.Delta = m.Value - pv
			}
		}
		view.Metrics = append(view.Metrics, sm)
		switch m.ID {
		case "mrr":
			view.MRR = m.Value
		case "arr":
			view.ARR = m.Value
		case "active_subscriptions":
			view.ActiveSubs = m.Value
		case "active_trials":
			view.ActiveTrials = m.Value
		case "revenue":
			view.Revenue = m.Value
		}
	}
	// Stable ordering for deterministic output.
	sort.Slice(view.Metrics, func(i, j int) bool { return view.Metrics[i].ID < view.Metrics[j].ID })

	if err := persistSnapshot(ctx, db, view); err != nil {
		// Persisting history is best-effort; surface as a note rather than
		// failing the whole command (the live metrics are still valid).
		view.Note = "captured metrics but failed to persist snapshot history: " + err.Error()
	}
	if len(view.Metrics) == 0 && view.Note == "" {
		view.Note = "overview endpoint returned no metrics"
	} else if priorErr != nil && view.Note == "" {
		// Distinct from a genuine first run: the prior snapshot existed but
		// couldn't be read, so deltas are zero because they were suppressed.
		view.Note = "prior snapshot unavailable (" + priorErr.Error() + "); deltas suppressed"
	} else if !hasPrior && view.Note == "" {
		view.Note = "first snapshot for this project; deltas are zero until the next run"
	}
	return view, nil
}

// loadPriorSnapshot returns the most recent prior snapshot's per-metric values
// keyed by metric id, its captured_at, and whether one exists. The returned
// error is non-nil only for a real failure (DB error or a corrupt prior blob),
// distinct from the normal first-run case (no row → ok=false, err=nil) so the
// caller can tell "no prior yet" apart from "prior unreadable".
func loadPriorSnapshot(ctx context.Context, db *store.Store, projectID string) (map[string]float64, string, bool, error) {
	out := map[string]float64{}
	row := db.DB().QueryRowContext(ctx,
		`SELECT captured_at, metrics_json FROM rc_snapshots
		 WHERE project_id = ? ORDER BY captured_at DESC, id DESC LIMIT 1`,
		projectID,
	)
	var capturedAt string
	var metricsJSON sql.NullString
	if err := row.Scan(&capturedAt, &metricsJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return out, "", false, nil // normal first run
		}
		return out, "", false, fmt.Errorf("reading prior snapshot: %w", err)
	}
	if metricsJSON.Valid && metricsJSON.String != "" {
		if err := json.Unmarshal([]byte(metricsJSON.String), &out); err != nil {
			// A corrupt prior blob would otherwise yield an empty prior map and
			// make every delta read as the full current value; suppress deltas.
			return map[string]float64{}, "", false, fmt.Errorf("prior snapshot metrics are corrupt: %w", err)
		}
	}
	return out, capturedAt, true, nil
}

// persistSnapshot writes one rc_snapshots row for this run.
func persistSnapshot(ctx context.Context, db *store.Store, view revenueSnapshotView) error {
	values := map[string]float64{}
	for _, m := range view.Metrics {
		values[m.ID] = m.Value
	}
	blob, err := json.Marshal(values)
	if err != nil {
		return err
	}
	_, err = db.DB().ExecContext(ctx,
		`INSERT INTO rc_snapshots
		 (project_id, captured_at, mrr, arr, active_subs, active_trials, revenue, metrics_json)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		view.ProjectID, view.CapturedAt, view.MRR, view.ARR,
		view.ActiveSubs, view.ActiveTrials, view.Revenue, string(blob),
	)
	return err
}
