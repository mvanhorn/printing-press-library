package anylist

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/config"
	"google.golang.org/protobuf/proto"
)

func TestNotificationLocationMethodsSendExactHandlers(t *testing.T) {
	t.Parallel()

	client := New(&config.Config{AccessToken: "access-token", UserID: "user-1"})
	call := 0
	client.httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/data/shopping-lists/update" {
			t.Fatalf("path = %q, want /data/shopping-lists/update", req.URL.Path)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatal(err)
		}
		var operations pb.PBListOperationList
		if err := proto.Unmarshal([]byte(form.Get("operations")), &operations); err != nil {
			t.Fatal(err)
		}
		if len(operations.GetOperations()) != 1 {
			t.Fatalf("operation count = %d, want 1", len(operations.GetOperations()))
		}
		op := operations.GetOperations()[0]
		location := op.GetNotificationLocation()
		if location == nil || location.GetName() != "Home" || location.GetAddress() != "123 Test Avenue" {
			t.Fatalf("location = %v", location)
		}
		if call == 0 {
			if op.GetMetadata().GetHandlerId() != "add-list-notification-location" {
				t.Fatalf("add handler = %q", op.GetMetadata().GetHandlerId())
			}
			if location.GetIdentifier() == "" {
				t.Fatal("add did not generate a location ID")
			}
		} else if op.GetMetadata().GetHandlerId() != "remove-list-notification-location" {
			t.Fatalf("remove handler = %q", op.GetMetadata().GetHandlerId())
		}
		call++
		return &http.Response{StatusCode: http.StatusOK, Status: http.StatusText(http.StatusOK), Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header), Request: req}, nil
	})}

	location := &pb.PBNotificationLocation{Latitude: 33.6, Longitude: -95.5, Name: "Home", Address: "123 Test Avenue"}
	id, err := client.AddListNotificationLocation(t.Context(), " list-1 ", location)
	if err != nil {
		t.Fatalf("add returned error: %v", err)
	}
	if id == "" || location.GetIdentifier() != "" {
		t.Fatalf("caller location was mutated or ID missing: id=%q location=%v", id, location)
	}
	if err := client.RemoveListNotificationLocation(t.Context(), "list-1", &pb.PBNotificationLocation{Identifier: id, Name: "Home", Address: "123 Test Avenue"}); err != nil {
		t.Fatalf("remove returned error: %v", err)
	}
}

func TestNotificationLocationMethodsValidate(t *testing.T) {
	t.Parallel()
	client := New(&config.Config{UserID: "user-1"})
	if _, err := client.AddListNotificationLocation(t.Context(), "list-1", &pb.PBNotificationLocation{Name: "Home", Address: "123 Test Avenue", Latitude: 91}); err == nil {
		t.Fatal("out-of-range latitude was accepted")
	}
	if err := client.RemoveListNotificationLocation(t.Context(), "list-1", &pb.PBNotificationLocation{}); err == nil {
		t.Fatal("missing location ID was accepted")
	}
}
