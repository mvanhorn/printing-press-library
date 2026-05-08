// Custom files search command using FTS5 full-text search.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/ufo-goat/internal/store"

	"github.com/spf13/cobra"
)

func newUFOFilesSearchCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search across file titles, descriptions, and locations",
		Long: `Search across all file titles, descriptions, incident locations, and agencies
using SQLite FTS5 full-text search. Supports stemming and prefix matching.`,
		Example: `  # Search for Apollo-related files
  ufo-goat-pp-cli search "Apollo"

  # Search for a location
  ufo-goat-pp-cli search "New Mexico"

  # Search with JSON output
  ufo-goat-pp-cli search "radar" --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")

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

			files, err := db.SearchUFOFiles(query, flagLimit)
			if err != nil {
				return fmt.Errorf("searching files: %w", err)
			}

			if len(files) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No files match %q\n", query)
				return nil
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

			// CSV output
			if flags.csv {
				data, _ := json.Marshal(files)
				return printCSV(cmd.OutOrStdout(), json.RawMessage(data))
			}

			// Table output
			fmt.Fprintf(os.Stderr, "%d results for %q\n\n", len(files), query)

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, strings.Join([]string{
				bold("ID"), bold("TITLE"), bold("TYPE"), bold("AGENCY"), bold("LOCATION"),
			}, "\t"))

			for _, f := range files {
				loc := f.IncidentLocation
				if loc == "" {
					loc = "-"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
					f.ID[:8],
					truncate(f.Title, 50),
					f.Type,
					f.Agency,
					truncate(loc, 30),
				)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum number of results")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}
