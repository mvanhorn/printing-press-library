// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: track review status transitions per ad and for the account.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. This command is local-only.

package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/openai-ads/internal/store"
	"github.com/spf13/cobra"
)

// reviewPoint is one captured review status at a point in time.
type reviewPoint struct {
	Status string
	At     string
}

// reviewChange reports the most recent review-status transition.
type reviewChange struct {
	From        string `json:"last_from"`
	To          string `json:"last_to"`
	At          string `json:"last_changed_at"`
	Transitions int
}

// lastReviewTransition walks an ad's review history in chronological order and
// returns the last place the status changed.
func lastReviewTransition(seq []reviewPoint) reviewChange {
	var c reviewChange
	var prev string
	for _, p := range seq {
		if prev != "" && p.Status != prev {
			c.From = prev
			c.To = p.Status
			c.At = p.At
			c.Transitions++
		}
		prev = p.Status
	}
	return c
}

// reviewWatchRow is the per-ad view.
type reviewWatchRow struct {
	AdID          string `json:"ad_id"`
	Name          string `json:"name"`
	Status        string `json:"status"`
	ReviewStatus  string `json:"review_status"`
	LastFrom      string `json:"last_from"`
	LastTo        string `json:"last_to"`
	LastChangedAt string `json:"last_changed_at"`
	Transitions   int    `json:"transitions"`
}

// accountReviewSummary is the account-wide review posture.
type accountReviewSummary struct {
	Pending  int `json:"pending"`
	Approved int `json:"approved"`
	Rejected int `json:"rejected"`
	Other    int `json:"other"`
}

type reviewWatchOutput struct {
	Ads     []reviewWatchRow     `json:"ads"`
	Account accountReviewSummary `json:"account"`
}

func newNovelReviewWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review-watch",
		Short: "Track approval and review status transitions across the account and every ad.",
		Long: `Report each ad's current review status plus its most recent review-status
transition, reconstructed from the entity_snapshots history written on each
sync, along with an account-wide summary.`,
		Example: strings.Trim(`
  openai-ads-pp-cli review-watch --agent
  openai-ads-pp-cli review-watch --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "review-watch")
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
					return printJSONFiltered(cmd.OutOrStdout(), reviewWatchOutput{Ads: make([]reviewWatchRow, 0), Account: accountReviewSummary{}}, flags)
				}
				return nil
			}
			defer db.Close()

			out, err := loadReviewWatch(db)
			if err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out.Ads) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no ad review history yet")
				return nil
			}
			maps, merr := rowsToMaps(out.Ads)
			if merr != nil {
				return merr
			}
			return printAutoTable(cmd.OutOrStdout(), maps)
		},
	}
	return cmd
}

func loadReviewWatch(db *store.Store) (reviewWatchOutput, error) {
	out := reviewWatchOutput{Ads: make([]reviewWatchRow, 0)}

	// current ad state
	rows, err := db.Query(`SELECT id, name, status, review_status FROM ads`)
	if err != nil {
		return out, fmt.Errorf("querying ads: %w", err)
	}
	cur := map[string]reviewWatchRow{}
	order := []string{}
	for rows.Next() {
		var id, name, status, rs sql.NullString
		if err := rows.Scan(&id, &name, &status, &rs); err != nil {
			_ = rows.Close()
			return out, err
		}
		cur[id.String] = reviewWatchRow{AdID: id.String, Name: name.String, Status: status.String, ReviewStatus: rs.String}
		order = append(order, id.String)
	}
	if err := closeRows(rows); err != nil {
		return out, err
	}

	// history from entity_snapshots
	hist := map[string][]reviewPoint{}
	rows, err = db.Query(`SELECT entity_id, captured_at, json_extract(data, '$.review_status') AS rs
		FROM entity_snapshots WHERE entity_type = 'ads' ORDER BY entity_id, captured_at`)
	if err != nil {
		return out, fmt.Errorf("querying ad review history: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var eid, at, rs sql.NullString
		if err := rows.Scan(&eid, &at, &rs); err != nil {
			return out, err
		}
		if !rs.Valid || rs.String == "" {
			continue
		}
		eidKey := eid.String
		hist[eidKey] = append(hist[eidKey], reviewPoint{Status: rs.String, At: at.String})
	}
	if err := rows.Err(); err != nil {
		return out, err
	}

	seen := map[string]bool{}
	for _, key := range order {
		row := cur[key]
		if pts := hist[key]; len(pts) > 0 {
			chg := lastReviewTransition(pts)
			row.LastFrom = chg.From
			row.LastTo = chg.To
			row.LastChangedAt = chg.At
			row.Transitions = chg.Transitions
		}
		out.Ads = append(out.Ads, row)
		seen[key] = true
		countReviewStatus(&out.Account, row.ReviewStatus)
	}
	// include ads that have history but no current row (deleted) as transitions-only
	for key := range hist {
		if seen[key] {
			continue
		}
		chg := lastReviewTransition(hist[key])
		row := reviewWatchRow{AdID: key, ReviewStatus: lastPoint(hist[key])}
		row.LastFrom = chg.From
		row.LastTo = chg.To
		row.LastChangedAt = chg.At
		row.Transitions = chg.Transitions
		out.Ads = append(out.Ads, row)
		countReviewStatus(&out.Account, row.ReviewStatus)
	}
	return out, nil
}

func lastPoint(pts []reviewPoint) string {
	if len(pts) == 0 {
		return ""
	}
	return pts[len(pts)-1].Status
}

func countReviewStatus(sum *accountReviewSummary, status string) {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "APPROVED":
		sum.Approved++
	case "PENDING":
		sum.Pending++
	case "REJECTED":
		sum.Rejected++
	default:
		if status != "" {
			sum.Other++
		}
	}
}
