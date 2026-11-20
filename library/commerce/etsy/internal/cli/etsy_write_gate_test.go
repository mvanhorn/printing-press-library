package cli

import (
	"net/http"
	"strings"
	"testing"
)

const gateMsg = "refusing to send DELETE /listings/1000000001 without --yes (preview it first with --dry-run)"

func TestWriteGateRefusesDeleteWithoutYes(t *testing.T) {
	srv := s2NewServer(t, nil)
	_, _, err := s2Run(t, "listings", "delete", "1000000001", "--json", "--no-cache", "--rate-limit", "1000")
	if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), gateMsg) {
		t.Fatalf("err = %v (exit %d)", err, ExitCode(err))
	}
	if n := len(srv.all()); n != 0 {
		t.Fatalf("requests sent = %d, want 0", n)
	}
}

func TestWriteGateYesSendsDelete(t *testing.T) {
	srv := s2NewServer(t, func(w http.ResponseWriter, r *http.Request, body string) bool {
		w.WriteHeader(204)
		return true
	})
	_, _, err := s2Run(t, "listings", "delete", "1000000001", "--yes", "--json", "--no-cache", "--rate-limit", "1000")
	s2Must(t, err)
	if srv.count("DELETE", "/listings/1000000001") != 1 {
		t.Fatalf("requests = %+v", srv.all())
	}
}

func TestWriteGateDryRunSendsNothing(t *testing.T) {
	srv := s2NewServer(t, nil)
	_, _, err := s2Run(t, "listings", "delete", "1000000001", "--dry-run", "--json", "--no-cache", "--rate-limit", "1000")
	s2Must(t, err)
	if n := len(srv.all()); n != 0 {
		t.Fatalf("requests sent = %d, want 0", n)
	}
}

func TestWriteGateRefusesUpdateAndCreate(t *testing.T) {
	srv := s2NewServer(t, nil)
	_, _, err := s2Run(t, "listings", "update", "9000001", "1000000001", "--title", "x", "--no-cache", "--rate-limit", "1000")
	if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "without --yes") {
		t.Fatalf("update err = %v (exit %d)", err, ExitCode(err))
	}
	if n := len(srv.all()); n != 0 {
		t.Fatalf("requests sent = %d, want 0", n)
	}
}

func TestWriteGateTokenScopesPostIsExempt(t *testing.T) {
	srv := s2NewServer(t, nil)
	_, _, err := s2Run(t, "app", "token-scopes", "--token", "12345.abcdef", "--json", "--no-cache", "--rate-limit", "1000")
	s2Must(t, err)
	if srv.count("POST", "/scopes") != 1 {
		t.Fatalf("requests = %+v", srv.all())
	}
}
