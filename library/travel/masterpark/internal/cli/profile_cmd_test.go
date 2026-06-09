package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/masterpark/internal/config"
)

// profileServer serves the nonce page and a verifyLogin response carrying a
// customer profile + vehicles, so sync-profile has something to persist.
func profileServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/reservation/book/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, `<script>window._wpnonce = "nonce";</script>`)
	})
	mux.HandleFunc("/wp-content/plugins/netParkV2/ajax.php", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]interface{}
		_ = json.Unmarshal(body, &parsed)
		w.Header().Set("Content-Type", "application/json")
		if parsed["method"] == "verifyLogin" {
			io.WriteString(w, `{"errors":[],"data":{"customer":{"first_name":"Alice","last_name":"Smith","email":"alice@example.com","phone":"phone-test","id":"C123"},"vehicles":[{"make":"Honda","model":"Civic","color":"Blue","license":"ABC123","state":"WA","type":"standard"}]}}`)
			return
		}
		io.WriteString(w, `{"errors":[],"data":{}}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestAuthSyncProfileSavesNonSecretProfile(t *testing.T) {
	srv := profileServer(t)
	t.Setenv("MASTERPARK_BASE_URL", srv.URL)
	t.Setenv("PRINTING_PRESS_VERIFY", "")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	g := &globalOpts{timeout: 5 * time.Second, configPath: cfgPath, json: true}
	out, err := runCmd(t, newAuthCmd(g), "sync-profile", "--lot", "B",
		"--username", "alice@example.com", "--password", "secret")
	if err != nil {
		t.Fatalf("sync-profile error: %v", err)
	}
	if strings.Contains(strings.ToLower(out), "secret") {
		t.Errorf("sync-profile output must not leak the password: %s", out)
	}

	f, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if f.Username != "alice@example.com" {
		t.Errorf("username not saved: %q", f.Username)
	}
	if f.Profile == nil || len(f.Profile.Vehicles) != 1 {
		t.Fatalf("profile not saved: %+v", f.Profile)
	}
	if f.Profile.Vehicles[0].Make != "Honda" || f.Profile.FirstName != "Alice" {
		t.Errorf("profile mismatch: %+v", f.Profile)
	}

	raw, _ := os.ReadFile(cfgPath)
	if strings.Contains(strings.ToLower(string(raw)), "password") {
		t.Errorf("config file must never contain a password: %s", raw)
	}
}

func TestReserveUsesSavedProfileDefaults(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "1")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	if err := config.Save(cfgPath, &config.File{
		Username: "alice@example.com",
		Profile: &config.Profile{
			FirstName: "Alice", LastName: "Smith",
			Email: "alice@example.com", Phone: "phone-test",
			Vehicles: []config.VehicleProfile{
				{Make: "Honda", Model: "Civic", Color: "Blue", License: "ABC123", State: "WA", Type: "standard"},
			},
		},
	}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	g := &globalOpts{timeout: 5 * time.Second, configPath: cfgPath, json: true}
	// Provide only scheduling flags; customer/vehicle should come from profile.
	out, err := runCmd(t, newReserveCmd(g),
		"--lot", "B",
		"--dropoff", "2026-06-11 07:00",
		"--pickup", "2026-06-13 18:30",
		"--quote", "1",
		"--submit", "--yes",
	)
	if err != nil {
		t.Fatalf("reserve with saved profile errored: %v", err)
	}
	if !strings.Contains(out, "verify_noop") {
		t.Errorf("expected verify no-op, got: %s", out)
	}
	// The filled customer/vehicle values should surface in the summary.
	if !strings.Contains(out, "Alice") || !strings.Contains(out, "ABC123") {
		t.Errorf("expected profile defaults in output, got: %s", out)
	}
}

func TestReserveMissingFieldsWithoutProfile(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	g := &globalOpts{timeout: 5 * time.Second, configPath: cfgPath}
	_, err := runCmd(t, newReserveCmd(g),
		"--lot", "B",
		"--dropoff", "2026-06-11 07:00",
		"--pickup", "2026-06-13 18:30",
		"--submit", "--yes",
	)
	if err == nil || !strings.Contains(err.Error(), "missing required fields") {
		t.Errorf("expected missing fields error without a saved profile, got: %v", err)
	}
}

func TestReserveNoUseSavedProfile(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	if err := config.Save(cfgPath, &config.File{
		Profile: &config.Profile{
			FirstName: "Alice", LastName: "Smith",
			Email: "alice@example.com", Phone: "phone-test",
			Vehicles: []config.VehicleProfile{{Make: "Honda", Model: "Civic", License: "ABC123"}},
		},
	}); err != nil {
		t.Fatalf("save config: %v", err)
	}

	g := &globalOpts{timeout: 5 * time.Second, configPath: cfgPath}
	_, err := runCmd(t, newReserveCmd(g),
		"--lot", "B",
		"--dropoff", "2026-06-11 07:00",
		"--pickup", "2026-06-13 18:30",
		"--use-saved-profile=false",
		"--submit", "--yes",
	)
	if err == nil || !strings.Contains(err.Error(), "missing required fields") {
		t.Errorf("expected missing fields when saved profile disabled, got: %v", err)
	}
}
