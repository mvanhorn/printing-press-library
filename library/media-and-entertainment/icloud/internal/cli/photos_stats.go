// Copyright 2026 Matias Sanchez Moises and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newStatsCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Quick summary: total items and total library size",
		Example: `  icloud-pp-cli photos stats
  icloud-pp-cli photos stats --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openPhotosDB(f.libraryPath)
			if err != nil {
				return err
			}
			defer db.Close()

			count, sizeBytes, err := queryTotals(db)
			if err != nil {
				return fmt.Errorf("stats query failed: %w", err)
			}

			byType, err := queryStorageByType(db)
			if err != nil {
				return fmt.Errorf("type query failed: %w", err)
			}

			if f.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printStatsJSON(cmd, count, sizeBytes, byType)
			}
			return printStatsTable(cmd, f, count, sizeBytes, byType)
		},
	}

	return cmd
}

type statsJSON struct {
	TotalItems     int64            `json:"total_items"`
	TotalSizeGB    float64          `json:"total_size_gb"`
	TotalSizeBytes int64            `json:"total_size_bytes"`
	ByType         []storageRowJSON `json:"by_type"`
}

func printStatsJSON(cmd *cobra.Command, count, sizeBytes int64, byType []StorageRow) error {
	return printJSON(cmd.OutOrStdout(), statsJSON{
		TotalItems:     count,
		TotalSizeGB:    roundTo2(float64(sizeBytes) / (1 << 30)),
		TotalSizeBytes: sizeBytes,
		ByType:         toStorageJSON(byType),
	})
}

func printStatsTable(cmd *cobra.Command, f *rootFlags, count, sizeBytes int64, byType []StorageRow) error {
	out := cmd.OutOrStdout()
	gb := float64(sizeBytes) / (1 << 30)

	fmt.Fprintf(out, "%s  %d items · %s\n",
		bold(f, out, "Photos library"),
		count,
		formatSize(f, out, gb),
	)
	fmt.Fprintln(out)

	w := newTabWriter(out)
	for _, r := range byType {
		fmt.Fprintf(w, "  %s\t%d items\t%s\n", r.Label, r.Count, formatSizeBytes(f, out, r.SizeBytes))
	}
	return w.Flush()
}
