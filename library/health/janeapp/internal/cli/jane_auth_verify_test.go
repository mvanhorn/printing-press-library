// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written: covers janeVerifySession, the guard that stops a logged-out
// browser cookie from being imported as a working session.

package cli

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// janeVerifyServer stands in for a Jane instance: /api/v2/appointments answers
// 200 only for the one session value it considers signed in, and 401 for
// everything else — which is how the real API behaves for an absent, an
// invalid, and a valid-but-anonymous session alike.
func janeVerifyServer(t *testing.T, authedSession string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/appointments" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		ck, err := r.Cookie("_front_desk_session")
		if err != nil || ck.Value != authedSession {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"Sorry, looks like you don't have access to that area."}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"appointments":[]}`))
	}))
}

func TestJaneVerifySessionAcceptsAuthenticated(t *testing.T) {
	srv := janeVerifyServer(t, "good-session")
	defer srv.Close()

	if err := janeVerifySession(context.Background(), srv.URL, "_front_desk_session=good-session", 5*time.Second); err != nil {
		t.Fatalf("expected authenticated session to verify, got %v", err)
	}
}

// The regression this whole change exists for: Jane hands _front_desk_session
// to logged-out visitors too, so a cookie import can succeed by name and still
// carry an anonymous session. Presence must never be treated as proof.
func TestJaneVerifySessionRejectsAnonymous(t *testing.T) {
	srv := janeVerifyServer(t, "good-session")
	defer srv.Close()

	err := janeVerifySession(context.Background(), srv.URL, "_front_desk_session=anonymous-visitor", 5*time.Second)
	if !errors.Is(err, errAnonymousSession) {
		t.Fatalf("expected errAnonymousSession for a logged-out cookie, got %v", err)
	}
}

func TestJaneVerifySessionRejectsEmptyCookies(t *testing.T) {
	srv := janeVerifyServer(t, "good-session")
	defer srv.Close()

	for _, cookies := range []string{"", "   ", ";", "  ;  "} {
		if err := janeVerifySession(context.Background(), srv.URL, cookies, 5*time.Second); !errors.Is(err, errAnonymousSession) {
			t.Fatalf("cookies %q: expected errAnonymousSession, got %v", cookies, err)
		}
	}
}

// A trailing "; " and surrounding whitespace are normal in cookie strings
// assembled from a jar; they must not be mistaken for an empty set.
func TestJaneVerifySessionParsesMultiCookieString(t *testing.T) {
	srv := janeVerifyServer(t, "good-session")
	defer srv.Close()

	if err := janeVerifySession(context.Background(), srv.URL, " jane_device=abc; _front_desk_session=good-session; ", 5*time.Second); err != nil {
		t.Fatalf("expected multi-cookie string to verify, got %v", err)
	}
}

// A server-side fault must not be reported as "you are logged out" — that
// would send the user off to re-authenticate a session that is actually fine.
func TestJaneVerifySessionDistinguishesServerErrorFromAnonymous(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := janeVerifySession(context.Background(), srv.URL, "_front_desk_session=whatever", 5*time.Second)
	if err == nil {
		t.Fatal("expected an error for a 500 response")
	}
	if errors.Is(err, errAnonymousSession) {
		t.Fatalf("500 must not be reported as an anonymous session, got %v", err)
	}
}

func TestJaneVerifySessionUnreachableHost(t *testing.T) {
	srv := janeVerifyServer(t, "good-session")
	url := srv.URL
	srv.Close() // close immediately so the address refuses connections

	err := janeVerifySession(context.Background(), url, "_front_desk_session=good-session", 2*time.Second)
	if err == nil {
		t.Fatal("expected an error for an unreachable host")
	}
	if errors.Is(err, errAnonymousSession) {
		t.Fatalf("a transport failure must not be reported as anonymous, got %v", err)
	}
}
