// Hand-authored offer-detail command. The robust data source is the signed
// mtop search gateway (detail.1688.com throttles direct fetches), so this
// reads the local store populated by search/sync. Not generator-emitted.
// pp:data-source local
package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/commerce/1688/internal/mtop"
	"github.com/mvanhorn/printing-press-library/library/commerce/1688/internal/store"

	"github.com/spf13/cobra"
)

func newOffersPromotedCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "offers <offer_id>",
		Short:       "Show a stored 1688 offer by ID (full record from a prior search/sync)",
		Long:        "Return the full record for a single offer by ID from the local store: price, MOQ, supplier, region, transaction count, reorder rate, factory flags, and trade scores. Populate the store first with 'search <keyword>' or 'sync <keyword>' (detail.1688.com itself throttles direct fetches, so the mtop search gateway is the reliable source).",
		Example:     "  1688-pp-cli offers 927875250705",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "offer_id=927875250705", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch a stored offer by ID")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("offer_id is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			offerID := args[0]
			if dbPath == "" {
				dbPath = defaultDBPath("1688-pp-cli")
			}
			notFound := map[string]any{
				"offer_id": offerID,
				"found":    false,
				"url":      fmt.Sprintf("https://detail.1688.com/offer/%s.html", offerID),
				"note":     "not in local store; run: 1688-pp-cli search <keyword> (or sync <keyword>) to populate the full record",
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				return emit(cmd, flags, notFound)
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			raw, gerr := db.Get("offer", offerID)
			if errors.Is(gerr, sql.ErrNoRows) || raw == nil {
				return emit(cmd, flags, notFound)
			}
			if gerr != nil {
				return gerr
			}
			var o mtop.Offer
			if json.Unmarshal(raw, &o) == nil {
				return emit(cmd, flags, o)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store database path")
	return cmd
}
