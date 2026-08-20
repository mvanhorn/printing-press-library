// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0.

package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/art-goat/internal/store"

	"github.com/spf13/cobra"
)

func newBrowseCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var sourceFilter string
	var medium string
	var region string
	var limit int
	var offset int

	cmd := &cobra.Command{
		Use:   "browse",
		Short: "Paginate the unified works table",
		Long: `Paginate over the local works corpus with optional filters. Ordered
deterministically by id so --limit / --offset give a stable cursor across runs.

Filters are conjunctive (AND): --source matches exactly; --medium and
--region are case-insensitive substring matches over the works columns of
the same name.`,
		Example: `  # First 20 works
  art-goat-pp-cli browse

  # Next page
  art-goat-pp-cli browse --offset 20

  # Only AIC, only paintings
  art-goat-pp-cli browse --source aic --medium painting

  # Structured envelope
  art-goat-pp-cli browse --json --limit 5`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() && !flags.asJSON {
				return emitBrowseVerifyEnvelope(cmd)
			}

			if limit <= 0 {
				limit = 20
			}
			if limit > 200 {
				limit = 200
			}
			if offset < 0 {
				offset = 0
			}

			if dbPath == "" {
				dbPath = defaultDBPath("art-goat-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			if err := db.EnsureArtGoatTables(cmd.Context()); err != nil {
				return err
			}

			works, err := db.BrowseWorks(cmd.Context(), store.BrowseFilter{
				Source: sourceFilter,
				Medium: medium,
				Region: region,
				Limit:  limit,
				Offset: offset,
			})
			if err != nil {
				return err
			}

			envelopes := make([]map[string]any, 0, len(works))
			for _, w := range works {
				envelopes = append(envelopes, map[string]any{
					"id":         w.ID,
					"source":     w.Source,
					"title":      w.Title,
					"creator":    w.Creator,
					"date":       w.DateText,
					"medium":     w.Medium,
					"region":     w.CultureRegion,
					"image_url":  w.ImageURL,
					"source_url": w.SourceURL,
				})
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), envelopes, flags)
			}
			renderBrowse(cmd, works, limit, offset)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/art-goat-pp-cli/data.db)")
	cmd.Flags().StringVar(&sourceFilter, "source", "", "Restrict to one source (e.g. aic, apod)")
	cmd.Flags().StringVar(&medium, "medium", "", "Substring match on medium (case-insensitive)")
	cmd.Flags().StringVar(&region, "region", "", "Substring match on culture_region (case-insensitive)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Page size for the unified works browse (max 200)")
	cmd.Flags().IntVar(&offset, "offset", 0, "Pagination offset into the works table for resumable scans")
	if cliutil.IsStrictFlagsEnv() {
		_ = cmd.MarkFlagRequired("limit")
	}
	return cmd
}

func renderBrowse(cmd *cobra.Command, works []store.Work, limit, offset int) {
	out := cmd.OutOrStdout()
	if len(works) == 0 {
		fmt.Fprintln(out, "No works match these filters.")
		fmt.Fprintln(out, "Run `art-goat-pp-cli sources sync` to populate the local corpus, or loosen filters.")
		return
	}
	tw := tabwriter.NewWriter(out, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSource\tTitle\tCreator\tDate")
	for _, w := range works {
		title := coalesce(w.Title, "(untitled)")
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", w.ID, w.Source, truncateForPreview(title, 60), truncateForPreview(w.Creator, 40), w.DateText)
	}
	_ = tw.Flush()
	fmt.Fprintf(out, "\nShowing %d works (offset %d, limit %d). Next page: --offset %d\n", len(works), offset, limit, offset+limit)
}

func emitBrowseVerifyEnvelope(cmd *cobra.Command) error {
	envelope := map[string]any{
		"command":                 "browse",
		"verify_noop":             true,
		"success":                 true,
		"__pp_verify_synthetic__": true,
		"reason":                  "verify_short_circuit",
		"note":                    "browse renders a table to terminal by default; PRINTING_PRESS_VERIFY=1 short-circuits rendering. Pass --json to get the data envelope.",
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}
