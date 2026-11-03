package cli

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGravitusDjangoLoginUsesCurrentLoginForm(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			http.SetCookie(w, &http.Cookie{Name: "csrftoken", Value: "synthetic-csrf-cookie", Path: "/"})
			fmt.Fprint(w, `<form action="/accounts/login/"><input name="csrfmiddlewaretoken" value="synthetic-csrf-token"></form>`)
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/accounts/login/" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if !strings.Contains(r.Header.Get("Cookie"), "csrftoken=synthetic-csrf-cookie") {
			t.Errorf("CSRF cookie was not sent: %q", r.Header.Get("Cookie"))
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
			return
		}
		if r.Form.Get("username") != "person@example.invalid" || r.Form.Get("email") != "" {
			t.Errorf("unexpected username fields: %#v", r.Form)
		}
		if r.Form.Get("csrfmiddlewaretoken") != "synthetic-csrf-token" || r.Form.Get("password") != "synthetic p+=ss" {
			t.Errorf("unexpected login form values: %#v", r.Form)
		}
		http.SetCookie(w, &http.Cookie{Name: "sessionid", Value: "synthetic-session", Path: "/"})
	}))
	defer server.Close()
	trustTLSTestServer(t, server)

	sessionID, _, err := gravitusDjangoLogin(server.URL, "person@example.invalid", "synthetic p+=ss")
	if err != nil {
		t.Fatal(err)
	}
	if sessionID != "synthetic-session" {
		t.Fatalf("got session ID %q", sessionID)
	}
}

func TestGravitusDjangoLoginRejectsInsecureOrCrossOriginRequests(t *testing.T) {
	plainRequests := 0
	plainServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		plainRequests++
	}))
	defer plainServer.Close()
	if _, _, err := gravitusDjangoLogin(plainServer.URL, "person@example.invalid", "synthetic-password"); err == nil || plainRequests != 0 {
		t.Fatalf("expected HTTP base URL rejection before request, got err %v and %d requests", err, plainRequests)
	}

	for _, action := range []string{"https://example.invalid/accounts/login/", "http://127.0.0.1/accounts/login/"} {
		t.Run(action, func(t *testing.T) {
			posts := 0
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					posts++
				}
				fmt.Fprintf(w, `<form action="%s"><input name="csrfmiddlewaretoken" value="synthetic-token"></form>`, action)
			}))
			defer server.Close()
			trustTLSTestServer(t, server)

			_, _, err := gravitusDjangoLogin(server.URL, "person@example.invalid", "synthetic-password")
			if err == nil || !strings.Contains(err.Error(), "same HTTPS origin") {
				t.Fatalf("expected unsafe form action error, got %v", err)
			}
			if posts != 0 {
				t.Fatalf("unsafe form action received %d POST requests", posts)
			}
		})
	}
}

func TestGravitusDjangoLoginRejectsCrossOriginRedirect(t *testing.T) {
	requests := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Redirect(w, r, "https://example.invalid/", http.StatusFound)
	}))
	defer server.Close()
	trustTLSTestServer(t, server)

	_, _, err := gravitusDjangoLogin(server.URL, "person@example.invalid", "synthetic-password")
	if err == nil || !strings.Contains(err.Error(), "same HTTPS origin") || requests != 1 {
		t.Fatalf("expected redirect rejection before following, got err %v and %d requests", err, requests)
	}
}

func trustTLSTestServer(t *testing.T, server *httptest.Server) {
	t.Helper()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	transport.TLSClientConfig = &tls.Config{RootCAs: roots}
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() {
		http.DefaultTransport = previous
		transport.CloseIdleConnections()
	})
}
