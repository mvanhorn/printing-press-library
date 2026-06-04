// Copyright 2026 Matias Sanchez Moises and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVideosCmd(f *rootFlags) *cobra.Command {
	var limit, year, month int

	cmd := &cobra.Command{
		Use:   "videos",
		Short: "List your largest videos sorted by file size",
		Long: `List videos in your Photos library sorted by original file size, largest first.

Use this to quickly identify which videos are consuming the most storage.
Sizes reflect the original file — useful for deciding what to delete from iCloud.`,
		Example: `  # Top 20 largest videos
  icloud-pp-cli photos videos --limit 20

  # Videos from 2023 as JSON
  icloud-pp-cli photos videos --year 2023 --json

  # Pipe to agent (auto-enables JSON + compact)
  icloud-pp-cli photos videos | jq '.[].size_gb'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if month > 0 && year == 0 {
				return usageErr(fmt.Errorf("--month requires --year"))
			}
			db, err := openPhotosDB(f.libraryPath)
			if err != nil {
				return err
			}
			defer db.Close()

			videos, err := queryLargestVideos(db, limit, year, month)
			if err != nil {
				return fmt.Errorf("query failed: %w", err)
			}
			if len(videos) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No videos found.")
				return nil
			}

			if f.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printVideosJSON(cmd, f, videos)
			}
			return printVideosTable(cmd, f, videos)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of videos to return (0 = all)")
	cmd.Flags().IntVar(&year, "year", 0, "Filter to a specific year (e.g. 2023)")
	cmd.Flags().IntVar(&month, "month", 0, "Filter to a specific month 1-12 (requires --year)")

	return cmd
}

type videoJSON struct {
	Rank     int     `json:"rank"`
	Filename string  `json:"filename"`
	SizeGB   float64 `json:"size_gb"`
	SizeMB   float64 `json:"size_mb"`
	Date     string  `json:"date"`
	UUID     string  `json:"uuid,omitempty"`
}

func printVideosJSON(cmd *cobra.Command, f *rootFlags, videos []Asset) error {
	out := make([]videoJSON, len(videos))
	for i, v := range videos {
		row := videoJSON{
			Rank:     i + 1,
			Filename: v.Filename,
			SizeGB:   roundTo2(v.SizeGB()),
			SizeMB:   roundTo2(v.SizeMB()),
			Date:     v.Date.Format("2006-01-02"),
		}
		if !f.compact {
			row.UUID = v.UUID
		}
		out[i] = row
	}
	return printJSON(cmd.OutOrStdout(), out)
}

func printVideosTable(cmd *cobra.Command, f *rootFlags, videos []Asset) error {
	out := cmd.OutOrStdout()
	w := newTabWriter(out)
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
		bold(f, out, "#"),
		bold(f, out, "Size"),
		bold(f, out, "Date"),
		bold(f, out, "Filename"),
		bold(f, out, "UUID"),
	)
	for i, v := range videos {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n",
			i+1,
			formatSize(f, out, v.SizeGB()),
			v.Date.Format("2006-01-02"),
			v.Filename,
			v.UUID,
		)
	}
	return w.Flush()
}

func roundTo2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}
