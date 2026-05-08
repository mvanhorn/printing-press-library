// Custom files get command that queries the local SQLite store.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/ufo-goat/internal/store"

	"github.com/spf13/cobra"
)

func newUFOFilesGetCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "get <id-or-title>",
		Short: "Get details of a specific declassified file",
		Long: `Look up a single UAP file by ID (or partial title match) and display
all available details including description, download URL, and pairings.`,
		Example: `  # Get by ID
  ufo-goat-pp-cli files get abc123def456

  # Get by partial title
  ufo-goat-pp-cli files get "Apollo"`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
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

			f, err := db.GetUFOFileByID(args[0])
			if err != nil {
				return err
			}

			// JSON output
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				data, _ := json.Marshal(f)
				filtered := json.RawMessage(data)
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				return printOutput(cmd.OutOrStdout(), filtered, true)
			}

			// Human-friendly detail view
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s\n", bold(f.Title))
			fmt.Fprintf(w, "%s\n\n", strings.Repeat("─", min(len(f.Title), 60)))

			printField(w, "ID", f.ID)
			printField(w, "Type", f.Type)
			printField(w, "Agency", f.Agency)
			printField(w, "Release Date", f.ReleaseDate)
			printField(w, "Incident Date", f.IncidentDate)
			printField(w, "Location", f.IncidentLocation)
			printField(w, "Redacted", fmt.Sprintf("%v", f.Redacted))

			if f.Description != "" {
				fmt.Fprintf(w, "\n%s\n  %s\n", bold("Description:"), f.Description)
			}

			if f.DownloadURL != "" {
				fmt.Fprintf(w, "\n%s %s\n", bold("Download:"), f.DownloadURL)
			}

			if f.DVIDSVideoID != "" {
				printField(w, "DVIDS Video ID", f.DVIDSVideoID)
			}
			if f.VideoTitle != "" {
				printField(w, "Video Title", f.VideoTitle)
			}
			if f.VideoPairing != "" {
				printField(w, "Video Pairing", f.VideoPairing)
			}
			if f.PDFPairing != "" {
				printField(w, "PDF Pairing", f.PDFPairing)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}

func printField(w io.Writer, label, value string) {
	if value != "" && value != "false" {
		fmt.Fprintf(w, "  %-16s %s\n", label+":", value)
	}
}

// min is built-in in Go 1.21+ (go.mod says 1.23)
