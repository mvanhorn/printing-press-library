// Hand-authored shared helpers for the 1688 signed-search commands. Not
// generator-emitted.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/1688/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/1688/internal/mtop"
	"github.com/mvanhorn/printing-press-library/library/commerce/1688/internal/store"

	"github.com/spf13/cobra"
)

func (f *rootFlags) newMtopClient() *mtop.Client {
	return mtop.New(f.timeout, f.rateLimit)
}

// classify1688Err maps a typed rate-limit error to exit 7 and otherwise
// defers to the framework classifier.
func classify1688Err(err error, flags *rootFlags) error {
	var rle *cliutil.RateLimitError
	if errors.As(err, &rle) {
		return rateLimitErr(err)
	}
	return classifyAPIError(err, flags)
}

// persistSearch writes offers, suppliers, and drift snapshots from a live
// search result into the local store.
func persistSearch(ctx context.Context, db *store.Store, res *mtop.SearchResult) error {
	if res == nil {
		return nil
	}
	if err := db.Ensure1688Schema(ctx); err != nil {
		return err
	}
	for _, o := range res.Offers {
		data, err := json.Marshal(o)
		if err != nil {
			continue
		}
		if err := db.Upsert("offer", o.OfferID, data); err != nil {
			return err
		}
		if o.SupplierMemberID != "" {
			if sdata, err := json.Marshal(supplierFrom(o)); err == nil {
				if err := db.Upsert("supplier", o.SupplierMemberID, sdata); err != nil {
					return err
				}
			}
		}
		if err := db.InsertOfferSnapshot(ctx, store.OfferSnapshot{
			OfferID:       o.OfferID,
			SyncedAt:      o.SyncedAt,
			Keyword:       o.Keyword,
			PriceCNY:      o.PriceCNY,
			RepurchasePct: o.RepurchasePct,
			BookedCount:   o.TransactionCount,
		}); err != nil {
			return err
		}
	}
	return nil
}

func supplierFrom(o mtop.Offer) map[string]any {
	return map[string]any{
		"member_id":                o.SupplierMemberID,
		"name":                     o.SupplierName,
		"login_id":                 o.SupplierLoginID,
		"province":                 o.Province,
		"city":                     o.City,
		"shop_url":                 o.ShopURL,
		"trade_score_composite":    o.TradeComposite,
		"trade_score_logistics":    o.TradeLogistics,
		"trade_score_dispute":      o.TradeDispute,
		"trade_score_consultation": o.TradeConsultation,
	}
}

// openLocalStore enforces the missing-mirror contract for store-reading
// commands. Returns (nil, nil) after printing a sync hint when the DB file is
// absent; callers must treat a nil store as "handled, return nil".
func openLocalStore(ctx context.Context, cmd *cobra.Command, flags *rootFlags, dbPath string) (*store.Store, error) {
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: 1688-pp-cli sync <keyword> --db %s\n", dbPath, dbPath)
		if flags.asJSON || flags.agent {
			fmt.Fprintln(cmd.OutOrStdout(), "[]")
		}
		return nil, nil
	}
	return store.OpenWithContext(ctx, dbPath)
}

func decodeStoredOffers(raws []json.RawMessage) []mtop.Offer {
	out := make([]mtop.Offer, 0, len(raws))
	for _, r := range raws {
		var o mtop.Offer
		if json.Unmarshal(r, &o) == nil {
			out = append(out, o)
		}
	}
	return out
}

func hasServiceTag(tags []string, want string) bool {
	for _, t := range tags {
		if strings.Contains(t, want) {
			return true
		}
	}
	return false
}

// emit marshals a Go value and routes it through the shared output pipeline
// (--json/--csv/--compact/--select/--quiet/--plain + human table).
func emit(cmd *cobra.Command, flags *rootFlags, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
}
