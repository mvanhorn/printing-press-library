// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/durianpay/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/durianpay/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source local

// webhookRetryLadder is Durianpay's disbursement webhook retry schedule, in
// minutes. A row that has been pending for longer than the sum of the first N
// intervals has had N retries elapse.
var webhookRetryLadder = []int{2, 5, 10, 90, 210}

type stuckItem struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	AgeMinutes     int    `json:"age_minutes"`
	RetriesElapsed int    `json:"retries_elapsed"`
	NextRetryIn    string `json:"next_retry_in,omitempty"`
}

type stuckResult struct {
	Items     []stuckItem `json:"items"`
	OlderThan string      `json:"older_than"`
	Scanned   int         `json:"scanned"`
	Note      string      `json:"note,omitempty"`
}

// retriesElapsed returns how many webhook retries have fired given an age in
// minutes, by walking the cumulative retry ladder. Pure function — unit tested.
func retriesElapsed(ageMinutes int, ladder []int) int {
	elapsed := 0
	cumulative := 0
	for _, interval := range ladder {
		cumulative += interval
		if ageMinutes >= cumulative {
			elapsed++
		} else {
			break
		}
	}
	return elapsed
}

// nextRetryIn returns a human label for the next scheduled retry given the
// number elapsed, or "" when the ladder is exhausted.
func nextRetryIn(elapsed int, ladder []int) string {
	if elapsed >= len(ladder) {
		return ""
	}
	cumulative := 0
	for i := 0; i <= elapsed; i++ {
		cumulative += ladder[i]
	}
	return fmt.Sprintf("at ~%dm total age", cumulative)
}

func disbursementIsPending(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "pending", "processing":
		return true
	}
	return false
}

func queryStuckDisbursements(db *store.Store, olderThan time.Duration, now time.Time) ([]stuckItem, int, error) {
	rows, err := db.DB().Query(
		`SELECT id,
		        json_extract(data,'$.status'),
		        ` + reconcileTimeExpr + `
		   FROM resources WHERE resource_type = 'disbursements'`)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]stuckItem, 0)
	scanned := 0
	for rows.Next() {
		scanned++
		var id string
		var status, created sql.NullString
		if err := rows.Scan(&id, &status, &created); err != nil {
			return nil, 0, err
		}
		if !disbursementIsPending(nullStr(status)) {
			continue
		}
		var age time.Duration
		if created.Valid {
			if t, ok := parseLooseTime(created.String); ok {
				age = now.Sub(t)
			}
		}
		if age < olderThan {
			continue
		}
		ageMin := int(age.Minutes())
		elapsed := retriesElapsed(ageMin, webhookRetryLadder)
		items = append(items, stuckItem{
			ID:             id,
			Status:         nullStr(status),
			AgeMinutes:     ageMin,
			RetriesElapsed: elapsed,
			NextRetryIn:    nextRetryIn(elapsed, webhookRetryLadder),
		})
	}
	return items, scanned, rows.Err()
}

func newNovelStuckCmd(flags *rootFlags) *cobra.Command {
	var flagOlderThan string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "stuck",
		Short:       "List disbursements still pending past a chosen age, bucketed against the webhook retry ladder.",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--older-than=90m"},
		Long: `List local disbursement rows still pending/processing past --older-than,
bucketed by how many webhook retries (2/5/10/90/210 minutes) have elapsed.

Disbursements have no list endpoint, so local rows exist only when other CLI
commands recorded them. Reads only the local store. Offline.`,
		Example: strings.TrimLeft(`
  durianpay-pp-cli stuck
  durianpay-pp-cli stuck --older-than=210m --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := rejectLiveDataSource(cmd, flags, "stuck"); err != nil {
				return err
			}
			olderThan, err := cliutil.ParseDurationLoose(flagOlderThan)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --older-than %q: %w", flagOlderThan, err))
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.ErrOrStderr(), "dry-run: would query local store (disbursements)")
				return nil
			}
			db, err := openLocalStore(dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			hintIfUnsynced(cmd, db, "disbursements")
			hintIfStale(cmd, db, "disbursements", flags.maxAge)

			items, scanned, err := queryStuckDisbursements(db, olderThan, time.Now())
			if err != nil {
				return fmt.Errorf("querying disbursements: %w", err)
			}

			res := stuckResult{Items: items, OlderThan: flagOlderThan, Scanned: scanned}
			if scanned == 0 {
				res.Note = "no local disbursement rows; disbursements are recorded when fetched or submitted via this CLI — run 'durianpay-pp-cli disbursements fetch-by-id <id>' or check --db path"
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			out := cmd.OutOrStdout()
			if res.Note != "" {
				fmt.Fprintln(out, res.Note)
				return nil
			}
			fmt.Fprintf(out, "older-than=%s  scanned=%d  stuck=%d\n", res.OlderThan, res.Scanned, len(res.Items))
			for _, it := range res.Items {
				fmt.Fprintf(out, "  %s status=%s age=%dm retries_elapsed=%d", it.ID, it.Status, it.AgeMinutes, it.RetriesElapsed)
				if it.NextRetryIn != "" {
					fmt.Fprintf(out, " next_retry=%s", it.NextRetryIn)
				}
				fmt.Fprintln(out)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagOlderThan, "older-than", "90m", "Only list disbursements pending longer than this (e.g. 90m, 4h)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite store (default: standard data dir)")
	return cmd
}
