// Copyright 2026 Ryan Kelley and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestImportValidResources_KnownAccepted(t *testing.T) {
	for _, r := range importValidResourceList {
		if !importValidResources[r] {
			t.Errorf("resource %q is in importValidResourceList but not in importValidResources map", r)
		}
	}
}

func TestImportValidResources_UnknownRejected(t *testing.T) {
	unknown := []string{
		"../etc/passwd",     // path traversal attempt
		"adgroups",          // valid API resource but not in allowlist
		"targetingkeywords", // same
		"",
		"arbitrary-path",
	}
	for _, r := range unknown {
		if importValidResources[r] {
			t.Errorf("resource %q should be rejected but is in the allowlist", r)
		}
	}
}

func TestImportExportAllowlistParity(t *testing.T) {
	// export.go and import.go share the same resource set; keep them in sync.
	exportResources := map[string]bool{
		"budgetorders":         true,
		"campaigns":            true,
		"countries-or-regions": true,
		"creatives":            true,
		"custom-reports":       true,
		"me":                   true,
	}
	for r := range exportResources {
		if !importValidResources[r] {
			t.Errorf("export allows %q but import does not", r)
		}
	}
	for r := range importValidResources {
		if !exportResources[r] {
			t.Errorf("import allows %q but export does not", r)
		}
	}
}
