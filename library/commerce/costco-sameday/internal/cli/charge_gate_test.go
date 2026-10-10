// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored charge-gate and outcome tests (mock transport, no network).

package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"
)

// mockSameDay points the CLI at an httptest server (never the real API) and
// returns a hit counter. reply is written for every request.
func mockSameDay(t *testing.T, status int, reply string) *int32 {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	t.Setenv("COSTCO_SAMEDAY_CONFIG", filepath.Join(dir, "config.toml"))
	t.Setenv("COSTCO_SAMEDAY_BASE_URL", srv.URL)
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "")
	return &hits
}

func withStdin(t *testing.T, content string) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	orig := os.Stdin
	os.Stdin = f
	t.Cleanup(func() { os.Stdin = orig; _ = f.Close() })
}

// runCLI builds a root with the persistent --yes / --dry-run flags and the
// given subcommands, then executes args.
func runCLI(t *testing.T, args []string, build func(*rootFlags) []*cobra.Command) (string, error) {
	t.Helper()
	flags := &rootFlags{noCache: true}
	root := &cobra.Command{Use: "root", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().BoolVar(&flags.yes, "yes", false, "")
	root.PersistentFlags().BoolVar(&flags.dryRun, "dry-run", false, "")
	for _, c := range build(flags) {
		root.AddCommand(c)
	}
	root.SetArgs(args)
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	err := root.Execute()
	return buf.String(), err
}

func checkoutCmds(flags *rootFlags) []*cobra.Command {
	checkout := &cobra.Command{Use: "checkout"}
	checkout.AddCommand(newCheckoutUpdatecheckoutCmd(flags))
	products := &cobra.Command{Use: "products"}
	products.AddCommand(newProductsItemsCmd(flags))
	return []*cobra.Command{checkout, products}
}

func orderCmds(flags *rootFlags) []*cobra.Command {
	return []*cobra.Command{newOrderNarrativeCmd(flags)}
}

const finalizeOK = `{"data":{"finalizeCheckout":{"legacyOrderId":"123","orderDeliveryId":"456"}}}`

func TestUpdatecheckoutCannotFinalizeCheckout(t *testing.T) {
	stdinFinalize := `{"operationName":"FinalizeCheckout","variables":{"checkoutSessionId":"sess_test"}}`
	// Same charge, but the stdin body keeps the declared name and smuggles
	// FinalizeCheckout through query text; only the client gate can see it.
	stdinSmuggled := `{"operationName":"UpdateCheckout","query":"mutation { finalizeCheckout(checkoutSessionId:\"s\") { legacyOrderId } }"}`
	cases := []struct {
		name  string
		args  []string
		stdin string
	}{
		{"flag, no consent", []string{"checkout", "updatecheckout", "--operation-name", "FinalizeCheckout"}, ""},
		{"flag, --yes only", []string{"checkout", "updatecheckout", "--operation-name", "FinalizeCheckout", "--yes"}, ""},
		{"stdin, no consent", []string{"checkout", "updatecheckout", "--stdin"}, stdinFinalize},
		{"stdin, --yes only", []string{"checkout", "updatecheckout", "--stdin", "--yes"}, stdinFinalize},
		{"stdin smuggled query, --yes", []string{"checkout", "updatecheckout", "--stdin", "--yes"}, stdinSmuggled},
		{"other command --operation-name override", []string{"products", "items", "--operation-name", "FinalizeCheckout", "--yes"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hits := mockSameDay(t, 200, finalizeOK)
			if tc.stdin != "" {
				withStdin(t, tc.stdin)
			}
			out, err := runCLI(t, tc.args, checkoutCmds)
			if err == nil {
				t.Fatalf("expected refusal; out=%s", out)
			}
			t.Logf("refusal: %v", err)
			if !strings.Contains(err.Error(), "FinalizeCheckout") && !strings.Contains(out, "FinalizeCheckout") {
				t.Fatalf("refusal should name FinalizeCheckout: err=%v out=%s", err, out)
			}
			if n := atomic.LoadInt32(hits); n != 0 {
				t.Fatalf("FinalizeCheckout reached the server (%d hits) without order place consent", n)
			}
		})
	}
}

func TestOrderPlaceRequiresBothFlagsAtTransport(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		wantSent  bool
		wantError bool
	}{
		{"no flags", []string{"order", "place", "--checkout-session-id", "s"}, false, true},
		{"--yes only", []string{"order", "place", "--checkout-session-id", "s", "--yes"}, false, true},
		{"--confirm-charge only", []string{"order", "place", "--checkout-session-id", "s", "--confirm-charge"}, false, true},
		{"both flags", []string{"order", "place", "--checkout-session-id", "s", "--yes", "--confirm-charge"}, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hits := mockSameDay(t, 200, finalizeOK)
			out, err := runCLI(t, tc.args, orderCmds)
			if tc.wantError != (err != nil) {
				t.Fatalf("err=%v wantError=%v out=%s", err, tc.wantError, out)
			}
			if sent := atomic.LoadInt32(hits) > 0; sent != tc.wantSent {
				t.Fatalf("sent=%v want %v out=%s", sent, tc.wantSent, out)
			}
			if tc.wantSent && !strings.Contains(out, `"charged": true`) {
				t.Fatalf("expected charged true on mock success: %s", out)
			}
		})
	}
}

func TestOrderPlaceChargeOutcome(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		reply       string
		verify      bool
		wantCharged bool
	}{
		{"real success", 200, finalizeOK, false, true},
		{"200 with GraphQL errors", 200, `{"errors":[{"message":"payment declined"}],"data":{"finalizeCheckout":null}}`, false, false},
		{"synthetic verify reply", 200, finalizeOK, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hits := mockSameDay(t, tc.status, tc.reply)
			if tc.verify {
				t.Setenv("PRINTING_PRESS_VERIFY", "1")
			}
			out, err := runCLI(t, []string{"order", "place", "--checkout-session-id", "s", "--yes", "--confirm-charge"}, orderCmds)
			charged := strings.Contains(out, `"charged": true`)
			if charged != tc.wantCharged {
				t.Fatalf("charged=%v want %v out=%s", charged, tc.wantCharged, out)
			}
			if tc.wantCharged && err != nil {
				t.Fatalf("success should not error: %v", err)
			}
			if !tc.wantCharged {
				if err == nil {
					t.Fatalf("not-charged outcome must exit non-zero; out=%s", out)
				}
				if !strings.Contains(out, `"charged": false`) {
					t.Fatalf("expected charged false: %s", out)
				}
			}
			if tc.verify && atomic.LoadInt32(hits) != 0 {
				t.Fatalf("verify mode must not reach the server")
			}
		})
	}
}

func TestOrderCancelOutcome(t *testing.T) {
	cases := []struct {
		name         string
		status       int
		reply        string
		verify       bool
		wantCanceled bool
	}{
		{"real success", 200, `{}`, false, true},
		{"200 with REST error body", 200, `{"error":"order cannot be canceled"}`, false, false},
		{"synthetic verify reply", 200, `{}`, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hits := mockSameDay(t, tc.status, tc.reply)
			if tc.verify {
				t.Setenv("PRINTING_PRESS_VERIFY", "1")
			}
			out, err := runCLI(t, []string{"order", "cancel", "--order-id", "123", "--yes"}, orderCmds)
			canceled := strings.Contains(out, `"canceled": true`)
			if canceled != tc.wantCanceled {
				t.Fatalf("canceled=%v want %v out=%s", canceled, tc.wantCanceled, out)
			}
			if tc.wantCanceled && err != nil {
				t.Fatalf("success should not error: %v", err)
			}
			if !tc.wantCanceled && err == nil {
				t.Fatalf("unconfirmed cancel must exit non-zero; out=%s", out)
			}
			if tc.verify && atomic.LoadInt32(hits) != 0 {
				t.Fatalf("verify mode must not reach the server")
			}
		})
	}
}
