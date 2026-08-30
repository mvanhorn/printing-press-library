package anylist

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/config"
	"google.golang.org/protobuf/proto"
)

func TestUpdateItemCategoryAssignmentMatchesBrowserWireShape(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Path:        filepath.Join(t.TempDir(), "config.toml"),
		AccessToken: "access-token",
		UserID:      "user-1",
	}
	client := New(cfg)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/data/shopping-lists/update" {
			t.Fatalf("request path = %q, want shopping-list update", req.URL.Path)
		}
		contentType := req.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "multipart/form-data; boundary=") {
			t.Fatalf("content type = %q, want multipart", contentType)
		}
		mediaType, params, err := mime.ParseMediaType(contentType)
		if err != nil || mediaType != "multipart/form-data" {
			t.Fatalf("parse content type = %q, %v", mediaType, err)
		}
		reader := multipart.NewReader(req.Body, params["boundary"])
		part, err := reader.NextPart()
		if err != nil {
			t.Fatalf("read operations part: %v", err)
		}
		if part.FormName() != "operations" {
			t.Fatalf("multipart field = %q, want operations", part.FormName())
		}
		body, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("read operations part: %v", err)
		}
		var operations pb.PBListOperationList
		if err := proto.Unmarshal(body, &operations); err != nil {
			t.Fatalf("unmarshal operations: %v", err)
		}
		if len(operations.GetOperations()) != 2 {
			t.Fatalf("operation count = %d, want 2", len(operations.GetOperations()))
		}
		assignment, match := operations.GetOperations()[0], operations.GetOperations()[1]
		if got := assignment.GetMetadata().GetHandlerId(); got != "update-list-item-category-assignment" {
			t.Errorf("assignment handler = %q", got)
		}
		if got := assignment.GetListItem().GetCategoryMatchId(); got != "other" {
			t.Errorf("assignment original category match = %q, want other", got)
		}
		if assignment.GetUpdatedValue() != "" || assignment.GetOriginalValue() != "" {
			t.Errorf("assignment values = updated %q/original %q, want both empty", assignment.GetUpdatedValue(), assignment.GetOriginalValue())
		}
		if got := match.GetMetadata().GetHandlerId(); got != "set-list-item-category-match-id" {
			t.Errorf("match handler = %q", got)
		}
		if got := match.GetOriginalValue(); got != "category-1" {
			t.Errorf("match originalValue = %q, want category-1", got)
		}
		if match.GetUpdatedValue() != "" {
			t.Errorf("match updatedValue = %q, want empty", match.GetUpdatedValue())
		}
		if got := match.GetListItem().GetCategoryMatchId(); got != "category-1" {
			t.Errorf("match item category = %q, want category-1", got)
		}
		if got := match.GetListItem().GetCategory(); got != "Pantry Aisle" {
			t.Errorf("match item category label = %q, want Pantry Aisle", got)
		}
		for _, op := range operations.GetOperations() {
			if op.GetListId() != "list-1" || op.GetListItemId() != "item-1" {
				t.Errorf("operation IDs = %q/%q, want list-1/item-1", op.GetListId(), op.GetListItemId())
			}
			if op.GetListItem() == nil {
				t.Error("category operation omitted full list item")
			}
		}
		return &http.Response{StatusCode: http.StatusOK, Status: http.StatusText(http.StatusOK), Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header), Request: req}, nil
	})}

	item := &pb.ListItem{Identifier: "item-1", ListId: "list-1", Name: "Probe Crate", Category: "other", CategoryMatchId: "other"}
	if err := client.UpdateItemCategoryAssignment(t.Context(), "list-1", item, "other", "category-1", "Pantry Aisle"); err != nil {
		t.Fatalf("UpdateItemCategoryAssignment returned error: %v", err)
	}
}

func TestItemStoreOperationsUseExactHandlersAndValues(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Path: filepath.Join(t.TempDir(), "config.toml"), AccessToken: "access-token", UserID: "user-1"}
	client := New(cfg)
	marker := ""
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		form, err := url.ParseQuery(string(body))
		if err != nil {
			return nil, err
		}
		var operations pb.PBListOperationList
		if err := proto.Unmarshal([]byte(form.Get("operations")), &operations); err != nil {
			return nil, err
		}
		if len(operations.GetOperations()) != 1 {
			return nil, io.ErrUnexpectedEOF
		}
		op := operations.GetOperations()[0]
		wantHandler := "add-list-item-store-id"
		if marker == "remove" {
			wantHandler = "remove-list-item-store-id"
		}
		if op.GetMetadata().GetHandlerId() != wantHandler || op.GetUpdatedValue() != "store-1" {
			return nil, io.ErrUnexpectedEOF
		}
		return &http.Response{StatusCode: http.StatusOK, Status: http.StatusText(http.StatusOK), Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header), Request: req}, nil
	})}
	marker = "add"
	if err := client.AddItemStoreID(t.Context(), "list-1", "item-1", "store-1"); err != nil {
		t.Fatalf("AddItemStoreID returned error: %v", err)
	}
	marker = "remove"
	if err := client.RemoveItemStoreID(t.Context(), "list-1", "item-1", "store-1"); err != nil {
		t.Fatalf("RemoveItemStoreID returned error: %v", err)
	}
}

func TestSaveItemPriceUsesCapturedMultipartOperation(t *testing.T) {
	t.Parallel()

	client := New(&config.Config{AccessToken: "access-token", UserID: "user-1"})
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		_, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
		if err != nil {
			return nil, err
		}
		part, err := multipart.NewReader(req.Body, params["boundary"]).NextPart()
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(part)
		if err != nil {
			return nil, err
		}
		var operations pb.PBListOperationList
		if err := proto.Unmarshal(body, &operations); err != nil {
			return nil, err
		}
		if len(operations.GetOperations()) != 1 {
			return nil, io.ErrUnexpectedEOF
		}
		op := operations.GetOperations()[0]
		price := op.GetItemPrice()
		if op.GetMetadata().GetHandlerId() != "save-item-price" || op.GetListId() != "list-1" || op.GetListItemId() != "item-1" || price.GetAmount() != 3.49 || price.GetDate() != "2026-08-16" {
			return nil, io.ErrUnexpectedEOF
		}
		return &http.Response{StatusCode: http.StatusOK, Status: http.StatusText(http.StatusOK), Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header), Request: req}, nil
	})}
	if err := client.SaveItemPrice(t.Context(), "list-1", "item-1", &pb.PBItemPrice{Amount: 3.49, Date: "2026-08-16"}); err != nil {
		t.Fatalf("SaveItemPrice returned error: %v", err)
	}
}

func TestUpdateItemCategoryAssignmentClearUsesOtherMatchID(t *testing.T) {
	t.Parallel()

	client := New(&config.Config{AccessToken: "access-token", UserID: "user-1"})
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		_, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
		if err != nil {
			return nil, err
		}
		part, err := multipart.NewReader(req.Body, params["boundary"]).NextPart()
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(part)
		if err != nil {
			return nil, err
		}
		var operations pb.PBListOperationList
		if err := proto.Unmarshal(body, &operations); err != nil {
			return nil, err
		}
		if len(operations.GetOperations()) != 2 {
			return nil, io.ErrUnexpectedEOF
		}
		match := operations.GetOperations()[1]
		if match.GetOriginalValue() != "other" || match.GetUpdatedValue() != "" || match.GetListItem().GetCategoryMatchId() != "other" || match.GetListItem().GetCategory() != "other" {
			return nil, io.ErrUnexpectedEOF
		}
		return &http.Response{StatusCode: http.StatusOK, Status: http.StatusText(http.StatusOK), Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header), Request: req}, nil
	})}
	item := &pb.ListItem{Identifier: "item-1", ListId: "list-1", Category: "Pantry Aisle", CategoryMatchId: "pantry-aisle"}
	if err := client.UpdateItemCategoryAssignment(t.Context(), "list-1", item, "pantry-aisle", "other", "other"); err != nil {
		t.Fatalf("clear category returned error: %v", err)
	}
}

func TestUpdateItemCategoryAssignmentRejectsStaleOriginal(t *testing.T) {
	t.Parallel()

	client := New(&config.Config{UserID: "user-1"})
	err := client.UpdateItemCategoryAssignment(t.Context(), "list-1", &pb.ListItem{
		Identifier: "item-1", ListId: "list-1", CategoryMatchId: "other",
	}, "produce", "category-1", "Produce")
	if err == nil {
		t.Fatal("UpdateItemCategoryAssignment accepted stale original category")
	}
}

func TestItemStoreOperationsRejectEmptyIDs(t *testing.T) {
	t.Parallel()

	client := New(&config.Config{UserID: "user-1"})
	if err := client.AddItemStoreID(t.Context(), "list-1", "item-1", " "); err == nil {
		t.Fatal("AddItemStoreID accepted an empty store ID")
	}
	if err := client.RemoveItemStoreID(t.Context(), "list-1", "", "store-1"); err == nil {
		t.Fatal("RemoveItemStoreID accepted an empty item ID")
	}
}
