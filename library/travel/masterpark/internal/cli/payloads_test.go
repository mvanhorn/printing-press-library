package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/masterpark/internal/client"
)

// ajaxRecorder is a mock MasterPark site that serves the nonce page and
// captures every ajax.php JSON body it receives.
type ajaxRecorder struct {
	mu     sync.Mutex
	bodies []map[string]interface{}
	srv    *httptest.Server
}

func newAjaxRecorder(t *testing.T) *ajaxRecorder {
	t.Helper()
	rec := &ajaxRecorder{}
	mux := http.NewServeMux()
	mux.HandleFunc("/reservation/book/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, `<script>window._wpnonce = "nonce";</script>`)
	})
	mux.HandleFunc("/wp-content/plugins/netParkV2/ajax.php", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]interface{}
		_ = json.Unmarshal(body, &parsed)
		rec.mu.Lock()
		rec.bodies = append(rec.bodies, parsed)
		rec.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"errors":[],"data":[]}`)
	})
	rec.srv = httptest.NewServer(mux)
	t.Cleanup(rec.srv.Close)
	return rec
}

func TestVerifyLoginPayloadShape(t *testing.T) {
	rec := newAjaxRecorder(t)
	c := client.New(rec.srv.URL, 5*time.Second)

	ok, err := verifyLoginWithClient(context.Background(), c, "alice@example.com", "secret", "2515-1-889")
	if err != nil {
		t.Fatalf("verifyLoginWithClient: %v", err)
	}
	if !ok {
		t.Fatalf("expected verifyLogin to succeed")
	}
	if len(rec.bodies) != 1 {
		t.Fatalf("want 1 ajax call, got %d", len(rec.bodies))
	}
	p := rec.bodies[0]
	if p["action"] != "np_ajax" || p["method"] != "verifyLogin" {
		t.Errorf("action/method = %v/%v", p["action"], p["method"])
	}
	if p["login"] != "alice@example.com" {
		t.Errorf("login = %v", p["login"])
	}
	if p["password"] != "secret" {
		t.Errorf("password = %v", p["password"])
	}
	if p["location"] != "2515-1-889" {
		t.Errorf("location = %v", p["location"])
	}
	if _, ok := p["email"]; ok {
		t.Errorf("verifyLogin must not send email")
	}
	if _, ok := p["username"]; ok {
		t.Errorf("verifyLogin must not send username")
	}
}

func TestReservationsListPayloadShape(t *testing.T) {
	rec := newAjaxRecorder(t)
	t.Setenv("MASTERPARK_BASE_URL", rec.srv.URL)
	t.Setenv("MASTERPARK_USERNAME", "alice@example.com")
	t.Setenv("MASTERPARK_PASSWORD", "secret")

	g := &globalOpts{timeout: 5 * time.Second}
	if _, err := runCmd(t, newReservationsCmd(g), "list", "--lot", "B"); err != nil {
		t.Fatalf("reservations list: %v", err)
	}

	if len(rec.bodies) != 2 {
		t.Fatalf("want 2 ajax calls (verifyLogin, listReservations), got %d", len(rec.bodies))
	}
	login := rec.bodies[0]
	if login["method"] != "verifyLogin" || login["location"] != "2515-1-889" {
		t.Errorf("login call = %v", login)
	}
	list := rec.bodies[1]
	if list["action"] != "np_ajax" || list["method"] != "listReservations" {
		t.Errorf("list action/method = %v/%v", list["action"], list["method"])
	}
	if _, ok := list["email"]; ok {
		t.Errorf("listReservations must not send email")
	}
	if _, ok := list["username"]; ok {
		t.Errorf("listReservations must not send username")
	}
}

func TestSaveReservationPayloadShape(t *testing.T) {
	rf := &reserveFlags{
		lot:         "B",
		dropoff:     "2026-06-11 07:00",
		pickup:      "2026-06-13 18:30",
		vehicleType: "standard",
		promoCode:   "SAVE10",
		quoteID:     1,
		firstName:   "Alice",
		lastName:    "Smith",
		email:       "alice@example.com",
		phone:       "phone-test",
		vehMake:     "Honda",
		vehModel:    "Civic",
		vehColor:    "blue",
		plate:       "ABC123",
	}
	payload := buildSaveReservationPayload("2515-1-889", rf)

	if payload["action"] != "np_ajax" || payload["method"] != "saveReservation" {
		t.Errorf("action/method = %v/%v", payload["action"], payload["method"])
	}
	if payload["location"] != "2515-1-889" {
		t.Errorf("location = %v", payload["location"])
	}
	if _, ok := payload["multi_locations"]; ok {
		t.Errorf("saveReservation must not send multi_locations")
	}
	if _, ok := payload["resRate"]; ok {
		t.Errorf("saveReservation must not send resRate")
	}

	res, ok := payload["reservation"].(map[string]interface{})
	if !ok {
		t.Fatalf("reservation is not an object: %v", payload["reservation"])
	}
	flat := map[string]interface{}{
		"start_date": "2026-06-11 07:00",
		"end_date":   "2026-06-13 18:30",
		"promo_code": "SAVE10",
		"source":     "website",
		"quote":      1,
		"first_name": "Alice",
		"last_name":  "Smith",
		"email":      "alice@example.com",
		"phone":      "phone-test",
		"license":    "ABC123",
		"make":       "Honda",
		"model":      "Civic",
		"color":      "blue",
	}
	for k, want := range flat {
		if got := res[k]; got != want {
			t.Errorf("reservation[%q] = %v, want %v", k, got, want)
		}
	}
	if _, ok := res["customer"]; ok {
		t.Errorf("reservation must not nest a customer object; fields are flat")
	}
	// A nested vehicle object may exist but must not be the only home for the
	// flat fields above.
	if veh, ok := res["vehicle"].(map[string]interface{}); ok {
		if veh["make"] != "Honda" {
			t.Errorf("nested vehicle.make = %v", veh["make"])
		}
	}
}
