// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

package granola

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// deviceServer stands in for Granola's WorkOS endpoints. tokenStages is walked
// one entry per poll, so a test can script pending -> pending -> success.
func deviceServer(t *testing.T, expiresIn, interval int, tokenStages []func(w http.ResponseWriter)) *httptest.Server {
	t.Helper()
	var polls int32
	mux := http.NewServeMux()
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"device_code":"dev-code","user_code":"ABCD-EFGH",` +
			`"verification_uri":"https://example.test/device",` +
			`"verification_uri_complete":"https://example.test/device?user_code=ABCD-EFGH",` +
			`"expires_in":` + itoa(expiresIn) + `,"interval":` + itoa(interval) + `}`))
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		i := int(atomic.AddInt32(&polls, 1)) - 1
		if i >= len(tokenStages) {
			i = len(tokenStages) - 1
		}
		tokenStages[i](w)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	SetDeviceEndpoints(srv.URL+"/authorize", srv.URL+"/token")
	SetDeviceHTTPClient(srv.Client())
	t.Cleanup(func() {
		SetDeviceEndpoints(DeviceAuthorizeEndpoint, DeviceTokenEndpoint)
		SetDeviceHTTPClient(&http.Client{Timeout: 30 * time.Second})
	})
	return srv
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

func pendingStage(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(`{"error":"authorization_pending"}`))
}

func successStage(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"access_token":"at-value","refresh_token":"rt-value",` +
		`"user":{"id":"user_1","email":"someone@example.com"},"authentication_method":"GoogleOAuth"}`))
}

func TestDeviceAuthPendingThenSuccess(t *testing.T) {
	deviceServer(t, 60, 0, []func(http.ResponseWriter){pendingStage, pendingStage, successStage})
	ctx := context.Background()
	dc, err := RequestDeviceCode(ctx)
	if err != nil {
		t.Fatalf("RequestDeviceCode: %v", err)
	}
	if dc.UserCode != "ABCD-EFGH" {
		t.Errorf("user code: got %q", dc.UserCode)
	}
	// A zero interval from the server must be defaulted, not used as a
	// busy-loop.
	if dc.Interval <= 0 {
		t.Errorf("interval should default to a positive value, got %d", dc.Interval)
	}
	dc.Interval = 0 // keep the test fast; the loop still polls
	s, err := PollDeviceToken(ctx, dc)
	if err != nil {
		t.Fatalf("PollDeviceToken: %v", err)
	}
	if s.AccessToken != "at-value" || s.RefreshToken != "rt-value" {
		t.Error("tokens not carried out of the success envelope")
	}
	if s.AccountEmail != "someone@example.com" || s.AccountID != "user_1" {
		t.Errorf("account identity not captured: %q %q", s.AccountEmail, s.AccountID)
	}
}

// slow_down must lengthen the interval, not abort. Aborting would drop a grant
// the user is in the middle of approving.
func TestDeviceAuthSlowDownBacksOff(t *testing.T) {
	slow := func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"slow_down"}`))
	}
	deviceServer(t, 60, 0, []func(http.ResponseWriter){slow, successStage})
	dc, err := RequestDeviceCode(context.Background())
	if err != nil {
		t.Fatalf("RequestDeviceCode: %v", err)
	}
	dc.Interval = 0
	start := time.Now()
	s, err := PollDeviceToken(context.Background(), dc)
	if err != nil {
		t.Fatalf("slow_down should not abort the grant: %v", err)
	}
	if s.AccessToken == "" {
		t.Error("no token after backoff")
	}
	if time.Since(start) < 5*time.Second {
		t.Error("slow_down did not lengthen the poll interval")
	}
}

func TestDeviceAuthExpiredToken(t *testing.T) {
	expired := func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"expired_token"}`))
	}
	deviceServer(t, 60, 0, []func(http.ResponseWriter){expired})
	dc, _ := RequestDeviceCode(context.Background())
	dc.Interval = 0
	if _, err := PollDeviceToken(context.Background(), dc); !errors.Is(err, ErrDeviceCodeExpired) {
		t.Fatalf("want ErrDeviceCodeExpired, got %v", err)
	}
}

func TestDeviceAuthDeadlineElapses(t *testing.T) {
	deviceServer(t, 0, 0, []func(http.ResponseWriter){pendingStage})
	dc, _ := RequestDeviceCode(context.Background())
	dc.Interval = 0
	dc.ExpiresIn = 0 // already past
	if _, err := PollDeviceToken(context.Background(), dc); !errors.Is(err, ErrDeviceCodeExpired) {
		t.Fatalf("want ErrDeviceCodeExpired once the window lapses, got %v", err)
	}
}

func TestDeviceAuthDenied(t *testing.T) {
	denied := func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"access_denied"}`))
	}
	deviceServer(t, 60, 0, []func(http.ResponseWriter){denied})
	dc, _ := RequestDeviceCode(context.Background())
	dc.Interval = 0
	if _, err := PollDeviceToken(context.Background(), dc); !errors.Is(err, ErrDeviceAuthDenied) {
		t.Fatalf("want ErrDeviceAuthDenied, got %v", err)
	}
}

func TestDeviceAuthContextCancel(t *testing.T) {
	deviceServer(t, 60, 0, []func(http.ResponseWriter){pendingStage})
	ctx, cancel := context.WithCancel(context.Background())
	dc, _ := RequestDeviceCode(ctx)
	dc.Interval = 1
	go func() { time.Sleep(150 * time.Millisecond); cancel() }()
	_, err := PollDeviceToken(ctx, dc)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}
