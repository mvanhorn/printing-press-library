// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFetchLocalLinkedInCompanyUsesCLIJSON(t *testing.T) {
	script := filepath.Join(t.TempDir(), "linkedin-network-fake")
	body := `#!/bin/sh
if [ "$1" != "company" ]; then
  echo "unexpected command: $1" >&2
  exit 2
fi
cat <<'JSON'
{
  "count": 1,
  "results": [
    {
      "id": "local-1",
      "name": "Alex Local",
      "linkedin_url": "https://linkedin.com/in/alex-local",
      "company": "Acme",
      "title": "VP Partnerships",
      "owners": ["Holger", "James"],
      "relationship": "linkedin_export_1deg_both",
      "sources": ["local_linkedin_export"],
      "rationale": "Known by Holger and James via LinkedIn export",
      "score": 13.2
    }
  ]
}
JSON
`
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatalf("write fake local LinkedIn CLI: %v", err)
	}
	t.Setenv(localLinkedInCmdEnv, script)

	got, err := fetchLocalLinkedInCompany(context.Background(), "Acme", 5)
	if err != nil {
		t.Fatalf("fetchLocalLinkedInCompany returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	row := got[0]
	if row.Name != "Alex Local" || row.Company != "Acme" || row.LinkedInURL == "" {
		t.Fatalf("unexpected row: %+v", row)
	}
	if row.Relationship != "linkedin_export_1deg_both" {
		t.Fatalf("relationship = %q", row.Relationship)
	}
	if len(row.Owners) != 2 || row.Owners[0] != "Holger" || row.Owners[1] != "James" {
		t.Fatalf("owners = %#v, want Holger+James", row.Owners)
	}
	if row.MutualCount != 2 {
		t.Fatalf("mutual count = %d, want 2", row.MutualCount)
	}
}

func TestParseSourceFlagAcceptsLocalAliases(t *testing.T) {
	got := parseSourceFlag("ln,li")
	if !got[SourceFlagLocal] {
		t.Fatalf("ln should map to %q, got %#v", SourceFlagLocal, got)
	}
	if got[SourceFlagLN] {
		t.Fatalf("raw ln token should be normalized away, got %#v", got)
	}
	if !got["li"] {
		t.Fatalf("li source missing: %#v", got)
	}

	both := parseSourceFlag("both")
	for _, want := range []string{SourceFlagLocal, "li", "hp"} {
		if !both[want] {
			t.Fatalf("both should include %q, got %#v", want, both)
		}
	}
}

func TestLocalLinkedInRankingStrength(t *testing.T) {
	if got := sourceStrength(localLinkedInSourceTag); got <= sourceStrength("li_search") {
		t.Fatalf("local source should outrank raw LinkedIn search, got %.1f", got)
	}
	if got := scoreForRelationship("linkedin_export_1deg_both"); got <= scoreForRelationship("linkedin_export_1deg_james") {
		t.Fatalf("both-owner local relationship should outrank single owner, got %.1f", got)
	}
}
