// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

// playbooks_authored_test.go exercises the U10 hand-authored playbook
// content shipped in internal/cli/playbooks/*.json. The U9 embed-FS
// install path tests inject an fstest.MapFS for scenario coverage;
// this file walks the *real* playbooks.FS and validates every JSON
// payload parses cleanly and produces at least one non-empty query
// family. Catches drift between the hand-authored content and the
// learn.Normalize / learn.QueryFamily contract — a playbook that
// silently fails to derive a family is unreachable at recall time.

package cli

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/cli/playbooks"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn"
)

// TestAuthoredPlaybooks_ParseAndDeriveFamilies enumerates every *.json
// file in playbooks.FS and validates: (a) ParsePlaybook succeeds, (b)
// at least one query_family_example is present, (c) at least one of
// those examples normalizes to a non-empty QueryFamily.
//
// Why this matters: the install path in playbook_init.go skips any
// playbook whose examples all normalize to empty families ("would be
// unreachable at recall time" warning). A misauthored example set
// would silently disappear from the seed step — this test fails loud
// instead.
func TestAuthoredPlaybooks_ParseAndDeriveFamilies(t *testing.T) {
	cfg := learn.DefaultPredictionGoatConfig()
	entries, err := fs.ReadDir(playbooks.FS, ".")
	if err != nil {
		t.Fatalf("read playbooks.FS: %v", err)
	}

	jsonCount := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		jsonCount++
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			data, rerr := fs.ReadFile(playbooks.FS, name)
			if rerr != nil {
				t.Fatalf("read %s: %v", name, rerr)
			}
			pb, perr := learn.ParsePlaybook(data, name)
			if perr != nil {
				t.Fatalf("parse %s: %v", name, perr)
			}
			if len(pb.QueryFamilyExamples) == 0 {
				t.Fatalf("%s: query_family_examples must be non-empty (otherwise unreachable at recall time)", name)
			}
			if len(pb.Steps) == 0 {
				t.Fatalf("%s: steps must be non-empty", name)
			}

			anyFamily := false
			for _, ex := range pb.QueryFamilyExamples {
				normalized := learn.Normalize(ex, cfg)
				fam := learn.QueryFamily(normalized)
				if fam != "" {
					anyFamily = true
				}
			}
			if !anyFamily {
				t.Fatalf("%s: every query_family_example normalized to an empty family; playbook would be unreachable at recall time", name)
			}
		})
	}

	if jsonCount < 2 {
		t.Errorf("expected at least 2 hand-authored playbook JSON files in playbooks.FS; got %d", jsonCount)
	}
}

// TestAuthoredPlaybooks_DistinctFamilies asserts that the shipped
// playbooks address distinct query families — if two playbooks
// landed on the same family key the second would clobber the first
// in the upsert. The U10 plan called out "each playbook's
// query_family_examples should normalize to a different
// QueryFamily" as a hard requirement.
func TestAuthoredPlaybooks_DistinctFamilies(t *testing.T) {
	cfg := learn.DefaultPredictionGoatConfig()
	entries, err := fs.ReadDir(playbooks.FS, ".")
	if err != nil {
		t.Fatalf("read playbooks.FS: %v", err)
	}

	// Map family -> first playbook filename that claimed it.
	familyOwner := map[string]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := e.Name()
		data, rerr := fs.ReadFile(playbooks.FS, name)
		if rerr != nil {
			t.Fatalf("read %s: %v", name, rerr)
		}
		pb, perr := learn.ParsePlaybook(data, name)
		if perr != nil {
			t.Fatalf("parse %s: %v", name, perr)
		}

		// Collect this playbook's family set.
		own := map[string]bool{}
		for _, ex := range pb.QueryFamilyExamples {
			normalized := learn.Normalize(ex, cfg)
			if fam := learn.QueryFamily(normalized); fam != "" {
				own[fam] = true
			}
		}
		for fam := range own {
			if prev, claimed := familyOwner[fam]; claimed && prev != name {
				t.Errorf("family %q is claimed by both %s and %s; one playbook will clobber the other at upsert time", fam, prev, name)
			}
			familyOwner[fam] = name
		}
	}
}

// TestAuthoredPlaybooks_NotesPairExists asserts every shipped JSON
// has a matching _notes.md file. The install path doesn't require it
// (notes_text gets a default empty string) but the agent expects the
// notes — they're the free-text gotchas the recall envelope surfaces.
func TestAuthoredPlaybooks_NotesPairExists(t *testing.T) {
	entries, err := fs.ReadDir(playbooks.FS, ".")
	if err != nil {
		t.Fatalf("read playbooks.FS: %v", err)
	}
	notes := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), "_notes.md") {
			base := strings.TrimSuffix(e.Name(), "_notes.md")
			notes[base] = true
		}
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".json")
		if !notes[base] {
			t.Errorf("playbook %s.json has no matching %s_notes.md companion; agent would see empty notes_text at recall time", base, base)
		}
	}
}
