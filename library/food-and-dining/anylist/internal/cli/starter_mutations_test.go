package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
)

func TestResolveFavoriteStarterListByLinkedShoppingListName(t *testing.T) {
	data := &pb.PBUserDataResponse{
		ShoppingListsResponse: &pb.ShoppingListsResponse{NewLists: []*pb.ShoppingList{{Identifier: "shop-1", Name: "Groceries"}}},
		StarterListsResponse:  &pb.StarterListsResponseV2{FavoriteItemListsResponse: &pb.StarterListBatchResponse{ListResponses: []*pb.StarterListResponse{{StarterList: &pb.StarterList{Identifier: "fav-1", ListId: "shop-1", Name: "Favorite Items"}}}}},
	}
	list, err := resolveStarterList(data, starterListFavorites, "groceries")
	if err != nil {
		t.Fatalf("resolveStarterList returned error: %v", err)
	}
	if list.GetIdentifier() != "fav-1" {
		t.Fatalf("resolved starter ID = %q, want fav-1", list.GetIdentifier())
	}
}

func TestResolveStarterListRejectsAmbiguousNames(t *testing.T) {
	data := &pb.PBUserDataResponse{StarterListsResponse: &pb.StarterListsResponseV2{UserListsResponse: &pb.StarterListBatchResponse{ListResponses: []*pb.StarterListResponse{
		{StarterList: &pb.StarterList{Identifier: "one", Name: "Weeknight"}},
		{StarterList: &pb.StarterList{Identifier: "two", Name: " weeknight "}},
	}}}}
	if _, err := resolveStarterList(data, starterListUser, "Weeknight"); err == nil {
		t.Fatal("resolveStarterList accepted an ambiguous starter name")
	}
}

func TestStarterItemBySelectorRejectsDuplicateNames(t *testing.T) {
	list := &pb.StarterList{Identifier: "starter-1", Name: "Weeknight", Items: []*pb.ListItem{
		{Identifier: "one", Name: "Milk"},
		{Identifier: "two", Name: " milk "},
	}}
	if _, err := starterItemBySelector(list, "Milk"); err == nil {
		t.Fatal("starterItemBySelector accepted duplicate item names")
	}
}

func TestStarterMutationPreviewDoesNotRequireAuth(t *testing.T) {
	flags := &rootFlags{}
	cmd := newStarterListAddCmd(flags, starterListFavorites)
	cmd.SetArgs([]string{"--list", "Groceries", "--name", "Milk"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("preview returned error: %v", err)
	}
}
