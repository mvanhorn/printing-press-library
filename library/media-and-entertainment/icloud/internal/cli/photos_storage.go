// Copyright 2026 Matias Sanchez Moises and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newStorageCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "storage",
		Short: "Storage breakdown by media type and year",
		Long: `Show how your Photos library storage is distributed across media types (photos
vs videos) and across years.

Sizes are based on original file sizes and include both locally stored and
iCloud-only assets — useful for understanding the total footprint before
deciding what to remove.`,
		Example: `  icloud-pp-cli photos storage
  icloud-pp-cli photos storage --json
  icloud-pp-cli photos storage --agent | jq '.by_type'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openPhotosDB(f.libraryPath)
			if err != nil {
				return err
			}
			defer db.Close()

			byType, err := queryStorageByType(db)
			if err != nil {
				return fmt.Errorf("type query failed: %w", err)
			}
			byYear, err := queryStorageByYear(db)
			if err != nil {
				return fmt.Errorf("year query failed: %w", err)
			}

			if f.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printStorageJSON(cmd, byType, byYear)
			}
			return printStorageTable(cmd, f, byType, byYear)
		},
	}

	return cmd
}

type storageRowJSON struct {
	Label     string  `json:"label"`
	Count     int64   `json:"count"`
	SizeGB    float64 `json:"size_gb"`
	SizeBytes int64   `json:"size_bytes,omitempty"`
}

type storageJSON struct {
	ByType []storageRowJSON `json:"by_type"`
	ByYear []storageRowJSON `json:"by_year"`
}

func printStorageJSON(cmd *cobra.Command, byType, byYear []StorageRow) error {
	out := storageJSON{
		ByType: toStorageJSON(byType),
		ByYear: toStorageJSON(byYear),
	}
	return printJSON(cmd.OutOrStdout(), out)
}

func toStorageJSON(rows []StorageRow) []storageRowJSON {
	out := make([]storageRowJSON, len(rows))
	for i, r := range rows {
		out[i] = storageRowJSON{
			Label:     r.Label,
			Count:     r.Count,
			SizeGB:    roundTo2(r.SizeGB()),
			SizeBytes: r.SizeBytes,
		}
	}
	return out
}

func printStorageTable(cmd *cobra.Command, f *rootFlags, byType, byYear []StorageRow) error {
	out := cmd.OutOrStdout()

	fmt.Fprintln(out, bold(f, out, "By media type"))
	w := newTabWriter(out)
	fmt.Fprintf(w, "  %s\t%s\t%s\n", bold(f, out, "Type"), bold(f, out, "Items"), bold(f, out, "Size"))
	for _, r := range byType {
		fmt.Fprintf(w, "  %s\t%d\t%s\n", r.Label, r.Count, formatSizeBytes(f, out, r.SizeBytes))
	}
	if err := w.Flush(); err != nil {
		return err
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, bold(f, out, "By year"))
	w2 := newTabWriter(out)
	fmt.Fprintf(w2, "  %s\t%s\t%s\n", bold(f, out, "Year"), bold(f, out, "Items"), bold(f, out, "Size"))
	for _, r := range byYear {
		fmt.Fprintf(w2, "  %s\t%d\t%s\n", r.Label, r.Count, formatSizeBytes(f, out, r.SizeBytes))
	}
	return w2.Flush()
}
