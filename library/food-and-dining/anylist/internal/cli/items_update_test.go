// Copyright 2026 Jeeves and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/store"
)

func TestVerifyItemUpdateIncludesBarcode(t *testing.T) {
	t.Parallel()

	updated := &store.ItemRow{
		Quantity:   "2",
		Details:    "fresh",
		ProductUpc: "049000028904",
	}
	err := verifyItemUpdate(updated, map[string]string{
		"quantity":    "2",
		"details":     "fresh",
		"product_upc": "049000028904",
	})
	if err != nil {
		t.Fatalf("verifyItemUpdate returned error for matching fields: %v", err)
	}
}

func TestVerifyItemUpdateIncludesName(t *testing.T) {
	t.Parallel()

	updated := &store.ItemRow{Name: "Renamed Milk"}
	if err := verifyItemUpdate(updated, map[string]string{"name": "Renamed Milk"}); err != nil {
		t.Fatalf("verifyItemUpdate returned error for matching name: %v", err)
	}
}

func TestVerifyItemUpdateRejectsNameMismatch(t *testing.T) {
	t.Parallel()

	err := verifyItemUpdate(&store.ItemRow{Name: "Milk"}, map[string]string{"name": "Renamed Milk"})
	if err == nil {
		t.Fatal("verifyItemUpdate returned nil for mismatched name")
	}
	if !strings.Contains(err.Error(), "name") || !strings.Contains(err.Error(), "Renamed Milk") {
		t.Fatalf("error = %q, want name verification detail", err)
	}
}

func TestVerifyItemUpdateRejectsBarcodeMismatch(t *testing.T) {
	t.Parallel()

	err := verifyItemUpdate(&store.ItemRow{ProductUpc: "000000000000"}, map[string]string{
		"product_upc": "049000028904",
	})
	if err == nil {
		t.Fatal("verifyItemUpdate returned nil for mismatched barcode")
	}
	if !strings.Contains(err.Error(), "barcode") || !strings.Contains(err.Error(), "049000028904") {
		t.Fatalf("error = %q, want barcode verification detail", err)
	}
}

func TestVerifyItemUpdateIncludesPackageSize(t *testing.T) {
	t.Parallel()

	updated := &store.ItemRow{
		PackageSize: &pb.PBItemPackageSize{RawPackageSize: "12 oz carton"},
	}
	if err := verifyItemUpdate(updated, map[string]string{"package_size": "12 oz carton"}); err != nil {
		t.Fatalf("verifyItemUpdate returned error for matching package size: %v", err)
	}
}

func TestVerifyItemUpdateRejectsPackageSizeMismatch(t *testing.T) {
	t.Parallel()

	err := verifyItemUpdate(&store.ItemRow{
		PackageSize: &pb.PBItemPackageSize{RawPackageSize: "1 gallon jug"},
	}, map[string]string{"package_size": "12 oz carton"})
	if err == nil {
		t.Fatal("verifyItemUpdate returned nil for mismatched package size")
	}
	if !strings.Contains(err.Error(), "package size") || !strings.Contains(err.Error(), "12 oz carton") {
		t.Fatalf("error = %q, want package-size verification detail", err)
	}
}

func TestFindLiveItemByIDRequiresMatchingListAndItem(t *testing.T) {
	t.Parallel()

	data := &pb.PBUserDataResponse{
		ShoppingListsResponse: &pb.ShoppingListsResponse{
			NewLists: []*pb.ShoppingList{{
				Identifier: "list-1",
				Items:      []*pb.ListItem{{Identifier: "item-1", Name: "Milk"}},
			}},
		},
	}
	item, found := findLiveItemByID(data, "list-1", "item-1")
	if !found || item.GetName() != "Milk" {
		t.Fatalf("findLiveItemByID = %#v, %v; want Milk, true", item, found)
	}
	if _, found := findLiveItemByID(data, "wrong-list", "item-1"); found {
		t.Fatal("findLiveItemByID matched an item from the wrong list")
	}
}
