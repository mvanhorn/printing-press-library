// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored. `export` bulk-downloads images from designs matching a query
// into a directory, with templated filenames. It reads designs live and never
// generates, so it costs no quota.
//
// pp:data-source live

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/artistly/internal/cliutil"

	"github.com/spf13/cobra"
)

func newNovelExportCmd(flags *rootFlags) *cobra.Command {
	var query string
	var to string
	var nameTemplate string
	var folderID int
	var since string
	var limit int

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Bulk-download images from designs matching a query into a directory (no generation).",
		Long: trimLong(`
Download the images of designs you already generated, selected by query, into a
directory with templated filenames. This never generates anything and costs no
quota.

Filter with --query (prompt substring), --folder, and --since (e.g. 7d, 24h).
--name-template supports {id}, {uuid}, {prompt}, and {n}.

To generate new images and download them, use 'generate --wait --download' or
'batch --wait --download' instead.`),
		Example:     "  artistly-pp-cli export --query \"coloring book\" --to ./out --name-template '{prompt}-{id}'",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would export matching designs to %q\n", to)
				return nil
			}
			if to == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--to <dir> is required"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			designs, err := fetchPersonalDesigns(ctx, c)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var cutoff time.Time
			if since != "" {
				d, perr := cliutil.ParseDurationLoose(since)
				if perr != nil {
					return usageErr(fmt.Errorf("--since: %w", perr))
				}
				cutoff = time.Now().Add(-d)
			}

			type exported struct {
				ID    int      `json:"id"`
				UUID  string   `json:"uuid"`
				Files []string `json:"files"`
			}
			results := make([]exported, 0)
			totalFiles := 0
			matched := 0
			for _, d := range designs {
				if !designMatches(d, query, folderID, cutoff) {
					continue
				}
				if len(designImageURLs(d)) == 0 {
					continue
				}
				matched++
				files, derr := downloadDesign(ctx, d, to, nameTemplate)
				if derr != nil {
					return derr
				}
				totalFiles += len(files)
				results = append(results, exported{ID: d.ID, UUID: d.UUID, Files: files})
				if limit > 0 && matched >= limit {
					break
				}
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"matched":     matched,
					"files":       totalFiles,
					"destination": to,
					"designs":     results,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Exported %d image(s) from %d design(s) to %s\n", totalFiles, matched, to)
			return nil
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Only export designs whose prompt contains this text")
	cmd.Flags().StringVar(&to, "to", "", "Destination directory (required)")
	cmd.Flags().StringVar(&nameTemplate, "name-template", "{id}", "Filename template: {id} {uuid} {prompt} {n}")
	cmd.Flags().IntVar(&folderID, "folder", 0, "Only export designs in this folder id")
	cmd.Flags().StringVar(&since, "since", "", "Only export designs created within this window (e.g. 7d, 24h)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max designs to export (0 = no limit)")
	return cmd
}

func designMatches(d Design, query string, folderID int, cutoff time.Time) bool {
	if query != "" && !strings.Contains(strings.ToLower(d.PositivePrompt), strings.ToLower(query)) {
		return false
	}
	if folderID != 0 {
		if d.FolderID == nil || *d.FolderID != folderID {
			return false
		}
	}
	if !cutoff.IsZero() && d.CreatedAt != "" {
		if t, ok := parseDesignTime(d.CreatedAt); ok && t.Before(cutoff) {
			return false
		}
	}
	return true
}
