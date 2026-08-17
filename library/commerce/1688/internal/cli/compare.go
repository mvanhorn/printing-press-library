// Hand-authored transcendence command. Not generator-emitted.
// pp:data-source local
package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/commerce/1688/internal/mtop"

	"github.com/spf13/cobra"
)

type compareRow struct {
	OfferID string      `json:"offer_id"`
	Found   bool        `json:"found"`
	Offer   *mtop.Offer `json:"offer,omitempty"`
	Note    string      `json:"note,omitempty"`
}

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "compare <offer-id> [offer-id...]",
		Short:       "Render a side-by-side table of price, MOQ, reorder rate, transactions, and factory flags for several offers",
		Long:        "Compare specific stored offers head to head on price, MOQ, reorder rate, transactions, factory flags, and supplier trade scores. Use for offers you already synced. Do NOT use to rank a fresh keyword search; use 'factory-find' (rank) or 'repurchase-top' (sort) for that.",
		Example:     "  1688-pp-cli compare 927875250705 836112681124",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare stored offers")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("at least one offer ID is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("1688-pp-cli")
			}
			db, err := openLocalStore(ctx, cmd, flags, dbPath)
			if err != nil {
				return err
			}
			if db == nil {
				return nil
			}
			defer db.Close()

			rows := make([]compareRow, 0, len(args))
			for _, id := range args {
				raw, gerr := db.Get("offer", id)
				if errors.Is(gerr, sql.ErrNoRows) || raw == nil {
					rows = append(rows, compareRow{OfferID: id, Found: false, Note: "not in local store; run: 1688-pp-cli sync <keyword>"})
					continue
				}
				if gerr != nil {
					return gerr
				}
				var o mtop.Offer
				if json.Unmarshal(raw, &o) != nil {
					rows = append(rows, compareRow{OfferID: id, Found: false, Note: "stored record could not be decoded"})
					continue
				}
				rows = append(rows, compareRow{OfferID: id, Found: true, Offer: &o})
			}
			return emit(cmd, flags, rows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store database path")
	return cmd
}
