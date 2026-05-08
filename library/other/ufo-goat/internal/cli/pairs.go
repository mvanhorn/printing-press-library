// Pairs command — show video-PDF pairings.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/ufo-goat/internal/store"

	"github.com/spf13/cobra"
)

func newPairsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "pairs",
		Short: "Show video-PDF pairings for cross-referencing",
		Long: `Display video-PDF pairings so researchers can locate the document
that accompanies a video and vice versa.`,
		Example: `  # Show all pairings
  ufo-goat-pp-cli pairs

  # JSON output
  ufo-goat-pp-cli pairs --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("ufo-goat-pp-cli")
			}

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'ufo-goat-pp-cli sync' first.", err)
			}
			defer db.Close()
			_ = db.EnsureUFOSchema()

			count, _ := db.GetFileCount()
			if count == 0 {
				return fmt.Errorf("no files in local store. Run 'ufo-goat-pp-cli sync' first")
			}

			pairs, err := db.GetPairs()
			if err != nil {
				return err
			}

			if len(pairs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No video-PDF pairings found.")
				return nil
			}

			// JSON output
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				data, _ := json.Marshal(pairs)
				filtered := json.RawMessage(data)
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				}
				return printOutput(cmd.OutOrStdout(), filtered, true)
			}

			// Table output
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%d video-PDF pairings found\n\n", len(pairs))

			tw := newTabWriter(w)
			fmt.Fprintln(tw, strings.Join([]string{
				bold("VIDEO"), bold("PDF"), bold("AGENCY"),
			}, "\t"))

			for _, p := range pairs {
				videoTitle := p.VideoTitle
				if videoTitle == "" {
					videoTitle = p.VideoID
				}
				pdfTitle := p.PDFTitle
				if pdfTitle == "" {
					pdfTitle = p.PDFID
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\n",
					truncate(videoTitle, 40),
					truncate(pdfTitle, 40),
					p.Agency,
				)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// Ensure store is imported
var _ = store.FilePair{}
