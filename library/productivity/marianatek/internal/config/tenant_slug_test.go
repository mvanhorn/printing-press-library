// Copyright 2026 salmonumbrella. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"path/filepath"
	"testing"
)

func TestTenantConfigPathRejectsTraversal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got, err := TenantConfigPath("kolm-kontrast_1")
	if err != nil {
		t.Fatalf("TenantConfigPath(valid) returned error: %v", err)
	}
	want := filepath.Join(home, ".config", "marianatek-pp-cli", "tenants", "kolm-kontrast_1.toml")
	if got != want {
		t.Fatalf("TenantConfigPath(valid) = %q, want %q", got, want)
	}

	for _, slug := range []string{"", ".", "..", "../evil", "evil/tenant", `evil\tenant`, "evil..tenant", "evil tenant"} {
		t.Run(slug, func(t *testing.T) {
			if _, err := TenantConfigPath(slug); err == nil {
				t.Fatalf("TenantConfigPath(%q) returned nil error", slug)
			}
		})
	}
}
