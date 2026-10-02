// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/haneda-airport/internal/haneda"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelSnapshotSaveCmd(flags *rootFlags) *cobra.Command {
	q := haneda.Query{}
	var file string
	var overwrite bool
	cmd := &cobra.Command{Use: "save", Short: "Save a complete, timestamped source scope for later offline queries", Long: "Save a complete board for one date/kind/direction. The default is an atomic new file in the CLI cache; --file chooses an explicit destination. Existing files are preserved unless --overwrite is supplied. No bookings or accounts are changed.", Annotations: map[string]string{"pp:data-source": "live", "mcp:read-only": "false", "pp:fixture-local-write": "true", "mcp:write-flags": "file", "pp:happy-args": "--kind=international;--direction=departure"}, Example: "  haneda-airport-pp-cli snapshot save --kind international --direction departure --agent"}
	q.Limit, q.MaxScan = 20, 5000
	cmd.Flags().StringVar(&q.Kind, "kind", "international", "Flight source: domestic, international, or all")
	cmd.Flags().StringVar(&q.Direction, "direction", "departure", "Complete Haneda scope: departure, arrival, or both")
	cmd.Flags().StringVar(&q.Date, "date", "", "Exact service date YYYY-MM-DD; default today JST")
	cmd.Flags().IntVar(&q.MaxScan, "max-scan-records", 5000, "Maximum examined source records; incomplete snapshots are refused")
	cmd.Flags().StringVar(&file, "file", "", "Destination snapshot file; default creates a new file in the CLI cache")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Explicitly replace an existing regular snapshot file")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "snapshot save")
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("snapshot save takes --file and source flags"))
		}
		if err := checkHanedaLive(flags); err != nil {
			return err
		}
		prepareHanedaQuery(&q, true)
		q.IncludeFacilities = true
		if q.Offset != 0 {
			return usageErr(fmt.Errorf("snapshot save captures a complete scope; --offset must be 0"))
		}
		if err := haneda.ValidateQuery(q, time.Now(), true); err != nil {
			return usageErr(err)
		}
		ctx, cancel := hanedaContext(cmd, flags)
		defer cancel()
		c, err := newHanedaClient(flags)
		if err != nil {
			return err
		}
		r, err := c.FetchBoard(ctx, q, time.Now(), "")
		if err != nil {
			return hanedaAPIError(cmd, flags, err)
		}
		if !r.Complete {
			return usageErr(fmt.Errorf("snapshot not saved because the scan cap omitted source rows; raise --max-scan-records"))
		}
		r.TotalMatches = len(r.Flights)
		r.Limit = len(r.Flights)
		now := time.Now()
		if file == "" {
			dir, err := hanedaSnapshotDir()
			if err != nil {
				return configErr(err)
			}
			file = filepath.Join(dir, "hnd-snapshot-"+now.UTC().Format("20060102T150405.000000000Z")+"_"+q.Kind+"_"+q.Direction+"_"+q.Date+".json")
		}
		file, err = filepath.Abs(file)
		if err != nil {
			return usageErr(err)
		}
		s := haneda.Snapshot{Schema: haneda.SnapshotSchema, SavedAt: now.In(haneda.JST).Format(time.RFC3339), Board: r}
		if err = haneda.SaveSnapshot(file, s, overwrite); err != nil {
			return configErr(fmt.Errorf("save snapshot: %w", err))
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"snapshot_file": file, "schema": s.Schema, "saved_at": s.SavedAt, "flight_groups": len(r.Flights), "coverage": r.Coverage, "sources": r.Sources, "budget": r.Budget}, flags)
	}
	return cmd
}
