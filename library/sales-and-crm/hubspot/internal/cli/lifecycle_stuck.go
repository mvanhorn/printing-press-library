// Copyright 2026 sdhilip200. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
	"github.com/spf13/cobra"
)

// lifecycleStuckItem reports a contact whose age in HubSpot exceeds the
// stage-local median by --multiplier. The dwell basis is the contact's
// createDate, not a per-stage entry timestamp, because HubSpot's default
// /crm/v3/objects/contacts response does not return per-stage entry dates.
// Field names use age/created semantics to match what is actually computed;
// the dwell_basis field in the top-level envelope makes the approximation
// explicit for downstream consumers.
type lifecycleStuckItem struct {
	ID               string  `json:"id"`
	Email            string  `json:"email,omitempty"`
	Name             string  `json:"name,omitempty"`
	CreatedAt        string  `json:"created_at,omitempty"`
	AgeDays          float64 `json:"age_days"`
	MultiplierActual float64 `json:"multiplier_actual"`
}

func newNovelLifecycleStuckCmd(flags *rootFlags) *cobra.Command {
	var stage string
	var multiplier float64
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "lifecycle-stuck",
		Short: "Surface contacts whose age in stage exceeds the stage-local median by --multiplier.",
		Long: strings.Trim(`
Compute the median age (now - contact createDate) for every contact currently
in --stage, then flag contacts whose age exceeds --multiplier x median.
The median baseline is scoped to the result set so each stage gets a
stage-local "normal" — what counts as old for a lead is different from
what counts as old for an opportunity.

Dwell-time approximation: HubSpot's default sync does not return per-stage
entry timestamps, so this command treats contact creation as the anchor.
Field names use age semantics (created_at, age_days, median_age_days,
threshold_age_days) and the top-level envelope sets dwell_basis to
"contact_create_date" so callers know the basis. A contact created three
years ago and promoted last week will show a long age; cross-reference with
last_activity_date for activity-based staleness signals.

When fewer than 5 contacts are in the stage the command returns an empty
items array with an explanatory note rather than guessing.
`, "\n"),
		Example: strings.Trim(`
  # Find leads who have been parked at least 2x longer than the median lead
  hubspot-pp-cli lifecycle-stuck --stage lead --multiplier 2.0 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if strings.TrimSpace(stage) == "" {
				return usageErr(fmt.Errorf("--stage is required (one of %s)", strings.Join(canonicalLifecycleStages, ", ")))
			}
			stageNorm := strings.ToLower(strings.TrimSpace(stage))
			validStage := false
			for _, s := range canonicalLifecycleStages {
				if s == stageNorm {
					validStage = true
					break
				}
			}
			if !validStage {
				return usageErr(fmt.Errorf("--stage %q: must be one of %s", stage, strings.Join(canonicalLifecycleStages, ", ")))
			}
			if multiplier <= 0 {
				return usageErr(fmt.Errorf("--multiplier must be > 0"))
			}
			if limit <= 0 {
				return usageErr(fmt.Errorf("--limit must be > 0"))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("hubspot-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			now := time.Now().UTC()
			type sample struct {
				props contactProps
				dwell float64
			}
			var samples []sample
			scanned, parseErrors, err := streamContacts(cmd.Context(), db, func(p contactProps) error {
				if !matchesStage(p.LifecycleStage, stageNorm) {
					return nil
				}
				if p.CreateDate.IsZero() {
					return nil
				}
				dwell := now.Sub(p.CreateDate).Hours() / 24
				if dwell < 0 {
					dwell = 0
				}
				samples = append(samples, sample{props: p, dwell: dwell})
				return nil
			})
			if err != nil {
				return err
			}

			result := map[string]any{
				"stage":        stageNorm,
				"multiplier":   multiplier,
				"scanned":      scanned,
				"items":        []lifecycleStuckItem{},
				"matches":      0,
				"dwell_basis":  "contact_create_date",
			}
			if parseErrors > 0 {
				result["parse_errors"] = parseErrors
			}

			if len(samples) < 5 {
				result["note"] = fmt.Sprintf("insufficient data: stage %s has only %d contacts, need >= 5 for median baseline", stageNorm, len(samples))
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			dwells := make([]float64, 0, len(samples))
			for _, s := range samples {
				dwells = append(dwells, s.dwell)
			}
			med := median(dwells)
			threshold := multiplier * med
			result["median_age_days"] = roundTo(med, 2)
			result["threshold_age_days"] = roundTo(threshold, 2)

			items := make([]lifecycleStuckItem, 0)
			for _, s := range samples {
				if s.dwell <= threshold {
					continue
				}
				ratio := 0.0
				if med > 0 {
					ratio = s.dwell / med
				}
				items = append(items, lifecycleStuckItem{
					ID:               s.props.ID,
					Email:            s.props.Email,
					Name:             contactDisplayName(s.props),
					CreatedAt:        s.props.CreateDate.UTC().Format(time.RFC3339),
					AgeDays:          roundTo(s.dwell, 2),
					MultiplierActual: roundTo(ratio, 2),
				})
			}
			sort.Slice(items, func(i, j int) bool {
				if items[i].AgeDays != items[j].AgeDays {
					return items[i].AgeDays > items[j].AgeDays
				}
				return items[i].ID < items[j].ID
			})
			matches := len(items)
			items = limitInt(items, limit)
			result["items"] = items
			result["matches"] = matches
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&stage, "stage", "", "Lifecycle stage to inspect (lead, marketingqualifiedlead, salesqualifiedlead, opportunity, customer, evangelist, other)")
	cmd.Flags().Float64Var(&multiplier, "multiplier", 2.0, "Dwell-time multiplier vs stage median that defines 'stuck'")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of items to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override default SQLite database path")
	return cmd
}

// matchesStage compares a contact's lifecyclestage to the requested filter.
// The "other" filter catches custom (non-canonical) lifecycle stages so the
// user can audit the long tail. Empty/unset lifecyclestage is NOT a custom
// stage — it's no signal at all — so it's excluded from "other" matches;
// contacts with no stage at all are out of scope for lifecycle-stuck.
func matchesStage(actual, want string) bool {
	a := strings.ToLower(strings.TrimSpace(actual))
	if want == "other" {
		if a == "" {
			return false
		}
		for _, c := range canonicalLifecycleStages {
			if c == "other" {
				continue
			}
			if c == a {
				return false
			}
		}
		return true
	}
	return a == want
}
