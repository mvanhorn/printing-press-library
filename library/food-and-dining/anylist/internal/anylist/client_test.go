package anylist

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/config"
	"google.golang.org/protobuf/proto"
)

func TestGetUserDataReturnsTypedRateLimitError(t *testing.T) {
	t.Parallel()

	client := New(&config.Config{Path: filepath.Join(t.TempDir(), "config.toml"), AccessToken: "token"})
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Status:     http.StatusText(http.StatusTooManyRequests),
			Header:     http.Header{"Retry-After": []string{"30"}},
			Body:       io.NopCloser(strings.NewReader("slow down")),
			Request:    req,
		}, nil
	})}

	_, err := client.GetUserData(t.Context())
	if err == nil {
		t.Fatal("GetUserData returned nil error for HTTP 429")
	}
	var rateErr *RateLimitError
	if !errors.As(err, &rateErr) {
		t.Fatalf("error = %v, want RateLimitError", err)
	}
	if rateErr.RetryAfter != "30" {
		t.Fatalf("RetryAfter = %q, want 30", rateErr.RetryAfter)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type shareInviteTestField struct {
	number int
	wire   int
	value  []byte
}

func parseShareInviteTestPayload(raw []byte) ([]shareInviteTestField, error) {
	var fields []shareInviteTestField
	for len(raw) > 0 {
		tag, n := testVarint(raw)
		if n == 0 {
			return nil, fmt.Errorf("truncated tag")
		}
		raw = raw[n:]
		field := shareInviteTestField{number: int(tag >> 3), wire: int(tag & 7)}
		if field.wire != 2 {
			return nil, fmt.Errorf("field %d has wire type %d", field.number, field.wire)
		}
		length, n := testVarint(raw)
		if n == 0 || uint64(len(raw)-n) < length {
			return nil, fmt.Errorf("truncated field %d", field.number)
		}
		raw = raw[n:]
		field.value = append([]byte(nil), raw[:length]...)
		raw = raw[length:]
		fields = append(fields, field)
	}
	return fields, nil
}

func testVarint(raw []byte) (uint64, int) {
	var value uint64
	for i := 0; i < len(raw) && i < 10; i++ {
		value |= uint64(raw[i]&0x7f) << (7 * uint(i))
		if raw[i]&0x80 == 0 {
			return value, i + 1
		}
	}
	return 0, 0
}

func shareInviteClientFixture(t *testing.T, response []byte, inspect func([]byte)) *Client {
	t.Helper()
	client := New(&config.Config{Path: filepath.Join(t.TempDir(), "config.toml"), AccessToken: "access-token", UserID: "user-1"})
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/data/shopping-lists/share-list" {
			t.Fatalf("request = %s %s, want POST /data/shopping-lists/share-list", req.Method, req.URL.Path)
		}
		if req.Header.Get("Authorization") != "Bearer access-token" {
			t.Fatalf("Authorization = %q", req.Header.Get("Authorization"))
		}
		_, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("parse multipart content type: %v", err)
		}
		part, err := multipart.NewReader(req.Body, params["boundary"]).NextPart()
		if err != nil || part.FormName() != "operation" {
			t.Fatalf("operation part = %v, %v", part, err)
		}
		if got := part.Header.Get("Content-Type"); got != "" {
			t.Fatalf("operation part Content-Type = %q, want empty", got)
		}
		raw, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("read operation part: %v", err)
		}
		inspect(raw)
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(bytes.NewReader(response)), Header: make(http.Header), Request: req}, nil
	})}
	return client
}

func TestShareListInviteWireShape(t *testing.T) {
	response, err := proto.Marshal(&pb.PBShareListOperationResponse{SharedUser: &pb.PBEmailUserIDPair{Email: "friend@example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	client := shareInviteClientFixture(t, response, func(raw []byte) {
		fields, err := parseShareInviteTestPayload(raw)
		if err != nil || len(fields) != 3 {
			t.Fatalf("payload fields = %#v, %v; want fields 1,2,4", fields, err)
		}
		byNumber := map[int][]byte{}
		for _, field := range fields {
			byNumber[field.number] = field.value
		}
		var metadata pb.PBOperationMetadata
		if err := proto.Unmarshal(byNumber[1], &metadata); err != nil {
			t.Fatalf("metadata: %v", err)
		}
		if metadata.GetHandlerId() != "share-shopping-list" || metadata.GetUserId() != "user-1" || metadata.GetOperationId() == "" {
			t.Fatalf("metadata = %v", &metadata)
		}
		if string(byNumber[2]) != "list-1" || string(byNumber[4]) != "friend@example.com" {
			t.Fatalf("payload fields = %#v", byNumber)
		}
		if _, present := byNumber[3]; present {
			t.Fatal("field 3 must remain absent")
		}
	})
	if _, err := client.ShareListInvite(t.Context(), "list-1", "friend@example.com"); err != nil {
		t.Fatalf("ShareListInvite: %v", err)
	}
}

func TestShareListInviteValidationAndResponseErrors(t *testing.T) {
	client := New(&config.Config{UserID: "user-1"})
	requests := 0
	client.httpClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return nil, fmt.Errorf("unexpected transport")
	})}
	for _, input := range [][2]string{{"", "friend@example.com"}, {"list-1", ""}, {"list-1", "not-an-email"}, {"list-1", "Person <friend@example.com>"}} {
		if _, err := client.ShareListInvite(t.Context(), input[0], input[1]); err == nil {
			t.Errorf("accepted invalid input %#v", input)
		}
	}
	if requests != 0 {
		t.Fatalf("validation sent %d requests", requests)
	}
	for _, response := range []*pb.PBShareListOperationResponse{{StatusCode: 7}, {ErrorTitle: "failed"}} {
		raw, _ := proto.Marshal(response)
		client := shareInviteClientFixture(t, raw, func([]byte) {})
		if _, err := client.ShareListInvite(t.Context(), "list-1", "friend@example.com"); err == nil {
			t.Error("accepted unsuccessful response")
		}
	}
}

func TestShareListInviteRetriesIdenticalBodyAfterRefresh(t *testing.T) {
	client := New(&config.Config{Path: filepath.Join(t.TempDir(), "config.toml"), AccessToken: "expired", RefreshToken: "refresh", UserID: "user-1"})
	var bodies [][]byte
	invites := 0
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/auth/token/refresh" {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"access_token":"fresh","refresh_token":"new"}`)), Header: make(http.Header), Request: req}, nil
		}
		invites++
		_, params, _ := mime.ParseMediaType(req.Header.Get("Content-Type"))
		part, _ := multipart.NewReader(req.Body, params["boundary"]).NextPart()
		raw, _ := io.ReadAll(part)
		bodies = append(bodies, raw)
		if invites == 1 {
			return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader("expired")), Header: make(http.Header), Request: req}, nil
		}
		response, _ := proto.Marshal(&pb.PBShareListOperationResponse{SharedUser: &pb.PBEmailUserIDPair{Email: "friend@example.com"}})
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(response)), Header: make(http.Header), Request: req}, nil
	})}
	if _, err := client.ShareListInvite(t.Context(), "list-1", "friend@example.com"); err != nil {
		t.Fatalf("ShareListInvite: %v", err)
	}
	if invites != 2 || len(bodies) != 2 || !bytes.Equal(bodies[0], bodies[1]) {
		t.Fatalf("invite retries = %d bodies = %d equal = %v", invites, len(bodies), len(bodies) == 2 && bytes.Equal(bodies[0], bodies[1]))
	}
}

func TestEnsureClientIdentifierPersistsGeneratedIdentifier(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.toml")
	cfg := &config.Config{Path: configPath}

	if err := EnsureClientIdentifier(cfg); err != nil {
		t.Fatalf("EnsureClientIdentifier returned error: %v", err)
	}
	if cfg.ClientIdentifier == "" {
		t.Fatal("ClientIdentifier was not set")
	}

	reloaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if reloaded.ClientIdentifier != cfg.ClientIdentifier {
		t.Fatalf("persisted ClientIdentifier = %q, want %q", reloaded.ClientIdentifier, cfg.ClientIdentifier)
	}
}

func TestProductLookupRefreshesAfterUnauthorized(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Path:         filepath.Join(t.TempDir(), "config.toml"),
		AccessToken:  "expired-token",
		RefreshToken: "refresh-token",
		UserID:       "user-1",
	}
	client := New(cfg)
	lookupCount := 0
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		response := func(status int, body []byte) (*http.Response, error) {
			return &http.Response{
				StatusCode: status,
				Status:     http.StatusText(status),
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}

		switch req.URL.Path {
		case "/data/product-lookup/049000028904":
			lookupCount++
			if lookupCount == 1 {
				return response(http.StatusUnauthorized, []byte("expired"))
			}
			if got := req.Header.Get("Authorization"); got != "Bearer refreshed-token" {
				t.Fatalf("retry Authorization = %q, want refreshed token", got)
			}
			body, err := proto.Marshal(&pb.PBProductLookupResponse{
				ListItem: &pb.ListItem{ProductUpc: "049000028904", Name: "Coca-Cola"},
			})
			if err != nil {
				t.Fatalf("marshal lookup response: %v", err)
			}
			return response(http.StatusOK, body)
		case "/auth/token/refresh":
			return response(http.StatusOK, []byte(`{"access_token":"refreshed-token","refresh_token":"new-refresh-token"}`))
		default:
			t.Fatalf("unexpected request path %q", req.URL.Path)
			return nil, nil
		}
	})}

	result, err := client.ProductLookup(t.Context(), "049000028904")
	if err != nil {
		t.Fatalf("ProductLookup returned error: %v", err)
	}
	if result.GetListItem().GetName() != "Coca-Cola" {
		t.Fatalf("lookup name = %q, want Coca-Cola", result.GetListItem().GetName())
	}
	if lookupCount != 2 {
		t.Fatalf("lookup requests = %d, want 2", lookupCount)
	}
	if cfg.AccessToken != "refreshed-token" {
		t.Fatalf("refreshed access token = %q, want refreshed-token", cfg.AccessToken)
	}
}

func TestRemoveListSendsVerifiedDeleteOperation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Path:        filepath.Join(t.TempDir(), "config.toml"),
		AccessToken: "access-token",
		UserID:      "user-1",
	}
	client := New(cfg)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/data/shopping-lists/update" {
			t.Fatalf("request path = %q, want /data/shopping-lists/update", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse form body: %v", err)
		}
		var operations pb.PBListOperationList
		if err := proto.Unmarshal([]byte(form.Get("operations")), &operations); err != nil {
			t.Fatalf("unmarshal operations: %v", err)
		}
		if len(operations.GetOperations()) != 1 {
			t.Fatalf("operation count = %d, want 1", len(operations.GetOperations()))
		}
		op := operations.GetOperations()[0]
		if got := op.GetMetadata().GetHandlerId(); got != "remove-shopping-list" {
			t.Fatalf("handler = %q, want remove-shopping-list", got)
		}
		if got := op.GetMetadata().GetUserId(); got != "user-1" {
			t.Fatalf("user ID = %q, want user-1", got)
		}
		if got := op.GetListId(); got != "list-1" {
			t.Fatalf("list ID = %q, want list-1", got)
		}
		if op.GetList() != nil || op.GetListItem() != nil {
			t.Fatal("delete operation must not include a list or list item payload")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     http.StatusText(http.StatusOK),
			Body:       io.NopCloser(bytes.NewReader(nil)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	if err := client.RemoveList(t.Context(), " list-1 "); err != nil {
		t.Fatalf("RemoveList returned error: %v", err)
	}
}

func TestRemoveListRejectsEmptyID(t *testing.T) {
	t.Parallel()

	client := New(&config.Config{UserID: "user-1"})
	if err := client.RemoveList(t.Context(), "  "); err == nil {
		t.Fatal("RemoveList returned nil for empty list ID")
	}
}

// decodeStarterOperation verifies the exact starter-write wire shape and
// decodes its typed protobuf payload. A successful HTTP response alone is not
// enough evidence that the correct handler was sent.
func decodeStarterOperation(t *testing.T, req *http.Request) *pb.PBStarterListOperation {
	t.Helper()
	if req.URL.Path != "/data/starter-lists/update" {
		t.Fatalf("request path = %q, want /data/starter-lists/update", req.URL.Path)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer access-token" {
		t.Fatalf("Authorization = %q, want Bearer access-token", got)
	}
	mediaType, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" {
		t.Fatalf("content type = %q, want multipart/form-data: %v", req.Header.Get("Content-Type"), err)
	}
	part, err := multipart.NewReader(req.Body, params["boundary"]).NextPart()
	if err != nil {
		t.Fatalf("read operations part: %v", err)
	}
	if part.FormName() != "operations" || part.Header.Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("unexpected operations part: name=%q content-type=%q", part.FormName(), part.Header.Get("Content-Type"))
	}
	raw, err := io.ReadAll(part)
	if err != nil {
		t.Fatalf("read operations part: %v", err)
	}
	var operations pb.PBStarterListOperationList
	if err := proto.Unmarshal(raw, &operations); err != nil {
		t.Fatalf("unmarshal starter operations: %v", err)
	}
	if len(operations.GetOperations()) != 1 {
		t.Fatalf("operation count = %d, want 1", len(operations.GetOperations()))
	}
	return operations.GetOperations()[0]
}

func TestAddStarterListItemSendsTypedOperation(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{Path: filepath.Join(t.TempDir(), "config.toml"), AccessToken: "access-token", UserID: "user-1"}
	client := New(cfg)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		op := decodeStarterOperation(t, req)
		if got := op.GetMetadata().GetHandlerId(); got != "bulk-add-list-items" {
			t.Fatalf("handler = %q, want bulk-add-list-items", got)
		}
		if got := op.GetListId(); got != "starter-1" || op.GetList().GetIdentifier() != "starter-1" {
			t.Fatalf("starter list IDs = (%q, %q), want starter-1", got, op.GetList().GetIdentifier())
		}
		if op.GetListItemId() != "" || op.GetListItem() != nil {
			t.Fatalf("add payload used legacy list-item fields: item_id=%q item=%v", op.GetListItemId(), op.GetListItem())
		}
		if len(op.GetList().GetItems()) != 1 || op.GetList().GetItems()[0].GetUserId() != "user-1" {
			t.Fatalf("add payload IDs = list=%q items=%d user=%q", op.GetList().GetIdentifier(), len(op.GetList().GetItems()), op.GetList().GetItems()[0].GetUserId())
		}
		if op.GetList().GetItems()[0].GetName() != "Milk" || op.GetList().GetItems()[0].GetQuantity() != "2" {
			t.Fatalf("item payload = %v, want Milk/2", op.GetList().GetItems()[0])
		}
		if op.GetList().GetItems()[0].GetCategoryMatchId() != "other" {
			t.Fatalf("item category match = %q, want AnyList's neutral fallback %q", op.GetList().GetItems()[0].GetCategoryMatchId(), "other")
		}
		return &http.Response{StatusCode: http.StatusOK, Status: http.StatusText(http.StatusOK), Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header), Request: req}, nil
	})}
	item := &pb.ListItem{Name: "Milk", Quantity: "2"}
	itemID, err := client.AddStarterListItem(t.Context(), " starter-1 ", item)
	if err != nil || itemID == "" {
		t.Fatalf("AddStarterListItem = (%q, %v), want non-empty ID and nil error", itemID, err)
	}
	if item.GetIdentifier() != "" || item.GetListId() != "" || item.GetUserId() != "" {
		t.Fatalf("client mutated caller item: %v", item)
	}
}

func TestRemoveStarterListItemSendsTypedOperation(t *testing.T) {
	t.Parallel()
	client := New(&config.Config{AccessToken: "access-token", UserID: "user-1"})
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		op := decodeStarterOperation(t, req)
		if got := op.GetMetadata().GetHandlerId(); got != "bulk-remove-list-items" {
			t.Fatalf("handler = %q, want bulk-remove-list-items", got)
		}
		if op.GetListId() != "starter-1" || op.GetList().GetIdentifier() != "starter-1" {
			t.Fatalf("IDs = (%q, %q), want (starter-1, starter-1)", op.GetListId(), op.GetList().GetIdentifier())
		}
		if op.GetListItemId() != "" || op.GetListItem() != nil {
			t.Fatalf("remove payload used legacy list-item fields: item_id=%q item=%v", op.GetListItemId(), op.GetListItem())
		}
		want := &pb.ListItem{Identifier: "item-1", ListId: "starter-1", Name: "Milk", UserId: "user-1", CategoryMatchId: "other"}
		if len(op.GetList().GetItems()) != 1 || !proto.Equal(op.GetList().GetItems()[0], want) {
			t.Fatalf("remove payload = %v, want complete current item %v", op.GetList().GetItems(), want)
		}
		return &http.Response{StatusCode: http.StatusOK, Status: http.StatusText(http.StatusOK), Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header), Request: req}, nil
	})}
	item := &pb.ListItem{Identifier: "item-1", ListId: "starter-1", Name: "Milk", UserId: "user-1"}
	if err := client.RemoveStarterListItem(t.Context(), " starter-1 ", item); err != nil {
		t.Fatalf("RemoveStarterListItem returned error: %v", err)
	}
}

func TestCreateListSendsVerifiedCreateOperation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Path:        filepath.Join(t.TempDir(), "config.toml"),
		AccessToken: "access-token",
		UserID:      "user-1",
	}
	client := New(cfg)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/data/shopping-lists/update" {
			t.Fatalf("request path = %q, want /data/shopping-lists/update", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse form body: %v", err)
		}
		var operations pb.PBListOperationList
		if err := proto.Unmarshal([]byte(form.Get("operations")), &operations); err != nil {
			t.Fatalf("unmarshal operations: %v", err)
		}
		if len(operations.GetOperations()) != 1 {
			t.Fatalf("operation count = %d, want 1", len(operations.GetOperations()))
		}
		op := operations.GetOperations()[0]
		if got := op.GetMetadata().GetHandlerId(); got != "new-shopping-list" {
			t.Fatalf("handler = %q, want new-shopping-list", got)
		}
		if got := op.GetMetadata().GetUserId(); got != "user-1" {
			t.Fatalf("user ID = %q, want user-1", got)
		}
		if op.GetList() == nil {
			t.Fatal("create operation is missing list payload")
		}
		if got := op.GetList().GetName(); got != "New List" {
			t.Fatalf("list name = %q, want New List", got)
		}
		if got := op.GetList().GetCreator(); got != "user-1" {
			t.Fatalf("list creator = %q, want user-1", got)
		}
		if op.GetList().GetIdentifier() == "" || op.GetListId() != op.GetList().GetIdentifier() {
			t.Fatalf("list IDs = operation %q, payload %q; want equal non-empty IDs", op.GetListId(), op.GetList().GetIdentifier())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     http.StatusText(http.StatusOK),
			Body:       io.NopCloser(bytes.NewReader(nil)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	list, err := client.CreateList(t.Context(), " New List ")
	if err != nil {
		t.Fatalf("CreateList returned error: %v", err)
	}
	if list.GetName() != "New List" || list.GetIdentifier() == "" {
		t.Fatalf("returned list = %#v, want trimmed name and generated ID", list)
	}
}

func TestRenameListSendsFullRenameOperation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{AccessToken: "access-token", UserID: "user-1"}
	client := New(cfg)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/data/shopping-lists/update" {
			t.Fatalf("request path = %q", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse form body: %v", err)
		}
		var operations pb.PBListOperationList
		if err := proto.Unmarshal([]byte(form.Get("operations")), &operations); err != nil {
			t.Fatalf("unmarshal operations: %v", err)
		}
		if len(operations.GetOperations()) != 1 {
			t.Fatalf("operation count = %d", len(operations.GetOperations()))
		}
		op := operations.GetOperations()[0]
		if op.GetMetadata().GetHandlerId() != "rename-list" || op.GetListId() != "list-1" || op.GetOriginalValue() != "Old" {
			t.Fatalf("operation = %#v", op)
		}
		if op.GetList().GetName() != "New" || len(op.GetList().GetItems()) != 1 || op.GetList().GetItems()[0].GetName() != "Milk" {
			t.Fatalf("updated list did not preserve payload: %#v", op.GetList())
		}
		return &http.Response{StatusCode: http.StatusOK, Status: http.StatusText(http.StatusOK), Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header), Request: req}, nil
	})}

	list := &pb.ShoppingList{Identifier: "list-1", Name: "Old", Items: []*pb.ListItem{{Name: "Milk"}}}
	updated := proto.Clone(list).(*pb.ShoppingList)
	updated.Name = "New"
	if err := client.RenameList(t.Context(), "list-1", "Old", "New", updated); err != nil {
		t.Fatalf("RenameList returned error: %v", err)
	}
	if list.GetName() != "Old" {
		t.Fatalf("RenameList mutated original list name to %q", list.GetName())
	}
}

func TestCreateListRejectsEmptyName(t *testing.T) {
	t.Parallel()

	client := New(&config.Config{UserID: "user-1"})
	if _, err := client.CreateList(t.Context(), "  "); err == nil {
		t.Fatal("CreateList returned nil error for empty list name")
	}
}

func TestUpdateItemFieldsSendsRenameOperation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Path:        filepath.Join(t.TempDir(), "config.toml"),
		AccessToken: "access-token",
		UserID:      "user-1",
	}
	client := New(cfg)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse form body: %v", err)
		}
		var operations pb.PBListOperationList
		if err := proto.Unmarshal([]byte(form.Get("operations")), &operations); err != nil {
			t.Fatalf("unmarshal operations: %v", err)
		}
		if len(operations.GetOperations()) != 1 {
			t.Fatalf("operation count = %d, want 1", len(operations.GetOperations()))
		}
		op := operations.GetOperations()[0]
		if got := op.GetMetadata().GetHandlerId(); got != "set-list-item-name" {
			t.Fatalf("handler = %q, want set-list-item-name", got)
		}
		if got := op.GetListId(); got != "list-1" || op.GetListItemId() != "item-1" {
			t.Fatalf("operation IDs = %q/%q, want list-1/item-1", got, op.GetListItemId())
		}
		if got := op.GetUpdatedValue(); got != "Renamed Milk" {
			t.Fatalf("updated value = %q, want Renamed Milk", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     http.StatusText(http.StatusOK),
			Body:       io.NopCloser(bytes.NewReader(nil)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	if err := client.UpdateItemFields(t.Context(), "list-1", "item-1", map[string]string{"name": "Renamed Milk"}); err != nil {
		t.Fatalf("UpdateItemFields returned error: %v", err)
	}
}

func TestAddItemWithOptionsSendsOnlyExplicitItemFields(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Path:        filepath.Join(t.TempDir(), "config.toml"),
		AccessToken: "access-token",
		UserID:      "user-1",
	}
	client := New(cfg)
	requestCount := 0
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		if req.URL.Path != "/data/shopping-lists/update" {
			t.Fatalf("request path = %q, want /data/shopping-lists/update", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse form body: %v", err)
		}
		var operations pb.PBListOperationList
		if err := proto.Unmarshal([]byte(form.Get("operations")), &operations); err != nil {
			t.Fatalf("unmarshal operations: %v", err)
		}
		if len(operations.GetOperations()) != 1 {
			t.Fatalf("operation count = %d, want 1", len(operations.GetOperations()))
		}
		op := operations.GetOperations()[0]
		if got := op.GetMetadata().GetHandlerId(); got != "add-shopping-list-item" {
			t.Fatalf("handler = %q, want add-shopping-list-item", got)
		}
		item := op.GetListItem()
		if item == nil {
			t.Fatal("add operation is missing item payload")
		}
		if item.GetName() != "Shredded Sharp Cheddar Cheese" {
			t.Fatalf("item name = %q, want explicit name", item.GetName())
		}
		if item.GetQuantity() != "1/2 cup" || item.GetDetails() != "8 oz bag" {
			t.Fatalf("item fields = quantity %q details %q, want explicit values", item.GetQuantity(), item.GetDetails())
		}
		if item.GetProductUpc() != "049000028904" || item.GetPackageSizePb().GetRawPackageSize() != "8 oz bag" {
			t.Fatalf("product metadata was not preserved: upc=%q package=%q", item.GetProductUpc(), item.GetPackageSizePb().GetRawPackageSize())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     http.StatusText(http.StatusOK),
			Body:       io.NopCloser(bytes.NewReader(nil)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	itemID, err := client.AddItemWithOptionsAndID(t.Context(), "list-1", "Shredded Sharp Cheddar Cheese", ItemAddOptions{
		Quantity:    "1/2 cup",
		Details:     "8 oz bag",
		ProductUpc:  "049000028904",
		PackageSize: "8 oz bag",
	})
	if err != nil {
		t.Fatalf("AddItemWithOptionsAndID returned error: %v", err)
	}
	if itemID == "" {
		t.Fatal("AddItemWithOptionsAndID returned an empty stable item ID")
	}
	if requestCount != 1 {
		t.Fatalf("request count = %d, want exactly one explicit add request", requestCount)
	}
}

func TestUpdateItemPackageSizeSendsFullItemOperation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Path:        filepath.Join(t.TempDir(), "config.toml"),
		AccessToken: "access-token",
		UserID:      "user-1",
	}
	client := New(cfg)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse form body: %v", err)
		}
		var operations pb.PBListOperationList
		if err := proto.Unmarshal([]byte(form.Get("operations")), &operations); err != nil {
			t.Fatalf("unmarshal operations: %v", err)
		}
		op := operations.GetOperations()[0]
		if got := op.GetMetadata().GetHandlerId(); got != "set-list-item-package-size" {
			t.Fatalf("handler = %q, want set-list-item-package-size", got)
		}
		if got := op.GetListId(); got != "list-1" || op.GetListItemId() != "item-1" {
			t.Fatalf("operation IDs = %q/%q, want list-1/item-1", got, op.GetListItemId())
		}
		if got := op.GetListItem().GetName(); got != "Milk" {
			t.Fatalf("full item name = %q, want Milk", got)
		}
		if got := op.GetListItem().GetProductUpc(); got != "049000028904" {
			t.Fatalf("full item barcode = %q, want preserved barcode", got)
		}
		if got := op.GetListItem().GetPackageSizePb().GetRawPackageSize(); got != "12 oz carton" {
			t.Fatalf("package size = %q, want 12 oz carton", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     http.StatusText(http.StatusOK),
			Body:       io.NopCloser(bytes.NewReader(nil)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	err := client.UpdateItemPackageSize(t.Context(), "list-1", &pb.ListItem{
		Identifier: "item-1",
		ListId:     "list-1",
		Name:       "Milk",
		ProductUpc: "049000028904",
	}, &pb.PBItemPackageSize{RawPackageSize: "12 oz carton"})
	if err != nil {
		t.Fatalf("UpdateItemPackageSize returned error: %v", err)
	}
}

func TestUploadPhotoBuildsMultipartRequest(t *testing.T) {
	t.Parallel()

	photoPath := filepath.Join(t.TempDir(), "source.png")
	photoBytes := []byte{0xff, 0xd8, 0xff, 0xe0, 'J', 'F', 'I', 'F'}
	if err := os.WriteFile(photoPath, photoBytes, 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	cfg := &config.Config{
		Path:             filepath.Join(t.TempDir(), "config.toml"),
		AccessToken:      "access-token",
		ClientIdentifier: "client-1",
		UserID:           "user-1",
	}
	client := New(cfg)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/data/photos/upload" {
			t.Fatalf("request = %s %s, want POST /data/photos/upload", req.Method, req.URL.Path)
		}
		if got := req.Header.Get("Authorization"); got != "Bearer access-token" {
			t.Fatalf("Authorization = %q, want bearer header", got)
		}
		if got := req.Header.Get("X-AnyLeaf-API-Version"); got != "3" {
			t.Fatalf("API version = %q, want 3", got)
		}
		mediaType := req.Header.Get("Content-Type")
		if !strings.HasPrefix(mediaType, "multipart/form-data; boundary=") {
			t.Fatalf("Content-Type = %q, want multipart form", mediaType)
		}
		reader, err := req.MultipartReader()
		if err != nil {
			t.Fatalf("MultipartReader returned error: %v", err)
		}
		part, err := reader.NextPart()
		if err != nil {
			t.Fatalf("NextPart returned error: %v", err)
		}
		if part.FormName() != "photo" {
			t.Fatalf("form field = %q, want photo", part.FormName())
		}
		filename := part.FileName()
		if !strings.HasSuffix(filename, ".jpg") {
			t.Fatalf("filename = %q, want UUID.jpg", filename)
		}
		photoID := strings.TrimSuffix(filename, ".jpg")
		if !isHex32(photoID) {
			t.Fatalf("filename UUID = %q, want 32 lowercase hex characters", photoID)
		}
		if got := part.Header.Get("Content-Type"); got != "image/jpeg" {
			t.Fatalf("photo content type = %q, want image/jpeg", got)
		}
		got, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("reading photo part: %v", err)
		}
		if !bytes.Equal(got, photoBytes) {
			t.Fatalf("photo bytes = %v, want %v", got, photoBytes)
		}
		filenameField, err := reader.NextPart()
		if err != nil {
			t.Fatalf("NextPart filename returned error: %v", err)
		}
		if filenameField.FormName() != "filename" {
			t.Fatalf("filename field = %q, want filename", filenameField.FormName())
		}
		filenameValue, err := io.ReadAll(filenameField)
		if err != nil {
			t.Fatalf("reading filename field: %v", err)
		}
		if string(filenameValue) != photoID+".jpg" {
			t.Fatalf("filename field value = %q, want %q", filenameValue, photoID+".jpg")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     http.StatusText(http.StatusOK),
			Body:       io.NopCloser(bytes.NewReader(nil)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	photoID, info, err := client.UploadPhoto(t.Context(), photoPath)
	if err != nil {
		t.Fatalf("UploadPhoto returned error: %v", err)
	}
	if !isHex32(photoID) {
		t.Fatalf("photo ID = %q, want 32 lowercase hex characters", photoID)
	}
	if info.Size != int64(len(photoBytes)) || info.ContentType != "image/jpeg" {
		t.Fatalf("photo info = %#v, want JPEG size %d", info, len(photoBytes))
	}
}

func TestInspectPhotoFileAcceptsExtendedImageTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ext  string
		data []byte
		want string
	}{
		{name: "bmp", ext: ".bmp", data: []byte{'B', 'M', 0, 0}, want: "image/bmp"},
		{name: "tiff-little-endian", ext: ".tiff", data: []byte{'I', 'I', '*', 0}, want: "image/tiff"},
		{name: "tiff-big-endian", ext: ".tiff", data: []byte{'M', 'M', 0, '*'}, want: "image/tiff"},
		{name: "avif", ext: ".avif", data: []byte{0, 0, 0, 0, 'f', 't', 'y', 'p', 'a', 'v', 'i', 'f'}, want: "image/avif"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tt.name+tt.ext)
			if err := os.WriteFile(path, tt.data, 0o600); err != nil {
				t.Fatalf("WriteFile returned error: %v", err)
			}
			info, err := InspectPhotoFile(path)
			if err != nil {
				t.Fatalf("InspectPhotoFile returned error: %v", err)
			}
			if info.ContentType != tt.want {
				t.Fatalf("content type = %q, want %q", info.ContentType, tt.want)
			}
		})
	}
}

func TestFormBasedRequestReplaysBodyAfterRefresh(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Path:         filepath.Join(t.TempDir(), "config.toml"),
		AccessToken:  "expired-token",
		RefreshToken: "refresh-token",
		UserID:       "user-1",
	}
	client := New(cfg)
	refreshCount := 0
	requestCount := 0
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		response := func(status int, body []byte) (*http.Response, error) {
			return &http.Response{
				StatusCode: status,
				Status:     http.StatusText(status),
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}

		switch req.URL.Path {
		case "/data/shopping-lists/update":
			requestCount++
			wantToken := "Bearer expired-token"
			if requestCount > 1 {
				wantToken = "Bearer refreshed-token"
			}
			if got := req.Header.Get("Authorization"); got != wantToken {
				t.Fatalf("Authorization = %q, want %q", got, wantToken)
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			form, err := url.ParseQuery(string(body))
			if err != nil {
				t.Fatalf("parse form body: %v", err)
			}
			var operations pb.PBListOperationList
			if err := proto.Unmarshal([]byte(form.Get("operations")), &operations); err != nil {
				t.Fatalf("unmarshal operations: %v", err)
			}
			op := operations.GetOperations()[0]
			if got := op.GetMetadata().GetHandlerId(); got != "add-shopping-list-item" {
				t.Fatalf("handler = %q, want add-shopping-list-item", got)
			}
			if got := op.GetMetadata().GetUserId(); got != "user-1" {
				t.Fatalf("user ID = %q, want user-1", got)
			}
			if got := op.GetListId(); got != "list-1" {
				t.Fatalf("list ID = %q, want list-1", got)
			}
			if requestCount == 1 {
				return response(http.StatusUnauthorized, []byte("expired"))
			}
			return response(http.StatusOK, nil)
		case "/auth/token/refresh":
			refreshCount++
			return response(http.StatusOK, []byte(`{"access_token":"refreshed-token","refresh_token":"new-refresh-token"}`))
		default:
			t.Fatalf("unexpected request path %q", req.URL.Path)
			return nil, nil
		}
	})}

	if err := client.AddItemWithOptions(t.Context(), "list-1", "Milk", ItemAddOptions{Quantity: "2"}); err != nil {
		t.Fatalf("AddItemWithOptions returned error: %v", err)
	}
	if refreshCount != 1 {
		t.Fatalf("refresh calls = %d, want 1", refreshCount)
	}
	if requestCount != 2 {
		t.Fatalf("request count = %d, want 2", requestCount)
	}
	if cfg.AccessToken != "refreshed-token" {
		t.Fatalf("refreshed access token = %q, want refreshed-token", cfg.AccessToken)
	}
}

func TestMultipartRequestReplaysBodyAfterRefresh(t *testing.T) {
	t.Parallel()

	photoPath := filepath.Join(t.TempDir(), "source.png")
	photoBytes := []byte{0xff, 0xd8, 0xff, 0xe0, 'J', 'F', 'I', 'F'}
	if err := os.WriteFile(photoPath, photoBytes, 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	cfg := &config.Config{
		Path:             filepath.Join(t.TempDir(), "config.toml"),
		AccessToken:      "expired-token",
		RefreshToken:     "refresh-token",
		ClientIdentifier: "client-1",
		UserID:           "user-1",
	}
	client := New(cfg)
	requestCount := 0
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		response := func(status int, body []byte) (*http.Response, error) {
			return &http.Response{
				StatusCode: status,
				Status:     http.StatusText(status),
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}

		switch req.URL.Path {
		case "/data/photos/upload":
			requestCount++
			wantToken := "Bearer expired-token"
			if requestCount > 1 {
				wantToken = "Bearer refreshed-token"
			}
			if got := req.Header.Get("Authorization"); got != wantToken {
				t.Fatalf("Authorization = %q, want %q", got, wantToken)
			}
			boundary := strings.SplitN(req.Header.Get("Content-Type"), "boundary=", 2)[1]
			multipartReader := multipart.NewReader(req.Body, boundary)
			part, err := multipartReader.NextPart()
			if err != nil {
				t.Fatalf("NextPart returned error: %v", err)
			}
			if part.FormName() != "photo" {
				t.Fatalf("form field = %q, want photo", part.FormName())
			}
			got, err := io.ReadAll(part)
			if err != nil {
				t.Fatalf("reading photo part: %v", err)
			}
			if !bytes.Equal(got, photoBytes) {
				t.Fatalf("photo bytes = %v, want %v", got, photoBytes)
			}
			filenameField, err := multipartReader.NextPart()
			if err != nil {
				t.Fatalf("NextPart filename returned error: %v", err)
			}
			if filenameField.FormName() != "filename" {
				t.Fatalf("filename field = %q, want filename", filenameField.FormName())
			}
			if requestCount == 1 {
				return response(http.StatusUnauthorized, []byte("expired"))
			}
			return response(http.StatusOK, nil)
		case "/auth/token/refresh":
			return response(http.StatusOK, []byte(`{"access_token":"refreshed-token","refresh_token":"new-refresh-token"}`))
		default:
			t.Fatalf("unexpected request path %q", req.URL.Path)
			return nil, nil
		}
	})}

	if _, _, err := client.UploadPhoto(t.Context(), photoPath); err != nil {
		t.Fatalf("UploadPhoto returned error: %v", err)
	}
	if requestCount != 2 {
		t.Fatalf("upload requests = %d, want 2", requestCount)
	}
}

func TestSecond401FailsAfterExactlyOneRefresh(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Path:         filepath.Join(t.TempDir(), "config.toml"),
		AccessToken:  "expired-token",
		RefreshToken: "refresh-token",
		UserID:       "user-1",
	}
	client := New(cfg)
	refreshCount := 0
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		response := func(status int, body []byte) (*http.Response, error) {
			return &http.Response{
				StatusCode: status,
				Status:     http.StatusText(status),
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}

		switch req.URL.Path {
		case "/data/user-data/get":
			return response(http.StatusUnauthorized, []byte("expired"))
		case "/auth/token/refresh":
			refreshCount++
			return response(http.StatusOK, []byte(`{"access_token":"refreshed-token","refresh_token":"new-refresh-token"}`))
		default:
			t.Fatalf("unexpected request path %q", req.URL.Path)
			return nil, nil
		}
	})}

	_, err := client.GetUserData(t.Context())
	if err == nil {
		t.Fatal("GetUserData returned nil error after second 401")
	}
	if !strings.Contains(err.Error(), "unauthorized after token refresh") {
		t.Fatalf("error = %q, want 'unauthorized after token refresh'", err)
	}
	if refreshCount != 1 {
		t.Fatalf("refresh calls = %d, want 1", refreshCount)
	}
}

func TestRefreshEndpointCalledExactlyOnce(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Path:         filepath.Join(t.TempDir(), "config.toml"),
		AccessToken:  "expired-token",
		RefreshToken: "refresh-token",
		UserID:       "user-1",
	}
	client := New(cfg)
	refreshCount := 0
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		response := func(status int, body []byte) (*http.Response, error) {
			return &http.Response{
				StatusCode: status,
				Status:     http.StatusText(status),
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}

		switch req.URL.Path {
		case "/data/user-data/get":
			return response(http.StatusUnauthorized, []byte("expired"))
		case "/auth/token/refresh":
			refreshCount++
			return response(http.StatusOK, []byte(`{"access_token":"refreshed-token","refresh_token":"new-refresh-token"}`))
		default:
			t.Fatalf("unexpected request path %q", req.URL.Path)
			return nil, nil
		}
	})}

	_, err := client.GetUserData(t.Context())
	if err == nil {
		t.Fatal("GetUserData returned nil error after second 401")
	}
	if !strings.Contains(err.Error(), "unauthorized after token refresh") {
		t.Fatalf("error = %q, want 'unauthorized after token refresh'", err)
	}
	if refreshCount != 1 {
		t.Fatalf("refresh calls = %d, want 1", refreshCount)
	}
}

func isHex32(value string) bool {
	if len(value) != 32 {
		return false
	}
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

func TestUploadPhotoRejectsOversizeAndUnsupportedFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	oversize := filepath.Join(dir, "oversize.jpg")
	if err := os.WriteFile(oversize, bytes.Repeat([]byte{'x'}, maxPhotoUploadBytes+1), 0o600); err != nil {
		t.Fatalf("WriteFile oversize returned error: %v", err)
	}
	if _, err := InspectPhotoFile(oversize); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversize validation error = %v, want size error", err)
	}

	unsupported := filepath.Join(dir, "not-an-image.txt")
	if err := os.WriteFile(unsupported, []byte("not an image"), 0o600); err != nil {
		t.Fatalf("WriteFile unsupported returned error: %v", err)
	}
	if _, err := InspectPhotoFile(unsupported); err == nil || !strings.Contains(err.Error(), "unsupported photo type") {
		t.Fatalf("unsupported validation error = %v, want type error", err)
	}
}

func TestSetItemPhotoIDSendsTypedOperation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Path:        filepath.Join(t.TempDir(), "config.toml"),
		AccessToken: "access-token",
		UserID:      "user-1",
	}
	client := New(cfg)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse form body: %v", err)
		}
		var operations pb.PBListOperationList
		if err := proto.Unmarshal([]byte(form.Get("operations")), &operations); err != nil {
			t.Fatalf("unmarshal operations: %v", err)
		}
		if len(operations.GetOperations()) != 1 {
			t.Fatalf("operation count = %d, want 1", len(operations.GetOperations()))
		}
		op := operations.GetOperations()[0]
		if got := op.GetMetadata().GetHandlerId(); got != "set-list-item-photo-id" {
			t.Fatalf("handler = %q, want set-list-item-photo-id", got)
		}
		if op.GetListId() != "list-1" || op.GetListItemId() != "item-1" || op.GetUpdatedValue() != "photo-1" {
			t.Fatalf("operation = list %q item %q value %q, want list-1/item-1/photo-1", op.GetListId(), op.GetListItemId(), op.GetUpdatedValue())
		}
		if op.GetListItem() != nil {
			t.Fatal("photo-ID operation must not include a full list item")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     http.StatusText(http.StatusOK),
			Body:       io.NopCloser(bytes.NewReader(nil)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	if err := client.SetItemPhotoID(t.Context(), " list-1 ", " item-1 ", " photo-1 "); err != nil {
		t.Fatalf("SetItemPhotoID returned error: %v", err)
	}
}

func TestSetItemPhotoIDRejectsMissingIDs(t *testing.T) {
	t.Parallel()

	client := New(&config.Config{UserID: "user-1"})
	if err := client.SetItemPhotoID(t.Context(), "list-1", "item-1", " "); err == nil {
		t.Fatal("SetItemPhotoID returned nil for empty photo ID")
	}
}

func TestSaveRecipeCollectionSendsVerifiedOperation(t *testing.T) {
	t.Parallel()

	client := newRecipeCollectionTestClient(t, func(op *pb.PBRecipeOperation) {
		if got := op.GetMetadata().GetHandlerId(); got != "save-recipe-collection" {
			t.Fatalf("handler = %q, want save-recipe-collection", got)
		}
		if got := op.GetRecipeDataId(); got != "recipe-data-1" {
			t.Fatalf("recipe data ID = %q, want recipe-data-1", got)
		}
		collection := op.GetRecipeCollection()
		if collection == nil {
			t.Fatal("save operation is missing collection payload")
		}
		if collection.GetIdentifier() != "collection-1" || collection.GetName() != "Dinners" {
			t.Fatalf("collection = %#v, want collection-1/Dinners", collection)
		}
		if got := op.GetRecipeCollectionIds(); len(got) != 1 || got[0] != "collection-1" {
			t.Fatalf("collection IDs = %v, want [collection-1]", got)
		}
		if len(collection.GetRecipeIds()) != 2 || collection.GetRecipeIds()[1] != "recipe-2" {
			t.Fatalf("recipe IDs = %v, want [recipe-1 recipe-2]", collection.GetRecipeIds())
		}
	})

	err := client.SaveRecipeCollection(t.Context(), "recipe-data-1", &pb.PBRecipeCollection{
		Identifier: "collection-1",
		Name:       "Dinners",
		RecipeIds:  []string{"recipe-1", "recipe-2"},
	})
	if err != nil {
		t.Fatalf("SaveRecipeCollection returned error: %v", err)
	}
}

func TestSaveRecipeSendsCompleteRecipePayload(t *testing.T) {
	t.Parallel()

	client := newRecipeCollectionTestClient(t, func(op *pb.PBRecipeOperation) {
		if got := op.GetMetadata().GetHandlerId(); got != "save-recipe" {
			t.Fatalf("handler = %q, want save-recipe", got)
		}
		if got := op.GetRecipeDataId(); got != "recipe-data-1" {
			t.Fatalf("recipe data ID = %q, want recipe-data-1", got)
		}
		if !op.GetIsNewRecipeFromWebImport() {
			t.Fatal("web-import marker = false, want true")
		}
		recipe := op.GetRecipe()
		if recipe == nil {
			t.Fatal("save operation is missing recipe payload")
		}
		if recipe.GetIdentifier() != "recipe-1" || recipe.GetName() != "Meatloaf" {
			t.Fatalf("recipe identity = %q/%q, want recipe-1/Meatloaf", recipe.GetIdentifier(), recipe.GetName())
		}
		if len(recipe.GetIngredients()) != 1 || recipe.GetIngredients()[0].GetRawIngredient() != "1 lb ground beef" {
			t.Fatalf("ingredients were not preserved: %#v", recipe.GetIngredients())
		}
		if len(recipe.GetPreparationSteps()) != 1 || recipe.GetPreparationSteps()[0] != "Bake until done" {
			t.Fatalf("preparation steps were not preserved: %#v", recipe.GetPreparationSteps())
		}
	})

	err := client.SaveRecipe(t.Context(), "recipe-data-1", &pb.PBRecipe{
		Identifier:       "recipe-1",
		Name:             "Meatloaf",
		Ingredients:      []*pb.PBIngredient{{RawIngredient: "1 lb ground beef", Name: "ground beef", Quantity: "1 lb"}},
		PreparationSteps: []string{"Bake until done"},
	}, true)
	if err != nil {
		t.Fatalf("SaveRecipe returned error: %v", err)
	}
}

func TestRemoveRecipeCollectionSendsVerifiedOperation(t *testing.T) {
	t.Parallel()

	client := newRecipeCollectionTestClient(t, func(op *pb.PBRecipeOperation) {
		if got := op.GetMetadata().GetHandlerId(); got != "remove-recipe-collection" {
			t.Fatalf("handler = %q, want remove-recipe-collection", got)
		}
		if got := op.GetRecipeDataId(); got != "recipe-data-1" {
			t.Fatalf("recipe data ID = %q, want recipe-data-1", got)
		}
		if got := op.GetRecipeCollectionIds(); len(got) != 1 || got[0] != "collection-1" {
			t.Fatalf("collection IDs = %v, want [collection-1]", got)
		}
		if op.GetRecipeCollection() == nil || op.GetRecipeCollection().GetIdentifier() != "collection-1" {
			t.Fatalf("remove collection payload = %#v, want collection-1", op.GetRecipeCollection())
		}
	})

	err := client.RemoveRecipeCollection(t.Context(), "recipe-data-1", &pb.PBRecipeCollection{Identifier: "collection-1"})
	if err != nil {
		t.Fatalf("RemoveRecipeCollection returned error: %v", err)
	}
}

// newListFolderTestClient returns a Client whose transport decodes the
// "operations" form field of POST /data/list-folders/update as a
// PBListFolderOperationList and hands the single captured operation to inspect.
func newListFolderTestClient(t *testing.T, inspect func(*pb.PBListFolderOperation)) *Client {
	t.Helper()

	cfg := &config.Config{
		Path:        filepath.Join(t.TempDir(), "config.toml"),
		AccessToken: "access-token",
		UserID:      "user-1",
	}
	client := New(cfg)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/data/list-folders/update" {
			t.Fatalf("request path = %q, want /data/list-folders/update", req.URL.Path)
		}
		_, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("parse multipart content type: %v", err)
		}
		reader := multipart.NewReader(req.Body, params["boundary"])
		var operationsBytes []byte
		for {
			part, nextErr := reader.NextPart()
			if nextErr == io.EOF {
				break
			}
			if nextErr != nil {
				t.Fatalf("read multipart body: %v", nextErr)
			}
			if part.FormName() == "operations" {
				operationsBytes, err = io.ReadAll(part)
				if err != nil {
					t.Fatalf("read operations part: %v", err)
				}
				break
			}
		}
		var operations pb.PBListFolderOperationList
		if err := proto.Unmarshal(operationsBytes, &operations); err != nil {
			t.Fatalf("unmarshal operations: %v", err)
		}
		if len(operations.GetOperations()) != 1 {
			t.Fatalf("operation count = %d, want 1", len(operations.GetOperations()))
		}
		inspect(operations.GetOperations()[0])
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     http.StatusText(http.StatusOK),
			Body:       io.NopCloser(bytes.NewReader(nil)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}
	return client
}

func TestSaveListFolderSendsTypedCreateOperation(t *testing.T) {
	t.Parallel()

	client := newListFolderTestClient(t, func(op *pb.PBListFolderOperation) {
		if got := op.GetMetadata().GetHandlerId(); got != "save-list-folder" {
			t.Errorf("handler = %q, want save-list-folder", got)
		}
		if got := op.GetMetadata().GetUserId(); got != "user-1" {
			t.Errorf("user ID = %q, want user-1", got)
		}
		if op.GetMetadata().GetOperationId() == "" {
			t.Error("operation ID must not be empty")
		}
		if got := op.GetListDataId(); got != "list-1" {
			t.Errorf("listDataId = %q, want list-1", got)
		}
		folder := op.GetListFolder()
		if folder == nil {
			t.Fatal("list folder payload is nil")
		}
		if folder.GetIdentifier() != "folder-new" {
			t.Errorf("create folder identifier = %q, want folder-new", folder.GetIdentifier())
		}
		if folder.GetName() != "Produce" {
			t.Errorf("folder name = %q, want Produce", folder.GetName())
		}
		if len(folder.GetItems()) != 1 {
			t.Fatalf("folder item count = %d, want 1", len(folder.GetItems()))
		}
		item := folder.GetItems()[0]
		if item.GetIdentifier() != "item-1" || item.GetItemType() != 5 {
			t.Errorf("folder item = %q/type %d, want item-1/type 5", item.GetIdentifier(), item.GetItemType())
		}
		if got := op.GetUpdatedParentFolderId(); got != "parent-1" {
			t.Errorf("updated parent = %q, want parent-1", got)
		}
		if got := op.GetOriginalParentFolderId(); got != "" {
			t.Errorf("original parent = %q, want empty", got)
		}
		if len(op.GetFolderItems()) != 0 {
			t.Errorf("top-level folderItems count = %d, want 0", len(op.GetFolderItems()))
		}
	})

	folder := &pb.PBListFolder{Identifier: "folder-new", Name: "Produce", Items: []*pb.PBListFolderItem{{Identifier: "item-1", ItemType: 5}}}
	if err := client.SaveListFolder(t.Context(), "list-1", folder, "parent-1"); err != nil {
		t.Fatalf("SaveListFolder returned error: %v", err)
	}
}

func TestSaveListFolderSendsTypedUpdateOperation(t *testing.T) {
	t.Parallel()

	client := newListFolderTestClient(t, func(op *pb.PBListFolderOperation) {
		if got := op.GetMetadata().GetHandlerId(); got != "save-list-folder" {
			t.Errorf("handler = %q, want save-list-folder", got)
		}
		folder := op.GetListFolder()
		if folder == nil || folder.GetIdentifier() != "folder-1" || folder.GetName() != "Dairy" {
			t.Errorf("folder payload = %#v, want folder-1/Dairy", folder)
		}
		if folder != nil && len(folder.GetItems()) != 2 {
			t.Errorf("folder item count = %d, want 2", len(folder.GetItems()))
		}
		if got := op.GetUpdatedParentFolderId(); got != "parent-2" {
			t.Errorf("updated parent = %q, want parent-2", got)
		}
		// The current typed client exposes no original-parent parameter.
		if got := op.GetOriginalParentFolderId(); got != "parent-1" {
			t.Errorf("original parent = %q, want parent-1", got)
		}
	})

	folder := &pb.PBListFolder{Identifier: "folder-1", Name: "Dairy", Items: []*pb.PBListFolderItem{{Identifier: "item-1", ItemType: 1}, {Identifier: "item-2", ItemType: 1}}}
	if err := client.SaveListFolderWithParents(t.Context(), "list-1", folder, "parent-1", "parent-2"); err != nil {
		t.Fatalf("SaveListFolder returned error: %v", err)
	}
}

func TestCreateListFolderSendsVerifiedCreateHandler(t *testing.T) {
	t.Parallel()

	client := newListFolderTestClient(t, func(op *pb.PBListFolderOperation) {
		if got := op.GetMetadata().GetHandlerId(); got != "create-new-folder" {
			t.Errorf("handler = %q, want create-new-folder", got)
		}
		if got := op.GetListDataId(); got != "list-1" {
			t.Errorf("listDataId = %q, want list-1", got)
		}
		if got := op.GetListFolder().GetIdentifier(); got != "folder-new" {
			t.Errorf("folder ID = %q, want folder-new", got)
		}
		if got := op.GetUpdatedParentFolderId(); got != "parent-1" {
			t.Errorf("updated parent = %q, want parent-1", got)
		}
		if got := op.GetOriginalParentFolderId(); got != "" {
			t.Errorf("original parent = %q, want empty", got)
		}
	})

	if err := client.CreateListFolder(t.Context(), "list-1", &pb.PBListFolder{Identifier: "folder-new", Name: "Produce"}, "parent-1"); err != nil {
		t.Fatalf("CreateListFolder returned error: %v", err)
	}
}

func TestRenameListFolderSendsVerifiedNameHandler(t *testing.T) {
	t.Parallel()

	client := newListFolderTestClient(t, func(op *pb.PBListFolderOperation) {
		if got := op.GetMetadata().GetHandlerId(); got != "set-folder-name" {
			t.Errorf("handler = %q, want set-folder-name", got)
		}
		if got := op.GetListDataId(); got != "list-1" {
			t.Errorf("listDataId = %q, want list-1", got)
		}
		folder := op.GetListFolder()
		if folder == nil || folder.GetIdentifier() != "folder-1" || folder.GetName() != "Renamed" {
			t.Errorf("folder payload = %#v, want folder-1/Renamed", folder)
		}
		if op.GetOriginalParentFolderId() != "" || op.GetUpdatedParentFolderId() != "" {
			t.Error("rename operation carried parent IDs")
		}
	})

	if err := client.RenameListFolder(t.Context(), "list-1", &pb.PBListFolder{Identifier: "folder-1", Name: "Original"}, "Renamed"); err != nil {
		t.Fatalf("RenameListFolder returned error: %v", err)
	}
}

func TestMoveListFolderItemsSendsVerifiedMoveHandler(t *testing.T) {
	t.Parallel()

	client := newListFolderTestClient(t, func(op *pb.PBListFolderOperation) {
		if got := op.GetMetadata().GetHandlerId(); got != "move-folder-items" {
			t.Errorf("handler = %q, want move-folder-items", got)
		}
		if got := op.GetListDataId(); got != "list-1" {
			t.Errorf("listDataId = %q, want list-1", got)
		}
		items := op.GetFolderItems()
		if len(items) != 1 || items[0].GetIdentifier() != "folder-1" || items[0].GetItemType() != int32(pb.PBListFolderItem_FolderType) {
			t.Errorf("folder items = %#v, want folder-1/folder type", items)
		}
		if op.GetOriginalParentFolderId() != "parent-1" || op.GetUpdatedParentFolderId() != "parent-2" {
			t.Errorf("parents = %q -> %q, want parent-1 -> parent-2", op.GetOriginalParentFolderId(), op.GetUpdatedParentFolderId())
		}
		if op.GetListFolder() != nil {
			t.Error("move operation carried a complete folder payload")
		}
	})

	if err := client.MoveListFolderItems(t.Context(), "list-1", "folder-1", "parent-1", "parent-2"); err != nil {
		t.Fatalf("MoveListFolderItems returned error: %v", err)
	}
}

func TestRemoveListFolderSendsTypedDeleteOperation(t *testing.T) {
	t.Parallel()

	client := newListFolderTestClient(t, func(op *pb.PBListFolderOperation) {
		if got := op.GetMetadata().GetHandlerId(); got != "delete-folder-items" {
			t.Errorf("handler = %q, want delete-folder-items", got)
		}
		if got := op.GetMetadata().GetUserId(); got != "user-1" {
			t.Errorf("user ID = %q, want user-1", got)
		}
		if op.GetMetadata().GetOperationId() == "" {
			t.Error("operation ID must not be empty")
		}
		if got := op.GetListDataId(); got != "list-1" {
			t.Errorf("listDataId = %q, want list-1", got)
		}
		items := op.GetFolderItems()
		if len(items) != 1 || items[0].GetIdentifier() != "folder-1" || items[0].GetItemType() != int32(pb.PBListFolderItem_FolderType) {
			t.Errorf("folder items = %#v, want folder-1/folder type", items)
		}
		if op.GetListFolder() != nil || op.GetOriginalParentFolderId() != "" || op.GetUpdatedParentFolderId() != "" {
			t.Errorf("delete operation carried a folder payload or parent IDs")
		}
	})

	if err := client.RemoveListFolder(t.Context(), "list-1", &pb.PBListFolder{Identifier: "folder-1", Name: "Old Folder"}); err != nil {
		t.Fatalf("RemoveListFolder returned error: %v", err)
	}
}

func TestRemoveListFolderFromParentIncludesOriginalParent(t *testing.T) {
	t.Parallel()
	client := newListFolderTestClient(t, func(op *pb.PBListFolderOperation) {
		if got := op.GetOriginalParentFolderId(); got != "root-1" {
			t.Errorf("original parent = %q, want root-1", got)
		}
	})
	if err := client.RemoveListFolderFromParent(t.Context(), "list-1", &pb.PBListFolder{Identifier: "folder-1"}, "root-1"); err != nil {
		t.Fatalf("RemoveListFolderFromParent returned error: %v", err)
	}
}

func TestDeleteListFolderItemsSendsTypedOperation(t *testing.T) {
	t.Parallel()

	client := newListFolderTestClient(t, func(op *pb.PBListFolderOperation) {
		if got := op.GetMetadata().GetHandlerId(); got != "delete-folder-items" {
			t.Fatalf("handler = %q, want delete-folder-items", got)
		}
		if got := op.GetListDataId(); got != "list-data-1" {
			t.Fatalf("list data ID = %q, want list-data-1", got)
		}
		items := op.GetFolderItems()
		if len(items) != 2 {
			t.Fatalf("folder item count = %d, want 2", len(items))
		}
		if items[0].GetIdentifier() != "list-1" || items[0].GetItemType() != int32(pb.PBListFolderItem_ListType) {
			t.Errorf("first folder item = %q/type %d, want list-1/type 0", items[0].GetIdentifier(), items[0].GetItemType())
		}
		if items[1].GetIdentifier() != "folder-1" || items[1].GetItemType() != int32(pb.PBListFolderItem_FolderType) {
			t.Errorf("second folder item = %q/type %d, want folder-1/type 1", items[1].GetIdentifier(), items[1].GetItemType())
		}
		if op.GetListFolder() != nil || op.GetOriginalParentFolderId() != "" || op.GetUpdatedParentFolderId() != "" {
			t.Error("delete-folder-items must not carry a folder payload or parent IDs")
		}
	})
	items := []*pb.PBListFolderItem{
		{Identifier: "list-1", ItemType: int32(pb.PBListFolderItem_ListType)},
		{Identifier: "folder-1", ItemType: int32(pb.PBListFolderItem_FolderType)},
	}
	if err := client.DeleteListFolderItems(t.Context(), "list-data-1", items...); err != nil {
		t.Fatalf("DeleteListFolderItems returned error: %v", err)
	}
	items[0].Identifier = "changed-after-send"
}

func TestDeleteListFolderItemsFromParentIncludesOriginalParent(t *testing.T) {
	t.Parallel()
	client := newListFolderTestClient(t, func(op *pb.PBListFolderOperation) {
		if got := op.GetOriginalParentFolderId(); got != "folder-1" {
			t.Errorf("original parent = %q, want folder-1", got)
		}
	})
	if err := client.DeleteListFolderItemsFromParent(t.Context(), "list-data-1", "folder-1", &pb.PBListFolderItem{Identifier: "list-1"}); err != nil {
		t.Fatalf("DeleteListFolderItemsFromParent returned error: %v", err)
	}
}

func TestDeleteListFolderItemsRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	client := New(&config.Config{AccessToken: "access-token", UserID: "user-1"})
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected request for invalid input: %s %s", req.Method, req.URL.Path)
		return nil, nil
	})}
	for _, tc := range []struct {
		name  string
		data  string
		items []*pb.PBListFolderItem
	}{
		{name: "empty data ID"},
		{name: "blank data ID", data: "  "},
		{name: "no items", data: "list-data-1"},
		{name: "nil item", data: "list-data-1", items: []*pb.PBListFolderItem{nil}},
		{name: "blank item ID", data: "list-data-1", items: []*pb.PBListFolderItem{{ItemType: 1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := client.DeleteListFolderItems(t.Context(), tc.data, tc.items...); err == nil {
				t.Fatal("DeleteListFolderItems returned nil for invalid input")
			}
		})
	}
}

func newRecipeCollectionTestClient(t *testing.T, inspect func(*pb.PBRecipeOperation)) *Client {
	t.Helper()

	cfg := &config.Config{
		Path:        filepath.Join(t.TempDir(), "config.toml"),
		AccessToken: "access-token",
		UserID:      "user-1",
	}
	client := New(cfg)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/data/user-recipe-data/update" {
			t.Fatalf("request path = %q, want /data/user-recipe-data/update", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse form body: %v", err)
		}
		var operations pb.PBRecipeOperationList
		if err := proto.Unmarshal([]byte(form.Get("operations")), &operations); err != nil {
			t.Fatalf("unmarshal operations: %v", err)
		}
		if len(operations.GetOperations()) != 1 {
			t.Fatalf("operation count = %d, want 1", len(operations.GetOperations()))
		}
		inspect(operations.GetOperations()[0])
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     http.StatusText(http.StatusOK),
			Body:       io.NopCloser(bytes.NewReader(nil)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}
	return client
}

func TestUpdateListSettingsSendsTypedMultipartOperation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Path:        filepath.Join(t.TempDir(), "config.toml"),
		AccessToken: "access-token",
		UserID:      "user-1",
	}
	client := New(cfg)

	var operationIDs []string
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/data/list-settings/update" {
			t.Fatalf("request path = %q, want /data/list-settings/update", req.URL.Path)
		}
		if got := req.Header.Get("Authorization"); got != "Bearer access-token" {
			t.Fatalf("Authorization = %q, want Bearer access-token", got)
		}

		_, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("parse Content-Type: %v", err)
		}
		boundary, ok := params["boundary"]
		if !ok {
			t.Fatalf("Content-Type %q has no boundary", req.Header.Get("Content-Type"))
		}
		reader := multipart.NewReader(req.Body, boundary)
		part, err := reader.NextPart()
		if err != nil {
			t.Fatalf("read multipart part: %v", err)
		}
		if part.FormName() != "operations" {
			t.Fatalf("multipart form field = %q, want operations", part.FormName())
		}
		if got := part.Header.Get("Content-Type"); got != "application/octet-stream" {
			t.Fatalf("operations part Content-Type = %q, want application/octet-stream", got)
		}
		raw, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("read operations part: %v", err)
		}

		var operations pb.PBListSettingsOperationList
		if err := proto.Unmarshal(raw, &operations); err != nil {
			t.Fatalf("unmarshal list settings operations: %v", err)
		}
		if len(operations.GetOperations()) != 1 {
			t.Fatalf("operation count = %d, want 1", len(operations.GetOperations()))
		}
		op := operations.GetOperations()[0]
		if got := op.GetMetadata().GetHandlerId(); got != "captured-handler-xyz" {
			t.Fatalf("handler = %q, want captured-handler-xyz", got)
		}
		if got := op.GetMetadata().GetUserId(); got != "user-1" {
			t.Fatalf("user ID = %q, want user-1", got)
		}
		operationID := op.GetMetadata().GetOperationId()
		if operationID == "" {
			t.Fatal("operation ID is empty; metadata must be fresh")
		}
		if _, err := uuid.Parse(operationID); err != nil {
			t.Fatalf("operation ID %q is not a valid UUID: %v", operationID, err)
		}
		operationIDs = append(operationIDs, operationID)

		settings := op.GetUpdatedSettings()
		if settings == nil {
			t.Fatal("updatedSettings payload is nil")
		}
		if got := settings.GetListId(); got != " list-1 " {
			t.Fatalf("submitted list ID = %q, want the stable caller list ID %q", got, " list-1 ")
		}
		if !proto.Equal(settings, listSettingsTestPayload()) {
			t.Error("submitted settings do not preserve the complete caller payload")
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     http.StatusText(http.StatusOK),
			Body:       io.NopCloser(bytes.NewReader(nil)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}

	payload := listSettingsTestPayload()
	if err := client.UpdateListSettings(t.Context(), "captured-handler-xyz", payload); err != nil {
		t.Fatalf("UpdateListSettings returned error: %v", err)
	}
	if !proto.Equal(payload, listSettingsTestPayload()) {
		t.Error("UpdateListSettings mutated the caller-supplied settings payload")
	}
	if err := client.UpdateListSettings(t.Context(), "captured-handler-xyz", listSettingsTestPayload()); err != nil {
		t.Fatalf("second UpdateListSettings returned error: %v", err)
	}
	if len(operationIDs) != 2 {
		t.Fatalf("captured operation ID count = %d, want 2", len(operationIDs))
	}
	if operationIDs[0] == operationIDs[1] {
		t.Fatalf("operation IDs are not fresh across calls: %q", operationIDs[0])
	}
}

func listSettingsTestPayload() *pb.PBListSettings {
	return &pb.PBListSettings{
		Identifier:                        "settings-1",
		ListId:                            " list-1 ",
		ShouldHideCategories:              true,
		SelectedCategoryOrdering:          "a,b,c",
		GenericGroceryAutocompleteEnabled: true,
		ListItemSortOrder:                 "custom",
		ShouldHideCompletedItems:          true,
		ListColorType:                     2,
		ListThemeId:                       "theme-9",
		BadgeMode:                         "none",
		ShouldHideStoreNames:              true,
		ShouldHideRunningTotals:           true,
		ShouldHidePrices:                  true,
		CategoryOrderings: []*pb.PBCategoryOrdering{
			{Identifier: "order-1", Name: "custom", Categories: []string{"cat-1", "cat-2"}},
		},
	}
}

func TestUpdateListSettingsValidatesBeforeTransport(t *testing.T) {
	t.Parallel()

	requestCount := 0
	client := New(&config.Config{UserID: "user-1"})
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		t.Fatalf("transport must not be reached for invalid input; path = %q", req.URL.Path)
		return nil, nil
	})}

	payload := listSettingsTestPayload()
	cases := map[string]error{
		"empty handler ID":   client.UpdateListSettings(t.Context(), "  ", payload),
		"nil payload":        client.UpdateListSettings(t.Context(), "captured-handler-xyz", nil),
		"empty list ID":      client.UpdateListSettings(t.Context(), "captured-handler-xyz", &pb.PBListSettings{ListId: " "}),
		"zero list ID field": client.UpdateListSettings(t.Context(), "captured-handler-xyz", &pb.PBListSettings{Identifier: "settings-2"}),
	}
	for name, err := range cases {
		if err == nil {
			t.Errorf("%s: UpdateListSettings returned nil error", name)
		}
	}
	if requestCount != 0 {
		t.Fatalf("transport requests = %d, want 0 for validation failures", requestCount)
	}
}

func newFolderMetadataTestClient(t *testing.T, inspect func(*pb.PBListFolderOperation)) *Client {
	return newFolderMetadataTestClientWithPart(t, "operations", inspect)
}

func newFolderMetadataTestClientWithPart(t *testing.T, partName string, inspect func(*pb.PBListFolderOperation)) *Client {
	t.Helper()
	client := New(&config.Config{
		Path:         filepath.Join(t.TempDir(), "config.toml"),
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		UserID:       "user-1",
	})
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.Path != "/data/list-folders/update" {
			t.Fatalf("request = %s %s, want POST /data/list-folders/update", req.Method, req.URL.Path)
		}
		if got := req.Header.Get("Authorization"); got != "Bearer access-token" {
			t.Fatalf("Authorization = %q, want Bearer access-token", got)
		}
		_, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("parse Content-Type: %v", err)
		}
		part, err := multipart.NewReader(req.Body, params["boundary"]).NextPart()
		if err != nil {
			t.Fatalf("read multipart part: %v", err)
		}
		if part.FormName() != partName || part.Header.Get("Content-Type") != "application/octet-stream" {
			t.Fatalf("unexpected operations part: name=%q type=%q", part.FormName(), part.Header.Get("Content-Type"))
		}
		raw, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("read operations part: %v", err)
		}
		var operations pb.PBListFolderOperationList
		if err := proto.Unmarshal(raw, &operations); err != nil {
			t.Fatalf("unmarshal operations: %v", err)
		}
		if len(operations.GetOperations()) != 1 {
			t.Fatalf("operation count = %d, want 1", len(operations.GetOperations()))
		}
		inspect(operations.GetOperations()[0])
		return &http.Response{StatusCode: http.StatusOK, Status: http.StatusText(http.StatusOK), Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header), Request: req}, nil
	})}
	return client
}

func TestSetListFolderHexColorSendsTypedMultipartOperation(t *testing.T) {
	t.Parallel()
	client := newFolderMetadataTestClient(t, func(op *pb.PBListFolderOperation) {
		if got := op.GetMetadata().GetHandlerId(); got != "set-folder-hex-color" {
			t.Errorf("handler = %q", got)
		}
		if got := op.GetListDataId(); got != "list-1" {
			t.Errorf("listDataId = %q", got)
		}
		folder := op.GetListFolder()
		if folder == nil || folder.GetIdentifier() != "folder-1" || folder.GetName() != "Produce" {
			t.Fatalf("folder payload = %v", folder)
		}
		settings := folder.GetFolderSettings()
		if settings == nil || settings.GetFolderHexColor() != "#123456" {
			t.Fatalf("folder settings = %v", settings)
		}
		if got := settings.GetFolderSortPosition(); got != int32(pb.PBListFolderSettings_FolderSortPositionWithLists) {
			t.Errorf("sort position = %d", got)
		}
	})
	folder := &pb.PBListFolder{Identifier: "folder-1", Name: "Produce", FolderSettings: &pb.PBListFolderSettings{FolderHexColor: "#ABCDEF", FolderSortPosition: int32(pb.PBListFolderSettings_FolderSortPositionWithLists)}}
	if err := client.SetListFolderHexColor(t.Context(), "list-1", folder, "#123456"); err != nil {
		t.Fatalf("SetListFolderHexColor: %v", err)
	}
	if got := folder.GetFolderSettings().GetFolderHexColor(); got != "#ABCDEF" {
		t.Errorf("caller color = %q", got)
	}
}

func TestSetListFolderSortPositionSendsTypedMultipartOperation(t *testing.T) {
	t.Parallel()
	client := newFolderMetadataTestClient(t, func(op *pb.PBListFolderOperation) {
		if got := op.GetMetadata().GetHandlerId(); got != "set-folder-sort-position" {
			t.Errorf("handler = %q", got)
		}
		if got := op.GetListDataId(); got != "list-1" {
			t.Errorf("listDataId = %q", got)
		}
		settings := op.GetListFolder().GetFolderSettings()
		if settings == nil || settings.GetFolderSortPosition() != int32(pb.PBListFolderSettings_FolderSortPositionBeforeLists) {
			t.Fatalf("folder settings = %v", settings)
		}
	})
	folder := &pb.PBListFolder{Identifier: "folder-1", Name: "Produce"}
	if err := client.SetListFolderSortPosition(t.Context(), "list-1", folder, pb.PBListFolderSettings_FolderSortPositionBeforeLists); err != nil {
		t.Fatalf("SetListFolderSortPosition: %v", err)
	}
	if folder.GetFolderSettings() != nil {
		t.Errorf("caller settings mutated: %v", folder.GetFolderSettings())
	}
}

func TestSetOrderedFolderItemsSendsTypedMultipartOperation(t *testing.T) {
	t.Parallel()
	client := newFolderMetadataTestClientWithPart(t, "operations", func(op *pb.PBListFolderOperation) {
		if got := op.GetMetadata().GetHandlerId(); got != "set-ordered-folder-items" {
			t.Errorf("handler = %q", got)
		}
		if got := op.GetListDataId(); got != "list-1" {
			t.Errorf("listDataId = %q", got)
		}
		items := op.GetFolderItems()
		if len(items) != 2 || items[0].GetIdentifier() != "list-b" || items[1].GetIdentifier() != "list-a" {
			t.Fatalf("folder items = %v, want list-b,list-a", items)
		}
		if op.GetListFolder() != nil || op.GetOriginalParentFolderId() != "folder-1" || op.GetUpdatedParentFolderId() != "" {
			t.Fatalf("unexpected ordering folder fields: folder=%v original=%q updated=%q", op.GetListFolder(), op.GetOriginalParentFolderId(), op.GetUpdatedParentFolderId())
		}
	})
	folder := &pb.PBListFolder{
		Identifier: "folder-1",
		Name:       "Produce",
		Items: []*pb.PBListFolderItem{
			{Identifier: "list-a", ItemType: int32(pb.PBListFolderItem_ListType)},
			{Identifier: "list-b", ItemType: int32(pb.PBListFolderItem_ListType)},
		},
	}
	ordered := []*pb.PBListFolderItem{
		{Identifier: "list-b", ItemType: int32(pb.PBListFolderItem_ListType)},
		{Identifier: "list-a", ItemType: int32(pb.PBListFolderItem_ListType)},
	}
	if err := client.SetOrderedFolderItems(t.Context(), "list-1", folder, ordered); err != nil {
		t.Fatalf("SetOrderedFolderItems: %v", err)
	}
	if folder.GetItems()[0].GetIdentifier() != "list-a" {
		t.Fatalf("caller folder was mutated: %v", folder.GetItems())
	}
}

func TestSetOrderedFolderItemsValidatesBeforeTransport(t *testing.T) {
	t.Parallel()
	requestCount := 0
	client := New(&config.Config{UserID: "user-1"})
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		t.Fatalf("unexpected transport request")
		return nil, nil
	})}
	folder := &pb.PBListFolder{Identifier: "folder-1"}
	for _, err := range []error{
		client.SetOrderedFolderItems(t.Context(), "", folder, []*pb.PBListFolderItem{{Identifier: "a"}}),
		client.SetOrderedFolderItems(t.Context(), "list-1", nil, []*pb.PBListFolderItem{{Identifier: "a"}}),
		client.SetOrderedFolderItems(t.Context(), "list-1", &pb.PBListFolder{}, []*pb.PBListFolderItem{{Identifier: "a"}}),
		client.SetOrderedFolderItems(t.Context(), "list-1", folder, nil),
		client.SetOrderedFolderItems(t.Context(), "list-1", folder, []*pb.PBListFolderItem{nil}),
		client.SetOrderedFolderItems(t.Context(), "list-1", folder, []*pb.PBListFolderItem{{Identifier: " "}}),
	} {
		if err == nil {
			t.Error("invalid ordering input returned nil error")
		}
	}
	if requestCount != 0 {
		t.Fatalf("transport requests = %d, want 0", requestCount)
	}
}

func TestSetListFolderMetadataValidatesBeforeTransport(t *testing.T) {
	t.Parallel()
	requestCount := 0
	client := New(&config.Config{UserID: "user-1"})
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestCount++
		t.Fatalf("unexpected transport request")
		return nil, nil
	})}
	folder := &pb.PBListFolder{Identifier: "folder-1"}
	for _, err := range []error{
		client.SetListFolderHexColor(t.Context(), "", folder, "#123456"),
		client.SetListFolderHexColor(t.Context(), "list-1", nil, "#123456"),
		client.SetListFolderHexColor(t.Context(), "list-1", folder, "#GGHHII"),
		client.SetListFolderSortPosition(t.Context(), "", folder, pb.PBListFolderSettings_FolderSortPositionBeforeLists),
		client.SetListFolderSortPosition(t.Context(), "list-1", nil, pb.PBListFolderSettings_FolderSortPositionBeforeLists),
		client.SetListFolderSortPosition(t.Context(), "list-1", folder, pb.PBListFolderSettings_FolderSortPosition(7)),
	} {
		if err == nil {
			t.Error("invalid metadata input returned nil error")
		}
	}
	if requestCount != 0 {
		t.Fatalf("transport requests = %d, want 0", requestCount)
	}
}
