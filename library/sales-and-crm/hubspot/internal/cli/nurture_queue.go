// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: ranked daily contact list under `nurture queue`.

package cli

import (
	"database/sql"
	"fmt"
	"math"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
	"github.com/spf13/cobra"
)

func newNovelNurtureQueueCmd(flags *rootFlags) *cobra.Command {
	var owner string
	var top int
	var staleDays int
	var stageUnder string
	var dbPath string
	var filterFlags []string
	var filterDebug bool

	cmd := &cobra.Command{
		Use:         "queue",
		Short:       "Ranked 'who to contact today' list with scoring rationale",
		Long:        `Score = stale_days * 1.0 + log(deal_amount) * 0.5 + (1 - probability) * 100, sorted high to low.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  hubspot-pp-cli nurture queue --top 20
  hubspot-pp-cli nurture queue --owner me --filter 'lifecyclestage=opportunity'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			expr, err := cliutil.ParseFilters(filterFlags)
			if err != nil {
				return err
			}
			if filterDebug {
				fmt.Fprint(cmd.ErrOrStderr(), expr.DebugString())
			}
			if dbPath == "" {
				dbPath = defaultDBPath("hubspot-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			ownerID, err := resolveOwnerArg(db, owner)
			if err != nil {
				return err
			}

			// When filtering is active we widen the candidate pool so rows
			// rejected by --filter don't shrink the visible queue below
			// --top. The 4x multiplier matches the existing 5x candidate
			// fetch as a Fermi estimate; for huge tenants we still cap via
			// the underlying staleDays query.
			fetchLimit := top * 5
			if !expr.IsEmpty() {
				fetchLimit = top * 20
			}
			candidates, source, err := queryNurtureMine(cmd, db, ownerID, staleDays, stageUnder, fetchLimit)
			if err != nil {
				return err
			}

			if !expr.IsEmpty() {
				filterFields := expr.FieldsReferenced()
				kept := candidates[:0]
				for _, c := range candidates {
					var raw sql.NullString
					_ = db.DB().QueryRowContext(cmd.Context(), `SELECT data FROM hubspot_contacts_crm WHERE id = ?`, c.ContactID).Scan(&raw)
					if !expr.Match(extractPropertiesRow(raw.String, filterFields)) {
						continue
					}
					kept = append(kept, c)
				}
				candidates = kept
			}

			type scored struct {
				Rank      int     `json:"rank"`
				ContactID string  `json:"contact_id"`
				Name      string  `json:"name"`
				Score     float64 `json:"score"`
				StaleDays int64   `json:"stale_days"`
				TopAmount float64 `json:"top_deal_amount"`
				TopStage  string  `json:"top_deal_stage"`
				Reason    string  `json:"reason"`
			}
			scoredList := make([]scored, 0, len(candidates))
			for _, c := range candidates {
				amt := c.TopDealAmount
				if amt < 1 {
					amt = 1
				}
				// We don't have per-stage probability without a pipeline lookup;
				// approximate by using 0.5 (mid-funnel) when stage is non-closed.
				probability := 0.5
				score := float64(c.DaysStale)*1.0 + math.Log(amt)*0.5 + (1.0-probability)*100.0
				reason := fmt.Sprintf("%d days idle", c.DaysStale)
				if c.TopDealAmount > 0 {
					reason += fmt.Sprintf(", %s deal in %q", formatAmount(c.TopDealAmount), firstNonEmptyString(c.LatestDealStage, "open"))
				}
				scoredList = append(scoredList, scored{
					ContactID: c.ContactID, Name: c.Name, Score: score,
					StaleDays: c.DaysStale, TopAmount: c.TopDealAmount,
					TopStage: c.LatestDealStage, Reason: reason,
				})
			}
			sort.Slice(scoredList, func(i, j int) bool { return scoredList[i].Score > scoredList[j].Score })
			if top > 0 && len(scoredList) > top {
				scoredList = scoredList[:top]
			}
			for i := range scoredList {
				scoredList[i].Rank = i + 1
			}

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"data_source": source,
					"results":     scoredList,
				})
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "data_source: %s\n", source)
			headers := []string{"rank", "contact_id", "name", "score", "stale_days", "top_deal_amount", "top_deal_stage", "reason"}
			out := make([][]string, 0, len(scoredList))
			for _, s := range scoredList {
				out = append(out, []string{
					fmt.Sprintf("%d", s.Rank), s.ContactID, s.Name,
					fmt.Sprintf("%.2f", s.Score),
					fmt.Sprintf("%d", s.StaleDays),
					formatAmount(s.TopAmount),
					s.TopStage, s.Reason,
				})
			}
			return flags.printTable(cmd, headers, out)
		},
	}
	cmd.Flags().StringVar(&owner, "owner", "me", "Filter by owner id, email, or 'me'")
	cmd.Flags().IntVar(&top, "top", 20, "Maximum number of contacts to return")
	cmd.Flags().IntVar(&staleDays, "stale-days", 14, "Minimum days since last contact for candidate set")
	cmd.Flags().StringVar(&stageUnder, "stage-under", "closedwon", "Treat deals AT or PAST this stage id as closed")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringSliceVar(&filterFlags, "filter", nil, filterDescription)
	cmd.Flags().BoolVar(&filterDebug, "filter-debug", false, "Print parsed --filter expression to stderr before applying it")
	return cmd
}
