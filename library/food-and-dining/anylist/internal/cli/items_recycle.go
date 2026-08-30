// Copyright 2026 Jeeves and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"

	"github.com/spf13/cobra"
)

func newItemsRecycleCmd(flags *rootFlags) *cobra.Command {
	var bodySourceListName string
	var bodySourceItemID string
	var bodyTargetListName string
	var apply bool

	cmd := &cobra.Command{
		Use:         "recycle",
		Short:       "Recycle (explicit copy) one item from a source list into a target list",
		Example:     "  anylist-pp-cli items recycle --source-list pantry --source-item-id abc123 --target-list grocery",
		Annotations: map[string]string{"pp:endpoint": "items.recycle", "pp:method": "POST", "pp:path": "/data/shopping-lists/update"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("source-list") && !flags.dryRun {
				return fmt.Errorf("required flag \"source-list\" not set")
			}
			if !cmd.Flags().Changed("source-item-id") && !flags.dryRun {
				return fmt.Errorf("required flag \"source-item-id\" not set")
			}
			if !cmd.Flags().Changed("target-list") && !flags.dryRun {
				return fmt.Errorf("required flag \"target-list\" not set")
			}
			if dryRunOK(flags) {
				return nil
			}

			ctx := cmd.Context()

			cfg, st, err := openAuthedLocalStore(flags)
			if err != nil {
				return err
			}
			defer st.Close()

			sourceList, err := st.FindListByName(bodySourceListName)
			if err != nil {
				return err
			}

			targetList, err := st.FindListByName(bodyTargetListName)
			if err != nil {
				return err
			}

			// Resolve source item by stable ID within the source list only.
			sourceItem, err := st.FindItemByID(sourceList.ID, bodySourceItemID)
			if err != nil {
				return fmt.Errorf("source item %q not found in list %q: %w", bodySourceItemID, bodySourceListName, err)
			}
			// Defensive: confirm the source item actually belongs to sourceList.
			if sourceItem.ListID != sourceList.ID {
				return fmt.Errorf("source item %q belongs to list %q, not the requested source list %q",
					bodySourceItemID, sourceItem.ListID, bodySourceListName)
			}

			// Source and target must not be the same list.
			if sourceList.ID == targetList.ID {
				return fmt.Errorf("source list and target list are the same: %q", bodySourceListName)
			}

			// Prevent reusing already-checked items (source mutation would be wrong).
			if sourceItem.Checked {
				return fmt.Errorf("source item %q in %q is already checked; recycle does not mutate checked items",
					sourceItem.Name, bodySourceListName)
			}

			// Fresh-read both lists from the live API so we have authoritative
			// metadata references before deciding what to write.
			alClient := anylist.New(cfg)
			liveData, err := alClient.GetUserData(ctx)
			if err != nil {
				return fmt.Errorf("reading live data: %w", err)
			}

			// Verify source item still exists in live data under the source list.
			liveSourceItem, found := findLiveItemByID(liveData, sourceList.ID, bodySourceItemID)
			if !found {
				return fmt.Errorf("source item %q no longer present in list %q", bodySourceItemID, bodySourceListName)
			}

			// Reject checked items from the live read — the cached check above
			// is not enough; a stale cache could have missed a check.
			if liveSourceItem.GetChecked() {
				return fmt.Errorf("source item %q in %q is already checked; recycle does not mutate checked items",
					liveSourceItem.GetName(), bodySourceListName)
			}

			// --- Build preview of what would be copied ---

			// Safe scalar metadata to clone into the target item.
			var copiedFields []string
			var omittedFields []string
			copied := map[string]string{}

			name := liveSourceItem.GetName()
			copied["name"] = name
			copiedFields = append(copiedFields, "name")

			qty := liveSourceItem.GetQuantity()
			if qty != "" {
				copied["quantity"] = qty
				copiedFields = append(copiedFields, "quantity")
			}

			details := liveSourceItem.GetDetails()
			if details != "" {
				copied["details"] = details
				copiedFields = append(copiedFields, "details")
			}

			upc := liveSourceItem.GetProductUpc()
			if upc != "" {
				copied["product_upc"] = upc
				copiedFields = append(copiedFields, "product_upc")
			}

			psText := formatPackageSize(liveSourceItem.GetPackageSizePb())
			if psText != "" {
				copied["package_size"] = psText
				copiedFields = append(copiedFields, "package_size")
			}

			// List-scoped category metadata is not copied unless a typed
			// category-assignment operation and read-back verification are used.
			// The create path cannot provide that guarantee, so report it omitted.
			sourceCat := liveSourceItem.GetCategory()
			if sourceCat != "" {
				omittedFields = append(omittedFields, "category (list-scoped assignment not supported by recycle)")
			}

			// Store and price metadata are list-scoped; the typed create path
			// (AddItemWithOptionsAndID) does not apply stores or prices, so
			// these are always reported as omitted even when resolvable.
			sourceStores := liveSourceItem.GetStoreIds()
			if len(sourceStores) > 0 {
				omittedFields = append(omittedFields, "store ("+fmt.Sprintf("%d store(s), not supported by this command)", len(sourceStores)))
			}

			sourcePrices := liveSourceItem.GetPrices()
			if len(sourcePrices) > 0 {
				omittedFields = append(omittedFields, "prices ("+fmt.Sprintf("%d price(s), not supported by this command)", len(sourcePrices)))
			}

			// Check photos — always omitted because the typed path cannot clone them.
			sourcePhotos := liveSourceItem.GetPhotoIds()
			if len(sourcePhotos) > 0 {
				omittedFields = append(omittedFields, "photos ("+fmt.Sprintf("%d photo(s), not supported by this command)", len(sourcePhotos)))
			}

			// --- Duplicate check against target list ---

			liveTargetList, found := findLiveShoppingListByID(liveData, targetList.ID)
			if !found {
				return fmt.Errorf("target list %q not found in live data", bodyTargetListName)
			}

			var existingMatch string
			var conflictUPC string
			var conflictName string

			for _, item := range liveTargetList.GetItems() {
				// UPC conflict: same non-empty UPC → hard conflict.
				if item.GetProductUpc() != "" && upc != "" &&
					normalizedUPC(item.GetProductUpc()) == normalizedUPC(upc) {
					conflictUPC = upc
					conflictName = item.GetName()
					break
				}
				// Exact normalized name + matching package-size → existing-target no-op.
				if normalizedName(item.GetName()) == normalizedName(name) && normalizedPackageSize(formatPackageSize(item.GetPackageSizePb())) == normalizedPackageSize(psText) {
					existingMatch = item.GetIdentifier()
					break
				}
			}

			if conflictUPC != "" {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"action":           "conflict",
						"source":           bodySourceListName,
						"source_item":      bodySourceItemID,
						"target":           bodyTargetListName,
						"reason":           "target item with matching UPC already exists",
						"conflict_item":    conflictName,
						"conflict_upc":     conflictUPC,
						"copied_metadata":  copied,
						"omitted_metadata": omittedFields,
					}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "CONFLICT: target %q already has item %q with UPC %q\n", bodyTargetListName, conflictName, conflictUPC)
				fmt.Fprintf(cmd.OutOrStdout(), "Source item %q (ID %s) in %q would copy UPC %q which conflicts\n", name, bodySourceItemID, bodySourceListName, conflictUPC)
				return nil
			}

			if existingMatch != "" {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"action":           "existing",
						"source":           bodySourceListName,
						"source_item":      bodySourceItemID,
						"target":           bodyTargetListName,
						"target_item":      existingMatch,
						"reason":           "exact normalized name + package-size match already exists in target",
						"copied_metadata":  copied,
						"omitted_metadata": omittedFields,
					}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "EXISTS: target %q already has item matching %q (ID %s)\n", bodyTargetListName, name, existingMatch)
				fmt.Fprintf(cmd.OutOrStdout(), "No write needed\n")
				return nil
			}

			// Preview mode: report what would happen without writing.
			if !apply {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"action":           "recycle_preview",
						"source":           bodySourceListName,
						"source_item":      bodySourceItemID,
						"target":           bodyTargetListName,
						"copied_metadata":  copied,
						"omitted_metadata": omittedFields,
						"message":          "preview — pass --apply to write",
					}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Preview: would recycle %q (ID %s) from %q into %q\n", name, bodySourceItemID, bodySourceListName, bodyTargetListName)
				if len(copiedFields) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "Copied: %s\n", joinedStrings(copiedFields))
				}
				if len(omittedFields) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "Omitted: %s\n", joinedStrings(omittedFields))
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Pass --apply to write\n")
				return nil
			}

			// --- Apply: create the target item ---

			// Build the new item with cloned safe metadata.
			_, err = alClient.AddItemWithOptionsAndID(ctx, targetList.ID, name, anylist.ItemAddOptions{
				Quantity:    copied["quantity"],
				Details:     copied["details"],
				ProductUpc:  copied["product_upc"],
				PackageSize: copied["package_size"],
			})
			if err != nil {
				return fmt.Errorf("creating recycled item: %w", err)
			}

			// Fresh read-after-write verification.
			verifiedData, err := alClient.GetUserData(ctx)
			if err != nil {
				return fmt.Errorf("recycle wrote item but fresh read-back failed: %w", err)
			}
			verifiedList, found := findLiveShoppingListByID(verifiedData, targetList.ID)
			if !found {
				return fmt.Errorf("recycle wrote item but target list disappeared from live data")
			}
			// Verify the new item exists with expected fields.
			foundNew := false
			for _, item := range verifiedList.GetItems() {
				if normalizedName(item.GetName()) == normalizedName(name) && item.GetProductUpc() == copied["product_upc"] {
					foundNew = true
					break
				}
			}
			if !foundNew {
				return fmt.Errorf("recycle wrote item %q but it was not found in fresh read-back from %q", name, bodyTargetListName)
			}

			// Update local cache.
			if err := st.SyncFromUserData(verifiedData); err != nil {
				return fmt.Errorf("recycle wrote item but cache sync failed: %w", err)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"action":           "recycled",
					"source":           bodySourceListName,
					"source_item":      bodySourceItemID,
					"target":           bodyTargetListName,
					"copied_metadata":  copied,
					"omitted_metadata": omittedFields,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Recycled %q from %q into %q\n", name, bodySourceListName, bodyTargetListName)
			return nil
		},
	}

	cmd.Flags().StringVar(&bodySourceListName, "source-list", "", "Source list name (item to recycle from)")
	cmd.Flags().StringVar(&bodySourceItemID, "source-item-id", "", "Source item stable ID")
	cmd.Flags().StringVar(&bodyTargetListName, "target-list", "", "Target list name (item to copy into)")
	cmd.Flags().BoolVar(&apply, "apply", false, "Enable live mutation (default: preview only)")

	return cmd
}

// joinedStrings formats a string slice for human-readable output.
func joinedStrings(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = `"` + s + `"`
	}
	return join(out, ", ")
}

func join(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	s := ss[0]
	for i := 1; i < len(ss); i++ {
		s += sep + ss[i]
	}
	return s
}

// resolveCategoryForTarget looks up the source category's name in the target
// list's fresh metadata. If the target has an exact name match, returns it.
// Otherwise returns an error so the caller can report the omission.
func resolveCategoryForTarget(userData *pb.PBUserDataResponse, targetListID, sourceCategory string) (string, error) {
	sourceCategory = strings.TrimSpace(sourceCategory)
	if sourceCategory == "" {
		return "", fmt.Errorf("empty category")
	}
	// Direct lookup by exact name or identifier in target list.
	resp, ok := liveListResponseByID(userData, targetListID)
	if !ok {
		return "", fmt.Errorf("target list %q has no fresh metadata", targetListID)
	}
	for _, groupResp := range resp.GetCategoryGroupResponses() {
		for _, cat := range groupResp.GetCategoryGroup().GetCategories() {
			if cat.GetIdentifier() == sourceCategory || strings.EqualFold(cat.GetName(), sourceCategory) {
				return cat.GetName(), nil
			}
		}
	}
	return "", fmt.Errorf("category %q not found in target list %q", sourceCategory, targetListID)
}

// resolveStoreForTarget looks up the source store (identifier or name) in the
// target list's fresh metadata. Returns the target store's display name, or
// an error if no exact match is found.
func resolveStoreForTarget(userData *pb.PBUserDataResponse, targetListID, sourceStoreToken string) (string, error) {
	sourceStoreToken = strings.TrimSpace(sourceStoreToken)
	if sourceStoreToken == "" {
		return "", fmt.Errorf("empty store token")
	}
	response, ok := liveListResponseByID(userData, targetListID)
	if !ok {
		return "", fmt.Errorf("target list %q has no fresh store metadata", targetListID)
	}
	for _, store := range response.GetStores() {
		if store.GetIdentifier() == sourceStoreToken || strings.EqualFold(store.GetName(), sourceStoreToken) {
			return store.GetName(), nil
		}
	}
	return "", fmt.Errorf("store %q not found in target list %q", sourceStoreToken, targetListID)
}
