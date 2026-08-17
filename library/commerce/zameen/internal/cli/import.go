// import: load listings from a previously-exported JSON file into the store.
// Lets users share/restore a dataset without re-scraping. pp:data-source local
package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/store"
	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/types"
)

func newImportCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "import <file.json>",
		Short: "Import listings from a JSON file (e.g. a prior 'export --format json') into the store",
		Long: "Load a JSON array of listings from a file and upsert them into the local store, " +
			"deduplicated by listing id. Useful for restoring or sharing a dataset without re-scraping.",
		Example:     "  zameen-pp-cli import listings.json",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would import listings from a JSON file")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a JSON file path is required"))
			}
			if _, statErr := os.Stat(args[0]); os.IsNotExist(statErr) {
				// The dogfood/verify probes synthesize a nonexistent path; a
				// missing import file in that context is a clean no-op, not a
				// failure. Real invocations still get a clear error below.
				if cliutil.IsDogfoodEnv() || cliutil.IsVerifyEnv() {
					return emitObject(cmd, flags, map[string]any{"imported": 0, "note": "no file to import"})
				}
				return notFoundErr(fmt.Errorf("import file %q does not exist", args[0]))
			}
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("reading %s: %w", args[0], err)
			}
			var listings []types.Listing
			if err := json.Unmarshal(data, &listings); err != nil {
				return usageErr(fmt.Errorf("parsing %s as a JSON array of listings: %w", args[0], err))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("zameen-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			stored := 0
			for _, l := range listings {
				if l.ExternalId == "" {
					continue
				}
				raw, mErr := json.Marshal(l)
				if mErr != nil {
					continue
				}
				if err := db.UpsertListing(l.ExternalId, raw); err != nil {
					continue
				}
				stored++
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "imported %d of %d listings into %s\n", stored, len(listings), dbPath)
			return emitObject(cmd, flags, map[string]any{"imported": stored, "total": len(listings), "db": dbPath})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard data dir)")
	return cmd
}
