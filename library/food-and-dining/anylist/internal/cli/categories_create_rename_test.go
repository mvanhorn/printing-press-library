// Copyright 2026 Jeeves and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
)

// categoryTestUserData builds a fresh user-data response for one list with the
// given category groups, plus the list itself in the shopping-list response so
// name resolution works the way it does on a live read.
func categoryTestUserData(listID, listName string, groups ...*pb.PBListCategoryGroup) *pb.PBUserDataResponse {
	groupResponses := make([]*pb.PBListCategoryGroupResponse, 0, len(groups))
	for _, group := range groups {
		groupResponses = append(groupResponses, &pb.PBListCategoryGroupResponse{CategoryGroup: group})
	}
	return &pb.PBUserDataResponse{
		ShoppingListsResponse: &pb.ShoppingListsResponse{
			NewLists: []*pb.ShoppingList{{Identifier: listID, Name: listName}},
			ListResponses: []*pb.PBListResponse{{
				ListId:                 listID,
				CategoryGroupResponses: groupResponses,
			}},
		},
	}
}

func TestCategoriesCreatePreviewDoesNotRequireAuth(t *testing.T) {
	t.Parallel()
	flags := &rootFlags{}
	cmd := newCategoriesCreateCmd(flags)
	cmd.SetArgs([]string{"--list", "Groceries", "--name", "Pantry Aisle"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("preview returned error: %v", err)
	}
}

func TestCategoriesCreatePreviewJSONIsDryRun(t *testing.T) {
	t.Parallel()
	flags := &rootFlags{asJSON: true}
	cmd := newCategoriesCreateCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--list", "Groceries", "--name", "Pantry Aisle", "--category-group", "Aisles"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("preview returned error: %v", err)
	}
	var preview map[string]any
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatalf("preview output is not JSON: %v (%s)", err, out.String())
	}
	if preview["dry_run"] != true {
		t.Errorf("dry_run = %v, want true", preview["dry_run"])
	}
	if preview["apply"] != false {
		t.Errorf("apply = %v, want false", preview["apply"])
	}
}

func TestCategoriesRenamePreviewDoesNotRequireAuth(t *testing.T) {
	t.Parallel()
	flags := &rootFlags{}
	cmd := newCategoriesRenameCmd(flags)
	cmd.SetArgs([]string{"--list", "Groceries", "--category", "Pantry Aisle", "--new-name", "Pantry"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("preview returned error: %v", err)
	}
}

// --apply must fail closed before any network activity when the CLI is not
// authenticated; nothing may be written with a missing or broken config.
func TestCategoriesApplyFailsClosedWithoutAuth(t *testing.T) {
	t.Parallel()
	for name, run := range map[string]func() error{
		"create": func() error {
			cmd := newCategoriesCreateCmd(&rootFlags{})
			cmd.SetArgs([]string{"--list", "Groceries", "--name", "Pantry Aisle", "--apply"})
			return cmd.Execute()
		},
		"rename": func() error {
			cmd := newCategoriesRenameCmd(&rootFlags{})
			cmd.SetArgs([]string{"--list", "Groceries", "--category", "Pantry Aisle", "--new-name", "Pantry", "--apply"})
			return cmd.Execute()
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := run(); err == nil {
				t.Fatalf("%s --apply without config returned nil; must fail closed", name)
			}
		})
	}
}

func TestResolveCategoryListRecordByIDAndExactName(t *testing.T) {
	t.Parallel()
	data := categoryTestUserData("list-1", "Groceries")
	if list, err := resolveCategoryListRecord(data, "list-1"); err != nil || list.GetIdentifier() != "list-1" {
		t.Fatalf("resolve by ID = %v, %v", list, err)
	}
	if list, err := resolveCategoryListRecord(data, "groceries"); err != nil || list.GetIdentifier() != "list-1" {
		t.Fatalf("resolve by name = %v, %v", list, err)
	}
	if _, err := resolveCategoryListRecord(data, "Missing"); err == nil {
		t.Fatal("resolve missing list returned nil error")
	}
	ambiguous := &pb.PBUserDataResponse{ShoppingListsResponse: &pb.ShoppingListsResponse{NewLists: []*pb.ShoppingList{
		{Identifier: "a", Name: "Groceries"},
		{Identifier: "b", Name: " groceries "},
	}}}
	if _, err := resolveCategoryListRecord(ambiguous, "Groceries"); err == nil {
		t.Fatal("resolve ambiguous list name returned nil error")
	}
}

func TestSelectCategoryGroupForCreate(t *testing.T) {
	t.Parallel()
	group1 := &pb.PBListCategoryGroup{Identifier: "group-1", Name: "Aisles", ListId: "list-1", Categories: []*pb.PBListCategory{{Identifier: "produce", Name: "Produce", SortIndex: 0}}}
	group2 := &pb.PBListCategoryGroup{Identifier: "group-2", Name: "Stores", ListId: "list-1", Categories: []*pb.PBListCategory{}}

	// Exactly one group resolves without a selector.
	single := categoryTestUserData("list-1", "Groceries", group1)
	if group, err := selectCategoryGroupForCreate(single, "list-1", ""); err != nil || group.GetIdentifier() != "group-1" {
		t.Fatalf("single group = %v, %v", group, err)
	}

	multi := categoryTestUserData("list-1", "Groceries", group1, group2)
	if _, err := selectCategoryGroupForCreate(multi, "list-1", ""); err == nil {
		t.Fatal("multiple groups without --category-group must fail")
	}
	if group, err := selectCategoryGroupForCreate(multi, "list-1", "group-2"); err != nil || group.GetIdentifier() != "group-2" {
		t.Fatalf("resolve by group ID = %v, %v", group, err)
	}
	if group, err := selectCategoryGroupForCreate(multi, "list-1", "aisles"); err != nil || group.GetIdentifier() != "group-1" {
		t.Fatalf("resolve by group name = %v, %v", group, err)
	}
	if _, err := selectCategoryGroupForCreate(multi, "list-1", "Nowhere"); err == nil {
		t.Fatal("unknown group name must fail")
	}

	duplicateName := categoryTestUserData("list-1", "Groceries",
		&pb.PBListCategoryGroup{Identifier: "g1", Name: "Aisles", ListId: "list-1"},
		&pb.PBListCategoryGroup{Identifier: "g2", Name: "Aisles", ListId: "list-1"})
	if _, err := selectCategoryGroupForCreate(duplicateName, "list-1", "Aisles"); err == nil {
		t.Fatal("ambiguous group name must fail")
	}
}

func TestNewCategoryStableIDUsesOpaqueAnyListShape(t *testing.T) {
	t.Parallel()
	existing := []*pb.PBListCategory{{Identifier: "00000000000000000000000000000000"}}
	id := newCategoryStableID("Pantry Aisle", existing)
	if len(id) != 32 {
		t.Fatalf("generated ID length = %d, want 32", len(id))
	}
	if id == existing[0].GetIdentifier() {
		t.Fatalf("generated ID collided with existing category: %q", id)
	}
	for _, r := range id {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			t.Fatalf("generated ID = %q, want lowercase hexadecimal", id)
		}
	}
}

func TestNextCategorySortIndex(t *testing.T) {
	t.Parallel()
	if got := nextCategorySortIndex(nil); got != 0 {
		t.Errorf("empty group sort index = %d, want 0", got)
	}
	if got := nextCategorySortIndex([]*pb.PBListCategory{{SortIndex: 0}, {SortIndex: 2}, {SortIndex: 1}}); got != 3 {
		t.Errorf("next sort index = %d, want 3", got)
	}
}

func TestResolveCategoryRecordInListExactOrAmbiguous(t *testing.T) {
	t.Parallel()
	data := categoryTestUserData("list-1", "Groceries",
		&pb.PBListCategoryGroup{Identifier: "group-1", Name: "Aisles", ListId: "list-1", Categories: []*pb.PBListCategory{
			{Identifier: "pantry-aisle", Name: "Pantry Aisle", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 0},
		}},
		&pb.PBListCategoryGroup{Identifier: "group-2", Name: "Stores", ListId: "list-1", Categories: []*pb.PBListCategory{
			{Identifier: "milk", Name: "Dairy", CategoryGroupId: "group-2", ListId: "list-1", SortIndex: 0},
		}},
	)
	if category, err := resolveCategoryRecordInList(data, "list-1", "pantry-aisle"); err != nil || category.GetName() != "Pantry Aisle" {
		t.Fatalf("resolve by stable ID = %v, %v", category, err)
	}
	if category, err := resolveCategoryRecordInList(data, "list-1", "DAIRY"); err != nil || category.GetIdentifier() != "milk" {
		t.Fatalf("resolve by exact name = %v, %v", category, err)
	}
	if _, err := resolveCategoryRecordInList(data, "list-1", "Dairy Shelf"); err == nil {
		t.Fatal("missing category name must fail")
	}
	duplicate := categoryTestUserData("list-1", "Groceries",
		&pb.PBListCategoryGroup{Identifier: "group-1", ListId: "list-1", Categories: []*pb.PBListCategory{{Identifier: "a", Name: "Dairy"}}},
		&pb.PBListCategoryGroup{Identifier: "group-2", ListId: "list-1", Categories: []*pb.PBListCategory{{Identifier: "b", Name: "Dairy"}}})
	if _, err := resolveCategoryRecordInList(duplicate, "list-1", "Dairy"); err == nil {
		t.Fatal("ambiguous category name must fail")
	}
}

func TestCategoryNameConflictInList(t *testing.T) {
	t.Parallel()
	data := categoryTestUserData("list-1", "Groceries",
		&pb.PBListCategoryGroup{Identifier: "group-1", ListId: "list-1", Categories: []*pb.PBListCategory{{Identifier: "pantry-aisle", Name: "Pantry Aisle"}}})
	if conflict := categoryNameConflictInList(data, "list-1", "pantry aisle", ""); conflict == nil {
		t.Fatal("existing name (case-insensitive) must be a conflict")
	}
	if conflict := categoryNameConflictInList(data, "list-1", "pantry aisle", "pantry-aisle"); conflict != nil {
		t.Fatalf("self must not conflict with itself, got %v", conflict.GetIdentifier())
	}
	if conflict := categoryNameConflictInList(data, "list-1", "Freezer", ""); conflict != nil {
		t.Fatalf("fresh name must not conflict, got %v", conflict.GetIdentifier())
	}
}

func TestVerifyLiveCategoryCreate(t *testing.T) {
	t.Parallel()
	expected := &pb.PBListCategory{
		Identifier:      "pantry-aisle",
		ListId:          "list-1",
		CategoryGroupId: "group-1",
		Name:            "Pantry Aisle",
		SortIndex:       1,
	}
	ok := categoryTestUserData("list-1", "Groceries",
		&pb.PBListCategoryGroup{Identifier: "group-1", ListId: "list-1", Categories: []*pb.PBListCategory{{Identifier: "pantry-aisle", Name: "Pantry Aisle", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 1}}})
	if found, err := verifyLiveCategoryCreate(ok, "list-1", expected); err != nil || found.GetIdentifier() != "pantry-aisle" {
		t.Fatalf("successful verification = %v, %v", found, err)
	}

	missing := categoryTestUserData("list-1", "Groceries", &pb.PBListCategoryGroup{Identifier: "group-1", ListId: "list-1"})
	if _, err := verifyLiveCategoryCreate(missing, "list-1", expected); err == nil || !strings.Contains(err.Error(), "create verification failed") {
		t.Fatalf("missing category must fail verification: %v", err)
	}
	wrongName := categoryTestUserData("list-1", "Groceries",
		&pb.PBListCategoryGroup{Identifier: "group-1", ListId: "list-1", Categories: []*pb.PBListCategory{{Identifier: "pantry-aisle", Name: "Something Else", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 1}}})
	if _, err := verifyLiveCategoryCreate(wrongName, "list-1", expected); err == nil {
		t.Fatal("wrong read-back name must fail verification")
	}
	wrongGroup := categoryTestUserData("list-1", "Groceries",
		&pb.PBListCategoryGroup{Identifier: "group-2", ListId: "list-1", Categories: []*pb.PBListCategory{{Identifier: "pantry-aisle", Name: "Pantry Aisle", CategoryGroupId: "group-2", ListId: "list-1", SortIndex: 1}}})
	if _, err := verifyLiveCategoryCreate(wrongGroup, "list-1", expected); err == nil {
		t.Fatal("wrong read-back group must fail verification")
	}
	wrongSort := categoryTestUserData("list-1", "Groceries",
		&pb.PBListCategoryGroup{Identifier: "group-1", ListId: "list-1", Categories: []*pb.PBListCategory{{Identifier: "pantry-aisle", Name: "Pantry Aisle", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 9}}})
	if _, err := verifyLiveCategoryCreate(wrongSort, "list-1", expected); err == nil {
		t.Fatal("wrong read-back sort index must fail verification")
	}
}

func TestVerifyLiveCategoryRename(t *testing.T) {
	t.Parallel()
	original := &pb.PBListCategory{
		Identifier:      "pantry-aisle",
		ListId:          "list-1",
		CategoryGroupId: "group-1",
		Name:            "Pantry Aisle",
		SortIndex:       1,
	}
	updated := &pb.PBListCategory{
		Identifier:      "pantry-aisle",
		ListId:          "list-1",
		CategoryGroupId: "group-1",
		Name:            "Pantry",
		SortIndex:       1,
	}
	ok := categoryTestUserData("list-1", "Groceries",
		&pb.PBListCategoryGroup{Identifier: "group-1", ListId: "list-1", Categories: []*pb.PBListCategory{{Identifier: "pantry-aisle", Name: "Pantry", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 1}}})
	if found, err := verifyLiveCategoryRename(ok, "list-1", original, updated); err != nil || found.GetName() != "Pantry" {
		t.Fatalf("successful verification = %v, %v", found, err)
	}

	unchanged := categoryTestUserData("list-1", "Groceries",
		&pb.PBListCategoryGroup{Identifier: "group-1", ListId: "list-1", Categories: []*pb.PBListCategory{{Identifier: "pantry-aisle", Name: "Pantry Aisle", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 1}}})
	if _, err := verifyLiveCategoryRename(unchanged, "list-1", original, updated); err == nil || !strings.Contains(err.Error(), "rename verification failed") {
		t.Fatalf("unchanged read-back name must fail verification: %v", err)
	}
	groupDrift := categoryTestUserData("list-1", "Groceries",
		&pb.PBListCategoryGroup{Identifier: "group-2", ListId: "list-1", Categories: []*pb.PBListCategory{{Identifier: "pantry-aisle", Name: "Pantry", CategoryGroupId: "group-2", ListId: "list-1", SortIndex: 1}}})
	if _, err := verifyLiveCategoryRename(groupDrift, "list-1", original, updated); err == nil {
		t.Fatal("read-back group drift must fail verification")
	}
	sortDrift := categoryTestUserData("list-1", "Groceries",
		&pb.PBListCategoryGroup{Identifier: "group-1", ListId: "list-1", Categories: []*pb.PBListCategory{{Identifier: "pantry-aisle", Name: "Pantry", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 4}}})
	if _, err := verifyLiveCategoryRename(sortDrift, "list-1", original, updated); err == nil {
		t.Fatal("read-back sort index drift must fail verification")
	}
	missing := categoryTestUserData("list-1", "Groceries", &pb.PBListCategoryGroup{Identifier: "group-1", ListId: "list-1"})
	if _, err := verifyLiveCategoryRename(missing, "list-1", original, updated); err == nil {
		t.Fatal("missing category must fail verification")
	}
}
