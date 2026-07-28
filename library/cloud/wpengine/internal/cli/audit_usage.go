// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: account overage projection from live usage vs plan limits.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/cloud/wpengine/internal/cliutil"
)

type usageProjectionRow struct {
	Account            string   `json:"account"`
	AccountID          string   `json:"account_id"`
	LimitVisitors      float64  `json:"limit_visitors,omitempty"`
	LimitStorageGB     float64  `json:"limit_storage_gb,omitempty"`
	LimitBandwidthGB   float64  `json:"limit_bandwidth_gb,omitempty"`
	BillableVisitsMTD  float64  `json:"billable_visits_mtd"`
	ProjectedVisits    float64  `json:"projected_visits"`
	StorageGB          float64  `json:"storage_gb"`
	BandwidthGBMTD     float64  `json:"bandwidth_gb_mtd"`
	ProjectedBandwidth float64  `json:"projected_bandwidth_gb"`
	VisitsPctOfLimit   float64  `json:"visits_pct_of_limit,omitempty"`
	StoragePctOfLimit  float64  `json:"storage_pct_of_limit,omitempty"`
	Status             string   `json:"status"`
	Flags              []string `json:"flags,omitempty"`
}

type usageFetchFailure struct {
	AccountID string `json:"account_id"`
	Step      string `json:"step"`
	Error     string `json:"error"`
}

type auditUsageView struct {
	HorizonDays     int                  `json:"horizon_days"`
	DaysElapsed     int                  `json:"days_elapsed_in_month"`
	Accounts        []usageProjectionRow `json:"accounts"`
	FetchFailures   []usageFetchFailure  `json:"fetch_failures,omitempty"`
	ScannedAccounts int                  `json:"scanned_accounts"`
}

func newNovelAuditUsageCmd(flags *rootFlags) *cobra.Command {
	var flagHorizon string

	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Project billable visits and storage against account limits and flag accounts trending over",
		Long: strings.Trim(`
Fetches every account's plan limits and month-to-date usage summary from the
live API, then projects usage forward at the current daily rate over the
horizon. Accounts projected past a limit are flagged 'over'; past 90% of a
limit, 'warning'.

Visits and bandwidth are cumulative month-to-date and get projected; storage
is a level and is compared directly against the limit.
`, "\n"),
		Example: strings.Trim(`
  # Who is trending over their plan in the next 30 days?
  wpengine-pp-cli audit usage --horizon 30d --agent

  # Quick check with defaults
  wpengine-pp-cli audit usage
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch account limits and usage summaries, then project", flagHorizon, "ahead")
				return nil
			}
			if err := rejectLocalDataSource(flags); err != nil {
				return err
			}
			horizon, err := cliutil.ParseDurationLoose(flagHorizon)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --horizon %q: %w (use forms like 30d, 2w)", flagHorizon, err))
			}
			horizonDays := int(math.Round(horizon.Hours() / 24))
			if horizonDays < 1 {
				horizonDays = 1
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Enumerate accounts (paginated).
			type acctRef struct{ id, name string }
			accounts := make([]acctRef, 0)
			offset := 0
			pages := 0
			for {
				pages++
				if pages > 100 {
					fmt.Fprintln(cmd.ErrOrStderr(), "warning: stopped account enumeration after 100 pages; projections cover the accounts fetched so far")
					break
				}
				page, err := c.Get(ctx, "/accounts", map[string]string{"limit": "100", "offset": fmt.Sprintf("%d", offset)})
				if err != nil {
					return apiErr(fmt.Errorf("listing accounts: %w", err))
				}
				var envl struct {
					Results []struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"results"`
					Next json.RawMessage `json:"next"`
				}
				if err := json.Unmarshal(page, &envl); err != nil {
					return apiErr(fmt.Errorf("parsing accounts response: %w", err))
				}
				for _, a := range envl.Results {
					accounts = append(accounts, acctRef{id: a.ID, name: a.Name})
				}
				if len(envl.Results) < 100 || string(envl.Next) == "null" || len(envl.Next) == 0 {
					break
				}
				offset += 100
				if cliutil.IsDogfoodEnv() {
					break // curtail pagination under live dogfood
				}
			}

			now := time.Now().UTC()
			daysElapsed := now.Day()
			view := auditUsageView{
				HorizonDays:     horizonDays,
				DaysElapsed:     daysElapsed,
				Accounts:        make([]usageProjectionRow, 0, len(accounts)),
				FetchFailures:   make([]usageFetchFailure, 0),
				ScannedAccounts: len(accounts),
			}

			metricSum := func(m map[string]json.RawMessage, key string) float64 {
				raw, ok := m[key]
				if !ok {
					return 0
				}
				var inner map[string]json.RawMessage
				if err := json.Unmarshal(raw, &inner); err != nil {
					return 0
				}
				v, _ := cliutil.ExtractNumber(inner, "sum")
				return v
			}
			metricLatest := func(m map[string]json.RawMessage, key string) float64 {
				raw, ok := m[key]
				if !ok {
					return 0
				}
				var inner map[string]json.RawMessage
				if err := json.Unmarshal(raw, &inner); err != nil {
					return 0
				}
				latestRaw, ok := inner["latest"]
				if !ok {
					return 0
				}
				var latest map[string]json.RawMessage
				if err := json.Unmarshal(latestRaw, &latest); err != nil {
					return 0
				}
				v, _ := cliutil.ExtractNumber(latest, "value")
				return v
			}

			const gib = 1024 * 1024 * 1024

			for _, acct := range accounts {
				// Account ids feed URL path construction; skip malformed ones
				// instead of splicing them into request paths.
				if !uuidRe.MatchString(acct.id) {
					view.FetchFailures = append(view.FetchFailures, usageFetchFailure{AccountID: acct.id, Step: "id-validation", Error: "account id is not a UUID; skipped"})
					continue
				}
				row := usageProjectionRow{Account: acct.name, AccountID: acct.id, Status: "ok"}

				limitsData, err := c.Get(ctx, "/accounts/"+acct.id+"/limits", nil)
				if err != nil {
					view.FetchFailures = append(view.FetchFailures, usageFetchFailure{AccountID: acct.id, Step: "limits", Error: err.Error()})
					continue
				}
				var limits map[string]json.RawMessage
				if err := json.Unmarshal(limitsData, &limits); err != nil {
					view.FetchFailures = append(view.FetchFailures, usageFetchFailure{AccountID: acct.id, Step: "limits", Error: "unparseable response"})
					continue
				}
				var okV, okS, okB bool
				row.LimitVisitors, okV = cliutil.ExtractNumber(limits, "visitors")
				row.LimitStorageGB, okS = cliutil.ExtractNumber(limits, "storage")
				row.LimitBandwidthGB, okB = cliutil.ExtractNumber(limits, "bandwidth")
				if !okV && !okS && !okB {
					// Zero recognizable limits would silently disable every
					// over-limit check and report a false "ok".
					view.FetchFailures = append(view.FetchFailures, usageFetchFailure{AccountID: acct.id, Step: "limits", Error: "no recognizable limit fields in response"})
					continue
				}

				summaryData, err := c.Get(ctx, "/accounts/"+acct.id+"/usage/summary", nil)
				if err != nil {
					view.FetchFailures = append(view.FetchFailures, usageFetchFailure{AccountID: acct.id, Step: "usage/summary", Error: err.Error()})
					continue
				}
				var summary map[string]json.RawMessage
				if err := json.Unmarshal(summaryData, &summary); err != nil {
					view.FetchFailures = append(view.FetchFailures, usageFetchFailure{AccountID: acct.id, Step: "usage/summary", Error: "unparseable response"})
					continue
				}
				// A structurally valid body with none of the expected metric
				// keys would silently project zeros and report a false "ok".
				if _, hasBV := summary["billable_visits"]; !hasBV {
					if _, hasNT := summary["network_total_bytes"]; !hasNT {
						view.FetchFailures = append(view.FetchFailures, usageFetchFailure{AccountID: acct.id, Step: "usage/summary", Error: "no recognizable usage fields in response"})
						continue
					}
				}

				row.BillableVisitsMTD = metricSum(summary, "billable_visits")
				bandwidthBytes := metricSum(summary, "network_total_bytes")
				row.BandwidthGBMTD = round2(bandwidthBytes / gib)
				storageBytes := metricLatest(summary, "storage_file_bytes") + metricLatest(summary, "storage_database_bytes")
				row.StorageGB = round2(storageBytes / gib)

				// Linear projection at the current daily rate.
				rateVisits := row.BillableVisitsMTD / float64(daysElapsed)
				rateBandwidth := row.BandwidthGBMTD / float64(daysElapsed)
				row.ProjectedVisits = math.Round(row.BillableVisitsMTD + rateVisits*float64(horizonDays))
				row.ProjectedBandwidth = round2(row.BandwidthGBMTD + rateBandwidth*float64(horizonDays))

				if row.LimitVisitors > 0 {
					row.VisitsPctOfLimit = round2(100 * row.ProjectedVisits / row.LimitVisitors)
					if row.ProjectedVisits > row.LimitVisitors {
						row.Flags = append(row.Flags, "projected_visits_over_limit")
					} else if row.ProjectedVisits > 0.9*row.LimitVisitors {
						row.Flags = append(row.Flags, "projected_visits_near_limit")
					}
				}
				if row.LimitStorageGB > 0 {
					row.StoragePctOfLimit = round2(100 * row.StorageGB / row.LimitStorageGB)
					if row.StorageGB > row.LimitStorageGB {
						row.Flags = append(row.Flags, "storage_over_limit")
					} else if row.StorageGB > 0.9*row.LimitStorageGB {
						row.Flags = append(row.Flags, "storage_near_limit")
					}
				}
				if row.LimitBandwidthGB > 0 {
					if row.ProjectedBandwidth > row.LimitBandwidthGB {
						row.Flags = append(row.Flags, "projected_bandwidth_over_limit")
					} else if row.ProjectedBandwidth > 0.9*row.LimitBandwidthGB {
						row.Flags = append(row.Flags, "projected_bandwidth_near_limit")
					}
				}
				over := false
				for _, f := range row.Flags {
					if strings.HasSuffix(f, "_over_limit") {
						over = true
						break
					}
				}
				switch {
				case over:
					row.Status = "over"
				case len(row.Flags) > 0:
					row.Status = "warning"
				}
				view.Accounts = append(view.Accounts, row)
			}

			if len(view.FetchFailures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d account fetches failed; projections computed over the remaining %d accounts\n",
					len(view.FetchFailures), len(accounts), len(view.Accounts))
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagHorizon, "horizon", "30d", "Projection window ahead of today (e.g. 30d, 2w)")
	return cmd
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
