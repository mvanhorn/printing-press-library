// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/store"
	"github.com/spf13/cobra"
)

func TestGreptileG3ReceiptCapturesDeliveryWriter(t *testing.T) {
	for _, order := range []string{"delivery first", "receipt first"} {
		t.Run(order, func(t *testing.T) {
			for _, name := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME"} {
				t.Setenv(name, t.TempDir())
			}
			receiptPath := filepath.Join(t.TempDir(), "receipt.json")
			deliveredPath := filepath.Join(t.TempDir(), "delivered.json")
			flags := &rootFlags{platformSession: platformCommandTestSession(t), platformMetadataEmitted: true}
			root := &cobra.Command{Use: "klaviyo-pp-cli"}
			var stdout, stderr bytes.Buffer
			root.SetOut(&stdout)
			root.SetErr(&stderr)
			root.PersistentFlags().StringVar(&flags.deliverSpec, "deliver", "", "delivery")
			root.PersistentFlags().StringVar(&flags.receiptFile, "receipt-file", "", "receipt")
			root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
				if order == "delivery first" {
					if err := configureDelivery(cmd, flags); err != nil {
						return err
					}
					return initializePlatformReceipt(cmd, flags)
				}
				if err := initializePlatformReceipt(cmd, flags); err != nil {
					return err
				}
				return configureDelivery(cmd, flags)
			}
			payload := "{\"result\":true}\n"
			root.AddCommand(&cobra.Command{Use: "report", RunE: func(cmd *cobra.Command, _ []string) error {
				_, err := io.WriteString(cmd.OutOrStdout(), payload)
				return err
			}})
			t.Cleanup(func() {
				if flags.receiptWriter != nil {
					_ = flags.receiptWriter.CloseProcessOutputs()
				}
			})
			root.SetArgs([]string{"report", "--deliver", "file:" + deliveredPath, "--receipt-file", receiptPath})
			err := root.Execute()
			if err == nil {
				err = finalizeDelivery(flags)
			}
			finalizePlatformInvocation(flags, &err)
			if err != nil {
				t.Fatal(err)
			}
			delivered, err := os.ReadFile(deliveredPath)
			if err != nil || string(delivered) != payload || stdout.String() != payload {
				t.Fatalf("delivered=%q stdout=%q, error=%v", delivered, stdout.String(), err)
			}
			data, err := os.ReadFile(receiptPath)
			if err != nil {
				t.Fatal(err)
			}
			var receipt platform.Receipt
			if err := json.Unmarshal(data, &receipt); err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(delivered)
			outputs := map[string]platform.OutputDigest{}
			for _, output := range receipt.Outputs {
				outputs[output.Name] = output
			}
			for _, name := range []string{"stdout", "delivered_file"} {
				output := outputs[name]
				if output.Bytes != int64(len(delivered)) || output.SHA256 != hex.EncodeToString(digest[:]) {
					t.Errorf("%s receipt digest=%+v, want delivered bytes and SHA256", name, output)
				}
			}
		})
	}
}

func TestGreptileG3FirstDefaultSyncIncludesCouponCodes(t *testing.T) {
	for _, concurrency := range []int{1, 4} {
		t.Run(fmt.Sprint(concurrency), func(t *testing.T) {
			profile := greptileG1Profile(t)
			var couponsComplete atomic.Bool
			var independentOnce sync.Once
			independent := make(chan struct{})
			var mu sync.Mutex
			filters := map[string]int{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil || len(body) != 0 || r.Method != http.MethodGet {
					t.Errorf("request=%s %s body=%q error=%v", r.Method, r.URL, body, err)
				}
				if r.Header.Get("Authorization") != "Klaviyo-API-Key test-key" || r.Header.Get("revision") != "2026-04-15" {
					t.Error("sync changed authorization or revision headers")
				}
				w.Header().Set("Content-Type", "application/vnd.api+json")
				switch r.URL.Path {
				case "/api/accounts":
					fmt.Fprint(w, `{"data":[{"id":"acct-a","attributes":{"contact_information":{"organization_name":"Acme"}}}],"links":{"next":null}}`)
				case "/api/custom-metrics":
					independentOnce.Do(func() { close(independent) })
					fmt.Fprint(w, `{"data":[],"links":{"next":null}}`)
				case "/api/coupons":
					if concurrency > 1 {
						select {
						case <-independent:
						case <-time.After(3 * time.Second):
							t.Error("independent resources did not run concurrently")
						}
					}
					if got := r.URL.Query().Get("page[size]"); got != "100" {
						t.Errorf("coupon page size=%q", got)
					}
					if r.URL.Query().Get("page[cursor]") != "" {
						t.Errorf("unexpected coupons path: %s", r.URL.RequestURI())
					}
					couponsComplete.Store(true)
					fmt.Fprint(w, `{"data":[{"type":"coupon","id":"coupon-a"},{"type":"coupon","id":"coupon-b"}],"links":{"next":null}}`)
				case "/api/coupon-codes":
					if !couponsComplete.Load() {
						t.Error("coupon codes requested before coupons completed")
					}
					filter := r.URL.Query().Get("filter")
					mu.Lock()
					filters[filter]++
					mu.Unlock()
					id := "code-a"
					if filter == "equals(coupon.id,'coupon-b')" {
						id = "code-b"
					} else if filter != "equals(coupon.id,'coupon-a')" {
						t.Errorf("unexpected coupon code filter: %q", filter)
					}
					if r.URL.Query().Get("page[size]") != "100" || len(r.URL.Query()) != 2 {
						t.Errorf("coupon codes request=%s", r.URL.RequestURI())
					}
					fmt.Fprintf(w, `{"data":[{"type":"coupon-code","id":%q}],"links":{"next":null}}`, id)
				default:
					if !strings.HasPrefix(r.URL.Path, "/api/") {
						t.Errorf("unexpected endpoint: %s", r.URL.RequestURI())
					}
					fmt.Fprint(w, `{"data":[],"links":{"next":null}}`)
				}
			}))
			defer server.Close()
			previous := registeredPlatformSource
			registration := *previous
			registration.Adapter = klaviyoIdentityAdapter{baseURL: server.URL, httpClient: server.Client()}
			registration.BindClient = func(c *client.Client, session *platform.Session) error {
				if err := previous.BindClient(c, session); err != nil {
					return err
				}
				c.BaseURL = server.URL
				for _, class := range []string{"*", "GET /api/accounts", "GET /api/profiles"} {
					if err := c.SetPlatformEndpointBudget(platform.EndpointBudget{Class: class, Steady: 10000, Interval: time.Second, Burst: 100, MaxAttempts: 1, RetryBudget: time.Second}); err != nil {
						return err
					}
				}
				return nil
			}
			registeredPlatformSource = &registration
			t.Cleanup(func() { registeredPlatformSource = previous })
			flags := &rootFlags{}
			root := newRootCmd(flags)
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)
			root.SetArgs([]string{"sync", "--client-profile", profile.Name, "--concurrency", fmt.Sprint(concurrency), "--strict"})
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := root.ExecuteContext(ctx); err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			defer mu.Unlock()
			want := map[string]int{"equals(coupon.id,'coupon-a')": 1, "equals(coupon.id,'coupon-b')": 1}
			if !reflect.DeepEqual(filters, want) {
				t.Fatalf("coupon code requests=%v, want %v", filters, want)
			}
			db, err := store.OpenVerifiedWithContext(ctx, flags.platformSession.Paths.DataFile, flags.platformSession)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			ids, err := db.ListIDs("coupon-codes")
			if err != nil || len(ids) != 2 {
				t.Fatalf("stored coupon codes=%v, error=%v", ids, err)
			}
		})
	}
}

type greptileG3ConfigEntry struct {
	mode fs.FileMode
	hash [sha256.Size]byte
}

func greptileG3ConfigSnapshot(t *testing.T, directory string) map[string]greptileG3ConfigEntry {
	t.Helper()
	snapshot := map[string]greptileG3ConfigEntry{}
	err := filepath.WalkDir(directory, func(filename string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		var data []byte
		if !entry.IsDir() {
			data, err = os.ReadFile(filename)
			if err != nil {
				return err
			}
		}
		snapshot[filename] = greptileG3ConfigEntry{info.Mode(), sha256.Sum256(data)}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestGreptileG3AuthAccountsDryRunPreservesConfig(t *testing.T) {
	for _, test := range []struct {
		args            []string
		action          string
		selectAccount   bool
		replaceAccount  bool
		clearsSelection bool
	}{
		{[]string{"add", "new", "preview-key"}, "add", true, false, false},
		{[]string{"add", "new", "preview-key", "--use=false"}, "add", false, false, false},
		{[]string{"add", "other", "preview-key"}, "add", true, true, false},
		{[]string{"add", "other", "preview-key", "--use=false"}, "add", false, true, false},
		{[]string{"use", "other"}, "use", true, false, false},
		{[]string{"delete", "selected", "--yes"}, "delete", false, false, true},
		{[]string{"delete", "selected"}, "delete", false, false, true},
		{[]string{"delete", "other"}, "delete", false, false, false},
	} {
		for _, asJSON := range []bool{false, true} {
			t.Run(strings.Join(test.args, " ")+fmt.Sprint(asJSON), func(t *testing.T) {
				directory := t.TempDir()
				t.Setenv("XDG_CONFIG_HOME", directory)
				t.Setenv(mcpBoundProfileEnv, "")
				t.Setenv("PRINTING_PRESS_CLIENT_PROFILE", "")
				for _, name := range []string{"selected", "other"} {
					filename, err := klaviyoAccountConfigPath(name)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filename, []byte("api_key = 'fixture-key'\n"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				if err := writeCurrentKlaviyoAccountName("selected"); err != nil {
					t.Fatal(err)
				}
				before := greptileG3ConfigSnapshot(t, directory)
				root := RootCmd()
				var stdout, stderr bytes.Buffer
				root.SetOut(&stdout)
				root.SetErr(&stderr)
				args := append([]string{"auth", "accounts"}, test.args...)
				args = append(args, "--dry-run")
				if asJSON {
					args = append(args, "--json")
				}
				root.SetArgs(args)
				if err := root.Execute(); err != nil {
					t.Fatal(err)
				}
				if after := greptileG3ConfigSnapshot(t, directory); !reflect.DeepEqual(after, before) {
					t.Fatal("dry run changed config contents, permissions, or directory entries")
				}
				if asJSON {
					var preview map[string]any
					if err := json.Unmarshal(stdout.Bytes(), &preview); err != nil {
						t.Fatal(err)
					}
					want := map[string]any{"dry_run": true, "action": test.action, "account": test.args[1]}
					if test.action == "delete" {
						want["clears_selection"] = test.clearsSelection
					} else {
						want["select_account"] = test.selectAccount
						if test.action == "add" {
							want["replace_account"] = test.replaceAccount
						}
					}
					if !reflect.DeepEqual(preview, want) {
						t.Fatalf("preview=%v, want %v", preview, want)
					}
				} else {
					detail := fmt.Sprintf("select account: %t", test.selectAccount)
					if test.action == "add" {
						detail = fmt.Sprintf("replace account: %t, select account: %t", test.replaceAccount, test.selectAccount)
					} else if test.action == "delete" {
						detail = fmt.Sprintf("clear selection: %t", test.clearsSelection)
					}
					want := fmt.Sprintf("dry run: would %s account %q (%s)\n", test.action, test.args[1], detail)
					if stdout.String() != want {
						t.Fatalf("preview=%q, want %q", stdout.String(), want)
					}
				}
				if strings.Contains(stdout.String()+stderr.String(), "preview-key") {
					t.Fatal("preview leaked a credential")
				}
			})
		}
	}
}

func TestGreptileG3AuthAccountsDryRunCreatesNoDirectories(t *testing.T) {
	for _, action := range []string{"add", "use", "delete"} {
		for _, asJSON := range []bool{false, true} {
			t.Run(action+fmt.Sprint(asJSON), func(t *testing.T) {
				directory := t.TempDir()
				t.Setenv("XDG_CONFIG_HOME", filepath.Join(directory, "missing"))
				t.Setenv(mcpBoundProfileEnv, "")
				t.Setenv("PRINTING_PRESS_CLIENT_PROFILE", "")
				before := greptileG3ConfigSnapshot(t, directory)
				root := RootCmd()
				var stdout, stderr bytes.Buffer
				root.SetOut(&stdout)
				root.SetErr(&stderr)
				args := []string{"auth", "accounts", action, "new", "--dry-run"}
				if action == "add" {
					args = append(args, "preview-key")
				}
				if asJSON {
					args = append(args, "--json")
				}
				root.SetArgs(args)
				err := root.Execute()
				if action == "add" {
					if err != nil {
						t.Fatal(err)
					}
				} else if err == nil || err.Error() != `account "new" not found` {
					t.Fatalf("error=%v, want account %q not found", err, "new")
				} else if stdout.Len() != 0 {
					t.Fatalf("missing account produced a preview: %q", stdout.String())
				}
				if !reflect.DeepEqual(greptileG3ConfigSnapshot(t, directory), before) {
					t.Fatal("dry run created config directories")
				}
				if strings.Contains(stdout.String()+stderr.String(), "preview-key") {
					t.Fatal("preview leaked a credential")
				}
			})
		}
	}
}
