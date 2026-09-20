// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. The --agent JSON envelope, asserted as ONE invariant over the
// machinery and one over the command tree, rather than a per-command literal.
//
// WHY THIS FILE EXISTS. The --agent envelope is this CLI's machine-facing
// contract, and it had drifted into three incompatible shapes:
//
//   - coverage and licence emitted {meta:{source}, results:{meta, results}} —
//     a SECOND envelope wrapped around one the command had already built. A
//     consumer running `jq '.results[]'` got one opaque object, not rows.
//   - events and gen emitted a BARE top-level array from their catalogue
//     branches, so .meta and .results did not exist at all and the same
//     consumer got nothing.
//   - verify, fleet and conflicts return .results as keyed report sections.
//     That one is deliberate: those commands report named sections, not a row
//     list, and this file does not require them to lie about their shape.
//
// The first two were defects with a single shared cause: a generic wrapper ran
// unconditionally on top of a payload that already carried its own envelope,
// or on a payload that carried none. wrapPlatformStructuredOutput had merged
// in that situation for as long as it has existed; wrapWithProvenance and
// wrapAgentOutput did not. The fix taught the other two wrappers the same
// rule, so the invariant belongs to the machinery and is asserted here once.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// envelopeIsDoubled reports whether an emitted document nests a second
// {meta, results} envelope inside its results — the shape that makes
// `.results[]` iterate nothing.
func envelopeIsDoubled(doc map[string]any) bool {
	inner, ok := doc["results"].(map[string]any)
	if !ok {
		return false
	}
	_, hasMeta := inner["meta"]
	_, hasResults := inner["results"]
	return hasMeta && hasResults
}

// TestOwnedEnvelopeIsMergedNotNested is the machinery-level invariant: every
// wrapper that can be handed a payload which already owns a {meta, results}
// envelope must merge into it, never nest a second one around it.
//
// This is the test that would have caught the original defect. It is written
// against the wrappers rather than against commands so that a NEW command
// inherits it for free, and so it needs no network.
func TestOwnedEnvelopeIsMergedNotNested(t *testing.T) {
	// A command's own envelope: a rich meta no generic wrapper could
	// reconstruct, and a row array a consumer expects to iterate.
	owned := json.RawMessage(`{"meta":{"source":"catalogue","surfaces":104,` +
		`"staleness_note":"age is measured from the catalogue date"},` +
		`"results":[{"id":"a"},{"id":"b"}]}`)

	wrappers := []struct {
		name string
		wrap func(json.RawMessage) (json.RawMessage, error)
	}{
		{"wrapAgentOutput", func(d json.RawMessage) (json.RawMessage, error) {
			return wrapAgentOutput(d, map[string]any{"source": "local"})
		}},
		{"wrapWithProvenance", func(d json.RawMessage) (json.RawMessage, error) {
			return wrapWithProvenance(d, DataProvenance{Source: "local"})
		}},
	}

	for _, w := range wrappers {
		t.Run(w.name, func(t *testing.T) {
			out, err := w.wrap(owned)
			if err != nil {
				t.Fatalf("%s: %v", w.name, err)
			}
			var doc map[string]any
			if err := json.Unmarshal(out, &doc); err != nil {
				t.Fatalf("%s produced unparseable JSON: %v", w.name, err)
			}
			if envelopeIsDoubled(doc) {
				t.Fatalf("%s nested a second envelope around one the payload already owned; "+
					"`.results[]` now iterates one opaque object instead of the rows:\n%s",
					w.name, out)
			}
			rows, ok := doc["results"].([]any)
			if !ok || len(rows) != 2 {
				t.Fatalf("%s: .results = %T (len %d), want the 2 original rows",
					w.name, doc["results"], len(rows))
			}
			meta, ok := doc["meta"].(map[string]any)
			if !ok {
				t.Fatalf("%s dropped meta entirely", w.name)
			}
			// The command's specific claim must survive the generic one.
			if meta["source"] != "catalogue" {
				t.Errorf("%s: meta.source = %v, want the command's measured \"catalogue\" — "+
					"a generic wrapper default must not displace a measured origin",
					w.name, meta["source"])
			}
			if meta["staleness_note"] == nil {
				t.Errorf("%s dropped the command's staleness_note during the merge", w.name)
			}
			if got, want := meta["surfaces"], float64(104); got != want {
				t.Errorf("%s: meta.surfaces = %v, want %v", w.name, got, want)
			}
		})
	}
}

// TestOwnedEnvelopeMergePreservesADisagreeingClaim pins the one case where the
// merge must NOT simply prefer the command: when the wrapper's value is a
// genuine competing observation rather than a default.
//
// DataProvenance.Source is a TRANSPORT claim ("live" = this run touched the
// network) while a command's meta.source is a DATA-ORIGIN claim ("catalogue").
// They are different axes, so collapsing one into the other would assert
// something neither observed. The disagreeing value is kept under
// envelope_meta instead of being dropped.
func TestOwnedEnvelopeMergePreservesADisagreeingClaim(t *testing.T) {
	owned := json.RawMessage(`{"meta":{"source":"catalogue"},"results":[{"id":"a"}]}`)

	out, err := wrapWithProvenance(owned, DataProvenance{Source: "live"})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	meta := doc["meta"].(map[string]any)
	if meta["source"] != "catalogue" {
		t.Errorf("meta.source = %v, want the command's \"catalogue\"", meta["source"])
	}
	displaced, ok := meta["envelope_meta"].(map[string]any)
	if !ok {
		t.Fatalf("a disagreeing transport claim was DROPPED; meta = %v", meta)
	}
	if displaced["source"] != "live" {
		t.Errorf("envelope_meta.source = %v, want the preserved \"live\"", displaced["source"])
	}

	// And when the two agree there must be no noise: envelope_meta is only
	// for a real disagreement, not for every merge.
	out, err = wrapWithProvenance(owned, DataProvenance{Source: "catalogue"})
	if err != nil {
		t.Fatal(err)
	}
	doc = map[string]any{}
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	if _, noisy := doc["meta"].(map[string]any)["envelope_meta"]; noisy {
		t.Error("envelope_meta appeared even though the two claims agreed")
	}
}

// TestNoRequestCommandsEmitTheStandardEnvelope is the command-level invariant
// over every path that answers from the in-code catalogues. These are exactly
// the paths that carried the defect — a catalogue branch has no upstream
// document to wrap, which is how events and gen came to emit a bare array.
//
// The base URL points at a server that FAILS the test if it is hit, so this
// also re-asserts the no-request promise these commands make.
func TestNoRequestCommandsEmitTheStandardEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("a no-request command fetched %s", r.URL.Path)
		w.WriteHeader(http.StatusTeapot)
	}))
	defer srv.Close()

	// report: results is a keyed object of named sections rather than a row
	// array. That shape is deliberate for the commands that report sections
	// (see README, Agent Usage), so it is declared here per command rather
	// than tolerated everywhere — a row command that silently became keyed
	// would otherwise pass.
	cases := []struct {
		name   string
		args   []string
		report bool
	}{
		{"coverage", []string{"coverage", "--agent"}, false},
		{"events catalogue", []string{"events", "--agent"}, false},
		{"gen catalogue", []string{"gen", "--agent"}, false},
		{"sources", []string{"sources", "--agent"}, false},
		{"verify manifest", []string{"verify", "--manifest", "--agent"}, true},
		{"disco catalogue", []string{"disco", "--agent"}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("NEPRA_BASE_URL", srv.URL)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
			t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
			t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
			t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
			t.Setenv("NEPRA_CONFIG", filepath.Join(home, "config", "config.toml"))

			root := RootCmd()
			var stdout, stderr bytes.Buffer
			root.SetOut(&stdout)
			root.SetErr(&stderr)
			root.SetArgs(append(tc.args, "--no-learn"))
			if err := root.Execute(); err != nil {
				t.Fatalf("%v: %v\n%s", tc.args, err, stderr.String())
			}

			raw := stdout.Bytes()
			// A bare top-level array is the second defect shape: .meta and
			// .results simply do not exist, so a consumer sees nothing.
			var asArray []any
			if json.Unmarshal(raw, &asArray) == nil {
				t.Fatalf("%v emitted a BARE top-level array with no envelope, so .meta and "+
					".results do not exist and `jq '.results[]'` yields nothing", tc.args)
			}
			var doc map[string]any
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatalf("%v emitted unparseable JSON: %v\n%s", tc.args, err, truncate(string(raw), 300))
			}
			if _, ok := doc["meta"]; !ok {
				t.Errorf("%v emitted no meta; machine consumers lose provenance", tc.args)
			}
			if _, ok := doc["results"]; !ok {
				t.Errorf("%v emitted no results key", tc.args)
			}
			if envelopeIsDoubled(doc) {
				t.Errorf("%v nested a second envelope: rows sit at .results.results[] so "+
					"`.results[]` iterates nothing", tc.args)
			}
			if _, ok := doc["results"].(map[string]any); ok && !tc.report {
				t.Errorf("%v returned results as a keyed object, but it is documented as a "+
					"row-returning command; `.results[]` iterates nothing", tc.args)
			}
		})
	}
}
