// Copyright 2026 Jeeves and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
)

func categoryDeleteReorderGroup() *pb.PBListCategoryGroup {
	return &pb.PBListCategoryGroup{
		Identifier: "group-1",
		ListId:     "list-1",
		Name:       "Aisles",
		Categories: []*pb.PBListCategory{
			{Identifier: "produce", Name: "Produce", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 0},
			{Identifier: "dairy-id", Name: "Dairy", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 1},
			{Identifier: "pantry-aisle", Name: "Pantry Aisle", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 2},
		},
	}
}

func TestCategoriesDeletePreviewDoesNotRequireAuth(t *testing.T) {
	t.Parallel()
	flags := &rootFlags{}
	cmd := newCategoriesDeleteCmd(flags)
	cmd.SetArgs([]string{"--list", "Groceries", "--category", "Pantry Aisle"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("preview returned error: %v", err)
	}
}

func TestCategoriesDeletePreviewJSONIsDryRun(t *testing.T) {
	t.Parallel()
	flags := &rootFlags{asJSON: true}
	cmd := newCategoriesDeleteCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--list", "Groceries", "--category", "pantry-aisle"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("preview returned error: %v", err)
	}
	var preview map[string]any
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatalf("preview output is not JSON: %v (%s)", err, out.String())
	}
	if preview["dry_run"] != true || preview["apply"] != false {
		t.Errorf("preview = %v, want dry_run=true apply=false", preview)
	}
}

func TestCategoriesReorderPreviewDoesNotRequireAuth(t *testing.T) {
	t.Parallel()
	flags := &rootFlags{}
	cmd := newCategoriesReorderCmd(flags)
	cmd.SetArgs([]string{"--list", "Groceries", "--order", "Dairy,Produce,Pantry Aisle"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("preview returned error: %v", err)
	}
}

func TestCategoriesReorderPreviewJSONIsDryRun(t *testing.T) {
	t.Parallel()
	flags := &rootFlags{asJSON: true}
	cmd := newCategoriesReorderCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--list", "Groceries", "--category-group", "Aisles", "--order", "Dairy,Produce,Pantry Aisle"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("preview returned error: %v", err)
	}
	var preview map[string]any
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatalf("preview output is not JSON: %v (%s)", err, out.String())
	}
	if preview["dry_run"] != true || preview["apply"] != false {
		t.Errorf("preview = %v, want dry_run=true apply=false", preview)
	}
	order, ok := preview["order"].([]any)
	if !ok || len(order) != 3 || order[0] != "Dairy" || order[2] != "Pantry Aisle" {
		t.Errorf("preview order = %v, want [Dairy Produce Pantry Aisle]", preview["order"])
	}
}

// --apply must fail closed before any network activity when the CLI is not
// authenticated; nothing may be written with a missing or broken config.
func TestCategoriesDeleteReorderApplyFailsClosedWithoutAuth(t *testing.T) {
	t.Parallel()
	for name, run := range map[string]func() error{
		"delete": func() error {
			cmd := newCategoriesDeleteCmd(&rootFlags{})
			cmd.SetArgs([]string{"--list", "Groceries", "--category", "Pantry Aisle", "--apply"})
			return cmd.Execute()
		},
		"reorder": func() error {
			cmd := newCategoriesReorderCmd(&rootFlags{})
			cmd.SetArgs([]string{"--list", "Groceries", "--order", "Dairy,Produce,Pantry Aisle", "--apply"})
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

// System categories carry a systemCategory marker (e.g. the default Other
// category); only custom categories may be deleted.
func TestCategoriesDeleteSystemCategoryProtection(t *testing.T) {
	t.Parallel()
	if err := ensureDeletableCategory(&pb.PBListCategory{Identifier: "other", Name: "Other", SystemCategory: "other"}); err == nil {
		t.Fatal("system category (systemCategory=other) must be rejected")
	}
	if err := ensureDeletableCategory(&pb.PBListCategory{Identifier: "grocery-1", Name: "Grocery", SystemCategory: "grocery"}); err == nil {
		t.Fatal("system category (systemCategory=grocery) must be rejected")
	}
	if err := ensureDeletableCategory(&pb.PBListCategory{Identifier: "pantry-aisle", Name: "Pantry Aisle"}); err != nil {
		t.Fatalf("custom category must be deletable, got: %v", err)
	}
}

func TestCategoriesDeleteResolvesCategoryGroup(t *testing.T) {
	t.Parallel()
	group := categoryDeleteReorderGroup()
	data := categoryTestUserData("list-1", "Groceries", group)
	second := group.Categories[1]
	found, err := findCategoryGroupForCategory(data, "list-1", second)
	if err != nil || found.GetIdentifier() != "group-1" {
		t.Fatalf("find group = %v, %v", found, err)
	}
	absent := &pb.PBListCategory{Identifier: "missing-id", Name: "Missing"}
	if _, err := findCategoryGroupForCategory(data, "list-1", absent); err == nil {
		t.Fatal("category not in any group must fail closed")
	}
}

func TestCategoriesReorderResolutionExactOrRejected(t *testing.T) {
	t.Parallel()
	group := categoryDeleteReorderGroup()

	// Happy path: stable IDs and exact names both resolve.
	ordered, err := resolveCategoryReorderInGroup(group, []string{"dairy-id", "PRODUCE", "Pantry Aisle"})
	if err != nil {
		t.Fatalf("happy path: %v", err)
	}
	gotIDs := orderedCategoryIDs(ordered)
	wantIDs := []string{"dairy-id", "produce", "pantry-aisle"}
	for i := range wantIDs {
		if gotIDs[i] != wantIDs[i] {
			t.Fatalf("resolved order = %v, want %v", gotIDs, wantIDs)
		}
	}

	cases := map[string][]string{
		"duplicate":    {"produce", "produce", "produce"},
		"missing":     {"produce", "dairy-id"},
		"appended":    {"produce", "dairy-id", "pantry-aisle", "produce"},
		"unknown":     {"produce", "dairy-id", "nowhere"},
		"empty entry": {"produce", "", "pantry-aisle"},
	}
	for name, tokens := range cases {
		if _, err := resolveCategoryReorderInGroup(group, tokens); err == nil {
			t.Errorf("%s: order %v must fail closed", name, tokens)
		}
	}
	if _, err := resolveCategoryReorderInGroup(group, nil); err == nil {
		t.Error("empty order must fail closed")
	}

	// Ambiguous exact name within the group fails closed.
	ambiguous := &pb.PBListCategoryGroup{Identifier: "g", ListId: "l", Name: "Aisles", Categories: []*pb.PBListCategory{
		{Identifier: "a", Name: "Dairy", CategoryGroupId: "g", ListId: "l"},
		{Identifier: "b", Name: "dairy", CategoryGroupId: "g", ListId: "l"},
	}}
	if _, err := resolveCategoryReorderInGroup(ambiguous, []string{"Dairy", "b"}); err == nil {
		t.Error("ambiguous name in the group must fail closed")
	}

	// An empty group has nothing to reorder.
	empty := &pb.PBListCategoryGroup{Identifier: "g", ListId: "l", Name: "Empty"}
	if _, err := resolveCategoryReorderInGroup(empty, []string{"x"}); err == nil {
		t.Error("empty group must fail closed")
	}
}

func TestCategoriesDeleteAbsenceVerification(t *testing.T) {
	t.Parallel()
	group := categoryDeleteReorderGroup()
	removed := group.Categories[1] // dairy-id

	// Absent from the fresh read: verification passes.
	after := &pb.PBListCategoryGroup{Identifier: "group-1", ListId: "list-1", Name: "Aisles", Categories: []*pb.PBListCategory{
		{Identifier: "produce", Name: "Produce", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 0},
		{Identifier: "pantry-aisle", Name: "Pantry Aisle", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 1},
	}}
	if err := verifyLiveCategoryDelete(categoryTestUserData("list-1", "Groceries", after), "list-1", removed); err != nil {
		t.Fatalf("absence must pass verification, got: %v", err)
	}

	// Still present: verification fails.
	if err := verifyLiveCategoryDelete(categoryTestUserData("list-1", "Groceries", group), "list-1", removed); err == nil || !strings.Contains(err.Error(), "delete verification failed") {
		t.Fatalf("still-present category must fail verification: %v", err)
	}

	// A fresh read carrying no response for the list fails closed —
	// absence must be proven from fresh metadata, not assumed.
	noMetadata := &pb.PBUserDataResponse{ShoppingListsResponse: &pb.ShoppingListsResponse{}}
	if err := verifyLiveCategoryDelete(noMetadata, "list-1", removed); err == nil {
		t.Fatal("missing fresh metadata must fail verification")
	}
	// A list that genuinely carries no groups proves absence without error.
	if err := verifyLiveCategoryDelete(categoryTestUserData("list-1", "Groceries"), "list-1", removed); err != nil {
		t.Fatalf("group-less list must pass absence verification, got: %v", err)
	}
}

func TestCategoriesReorderReadBackVerification(t *testing.T) {
	t.Parallel()
	group := categoryDeleteReorderGroup()
	expected := []*pb.PBListCategory{group.Categories[1], group.Categories[0], group.Categories[2]} // dairy, produce, pantry

	// The server persists the order and reassigns sortIndex: read-back
	// matches exactly.
	okGroup := &pb.PBListCategoryGroup{Identifier: "group-1", ListId: "list-1", Name: "Aisles", Categories: []*pb.PBListCategory{
		{Identifier: "dairy-id", Name: "Dairy", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 0},
		{Identifier: "produce", Name: "Produce", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 1},
		{Identifier: "pantry-aisle", Name: "Pantry Aisle", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 2},
	}}
	if _, err := verifyLiveCategoryReorder(categoryTestUserData("list-1", "Groceries", okGroup), "list-1", "group-1", expected); err != nil {
		t.Fatalf("exact read-back order must pass: %v", err)
	}

	// Wire order carries the order with stable (equal) sortIndex values.
	wireOrder := &pb.PBListCategoryGroup{Identifier: "group-1", ListId: "list-1", Name: "Aisles", Categories: []*pb.PBListCategory{
		{Identifier: "dairy-id", Name: "Dairy", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 5},
		{Identifier: "produce", Name: "Produce", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 5},
		{Identifier: "pantry-aisle", Name: "Pantry Aisle", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 5},
	}}
	if _, err := verifyLiveCategoryReorder(categoryTestUserData("list-1", "Groceries", wireOrder), "list-1", "group-1", expected); err != nil {
		t.Fatalf("wire order with equal sortIndex must pass: %v", err)
	}

	cases := map[string]*pb.PBListCategoryGroup{
		"wrong order": {Identifier: "group-1", ListId: "list-1", Name: "Aisles", Categories: []*pb.PBListCategory{
			{Identifier: "produce", Name: "Produce", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 0},
			{Identifier: "dairy-id", Name: "Dairy", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 1},
			{Identifier: "pantry-aisle", Name: "Pantry Aisle", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 2},
		}},
		"missing category": {Identifier: "group-1", ListId: "list-1", Name: "Aisles", Categories: []*pb.PBListCategory{
			{Identifier: "dairy-id", Name: "Dairy", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 0},
			{Identifier: "produce", Name: "Produce", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 1},
		}},
		"extra category": {Identifier: "group-1", ListId: "list-1", Name: "Aisles", Categories: []*pb.PBListCategory{
			{Identifier: "dairy-id", Name: "Dairy", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 0},
			{Identifier: "produce", Name: "Produce", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 1},
			{Identifier: "pantry-aisle", Name: "Pantry Aisle", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 2},
			{Identifier: "sundry", Name: "Sundry", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 3},
		}},
	}
	for name, readGroup := range cases {
		if _, err := verifyLiveCategoryReorder(categoryTestUserData("list-1", "Groceries", readGroup), "list-1", "group-1", expected); err == nil || !strings.Contains(err.Error(), "reorder verification failed") {
			t.Errorf("%s: read-back must fail verification: %v", name, err)
		}
	}

	// Group absent from the fresh read: verification fails.
	if _, err := verifyLiveCategoryReorder(categoryTestUserData("list-1", "Groceries"), "list-1", "group-1", expected); err == nil {
		t.Fatal("missing group must fail verification")
	}
}
