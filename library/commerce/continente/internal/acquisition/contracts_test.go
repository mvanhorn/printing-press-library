package acquisition

import (
	"context"
	"errors"
	"testing"
)

type fakeProber struct {
	statusByPath map[string]int
	err          error
}

func (p fakeProber) ProbeGet(_ context.Context, path string) (int, error) {
	if p.err != nil {
		return 0, p.err
	}
	return p.statusByPath[path], nil
}

func TestDiscover_PrefersStructuredWhenProbeSucceeds(t *testing.T) {
	t.Parallel()

	report := Discover(context.Background(), fakeProber{
		statusByPath: map[string]int{
			"/mobify/proxy/api/search/shopper-search/v1/organizations": 200,
			"/s/-/dw/data/v23_2": 200,
		},
	})

	search := report.ContractFor(CapabilitySearch)
	if search.StructuredProbe != ProbeAvailable {
		t.Fatalf("search probe = %s, want available", search.StructuredProbe)
	}
	if search.PreferredAdapter != AdapterStructuredSCAPI {
		t.Fatalf("search preferred adapter = %s, want %s", search.PreferredAdapter, AdapterStructuredSCAPI)
	}
	if search.ActiveAdapter != AdapterStorefrontHTML {
		t.Fatalf("search active adapter = %s, want storefront fallback", search.ActiveAdapter)
	}

	product := report.ContractFor(CapabilityProduct)
	if product.PreferredAdapter != AdapterStructuredOCAPI {
		t.Fatalf("product preferred adapter = %s, want %s", product.PreferredAdapter, AdapterStructuredOCAPI)
	}
}

func TestDiscover_FallsBackWhenStructuredIsBlockedOrMissing(t *testing.T) {
	t.Parallel()

	report := Discover(context.Background(), fakeProber{
		statusByPath: map[string]int{
			"/mobify/proxy/api/search/shopper-search/v1/organizations": 403,
			"/s/-/dw/data/v23_2": 404,
		},
	})

	search := report.ContractFor(CapabilitySearch)
	if search.StructuredProbe != ProbeInaccessible {
		t.Fatalf("search probe = %s, want inaccessible", search.StructuredProbe)
	}
	if search.ActiveAdapter != AdapterStorefrontHTML {
		t.Fatalf("search active adapter = %s, want storefront", search.ActiveAdapter)
	}

	product := report.ContractFor(CapabilityProduct)
	if product.StructuredProbe != ProbeUnavailable {
		t.Fatalf("product probe = %s, want unavailable", product.StructuredProbe)
	}
}

func TestDiscover_HandlesProbeFailure(t *testing.T) {
	t.Parallel()

	report := Discover(context.Background(), fakeProber{err: errors.New("dial tcp timeout")})
	search := report.ContractFor(CapabilitySearch)
	if search.StructuredProbe != ProbeUnknown {
		t.Fatalf("search probe = %s, want unknown", search.StructuredProbe)
	}
	if search.Detail == "" {
		t.Fatal("expected detail for probe failure")
	}
}
