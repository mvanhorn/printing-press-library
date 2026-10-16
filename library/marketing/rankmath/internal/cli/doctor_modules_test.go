// Copyright 2026 ElizabethcStone and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/cliutil/testenv"
)

func TestModulesFromIndex(t *testing.T) {
	for _, tc := range []struct {
		name          string
		raw           json.RawMessage
		installed     bool
		routes        int
		allModules    bool
		wantDecodeErr bool
	}{
		{name: "full index", raw: json.RawMessage(`{"namespaces":["wp/v2","rankmath/v1"],"routes":{"/rankmath/v1/an/dashboard":{},"/rankmath/v1/an/getTrackedKeywords":{},"/rankmath/v1/an/inspectionResults":{},"/rankmath/v1/in/submitUrls":{},"/rankmath/v1/link-genius/links":{},"/rankmath/v1/ai-visibility/overview":{},"/rankmath/v1/getHead":{},"/rankmath/v1/status/exportSettings":{},"/rankmath/v1/other":{},"/wp/v2/posts":{}}}`), installed: true, routes: 9, allModules: true},
		{name: "without Rank Math namespace", raw: json.RawMessage(`{"namespaces":["wp/v2"],"routes":{"/wp/v2/posts":{}}}`)},
		{name: "malformed JSON", raw: json.RawMessage(`{"namespaces":[`), wantDecodeErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := modulesFromIndex(tc.raw)
			if (err != nil) != tc.wantDecodeErr {
				t.Fatalf("error = %v", err)
			}
			if tc.wantDecodeErr {
				return
			}
			if got.RankMathInstalled != tc.installed || got.RankMathRoutes != tc.routes {
				t.Fatalf("installed/routes = %v/%d", got.RankMathInstalled, got.RankMathRoutes)
			}
			modules := []bool{got.Modules.Analytics, got.Modules.RankTracking, got.Modules.URLInspection, got.Modules.InstantIndexing, got.Modules.LinkGenius, got.Modules.AIVisibility, got.Modules.HeadlessGetHead, got.Modules.StatusTools}
			for i, available := range modules {
				if available != tc.allModules {
					t.Errorf("module %d availability = %v", i, available)
				}
			}
		})
	}
}

func TestDoctorModulesBaseURLScope(t *testing.T) {
	t.Setenv("RANKMATH_BASE_URL", "https://site-a.test/wp-json")
	got, err := doctorModulesIndexURL("site-b.test")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://site-b.test/wp-json/" {
		t.Fatalf("mismatched override used: %s", got)
	}
	got, err = doctorModulesIndexURL("site-a.test")
	if err != nil || got != "https://site-a.test/wp-json/" {
		t.Fatalf("matching override = %q, %v", got, err)
	}
	got, err = doctorModulesIndexURL("SITE-A.TEST")
	if err != nil || got != "https://site-a.test/wp-json/" {
		t.Fatalf("case-insensitive matching override = %q, %v", got, err)
	}
	u, err := url.Parse(got)
	if err != nil || u.Host != "site-a.test" {
		t.Fatalf("invalid URL %q: %v", got, err)
	}
}

func TestDoctorModulesHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"doctor", "modules", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor modules --help error = %v", err)
	}
	for _, want := range []string{"doctor modules", "--sites", "--json"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("help missing %q:\n%s", want, out.String())
		}
	}
}
