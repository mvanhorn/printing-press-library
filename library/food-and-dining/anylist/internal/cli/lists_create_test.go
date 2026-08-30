package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
)

func TestVerifyLiveShoppingListRequiresMatchingReadBack(t *testing.T) {
	t.Parallel()

	requested := &pb.ShoppingList{Identifier: "created-list", Name: "Created List"}
	data := &pb.PBUserDataResponse{
		ShoppingListsResponse: &pb.ShoppingListsResponse{
			NewLists: []*pb.ShoppingList{{Identifier: "created-list", Name: "Created List"}},
		},
	}

	created, err := verifyLiveShoppingList(data, requested)
	if err != nil {
		t.Fatalf("verifyLiveShoppingList returned error: %v", err)
	}
	if created.GetIdentifier() != requested.GetIdentifier() {
		t.Fatalf("created ID = %q, want %q", created.GetIdentifier(), requested.GetIdentifier())
	}

	if _, err := verifyLiveShoppingList(&pb.PBUserDataResponse{}, requested); err == nil {
		t.Fatal("verifyLiveShoppingList returned nil for a missing read-back list")
	}
	if _, err := verifyLiveShoppingList(&pb.PBUserDataResponse{
		ShoppingListsResponse: &pb.ShoppingListsResponse{
			NewLists: []*pb.ShoppingList{{Identifier: "created-list", Name: "Different List"}},
		},
	}, requested); err == nil {
		t.Fatal("verifyLiveShoppingList returned nil for a mismatched read-back name")
	}
}
