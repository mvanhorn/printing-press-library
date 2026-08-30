// Copyright 2026 Jeeves and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
)

func TestResolveCategoryForTargetNoMatch(t *testing.T) {
	t.Parallel()

	// Target list has no categories at all.
	data := &pb.PBUserDataResponse{
		ShoppingListsResponse: &pb.ShoppingListsResponse{
			ListResponses: []*pb.PBListResponse{{
				ListId:                 "target-1",
				CategoryGroupResponses: []*pb.PBListCategoryGroupResponse{},
			}},
		},
	}

	_, err := resolveCategoryForTarget(data, "target-1", "pantry")
	if err == nil {
		t.Fatal("expected error for missing category")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %q, want 'not found'", err)
	}
}

func TestResolveCategoryForTargetExactNameMatch(t *testing.T) {
	t.Parallel()

	data := &pb.PBUserDataResponse{
		ShoppingListsResponse: &pb.ShoppingListsResponse{
			ListResponses: []*pb.PBListResponse{{
				ListId: "target-1",
				CategoryGroupResponses: []*pb.PBListCategoryGroupResponse{{
					CategoryGroup: &pb.PBListCategoryGroup{
						Categories: []*pb.PBListCategory{{
							Name:       "Pantry",
							Identifier: "pantry-id",
						}},
					},
				}},
			}},
		},
	}

	got, err := resolveCategoryForTarget(data, "target-1", "Pantry")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Pantry" {
		t.Fatalf("resolved category = %q, want %q", got, "Pantry")
	}
}

func TestResolveCategoryForTargetIdentifierMatch(t *testing.T) {
	t.Parallel()

	data := &pb.PBUserDataResponse{
		ShoppingListsResponse: &pb.ShoppingListsResponse{
			ListResponses: []*pb.PBListResponse{{
				ListId: "target-1",
				CategoryGroupResponses: []*pb.PBListCategoryGroupResponse{{
					CategoryGroup: &pb.PBListCategoryGroup{
						Categories: []*pb.PBListCategory{{
							Name:       "Pantry",
							Identifier: "pantry-aisle",
						}},
					},
				}},
			}},
		},
	}

	got, err := resolveCategoryForTarget(data, "target-1", "pantry-aisle")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Pantry" {
		t.Fatalf("resolved category = %q, want %q", got, "Pantry")
	}
}

func TestResolveStoreForTargetNoMatch(t *testing.T) {
	t.Parallel()

	data := &pb.PBUserDataResponse{
		ShoppingListsResponse: &pb.ShoppingListsResponse{
			ListResponses: []*pb.PBListResponse{{
				ListId: "target-1",
				Stores: []*pb.PBStore{},
			}},
		},
	}

	_, err := resolveStoreForTarget(data, "target-1", "Walmart")
	if err == nil {
		t.Fatal("expected error for missing store")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %q, want 'not found'", err)
	}
}

func TestResolveStoreForTargetIdentifierMatch(t *testing.T) {
	t.Parallel()

	data := &pb.PBUserDataResponse{
		ShoppingListsResponse: &pb.ShoppingListsResponse{
			ListResponses: []*pb.PBListResponse{{
				ListId: "target-1",
				Stores: []*pb.PBStore{{
					Name:       "Walmart Supercenter",
					Identifier: "walmart",
				}},
			}},
		},
	}

	got, err := resolveStoreForTarget(data, "target-1", "walmart")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Walmart Supercenter" {
		t.Fatalf("resolved store = %q, want %q", got, "Walmart Supercenter")
	}
}

func TestResolveStoreForTargetNameMatch(t *testing.T) {
	t.Parallel()

	data := &pb.PBUserDataResponse{
		ShoppingListsResponse: &pb.ShoppingListsResponse{
			ListResponses: []*pb.PBListResponse{{
				ListId: "target-1",
				Stores: []*pb.PBStore{{
					Name:       "Target",
					Identifier: "target-id",
				}},
			}},
		},
	}

	got, err := resolveStoreForTarget(data, "target-1", "Target")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Target" {
		t.Fatalf("resolved store = %q, want %q", got, "Target")
	}
}

func TestUPCConflictDetection(t *testing.T) {
	t.Parallel()

	// Build live data where the target list already has an item with the same UPC.
	data := &pb.PBUserDataResponse{
		ShoppingListsResponse: &pb.ShoppingListsResponse{
			NewLists: []*pb.ShoppingList{
				{
					Identifier: "source-1",
					Items: []*pb.ListItem{{
						Identifier: "src-item-1",
						Name:       "Milk",
						ProductUpc: "012345678905",
					}},
				},
				{
					Identifier: "target-1",
					Items: []*pb.ListItem{{
						Identifier: "tgt-item-1",
						Name:       "Whole Milk",
						ProductUpc: "012345678905", // same UPC → conflict
					}},
				},
			},
		},
	}

	// Verify source item is in correct list.
	_, found := findLiveItemByID(data, "source-1", "src-item-1")
	if !found {
		t.Fatal("source item not found")
	}

	// Simulate what the recycle command would do for duplicate detection.
	liveSourceItem, _ := findLiveItemByID(data, "source-1", "src-item-1")
	liveTargetList, found := findLiveShoppingListByID(data, "target-1")
	if !found {
		t.Fatal("target list not found")
	}

	sourceUPC := liveSourceItem.GetProductUpc()
	for _, item := range liveTargetList.GetItems() {
		if item.GetProductUpc() != "" && sourceUPC != "" &&
			normalizedUPC(item.GetProductUpc()) == normalizedUPC(sourceUPC) {
			// This is the conflict path — UPC conflict detected.
			t.Logf("UPC conflict detected: %q in %q matches UPC %q",
				item.GetName(), "target-1", sourceUPC)
			return
		}
	}
	t.Fatal("expected UPC conflict not detected")
}

func TestExactTargetNameNoOpDetection(t *testing.T) {
	t.Parallel()

	data := &pb.PBUserDataResponse{
		ShoppingListsResponse: &pb.ShoppingListsResponse{
			NewLists: []*pb.ShoppingList{
				{
					Identifier: "source-1",
					Items: []*pb.ListItem{{
						Identifier:    "src-item-1",
						Name:          "Eggs",
						ProductUpc:    "",
						PackageSizePb: &pb.PBItemPackageSize{RawPackageSize: "12 count"},
					}},
				},
				{
					Identifier: "target-1",
					Items: []*pb.ListItem{{
						Identifier:    "tgt-item-1",
						Name:          "Eggs",
						ProductUpc:    "",
						PackageSizePb: &pb.PBItemPackageSize{RawPackageSize: "12 count"},
					}},
				},
			},
		},
	}

	liveSourceItem, _ := findLiveItemByID(data, "source-1", "src-item-1")
	liveTargetList, found := findLiveShoppingListByID(data, "target-1")
	if !found {
		t.Fatal("target list not found")
	}

	sourceName := liveSourceItem.GetName()
	sourcePS := formatPackageSize(liveSourceItem.GetPackageSizePb())

	var existingMatch string
	for _, item := range liveTargetList.GetItems() {
		if item.GetName() == sourceName &&
			normalizedPackageSize(item.GetPackageSizePb().GetRawPackageSize()) == normalizedPackageSize(sourcePS) {
			existingMatch = item.GetIdentifier()
			break
		}
	}

	if existingMatch == "" {
		t.Fatal("expected exact target no-op match not detected")
	}
	if existingMatch != "tgt-item-1" {
		t.Fatalf("existing match = %q, want %q", existingMatch, "tgt-item-1")
	}
}

func TestNoUPCFuzzyMatch(t *testing.T) {
	t.Parallel()

	// UPCs that are close but not identical should NOT trigger a conflict.
	data := &pb.PBUserDataResponse{
		ShoppingListsResponse: &pb.ShoppingListsResponse{
			NewLists: []*pb.ShoppingList{
				{
					Identifier: "source-1",
					Items: []*pb.ListItem{{
						Identifier: "src-item-1",
						Name:       "Milk",
						ProductUpc: "012345678905",
					}},
				},
				{
					Identifier: "target-1",
					Items: []*pb.ListItem{{
						Identifier: "tgt-item-1",
						Name:       "Whole Milk",
						ProductUpc: "012345678906", // off by one digit
					}},
				},
			},
		},
	}

	liveSourceItem, _ := findLiveItemByID(data, "source-1", "src-item-1")
	liveTargetList, found := findLiveShoppingListByID(data, "target-1")
	if !found {
		t.Fatal("target list not found")
	}

	sourceUPC := liveSourceItem.GetProductUpc()
	for _, item := range liveTargetList.GetItems() {
		if item.GetProductUpc() != "" && sourceUPC != "" &&
			normalizedUPC(item.GetProductUpc()) == normalizedUPC(sourceUPC) {
			t.Fatal("close UPC should not trigger conflict (no fuzzy matching)")
		}
	}
	// If we get here, no conflict was triggered — correct.
	t.Log("correctly did not treat close UPCs as a conflict")
}

func TestWrongListSourceItemRejection(t *testing.T) {
	t.Parallel()

	// An item ID exists in the store, but belongs to a different list.
	// Simulate via protobuf: findLiveItemByID should not match across lists.
	data := &pb.PBUserDataResponse{
		ShoppingListsResponse: &pb.ShoppingListsResponse{
			NewLists: []*pb.ShoppingList{
				{
					Identifier: "source-1",
					Items:      []*pb.ListItem{}, // source list has no items
				},
				{
					Identifier: "other-1",
					Items: []*pb.ListItem{{
						Identifier: "src-item-1",
						Name:       "Milk",
					}},
				},
			},
		},
	}

	// Looking for "src-item-1" in source-1 should not find it (it's in other-1).
	_, found := findLiveItemByID(data, "source-1", "src-item-1")
	if found {
		t.Fatal("findLiveItemByID should not match an item from a different list")
	}
}

func TestCheckedSourceItemRejection(t *testing.T) {
	t.Parallel()

	data := &pb.PBUserDataResponse{
		ShoppingListsResponse: &pb.ShoppingListsResponse{
			NewLists: []*pb.ShoppingList{
				{
					Identifier: "source-1",
					Items: []*pb.ListItem{{
						Identifier: "src-item-1",
						Name:       "Milk",
						Checked:    true, // already checked — recycle should reject
					}},
				},
			},
		},
	}

	liveSourceItem, found := findLiveItemByID(data, "source-1", "src-item-1")
	if !found {
		t.Fatal("source item not found")
	}
	if !liveSourceItem.GetChecked() {
		t.Fatal("expected source item to be checked")
	}
	// The actual rejection happens in the command handler; this test
	// verifies the source item is correctly identified as checked.
}

// --- normalizedName tests ---

func TestNormalizedNameTrimAndCollapse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
	}{
		{"  eggs  ", "eggs"},
		{"  Large Eggs  ", "large eggs"},
		{"   Large   Eggs   ", "large eggs"},
		{"\t\tLarge\tEggs\t\n", "large eggs"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := normalizedName(tt.in)
			if got != tt.want {
				t.Fatalf("normalizedName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizedNameCaseFold(t *testing.T) {
	t.Parallel()

	got := normalizedName("LArGe EgGs")
	if got != "large eggs" {
		t.Fatalf("normalizedName(\"LArGe EgGs\") = %q, want %q", got, "large eggs")
	}
}

func TestNormalizedNameEmpty(t *testing.T) {
	t.Parallel()

	if got := normalizedName(""); got != "" {
		t.Fatalf("normalizedName(\"\") = %q, want \"\"", got)
	}
	if got := normalizedName("  "); got != "" {
		t.Fatalf("normalizedName(\"  \") = %q, want \"\"", got)
	}
}

// --- live checked-source rejection test ---

func TestLiveCheckedSourceRejection(t *testing.T) {
	t.Parallel()

	// The source item is checked in live data.
	// The command handler rejects GetChecked() after GetUserData;
	// this test verifies the live source item is correctly identified as checked
	// so the handler will return the appropriate error.
	data := &pb.PBUserDataResponse{
		ShoppingListsResponse: &pb.ShoppingListsResponse{
			NewLists: []*pb.ShoppingList{{
				Identifier: "source-1",
				Items: []*pb.ListItem{{
					Identifier: "src-item-1",
					Name:       "Milk",
					Checked:    true, // live says checked — recycle must reject
				}},
			}},
		},
	}

	liveItem, found := findLiveItemByID(data, "source-1", "src-item-1")
	if !found {
		t.Fatal("source item not found in live data")
	}
	if !liveItem.GetChecked() {
		t.Fatal("expected live item to be checked")
	}
}
