package cli

import (
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

// etsyReplaceEmbeddedListingRows leaves absent embedded keys untouched.
func etsyReplaceEmbeddedListingRows(db *store.Store, listings []json.RawMessage) (inventory, images bool, err error) {
	var inventoryParents, imageParents []string
	var inventoryRows, imageRows []json.RawMessage
	for _, raw := range listings {
		obj, err := store.DecodeJSONObject(raw)
		if err != nil {
			return false, false, err
		}
		id, ok := obj["listing_id"]
		if !ok || id == nil {
			continue
		}
		parent := fmt.Sprint(id)
		stamp := func(row map[string]any) json.RawMessage {
			row["listing_id"] = parent
			row["parent_id"] = parent
			b, _ := json.Marshal(row)
			return b
		}
		if inv, ok := obj["inventory"].(map[string]any); ok {
			inventoryParents = append(inventoryParents, parent)
			products, _ := inv["products"].([]any)
			for _, p := range products {
				if row, ok := p.(map[string]any); ok {
					for _, key := range []string{"price_on_property", "quantity_on_property", "sku_on_property", "readiness_state_on_property"} {
						if value, ok := inv[key]; ok {
							row[key] = value
						}
					}
					inventoryRows = append(inventoryRows, stamp(row))
				}
			}
		}
		if images, ok := obj["images"].([]any); ok {
			imageParents = append(imageParents, parent)
			for _, v := range images {
				if row, ok := v.(map[string]any); ok {
					imageRows = append(imageRows, stamp(row))
				}
			}
		}
	}
	if _, _, _, _, err := db.ReplaceParentPartition("listings_inventory", "", inventoryParents, inventoryRows); err != nil {
		return false, false, err
	}
	_, _, _, _, err = db.ReplaceParentPartition("listing_images", "listing_images", imageParents, imageRows)
	// etsy-ext: report only embedded families whose partitions were replaced.
	return len(inventoryParents) > 0, len(imageParents) > 0 && err == nil, err
}
