// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The From header is chosen by whoever sent the mail, so it must never be able
// to steer writes outside --out.
func TestSenderSlugNeutralizesPathTokens(t *testing.T) {
	cases := []struct {
		name, from string
	}{
		{"traversal in angle brackets", "Evil <../../../../tmp/pwned@example.com>"},
		{"absolute path", "</etc/cron.d/evil@example.com>"},
		{"bare traversal", "../../x@example.net"},
		{"dots only", "<..@..>"},
		{"empty", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := senderSlug(tc.from)
			if strings.ContainsAny(got, `/\`) || strings.Contains(got, "..") {
				t.Fatalf("senderSlug(%q) = %q, still contains path tokens", tc.from, got)
			}
			if got == "" {
				t.Fatalf("senderSlug(%q) returned empty string", tc.from)
			}
			// The slug must stay inside the root when joined.
			root := t.TempDir()
			if err := assertWithin(root, filepath.Join(root, got)); err != nil {
				t.Fatalf("senderSlug(%q) escapes root: %v", tc.from, err)
			}
		})
	}
}

func TestSenderSlugKeepsOrdinaryAddress(t *testing.T) {
	if got := senderSlug("Jane Doe <jane.doe+tag@example.com>"); got != "jane.doe+tag@example.com" {
		t.Fatalf("senderSlug mangled an ordinary address: %q", got)
	}
}

func TestAssertWithin(t *testing.T) {
	root := t.TempDir()
	if err := assertWithin(root, filepath.Join(root, "a", "b.txt")); err != nil {
		t.Fatalf("nested path rejected: %v", err)
	}
	if err := assertWithin(root, root); err != nil {
		t.Fatalf("root itself rejected: %v", err)
	}
	if err := assertWithin(root, filepath.Join(root, "..", "escape.txt")); err == nil {
		t.Fatal("traversal accepted; assertWithin must reject it")
	}
	if err := assertWithin(root, string(os.PathSeparator)+"etc"+string(os.PathSeparator)+"passwd"); err == nil {
		t.Fatal("absolute path outside root accepted")
	}
}
