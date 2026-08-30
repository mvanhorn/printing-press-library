package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/store"
)

func TestResolveLiveCategoryAndStoreUsesExactListMetadata(t *testing.T) {
	t.Parallel()

	data := &pb.PBUserDataResponse{ShoppingListsResponse: &pb.ShoppingListsResponse{
		ListResponses: []*pb.PBListResponse{{
			ListId: "list-1",
			CategoryGroupResponses: []*pb.PBListCategoryGroupResponse{{
				CategoryGroup: &pb.PBListCategoryGroup{Categories: []*pb.PBListCategory{
					{Identifier: "cat-1", Name: "Produce"},
				}},
			}},
			Stores: []*pb.PBStore{{Identifier: "store-1", Name: "Paris Walmart"}},
		}},
	}}

	if got, err := resolveLiveCategory(data, "list-1", "Produce"); err != nil || got != "cat-1" {
		t.Fatalf("resolveLiveCategory by name = %q, %v; want cat-1, nil", got, err)
	}
	if got, err := resolveLiveCategory(data, "list-1", "cat-1"); err != nil || got != "cat-1" {
		t.Fatalf("resolveLiveCategory by ID = %q, %v; want cat-1, nil", got, err)
	}
	if got, err := resolveLiveStore(data, "list-1", "Paris Walmart"); err != nil || got != "store-1" {
		t.Fatalf("resolveLiveStore by name = %q, %v; want store-1, nil", got, err)
	}
	if _, err := resolveLiveCategory(data, "list-1", "missing"); err == nil {
		t.Fatal("resolveLiveCategory accepted an unknown category")
	}
}

func TestResolveLiveCategoryRejectsAmbiguousNames(t *testing.T) {
	t.Parallel()

	data := &pb.PBUserDataResponse{ShoppingListsResponse: &pb.ShoppingListsResponse{
		ListResponses: []*pb.PBListResponse{{
			ListId: "list-1",
			CategoryGroupResponses: []*pb.PBListCategoryGroupResponse{
				{CategoryGroup: &pb.PBListCategoryGroup{Categories: []*pb.PBListCategory{{Identifier: "cat-1", Name: "Aisle"}}}},
				{CategoryGroup: &pb.PBListCategoryGroup{Categories: []*pb.PBListCategory{{Identifier: "cat-2", Name: "Aisle"}}}},
			},
		}},
	}}
	if _, err := resolveLiveCategory(data, "list-1", "Aisle"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("resolveLiveCategory error = %v, want ambiguity", err)
	}
}

func TestResolveLiveCategoryUsesMatchIDForBuiltInOther(t *testing.T) {
	t.Parallel()

	data := &pb.PBUserDataResponse{ShoppingListsResponse: &pb.ShoppingListsResponse{
		NewLists: []*pb.ShoppingList{{Identifier: "list-1", Items: []*pb.ListItem{{Category: "Other", CategoryMatchId: "other"}}}},
		ListResponses: []*pb.PBListResponse{{
			ListId: "list-1",
			CategoryGroupResponses: []*pb.PBListCategoryGroupResponse{{
				CategoryGroup: &pb.PBListCategoryGroup{Categories: []*pb.PBListCategory{{Identifier: "hashed-other", Name: "Other", SystemCategory: "other"}}},
			}},
		}},
	}}
	if got, err := resolveLiveCategory(data, "list-1", "Other"); err != nil || got != "other" {
		t.Fatalf("resolveLiveCategory Other = %q, %v; want other, nil", got, err)
	}
}

func TestResolveLiveCategoryHonorsExplicitMatchIDWhenObjectIDIsOpaque(t *testing.T) {
	t.Parallel()

	data := &pb.PBUserDataResponse{ShoppingListsResponse: &pb.ShoppingListsResponse{
		ListResponses: []*pb.PBListResponse{{
			ListId: "list-1",
			CategoryGroupResponses: []*pb.PBListCategoryGroupResponse{{
				CategoryGroup: &pb.PBListCategoryGroup{Categories: []*pb.PBListCategory{{Identifier: "opaque-id", Name: "Pantry Aisle"}}},
			}},
		}},
	}}
	if got, err := resolveLiveCategory(data, "list-1", "pantry-aisle"); err != nil || got != "pantry-aisle" {
		t.Fatalf("resolveLiveCategory explicit match ID = %q, %v; want pantry-aisle, nil", got, err)
	}
}

func TestVerifyLiveItemMetadataChecksCategoryAndStoreChanges(t *testing.T) {
	t.Parallel()

	item := &pb.ListItem{CategoryMatchId: "cat-1", StoreIds: []string{"store-1", "store-2"}}
	if err := verifyLiveItemMetadata(item, "cat-1", true, "store-2", ""); err != nil {
		t.Fatalf("verifyLiveItemMetadata matching values: %v", err)
	}
	if err := verifyLiveItemMetadata(item, "cat-1", true, "", "store-1"); err == nil {
		t.Fatal("verifyLiveItemMetadata accepted a store still assigned")
	}
	if err := verifyLiveItemMetadata(item, "cat-2", true, "", ""); err == nil {
		t.Fatal("verifyLiveItemMetadata accepted a category mismatch")
	}
}

func TestVerifyCachedItemMetadataChecksSyncedFields(t *testing.T) {
	t.Parallel()

	item := &store.ItemRow{CategoryMatchID: "cat-1", StoreIDs: []string{"store-1"}}
	if err := verifyCachedItemMetadata(item, "cat-1", true, "store-1", ""); err != nil {
		t.Fatalf("verifyCachedItemMetadata matching values: %v", err)
	}
	if err := verifyCachedItemMetadata(item, "cat-1", true, "", "store-1"); err == nil {
		t.Fatal("verifyCachedItemMetadata accepted a store still assigned")
	}
}

func TestVerifyItemPriceAndClear(t *testing.T) {
	t.Parallel()

	price := &pb.PBItemPrice{Amount: 3.49, Date: "2026-08-16"}
	item := &pb.ListItem{Prices: []*pb.PBItemPrice{price}}
	if err := verifyLiveItemPrice(item, price, false); err != nil {
		t.Fatalf("verifyLiveItemPrice matching value: %v", err)
	}
	if err := verifyLiveItemPrice(item, &pb.PBItemPrice{Amount: 2.99}, false); err == nil {
		t.Fatal("verifyLiveItemPrice accepted a mismatched amount")
	}
	if err := verifyLiveItemPrice(&pb.ListItem{Prices: []*pb.PBItemPrice{{Amount: 0}}}, &pb.PBItemPrice{Amount: 0}, true); err != nil {
		t.Fatalf("verifyLiveItemPrice accepted a cleared price: %v", err)
	}
	withOtherStore := &pb.ListItem{Prices: []*pb.PBItemPrice{
		{Amount: 0, StoreId: "cedar-market"},
		{Amount: 2.10, StoreId: "paris-walmart"},
	}}
	if err := verifyLiveItemPrice(withOtherStore, &pb.PBItemPrice{StoreId: "cedar-market"}, true); err != nil {
		t.Fatalf("verifyLiveItemPrice rejected a targeted store clear: %v", err)
	}
	if err := verifyLiveItemPrice(withOtherStore, &pb.PBItemPrice{}, true); err == nil {
		t.Fatal("verifyLiveItemPrice accepted an all-store clear while positive prices remained")
	}
	cached := &store.ItemRow{Prices: []*pb.PBItemPrice{
		{Amount: 0, StoreId: "cedar-market"},
		{Amount: 2.10, StoreId: "paris-walmart"},
	}}
	if err := verifyCachedItemPrice(cached, &pb.PBItemPrice{StoreId: "cedar-market"}, true); err != nil {
		t.Fatalf("verifyCachedItemPrice rejected a targeted store clear: %v", err)
	}
	if err := verifyCachedItemPrice(cached, &pb.PBItemPrice{}, true); err == nil {
		t.Fatal("verifyCachedItemPrice accepted an all-store clear while positive prices remained")
	}
}

func TestPriceClearTargetsCoverEveryExistingStore(t *testing.T) {
	t.Parallel()

	item := &pb.ListItem{Prices: []*pb.PBItemPrice{
		{Amount: 4.25, StoreId: "cedar-market"},
		{Amount: 2.10, StoreId: "paris-walmart"},
		{Amount: 3.00, StoreId: "cedar-market"},
	}}
	targets := priceClearTargets(item, "")
	if len(targets) != 2 || targets[0].GetStoreId() != "cedar-market" || targets[1].GetStoreId() != "paris-walmart" {
		t.Fatalf("priceClearTargets = %#v, want one target per unique store", targets)
	}
	targets = priceClearTargets(item, " paris-walmart ")
	if len(targets) != 1 || targets[0].GetStoreId() != "paris-walmart" {
		t.Fatalf("targeted priceClearTargets = %#v, want paris-walmart", targets)
	}
}

func TestItemsUpdateMetadataDefaultsToOfflinePreview(t *testing.T) {
	t.Parallel()

	flags := &rootFlags{asJSON: true}
	cmd := newItemsUpdateCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--list", "Groceries", "--item", "Milk", "--category", "Produce"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("preview returned error: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("preview output is not JSON: %v; output=%q", err, out.String())
	}
	if result["dry_run"] != true || result["category"] != "Produce" || result["apply"] != false {
		t.Fatalf("preview result = %#v, want dry_run/category/apply fields", result)
	}
}
