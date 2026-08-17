// Copyright 2026 waveriderai and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Hand-authored body.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

// pp:data-source auto
//
// movers spends exactly ONE live request on category_rankings (that single
// response carries the free, grossing, and paid charts), then reads and writes
// the local rank_snapshot mirror to derive new-entrant status. The API's own
// previous_rank drives the delta; the local mirror only answers "was this app
// on the chart the last time we looked".

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/sensortower/internal/store"
	"github.com/spf13/cobra"
)

type moversRow struct {
	AppID        json.RawMessage `json:"app_id"`
	Name         string          `json:"name"`
	Rank         int             `json:"rank"`
	PreviousRank *int            `json:"previous_rank"`
	Delta        *int            `json:"delta"`
	New          *bool           `json:"new"`
	Downloads    *string         `json:"downloads"`
	Revenue      *string         `json:"revenue"`
}

type moversResult struct {
	Category    string      `json:"category"`
	Country     string      `json:"country"`
	OS          string      `json:"os"`
	Chart       string      `json:"chart"`
	Date        string      `json:"date"`
	Movers      []moversRow `json:"movers"`
	NewEntrants int         `json:"new_entrants"`
	Note        string      `json:"note,omitempty"`
}

func newNovelMoversCmd(flags *rootFlags) *cobra.Command {
	var flagCountry string
	var flagOS string
	var flagDevice string
	var flagChart string
	var flagDate string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "movers <category>",
		Short: "See which apps climbed, fell, or newly appeared in a category chart since your last sync.",
		Long: "Rank the apps in a category chart by how far they moved.\n\n" +
			"delta = previous_rank - rank, so a positive delta means the app climbed.\n" +
			"Apps the API reports no previous_rank for get a null delta.\n\n" +
			"New-entrant detection compares this chart against the most recent snapshot\n" +
			"this CLI stored locally for the same os/category/country/chart. The first run\n" +
			"has nothing to compare against, so it reports \"new\": null and establishes a\n" +
			"baseline rather than claiming every app is new.\n\n" +
			"Download and revenue figures are Sensor Tower's one-significant-figure buckets,\n" +
			"printed as the bucket labels the API supplies (\"5m\", \"< $5k\").",
		Example:     "  sensortower-pp-cli movers 6015 --country US --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch category rankings (1 request) and diff them against the local rank_snapshot baseline")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<category> is required (an iOS category id such as 6015, or an Android slug such as application)"))
			}
			category := args[0]
			osName, device, err := resolveOSDevice(flagOS, flagDevice)
			if err != nil {
				return err
			}
			if err := validateChoice("chart", flagChart, "free", "grossing", "paid"); err != nil {
				return err
			}
			date := flagDate
			if date == "" {
				date = defaultChartDate()
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// The one and only request this command is allowed to make.
			resp, err := fetchCategoryRankings(ctx, c, flags, osName, category, flagCountry, device, date, flagLimit)
			if err != nil {
				return err
			}
			rows, err := resp.chart(flagChart)
			if err != nil {
				return usageErr(err)
			}

			result := moversResult{
				Category: category,
				Country:  flagCountry,
				OS:       osName,
				Chart:    flagChart,
				Date:     resp.Date,
				Movers:   make([]moversRow, 0, len(rows)),
			}

			// Read the baseline BEFORE writing this run's snapshot, otherwise the
			// rows we are about to store would become their own baseline.
			baselineIDs, baselineDate, baselineErr := readMoversBaseline(ctx, osName, category, flagCountry, flagChart, resp.Date)
			if baselineErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not read the local rank_snapshot baseline: %v\n", baselineErr)
			}
			haveBaseline := baselineErr == nil && baselineIDs != nil

			for _, r := range rows {
				out := moversRow{
					AppID:        r.AppID,
					Name:         r.Name,
					Rank:         r.Rank,
					PreviousRank: r.PreviousRank,
					Downloads:    downloadsBucket(r.HumanizedDownloads),
					Revenue:      revenueBucket(r.HumanizedRevenue),
				}
				if r.PreviousRank != nil {
					out.Delta = intPtr(*r.PreviousRank - r.Rank)
				}
				if haveBaseline {
					_, seen := baselineIDs[rawIDKey(r.AppID)]
					out.New = boolPtr(!seen)
					if !seen {
						result.NewEntrants++
					}
				}
				result.Movers = append(result.Movers, out)
			}

			// New entrants first, then the largest absolute move first. Rows with
			// no delta (the API reported no previous_rank) sort last; a missing
			// delta is not a zero move.
			sort.SliceStable(result.Movers, func(i, j int) bool {
				a, b := result.Movers[i], result.Movers[j]
				an := a.New != nil && *a.New
				bn := b.New != nil && *b.New
				if an != bn {
					return an
				}
				ad, bd := -1, -1
				if a.Delta != nil {
					ad = abs(*a.Delta)
				}
				if b.Delta != nil {
					bd = abs(*b.Delta)
				}
				if ad != bd {
					return ad > bd
				}
				return a.Rank < b.Rank
			})

			if !haveBaseline {
				result.Note = fmt.Sprintf("no prior local snapshot for %s/%s/%s/%s: \"new\" is null because a baseline is being established now, not because nothing is new. Re-run this command to get new-entrant detection.", osName, category, flagCountry, flagChart)
			} else {
				result.Note = fmt.Sprintf("\"new\" is measured against the local snapshot from %s (%d apps of the %s chart); an app deeper than that captured depth can read as new when it climbs into range.", baselineDate, len(baselineIDs), flagChart)
			}

			if err := writeMoversSnapshot(ctx, osName, category, flagCountry, flagChart, device, resp.Date, flagLimit, rows); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not store rank_snapshot rows (new-entrant detection will stay unavailable): %v\n", err)
			}

			return emitNovelResult(cmd, flags, result, "movers")
		},
	}
	cmd.Flags().StringVar(&flagCountry, "country", "US", "Two-letter country code (e.g. US, GB, JP)")
	cmd.Flags().StringVar(&flagOS, "os", "ios", "Platform to chart (one of: ios, android)")
	cmd.Flags().StringVar(&flagDevice, "device", "", "Device chart variant; defaults to iphone on iOS and phone on Android")
	cmd.Flags().StringVar(&flagChart, "chart", "free", "Which chart to diff (one of: free, grossing, paid)")
	cmd.Flags().StringVar(&flagDate, "date", "", "Chart date as YYYY-MM-DD; defaults to today (UTC). The API rejects a blank date.")
	cmd.Flags().IntVar(&flagLimit, "limit", 25, "Chart depth to fetch and snapshot")
	return cmd
}

// readMoversBaseline returns the app-id set of the most recent snapshot already
// stored for this chart on or before date, plus that snapshot's date.
//
// Returns (nil, "", nil) when no baseline exists — the caller must report
// "new": null in that case rather than treating an empty set as "everything is
// new". It runs before the current run writes, so a same-day re-run correctly
// baselines against the earlier run.
func readMoversBaseline(ctx context.Context, osName, category, country, chart, date string) (map[string]struct{}, string, error) {
	dbPath := defaultDBPath("sensortower-pp-cli")
	if !novelFileExists(dbPath) {
		return nil, "", nil
	}
	db, err := store.OpenReadOnlyContext(ctx, dbPath)
	if err != nil {
		return nil, "", err
	}
	defer db.Close()

	var baselineDate sql.NullString
	err = db.DB().QueryRowContext(ctx,
		`SELECT MAX(json_extract(data, '$.date'))
		   FROM resources
		  WHERE resource_type = ?
		    AND json_extract(data, '$.os') = ?
		    AND json_extract(data, '$.category') = ?
		    AND json_extract(data, '$.country') = ?
		    AND json_extract(data, '$.chart') = ?
		    AND json_extract(data, '$.date') <= ?`,
		rankSnapshotResource, osName, category, country, chart, date,
	).Scan(&baselineDate)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || syncHintMissingTable(err) {
			return nil, "", nil
		}
		return nil, "", err
	}
	if !baselineDate.Valid || baselineDate.String == "" {
		return nil, "", nil
	}

	rows, err := db.DB().QueryContext(ctx,
		`SELECT json_extract(data, '$.app_id_key')
		   FROM resources
		  WHERE resource_type = ?
		    AND json_extract(data, '$.os') = ?
		    AND json_extract(data, '$.category') = ?
		    AND json_extract(data, '$.country') = ?
		    AND json_extract(data, '$.chart') = ?
		    AND json_extract(data, '$.date') = ?`,
		rankSnapshotResource, osName, category, country, chart, baselineDate.String,
	)
	if err != nil {
		return nil, "", err
	}
	// Drain fully, then close, before using the connection again.
	ids := map[string]struct{}{}
	for rows.Next() {
		var key sql.NullString
		if err := rows.Scan(&key); err != nil {
			_ = rows.Close()
			return nil, "", err
		}
		if key.Valid && key.String != "" {
			ids[key.String] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, "", err
	}
	_ = rows.Close()

	if len(ids) == 0 {
		return nil, "", nil
	}
	return ids, baselineDate.String, nil
}

// writeMoversSnapshot mirrors the fetched chart into the local store so the
// next run (and `teardown` / `watch digest`) has history to work with.
func writeMoversSnapshot(ctx context.Context, osName, category, country, chart, device, date string, limit int, rows []rankingRow) error {
	if len(rows) == 0 {
		return nil
	}
	db, err := store.OpenWithContext(ctx, defaultDBPath("sensortower-pp-cli"))
	if err != nil {
		return err
	}
	defer db.Close()

	capturedAt := time.Now().UTC().Format(time.RFC3339)
	for _, r := range rows {
		key := rawIDKey(r.AppID)
		if key == "" {
			continue
		}
		snap := rankSnapshot{
			AppID:        r.AppID,
			AppIDKey:     key,
			Name:         r.Name,
			OS:           osName,
			Category:     category,
			Country:      country,
			Chart:        chart,
			Device:       device,
			Date:         date,
			Rank:         r.Rank,
			PreviousRank: r.PreviousRank,
			CapturedAt:   capturedAt,
			Limit:        limit,
		}
		if d := downloadsBucket(r.HumanizedDownloads); d != nil {
			snap.Downloads = *d
		}
		if rev := revenueBucket(r.HumanizedRevenue); rev != nil {
			snap.Revenue = *rev
		}
		payload, err := json.Marshal(snap)
		if err != nil {
			return err
		}
		if err := db.Upsert(rankSnapshotResource, rankSnapshotID(osName, category, country, chart, date, key), payload); err != nil {
			return err
		}
	}
	return nil
}
