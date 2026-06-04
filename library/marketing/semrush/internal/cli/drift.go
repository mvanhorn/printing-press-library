// Copyright 2026 Charles Garrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Novel feature #1 — weekly drift report. Hand-authored. NOT generator-emitted.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type driftRow struct {
	Phrase         string   `json:"phrase"`
	LatestPosition *float64 `json:"latest_position,omitempty"`
	PriorPosition  *float64 `json:"prior_position,omitempty"`
	DeltaPosition  *float64 `json:"delta_position,omitempty"`
	LatestKD       *float64 `json:"latest_kd,omitempty"`
	PriorKD        *float64 `json:"prior_kd,omitempty"`
	DeltaKD        *float64 `json:"delta_kd,omitempty"`
	LatestTraffic  *float64 `json:"latest_traffic,omitempty"`
	PriorTraffic   *float64 `json:"prior_traffic,omitempty"`
	Status         string   `json:"status"` // gainer | loser | new | lost
}

type driftReport struct {
	Domain   string     `json:"domain"`
	Since    string     `json:"since"`
	Database string     `json:"database"`
	Cutoff   time.Time  `json:"cutoff"`
	Gainers  []driftRow `json:"gainers"`
	Losers   []driftRow `json:"losers"`
	New      []driftRow `json:"new"`
	Lost     []driftRow `json:"lost"`
}

func newDriftCmd(flags *rootFlags) *cobra.Command {
	var since string
	var limit int
	var database string

	cmd := &cobra.Command{
		Use:   "drift [domain]",
		Short: "Diff two snapshots of organic keyword positions for a domain and emit gainers/losers/new/lost.",
		Long: `drift compares the latest snapshot of organic keyword positions for a
domain against the snapshot closest to now - --since, and emits the
keywords that moved up, moved down, newly appeared, or fell out of the
ranked set. All data is read from the local SQLite store — no API
credits are spent.

Run 'semrush-pp-cli sync --resource keyword' to populate the store first.`,
		Example:     "  semrush-pp-cli drift apple.com --since 7d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openNovelStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()

			recordBalanceSnapshotForCmd(ctx, db, flags, cmd.CommandPath(), cmd.ErrOrStderr())

			if !hintIfUnsynced(cmd, db, "keyword") {
				hintIfStale(cmd, db, "keyword", flags.maxAge)
			}

			domain := args[0]
			window, err := parseSince(since)
			if err != nil {
				return usageErr(err)
			}
			cutoff := time.Now().Add(-window)

			rows, err := db.DB().QueryContext(ctx,
				`SELECT json_extract(data, '$.Ph') AS phrase,
				        json_extract(data, '$.Po') AS position,
				        json_extract(data, '$.Kd') AS kd,
				        json_extract(data, '$.Tr') AS traffic,
				        json_extract(data, '$.database') AS database,
				        synced_at
				 FROM resources
				 WHERE resource_type IN ('keyword', 'domain_keywords')
				   AND (json_extract(data, '$.domain') = ? OR json_extract(data, '$.Dn') = ?)
				   AND (? = '' OR json_extract(data, '$.database') = ? OR json_extract(data, '$.database') IS NULL)
				 ORDER BY synced_at DESC`,
				domain, domain, database, database,
			)
			if err != nil {
				return fmt.Errorf("query keyword positions: %w", err)
			}
			defer rows.Close()

			type snap struct {
				position *float64
				kd       *float64
				traffic  *float64
				when     time.Time
			}
			latest := map[string]snap{}
			prior := map[string]snap{}

			for rows.Next() {
				var phrase, dbCol *string
				var posRaw, kdRaw, trafficRaw *float64
				var syncedAt time.Time
				if err := rows.Scan(&phrase, &posRaw, &kdRaw, &trafficRaw, &dbCol, &syncedAt); err != nil {
					return fmt.Errorf("scan keyword row: %w", err)
				}
				if phrase == nil || strings.TrimSpace(*phrase) == "" {
					continue
				}
				s := snap{position: posRaw, kd: kdRaw, traffic: trafficRaw, when: syncedAt}
				if syncedAt.After(cutoff) {
					// Within --since window. The query uses ORDER BY synced_at DESC,
					// so the first within-window row we see for a phrase is the
					// newest; subsequent within-window rows are older. Place the
					// newest in `latest` and any other within-window row in `prior`
					// (preferring the newest of those). This populates `prior` for
					// users who synced multiple times inside the window even when
					// no pre-cutoff snapshot exists.
					existing, hasLatest := latest[*phrase]
					switch {
					case !hasLatest:
						latest[*phrase] = s
					case syncedAt.After(existing.when):
						// Newer than current latest (defensive — shouldn't happen
						// with DESC ordering, but handle ASC + concurrent writes).
						if priorExisting, hasPrior := prior[*phrase]; !hasPrior || existing.when.After(priorExisting.when) {
							prior[*phrase] = existing
						}
						latest[*phrase] = s
					default:
						// Older than current latest but still within window: candidate for prior.
						if priorExisting, hasPrior := prior[*phrase]; !hasPrior || syncedAt.After(priorExisting.when) {
							prior[*phrase] = s
						}
					}
				} else {
					if existing, ok := prior[*phrase]; !ok || syncedAt.After(existing.when) {
						prior[*phrase] = s
					}
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterate keyword rows: %w", err)
			}

			report := driftReport{
				Domain:   domain,
				Since:    since,
				Database: database,
				Cutoff:   cutoff,
			}
			for phrase, l := range latest {
				p, hadPrior := prior[phrase]
				row := driftRow{Phrase: phrase}
				row.LatestPosition = l.position
				row.LatestKD = l.kd
				row.LatestTraffic = l.traffic
				if !hadPrior {
					row.Status = "new"
					report.New = append(report.New, row)
					continue
				}
				row.PriorPosition = p.position
				row.PriorKD = p.kd
				row.PriorTraffic = p.traffic
				if l.position != nil && p.position != nil {
					d := *l.position - *p.position
					row.DeltaPosition = &d
				}
				if l.kd != nil && p.kd != nil {
					d := *l.kd - *p.kd
					row.DeltaKD = &d
				}
				if row.DeltaPosition != nil && *row.DeltaPosition < 0 {
					row.Status = "gainer"
					report.Gainers = append(report.Gainers, row)
				} else if row.DeltaPosition != nil && *row.DeltaPosition > 0 {
					row.Status = "loser"
					report.Losers = append(report.Losers, row)
				}
			}
			for phrase, p := range prior {
				if _, ok := latest[phrase]; ok {
					continue
				}
				row := driftRow{Phrase: phrase, Status: "lost"}
				row.PriorPosition = p.position
				row.PriorKD = p.kd
				row.PriorTraffic = p.traffic
				report.Lost = append(report.Lost, row)
			}

			// Sort each category for deterministic, useful top-N before
			// applying --limit. Gainers/Losers ranked by absolute delta
			// (biggest movers first); New/Lost have no delta so rank by
			// latest/prior traffic desc (most-impactful first), tiebreak
			// by phrase asc. Without sorting, ranging over the latest map
			// upstream would produce a random sample under --limit.
			derefOrZero := func(p *float64) float64 {
				if p == nil {
					return 0
				}
				return *p
			}
			rankByAbsDelta := func(xs []driftRow) {
				sort.SliceStable(xs, func(i, j int) bool {
					di, dj := math.Abs(derefOrZero(xs[i].DeltaPosition)), math.Abs(derefOrZero(xs[j].DeltaPosition))
					if di != dj {
						return di > dj
					}
					return xs[i].Phrase < xs[j].Phrase
				})
			}
			rankByTraffic := func(xs []driftRow, useLatest bool) {
				sort.SliceStable(xs, func(i, j int) bool {
					var ti, tj float64
					if useLatest {
						ti, tj = derefOrZero(xs[i].LatestTraffic), derefOrZero(xs[j].LatestTraffic)
					} else {
						ti, tj = derefOrZero(xs[i].PriorTraffic), derefOrZero(xs[j].PriorTraffic)
					}
					if ti != tj {
						return ti > tj
					}
					return xs[i].Phrase < xs[j].Phrase
				})
			}
			rankByAbsDelta(report.Gainers)
			rankByAbsDelta(report.Losers)
			rankByTraffic(report.New, true)
			rankByTraffic(report.Lost, false)

			trimDrift := func(xs []driftRow) []driftRow {
				if limit > 0 && len(xs) > limit {
					return xs[:limit]
				}
				return xs
			}
			report.Gainers = trimDrift(report.Gainers)
			report.Losers = trimDrift(report.Losers)
			report.New = trimDrift(report.New)
			report.Lost = trimDrift(report.Lost)

			raw, err := json.Marshal(report)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "Compare latest snapshot vs snapshot closest to now-since (e.g. 7d, 24h, 4w)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum rows per category (gainers/losers/new/lost)")
	cmd.Flags().StringVar(&database, "database", "us", "Filter to a single Semrush database/country (us, uk, de, ...); empty matches all")
	return cmd
}
