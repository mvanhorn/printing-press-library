// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package opentable

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// withTempCacheDir redirects os.UserCacheDir() to a temp directory by
// overriding XDG_CACHE_HOME (Linux) and HOME (macOS) — Go's os.UserCacheDir
// honors these. Returns a cleanup func.
func withTempCacheDir(t *testing.T) func() {
	t.Helper()
	dir := t.TempDir()
	origXDG := os.Getenv("XDG_CACHE_HOME")
	origHome := os.Getenv("HOME")
	t.Setenv("XDG_CACHE_HOME", dir)
	t.Setenv("HOME", dir)
	return func() {
		_ = os.Setenv("XDG_CACHE_HOME", origXDG)
		_ = os.Setenv("HOME", origHome)
	}
}

func sampleKey() availCacheKey {
	return availCacheKey{
		RestID:          25606,
		Date:            "2026-05-09",
		Time:            "19:00",
		PartySize:       4,
		ForwardMinutes:  210,
		BackwardMinutes: 210,
	}
}

func sampleResponse() []RestaurantAvailability {
	return []RestaurantAvailability{
		{
			RestaurantID: 25606,
			AvailabilityDays: []AvailabilityDay{
				{
					DayOffset: 0,
					Slots: []AvailabilitySlot{
						{IsAvailable: true, TimeOffsetMinutes: 75},
						{IsAvailable: true, TimeOffsetMinutes: 90},
					},
				},
			},
		},
	}
}

const testHash = "cbcf4838a9b399f742e3741785df64560a826d8d3cc2828aa01ab09a8455e29e"

func TestAvailCache_RoundTripFresh(t *testing.T) {
	defer withTempCacheDir(t)()
	k := sampleKey()
	resp := sampleResponse()
	saveAvailCache(k, testHash, resp)
	got := loadAvailCache(k, testHash)
	if got == nil {
		t.Fatal("expected cache hit, got nil")
	}
	if !got.Fresh {
		t.Errorf("expected Fresh=true within TTL, got false")
	}
	if got.Entry == nil || len(got.Entry.Response) != 1 {
		t.Fatalf("expected 1 RestaurantAvailability, got %#v", got.Entry)
	}
	if got.Entry.Response[0].RestaurantID != 25606 {
		t.Errorf("got wrong restID: %d", got.Entry.Response[0].RestaurantID)
	}
}

func TestAvailCache_PastTTLButWithin24h_StaleButReadable(t *testing.T) {
	defer withTempCacheDir(t)()
	k := sampleKey()
	saveAvailCache(k, testHash, sampleResponse())
	// Manually age the entry past TTL
	path, err := availCachePath(k)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var e availCacheEntry
	if err := json.Unmarshal(data, &e); err != nil {
		t.Fatal(err)
	}
	e.FetchedAt = time.Now().Add(-30 * time.Minute) // way past 3m TTL, well under 24h
	data, _ = json.Marshal(e)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	got := loadAvailCache(k, testHash)
	if got == nil {
		t.Fatal("expected cache entry returned even when stale, got nil")
	}
	if got.Fresh {
		t.Errorf("expected Fresh=false past TTL, got true")
	}
}

func TestAvailCache_Past24h_TreatedAsMissing(t *testing.T) {
	defer withTempCacheDir(t)()
	k := sampleKey()
	saveAvailCache(k, testHash, sampleResponse())
	path, _ := availCachePath(k)
	data, _ := os.ReadFile(path)
	var e availCacheEntry
	_ = json.Unmarshal(data, &e)
	e.FetchedAt = time.Now().Add(-25 * time.Hour) // past 24h hard cap
	data, _ = json.Marshal(e)
	_ = os.WriteFile(path, data, 0o600)
	got := loadAvailCache(k, testHash)
	if got != nil {
		t.Errorf("expected nil for entry past 24h cap, got %#v", got)
	}
}

func TestAvailCache_HashMismatch_TreatedAsMissing(t *testing.T) {
	defer withTempCacheDir(t)()
	k := sampleKey()
	saveAvailCache(k, testHash, sampleResponse())
	got := loadAvailCache(k, "deadbeef")
	if got != nil {
		t.Errorf("expected nil on hash drift, got %#v", got)
	}
}

func TestAvailCache_SchemaVersionMismatch_TreatedAsMissing(t *testing.T) {
	defer withTempCacheDir(t)()
	k := sampleKey()
	saveAvailCache(k, testHash, sampleResponse())
	path, _ := availCachePath(k)
	data, _ := os.ReadFile(path)
	var e availCacheEntry
	_ = json.Unmarshal(data, &e)
	e.SchemaVersion = 99 // future-version drift
	data, _ = json.Marshal(e)
	_ = os.WriteFile(path, data, 0o600)
	got := loadAvailCache(k, testHash)
	if got != nil {
		t.Errorf("expected nil on schema drift, got %#v", got)
	}
}

func TestAvailCache_CorruptJSON_TreatedAsMissing(t *testing.T) {
	defer withTempCacheDir(t)()
	k := sampleKey()
	path, _ := availCachePath(k)
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	_ = os.WriteFile(path, []byte("not json"), 0o600)
	got := loadAvailCache(k, testHash)
	if got != nil {
		t.Errorf("expected nil on corrupt file, got %#v", got)
	}
}

func TestAvailCache_MalformedDate_Rejected(t *testing.T) {
	defer withTempCacheDir(t)()
	k := sampleKey()
	k.Date = "2026/05/09" // wrong format
	saveAvailCache(k, testHash, sampleResponse())
	got := loadAvailCache(k, testHash)
	if got != nil {
		t.Errorf("expected nil for malformed date, got %#v", got)
	}
}

func TestAvailCache_PathTraversalAttempt_Rejected(t *testing.T) {
	defer withTempCacheDir(t)()
	k := sampleKey()
	k.Time = "../../../etc/passwd"                // path traversal attempt
	saveAvailCache(k, testHash, sampleResponse()) // should be no-op due to validation
	// Confirm no file leaked outside cache dir
	got := loadAvailCache(k, testHash)
	if got != nil {
		t.Errorf("expected nil for path-traversal time, got %#v", got)
	}
}

func TestAvailCache_DifferentWindowsAreSeparateKeys(t *testing.T) {
	defer withTempCacheDir(t)()
	k1 := sampleKey()
	k1.ForwardMinutes = 210
	k2 := sampleKey()
	k2.ForwardMinutes = 150
	saveAvailCache(k1, testHash, sampleResponse())
	// k2 should miss because window differs
	got := loadAvailCache(k2, testHash)
	if got != nil {
		t.Errorf("expected nil for different forwardMinutes window, got hit")
	}
}

func TestReadAvailCacheTTL_Default(t *testing.T) {
	t.Setenv("TRG_OT_CACHE_TTL", "")
	if d := readAvailCacheTTL(); d != availCacheTTLDefault {
		t.Errorf("expected default %s, got %s", availCacheTTLDefault, d)
	}
}

func TestReadAvailCacheTTL_ValidOverride(t *testing.T) {
	t.Setenv("TRG_OT_CACHE_TTL", "10m")
	if d := readAvailCacheTTL(); d != 10*time.Minute {
		t.Errorf("expected 10m, got %s", d)
	}
}

func TestReadAvailCacheTTL_OutOfRangeFallsBack(t *testing.T) {
	cases := []string{"0s", "30s", "48h", "-1m", "abc"}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			t.Setenv("TRG_OT_CACHE_TTL", c)
			if d := readAvailCacheTTL(); d != availCacheTTLDefault {
				t.Errorf("expected default %s for %q, got %s", availCacheTTLDefault, c, d)
			}
		})
	}
}

func TestAvailCache_HashedFilenameIsHex(t *testing.T) {
	k := sampleKey()
	path, err := availCachePath(k)
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Base(path)
	// Filename is "<16 hex chars>.json"
	if len(base) != 21 {
		t.Errorf("expected filename of length 21, got %d (%q)", len(base), base)
	}
	for i := 0; i < 16; i++ {
		c := base[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("filename has non-hex char at %d: %q", i, base)
			break
		}
	}
}
