// Custom files list command that queries the local SQLite store.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/ufo-goat/internal/store"

	"github.com/spf13/cobra"
)

func newUFOFilesListCmd(flags *rootFlags) *cobra.Command {
	var flagAgency string
	var flagType string
	var flagLocation string
	var flagAfter string
	var flagBefore string
	var flagRedacted bool
	var flagRedactedSet bool
	var flagRelease int
	var flagLimit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all declassified UAP files from local store",
		Long: `List UAP files from the local SQLite store. Requires 'ufo-goat-pp-cli sync' first.
Supports filtering by agency, type, location, date range, and redaction status.`,
		Example: `  # List all files
  ufo-goat-pp-cli files list

  # Filter by agency
  ufo-goat-pp-cli files list --agency FBI

  # Filter by type
  ufo-goat-pp-cli files list --type PDF

  # Filter by location
  ufo-goat-pp-cli files list --location "New Mexico"

  # Filter by date range
  ufo-goat-pp-cli files list --after 1947-01-01 --before 1950-12-31

  # Show only redacted files
  ufo-goat-pp-cli files list --redacted

  # Show only files from release tranche 1
  ufo-goat-pp-cli files list --release 1`,
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

			// Ensure schema
			_ = db.EnsureUFOSchema()

			// Check if we have data
			count, _ := db.GetFileCount()
			if count == 0 {
				return fmt.Errorf("no files in local store. Run 'ufo-goat-pp-cli sync' first")
			}

			filter := store.FileFilter{
				Agency:       flagAgency,
				Type:         flagType,
				Location:     flagLocation,
				After:        flagAfter,
				Before:       flagBefore,
				ReleaseBatch: flagRelease,
				Limit:        flagLimit,
			}

			if cmd.Flags().Changed("redacted") {
				flagRedactedSet = true
			}
			if flagRedactedSet {
				filter.Redacted = &flagRedacted
			}

			files, err := db.ListUFOFiles(filter)
			if err != nil {
				return fmt.Errorf("listing files: %w", err)
			}

			if len(files) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No files match the given filters.")
				return nil
			}

			// CSV output (check before JSON since --csv should win over piped output)
			if flags.csv {
				data, _ := json.Marshal(files)
				return printCSV(cmd.OutOrStdout(), json.RawMessage(data))
			}
			if flags.plain {
				data, _ := json.Marshal(files)
				return printPlain(cmd.OutOrStdout(), json.RawMessage(data))
			}

			// JSON output
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				data, _ := json.Marshal(files)
				filtered := json.RawMessage(data)
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				return printOutput(cmd.OutOrStdout(), filtered, true)
			}

			// Table output
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, strings.Join([]string{
				bold("ID"), bold("TITLE"), bold("TYPE"), bold("AGENCY"), bold("DATE"), bold("LOCATION"),
			}, "\t"))

			for _, f := range files {
				date := f.IncidentDate
				if date == "" {
					date = "-"
				}
				loc := f.IncidentLocation
				if loc == "" {
					loc = "-"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
					f.ID[:8],
					truncate(f.Title, 50),
					f.Type,
					f.Agency,
					truncate(date, 12),
					truncate(loc, 25),
				)
			}
			tw.Flush()

			if len(files) >= 25 {
				fmt.Fprintf(os.Stderr, "\nShowing %d files. Use --agency, --type, or --location to narrow results.\n", len(files))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagAgency, "agency", "", "Filter results by originating agency, one of DoD, FBI, NASA, or State")
	cmd.Flags().StringVar(&flagType, "type", "", "Filter results by file type, one of PDF, VID, or IMG")
	cmd.Flags().StringVar(&flagLocation, "location", "", "Filter by incident location, matched as a case-insensitive substring")
	cmd.Flags().StringVar(&flagAfter, "after", "", "Show files with incident dates after this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagBefore, "before", "", "Show files with incident dates before this date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&flagRedacted, "redacted", false, "Filter by redaction status")
	cmd.Flags().IntVar(&flagRelease, "release", 0, "Filter by PURSUE release tranche number (e.g. 1, 2)")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Maximum number of files to return (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override the synced SQLite store path (default: ~/.local/share/ufo-goat-pp-cli/data.db)")

	return cmd
}
