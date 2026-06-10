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

// fakeOpOnPath installs a stub `op` executable on PATH that returns a
// username/password for the `op item get ... --fields label=<field>` calls the
// from-1password command makes, so the command can run without the real 1Password
// CLI. The password value it returns is a distinctive sentinel so tests can
// assert it never leaks into output or config.
func fakeOpOnPath(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\n" +
		"for a in \"$@\"; do\n" +
		"  case \"$a\" in\n" +
		"    label=username) echo alice@example.com; exit 0;;\n" +
		"    label=password) echo " + fakeOpPasswordSentinel + "; exit 0;;\n" +
		"  esac\n" +
		"done\n" +
		"echo unknown-field 1>&2\n" +
		"exit 1\n"
	opPath := filepath.Join(dir, "op")
	if err := os.WriteFile(opPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake op: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// fakeOpPasswordSentinel is the password value the stub `op` returns. It is
// chosen to not be a substring of any benign output token (e.g. "non-secret")
// so leak assertions are precise.
const fakeOpPasswordSentinel = "PWVAL-do-not-leak-7Z"

// TestAuthFrom1PasswordSyncProfileWithoutSaveDoesNotPersist verifies that
// `auth from-1password --sync-profile` without `--save` reports the profile as
// fetched-but-not-synced and writes nothing to config. Agents must not read
// profile_synced=true as confirmation that a later `reserve --use-saved-profile`
// can rely on a saved profile.
func TestAuthFrom1PasswordSyncProfileWithoutSaveDoesNotPersist(t *testing.T) {
	fakeOpOnPath(t)
	srv := profileServer(t)
	t.Setenv("MASTERPARK_BASE_URL", srv.URL)
	t.Setenv("PRINTING_PRESS_VERIFY", "")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	g := &globalOpts{timeout: 5 * time.Second, configPath: cfgPath, json: true}
	out, err := runCmd(t, newAuthCmd(g), "from-1password", "--sync-profile", "--lot", "B")
	if err != nil {
		t.Fatalf("from-1password --sync-profile: %v", err)
	}

	var res map[string]interface{}
	if uerr := json.Unmarshal([]byte(out), &res); uerr != nil {
		t.Fatalf("parse json output %q: %v", out, uerr)
	}
	if res["profile_synced"] != false {
		t.Errorf("profile_synced must be false without --save, got %v", res["profile_synced"])
	}
	if res["profile_fetched"] != true {
		t.Errorf("profile_fetched should be true after --sync-profile, got %v", res["profile_fetched"])
	}
	if res["saved_metadata"] != false {
		t.Errorf("saved_metadata must be false without --save, got %v", res["saved_metadata"])
	}

	// Nothing must be persisted to disk.
	if _, statErr := os.Stat(cfgPath); statErr == nil {
		t.Errorf("config file must not be written without --save")
	}
	f, lerr := config.Load(cfgPath)
	if lerr != nil {
		t.Fatalf("load config: %v", lerr)
	}
	if f.Profile != nil {
		t.Errorf("profile must not be saved without --save, got %+v", f.Profile)
	}
}

// TestAuthFrom1PasswordSyncProfileWithoutSaveTextOutput verifies the plain-text
// output does not claim the profile was saved when --save is omitted, and that
// the password never leaks.
func TestAuthFrom1PasswordSyncProfileWithoutSaveTextOutput(t *testing.T) {
	fakeOpOnPath(t)
	srv := profileServer(t)
	t.Setenv("MASTERPARK_BASE_URL", srv.URL)
	t.Setenv("PRINTING_PRESS_VERIFY", "")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	g := &globalOpts{timeout: 5 * time.Second, configPath: cfgPath}
	out, err := runCmd(t, newAuthCmd(g), "from-1password", "--sync-profile", "--lot", "B")
	if err != nil {
		t.Fatalf("from-1password --sync-profile: %v", err)
	}
	if strings.Contains(out, "Saved non-secret profile") {
		t.Errorf("must not claim profile saved without --save: %s", out)
	}
	if !strings.Contains(out, "did not save it") || !strings.Contains(out, "--save") {
		t.Errorf("expected fetched-but-not-saved notice mentioning --save, got: %s", out)
	}
	if strings.Contains(out, fakeOpPasswordSentinel) {
		t.Errorf("output must not leak the password: %s", out)
	}
}

// TestAuthFrom1PasswordSaveSyncProfilePersists verifies that with both --save and
// --sync-profile the profile is persisted and reported as profile_synced=true.
func TestAuthFrom1PasswordSaveSyncProfilePersists(t *testing.T) {
	fakeOpOnPath(t)
	srv := profileServer(t)
	t.Setenv("MASTERPARK_BASE_URL", srv.URL)
	t.Setenv("PRINTING_PRESS_VERIFY", "")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	g := &globalOpts{timeout: 5 * time.Second, configPath: cfgPath, json: true}
	out, err := runCmd(t, newAuthCmd(g), "from-1password", "--save", "--sync-profile", "--lot", "B")
	if err != nil {
		t.Fatalf("from-1password --save --sync-profile: %v", err)
	}

	var res map[string]interface{}
	if uerr := json.Unmarshal([]byte(out), &res); uerr != nil {
		t.Fatalf("parse json output %q: %v", out, uerr)
	}
	if res["profile_synced"] != true {
		t.Errorf("profile_synced must be true with --save --sync-profile, got %v", res["profile_synced"])
	}
	if res["profile_fetched"] != true {
		t.Errorf("profile_fetched should be true, got %v", res["profile_fetched"])
	}

	f, lerr := config.Load(cfgPath)
	if lerr != nil {
		t.Fatalf("load config: %v", lerr)
	}
	if f.Profile == nil || len(f.Profile.Vehicles) != 1 {
		t.Fatalf("profile must be saved with --save --sync-profile, got %+v", f.Profile)
	}
	raw, _ := os.ReadFile(cfgPath)
	if strings.Contains(string(raw), fakeOpPasswordSentinel) {
		t.Errorf("config file must never contain the password value: %s", raw)
	}
}

// TestAuthFrom1PasswordSyncProfileVerifyNoop verifies that under
// PRINTING_PRESS_VERIFY=1, `auth from-1password --sync-profile` returns a
// verify no-op, never contacts the live verifyLogin endpoint, and writes
// nothing to config. This mirrors the standalone `auth sync-profile` guard.
func TestAuthFrom1PasswordSyncProfileVerifyNoop(t *testing.T) {
	fakeOpOnPath(t)
	contacted := false
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		contacted = true
		t.Errorf("MasterPark must not be contacted under PRINTING_PRESS_VERIFY: %s", r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Setenv("MASTERPARK_BASE_URL", srv.URL)
	t.Setenv("PRINTING_PRESS_VERIFY", "1")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	g := &globalOpts{timeout: 5 * time.Second, configPath: cfgPath, json: true}
	out, err := runCmd(t, newAuthCmd(g), "from-1password", "--save", "--sync-profile", "--lot", "B")
	if err != nil {
		t.Fatalf("from-1password --sync-profile under verify: %v", err)
	}
	if contacted {
		t.Fatalf("verifyLogin endpoint was contacted under verify env")
	}

	var res map[string]interface{}
	if uerr := json.Unmarshal([]byte(out), &res); uerr != nil {
		t.Fatalf("parse json output %q: %v", out, uerr)
	}
	if res["verify_noop"] != true {
		t.Errorf("expected verify_noop=true, got %v", res["verify_noop"])
	}
	if strings.Contains(out, fakeOpPasswordSentinel) {
		t.Errorf("output must not leak the password: %s", out)
	}

	// Nothing must be persisted even with --save in the verify-noop branch.
	if _, statErr := os.Stat(cfgPath); statErr == nil {
		t.Errorf("config file must not be written in verify-noop branch")
	}
}

// TestAuthFrom1PasswordLoginCheckVerifyNoop verifies the `--login-check` path is
// equally guarded under PRINTING_PRESS_VERIFY=1.
func TestAuthFrom1PasswordLoginCheckVerifyNoop(t *testing.T) {
	fakeOpOnPath(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("MasterPark must not be contacted under PRINTING_PRESS_VERIFY: %s", r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Setenv("MASTERPARK_BASE_URL", srv.URL)
	t.Setenv("PRINTING_PRESS_VERIFY", "1")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	g := &globalOpts{timeout: 5 * time.Second, configPath: cfgPath, json: true}
	out, err := runCmd(t, newAuthCmd(g), "from-1password", "--login-check", "--lot", "B")
	if err != nil {
		t.Fatalf("from-1password --login-check under verify: %v", err)
	}
	var res map[string]interface{}
	if uerr := json.Unmarshal([]byte(out), &res); uerr != nil {
		t.Fatalf("parse json output %q: %v", out, uerr)
	}
	if res["verify_noop"] != true {
		t.Errorf("expected verify_noop=true, got %v", res["verify_noop"])
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
