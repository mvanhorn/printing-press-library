// Copyright 2026 Jeeves and contributors. Licensed under Apache-2.0. See LICENSE.

// Custom-category create/rename resolution and verification helpers.
//
// These functions operate on a fresh PBUserDataResponse only (no local cache
// reads) and fail closed: ambiguous or missing references are errors, never
// silent fallbacks.
package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
)

// listCategoryGroups returns the list's category groups from the fresh
// list response, in wire order.
func listCategoryGroups(userData *pb.PBUserDataResponse, listID string) ([]*pb.PBListCategoryGroup, error) {
	response, ok := liveListResponseByID(userData, listID)
	if !ok {
		return nil, fmt.Errorf("list %q has no fresh category metadata", listID)
	}
	var groups []*pb.PBListCategoryGroup
	for _, groupResponse := range response.GetCategoryGroupResponses() {
		if group := groupResponse.GetCategoryGroup(); group != nil {
			groups = append(groups, group)
		}
	}
	return groups, nil
}

// allListCategories flattens every category across the list's groups.
func allListCategories(userData *pb.PBUserDataResponse, listID string) ([]*pb.PBListCategory, error) {
	groups, err := listCategoryGroups(userData, listID)
	if err != nil {
		return nil, err
	}
	var categories []*pb.PBListCategory
	for _, group := range groups {
		categories = append(categories, group.GetCategories()...)
	}
	return categories, nil
}

// resolveCategoryListRecord resolves the target list by stable ID first, then
// by exact (case-insensitive) name. Ambiguous names fail closed.
func resolveCategoryListRecord(userData *pb.PBUserDataResponse, token string) (*pb.ShoppingList, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("list must not be empty")
	}
	if list, found := findLiveShoppingListByID(userData, token); found {
		return list, nil
	}
	return exactLiveShoppingListByName(userData, token)
}

// selectCategoryGroupForCreate resolves the category group a new category
// must join. Without a selector token the list must carry exactly one
// group; with one, the group is resolved by exact ID or exact name.
func selectCategoryGroupForCreate(userData *pb.PBUserDataResponse, listID, groupToken string) (*pb.PBListCategoryGroup, error) {
	groups, err := listCategoryGroups(userData, listID)
	if err != nil {
		return nil, err
	}
	groupToken = strings.TrimSpace(groupToken)
	if groupToken == "" {
		if len(groups) != 1 {
			return nil, fmt.Errorf("list %q has %d category groups; select one with --category-group", listID, len(groups))
		}
		return groups[0], nil
	}
	var matches []*pb.PBListCategoryGroup
	for _, group := range groups {
		if group.GetIdentifier() == groupToken || strings.EqualFold(strings.TrimSpace(group.GetName()), groupToken) {
			matches = append(matches, group)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("category group %q not found in list %q", groupToken, listID)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("category group %q is ambiguous in list %q", groupToken, listID)
	}
	return matches[0], nil
}

// newCategoryStableID generates the same 32-hex identifier shape observed on
// live AnyList categories and avoids the (extremely unlikely) collision with
// an existing category in the list. AnyList category identifiers are opaque;
// they must not be derived from user-visible names.
func newCategoryStableID(_ string, existing []*pb.PBListCategory) string {
	taken := map[string]bool{}
	for _, category := range existing {
		taken[strings.ToLower(category.GetIdentifier())] = true
	}
	for {
		id := strings.ReplaceAll(uuid.NewString(), "-", "")
		if !taken[strings.ToLower(id)] {
			return id
		}
	}
}

// nextCategorySortIndex returns the sort index for a new category appended
// after the group's current entries (0 for an empty group).
func nextCategorySortIndex(categories []*pb.PBListCategory) int32 {
	if len(categories) == 0 {
		return 0
	}
	max := categories[0].GetSortIndex()
	for _, category := range categories[1:] {
		if category.GetSortIndex() > max {
			max = category.GetSortIndex()
		}
	}
	return max + 1
}

// findCategoryByIDInList returns the category with the given stable
// identifier anywhere in the list, or nil.
func findCategoryByIDInList(userData *pb.PBUserDataResponse, listID, categoryID string) *pb.PBListCategory {
	categories, err := allListCategories(userData, listID)
	if err != nil {
		return nil
	}
	for _, category := range categories {
		if category.GetIdentifier() == categoryID {
			return category
		}
	}
	return nil
}

// resolveCategoryRecordInList resolves exactly one category by stable
// identifier or exact (case-insensitive) name within the selected list.
// Ambiguous or missing tokens fail closed; no fuzzy matching.
func resolveCategoryRecordInList(userData *pb.PBUserDataResponse, listID, token string) (*pb.PBListCategory, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("category must not be empty")
	}
	categories, err := allListCategories(userData, listID)
	if err != nil {
		return nil, err
	}
	var byID, byName []*pb.PBListCategory
	for _, category := range categories {
		if category.GetIdentifier() == token {
			byID = append(byID, category)
		}
		if strings.EqualFold(strings.TrimSpace(category.GetName()), token) {
			byName = append(byName, category)
		}
	}
	matches := byID
	if len(matches) == 0 {
		matches = byName
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("category %q was not found in list %q", token, listID)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("category %q is ambiguous in list %q; use its stable ID", token, listID)
	}
	return matches[0], nil
}

// categoryNameConflictInList returns another category in the same list that
// already carries the given name (case-insensitive), or nil.
func categoryNameConflictInList(userData *pb.PBUserDataResponse, listID, name string, excludeID string) *pb.PBListCategory {
	categories, err := allListCategories(userData, listID)
	if err != nil {
		return nil
	}
	for _, category := range categories {
		if category.GetIdentifier() != excludeID && strings.EqualFold(strings.TrimSpace(category.GetName()), strings.TrimSpace(name)) {
			return category
		}
	}
	return nil
}

// verifyLiveCategoryCreate checks that a freshly created category reads back
// from a fresh user-data read with the expected name, group, and sort index
// under its stable identifier.
func verifyLiveCategoryCreate(userData *pb.PBUserDataResponse, listID string, expected *pb.PBListCategory) (*pb.PBListCategory, error) {
	found := findCategoryByIDInList(userData, listID, expected.GetIdentifier())
	if found == nil {
		return nil, fmt.Errorf("create verification failed: category ID %q did not appear in list %q", expected.GetIdentifier(), listID)
	}
	if !strings.EqualFold(found.GetName(), expected.GetName()) {
		return nil, fmt.Errorf("create verification failed: category ID %q read back as %q, want %q", found.GetIdentifier(), found.GetName(), expected.GetName())
	}
	if found.GetCategoryGroupId() != expected.GetCategoryGroupId() {
		return nil, fmt.Errorf("create verification failed: category ID %q is in group %q, want %q", found.GetIdentifier(), found.GetCategoryGroupId(), expected.GetCategoryGroupId())
	}
	if found.GetSortIndex() != expected.GetSortIndex() {
		return nil, fmt.Errorf("create verification failed: category ID %q has sort index %d, want %d", found.GetIdentifier(), found.GetSortIndex(), expected.GetSortIndex())
	}
	return found, nil
}

// ensureDeletableCategory rejects system categories (the default Other /
// grocery categories carry a systemCategory marker) before any delete
// payload is built; custom categories have an empty systemCategory.
func ensureDeletableCategory(category *pb.PBListCategory) error {
	if strings.TrimSpace(category.GetSystemCategory()) != "" {
		return fmt.Errorf("category %q (ID %s) is a system category and cannot be deleted", category.GetName(), category.GetIdentifier())
	}
	return nil
}

// findCategoryGroupForCategory returns the fresh group in the list that
// carries the resolved category, failing closed when none does.
func findCategoryGroupForCategory(userData *pb.PBUserDataResponse, listID string, category *pb.PBListCategory) (*pb.PBListCategoryGroup, error) {
	groups, err := listCategoryGroups(userData, listID)
	if err != nil {
		return nil, err
	}
	for _, group := range groups {
		for _, member := range group.GetCategories() {
			if member.GetIdentifier() == category.GetIdentifier() {
				return group, nil
			}
		}
	}
	return nil, fmt.Errorf("category %q was not found in any group of list %q", category.GetIdentifier(), listID)
}

// resolveCategoryReorderInGroup resolves a full category ordering for one
// group: every token is resolved by stable ID or exact (case-insensitive)
// name within the group, and the order must name every group category
// exactly once. Duplicates, unknown tokens, empty entries, and orders that
// silently append or drop categories all fail closed; no fuzzy matching.
func resolveCategoryReorderInGroup(group *pb.PBListCategoryGroup, tokens []string) ([]*pb.PBListCategory, error) {
	if len(tokens) == 0 {
		return nil, fmt.Errorf("order must not be empty")
	}
	current := group.GetCategories()
	if len(current) == 0 {
		return nil, fmt.Errorf("category group %q has no categories to reorder", group.GetName())
	}
	if len(tokens) != len(current) {
		return nil, fmt.Errorf("order lists %d categories but group %q has %d; every category must be listed exactly once", len(tokens), group.GetName(), len(current))
	}
	byID := make(map[string]*pb.PBListCategory, len(current))
	for _, category := range current {
		byID[category.GetIdentifier()] = category
	}
	resolved := make([]*pb.PBListCategory, 0, len(tokens))
	seen := make(map[string]bool, len(tokens))
	for _, raw := range tokens {
		token := strings.TrimSpace(raw)
		if token == "" {
			return nil, fmt.Errorf("order contains an empty entry")
		}
		category, ok := byID[token]
		if !ok {
			var byName []*pb.PBListCategory
			for _, candidate := range current {
				if strings.EqualFold(strings.TrimSpace(candidate.GetName()), token) {
					byName = append(byName, candidate)
				}
			}
			if len(byName) == 0 {
				return nil, fmt.Errorf("category %q was not found in group %q", token, group.GetName())
			}
			if len(byName) > 1 {
				return nil, fmt.Errorf("category %q is ambiguous in group %q; use its stable ID", token, group.GetName())
			}
			category = byName[0]
		}
		if seen[category.GetIdentifier()] {
			return nil, fmt.Errorf("category %q is listed more than once in the order", token)
		}
		seen[category.GetIdentifier()] = true
		resolved = append(resolved, category)
	}
	return resolved, nil
}

// verifyLiveCategoryDelete checks that a deleted category's stable identifier
// is absent from the list in a fresh user-data read. A list with no fresh
// category metadata fails closed: absence must be proven, not assumed.
func verifyLiveCategoryDelete(userData *pb.PBUserDataResponse, listID string, removed *pb.PBListCategory) error {
	categories, err := allListCategories(userData, listID)
	if err != nil {
		return fmt.Errorf("delete verification failed: %v", err)
	}
	for _, category := range categories {
		if category.GetIdentifier() == removed.GetIdentifier() {
			return fmt.Errorf("delete verification failed: category ID %q still appears in list %q", removed.GetIdentifier(), listID)
		}
	}
	return nil
}

// verifyLiveCategoryReorder checks that the group reads back from a fresh
// user-data read with exactly the requested stable-ID order. The read-back
// order is the server's: categories sorted by sortIndex with wire order as
// the tie-break, which equals display order for a consistent server.
func verifyLiveCategoryReorder(userData *pb.PBUserDataResponse, listID, groupID string, expected []*pb.PBListCategory) (*pb.PBListCategoryGroup, error) {
	groups, err := listCategoryGroups(userData, listID)
	if err != nil {
		return nil, fmt.Errorf("reorder verification failed: %v", err)
	}
	var group *pb.PBListCategoryGroup
	for _, candidate := range groups {
		if candidate.GetIdentifier() == groupID {
			group = candidate
			break
		}
	}
	if group == nil {
		return nil, fmt.Errorf("reorder verification failed: category group %q did not appear in list %q", groupID, listID)
	}
	readBack := make([]*pb.PBListCategory, len(group.GetCategories()))
	copy(readBack, group.GetCategories())
	sort.SliceStable(readBack, func(i, j int) bool {
		return readBack[i].GetSortIndex() < readBack[j].GetSortIndex()
	})
	if len(readBack) != len(expected) {
		return nil, fmt.Errorf("reorder verification failed: group %q read back with %d categories, want %d", groupID, len(readBack), len(expected))
	}
	for i := range expected {
		if readBack[i].GetIdentifier() != expected[i].GetIdentifier() {
			return nil, fmt.Errorf("reorder verification failed: position %d in group %q read back as %q, want %q", i+1, groupID, readBack[i].GetIdentifier(), expected[i].GetIdentifier())
		}
	}
	return group, nil
}

// verifyLiveCategoryRename checks that a renamed category reads back by
// stable identifier with the new name while preserving its group and sort
// index from the original record.
func verifyLiveCategoryRename(userData *pb.PBUserDataResponse, listID string, original, expected *pb.PBListCategory) (*pb.PBListCategory, error) {
	found := findCategoryByIDInList(userData, listID, expected.GetIdentifier())
	if found == nil {
		return nil, fmt.Errorf("rename verification failed: category ID %q did not appear in list %q", expected.GetIdentifier(), listID)
	}
	if !strings.EqualFold(found.GetName(), expected.GetName()) {
		return nil, fmt.Errorf("rename verification failed: category ID %q did not read back as %q", found.GetIdentifier(), expected.GetName())
	}
	if found.GetCategoryGroupId() != original.GetCategoryGroupId() {
		return nil, fmt.Errorf("rename verification failed: category ID %q is in group %q, want %q", found.GetIdentifier(), found.GetCategoryGroupId(), original.GetCategoryGroupId())
	}
	if found.GetSortIndex() != original.GetSortIndex() {
		return nil, fmt.Errorf("rename verification failed: category ID %q has sort index %d, want %d", found.GetIdentifier(), found.GetSortIndex(), original.GetSortIndex())
	}
	return found, nil
}
