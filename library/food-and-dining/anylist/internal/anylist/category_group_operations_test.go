// Copyright 2026 Jeeves and contributors. Licensed under Apache-2.0. See LICENSE.

package anylist

import (
	"bytes"
	"io"
	"net/http"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/config"
	"google.golang.org/protobuf/proto"
)

func categoryGroupTestFixtures() (group *pb.PBListCategoryGroup, category *pb.PBListCategory) {
	group = &pb.PBListCategoryGroup{
		Identifier: "group-1",
		ListId:     "list-1",
		Name:       "Aisles",
		Categories: []*pb.PBListCategory{
			{Identifier: "produce", Name: "Produce", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 0},
			{Identifier: "pantry-aisle", Name: "Pantry Aisle", CategoryGroupId: "group-1", ListId: "list-1", SortIndex: 1},
		},
	}
	category = group.Categories[1]
	return group, category
}

func TestDeleteListCategorySendsProvenV2MultipartWireShape(t *testing.T) {
	t.Parallel()
	client, requests := newCategoryTestClient(t, http.StatusOK)
	group, category := categoryGroupTestFixtures()

	if err := client.DeleteListCategory(t.Context(), "list-1", group, category); err != nil {
		t.Fatalf("DeleteListCategory: %v", err)
	}
	if len(*requests) != 1 {
		t.Fatalf("request count = %d, want exactly one", len(*requests))
	}
	req := (*requests)[0]
	if req.URL.Path != "/data/shopping-lists/update-v2" {
		t.Fatalf("request path = %q, want /data/shopping-lists/update-v2", req.URL.Path)
	}
	if req.Method != http.MethodPost {
		t.Fatalf("method = %q, want POST", req.Method)
	}
	operations := readCategoryOperationsPart(t, req)
	if len(operations.GetOperations()) != 1 {
		t.Fatalf("operation count = %d, want 1", len(operations.GetOperations()))
	}
	op := operations.GetOperations()[0]
	if got := op.GetMetadata().GetHandlerId(); got != "remove-category-ids" {
		t.Errorf("handler = %q, want remove-category-ids (the old remove-category handler is non-persistent and must not be used)", got)
	}
	if got := op.GetMetadata().GetOperationClass(); got != int32(pb.PBOperationMetadata_ListCategoryGroupOperation) {
		t.Errorf("operation class = %d, want ListCategoryGroupOperation (4)", got)
	}
	if got := op.GetListId(); got != "list-1" {
		t.Errorf("listId = %q, want list-1", got)
	}
	// The deletion payload carries the group, not category-level fields.
	if op.GetUpdatedCategory() != nil || op.GetOriginalCategory() != nil {
		t.Errorf("remove-category-ids must carry updatedCategoryGroup, not category fields: updated=%v original=%v", op.GetUpdatedCategory(), op.GetOriginalCategory())
	}
	if op.GetOriginalCategoryGroup() != nil {
		t.Errorf("remove-category-ids must not carry originalCategoryGroup, got %v", op.GetOriginalCategoryGroup())
	}
	updatedGroup := op.GetUpdatedCategoryGroup()
	if updatedGroup == nil {
		t.Fatal("remove-category-ids must carry updatedCategoryGroup")
	}
	// Group identity fields are copied from the fresh group.
	if updatedGroup.GetIdentifier() != "group-1" || updatedGroup.GetListId() != "list-1" || updatedGroup.GetName() != "Aisles" {
		t.Errorf("updatedCategoryGroup identity = %q/%q/%q, want group-1/list-1/Aisles", updatedGroup.GetIdentifier(), updatedGroup.GetListId(), updatedGroup.GetName())
	}
	// The deletion set carries exactly the one selected full category record.
	if got := len(updatedGroup.GetCategories()); got != 1 {
		t.Fatalf("updatedCategoryGroup categories = %d, want exactly the selected full record", got)
	}
	sent := updatedGroup.GetCategories()[0]
	if !proto.Equal(sent, category) {
		t.Errorf("deletion-set category = %v, want the full fresh record %v", sent, category)
	}
	if sent.GetName() != "Pantry Aisle" || sent.GetSortIndex() != 1 {
		t.Errorf("deletion-set category must be the full record, got name=%q sortIndex=%d", sent.GetName(), sent.GetSortIndex())
	}
}

func TestReorderListCategoriesSendsProvenV2MultipartWireShape(t *testing.T) {
	t.Parallel()
	client, requests := newCategoryTestClient(t, http.StatusOK)
	group, _ := categoryGroupTestFixtures()
	// Caller records may be full; the proven wire shape is identifier-only.
	ordered := []*pb.PBListCategory{group.Categories[1], group.Categories[0]}

	if err := client.ReorderListCategories(t.Context(), "list-1", group, ordered); err != nil {
		t.Fatalf("ReorderListCategories: %v", err)
	}
	if len(*requests) != 1 {
		t.Fatalf("request count = %d, want exactly one", len(*requests))
	}
	req := (*requests)[0]
	if req.URL.Path != "/data/shopping-lists/update-v2" {
		t.Fatalf("request path = %q, want /data/shopping-lists/update-v2", req.URL.Path)
	}
	operations := readCategoryOperationsPart(t, req)
	if len(operations.GetOperations()) != 1 {
		t.Fatalf("operation count = %d, want 1", len(operations.GetOperations()))
	}
	op := operations.GetOperations()[0]
	if got := op.GetMetadata().GetHandlerId(); got != "set-sorted-category-ids" {
		t.Errorf("handler = %q, want set-sorted-category-ids", got)
	}
	if got := op.GetMetadata().GetOperationClass(); got != int32(pb.PBOperationMetadata_ListCategoryGroupOperation) {
		t.Errorf("operation class = %d, want ListCategoryGroupOperation (4)", got)
	}
	if got := op.GetListId(); got != "list-1" {
		t.Errorf("listId = %q, want list-1", got)
	}
	if op.GetUpdatedCategory() != nil || op.GetOriginalCategory() != nil {
		t.Errorf("set-sorted-category-ids must carry updatedCategoryGroup, not category fields: %v", op)
	}
	if op.GetOriginalCategoryGroup() != nil {
		t.Errorf("set-sorted-category-ids must not carry originalCategoryGroup, got %v", op.GetOriginalCategoryGroup())
	}
	updatedGroup := op.GetUpdatedCategoryGroup()
	if updatedGroup == nil {
		t.Fatal("set-sorted-category-ids must carry updatedCategoryGroup")
	}
	if updatedGroup.GetIdentifier() != "group-1" || updatedGroup.GetListId() != "list-1" || updatedGroup.GetName() != "Aisles" {
		t.Errorf("updatedCategoryGroup identity = %q/%q/%q, want group-1/list-1/Aisles", updatedGroup.GetIdentifier(), updatedGroup.GetListId(), updatedGroup.GetName())
	}
	sent := updatedGroup.GetCategories()
	if len(sent) != 2 {
		t.Fatalf("updatedCategoryGroup categories = %d, want 2", len(sent))
	}
	if sent[0].GetIdentifier() != "pantry-aisle" || sent[1].GetIdentifier() != "produce" {
		t.Errorf("order = %q, %q, want pantry-aisle, produce", sent[0].GetIdentifier(), sent[1].GetIdentifier())
	}
	// Identifier-only records: no other field may ride along on the wire.
	for i, record := range sent {
		if record.GetName() != "" || record.GetListId() != "" || record.GetCategoryGroupId() != "" || record.GetSortIndex() != 0 || record.GetIcon() != "" || record.GetSystemCategory() != "" || record.GetLogicalTimestamp() != 0 {
			t.Errorf("position %d is not identifier-only: %v", i+1, record)
		}
	}
	// The caller's records must not be mutated by the payload build.
	if ordered[0].GetIdentifier() != "pantry-aisle" || ordered[0].GetName() != "Pantry Aisle" {
		t.Errorf("caller record was mutated: %v", ordered[0])
	}
}

func TestDeleteListCategoryValidatesBeforeRequest(t *testing.T) {
	t.Parallel()
	client, requests := newCategoryTestClient(t, http.StatusOK)
	group, category := categoryGroupTestFixtures()

	if err := client.DeleteListCategory(t.Context(), "", group, category); err == nil {
		t.Error("empty list ID must be rejected")
	}
	if err := client.DeleteListCategory(t.Context(), "list-1", nil, category); err == nil {
		t.Error("nil group must be rejected")
	}
	if err := client.DeleteListCategory(t.Context(), "list-1", &pb.PBListCategoryGroup{Identifier: ""}, category); err == nil {
		t.Error("group without a stable ID must be rejected")
	}
	if err := client.DeleteListCategory(t.Context(), "list-1", group, nil); err == nil {
		t.Error("nil category must be rejected")
	}
	if err := client.DeleteListCategory(t.Context(), "list-1", group, &pb.PBListCategory{Identifier: ""}); err == nil {
		t.Error("category without a stable ID must be rejected")
	}
	otherGroupCategory := &pb.PBListCategory{Identifier: "pantry-aisle", CategoryGroupId: "group-2", ListId: "list-1", Name: "Pantry Aisle"}
	if err := client.DeleteListCategory(t.Context(), "list-1", group, otherGroupCategory); err == nil {
		t.Error("category from a different group must be rejected")
	}
	systemCategory := proto.Clone(category).(*pb.PBListCategory)
	systemCategory.SystemCategory = "other"
	if err := client.DeleteListCategory(t.Context(), "list-1", group, systemCategory); err == nil {
		t.Error("system category must be rejected")
	}
	if len(*requests) != 0 {
		t.Fatalf("rejected payloads produced %d requests, want 0", len(*requests))
	}
}

func TestReorderListCategoriesValidatesBeforeRequest(t *testing.T) {
	t.Parallel()
	client, requests := newCategoryTestClient(t, http.StatusOK)
	group, _ := categoryGroupTestFixtures()

	if err := client.ReorderListCategories(t.Context(), "", group, []*pb.PBListCategory{{Identifier: "a"}}); err == nil {
		t.Error("empty list ID must be rejected")
	}
	if err := client.ReorderListCategories(t.Context(), "list-1", nil, []*pb.PBListCategory{{Identifier: "a"}}); err == nil {
		t.Error("nil group must be rejected")
	}
	if err := client.ReorderListCategories(t.Context(), "list-1", group, nil); err == nil {
		t.Error("empty order must be rejected")
	}
	if err := client.ReorderListCategories(t.Context(), "list-1", group, []*pb.PBListCategory{{Identifier: "a"}, {Identifier: ""}}); err == nil {
		t.Error("order entry without a stable ID must be rejected")
	}
	if len(*requests) != 0 {
		t.Fatalf("rejected payloads produced %d requests, want 0", len(*requests))
	}
}

// Group mutations must hit the v2 multipart route and never the non-persistent
// v1 route or any other path, even when the server errors.
func TestCategoryGroupOperationNeverFallsBackToV1Route(t *testing.T) {
	t.Parallel()
	var v2, v1, other atomic.Int64
	cfg := &config.Config{
		Path:        filepath.Join(t.TempDir(), "config.toml"),
		AccessToken: "access-token",
		UserID:      "user-1",
	}
	client := New(cfg)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/data/shopping-lists/update-v2":
			v2.Add(1)
		case "/data/shopping-lists/update":
			v1.Add(1)
		default:
			other.Add(1)
		}
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(bytes.NewBufferString("boom")),
			Header:     http.Header{},
		}, nil
	})}

	group, category := categoryGroupTestFixtures()
	if err := client.DeleteListCategory(t.Context(), "list-1", group, category); err == nil {
		t.Fatal("DeleteListCategory succeeded against HTTP 500")
	}
	if err := client.ReorderListCategories(t.Context(), "list-1", group, []*pb.PBListCategory{{Identifier: "pantry-aisle"}, {Identifier: "produce"}}); err == nil {
		t.Fatal("ReorderListCategories succeeded against HTTP 500")
	}
	if got := v2.Load(); got != 2 {
		t.Fatalf("update-v2 request count = %d, want 2", got)
	}
	if got := v1.Load(); got != 0 {
		t.Fatalf("v1 route request count = %d, want 0 — category writes must never use the non-persistent v1 path", got)
	}
	if got := other.Load(); got != 0 {
		t.Fatalf("unexpected request count = %d", got)
	}
}
