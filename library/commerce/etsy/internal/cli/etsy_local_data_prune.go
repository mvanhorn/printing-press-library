// Hand-written Etsy local retention command; preserved across reprints.
package cli

import (
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source local
func newEtsyLocalDataPruneCmd(flags *rootFlags) *cobra.Command {
	var older time.Duration
	var receipts, reviews, clear, vacuum bool
	var dbPath string
	cmd := &cobra.Command{Use: "prune", Short: "Prune old shipped receipts and reviews (dry run unless --yes)", Args: cobra.NoArgs,
		Example: `  # Preview receipt retention
  etsy-pp-cli local-data prune --older-than 2160h --receipts

  # Preview review retention as JSON
  etsy-pp-cli local-data prune --older-than 8760h --reviews --json

  # Preview clearing receipt retention floors
  etsy-pp-cli local-data prune --clear-floor --receipts`,
		Annotations: map[string]string{"mcp:hidden": "true", "pp:typed-exit-codes": "0,2", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Refuse even previews and operator buyer-data opt-ins before opening a store.
			if cliutil.IsMCPChildEnv() {
				return usageErr(fmt.Errorf("local-data prune is unavailable over MCP"))
			}
			if clear && cmd.Flags().Changed("older-than") {
				return usageErr(fmt.Errorf("--clear-floor and --older-than are mutually exclusive"))
			}
			if !clear && older <= 0 {
				return usageErr(fmt.Errorf("--older-than must be a positive duration"))
			}
			if !receipts && !reviews {
				receipts = true
				reviews = true
			}
			return etsyRunSelective(cmd, flags, dbPath, store.EtsyPruneOptions{Cutoff: time.Now().Add(-older).Unix(), Receipts: receipts, Reviews: reviews, ClearFloor: clear, Vacuum: vacuum}, "local-data prune", map[string]any{"clear_floor": clear})
		}}
	cmd.Flags().DurationVar(&older, "older-than", 0, "Retention age, e.g. 2160h (90 days)")
	cmd.Flags().BoolVar(&receipts, "receipts", false, "Prune shipped receipts and dependent buyer data")
	cmd.Flags().BoolVar(&reviews, "reviews", false, "Prune reviews and listings_reviews")
	cmd.Flags().BoolVar(&clear, "clear-floor", false, "Remove the selected persisted retention floors")
	cmd.Flags().BoolVar(&vacuum, "vacuum", false, "Secure-delete and reclaim SQLite free pages (stop other readers first)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Additional store file to process (DataDir stores are always processed)")
	return cmd
}
