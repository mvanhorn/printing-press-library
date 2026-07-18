package cli

import (
	"context"
	"strings"
	"testing"
)

func TestResolveLocalRejectsEndpointFilters(t *testing.T) {
	_, _, err := resolveLocal(context.Background(), nil, nil, "classes", true, "/classes", map[string]string{"category": "strength"}, "test")
	if err == nil {
		t.Fatal("resolveLocal accepted endpoint filters")
	}
	if !strings.Contains(err.Error(), "local store cannot apply endpoint filters") {
		t.Fatalf("resolveLocal error = %q", err)
	}
}
