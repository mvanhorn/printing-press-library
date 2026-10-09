// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/parks"
	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/sources"
	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/store"
)

type snapshotParkResult struct {
	Park         string `json:"park"`
	QueueTimesID int    `json:"queue_times_id"`
	NameEN       string `json:"name_en"`
	RidesSeen    int    `json:"rides_seen"`
	OpenRides    int    `json:"open_rides"`
	Inserted     int    `json:"rows_inserted"`
	SkippedDup   int    `json:"rows_skipped_duplicate"`
	Invalid      int    `json:"rows_skipped_invalid"`
	SourceNewest string `json:"source_newest_update,omitempty"`
	FetchedAt    string `json:"fetched_at,omitempty"`
	Error        string `json:"error,omitempty"`
}

type snapshotResult struct {
	DBPath  string               `json:"db_path"`
	Parks   []snapshotParkResult `json:"parks"`
	Totals  snapshotTotals       `json:"totals"`
	DedupOn string               `json:"dedupe_key"`
}

type snapshotTotals struct {
	RowsSeen      int `json:"rows_seen"`
	RowsInserted  int `json:"rows_inserted"`
	RowsDuplicate int `json:"rows_skipped_duplicate"`
	RowsInvalid   int `json:"rows_skipped_invalid"`
	ParksOK       int `json:"parks_ok"`
	ParksFailed   int `json:"parks_failed"`
}

func newNovelSnapshotCmd(flags *rootFlags) *cobra.Command {
	var parkCSV string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Record current Queue-Times waits for the Japan parks into the local history",
		Long: strings.Trim(`
Fetch the current Queue-Times waits (Powered by Queue-Times.com,
https://queue-times.com/) for the chosen parks and append them to a local
SQLite history. A row is skipped when the same ride and source update time is
already stored, so running this more often than every 5 minutes adds nothing.

'typical' and 'waits' read this history. This command does not schedule
itself; run it from your own scheduler (see the README for a cron example).

Use this command to record wait history.
Do NOT use this command to read current waits; use 'waits' instead.`, "\n"),
		Example: strings.Trim(`
  japan-theme-parks-pp-cli snapshot
  japan-theme-parks-pp-cli snapshot --park 274,275,284`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "live", "pp:happy-args": "--park=275",
			// snapshot only writes the local SQLite history, so it opts in to one
			// real live dogfood run (with --allow-destructive) instead of dry-run.
			"pp:live-happy-path": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "snapshot")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			list, err := parks.LookupList(parkCSV, parks.Keys())
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if dbPath, err = resolveDBPath(dbPath); err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtxN(cmd.Context(), flags, len(list))
			defer cancel()

			client := sources.New(flags.timeout, flags.rateLimit)
			meta := newMeta()
			res := snapshotResult{DBPath: dbPath, Parks: make([]snapshotParkResult, 0, len(list)), DedupOn: "ride_id + source last_updated"}
			rows := make([]store.WaitRow, 0)
			tally := fetchTally{meta: &meta}
			failed := []string{}
			for _, p := range list {
				pr := snapshotParkResult{Park: p.Key, QueueTimesID: p.QueueTimesID, NameEN: p.NameEN}
				f, err := client.QueueTimes(ctx, p.QueueTimesID)
				if err == nil {
					var rides []parks.Ride
					rides, err = parks.ParseQueueTimes(f.Body)
					if err == nil {
						pr.FetchedAt = rfc(f.FetchedAt)
						pr.RidesSeen = len(rides)
						meta.Sources = append(meta.Sources, queueTimesRef(f))
						for _, r := range rides {
							if r.IsOpen {
								pr.OpenRides++
							}
							if s := jst(r.LastUpdated); s > pr.SourceNewest {
								pr.SourceNewest = s
							}
							rows = append(rows, store.WaitRow{ParkID: p.QueueTimesID, RideID: r.ID, RideName: r.Name, Land: r.Land,
								IsOpen: r.IsOpen, WaitMinutes: r.WaitMinutes, SourceUpdatedAt: r.LastUpdated, FetchedAt: f.FetchedAt})
						}
					}
				}
				if err != nil {
					pr.Error = sources.Describe(err)
					tally.fail(parks.SourceQueueTimes, fmt.Errorf("%s: %w", p.NameEN, err))
					failed = append(failed, p.Key)
				} else {
					tally.succeed()
				}
				res.Parks = append(res.Parks, pr)
			}
			if err := tally.allFailed("no park could be fetched from Queue-Times"); err != nil {
				return err
			}

			// The fetch deadline must not discard rows already collected:
			// write with the command context, not the fetch-bounded one.
			dbCtx := cmd.Context()
			db, err := store.OpenWithContext(dbCtx, dbPath)
			if err != nil {
				return fmt.Errorf("opening history database: %w", err)
			}
			counts, err := db.InsertWaitRows(dbCtx, rows)
			_ = db.Close()
			if err != nil {
				return err
			}
			res.Totals = snapshotTotals{RowsSeen: len(rows), ParksOK: tally.ok, ParksFailed: len(failed)}
			for i := range res.Parks {
				c := counts[res.Parks[i].QueueTimesID]
				res.Parks[i].Inserted, res.Parks[i].SkippedDup, res.Parks[i].Invalid = c.Inserted, c.Skipped, c.Invalid
				res.Totals.RowsInserted += c.Inserted
				res.Totals.RowsDuplicate += c.Skipped
				res.Totals.RowsInvalid += c.Invalid
			}
			if len(failed) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d parks failed (%s); stored rows for the remaining %d\n", len(failed), len(res.Parks), strings.Join(failed, ", "), tally.ok)
			}

			out := struct {
				Meta    respMeta       `json:"meta"`
				Results snapshotResult `json:"results"`
			}{meta, res}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			w := cmd.OutOrStdout()
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "PARK\tRIDES\tOPEN\tNEWEST SOURCE UPDATE\tERROR")
			for _, pr := range res.Parks {
				fmt.Fprintf(tw, "%s\t%d\t%d\t%s\t%s\n", pr.NameEN, pr.RidesSeen, pr.OpenRides, pr.SourceNewest, cliutil.ScrubTerminal(pr.Error))
			}
			_ = tw.Flush()
			fmt.Fprintf(w, "stored %d new rows, skipped %d duplicates in %s\n", res.Totals.RowsInserted, res.Totals.RowsDuplicate, dbPath)
			printAttribution(w, meta)
			return nil
		},
	}
	cmd.Flags().StringVar(&parkCSV, "park", "", "Comma-separated parks to record (keys, ids or names; default: all five)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Snapshot history database path (default: the CLI data directory)")
	return cmd
}
