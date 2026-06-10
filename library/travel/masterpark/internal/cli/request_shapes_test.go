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

type ajaxCapture struct {
	Methods []string
	Bodies  []map[string]interface{}
}

func newAjaxShapeServer(t *testing.T, cap *ajaxCapture) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/reservation/book/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, `<html><script>window._wpnonce = "shape-nonce";</script></html>`)
	})
	mux.HandleFunc("/wp-content/plugins/netParkV2/ajax.php", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]interface{}
		_ = json.Unmarshal(body, &parsed)
		method, _ := parsed["method"].(string)
		cap.Methods = append(cap.Methods, method)
		cap.Bodies = append(cap.Bodies, parsed)
		w.Header().Set("Content-Type", "application/json")
		switch method {
		case "verifyLogin":
			_, _ = io.WriteString(w, `{"errors":[],"data":{"customer":123}}`)
		case "listReservations":
			_, _ = io.WriteString(w, `{"errors":[],"data":[{"reservation":"R123"}]}`)
		case "saveReservation":
			_, _ = io.WriteString(w, `{"errors":[],"data":{"reservation":"R999"}}`)
		default:
			_, _ = io.WriteString(w, `{"errors":[],"data":{}}`)
		}
	})
	return httptest.NewServer(mux)
}

func TestVerifyLoginRequestShape(t *testing.T) {
	cap := &ajaxCapture{}
	srv := newAjaxShapeServer(t, cap)
	defer srv.Close()

	c := client.New(srv.URL, 5*time.Second)
	ok, err := verifyLoginWithClient(context.Background(), c, "alice@example.com", "secret", "2525-1-893")
	if err != nil {
		t.Fatalf("verifyLoginWithClient: %v", err)
	}
	if !ok {
		t.Fatalf("verifyLoginWithClient returned false")
	}
	if len(cap.Bodies) != 1 {
		t.Fatalf("want 1 ajax call, got %d", len(cap.Bodies))
	}
	body := cap.Bodies[0]
	want := map[string]interface{}{
		"action":   "np_ajax",
		"method":   "verifyLogin",
		"login":    "alice@example.com",
		"password": "secret",
		"location": "2525-1-893",
	}
	for k, v := range want {
		if body[k] != v {
			t.Errorf("%s = %#v, want %#v (body=%v)", k, body[k], v, body)
		}
	}
	if _, ok := body["email"]; ok {
		t.Errorf("verifyLogin body must not include obsolete email field: %v", body)
	}
	if _, ok := body["username"]; ok {
		t.Errorf("verifyLogin body must not include obsolete username field: %v", body)
	}
}

func TestReservationsListRequestShape(t *testing.T) {
	cap := &ajaxCapture{}
	srv := newAjaxShapeServer(t, cap)
	defer srv.Close()
	t.Setenv("MASTERPARK_BASE_URL", srv.URL)
	t.Setenv(config.EnvUsername, "alice@example.com")
	t.Setenv(config.EnvPassword, "secret")

	g := &globalOpts{timeout: 5 * time.Second, json: true}
	_, err := runCmd(t, newReservationsListCmd(g), "--lot", "G")
	if err != nil {
		t.Fatalf("reservations list: %v", err)
	}
	if len(cap.Bodies) != 2 {
		t.Fatalf("want verifyLogin + listReservations, got %d calls: %v", len(cap.Bodies), cap.Methods)
	}
	login := cap.Bodies[0]
	if login["method"] != "verifyLogin" || login["login"] != "alice@example.com" || login["location"] != "2525-1-893" {
		t.Fatalf("unexpected verifyLogin body: %v", login)
	}
	list := cap.Bodies[1]
	if list["method"] != "listReservations" || list["action"] != "np_ajax" {
		t.Fatalf("unexpected listReservations body: %v", list)
	}
	if _, ok := list["email"]; ok {
		t.Errorf("listReservations should rely on session, not email: %v", list)
	}
	if _, ok := list["username"]; ok {
		t.Errorf("listReservations should rely on session, not username: %v", list)
	}
}

func TestReservationsListVerifyNoopSkipsLiveEndpoints(t *testing.T) {
	cap := &ajaxCapture{}
	srv := newAjaxShapeServer(t, cap)
	defer srv.Close()
	t.Setenv("MASTERPARK_BASE_URL", srv.URL)
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Setenv(config.EnvUsername, "alice@example.com")
	t.Setenv(config.EnvPassword, "secret")

	g := &globalOpts{timeout: 5 * time.Second, json: true}
	out, err := runCmd(t, newReservationsListCmd(g), "--lot", "G")
	if err != nil {
		t.Fatalf("reservations list under verify env: %v", err)
	}
	if len(cap.Bodies) != 0 {
		t.Fatalf("PRINTING_PRESS_VERIFY reservations list must not hit AJAX endpoints, got %d calls: %v", len(cap.Bodies), cap.Methods)
	}
	var got map[string]interface{}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("parse verify-noop output %q: %v", out, err)
	}
	if got["verify_noop"] != true || got["status"] != "verify-noop" {
		t.Fatalf("expected verify-noop output, got %v", got)
	}
	if got["command"] != "reservations list" {
		t.Fatalf("expected command marker, got %v", got)
	}
}

func TestReservationsListTransportErrorDoesNotBlameAuthSession(t *testing.T) {
	cap := &ajaxCapture{}
	mux := http.NewServeMux()
	mux.HandleFunc("/reservation/book/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, `<html><script>window._wpnonce = "shape-nonce";</script></html>`)
	})
	mux.HandleFunc("/wp-content/plugins/netParkV2/ajax.php", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]interface{}
		_ = json.Unmarshal(body, &parsed)
		method, _ := parsed["method"].(string)
		cap.Methods = append(cap.Methods, method)
		cap.Bodies = append(cap.Bodies, parsed)
		w.Header().Set("Content-Type", "application/json")
		if method == "verifyLogin" {
			_, _ = io.WriteString(w, `{"errors":[],"data":{"customer":123}}`)
			return
		}
		http.Error(w, "upstream unavailable", http.StatusServiceUnavailable)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	t.Setenv("MASTERPARK_BASE_URL", srv.URL)
	t.Setenv(config.EnvUsername, "alice@example.com")
	t.Setenv(config.EnvPassword, "secret")

	g := &globalOpts{timeout: 5 * time.Second, json: true}
	_, err := runCmd(t, newReservationsListCmd(g), "--lot", "G")
	if err == nil {
		t.Fatalf("expected listReservations error")
	}
	if strings.Contains(err.Error(), "authenticated browser session") {
		t.Fatalf("transport error should not blame auth session: %v", err)
	}
	if !strings.Contains(err.Error(), "listReservations failed") {
		t.Fatalf("expected neutral listReservations failure, got: %v", err)
	}
}

func TestSaveReservationRequestShape(t *testing.T) {
	rf := &reserveFlags{
		dropoff:     "2026-06-11 07:00",
		pickup:      "2026-06-13 18:30",
		vehicleType: "standard",
		promoCode:   "",
		quoteID:     0,
		firstName:   "Alice",
		lastName:    "Smith",
		email:       "alice@example.com",
		phone:       "phone-test",
		vehMake:     "Honda",
		vehModel:    "Civic",
		vehColor:    "Blue",
		plate:       "ABC123",
	}
	payload := buildSaveReservationPayload("2525-1-893", rf)
	if payload["action"] != "np_ajax" || payload["method"] != "saveReservation" || payload["location"] != "2525-1-893" {
		t.Fatalf("bad top-level saveReservation payload: %v", payload)
	}
	if _, ok := payload["multi_locations"]; ok {
		t.Fatalf("saveReservation payload must not contain quote-only multi_locations: %v", payload)
	}
	if _, ok := payload["resRate"]; ok {
		t.Fatalf("saveReservation payload must not contain quote-only resRate: %v", payload)
	}
	res, ok := payload["reservation"].(map[string]interface{})
	if !ok {
		t.Fatalf("reservation missing/not object: %v", payload["reservation"])
	}
	want := map[string]interface{}{
		"start_date": "2026-06-11 07:00",
		"end_date":   "2026-06-13 18:30",
		"source":     "website",
		"quote":      0,
		"first_name": "Alice",
		"last_name":  "Smith",
		"email":      "alice@example.com",
		"phone":      "phone-test",
		"license":    "ABC123",
		"make":       "Honda",
		"model":      "Civic",
		"color":      "Blue",
	}
	for k, v := range want {
		if res[k] != v {
			t.Errorf("reservation.%s = %#v, want %#v (reservation=%v)", k, res[k], v, res)
		}
	}
	if _, ok := res["customer"]; ok {
		t.Errorf("reservation should use flat customer fields, not nested customer: %v", res)
	}
}
