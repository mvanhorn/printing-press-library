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
  ufo-goat-pp-cli files list --redacted`,
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
				Agency:   flagAgency,
				Type:     flagType,
				Location: flagLocation,
				After:    flagAfter,
				Before:   flagBefore,
				Limit:    flagLimit,
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

	cmd.Flags().StringVar(&flagAgency, "agency", "", "Filter by agency (DoD, FBI, NASA, State)")
	cmd.Flags().StringVar(&flagType, "type", "", "Filter by file type (PDF, VID, IMG)")
	cmd.Flags().StringVar(&flagLocation, "location", "", "Filter by incident location")
	cmd.Flags().StringVar(&flagAfter, "after", "", "Show files with incident dates after this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagBefore, "before", "", "Show files with incident dates before this date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&flagRedacted, "redacted", false, "Filter by redaction status")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Maximum number of files to return (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}
