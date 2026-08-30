// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: project period-end spend against budget caps and classify pace.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. This command is local-only.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/openai-ads/internal/store"
	"github.com/spf13/cobra"
)

// paceRow is one campaign's pacing projection.
type paceRow struct {
	CampaignID     string `json:"campaign_id"`
	Name           string `json:"name"`
	Status         string `json:"status"`
	SpendMicros    int64  `json:"spend_micros"`
	DailyBudget    string `json:"daily_budget"`    // rendered money
	ProjectedSpend string `json:"projected_spend"` // rendered money
	TotalBudget    string `json:"total_budget"`    // rendered money
	Pace           string `json:"pace"`            // under | on | over
}

// paceOutput is the internal carrier for rows plus an empty-state explanation.
// Only Results reaches stdout — every novel command emits a bare array there so
// agents get one shape across the surface — and Note goes to stderr.
type paceOutput struct {
	Results []paceRow `json:"results"`
	Note    string    `json:"note,omitempty"`
}

// paceProjectedSpend linearly projects the period-end spend from the fraction
// of the window already consumed.
func paceProjectedSpend(spendMicros int64, daysElapsed, days int) int64 {
	if daysElapsed <= 0 {
		return spendMicros
	}
	if daysElapsed > days {
		daysElapsed = days
	}
	return spendMicros * int64(days) / int64(daysElapsed)
}

// paceClassify maps a projected-spend-to-budget ratio to under/on/over pace.
func paceClassify(totalBudgetMicros, projectedMicros int64) string {
	if totalBudgetMicros <= 0 {
		return "unknown"
	}
	ratio := float64(projectedMicros) / float64(totalBudgetMicros)
	switch {
	case ratio > 1.10:
		return "over"
	case ratio < 0.90:
		return "under"
	default:
		return "on"
	}
}

func newNovelPaceCmd(flags *rootFlags) *cobra.Command {
	var flagDays int

	cmd := &cobra.Command{
		Use:   "pace",
		Short: "See whether a campaign will underspend or blow through its cap before the period ends.",
		Long: `Project each campaign's period-end spend from its insight spend over the
last --days days and compare it against its budget cap, classifying the pace
as under, on, or over. Requires campaign insight snapshots captured by sync.`,
		Example: strings.Trim(`
  openai-ads-pp-cli pace --agent
  openai-ads-pp-cli pace --days 14
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "pace")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := openStoreForRead(ctx, "openai-ads-pp-cli")
			if err != nil {
				return err
			}
			if db == nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "hint: run 'openai-ads-pp-cli sync' first to populate the local database.")
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]paceRow, 0), flags)
				}
				return nil
			}
			defer db.Close()

			days := flagDays
			if days <= 0 {
				days = 7
			}
			out, err := loadPace(db, days)
			if err != nil {
				return err
			}
			if out.Note == "" && len(out.Results) == 0 {
				out.Note = "no campaign insight snapshots exist yet; run 'openai-ads-pp-cli sync' to capture them."
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if out.Note != "" {
					fmt.Fprintln(cmd.ErrOrStderr(), "note: "+out.Note)
				}
				return printJSONFiltered(cmd.OutOrStdout(), out.Results, flags)
			}
			if len(out.Results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", out.Note)
				return nil
			}
			maps, merr := rowsToMaps(out.Results)
			if merr != nil {
				return merr
			}
			return printAutoTable(cmd.OutOrStdout(), maps)
		},
	}
	cmd.Flags().IntVar(&flagDays, "days", 7, "Number of days to project over from insight spend.")
	return cmd
}

// loadPace aggregates campaign insight spend and classifies each campaign.
func loadPace(db *store.Store, days int) (paceOutput, error) {
	currency := accountCurrency(db)
	rows, err := db.Query(`SELECT ci.campaigns_id, c.name, c.status, ci.data,
		COALESCE(json_extract(c.data, '$.budget.daily_spend_limit_micros'), json_extract(c.data, '$.budget.lifetime_spend_limit_micros')) AS budget
		FROM campaigns_insights ci
		JOIN campaigns c ON c.id = ci.campaigns_id`)
	if err != nil {
		return paceOutput{}, fmt.Errorf("querying campaign insights: %w", err)
	}
	defer rows.Close()

	var out paceOutput
	order := []string{}
	agg := map[string]*paceAgg{}
	for rows.Next() {
		var (
			cid, name, status sql.NullString
			data              sql.NullString
			budget            sql.NullInt64
		)
		if err := rows.Scan(&cid, &name, &status, &data, &budget); err != nil {
			return out, fmt.Errorf("scanning insight row: %w", err)
		}
		key := cid.String
		if _, ok := agg[key]; !ok {
			ord := paceAgg{
				campaignID:   cid.String,
				name:         name.String,
				status:       status.String,
				budgetMicros: budget.Int64,
			}
			if !budget.Valid {
				ord.budgetMicros = 0
			}
			agg[key] = &ord
			order = append(order, key)
		}
		agg[key].buckets++
		if spend, ok := insightSpendMicros(data.String); ok {
			agg[key].spendMicros += spend
		}
	}
	if err := rows.Err(); err != nil {
		return out, err
	}

	out.Results = make([]paceRow, 0, len(order))
	for _, key := range order {
		a := agg[key]
		daysElapsed := a.buckets
		if daysElapsed < 1 {
			daysElapsed = 1
		}
		projected := paceProjectedSpend(a.spendMicros, daysElapsed, days)
		totalBudget := a.budgetMicros * int64(days)
		out.Results = append(out.Results, paceRow{
			CampaignID:     a.campaignID,
			Name:           a.name,
			Status:         a.status,
			SpendMicros:    a.spendMicros,
			DailyBudget:    renderMicros(a.budgetMicros, currency),
			ProjectedSpend: renderMicros(projected, currency),
			TotalBudget:    renderMicros(totalBudget, currency),
			Pace:           paceClassify(totalBudget, projected),
		})
	}
	return out, nil
}

type paceAgg struct {
	campaignID   string
	name         string
	status       string
	budgetMicros int64
	spendMicros  int64
	buckets      int
}

// insightSpendMicros extracts the spend (in currency units) from an insight
// data blob and converts it to micros. Returns ok=false when spend is absent.
func insightSpendMicros(data string) (int64, bool) {
	if strings.TrimSpace(data) == "" {
		return 0, false
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		return 0, false
	}
	spend := obj["spend"]
	if spend == nil {
		spend = obj["campaign_spend"]
	}
	if spend == nil {
		return 0, false
	}
	switch v := spend.(type) {
	case float64:
		return int64(v * 1e6), true
	case json.Number:
		f, _ := v.Float64()
		return int64(f * 1e6), true
	}
	return 0, false
}
