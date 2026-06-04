// Copyright 2026 Matias Sanchez Moises and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newTopCmd(f *rootFlags) *cobra.Command {
	var limit int
	var mediaType string

	cmd := &cobra.Command{
		Use:   "top",
		Short: "Top heaviest files across all media types",
		Long: `List the largest files in your Photos library by original file size,
across all media types (photos and videos combined).

Use --type to narrow to a specific kind.`,
		Example: `  # Top 25 heaviest files (default)
  icloud-pp-cli photos top

  # Top 10, videos only
  icloud-pp-cli photos top --limit 10 --type video

  # All files over 1 GB (pipe + jq)
  icloud-pp-cli photos top --limit 0 --json | jq '[.[] | select(.size_gb > 1)]'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if mediaType != "all" && mediaType != "photo" && mediaType != "video" {
				return usageErr(fmt.Errorf("--type must be one of: all, photo, video"))
			}

			db, err := openPhotosDB(f.libraryPath)
			if err != nil {
				return err
			}
			defer db.Close()

			assets, err := queryTopFiles(db, limit, mediaType)
			if err != nil {
				return fmt.Errorf("query failed: %w", err)
			}
			if len(assets) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No files found.")
				return nil
			}

			if f.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printTopJSON(cmd, f, assets)
			}
			return printTopTable(cmd, f, assets)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum number of files to return (0 = all)")
	cmd.Flags().StringVar(&mediaType, "type", "all", "Media type filter: all, photo, video")

	return cmd
}

type topEntryJSON struct {
	Rank     int     `json:"rank"`
	Type     string  `json:"type"`
	Filename string  `json:"filename"`
	SizeGB   float64 `json:"size_gb"`
	SizeMB   float64 `json:"size_mb"`
	Date     string  `json:"date"`
	UUID     string  `json:"uuid,omitempty"`
}

func printTopJSON(cmd *cobra.Command, f *rootFlags, assets []Asset) error {
	out := make([]topEntryJSON, len(assets))
	for i, a := range assets {
		row := topEntryJSON{
			Rank:     i + 1,
			Type:     a.TypeLabel(),
			Filename: a.Filename,
			SizeGB:   roundTo2(a.SizeGB()),
			SizeMB:   roundTo2(a.SizeMB()),
			Date:     a.Date.Format("2006-01-02"),
		}
		if !f.compact {
			row.UUID = a.UUID
		}
		out[i] = row
	}
	return printJSON(cmd.OutOrStdout(), out)
}

func printTopTable(cmd *cobra.Command, f *rootFlags, assets []Asset) error {
	out := cmd.OutOrStdout()
	w := newTabWriter(out)
	fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
		bold(f, out, "#"),
		bold(f, out, "Type"),
		bold(f, out, "Size"),
		bold(f, out, "Date"),
		bold(f, out, "Filename"),
		bold(f, out, "UUID"),
	)
	for i, a := range assets {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n",
			i+1,
			a.TypeLabel(),
			formatSize(f, out, a.SizeGB()),
			a.Date.Format("2006-01-02"),
			a.Filename,
			a.UUID,
		)
	}
	return w.Flush()
}
