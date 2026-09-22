// Copyright 2026 educrvz and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelCheckoutPreviewHelpWires smoke-tests that the checkout preview command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelCheckoutPreviewHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"checkout", "preview", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("checkout preview --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "preview"} {
		if !strings.Contains(help, want) {
			t.Fatalf("checkout preview --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestUnicaIsNeitherSubscriptionNorUltraFast is the regression guard for the
// bug this patch fixes: `checkout preview` treated "not a subscription store"
// as "ultra fast", so `--store unica` reported is_ultra_fast=true even though
// GET /features/stores says is_ultra_fast_delivery=false for that storefront.
//
// PATCH: store-cluster-truth.
func TestUnicaIsNeitherSubscriptionNorUltraFast(t *testing.T) {
	if isSubscriptionStore("unica") {
		t.Error("unica has no recurring basket")
	}
	if isUltraFastStore("unica") {
		t.Error("unica is not ultra-fast — only now and now-bebidas are")
	}
	if isUltraFastStore("pontual") {
		t.Error("the `pontual` alias must resolve to unica, which is not ultra-fast")
	}
	for _, name := range []string{"now", "now-bebidas", "6", "8"} {
		if !isUltraFastStore(name) {
			t.Errorf("%s should be ultra-fast", name)
		}
		if isSubscriptionStore(name) {
			t.Errorf("%s has no recurring basket", name)
		}
	}
	for _, name := range []string{"programada", "mensal", "1", "fresh", "2", "pet", "5"} {
		if !isSubscriptionStore(name) {
			t.Errorf("%s should be a subscription store", name)
		}
		if isUltraFastStore(name) {
			t.Errorf("%s is not ultra-fast", name)
		}
	}
}

// TestResolveSubdomainCoversEveryStorefront pins the subdomain mapping after it
// was folded into the single store table.
//
// PATCH: store-cluster-truth.
func TestResolveSubdomainCoversEveryStorefront(t *testing.T) {
	cases := map[string]string{
		"programada": "programada", "mensal": "programada", "1": "programada",
		"fresh": "fresh", "2": "fresh",
		"unica": "unica", "pontual": "unica", "3": "unica",
		"pet": "pet", "5": "pet",
		"now": "now", "6": "now",
		"now-bebidas": "now-bebidas", "nowbebidas": "now-bebidas", "8": "now-bebidas",
		"": "programada", "bogus": "programada",
	}
	for in, want := range cases {
		if got := resolveSubdomain(in); got != want {
			t.Errorf("resolveSubdomain(%q) = %q, want %q", in, got, want)
		}
	}
}
