package cli

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

// etsyWriteThroughBatch keeps marketplace results out of the owned listing cache.
// C2 shop-scoped readers (batch C) will also filter by shop_id.
// marketplace_listings is the foreign/search partition; batch D market commands
// may reuse this name. This downstream policy belongs outside generated helpers.
func etsyWriteThroughBatch(db *store.Store, storedType string, items []json.RawMessage, configPath string) {
	if storedType != "listings" {
		_, _, _ = db.UpsertBatch(storedType, items)
		return
	}
	shopID := etsyWriteThroughShop(configPath)
	var own, foreign []json.RawMessage
	for _, item := range items {
		obj, _ := store.DecodeJSONObject(item)
		value, hasShop := obj["shop_id"]
		owned := shopID != "" && hasShop && etsyWriteThroughShopID(value) == shopID
		if shopID != "" && !hasShop {
			if id := store.ExtractResourceID("listings", obj); id != "" {
				if raw, err := db.Get("listings", id); err == nil {
					existing, _ := store.DecodeJSONObject(raw)
					owned = etsyWriteThroughShopID(existing["shop_id"]) == shopID
				}
			}
		}
		if owned {
			own = append(own, item)
		} else {
			foreign = append(foreign, item)
		}
	}
	// Keep both best-effort writes independent: failure of one must not skip the other.
	if len(own) > 0 {
		_, _, _ = db.UpsertBatch("listings", own)
	}
	if len(foreign) > 0 {
		_, _, _ = db.UpsertBatch("marketplace_listings", foreign)
	}
}

func etsyWriteThroughShopID(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return v.String()
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return ""
	}
}

// etsyWriteThroughShop shares the command-aware shop resolver without duplicating precedence.
func etsyWriteThroughShop(configPath string) string {
	var etsyCmd *cobra.Command
	if configPath != "" {
		etsyCmd = &cobra.Command{}
		etsyCmd.Flags().String("config", configPath, "")
	}
	return shopxShopID(etsyCmd)
}
