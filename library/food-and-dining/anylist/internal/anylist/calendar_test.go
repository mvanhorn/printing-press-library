package anylist

import (
	"bytes"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/anylist/pb"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/config"
	"google.golang.org/protobuf/proto"
)

func TestSaveCalendarEventSendsTypedCreateOperation(t *testing.T) {
	t.Parallel()

	client := calendarTestClient(t, func(req *http.Request) (*http.Response, error) {
		op := decodeCalendarOperation(t, req)
		if got := op.GetMetadata().GetHandlerId(); got != "new-event" {
			t.Fatalf("handler = %q, want new-event", got)
		}
		if got := op.GetCalendarId(); got != "calendar-1" {
			t.Fatalf("operation calendar ID = %q, want calendar-1", got)
		}
		if op.GetOriginalEvent() != nil {
			t.Fatal("create operation must not include original event")
		}
		if got := op.GetUpdatedEvent().GetIdentifier(); got != "event-1" {
			t.Fatalf("updated event ID = %q, want event-1", got)
		}
		if got := op.GetUpdatedEvent().GetCalendarId(); got != "calendar-1" {
			t.Fatalf("updated event calendar ID = %q, want calendar-1", got)
		}
		return calendarOKResponse(req), nil
	})

	event := &pb.PBCalendarEvent{Identifier: "event-1", Date: "2026-08-16", Title: "Dinner", CalendarId: "stale-calendar"}
	if err := client.SaveCalendarEvent(t.Context(), " calendar-1 ", event); err != nil {
		t.Fatalf("SaveCalendarEvent returned error: %v", err)
	}
	if event.GetCalendarId() != "stale-calendar" {
		t.Fatalf("SaveCalendarEvent mutated caller event calendar ID to %q", event.GetCalendarId())
	}
}

func TestUpdateCalendarEventSendsUpdatedAndOriginalEvents(t *testing.T) {
	t.Parallel()

	client := calendarTestClient(t, func(req *http.Request) (*http.Response, error) {
		op := decodeCalendarOperation(t, req)
		if got := op.GetMetadata().GetHandlerId(); got != "update-event" {
			t.Fatalf("handler = %q, want update-event", got)
		}
		if got := op.GetUpdatedEvent().GetTitle(); got != "Updated title" {
			t.Fatalf("updated title = %q, want Updated title", got)
		}
		if got := op.GetOriginalEvent().GetTitle(); got != "Original title" {
			t.Fatalf("original title = %q, want Original title", got)
		}
		if got := op.GetOriginalEvent().GetCalendarId(); got != "calendar-1" {
			t.Fatalf("original calendar ID = %q, want calendar-1", got)
		}
		return calendarOKResponse(req), nil
	})

	updated := &pb.PBCalendarEvent{Identifier: "event-1", CalendarId: "calendar-1", Title: "Updated title"}
	original := &pb.PBCalendarEvent{Identifier: "event-1", CalendarId: "calendar-1", Title: "Original title"}
	if err := client.UpdateCalendarEvent(t.Context(), "calendar-1", updated, original); err != nil {
		t.Fatalf("UpdateCalendarEvent returned error: %v", err)
	}
}

func TestRemoveCalendarEventCarriesUpdatedEvent(t *testing.T) {
	t.Parallel()

	client := calendarTestClient(t, func(req *http.Request) (*http.Response, error) {
		op := decodeCalendarOperation(t, req)
		if got := op.GetMetadata().GetHandlerId(); got != "delete-event" {
			t.Fatalf("handler = %q, want delete-event", got)
		}
		if op.GetUpdatedEvent() == nil || op.GetUpdatedEvent().GetIdentifier() != "event-1" {
			t.Fatalf("updated event = %#v, want event-1", op.GetUpdatedEvent())
		}
		if op.GetOriginalEvent() != nil {
			t.Fatal("delete operation must not include original event")
		}
		return calendarOKResponse(req), nil
	})

	if err := client.RemoveCalendarEvent(t.Context(), "calendar-1", &pb.PBCalendarEvent{Identifier: "event-1"}); err != nil {
		t.Fatalf("RemoveCalendarEvent returned error: %v", err)
	}
}

func TestCalendarEventOperationsRejectIncompleteInputs(t *testing.T) {
	t.Parallel()

	client := New(&config.Config{UserID: "user-1"})
	event := &pb.PBCalendarEvent{Identifier: "event-1"}
	if err := client.SaveCalendarEvent(t.Context(), " ", event); err == nil {
		t.Fatal("SaveCalendarEvent returned nil for empty calendar ID")
	}
	if err := client.SaveCalendarEvent(t.Context(), "calendar-1", nil); err == nil {
		t.Fatal("SaveCalendarEvent returned nil for nil event")
	}
	if err := client.UpdateCalendarEvent(t.Context(), "calendar-1", &pb.PBCalendarEvent{}, nil); err == nil {
		t.Fatal("UpdateCalendarEvent returned nil for empty event ID")
	}
}

func calendarTestClient(t *testing.T, transport roundTripFunc) *Client {
	t.Helper()
	client := New(&config.Config{
		Path:        filepath.Join(t.TempDir(), "config.toml"),
		AccessToken: "access-token",
		UserID:      "user-1",
	})
	client.httpClient = &http.Client{Transport: transport}
	return client
}

func decodeCalendarOperation(t *testing.T, req *http.Request) *pb.PBCalendarOperation {
	t.Helper()
	if req.Method != http.MethodPost || req.URL.Path != "/data/meal-planning-calendar/update" {
		t.Fatalf("request = %s %s, want POST /data/meal-planning-calendar/update", req.Method, req.URL.Path)
	}
	if contentType := req.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "multipart/form-data; boundary=") {
		t.Fatalf("Content-Type = %q, want multipart form", contentType)
	}
	reader, err := req.MultipartReader()
	if err != nil {
		t.Fatalf("MultipartReader returned error: %v", err)
	}
	part, err := reader.NextPart()
	if err != nil {
		t.Fatalf("NextPart returned error: %v", err)
	}
	if part.FormName() != "operations" {
		t.Fatalf("calendar form field = %q, want operations", part.FormName())
	}
	if got := part.Header.Get("Content-Type"); got != "application/octet-stream" {
		t.Fatalf("calendar part content type = %q, want application/octet-stream", got)
	}
	body, err := io.ReadAll(part)
	if err != nil {
		t.Fatalf("read operations part: %v", err)
	}
	var operations pb.PBCalendarOperationList
	if err := proto.Unmarshal(body, &operations); err != nil {
		t.Fatalf("unmarshal calendar operation: %v", err)
	}
	if len(operations.GetOperations()) != 1 {
		t.Fatalf("operation count = %d, want 1", len(operations.GetOperations()))
	}
	return operations.GetOperations()[0]
}

func calendarOKResponse(req *http.Request) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     http.StatusText(http.StatusOK),
		Body:       io.NopCloser(bytes.NewReader(nil)),
		Header:     make(http.Header),
		Request:    req,
	}
}
