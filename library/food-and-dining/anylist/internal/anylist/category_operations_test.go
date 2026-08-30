// Copyright 2026 Jeeves and contributors. Licensed under Apache-2.0. See LICENSE.

package anylist

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/config"
	"google.golang.org/protobuf/proto"
)

// newCategoryTestClient stubs the HTTP transport and records every request
// the client attempts, so tests can prove the v2 multipart wire shape and the
// absence of any v1/form-urlencoded fallback.
func newCategoryTestClient(t *testing.T, status int) (*Client, *[]*http.Request) {
	t.Helper()
	var requests []*http.Request
	cfg := &config.Config{
		Path:        filepath.Join(t.TempDir(), "config.toml"),
		AccessToken: "access-token",
		UserID:      "user-1",
	}
	client := New(cfg)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req)
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true}`)),
			Header:     http.Header{},
		}, nil
	})}
	return client, &requests
}

// readCategoryOperationsPart decodes the single "operations" multipart part of
// the request and enforces the proven part shape: form-data, part named
// "operations", application/octet-stream payload.
func readCategoryOperationsPart(t *testing.T, req *http.Request) *pb.PBListOperationList {
	t.Helper()
	contentType := req.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "multipart/form-data; boundary=") {
		t.Fatalf("content type = %q, want multipart/form-data", contentType)
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "multipart/form-data" {
		t.Fatalf("parse content type = %q, %v", contentType, err)
	}
	reader := multipart.NewReader(req.Body, params["boundary"])
	part, err := reader.NextPart()
	if err != nil {
		t.Fatalf("reading operations part: %v", err)
	}
	if part.FormName() != "operations" {
		t.Fatalf("multipart field = %q, want operations", part.FormName())
	}
	if ct := part.Header.Get("Content-Type"); ct != "application/octet-stream" {
		t.Fatalf("part content type = %q, want application/octet-stream", ct)
	}
	body, err := io.ReadAll(part)
	if err != nil {
		t.Fatalf("reading operations part body: %v", err)
	}
	if next, _ := reader.NextPart(); next != nil {
		t.Fatalf("expected exactly one multipart part, got a second: %q", next.FormName())
	}
	var operations pb.PBListOperationList
	if err := proto.Unmarshal(body, &operations); err != nil {
		t.Fatalf("unmarshaling PBListOperationList: %v", err)
	}
	return &operations
}

func TestCreateListCategorySendsProvenV2MultipartWireShape(t *testing.T) {
	t.Parallel()
	client, requests := newCategoryTestClient(t, http.StatusOK)

	category := &pb.PBListCategory{
		Identifier:      "pantry-aisle",
		ListId:          "list-1",
		CategoryGroupId: "group-1",
		Name:            "Pantry Aisle",
		SortIndex:       3,
	}
	if err := client.CreateListCategory(t.Context(), "list-1", category); err != nil {
		t.Fatalf("CreateListCategory: %v", err)
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
	if got := op.GetMetadata().GetHandlerId(); got != "create-category" {
		t.Errorf("handler = %q, want create-category", got)
	}
	if got := op.GetMetadata().GetOperationClass(); got != int32(pb.PBOperationMetadata_ListCategoryOperation) {
		t.Errorf("operation class = %d, want ListCategoryOperation (3)", got)
	}
	if got := op.GetListId(); got != "list-1" {
		t.Errorf("listId = %q, want list-1", got)
	}
	if op.GetUpdatedValue() != "" || op.GetOriginalValue() != "" {
		t.Errorf("operation scalar values = updated %q/original %q, want both empty", op.GetUpdatedValue(), op.GetOriginalValue())
	}
	if op.GetOriginalCategory() != nil {
		t.Errorf("create-category must not carry originalCategory, got %v", op.GetOriginalCategory())
	}
	updated := op.GetUpdatedCategory()
	if updated == nil {
		t.Fatal("create-category must carry updatedCategory")
	}
	if updated.GetIdentifier() != "pantry-aisle" || updated.GetListId() != "list-1" || updated.GetCategoryGroupId() != "group-1" {
		t.Errorf("updatedCategory identity = %q/%q/%q, want pantry-aisle/list-1/group-1", updated.GetIdentifier(), updated.GetListId(), updated.GetCategoryGroupId())
	}
	if updated.GetName() != "Pantry Aisle" {
		t.Errorf("updatedCategory name = %q", updated.GetName())
	}
	if updated.GetSortIndex() != 3 {
		t.Errorf("updatedCategory sortIndex = %d, want 3", updated.GetSortIndex())
	}
}

func TestRenameListCategorySendsProvenV2MultipartWireShape(t *testing.T) {
	t.Parallel()
	client, requests := newCategoryTestClient(t, http.StatusOK)

	original := &pb.PBListCategory{
		Identifier:      "pantry-aisle",
		ListId:          "list-1",
		CategoryGroupId: "group-1",
		Name:            "Pantry Aisle",
		SortIndex:       2,
	}
	updated := proto.Clone(original).(*pb.PBListCategory)
	updated.Name = "Pantry"

	if err := client.RenameListCategory(t.Context(), "list-1", original, updated); err != nil {
		t.Fatalf("RenameListCategory: %v", err)
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
	if got := op.GetMetadata().GetHandlerId(); got != "set-category-name" {
		t.Errorf("handler = %q, want set-category-name", got)
	}
	if got := op.GetMetadata().GetOperationClass(); got != int32(pb.PBOperationMetadata_ListCategoryOperation) {
		t.Errorf("operation class = %d, want ListCategoryOperation (3)", got)
	}
	if got := op.GetListId(); got != "list-1" {
		t.Errorf("listId = %q, want list-1", got)
	}
	orig := op.GetOriginalCategory()
	if orig == nil {
		t.Fatal("set-category-name must carry originalCategory")
	}
	if orig.GetName() != "Pantry Aisle" || orig.GetIdentifier() != "pantry-aisle" {
		t.Errorf("originalCategory = %q/%q", orig.GetName(), orig.GetIdentifier())
	}
	upd := op.GetUpdatedCategory()
	if upd == nil {
		t.Fatal("set-category-name must carry updatedCategory")
	}
	if upd.GetName() != "Pantry" {
		t.Errorf("updatedCategory name = %q, want Pantry", upd.GetName())
	}
	// Stable identity, group, list, and sort index must survive unchanged.
	if upd.GetIdentifier() != "pantry-aisle" || upd.GetCategoryGroupId() != "group-1" || upd.GetListId() != "list-1" || upd.GetSortIndex() != 2 {
		t.Errorf("updatedCategory identity changed: %q/%q/%q/%d", upd.GetIdentifier(), upd.GetCategoryGroupId(), upd.GetListId(), upd.GetSortIndex())
	}
}

func TestCreateListCategoryRejectsMismatchedPayloadWithoutRequest(t *testing.T) {
	t.Parallel()
	client, requests := newCategoryTestClient(t, http.StatusOK)

	category := &pb.PBListCategory{Identifier: "pantry-aisle", ListId: "other-list", CategoryGroupId: "group-1", Name: "Pantry Aisle"}
	if err := client.CreateListCategory(t.Context(), "list-1", category); err == nil {
		t.Fatal("CreateListCategory accepted a category whose listId does not match the target list")
	}
	for _, field := range []string{"", "group-1", "Pantry Aisle"} {
		var missing *pb.PBListCategory
		switch {
		case field == "":
			missing = &pb.PBListCategory{Identifier: "", ListId: "list-1", CategoryGroupId: "group-1", Name: "Pantry Aisle"}
		case field == "group-1":
			missing = &pb.PBListCategory{Identifier: "pantry-aisle", ListId: "list-1", CategoryGroupId: "", Name: "Pantry Aisle"}
		default:
			missing = &pb.PBListCategory{Identifier: "pantry-aisle", ListId: "list-1", CategoryGroupId: "group-1", Name: ""}
		}
		if err := client.CreateListCategory(t.Context(), "list-1", missing); err == nil {
			t.Errorf("CreateListCategory accepted a payload missing %q", field)
		}
	}
	if len(*requests) != 0 {
		t.Fatalf("rejected payloads produced %d requests, want 0", len(*requests))
	}
}

func TestRenameListCategoryValidatesBeforeRequest(t *testing.T) {
	t.Parallel()
	client, requests := newCategoryTestClient(t, http.StatusOK)

	original := &pb.PBListCategory{Identifier: "pantry-aisle", ListId: "list-1", CategoryGroupId: "group-1", Name: "Pantry Aisle", SortIndex: 2}

	sameName := proto.Clone(original).(*pb.PBListCategory)
	if err := client.RenameListCategory(t.Context(), "list-1", original, sameName); err == nil {
		t.Error("RenameListCategory accepted an unchanged name")
	}
	differentID := proto.Clone(original).(*pb.PBListCategory)
	differentID.Identifier = "other-id"
	differentID.Name = "Pantry"
	if err := client.RenameListCategory(t.Context(), "list-1", original, differentID); err == nil {
		t.Error("RenameListCategory accepted a different updated identifier")
	}
	differentGroup := proto.Clone(original).(*pb.PBListCategory)
	differentGroup.CategoryGroupId = "group-2"
	differentGroup.Name = "Pantry"
	if err := client.RenameListCategory(t.Context(), "list-1", original, differentGroup); err == nil {
		t.Error("RenameListCategory accepted a category group change")
	}
	differentSort := proto.Clone(original).(*pb.PBListCategory)
	differentSort.SortIndex = 9
	differentSort.Name = "Pantry"
	if err := client.RenameListCategory(t.Context(), "list-1", original, differentSort); err == nil {
		t.Error("RenameListCategory accepted a sort index change")
	}
	if len(*requests) != 0 {
		t.Fatalf("rejected payloads produced %d requests, want 0", len(*requests))
	}
}

func TestCategoryOperationNeverFallsBackToV1Route(t *testing.T) {
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

	category := &pb.PBListCategory{Identifier: "pantry-aisle", ListId: "list-1", CategoryGroupId: "group-1", Name: "Pantry Aisle"}
	if err := client.CreateListCategory(t.Context(), "list-1", category); err == nil {
		t.Fatal("CreateListCategory succeeded against HTTP 500")
	}
	if got := v2.Load(); got != 1 {
		t.Fatalf("update-v2 request count = %d, want 1", got)
	}
	if got := v1.Load(); got != 0 {
		t.Fatalf("v1 route request count = %d, want 0 — category writes must never use the non-persistent v1 path", got)
	}
	if got := other.Load(); got != 0 {
		t.Fatalf("unexpected request count = %d", got)
	}
}
