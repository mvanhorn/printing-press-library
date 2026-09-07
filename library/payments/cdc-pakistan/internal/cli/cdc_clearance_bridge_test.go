// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/cdc-pakistan/internal/client"
	"github.com/mvanhorn/printing-press-library/library/payments/cdc-pakistan/internal/config"
)

// The promoted downloads/statistics/assets commands build the GENERATED
// client, which reads credentials.toml and the cookie jar -- not
// cdc-clearance.json, which is what `auth clearance set` writes. Nothing
// joined the two, so a stored clearance authenticated nothing and those three
// commands sent no Cookie header at all. applyClearanceToClient is the bridge.
// These tests pin its three contracts: it applies the pair, it never fails,
// and it never touches dry-run.

func writeClearance(t *testing.T, dir, cookie, ua string) *rootFlags {
	t.Helper()
	b, err := json.Marshal(Clearance{Cookie: cookie, UserAgent: ua, MintedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cdc-clearance.json"), b, 0o600); err != nil {
		t.Fatal(err)
	}
	return &rootFlags{homePath: dir}
}

func newBridgeTestClient(t *testing.T) *client.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &client.Client{
		BaseURL:    "https://www.cdcpakistan.com",
		Config:     &config.Config{},
		HTTPClient: &http.Client{Jar: jar},
	}
}

func cookieNames(t *testing.T, c *client.Client) map[string]string {
	t.Helper()
	u, err := url.Parse("https://www.cdcpakistan.com/")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, ck := range c.HTTPClient.Jar.Cookies(u) {
		out[ck.Name] = ck.Value
	}
	return out
}

func TestApplyClearanceAppliesCookieAndUserAgentTogether(t *testing.T) {
	dir := t.TempDir()
	prev := activeClearanceFlags
	t.Cleanup(func() { activeClearanceFlags = prev })
	activeClearanceFlags = writeClearance(t, dir, "test-clearance-value", "TestUA/1.0")

	c := newBridgeTestClient(t)
	if err := applyClearanceToClient(c); err != nil {
		t.Fatalf("applyClearanceToClient: %v", err)
	}

	if got := c.Config.Headers["User-Agent"]; got != "TestUA/1.0" {
		t.Errorf("User-Agent = %q, want the minting UA. cf_clearance is UA-bound, so the cookie without its UA is useless", got)
	}
	got := cookieNames(t, c)
	if got["cf_clearance"] != "test-clearance-value" {
		t.Errorf("cf_clearance cookie = %q, want it seeded on the jar; got jar %v", got["cf_clearance"], got)
	}
}

// Apply-only: a missing clearance is a normal state. Returning an error would
// break doctor, api, import and every MCP tool, all of which build a client
// whether or not a clearance exists.
func TestApplyClearanceNeverFails(t *testing.T) {
	prev := activeClearanceFlags
	t.Cleanup(func() { activeClearanceFlags = prev })

	// No clearance file at all.
	activeClearanceFlags = &rootFlags{homePath: t.TempDir()}
	c := newBridgeTestClient(t)
	if err := applyClearanceToClient(c); err != nil {
		t.Errorf("missing clearance must not error, got %v", err)
	}
	if len(cookieNames(t, c)) != 0 {
		t.Error("no cookie should be seeded when none is configured")
	}

	// Corrupt clearance file.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cdc-clearance.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	activeClearanceFlags = &rootFlags{homePath: dir}
	if err := applyClearanceToClient(newBridgeTestClient(t)); err != nil {
		t.Errorf("corrupt clearance must not error, got %v", err)
	}

	// A cookie with no User-Agent must be refused as a pair, not half-applied.
	dir2 := t.TempDir()
	b, _ := json.Marshal(Clearance{Cookie: "orphan-cookie", MintedAt: time.Now()})
	if err := os.WriteFile(filepath.Join(dir2, "cdc-clearance.json"), b, 0o600); err != nil {
		t.Fatal(err)
	}
	activeClearanceFlags = &rootFlags{homePath: dir2}
	c3 := newBridgeTestClient(t)
	if err := applyClearanceToClient(c3); err != nil {
		t.Errorf("UA-less clearance must not error, got %v", err)
	}
	if v := cookieNames(t, c3)["cf_clearance"]; v != "" {
		t.Errorf("a cookie without its minting UA must NOT be sent; got %q", v)
	}

	// A nil client must be tolerated.
	if err := applyClearanceToClient(nil); err != nil {
		t.Errorf("nil client must not error, got %v", err)
	}
}

// --dry-run must never need a credential to print a request. root.go sets
// DryRun before it applies the hooks, so this branch is reachable.
func TestApplyClearanceSkipsDryRun(t *testing.T) {
	dir := t.TempDir()
	prev := activeClearanceFlags
	t.Cleanup(func() { activeClearanceFlags = prev })
	activeClearanceFlags = writeClearance(t, dir, "test-clearance-value", "TestUA/1.0")

	c := newBridgeTestClient(t)
	c.DryRun = true
	if err := applyClearanceToClient(c); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Config.Headers["User-Agent"]; ok {
		t.Error("dry-run must not have the clearance UA applied")
	}
	if len(cookieNames(t, c)) != 0 {
		t.Error("dry-run must not seed a cookie")
	}
}

// The bridge must not persist anything: the `auth clearance set` command
// declares pp:happy-args carrying --cookie=example-clearance-value, which the
// verify and shipcheck harnesses execute. A write-through would install that
// placeholder permanently.
func TestApplyClearanceDoesNotPersist(t *testing.T) {
	dir := t.TempDir()
	prev := activeClearanceFlags
	t.Cleanup(func() { activeClearanceFlags = prev })
	activeClearanceFlags = writeClearance(t, dir, "example-clearance-value", "TestUA/1.0")

	if err := applyClearanceToClient(newBridgeTestClient(t)); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"cookies.json", "credentials.toml"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Errorf("%s was written; the bridge must be in-memory only", name)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "cdc-clearance.json" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("bridge created files on disk: %v", names)
	}
}
