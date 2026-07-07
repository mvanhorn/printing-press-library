package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/masterpark/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/masterpark/internal/config"
)

// recordingLoginServer serves the nonce page and a verifyLogin response that
// carries a customer profile + vehicles, recording every ajax body it sees.
type recordingLoginServer struct {
	srv    *httptest.Server
	bodies []map[string]interface{}
}

func newRecordingLoginServer(t *testing.T) *recordingLoginServer {
	t.Helper()
	rec := &recordingLoginServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("/reservation/book/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, `<script>window._wpnonce = "nonce";</script>`)
	})
	mux.HandleFunc("/wp-content/plugins/netParkV2/ajax.php", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]interface{}
		_ = json.Unmarshal(body, &parsed)
		rec.bodies = append(rec.bodies, parsed)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"errors":[],"data":{
			"customer":{"id":"C123","first_name":"Alice","last_name":"Smith","email":"alice@example.com","phone":"phone-test"},
			"vehicles":[{"make":"Honda","model":"Civic","color":"Blue","license":"ABC123","state":"WA","type":"standard"}]
		}}`)
	})
	rec.srv = httptest.NewServer(mux)
	t.Cleanup(rec.srv.Close)
	return rec
}

func TestParseLoginProfileNested(t *testing.T) {
	data := json.RawMessage(`{
		"customer":{"id":"C123","first_name":"Alice","last_name":"Smith","email":"alice@example.com","phone":"phone-test"},
		"vehicles":[{"make":"Honda","model":"Civic","color":"Blue","license":"ABC123","state":"WA","type":"standard"}]
	}`)
	p := parseLoginProfile(data)
	if p == nil {
		t.Fatal("expected profile, got nil")
	}
	if p.FirstName != "Alice" || p.LastName != "Smith" || p.Email != "alice@example.com" || p.Phone != "phone-test" {
		t.Errorf("customer fields mismatch: %+v", p)
	}
	if p.CustomerID != "C123" {
		t.Errorf("customer id = %q", p.CustomerID)
	}
	if len(p.Vehicles) != 1 {
		t.Fatalf("vehicles = %d", len(p.Vehicles))
	}
	v := p.Vehicles[0]
	if v.Make != "Honda" || v.Model != "Civic" || v.Color != "Blue" || v.License != "ABC123" || v.State != "WA" || v.Type != "standard" {
		t.Errorf("vehicle mismatch: %+v", v)
	}
}

func TestParseLoginProfileFlatAliases(t *testing.T) {
	// Flat customer fields (no nested object) and alias vehicle keys.
	data := json.RawMessage(`{"first_name":"Bob","last_name":"Lee","email":"b@e.com",
		"vehicles":[{"vehicle_make":"Toyota","vehicle_model":"Camry","plate":"XYZ789"}]}`)
	p := parseLoginProfile(data)
	if p == nil {
		t.Fatal("expected profile, got nil")
	}
	if p.FirstName != "Bob" || p.LastName != "Lee" {
		t.Errorf("flat customer fields mismatch: %+v", p)
	}
	if len(p.Vehicles) != 1 || p.Vehicles[0].Make != "Toyota" || p.Vehicles[0].License != "XYZ789" {
		t.Errorf("flat vehicle alias mismatch: %+v", p.Vehicles)
	}
}

func TestParseLoginProfileEmpty(t *testing.T) {
	if p := parseLoginProfile(json.RawMessage(`[]`)); p != nil {
		t.Errorf("expected nil for array data, got %+v", p)
	}
	if p := parseLoginProfile(nil); p != nil {
		t.Errorf("expected nil for empty data, got %+v", p)
	}
	if p := parseLoginProfile(json.RawMessage(`{"unrelated":true}`)); p != nil {
		t.Errorf("expected nil for profile-less data, got %+v", p)
	}
}

func TestLoginAndProfileExtractsProfile(t *testing.T) {
	rec := newRecordingLoginServer(t)
	c := client.New(rec.srv.URL, 5*time.Second)
	profile, err := loginAndProfile(context.Background(), c, "alice@example.com", "secret", "2515-1-889")
	if err != nil {
		t.Fatalf("loginAndProfile: %v", err)
	}
	if profile == nil || profile.FirstName != "Alice" || len(profile.Vehicles) != 1 {
		t.Fatalf("profile not extracted: %+v", profile)
	}
}

func TestAuthSyncProfileVerifyNoOp(t *testing.T) {
	var hits int32
	srv := recordingServer(t, &hits)
	defer srv.Close()
	t.Setenv("MASTERPARK_BASE_URL", srv.URL)
	t.Setenv("PRINTING_PRESS_VERIFY", "1")

	g := &globalOpts{timeout: 5 * time.Second, json: true}
	out, err := runCmd(t, newAuthCmd(g), "sync-profile", "--lot", "B",
		"--username", "alice@example.com", "--password", "secret")
	if err != nil {
		t.Fatalf("sync-profile verify: %v", err)
	}
	if !strings.Contains(out, "verify_noop") {
		t.Errorf("expected verify_noop, got: %s", out)
	}
	if hits != 0 {
		t.Errorf("verify mode must not hit network, got %d hits", hits)
	}
}

// TestReservationsListUsesCredentialCommands exercises the generic,
// 1Password-independent credential path: a --password-command/--username-command
// whose stdout feeds verifyLogin, with no env/config/1Password involved.
func TestReservationsListUsesCredentialCommands(t *testing.T) {
	rec := newRecordingLoginServer(t)
	t.Setenv("MASTERPARK_BASE_URL", rec.srv.URL)
	t.Setenv(config.EnvUsername, "")
	t.Setenv(config.EnvPassword, "")

	g := &globalOpts{timeout: 5 * time.Second}
	_, err := runCmd(t, newReservationsCmd(g), "list", "--lot", "B",
		"--username-command", "echo alice@example.com",
		"--password-command", "echo secret")
	if err != nil {
		t.Fatalf("reservations list with cred commands: %v", err)
	}
	if len(rec.bodies) < 1 {
		t.Fatalf("expected at least a verifyLogin call, got %d", len(rec.bodies))
	}
	login := rec.bodies[0]
	if login["method"] != "verifyLogin" || login["login"] != "alice@example.com" {
		t.Errorf("cred command did not feed verifyLogin: %v", login)
	}
	if login["password"] != "secret" {
		t.Errorf("password command not applied to login payload")
	}
}

// TestReserveDryRunUsesSavedProfileJSON verifies that saved-profile defaults
// surface in the JSON dry-run summary without any customer/vehicle flags.
func TestReserveDryRunUsesSavedProfileJSON(t *testing.T) {
	dir := t.TempDir()
	cfgPath := dir + "/config.json"
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
		t.Fatal(err)
	}

	g := &globalOpts{timeout: 5 * time.Second, configPath: cfgPath, json: true}
	out, err := runCmd(t, newReserveCmd(g),
		"--lot", "B", "--dropoff", "2026-06-11 07:00", "--pickup", "2026-06-13 18:30", "--quote", "1")
	if err != nil {
		t.Fatalf("reserve dry-run with saved profile: %v", err)
	}
	if !strings.Contains(out, "dry-run") {
		t.Errorf("expected dry-run mode, got: %s", out)
	}
	for _, want := range []string{"Alice", "Smith", "Honda", "Civic", "ABC123"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected saved-profile value %q in JSON output: %s", want, out)
		}
	}
}
