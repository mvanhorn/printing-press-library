package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
)

func TestFindLiveShoppingListByIDSupportsMutationVerification(t *testing.T) {
	t.Parallel()

	data := &pb.PBUserDataResponse{
		ShoppingListsResponse: &pb.ShoppingListsResponse{
			NewLists:      []*pb.ShoppingList{{Identifier: "new-list", Name: "Groceries"}},
			ModifiedLists: []*pb.ShoppingList{{Identifier: "modified-list", Name: "Pantry"}},
		},
	}

	list, found := findLiveShoppingListByID(data, "modified-list")
	if !found || list.GetName() != "Pantry" {
		t.Fatalf("findLiveShoppingListByID = (%v, %v), want Pantry, true", list, found)
	}
	if _, found := findLiveShoppingListByID(data, "missing-list"); found {
		t.Fatal("findLiveShoppingListByID found a missing list")
	}
}

func TestFindLiveShoppingListByNamePreservesFuzzyLookup(t *testing.T) {
	t.Parallel()

	data := &pb.PBUserDataResponse{
		ShoppingListsResponse: &pb.ShoppingListsResponse{
			NewLists: []*pb.ShoppingList{{Identifier: "list-1", Name: "Weekly Groceries"}},
		},
	}

	list, err := findLiveShoppingListByName(data, "weekly gro")
	if err != nil {
		t.Fatalf("findLiveShoppingListByName returned error: %v", err)
	}
	if list.GetIdentifier() != "list-1" {
		t.Fatalf("list ID = %q, want list-1", list.GetIdentifier())
	}
}
