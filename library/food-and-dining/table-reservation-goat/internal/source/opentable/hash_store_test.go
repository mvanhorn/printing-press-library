package opentable

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCurrentAvailabilityHash_AbsentReturnsConstDefault(t *testing.T) {
	t.Setenv("TABLE_RESERVATION_GOAT_CONFIG_DIR", t.TempDir())
	if got := currentAvailabilityHash(); got != RestaurantsAvailabilityHash {
		t.Fatalf("absent persisted hash: got %q, want const default %q", got, RestaurantsAvailabilityHash)
	}
}

func TestAvailabilityHash_RoundTrip(t *testing.T) {
	t.Setenv("TABLE_RESERVATION_GOAT_CONFIG_DIR", t.TempDir())
	fresh := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if err := savePersistedAvailabilityHash(fresh); err != nil {
		t.Fatalf("save: %v", err)
	}
	if got := loadPersistedAvailabilityHash(); got != fresh {
		t.Fatalf("round-trip load: got %q, want %q", got, fresh)
	}
	if got := currentAvailabilityHash(); got != fresh {
		t.Fatalf("current should prefer persisted: got %q, want %q", got, fresh)
	}
}

func TestAvailabilityIdentity_RoundTripOperationName(t *testing.T) {
	t.Setenv("TABLE_RESERVATION_GOAT_CONFIG_DIR", t.TempDir())
	fresh := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	identity := availabilityQueryIdentity{Hash: fresh, OperationName: "RestaurantAvailability"}
	if err := savePersistedAvailabilityIdentity(identity); err != nil {
		t.Fatalf("save identity: %v", err)
	}
	got := currentAvailabilityIdentity()
	if got != identity {
		t.Fatalf("current identity = %#v, want %#v", got, identity)
	}
}

func TestAvailabilityIdentity_LegacyHashOnlyDefaultsOperationName(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TABLE_RESERVATION_GOAT_CONFIG_DIR", dir)
	path, err := availHashPath()
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	fresh := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	data, _ := json.Marshal(map[string]string{"hash": fresh})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write legacy hash-only file: %v", err)
	}
	got := currentAvailabilityIdentity()
	want := availabilityQueryIdentity{Hash: fresh, OperationName: restaurantsAvailabilityOperationName}
	if got != want {
		t.Fatalf("legacy identity = %#v, want %#v", got, want)
	}
}

func TestCurrentAvailabilityHash_CorruptFallsBackToConst(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TABLE_RESERVATION_GOAT_CONFIG_DIR", dir)
	path, err := availHashPath()
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not valid json"), 0o600); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}
	if got := currentAvailabilityHash(); got != RestaurantsAvailabilityHash {
		t.Fatalf("corrupt file: got %q, want const default %q", got, RestaurantsAvailabilityHash)
	}
}

func TestSaveAvailabilityHash_RejectsInvalidShape(t *testing.T) {
	t.Setenv("TABLE_RESERVATION_GOAT_CONFIG_DIR", t.TempDir())
	if err := savePersistedAvailabilityHash("not-a-hash"); err == nil {
		t.Fatal("expected error saving a non-64-hex hash")
	}
	if got := currentAvailabilityHash(); got != RestaurantsAvailabilityHash {
		t.Fatalf("after rejected save: got %q, want const default %q", got, RestaurantsAvailabilityHash)
	}
}

func TestSaveAvailabilityIdentity_RejectsInvalidOperationName(t *testing.T) {
	t.Setenv("TABLE_RESERVATION_GOAT_CONFIG_DIR", t.TempDir())
	fresh := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	err := savePersistedAvailabilityIdentity(availabilityQueryIdentity{Hash: fresh, OperationName: "bad-name"})
	if err == nil {
		t.Fatal("expected error saving an invalid GraphQL operation name")
	}
	if got := currentAvailabilityIdentity(); got.OperationName != restaurantsAvailabilityOperationName || got.Hash != RestaurantsAvailabilityHash {
		t.Fatalf("after rejected save: got %#v, want const/default identity", got)
	}
}
