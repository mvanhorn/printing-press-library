// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/haneda-airport/internal/haneda"
	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelSnapshotSearchCmd(flags *rootFlags) *cobra.Command {
	q := haneda.Query{}
	var file string
	var maxAge time.Duration
	cmd := &cobra.Command{Use: "search", Short: "Search a saved observation without network requests", Long: "Read --file or the latest snapshot in the CLI cache. Its original coverage, source timestamps and age remain visible. With no saved snapshot, return an explicit empty local state. Explicit kind/direction/date requests must be covered by the saved scope. An explicit --date must match the saved request date and narrows to that service day.", Annotations: hanedaAnnotations("local"), Example: "  haneda-airport-pp-cli snapshot search --flight NH849 --limit 5 --agent"}
	hanedaQueryFlags(cmd, &q, true)
	cmd.Flags().StringVar(&file, "file", "", "Input snapshot file; default reads the latest cached observation")
	cmd.Flags().DurationVar(&maxAge, "max-age", 5*time.Minute, "Saved observation age threshold for stale_snapshot (must be positive)")
	cmd.Flags().BoolVar(&q.IncludeFacilities, "include-facilities", false, "Include the saved provider facility map/detail links")
	cmd.Flags().StringVar(&q.Status, "status", "", "Source category/text or unknown; comma-separated status filters")
	cmd.Flags().StringVar(&q.Terminal, "terminal", "", "Published terminal filter: T1, T2, or T3")
	cmd.Annotations["pp:happy-args"] = "--limit=5"
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "snapshot search")
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("snapshot search accepts --file and filters"))
		}
		if flags.dataSource == "live" {
			return usageErr(fmt.Errorf("snapshot search is offline; use flights search for live source data"))
		}
		prepareHanedaQuery(&q, false)
		if maxAge <= 0 {
			return usageErr(fmt.Errorf("snapshot --max-age must be positive"))
		}
		if err := haneda.ValidateQuery(q, time.Now(), false); err != nil {
			return usageErr(err)
		}
		if file == "" {
			paths, err := hanedaLatestPaths()
			if err != nil {
				return configErr(err)
			}
			if len(paths) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"flights": []haneda.Flight{}, "total_matches": 0, "snapshot_file": nil, "budget": haneda.Budget{}, "notes": []string{"No saved snapshot. Run snapshot save to create an explicitly timestamped observation, or pass --file."}}, flags)
			}
			file = paths[0]
		}
		s, err := haneda.LoadSnapshot(file)
		if err != nil {
			return configErr(fmt.Errorf("read snapshot: %w", err))
		}
		if !cmd.Flags().Changed("kind") {
			q.Kind = s.Board.Coverage.Kind
		}
		if !cmd.Flags().Changed("direction") {
			q.Direction = s.Board.Coverage.Direction
		}
		if q.Date == "" {
			q.Date = s.Board.Coverage.RequestedDate
		} else {
			q.ServiceDayOnly = true
		}
		if err := haneda.ValidateSnapshotQuery(s, q); err != nil {
			return usageErr(err)
		}
		r := s.Board
		r.Budget = haneda.Budget{}
		r.MaxScanRecords = q.MaxScan
		r.ScannedRecords = len(r.Flights)
		if len(r.Flights) > q.MaxScan {
			r.Flights = r.Flights[:q.MaxScan]
			r.ScannedRecords = q.MaxScan
			r.ScanCapHit = true
			r.Complete = false
		}
		r = haneda.FilterBoard(r, q, time.Now())
		r.Stale = r.SnapshotAgeSeconds > int64(maxAge.Seconds())
		r.Notes = append(r.Notes, "This is a saved historical observation with zero network requests; its source row freshness remains unpublished.")
		if r.Stale {
			fmt.Fprintln(cmd.ErrOrStderr(), "warning: saved flight observation is stale; consult a new live board for terminal planning")
		}
		return printJSONFiltered(cmd.OutOrStdout(), r, flags)
	}
	return cmd
}
