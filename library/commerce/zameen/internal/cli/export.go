// export: dump synced listings from the local store to CSV or JSON.
// Fulfils absorbed feature "bulk export result set to CSV/JSON".
// pp:data-source local
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newExportCmd(flags *rootFlags) *cobra.Command {
	var dbPath, out, format string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export synced listings to CSV or JSON",
		Long: "Dump every listing in the local store to stdout (or --out <file>) as CSV or JSON. " +
			"Run 'zameen-pp-cli pull ...' first to populate the store.",
		Example:     "  zameen-pp-cli export --format csv --out listings.csv",
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:write-positionals": ""},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would export the local store")
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("zameen-pp-cli")
			}
			listings, err := loadStoredListings(cmd.Context(), dbPath)
			if err != nil {
				if errors.Is(err, errNoMirror) {
					return emitEmptyMirrorHint(cmd, flags, dbPath)
				}
				return err
			}
			raw, err := json.Marshal(listings)
			if err != nil {
				return err
			}
			toFile := out != "" && out != "-"
			w := cmd.OutOrStdout()
			if toFile {
				f, ferr := os.Create(out)
				if ferr != nil {
					return fmt.Errorf("creating %s: %w", out, ferr)
				}
				defer f.Close()
				w = f
			}
			// When writing to a file, stdout stays free for a machine-readable
			// status envelope under --json/--agent (so piping/consumers and the
			// dogfood json_fidelity probe get valid JSON, not an empty stream).
			if toFile && (flags.asJSON || flags.agent) {
				if serr := printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"exported": len(listings), "format": format, "file": out,
				}, flags); serr != nil {
					return serr
				}
			}
			switch format {
			case "json":
				if err := printOutput(w, raw, true); err != nil {
					return err
				}
			default: // csv
				if err := printCSV(w, raw); err != nil {
					return err
				}
			}
			if toFile {
				fmt.Fprintf(cmd.ErrOrStderr(), "exported %d listings to %s (%s)\n", len(listings), out, format)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "csv", "Output format: csv or json")
	cmd.Flags().StringVar(&out, "out", "", "Write to a file instead of stdout (use - for stdout)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard data dir)")
	return cmd
}
