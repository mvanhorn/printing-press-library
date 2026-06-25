package cli

import (
	"reflect"
	"testing"
)

func TestTailResourcePathUsesGeneratedAPIPath(t *testing.T) {
	path, err := tailResourcePath("subscribers")
	if err != nil {
		t.Fatalf("tailResourcePath returned error: %v", err)
	}
	if path != "/api/v2/subscribers" {
		t.Fatalf("tailResourcePath = %q, want /api/v2/subscribers", path)
	}
}

func TestTailKnownResourcesOnlyIncludesPollableReadResources(t *testing.T) {
	got := tailKnownResources()
	want := []string{"subscribers"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tailKnownResources = %#v, want %#v", got, want)
	}
}

func TestTailResourcePathRejectsNonSyncResource(t *testing.T) {
	_, err := tailResourcePath("events")
	if err == nil {
		t.Fatal("tailResourcePath(events) succeeded; want an unknown tail resource error")
	}
	if got, want := err.Error(), `unknown tail resource "events"`; got != want {
		t.Fatalf("tailResourcePath(events) error = %q, want %q", got, want)
	}
}
