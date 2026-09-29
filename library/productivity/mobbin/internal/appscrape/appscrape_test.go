// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package appscrape

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// extractPayloadArray must locate the Next.js Flight value chunk, walk
// brackets honoring strings + escapes, and return the closed [...] slice.
// Regressions here silently break every `app <slug>` scrape.
func TestExtractPayloadArray_Basic(t *testing.T) {
	in := `prefix-junk[{"value":[{"id":"flow1"}]}]suffix`
	got, err := extractPayloadArray(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, `[{"value":`) || !strings.HasSuffix(got, `}]`) {
		t.Fatalf("not a balanced slice: %s", got)
	}
}

// Bracket balance must survive nested objects + arrays.
func TestExtractPayloadArray_Nested(t *testing.T) {
	in := `noise[{"value":[{"a":[1,2,{"b":[3]}]}]}]more`
	got, err := extractPayloadArray(in)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(got, "[") != strings.Count(got, "]") {
		t.Fatalf("unbalanced: %s", got)
	}
}

// Strings containing closing brackets must not advance the depth counter.
func TestExtractPayloadArray_StringWithBrackets(t *testing.T) {
	in := `[{"value":[{"label":"close ] in string"}]}]`
	got, err := extractPayloadArray(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `close ] in string`) {
		t.Fatalf("string truncated: %s", got)
	}
}

// Missing payload pattern must error, not silently emit garbage.
func TestExtractPayloadArray_Missing(t *testing.T) {
	if _, err := extractPayloadArray(`nothing useful here`); err == nil {
		t.Fatal("expected error")
	}
}

func TestFindString(t *testing.T) {
	rows := []map[string]any{{"x": 1}, {"appName": "Stripe"}}
	if got := findString(rows, "appName", "app_name"); got != "Stripe" {
		t.Fatalf("got %q", got)
	}
	if got := findString(rows, "nope"); got != "" {
		t.Fatalf("got %q; want empty", got)
	}
}

// pushScript wraps a raw RSC stream the way Next.js inlines it in HTML.
func pushScript(stream string) string {
	b, _ := json.Marshal(stream)
	return `<script>self.__next_f.push([1,` + string(b) + `])</script>`
}

// testdata/app_page_rsc.html is a trimmed, scrubbed copy of a live Mobbin app
// page (2026-09): the component row references screens/partialFlows in
// another row, appInfo sits behind an element "props" path, and a T text row
// contains a newline plus a fake "4d:" line.
func TestParse_CurrentRSCAppPage(t *testing.T) {
	html, err := os.ReadFile("testdata/app_page_rsc.html")
	if err != nil {
		t.Fatal(err)
	}
	const slug = "brick-ios-72304e20-0d6a-4030-be48-f53a9831e891"
	const versionID = "897f091c-52a5-4c24-bf75-73f4eaef6fca"
	got, err := Parse(string(html), slug)
	if err != nil {
		t.Fatal(err)
	}
	if got.Slug != slug || got.AppName != "Brick" {
		t.Fatalf("slug/app name = %q/%q", got.Slug, got.AppName)
	}
	if len(got.Flows) != 2 || len(got.Screens) != 4 || len(got.Versions) != 1 {
		t.Fatalf("flows=%d screens=%d versions=%d; want 2/4/1", len(got.Flows), len(got.Screens), len(got.Versions))
	}
	if got.Flows[0]["name"] != "Onboarding" {
		t.Fatalf("first flow name = %v", got.Flows[0]["name"])
	}
	steps, ok := got.Flows[0]["screens"].([]any)
	if !ok || len(steps) != 2 {
		t.Fatalf("flow screens = %#v", got.Flows[0]["screens"])
	}
	if step, _ := steps[0].(map[string]any); step["screenId"] != got.Screens[0]["id"] {
		t.Fatalf("flow step %v does not point at first screen %v", step["screenId"], got.Screens[0]["id"])
	}
	for _, rows := range [][]map[string]any{got.Flows, got.Screens} {
		for _, r := range rows {
			if r["appVersionId"] != versionID {
				t.Fatalf("appVersionId not stamped: %v", r["appVersionId"])
			}
		}
	}
	if got.Versions[0]["id"] != versionID {
		t.Fatalf("version id = %v", got.Versions[0]["id"])
	}
	// "$undefined" decodes to null, not a literal string.
	if v, ok := got.Screens[0]["publishedAt"]; !ok || v != nil {
		t.Fatalf("publishedAt = %#v; want nil", v)
	}
	// "$$" is an escaped literal dollar and must not be followed as a ref.
	ocr, _ := got.Screens[2]["ocrBoundingBoxes"].([]any)
	if len(ocr) != 2 || ocr[0].(map[string]any)["text"] != "$" || ocr[1].(map[string]any)["text"] != "$4.99" {
		t.Fatalf("ocr text not unescaped: %#v", ocr)
	}
	// Nested dedupe references inside a screen resolve to the shared object.
	anim, _ := got.Screens[1]["animationCdnVideoSources"].(map[string]any)
	sources, _ := anim["sources"].([]any)
	if len(sources) != 2 {
		t.Fatalf("animation sources = %#v", anim["sources"])
	}
	if src, _ := sources[1].(map[string]any); src["width"] != float64(1080) {
		t.Fatalf("nested ref not resolved: %#v", sources[1])
	}
}

// Props inlined directly in the component row (no cross-row references).
func TestParse_InlineProps(t *testing.T) {
	stream := `0:{"P":null}` + "\n" +
		`4d:[["$","$La7",null,{"appSlug":"x","appInfo":{"appName":"Inline","appVersions":[{"id":"v1"},{"id":"v2"}]},"appVersionId":"v2","paywalled":false,"screens":[{"id":"s1","appVersionId":"v1"}],"partialFlows":[{"id":"f1","name":"Login","screens":[{"screenId":"s1"}]}]}]]` + "\n"
	got, err := Parse(pushScript(stream), "x")
	if err != nil {
		t.Fatal(err)
	}
	if got.AppName != "Inline" || len(got.Flows) != 1 || len(got.Screens) != 1 || len(got.Versions) != 2 {
		t.Fatalf("got %+v", got)
	}
	if got.Screens[0]["appVersionId"] != "v1" {
		t.Fatalf("existing appVersionId overwritten: %v", got.Screens[0]["appVersionId"])
	}
	if got.Flows[0]["appVersionId"] != "v2" {
		t.Fatalf("missing appVersionId not stamped: %v", got.Flows[0]["appVersionId"])
	}
}

// An app page whose lists are empty reports empty rows instead of failing.
func TestParse_EmptyAppPage(t *testing.T) {
	stream := `4d:["$","$La7",null,{"appSlug":"x","appInfo":{"appName":"Empty","appVersions":[]},"screens":[],"partialFlows":[]}]` + "\n"
	got, err := Parse(pushScript(stream), "x")
	if err != nil {
		t.Fatal(err)
	}
	if got.AppName != "Empty" || got.Flows == nil || got.Screens == nil || got.Versions == nil {
		t.Fatalf("got %+v", got)
	}
	if len(got.Flows)+len(got.Screens)+len(got.Versions) != 0 {
		t.Fatalf("expected empty lists, got %+v", got)
	}
}

// Pages that predate the RSC props shape still parse through the legacy path.
func TestParse_LegacyValuePayload(t *testing.T) {
	stream := `1:[{"value":[{"id":"f1"}]},{"value":[{"id":"s1","appName":"Stripe"}]}]`
	got, err := Parse(pushScript(stream), "stripe-web")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Flows) != 1 || len(got.Screens) != 1 || got.AppName != "Stripe" {
		t.Fatalf("got %+v", got)
	}
	if got.Versions == nil {
		t.Fatal("versions should be an empty list, not nil")
	}
}

func TestParse_NoPayload(t *testing.T) {
	if _, err := Parse(pushScript(`0:{"P":null}`+"\n"), "x"); err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveRef(t *testing.T) {
	fl := newFlightRows(`a:["$","div",null,{"children":[null,{"v":"$b:0"}]}]` + "\n" + `b:["ok",{"loop":"$b:1:loop"}]` + "\n")
	cases := map[string]any{
		"$a:props:children:1:v": "$b:0", // resolveRef returns the raw target; decode follows it
		"$b:0":                  "ok",
	}
	for ref, want := range cases {
		got, ok := fl.resolveRef(ref, 0)
		if !ok || got != want {
			t.Fatalf("resolveRef(%q) = %#v, %v; want %#v", ref, got, ok, want)
		}
	}
	if got := fl.decode("$a:props:children:1:v", 0); got != "ok" {
		t.Fatalf("decode chained ref = %#v", got)
	}
	for _, ref := range []string{"$zz:0", "$c:0", "$b:9", "$a:props:nope", "$L64"} {
		if _, ok := fl.resolveRef(ref, 0); ok {
			t.Fatalf("resolveRef(%q) should fail", ref)
		}
	}
	// A self-referencing value terminates instead of recursing forever.
	_ = fl.decode("$b:1", 0)
	// Non-reference markers pass through unchanged.
	if got := fl.decode("$L64", 0); got != "$L64" {
		t.Fatalf("decode($L64) = %#v", got)
	}
}

// Props whose screens/partialFlows references point nowhere are a scrape
// failure, never a successful empty app.
func TestParse_UnresolvableReferenceErrors(t *testing.T) {
	cases := map[string]string{
		"missing row": `4d:["$","$La7",null,{"appSlug":"x","screens":"$99:props:screens","partialFlows":"$99:props:partialFlows"}]` + "\n",
		"missing path": `2e:["$","$L2f",null,{"other":[]}]` + "\n" +
			`4d:["$","$La7",null,{"appSlug":"x","screens":"$2e:props:screens","partialFlows":"$2e:props:partialFlows"}]` + "\n",
		"one side unresolved": `4d:["$","$La7",null,{"appSlug":"x","screens":[{"id":"s1"}],"partialFlows":"$99:props:partialFlows"}]` + "\n",
	}
	for name, stream := range cases {
		got, err := Parse(pushScript(stream), "x")
		if err == nil {
			t.Fatalf("%s: expected error, got %+v", name, got)
		}
		if !strings.Contains(err.Error(), "unresolvable") {
			t.Fatalf("%s: error should name the unresolved props, got %v", name, err)
		}
	}
}

// Broken props still defer to a legacy payload when the page carries one.
func TestParse_UnresolvableReferenceFallsBackToLegacy(t *testing.T) {
	stream := `4d:["$","$La7",null,{"appSlug":"x","screens":"$99:props:screens","partialFlows":"$99:props:partialFlows"}]` + "\n" +
		`1:[{"value":[{"id":"f1"}]},{"value":[{"id":"s1"}]}]` + "\n"
	got, err := Parse(pushScript(stream), "x")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Flows) != 1 || len(got.Screens) != 1 {
		t.Fatalf("got %+v", got)
	}
}
