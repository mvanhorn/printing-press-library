// Copyright 2026 Giuliano Giacaglia and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
)

func TestComputeDrift_AddedRemovedModified(t *testing.T) {
	declared := map[string]driftEntity{
		"service/payments-api": {Kind: "service", Name: "payments-api", Plan: "standard", Region: "oregon"},
		"service/new-worker":   {Kind: "service", Name: "new-worker", Plan: "starter"},
		"env-group/shared": {
			Kind:    "env-group",
			Name:    "shared",
			EnvKeys: map[string]bool{"STRIPE_KEY": true, "DEBUG_TOKEN": true},
		},
	}
	live := map[string]driftEntity{
		"service/payments-api": {Kind: "service", Name: "payments-api", Plan: "starter", Region: "oregon"},
		"service/old-bg":       {Kind: "service", Name: "old-bg", Plan: "starter"},
		"env-group/shared": {
			Kind:    "env-group",
			Name:    "shared",
			EnvKeys: map[string]bool{"STRIPE_KEY": true, "OLD_TOKEN": true},
		},
	}

	rep := computeDrift(declared, live)
	if len(rep.Added) != 1 || rep.Added[0].Name != "new-worker" {
		t.Errorf("added: expected new-worker, got %+v", rep.Added)
	}
	if len(rep.Removed) != 1 || rep.Removed[0].Name != "old-bg" {
		t.Errorf("removed: expected old-bg, got %+v", rep.Removed)
	}
	if len(rep.Modified) != 2 {
		t.Fatalf("expected 2 modified, got %d (%+v)", len(rep.Modified), rep.Modified)
	}
	for _, m := range rep.Modified {
		if m.Name == "payments-api" {
			if len(m.Changes) != 1 || m.Changes[0].Field != "plan" {
				t.Errorf("payments-api: expected single plan change, got %+v", m.Changes)
			}
			if m.Changes[0].From != "starter" || m.Changes[0].To != "standard" {
				t.Errorf("payments-api: plan from/to wrong: %+v", m.Changes[0])
			}
		}
		if m.Name == "shared" {
			found := false
			for _, c := range m.Changes {
				if c.Field == "env-keys" {
					found = true
					if len(c.Added) != 1 || c.Added[0] != "DEBUG_TOKEN" {
						t.Errorf("shared: added env-keys: %+v", c.Added)
					}
					if len(c.Removed) != 1 || c.Removed[0] != "OLD_TOKEN" {
						t.Errorf("shared: removed env-keys: %+v", c.Removed)
					}
				}
			}
			if !found {
				t.Errorf("shared: expected env-keys change")
			}
		}
	}
}

func TestComputeDrift_NoDrift(t *testing.T) {
	declared := map[string]driftEntity{
		"service/x": {Kind: "service", Name: "x", Plan: "starter"},
	}
	live := map[string]driftEntity{
		"service/x": {Kind: "service", Name: "x", Plan: "starter"},
	}
	rep := computeDrift(declared, live)
	if hasDrift(rep) {
		t.Errorf("expected no drift, got %+v", rep)
	}
}

func TestParseBlueprint_Minimal(t *testing.T) {
	doc := []byte(`services:
  - type: web
    name: payments-api
    plan: standard
    region: oregon
    envVars:
      - key: STRIPE_KEY
      - key: DEBUG_TOKEN
  - type: worker
    name: new-worker
    plan: starter
envVarGroups:
  - name: shared
    envVars:
      - key: SHARED_KEY
databases:
  - name: payments-db
    plan: basic-1gb
`)
	bp, err := parseBlueprint(doc)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(bp.Services) != 2 {
		t.Errorf("expected 2 services, got %d", len(bp.Services))
	}
	if bp.Services[0].Name != "payments-api" || bp.Services[0].Plan != "standard" {
		t.Errorf("services[0]: %+v", bp.Services[0])
	}
	if len(bp.Services[0].EnvVars) != 2 {
		t.Errorf("services[0].envVars: got %d, want 2", len(bp.Services[0].EnvVars))
	}
	if len(bp.EnvGroups) != 1 || bp.EnvGroups[0].Name != "shared" {
		t.Errorf("envGroups: %+v", bp.EnvGroups)
	}
	if len(bp.Databases) != 1 || bp.Databases[0].Plan != "basic-1gb" {
		t.Errorf("databases: %+v", bp.Databases)
	}
}
