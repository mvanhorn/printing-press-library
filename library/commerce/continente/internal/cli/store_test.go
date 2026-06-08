package cli

import (
	"errors"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/continente/internal/acquisition/storefront"
	"github.com/mvanhorn/printing-press-library/library/commerce/continente/internal/config"
)

func TestNeedsStoreResolution(t *testing.T) {
	t.Parallel()

	if !needsStoreResolution(&config.PreferredStore{ID: "col-1-store"}) {
		t.Fatal("expected incomplete store to need resolution")
	}
	if needsStoreResolution(&config.PreferredStore{
		ID:         "col-1-store",
		Name:       "Continente Teste",
		City:       "Lisboa",
		PostalCode: "1000-000",
		Latitude:   38.7,
		Longitude:  -9.1,
	}) {
		t.Fatal("expected complete store not to need resolution")
	}
}

func TestFindStoreByID(t *testing.T) {
	t.Parallel()

	record, ok := findStoreByID([]storefront.StoreRecord{
		{ID: "col-1-store", Name: "One"},
		{ID: "col-2-store", Name: "Two"},
	}, "col-2-store")
	if !ok {
		t.Fatal("expected store match")
	}
	if record.Name != "Two" {
		t.Fatalf("record = %+v, want Two", record)
	}
}

func TestResolveStoreLookupCoordinates(t *testing.T) {
	t.Parallel()

	t.Run("uses explicit coordinates", func(t *testing.T) {
		t.Parallel()

		lat, lng, err := resolveStoreLookupCoordinates(38.7, -9.1, nil)
		if err != nil {
			t.Fatalf("resolveStoreLookupCoordinates(...) error = %v", err)
		}
		if lat != 38.7 || lng != -9.1 {
			t.Fatalf("coordinates = %v,%v; want 38.7,-9.1", lat, lng)
		}
	})

	t.Run("falls back to preferred store", func(t *testing.T) {
		t.Parallel()

		lat, lng, err := resolveStoreLookupCoordinates(0, 0, &config.PreferredStore{
			ID:        "col-1-store",
			Latitude:  40.1,
			Longitude: -8.2,
		})
		if err != nil {
			t.Fatalf("resolveStoreLookupCoordinates(...) error = %v", err)
		}
		if lat != 40.1 || lng != -8.2 {
			t.Fatalf("coordinates = %v,%v; want 40.1,-8.2", lat, lng)
		}
	})

	t.Run("rejects partial coordinates", func(t *testing.T) {
		t.Parallel()

		_, _, err := resolveStoreLookupCoordinates(38.7, 0, nil)
		var cliErr *cliError
		if !errors.As(err, &cliErr) || cliErr.code != 2 {
			t.Fatalf("err = %v; want usage error", err)
		}
	})
}
