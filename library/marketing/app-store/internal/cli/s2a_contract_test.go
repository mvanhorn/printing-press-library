// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

// s2aCommands are the slice-2a commands and a real argument list for each.
var s2aCommands = map[string][]string{
	"app":       {"324684580"},
	"privacy":   {"324684580"},
	"versions":  {"324684580"},
	"iap":       {"553834731"},
	"events":    {"553834731"},
	"similar":   {"324684580"},
	"developer": {"324684583"},
	"ratings":   {"324684580"},
	"prices":    {"324684580", "553834731"},
	"compare":   {"324684580", "389801252"},
}

func TestS2ACommandsFollowTheBuildChecklist(t *testing.T) {
	root := RootCmd()
	for name := range s2aCommands {
		cmd, _, err := root.Find([]string{name})
		if err != nil || cmd == nil || cmd.Name() != name {
			t.Fatalf("%s is not registered: %v", name, err)
		}
		if isNovelScaffoldCommand(cmd) {
			t.Fatalf("%s is still a scaffold", name)
		}
		a := cmd.Annotations
		if a["pp:data-source"] != "live" || a["mcp:read-only"] != "true" {
			t.Errorf("%s annotations = %v", name, a)
		}
		if h := a["pp:happy-args"]; h == "" || !strings.Contains(h, "=") {
			t.Errorf("%s pp:happy-args = %q (positional tokens need label=value)", name, h)
		}
		if !strings.HasPrefix(cmd.Example, "  app-store-pp-cli "+name+" ") {
			t.Errorf("%s Example must start with a 2-space indented invocation: %q", name, cmd.Example)
		}
		if cmd.Args != nil {
			t.Errorf("%s must validate arguments in RunE, not cobra Args", name)
		}
		if cmd.Long == "" || len(cmd.Short) > 90 {
			t.Errorf("%s Short %d chars / Long empty", name, len(cmd.Short))
		}
	}
}

func TestS2ADryRunAndHelpSendNothing(t *testing.T) {
	f := newS2AFake(t)
	for name, args := range s2aCommands {
		res := f.mustOK(name, "--dry-run", "--json")
		if v := s2aJSONObject(t, res.stdout); v["dry_run"] != true || v["action"] != name {
			t.Errorf("%s --dry-run --json = %s", name, res.stdout)
		}
		res = f.mustOK(append([]string{name, "--dry-run"}, args...)...)
		if !strings.HasPrefix(res.stdout, "dry-run: would run "+name) {
			t.Errorf("%s --dry-run = %q", name, res.stdout)
		}
		res = f.mustOK(name)
		if !strings.Contains(res.stdout, "Usage:") {
			t.Errorf("bare %s should print help: %q", name, res.stdout)
		}
		if res := f.run(name, "--json"); res.code != 2 {
			t.Errorf("%s --json without input exit = %d, want 2", name, res.code)
		}
	}
	if n := f.count(); n != 0 {
		t.Fatalf("dry-run/help/usage probes sent %d requests: %v", n, f.requestLog())
	}
}

func TestS2AHumanOutputEveryCommand(t *testing.T) {
	f := newS2AFake(t)
	want := map[string]string{
		"app":       "Spotify: Music and Podcasts",
		"privacy":   "privacy labels, storefront us",
		"versions":  "9.1.92",
		"iap":       "Candy Crush Saga (553834731) in-app purchases",
		"events":    "MAJOR UPDATE",
		"similar":   "TREBEL",
		"developer": "Spotify (developer 324684583, storefront us): 4 apps",
		"ratings":   "42,719,156",
		"prices":    "Free (USD)",
		"compare":   "Storefront us",
	}
	for name, args := range s2aCommands {
		res := f.mustOK(append(append([]string{name}, args...), "--human-friendly")...)
		if !strings.Contains(res.stdout, want[name]) {
			t.Errorf("%s human output missing %q:\n%s", name, want[name], res.stdout)
		}
		if strings.HasPrefix(strings.TrimSpace(res.stdout), "{") || strings.HasPrefix(strings.TrimSpace(res.stdout), "[") {
			t.Errorf("%s printed JSON in human mode", name)
		}
	}
	res := f.mustOK("events", "324684580", "--human-friendly")
	if !strings.Contains(res.stdout, "no in-app events are listed") {
		t.Fatalf("human empty result should explain itself: %q", res.stdout)
	}
}

func TestS2AAgentEnvelopeAndSelect(t *testing.T) {
	f := newS2AFake(t)
	res := f.mustOK("ratings", "324684580", "--agent")
	v := s2aJSONObject(t, res.stdout)
	meta, _ := v["meta"].(map[string]any)
	if meta["source"] != "live" {
		t.Fatalf("--agent meta.source = %v, want live", meta["source"])
	}
	results, _ := v["results"].(map[string]any)
	if items, _ := results["items"].([]any); len(items) != 1 {
		t.Fatalf("--agent results = %s", res.stdout)
	}
	res = f.mustOK("app", "324684580", "--json", "--select", "trackName,ratingCount")
	sel := s2aJSONObject(t, res.stdout)
	if len(sel) != 2 || sel["trackName"] == nil || sel["ratingCount"] == nil {
		t.Fatalf("--select = %s", res.stdout)
	}
	res = f.mustOK("versions", "324684580", "--limit", "3", "--csv")
	lines := strings.Split(strings.TrimSpace(res.stdout), "\n")
	if len(lines) != 4 || !strings.Contains(lines[0], "version") {
		t.Fatalf("--csv = %q", res.stdout)
	}
	res = f.mustOK("iap", "553834731", "--csv")
	if lines := strings.Split(strings.TrimSpace(res.stdout), "\n"); len(lines) != 11 || !strings.Contains(lines[0], "price") {
		t.Fatalf("envelope --csv should tabulate items: %q", res.stdout)
	}
}
