package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
)

func TestEtsyWriteGateVerbsAndEnvelope(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"POST", []string{"listings", "create-draft", "9000001", "--description", "Synthetic", "--price", "10", "--quantity", "1", "--taxonomy-id", "1", "--title", "Synthetic", "--when-made", "2020_2026", "--who-made", "i_did"}},
		{"PUT", []string{"listing-inventory", "update", "1000000001", "--products", `[]`}},
		{"copy", []string{"listings", "copy", "1000000001"}},
		{"DELETE JSON", []string{"listings", "delete", "1000000001", "--json"}},
		{"DELETE agent", []string{"listings", "delete", "1000000001", "--agent"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			srv := s2NewServer(t, nil)
			out, _, err := s2Run(t, append(tc.args, "--no-cache", "--rate-limit", "1000")...)
			if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "without --yes") {
				t.Fatalf("err=%v exit=%d", err, ExitCode(err))
			}
			if len(srv.all()) != 0 {
				t.Fatalf("sent requests: %+v", srv.all())
			}
			if strings.HasPrefix(tc.name, "DELETE") {
				var envelope map[string]any
				if err := json.Unmarshal([]byte(out), &envelope); err != nil {
					t.Fatalf("stdout not JSON: %v: %s", err, out)
				}
				if envelope["reason"] != "confirmation_required" || envelope["code"] != float64(2) || envelope["error"] == nil {
					t.Fatalf("envelope=%v", envelope)
				}
			}
		})
	}
}

func TestEtsySyncMissingShopHintOnce(t *testing.T) {
	testenv.Isolate(t)
	srv := s2NewServer(t, nil)
	for _, key := range []string{"ETSY_SHOP_ID", "ETSY_OAUTH2", "ETSY_ACCESS_TOKEN", "ETSY_REFRESH_TOKEN"} {
		t.Setenv(key, "")
	}
	// Human sync writes progress to the process stderr, including worker progress.
	old := os.Stderr
	f, err := os.CreateTemp(t.TempDir(), "stderr")
	s2Must(t, err)
	defer f.Close()
	os.Stderr = f
	defer func() { os.Stderr = old }()
	_, _, err = s2Run(t, "sync", "--human-friendly", "--resources", "listings,receipts", "--rate-limit", "1000")
	os.Stderr = old
	if err == nil || !strings.Contains(err.Error(), "warnings but no successful syncs") {
		t.Fatalf("sync err=%v", err)
	}
	_, err = f.Seek(0, 0)
	s2Must(t, err)
	data, err := io.ReadAll(f)
	s2Must(t, err)
	if n := strings.Count(string(data), "set ETSY_SHOP_ID to your shop id to sync shop data"); n != 1 {
		t.Fatalf("hint count=%d stderr=%s", n, data)
	}
	for _, req := range srv.all() {
		if strings.HasPrefix(req.Path, "/shops/") {
			t.Errorf("shop request: %+v", req)
		}
	}
}

func TestEtsyDoctorKeyFormatAndQuota(t *testing.T) {
	for _, key := range []string{"barekey", "key:secret"} {
		t.Run(key, func(t *testing.T) {
			testenv.Isolate(t) // Each isolated state directory also resets observed quota.
			s2NewServer(t, nil)
			t.Setenv("ETSY_API_KEY", key)
			t.Setenv("ETSY_SHARED_SECRET", "")
			run := func() map[string]any {
				out, _, err := s2Run(t, "doctor", "--json", "--no-cache", "--rate-limit", "1000")
				s2Must(t, err)
				var report map[string]any
				if err := json.Unmarshal([]byte(out), &report); err != nil {
					t.Fatal(err)
				}
				if strings.Contains(out, "key:secret") {
					t.Fatal("credential leaked")
				}
				return report
			}
			report := run()
			want := "OK"
			if key == "barekey" {
				want = "WARN"
			}
			if text, _ := report["api_key_format"].(string); !strings.HasPrefix(text, want) || strings.Contains(text, key) {
				t.Fatalf("format=%v", report["api_key_format"])
			}
			if report["quota"] != "unknown: no observation in the last 24h for this key" {
				t.Fatalf("unexpected quota=%v", report["quota"])
			}
			client.ObserveEtsyQuota(http.Header{"X-Limit-Per-Day": {"5000"}, "X-Remaining-Today": {"4321"}}, etsyTestQuotaKey(t))
			summary, ok := etsyQuotaSummary(os.Getenv("ETSY_API_KEY"))
			if !ok {
				t.Fatal("missing quota summary")
			}
			if report = run(); report["quota"] != summary {
				t.Fatalf("quota=%v want=%s", report["quota"], summary)
			}
		})
	}
}

// etsy-ext: doctor must tolerate broken TOML and compose selected keystrings.
func TestEtsyDoctorConfigQuota(t *testing.T) {
	for _, bad := range []bool{true, false} {
		t.Run(fmt.Sprint(bad), func(t *testing.T) {
			testenv.Isolate(t)
			t.Setenv("ETSY_API_KEY", "")
			t.Setenv("ETSY_SHARED_SECRET", "")
			s2NewServer(t, nil)
			t.Setenv("ETSY_API_KEY", "")
			path := filepath.Join(t.TempDir(), "config.toml")
			body := "api_key = \"k:s\"\n"
			want := "known: remaining 4321 of 5000"
			if bad {
				body = "[broken"
				want = "unknown (config error)"
			} else {
				client.ObserveEtsyQuota(http.Header{"X-Remaining-Today": {"4321"}, "X-Limit-Per-Day": {"5000"}}, "k")
			}
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			out, _, err := s2Run(t, "doctor", "--json", "--config", path)
			if err != nil {
				t.Fatal(err)
			}
			var report map[string]any
			if err := json.Unmarshal([]byte(out), &report); err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(fmt.Sprint(report["quota"]), want) {
				t.Fatalf("quota %v", report["quota"])
			}
			if bad && !strings.HasPrefix(fmt.Sprint(report["config"]), "error:") {
				t.Fatalf("config %v", report["config"])
			}
		})
	}
}
