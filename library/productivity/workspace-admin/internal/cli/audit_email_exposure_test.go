// Copyright 2026 RyanGravetteIDLA and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestNovelAuditEmailExposureHelpWires smoke-tests that the audit email-exposure command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelAuditEmailExposureHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"audit", "email-exposure", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("audit email-exposure --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "email-exposure"} {
		if !strings.Contains(help, want) {
			t.Fatalf("audit email-exposure --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestIsExternalAddr(t *testing.T) {
	internal := map[string]bool{"example.com": true}
	cases := []struct {
		name     string
		addr     string
		internal map[string]bool
		want     bool
	}{
		{"internal domain not external", "user@example.com", internal, false},
		{"external domain", "attacker@gmail.com", internal, true},
		{"no @ not external", "notanemail", internal, false},
		{"empty not external", "", internal, false},
		{"no internal set treats resolvable as external", "user@example.com", map[string]bool{}, true},
		{"case-insensitive internal match", "User@EXAMPLE.com", internal, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isExternalAddr(tc.addr, tc.internal); got != tc.want {
				t.Errorf("isExternalAddr(%q) = %v, want %v", tc.addr, got, tc.want)
			}
		})
	}
}

func TestContainsLabel(t *testing.T) {
	labels := []string{"INBOX", "IMPORTANT"}
	if !containsLabel(labels, "INBOX") {
		t.Error("containsLabel should find INBOX")
	}
	if !containsLabel(labels, "inbox") {
		t.Error("containsLabel should be case-insensitive")
	}
	if containsLabel(labels, "TRASH") {
		t.Error("containsLabel should not find TRASH")
	}
	if containsLabel(nil, "TRASH") {
		t.Error("containsLabel on nil should be false")
	}
}

func TestScanMailboxSettingsClassifies(t *testing.T) {
	internal := map[string]bool{"example.com": true}
	// A stub getter returning canned Gmail settings responses per endpoint.
	get := func(path string) ([]byte, error) {
		switch {
		case strings.HasSuffix(path, "/settings/autoForwarding"):
			return []byte(`{"enabled":true,"emailAddress":"exfil@evil.com"}`), nil
		case strings.HasSuffix(path, "/settings/forwardingAddresses"):
			return []byte(`{"forwardingAddresses":[{"forwardingEmail":"safe@example.com","verificationStatus":"accepted"}]}`), nil
		case strings.HasSuffix(path, "/settings/sendAs"):
			return []byte(`{"sendAs":[{"sendAsEmail":"primary@example.com","isPrimary":true},{"sendAsEmail":"alias@evil.com","isPrimary":false}]}`), nil
		case strings.HasSuffix(path, "/settings/delegates"):
			return []byte(`{"delegates":[{"delegateEmail":"assistant@example.com"}]}`), nil
		case strings.HasSuffix(path, "/settings/filters"):
			return []byte(`{"filter":[{"id":"f1","action":{"forward":"leak@evil.com"}},{"id":"f2","action":{"addLabelIds":["TRASH"]}}]}`), nil
		}
		return []byte(`{}`), nil
	}
	view := emailExposureView{}
	scanMailboxSettings("victim@example.com", func(p string) (json.RawMessage, error) {
		b, err := get(p)
		return json.RawMessage(b), err
	}, internal, &view)

	types := map[string]int{}
	for _, f := range view.Findings {
		types[f.Type]++
	}
	// external auto-forward, external send-as, delegate, forwarding filter, trashing filter.
	// The internal forwarding address must NOT be flagged.
	wantTypes := []string{"external_forwarding", "external_send_as", "delegate", "forwarding_filter", "trashing_filter"}
	for _, wt := range wantTypes {
		if types[wt] == 0 {
			t.Errorf("expected a %q finding, got findings: %+v", wt, view.Findings)
		}
	}
	if types["forwarding_address"] != 0 {
		t.Errorf("internal forwarding address should not be flagged; got %+v", view.Findings)
	}
}
