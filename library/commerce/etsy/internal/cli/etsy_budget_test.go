package cli

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
	"github.com/spf13/cobra"
)

func TestEvaluateBudget(t *testing.T) {
	for _, tc := range []struct {
		name               string
		known              bool
		planned, remaining int
		want               string
	}{
		{"unknown", false, 100, 0, ""}, {"ample", true, 100, 4000, ""},
		{"too many", true, 200, 150, "planned 200"}, {"low", true, 1, 200, "nearly exhausted"},
		{"boundary", true, 1, 250, ""}, {"zero", true, 0, 0, ""}, {"negative", true, -1, 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := evaluateBudget(client.EtsyQuota{LimitPerDay: 5000, RemainingToday: tc.remaining}, tc.known, tc.planned)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var ce *cliError
			if !errors.As(err, &ce) || ce.code != 7 || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v", err)
			}
			if tc.name == "too many" && !strings.Contains(err.Error(), "150 remaining") {
				t.Fatalf("missing balance: %v", err)
			}
		})
	}
}
func TestCheckBudgetAndSummary(t *testing.T) {
	restore, err := cliutil.SetHomeOverride("")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restore)
	testenv.Isolate(t, cliutil.StateDir)
	if s, ok := etsyQuotaSummary(os.Getenv("ETSY_API_KEY")); ok || s != "unknown: no observation in the last 24h for this key" {
		t.Fatalf("unknown summary: %q %v", s, ok)
	}
	h := make(http.Header)
	h.Set("x-limit-per-day", "5000")
	h.Set("x-remaining-today", "0")
	client.ObserveEtsyQuota(h, etsyTestQuotaKey(t))
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetErr(&buf)
	if err := checkBudget(cmd, 100, true); err != nil {
		t.Fatal(err)
	}
	if err := checkBudget(cmd, 0, false); err != nil {
		t.Fatal(err)
	}
	if err := checkBudget(cmd, 1, false); err == nil {
		t.Fatal("exhausted budget allowed")
	}
	h.Set("x-remaining-today", "500")
	client.ObserveEtsyQuota(h, etsyTestQuotaKey(t))
	if err := checkBudget(cmd, 1, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "note: 500 of 5000 daily Etsy calls left") {
		t.Fatalf("note = %q", buf.String())
	}
	if s, ok := etsyQuotaSummary(os.Getenv("ETSY_API_KEY")); !ok || !strings.HasPrefix(s, "known: remaining 500 of 5000 (observed ") {
		t.Fatalf("summary = %q %v", s, ok)
	}
}

var etsyQuotaFixtureSerial atomic.Int64

// etsyTestQuotaKey gives independent budget scenarios independent principals.
func etsyTestQuotaKey(t *testing.T) string {
	t.Helper()
	key := fmt.Sprintf("synthetic-quota-key-%d", etsyQuotaFixtureSerial.Add(1))
	t.Setenv("ETSY_API_KEY", key)
	return key
}

func TestEtsyQuotaUsesSelectedConfig(t *testing.T) {
	t.Setenv("ETSY_API_KEY", "")
	testenv.Isolate(t)
	s2NewServer(t, nil)
	t.Setenv("ETSY_API_KEY", "")
	dir, _ := cliutil.ConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("api_key = \"synthetic-default:secret\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	client.ObserveEtsyQuota(http.Header{"X-Remaining-Today": {"4000"}}, "synthetic-default")
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("api_key = \"synthetic-selected-key:synthetic-secret\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{}
	cmd.Flags().String("config", path, "")
	key := etsyQuotaKey(cmd)
	if key != "synthetic-selected-key:synthetic-secret" {
		t.Fatalf("selected key not loaded")
	}
	client.ObserveEtsyQuota(http.Header{"X-Remaining-Today": {"0"}}, key)
	out, _, runErr := s2Run(t, "doctor", "--json", "--config", path)
	if runErr != nil || !strings.Contains(out, "known: remaining 0") {
		t.Fatalf("root config: %s %v", out, runErr)
	}
	if err := checkBudget(cmd, 1, false); err == nil {
		t.Fatal("selected config quota ignored")
	}
	if q, ok := client.LoadEtsyQuota("synthetic-default"); !ok || q.RemainingToday != 4000 {
		t.Fatalf("default quota changed: %+v %v", q, ok)
	}
	if err := checkBudget(&cobra.Command{}, 1, false); err != nil {
		t.Fatalf("selected key leaked into default config: %v", err)
	}
}
