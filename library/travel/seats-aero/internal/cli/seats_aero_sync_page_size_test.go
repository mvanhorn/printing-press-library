package cli

import "testing"

func TestSeatsAeroMeteredPaginationUsesMaximumPageSize(t *testing.T) {
	for _, resource := range []string{"availability", "awards"} {
		if got := determinePaginationDefaults(resource).limit; got != 1000 {
			t.Fatalf("%s page size=%d, want 1000", resource, got)
		}
	}
}
