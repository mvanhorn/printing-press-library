// pp:data-source local

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelResearchCitationsCmd(flags *rootFlags) *cobra.Command {
	var snapshotID, format string
	cmd := &cobra.Command{
		Use:         "citations",
		Short:       "Export source-linked Markdown or JSON citations from a saved research run.",
		Example:     "  keenable-pp-cli research citations --snapshot latest --format markdown",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "research citations")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			s, err := openResearchStore(ctx)
			if err != nil {
				return fmt.Errorf("opening local research store: %w", err)
			}
			defer s.Close()
			snap, err := loadResearchSnapshot(s, snapshotID)
			if err != nil {
				return err
			}
			results, err := loadSnapshotResults(s, snap.ID)
			if err != nil {
				return err
			}
			pages, err := loadSnapshotPages(s, snap.ID)
			if err != nil {
				return err
			}
			if strings.EqualFold(format, "json") || flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"snapshot": snap, "results": results, "pages": pages}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "# Keenable research citations\n\nSnapshot: `%s`\nQuery: %s\n\n", snap.ID, snap.Query)
			for i, result := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%d. [%s](%s)", i+1, result.Title, result.URL)
				if result.PublishedAt != "" {
					fmt.Fprintf(cmd.OutOrStdout(), " — published %s", result.PublishedAt)
				}
				if result.AcquiredAt != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "; acquired %s", result.AcquiredAt)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			for _, page := range pages {
				fmt.Fprintf(cmd.OutOrStdout(), "\n## %s\n\n- URL: %s\n- Content hash: `%s`\n", page.Title, page.URL, page.ContentHash)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&snapshotID, "snapshot", "latest", "Snapshot ID or latest")
	cmd.Flags().StringVar(&format, "format", "markdown", "Output format: markdown or json")
	return cmd
}
