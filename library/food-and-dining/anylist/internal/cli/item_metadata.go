package cli

import (
	"fmt"
	"math"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/store"
)

func boolFromBody(body map[string]any, key string) bool {
	if body == nil {
		return false
	}
	v, ok := body[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func floatFromBody(body map[string]any, key string) (float64, bool) {
	if body == nil {
		return 0, false
	}
	v, ok := body[key]
	if !ok {
		return 0, false
	}
	switch value := v.(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	}
	return 0, false
}

func liveListResponseByID(userData *pb.PBUserDataResponse, listID string) (*pb.PBListResponse, bool) {
	if userData == nil || userData.GetShoppingListsResponse() == nil {
		return nil, false
	}
	for _, response := range userData.GetShoppingListsResponse().GetListResponses() {
		if response.GetListId() == listID {
			return response, true
		}
	}
	return nil, false
}

// resolveLiveCategory resolves only an exact category identifier or name from
// the fresh list response. Fuzzy matching would make an --apply request
// capable of assigning the wrong aisle/category, so ambiguous names fail.
func resolveLiveCategory(userData *pb.PBUserDataResponse, listID, token string) (string, error) {
	category, err := resolveLiveCategoryRecord(userData, listID, token)
	if err != nil {
		return "", err
	}
	if matchID := liveCategoryMatchID(userData, listID, category, token); matchID != "" {
		return matchID, nil
	}
	return "", fmt.Errorf("category %q has no verified category match ID in list %q", category.GetName(), listID)
}

func liveCategoryMatchID(userData *pb.PBUserDataResponse, listID string, category *pb.PBListCategory, token string) string {
	if category == nil {
		return ""
	}
	if strings.EqualFold(category.GetName(), "other") || strings.EqualFold(category.GetSystemCategory(), "other") {
		return "other"
	}
	// When the caller supplied a conventional match ID (for example,
	// pantry-aisle) rather than the category object's opaque identifier, keep
	// that explicit ID after resolving the category name exactly.
	if normalizeCategoryToken(token) == normalizeCategoryToken(category.GetName()) && !strings.EqualFold(token, category.GetName()) {
		return strings.TrimSpace(token)
	}
	if list, found := findLiveShoppingListByID(userData, listID); found {
		for _, item := range list.GetItems() {
			if strings.EqualFold(item.GetCategory(), category.GetName()) && item.GetCategoryMatchId() != "" {
				return item.GetCategoryMatchId()
			}
		}
	}
	// Custom AnyList categories normally use their category identifier as the
	// match ID (for example, pantry-aisle). This is only a fallback after the
	// fresh category record has been resolved exactly; built-in Other is handled
	// above because its object identifier is not its match ID.
	return category.GetIdentifier()
}

func normalizeCategoryToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func resolveLiveCategoryRecord(userData *pb.PBUserDataResponse, listID, token string) (*pb.PBListCategory, error) {
	response, ok := liveListResponseByID(userData, listID)
	if !ok {
		return nil, fmt.Errorf("list %q has no fresh category metadata", listID)
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("category must not be empty")
	}
	var matches []*pb.PBListCategory
	for _, groupResponse := range response.GetCategoryGroupResponses() {
		group := groupResponse.GetCategoryGroup()
		for _, category := range group.GetCategories() {
			if category.GetIdentifier() == token || strings.EqualFold(category.GetName(), token) || normalizeCategoryToken(category.GetName()) == normalizeCategoryToken(token) {
				matches = append(matches, category)
			}
		}
	}
	if len(matches) == 0 {
		if list, found := findLiveShoppingListByID(userData, listID); found {
			for _, item := range list.GetItems() {
				if item.GetCategoryMatchId() != token {
					continue
				}
				for _, groupResponse := range response.GetCategoryGroupResponses() {
					for _, category := range groupResponse.GetCategoryGroup().GetCategories() {
						if strings.EqualFold(category.GetName(), item.GetCategory()) {
							matches = append(matches, category)
						}
					}
				}
			}
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("category %q was not found in list %q", token, listID)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("category %q is ambiguous in list %q", token, listID)
	}
	return matches[0], nil
}

// resolveLiveStore applies the same exact-match policy to per-item store
// assignments. The store catalog is list-scoped in the AnyList protobuf.
func resolveLiveStore(userData *pb.PBUserDataResponse, listID, token string) (string, error) {
	response, ok := liveListResponseByID(userData, listID)
	if !ok {
		return "", fmt.Errorf("list %q has no fresh store metadata", listID)
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", fmt.Errorf("store must not be empty")
	}
	var matches []*pb.PBStore
	for _, store := range response.GetStores() {
		if store.GetIdentifier() == token || strings.EqualFold(store.GetName(), token) {
			matches = append(matches, store)
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("store %q was not found in list %q", token, listID)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("store %q is ambiguous in list %q", token, listID)
	}
	return matches[0].GetIdentifier(), nil
}

func itemHasStoreID(item *pb.ListItem, storeID string) bool {
	if item == nil {
		return false
	}
	return containsStoreID(item.GetStoreIds(), storeID)
}

func verifyLiveItemMetadata(item *pb.ListItem, categoryID string, categoryChanged bool, addStoreID, removeStoreID string) error {
	if item == nil {
		return fmt.Errorf("item metadata verification failed: item is missing")
	}
	if categoryChanged && !categoryMatchMatches(item.GetCategoryMatchId(), categoryID) {
		return fmt.Errorf("item metadata verification failed: category match is %q, expected %q", item.GetCategoryMatchId(), categoryID)
	}
	if addStoreID != "" && !itemHasStoreID(item, addStoreID) {
		return fmt.Errorf("item metadata verification failed: store %q was not assigned", addStoreID)
	}
	if removeStoreID != "" && itemHasStoreID(item, removeStoreID) {
		return fmt.Errorf("item metadata verification failed: store %q is still assigned", removeStoreID)
	}
	return nil
}

func verifyCachedItemMetadata(item *store.ItemRow, categoryID string, categoryChanged bool, addStoreID, removeStoreID string) error {
	if item == nil {
		return fmt.Errorf("cached item metadata verification failed: item is missing")
	}
	if categoryChanged && !categoryMatchMatches(item.CategoryMatchID, categoryID) {
		return fmt.Errorf("cached item metadata verification failed: category match is %q, expected %q", item.CategoryMatchID, categoryID)
	}
	if addStoreID != "" && !containsStoreID(item.StoreIDs, addStoreID) {
		return fmt.Errorf("cached item metadata verification failed: store %q was not assigned", addStoreID)
	}
	if removeStoreID != "" && containsStoreID(item.StoreIDs, removeStoreID) {
		return fmt.Errorf("cached item metadata verification failed: store %q is still assigned", removeStoreID)
	}
	return nil
}

func verifyLiveItemPrice(item *pb.ListItem, expected *pb.PBItemPrice, clear bool) error {
	if item == nil {
		return fmt.Errorf("item price verification failed: item is missing")
	}
	if clear {
		for _, price := range item.GetPrices() {
			if expected != nil && expected.GetStoreId() != "" && price.GetStoreId() != expected.GetStoreId() {
				continue
			}
			if price.GetAmount() > 0 {
				if expected != nil && expected.GetStoreId() != "" {
					return fmt.Errorf("item price verification failed: positive price %.2f remains for store %q", price.GetAmount(), expected.GetStoreId())
				}
				return fmt.Errorf("item price verification failed: positive price %.2f remains", price.GetAmount())
			}
		}
		return nil
	}
	if expected == nil {
		return fmt.Errorf("item price verification failed: expected price is missing")
	}
	for _, price := range item.GetPrices() {
		if math.Abs(price.GetAmount()-expected.GetAmount()) > 0.000001 || price.GetStoreId() != expected.GetStoreId() || price.GetDetails() != expected.GetDetails() {
			continue
		}
		return nil
	}
	return fmt.Errorf("item price verification failed: %.2f was not read back", expected.GetAmount())
}

func verifyLiveItemPrices(item *pb.ListItem, expected []*pb.PBItemPrice, clear bool) error {
	for _, price := range expected {
		if err := verifyLiveItemPrice(item, price, clear); err != nil {
			return err
		}
	}
	return nil
}

func verifyCachedItemPrice(item *store.ItemRow, expected *pb.PBItemPrice, clear bool) error {
	if item == nil {
		return fmt.Errorf("cached item price verification failed: item is missing")
	}
	if clear {
		for _, price := range item.Prices {
			if expected != nil && expected.GetStoreId() != "" && price.GetStoreId() != expected.GetStoreId() {
				continue
			}
			if price.GetAmount() > 0 {
				if expected != nil && expected.GetStoreId() != "" {
					return fmt.Errorf("cached item price verification failed: positive price %.2f remains for store %q", price.GetAmount(), expected.GetStoreId())
				}
				return fmt.Errorf("cached item price verification failed: positive price %.2f remains", price.GetAmount())
			}
		}
		return nil
	}
	if expected == nil {
		return fmt.Errorf("cached item price verification failed: expected price is missing")
	}
	for _, price := range item.Prices {
		if math.Abs(price.GetAmount()-expected.GetAmount()) <= 0.000001 && price.GetStoreId() == expected.GetStoreId() && price.GetDetails() == expected.GetDetails() {
			return nil
		}
	}
	return fmt.Errorf("cached item price verification failed: %.2f was not persisted", expected.GetAmount())
}

func verifyCachedItemPrices(item *store.ItemRow, expected []*pb.PBItemPrice, clear bool) error {
	for _, price := range expected {
		if err := verifyCachedItemPrice(item, price, clear); err != nil {
			return err
		}
	}
	return nil
}

func priceClearTargets(item *pb.ListItem, requestedStore string) []*pb.PBItemPrice {
	storeID := strings.TrimSpace(requestedStore)
	if storeID != "" {
		return []*pb.PBItemPrice{{StoreId: storeID}}
	}
	seen := make(map[string]struct{})
	targets := make([]*pb.PBItemPrice, 0, len(item.GetPrices()))
	for _, price := range item.GetPrices() {
		id := price.GetStoreId()
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		targets = append(targets, &pb.PBItemPrice{StoreId: id})
	}
	if len(targets) == 0 {
		// Keep the operation deterministic for an item with no cached price. The
		// read-after-write check still proves that no positive price exists.
		return []*pb.PBItemPrice{{}}
	}
	return targets
}

func categoryMatchMatches(got, expected string) bool {
	if got == expected {
		return true
	}
	// AnyList represents a cleared custom category as either the built-in
	// "other" match or an empty match depending on the sync generation.
	return expected == "other" && got == ""
}

func containsStoreID(storeIDs []string, want string) bool {
	for _, storeID := range storeIDs {
		if storeID == want {
			return true
		}
	}
	return false
}
