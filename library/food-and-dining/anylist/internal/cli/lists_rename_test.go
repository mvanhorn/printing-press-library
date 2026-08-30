package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
)

func TestListsRenamePreviewDoesNotRequireAuth(t *testing.T) {
	t.Parallel()
	flags := &rootFlags{}
	cmd := newListsRenameCmd(flags)
	cmd.SetArgs([]string{"--name", "Groceries", "--new-name", "Weekly Groceries"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("preview returned error: %v", err)
	}
}

func TestExactLiveShoppingListByNameRejectsAmbiguity(t *testing.T) {
	t.Parallel()
	data := &pb.PBUserDataResponse{ShoppingListsResponse: &pb.ShoppingListsResponse{NewLists: []*pb.ShoppingList{
		{Identifier: "one", Name: "Groceries"},
		{Identifier: "two", Name: " groceries "},
	}}}
	if _, err := exactLiveShoppingListByName(data, "Groceries"); err == nil {
		t.Fatal("exactLiveShoppingListByName returned nil for ambiguous names")
	}
}
