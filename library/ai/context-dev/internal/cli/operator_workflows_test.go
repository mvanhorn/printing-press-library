// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEntityDiscoverValidationRejectsSensitiveAndFreeFormInput(t *testing.T) {
	t.Parallel()
	bad := [][]string{
		{"--type", "provider", "--name", "Jane Smith", "--location", "patient DOB 01/02/1980"},
		{"--type", "company", "--name", "Acme; this is a long note", "--location", "Austin"},
		{"--type", "person", "--name", "Jane Smith", "--location", "Austin"},
	}
	for _, args := range bad {
		args := args
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Parallel()
			all := append([]string{"entity-discover"}, args...)
			_, err := runContextDevCommand(all...)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if ExitCode(err) != 2 {
				t.Fatalf("ExitCode = %d, want 2", ExitCode(err))
			}
		})
	}
}

func TestEntityDiscoverDistinguishesZeroResultsFromSearchFailure(t *testing.T) {
	t.Run("zero results", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/web/search" {
				t.Fatalf("unexpected path %s", r.URL.Path)
			}
			_, _ = w.Write([]byte(`{"results":[]}`))
		}))
		defer server.Close()
		t.Setenv("CONTEXT_DEV_BASE_URL", server.URL)
		t.Setenv("CONTEXT_DEV_API_KEY", "test-key")

		out, err := runContextDevCommand("entity-discover", "--type", "company", "--name", "Nope", "--location", "Austin", "--json")
		if err != nil {
			t.Fatalf("zero results returned error: %v", err)
		}
		var got []entityWorkflowCandidate
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatalf("invalid JSON: %v; raw=%s", err, out.String())
		}
		if len(got) != 0 {
			t.Fatalf("len = %d, want 0", len(got))
		}
	})

	t.Run("api failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"error":"upstream unavailable"}`, http.StatusBadGateway)
		}))
		defer server.Close()
		t.Setenv("CONTEXT_DEV_BASE_URL", server.URL)
		t.Setenv("CONTEXT_DEV_API_KEY", "test-key")

		_, err := runContextDevCommand("entity-discover", "--type", "company", "--name", "Acme", "--location", "Austin", "--json")
		if err == nil {
			t.Fatal("expected API failure")
		}
		if ExitCode(err) != 5 {
			t.Fatalf("ExitCode = %d, want 5", ExitCode(err))
		}
	})
}

func TestBrandBriefOutputShapeAndProvenance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/brand/retrieve":
			_, _ = w.Write([]byte(`{"title":"Example Co","description":"Example description","website":"https://example.com","logos":[{"url":"https://example.com/logo.png"}],"socials":{"linkedin":"https://linkedin.example/example"},"email":"hello@example.com"}`))
		case "/web/styleguide":
			_, _ = w.Write([]byte(`{"colors":["#112233"],"fonts":["Inter"]}`))
		case "/web/screenshot":
			_, _ = w.Write([]byte(`{"url":"https://cdn.example/screenshot.png"}`))
		case "/web/scrape/markdown":
			_, _ = w.Write([]byte(`{"markdown":"Welcome to Example Co. Contact us at https://example.com/contact"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("CONTEXT_DEV_BASE_URL", server.URL)
	t.Setenv("CONTEXT_DEV_API_KEY", "test-key")

	out, err := runContextDevCommand("brand-brief", "example.com", "--json")
	if err != nil {
		t.Fatalf("brand-brief returned error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v; raw=%s", err, out.String())
	}
	for _, key := range []string{"domain", "website", "title", "description", "logo", "colors", "fonts", "socials", "contact_surfaces", "screenshot", "summary", "provenance"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("missing output key %q in %#v", key, got)
		}
	}
	if len(got["provenance"].([]any)) < 4 {
		t.Fatalf("provenance = %#v, want all composed calls", got["provenance"])
	}
}

// TestBrandBriefUnwrapsLiveEnvelopeShape locks in the real Context.dev response
// envelope: /brand/retrieve nests under "brand", /web/styleguide nests under
// "styleguide", socials is a [{type,url}] array, and the category lives under
// industries.eic[].industry. Before the unwrap fix these endpoints returned "ok"
// but every brand/style field was silently dropped from the brief.
func TestBrandBriefUnwrapsLiveEnvelopeShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/brand/retrieve":
			_, _ = w.Write([]byte(`{"status":"ok","code":200,"brand":{"title":"Example Co","description":"Example makes things","slogan":"Be excellent","domain":"example.com","logos":[{"url":"https://media.example/logo.png","type":"logo"}],"colors":[{"hex":"#04142c","name":"Ink"}],"socials":[{"type":"instagram","url":"https://instagram.com/example"},{"type":"youtube","url":"https://youtube.com/example"}],"address":{"city":"Austin","state_province":"Texas","country":"UNITED STATES"},"industries":{"eic":[{"industry":"Retail & E-commerce","subindustry":"Direct-to-Consumer (DTC) Brands"}]}}}`))
		case "/web/styleguide":
			_, _ = w.Write([]byte(`{"status":"ok","domain":"example.com","styleguide":{"colors":{"accent":"#2c3c60","background":"#ffffff","text":"#000000"},"typography":{"headings":{"h1":{"fontFamily":"Geist"}}}}}`))
		case "/web/screenshot":
			_, _ = w.Write([]byte(`{"screenshot":"https://media.example/screenshot.png"}`))
		case "/web/scrape/markdown":
			_, _ = w.Write([]byte(`{"markdown":"We use cookies. Skip to content. Contact us at https://example.com/contact"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("CONTEXT_DEV_BASE_URL", server.URL)
	t.Setenv("CONTEXT_DEV_API_KEY", "test-key")

	out, err := runContextDevCommand("brand-brief", "example.com", "--json")
	if err != nil {
		t.Fatalf("brand-brief returned error: %v", err)
	}
	var got brandBrief
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v; raw=%s", err, out.String())
	}
	if got.Title != "Example Co" {
		t.Fatalf("title = %q, want unwrapped brand title", got.Title)
	}
	if got.Description != "Example makes things" {
		t.Fatalf("description = %q, want unwrapped brand description", got.Description)
	}
	if got.Logo != "https://media.example/logo.png" {
		t.Fatalf("logo = %q, want unwrapped logos[0].url", got.Logo)
	}
	if got.Colors == nil {
		t.Fatal("colors empty; styleguide palette was dropped")
	}
	if got.Fonts == nil {
		t.Fatal("fonts empty; styleguide typography was dropped")
	}
	if got.Socials["instagram"] != "https://instagram.com/example" || got.Socials["youtube"] != "https://youtube.com/example" {
		t.Fatalf("socials = %#v, want array normalized to platform=>url map", got.Socials)
	}
	// Summary should be the clean brand description, not the raw cookie-banner dump.
	if strings.Contains(strings.ToLower(got.Summary), "cookies") {
		t.Fatalf("summary leaked raw scrape noise: %q", got.Summary)
	}
	if got.Summary != "Example makes things" {
		t.Fatalf("summary = %q, want clean brand description", got.Summary)
	}
	// Address (object) and contact link should surface in contact_surfaces.
	var hasAddress, hasContact bool
	for _, s := range got.ContactSurfaces {
		if strings.Contains(s, "Austin") {
			hasAddress = true
		}
		if strings.Contains(s, "/contact") {
			hasContact = true
		}
	}
	if !hasAddress {
		t.Fatalf("contact_surfaces missing formatted address: %#v", got.ContactSurfaces)
	}
	if !hasContact {
		t.Fatalf("contact_surfaces missing scraped contact link: %#v", got.ContactSurfaces)
	}
}

// TestCompetitorMapResolvesCategoryFromIndustries verifies that the per-competitor
// brand enrichment unwraps the envelope and reads the nested industries
// classification, so clusters are meaningful instead of a single "unknown" bucket.
func TestCompetitorMapResolvesCategoryFromIndustries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/web/competitors":
			_, _ = w.Write([]byte(`{"competitors":[{"name":"Rival Co","website":"https://rival.example","description":"A direct rival"}]}`))
		case "/brand/retrieve":
			_, _ = w.Write([]byte(`{"status":"ok","brand":{"title":"Rival Co","industries":{"eic":[{"industry":"Retail & E-commerce","subindustry":"DTC"}]}}}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("CONTEXT_DEV_BASE_URL", server.URL)
	t.Setenv("CONTEXT_DEV_API_KEY", "test-key")

	out, err := runContextDevCommand("competitor-map", "--domain", "example.com", "--max", "1", "--json")
	if err != nil {
		t.Fatalf("competitor-map returned error: %v", err)
	}
	var got competitorMapOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v; raw=%s", err, out.String())
	}
	if len(got.Competitors) != 1 {
		t.Fatalf("competitors = %d, want 1", len(got.Competitors))
	}
	if got.Competitors[0].Category != "Retail & E-commerce" {
		t.Fatalf("category = %q, want resolved from industries.eic", got.Competitors[0].Category)
	}
	if len(got.Clusters) != 1 || got.Clusters[0].Category != "Retail & E-commerce" {
		t.Fatalf("clusters = %#v, want a single Retail & E-commerce cluster", got.Clusters)
	}
}

func TestPublicSearchQueryAllowsNaturalQuestionPunctuation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/web/search":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if got, _ := body["query"].(string); !strings.Contains(got, "pricing?") {
				t.Fatalf("query = %q, want punctuation preserved", got)
			}
			_, _ = w.Write([]byte(`{"results":[{"title":"Pricing","url":"https://pricing.example","snippet":"Context.dev pricing"}]}`))
		case "/web/scrape/markdown":
			_, _ = w.Write([]byte(`{"markdown":"Pricing details"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("CONTEXT_DEV_BASE_URL", server.URL)
	t.Setenv("CONTEXT_DEV_API_KEY", "test-key")

	_, err := runContextDevCommand("source-pack", "--query", "what's Context.dev pricing?", "--max-sources", "1", "--json")
	if err != nil {
		t.Fatalf("source-pack rejected natural search query: %v", err)
	}
}

func TestOverlapSignalsOnlyReportsComputedMatches(t *testing.T) {
	brand := map[string]any{"description": "privacy analytics platform", "industries": map[string]any{"eic": []any{map[string]any{"industry": "Analytics"}}}}
	result := searchResult{Title: "Rival Analytics", URL: "https://rival.example", Snippet: "privacy pricing"}
	signals := overlapSignals("seed.example", "pricing?", "healthcare", result, brand)
	joined := strings.Join(signals, "|")
	if strings.Contains(joined, "seed-domain") || strings.Contains(joined, "market: healthcare") {
		t.Fatalf("signals included echoed inputs instead of computed matches: %#v", signals)
	}
	if !strings.Contains(joined, "result text matches search query") || !strings.Contains(joined, "brand category available") {
		t.Fatalf("signals missing computed matches: %#v", signals)
	}
}

func TestSourcePackPartialScrapeFailureKeepsSuccessfulSources(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/web/search":
			_, _ = w.Write([]byte(`{"results":[{"title":"A","url":"https://a.example","snippet":"alpha"},{"title":"B","url":"https://b.example","snippet":"beta"}]}`))
		case "/web/scrape/markdown":
			if r.URL.Query().Get("url") == "https://b.example" {
				http.Error(w, `{"error":"blocked"}`, http.StatusBadGateway)
				return
			}
			_, _ = w.Write([]byte(`{"markdown":"alpha claim"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("CONTEXT_DEV_BASE_URL", server.URL)
	t.Setenv("CONTEXT_DEV_API_KEY", "test-key")

	out, err := runContextDevCommand("source-pack", "--query", "alpha beta", "--max-sources", "2", "--json")
	if err != nil {
		t.Fatalf("source-pack returned error despite partial scrape failure: %v", err)
	}
	var got sourcePackOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v; raw=%s", err, out.String())
	}
	if len(got.Sources) != 2 || got.Sources[0].Error != "" || got.Sources[1].Error == "" {
		t.Fatalf("partial source statuses = %#v", got.Sources)
	}
	if len(got.Claims) != 1 {
		t.Fatalf("claims = %d, want one successful claim", len(got.Claims))
	}
	if len(got.Sources[1].Provenance) == 0 {
		t.Fatal("failed source is missing provenance")
	}
}

func TestSchemaLabPartialExtractionFailuresDoNotAbortBatch(t *testing.T) {
	schemaPath := filepath.Join(t.TempDir(), "schema.json")
	if err := os.WriteFile(schemaPath, []byte(`{"type":"object","properties":{"title":{"type":"string"},"phone":{"type":"string"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/web/extract" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["url"] == "https://bad.example" {
			http.Error(w, `{"error":"extract failed"}`, http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"title":"Good"}`))
	}))
	defer server.Close()
	t.Setenv("CONTEXT_DEV_BASE_URL", server.URL)
	t.Setenv("CONTEXT_DEV_API_KEY", "test-key")

	out, err := runContextDevCommand("schema-lab", "--url", "https://good.example", "--url", "https://bad.example", "--schema", schemaPath, "--json")
	if err != nil {
		t.Fatalf("schema-lab returned error despite partial failure: %v", err)
	}
	var got schemaLabOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v; raw=%s", err, out.String())
	}
	if got.ParseFailures != 1 || len(got.Results) != 2 {
		t.Fatalf("ParseFailures=%d results=%d, want 1 and 2", got.ParseFailures, len(got.Results))
	}
	if got.FieldFillRates["title"].Filled != 1 || got.FieldFillRates["phone"].Filled != 0 {
		t.Fatalf("fill rates = %#v", got.FieldFillRates)
	}
}

// TestSchemaLabUnwrapsExtractDataEnvelope locks in the live /web/extract shape:
// extracted fields are nested under "data". Before the unwrap fix every field
// reported missing (0% fill) even though extraction succeeded.
func TestSchemaLabUnwrapsExtractDataEnvelope(t *testing.T) {
	schemaPath := filepath.Join(t.TempDir(), "schema.json")
	if err := os.WriteFile(schemaPath, []byte(`{"type":"object","properties":{"company_name":{"type":"string"},"product_count":{"type":"integer"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/web/extract" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"status":"ok","url":"https://example.com","data":{"company_name":"Example Co.","product_count":50},"metadata":{"numSucceeded":5}}`))
	}))
	defer server.Close()
	t.Setenv("CONTEXT_DEV_BASE_URL", server.URL)
	t.Setenv("CONTEXT_DEV_API_KEY", "test-key")

	out, err := runContextDevCommand("schema-lab", "--url", "https://example.com", "--schema", schemaPath, "--json")
	if err != nil {
		t.Fatalf("schema-lab returned error: %v", err)
	}
	var got schemaLabOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v; raw=%s", err, out.String())
	}
	if got.FieldFillRates["company_name"].Filled != 1 || got.FieldFillRates["product_count"].Filled != 1 {
		t.Fatalf("fill rates = %#v, want both filled from data envelope", got.FieldFillRates)
	}
	if got.ParseFailures != 0 {
		t.Fatalf("ParseFailures = %d, want 0", got.ParseFailures)
	}
}

// TestAssetPackUnwrapsEnvelopesAndCleansFonts verifies the asset bundle unwraps
// the brand/styleguide envelopes and that the fonts list contains family names
// only -- not the px size/spacing values that live alongside them in typography.
func TestAssetPackUnwrapsEnvelopesAndCleansFonts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/brand/retrieve":
			_, _ = w.Write([]byte(`{"status":"ok","brand":{"title":"Example Co","logos":[{"url":"https://media.example/logo.png"}],"socials":[{"type":"x","url":"https://x.com/example"}]}}`))
		case "/web/styleguide":
			_, _ = w.Write([]byte(`{"status":"ok","styleguide":{"colors":{"accent":"#2c3c60"},"typography":{"headings":{"h1":{"fontFamily":"Geist","fontSize":"32px","letterSpacing":"0px","fontFallbacks":["Geist","sans-serif"]}}}}}`))
		case "/web/screenshot":
			_, _ = w.Write([]byte(`{"screenshot":"https://media.example/shot.png"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("CONTEXT_DEV_BASE_URL", server.URL)
	t.Setenv("CONTEXT_DEV_API_KEY", "test-key")

	out, err := runContextDevCommand("brand-kit", "example.com", "--json")
	if err != nil {
		t.Fatalf("brand-kit returned error: %v", err)
	}
	var got assetPackOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v; raw=%s", err, out.String())
	}
	if got.Title != "Example Co" || got.Logo != "https://media.example/logo.png" {
		t.Fatalf("brand envelope not unwrapped: title=%q logo=%q", got.Title, got.Logo)
	}
	if got.Socials["x"] != "https://x.com/example" {
		t.Fatalf("socials = %#v, want normalized map", got.Socials)
	}
	if len(got.Fonts) == 0 {
		t.Fatal("fonts empty; typography was dropped")
	}
	for _, f := range got.Fonts {
		if strings.HasSuffix(f, "px") || (f != "" && f[0] >= '0' && f[0] <= '9') {
			t.Fatalf("fonts leaked size/spacing value %q: %#v", f, got.Fonts)
		}
	}
	wantFont := false
	for _, f := range got.Fonts {
		if f == "Geist" {
			wantFont = true
		}
	}
	if !wantFont {
		t.Fatalf("fonts missing family Geist: %#v", got.Fonts)
	}
}

// TestBrandKitAssetPackAliasStillResolves guards the back-compat alias after the
// asset-pack -> brand-kit rename.
func TestBrandKitAssetPackAliasStillResolves(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/brand/retrieve":
			_, _ = w.Write([]byte(`{"status":"ok","brand":{"title":"Example Co"}}`))
		case "/web/styleguide":
			_, _ = w.Write([]byte(`{"status":"ok","styleguide":{"colors":{"accent":"#000"}}}`))
		case "/web/screenshot":
			_, _ = w.Write([]byte(`{"screenshot":"https://media.example/shot.png"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("CONTEXT_DEV_BASE_URL", server.URL)
	t.Setenv("CONTEXT_DEV_API_KEY", "test-key")

	out, err := runContextDevCommand("asset-pack", "example.com", "--json")
	if err != nil {
		t.Fatalf("asset-pack alias returned error: %v", err)
	}
	var got assetPackOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v; raw=%s", err, out.String())
	}
	if got.Title != "Example Co" {
		t.Fatalf("alias did not run brand-kit: %#v", got)
	}
}

// TestBrandQAWrapsQuestionAndReadsDataExtracted verifies brand-qa wraps the
// --question as a text datapoint and surfaces data.data_extracted[].datapoint_value.
func TestBrandQAWrapsQuestionAndReadsDataExtracted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/brand/ai/query" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		dte, ok := body["data_to_extract"].([]any)
		if !ok || len(dte) != 1 {
			t.Fatalf("data_to_extract not wrapped as single datapoint: %#v", body["data_to_extract"])
		}
		dp, ok := dte[0].(map[string]any)
		if !ok || dp["datapoint_description"] != "What is the return policy?" {
			t.Fatalf("question not mapped to datapoint_description: %#v", dte[0])
		}
		if s, _ := dp["datapoint_example"].(string); len(s) == 0 {
			t.Fatalf("datapoint_example must be non-empty (API requires min length 1): %#v", dp)
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"data_extracted":[{"datapoint_name":"answer","datapoint_value":"30-day money-back guarantee."}],"urls_analyzed":["https://example.com","https://example.com/faqs"]}}`))
	}))
	defer server.Close()
	t.Setenv("CONTEXT_DEV_BASE_URL", server.URL)
	t.Setenv("CONTEXT_DEV_API_KEY", "test-key")

	out, err := runContextDevCommand("brand-qa", "example.com", "--question", "What is the return policy?", "--json")
	if err != nil {
		t.Fatalf("brand-qa returned error: %v", err)
	}
	var got brandQAOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v; raw=%s", err, out.String())
	}
	if got.Answer != "30-day money-back guarantee." {
		t.Fatalf("answer = %q, want extracted datapoint_value", got.Answer)
	}
	if len(got.URLsAnalyzed) != 2 {
		t.Fatalf("urls_analyzed = %#v, want 2 entries", got.URLsAnalyzed)
	}
}

// TestBrandQADryRunPlanMatchesRealBody guards that the --dry-run/--estimate plan
// shows the actual data_to_extract wrapper, not a flattened top-level question
// field that does not exist in the real request (Greptile P2).
func TestBrandQADryRunPlanMatchesRealBody(t *testing.T) {
	t.Parallel()
	out, err := runContextDevCommand("brand-qa", "example.com", "--question", "What is the return policy?", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("dry-run returned error: %v", err)
	}
	var plan workflowEstimate
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil {
		t.Fatalf("invalid JSON: %v; raw=%s", err, out.String())
	}
	if len(plan.PlannedRequests) != 1 {
		t.Fatalf("planned requests = %#v", plan.PlannedRequests)
	}
	input := plan.PlannedRequests[0].Input
	if _, leaked := input["question"]; leaked {
		t.Fatalf("plan leaks a top-level question field absent from the real body: %#v", input)
	}
	if _, ok := input["data_to_extract"]; !ok {
		t.Fatalf("plan missing data_to_extract wrapper: %#v", input)
	}
}

// TestBrandQARequiresQuestion guards the required --question flag.
func TestBrandQARequiresQuestion(t *testing.T) {
	t.Parallel()
	_, err := runContextDevCommand("brand-qa", "example.com", "--json")
	if err == nil {
		t.Fatal("expected error when --question is omitted")
	}
	if ExitCode(err) != 2 {
		t.Fatalf("ExitCode = %d, want 2", ExitCode(err))
	}
}

// TestEmailEnrichUnwrapsBrandEnvelope verifies email-enrich resolves the company
// + prefill fields from the /brand/retrieve-by-email brand envelope.
func TestEmailEnrichUnwrapsBrandEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/brand/retrieve-by-email" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.URL.Query().Get("email") != "founders@example.com" {
			t.Fatalf("email param = %q", r.URL.Query().Get("email"))
		}
		_, _ = w.Write([]byte(`{"status":"ok","brand":{"title":"Example Co","domain":"example.com","description":"We make things","logos":[{"url":"https://media.example/logo.png"}],"industries":{"eic":[{"industry":"Retail & E-commerce"}]}}}`))
	}))
	defer server.Close()
	t.Setenv("CONTEXT_DEV_BASE_URL", server.URL)
	t.Setenv("CONTEXT_DEV_API_KEY", "test-key")

	out, err := runContextDevCommand("email-enrich", "founders@example.com", "--json")
	if err != nil {
		t.Fatalf("email-enrich returned error: %v", err)
	}
	var got emailEnrichOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v; raw=%s", err, out.String())
	}
	if got.Domain != "example.com" || got.Company["name"] != "Example Co" {
		t.Fatalf("company not resolved: %#v", got)
	}
	if got.Prefill["company_name"] != "Example Co" || got.Prefill["website"] != "https://example.com" {
		t.Fatalf("prefill not built: %#v", got.Prefill)
	}
	if got.Prefill["industry"] != "Retail & E-commerce" {
		t.Fatalf("industry not resolved from industries.eic: %#v", got.Prefill)
	}
}

// TestTickerEnrichRoutesAndEnriches verifies ticker vs ISIN routing and that the
// resolved domain drives the NAICS/SIC enrichment calls.
func TestTickerEnrichRoutesAndEnriches(t *testing.T) {
	for _, tc := range []struct {
		name, arg, wantType, wantPath string
	}{
		{"ticker", "AAPL", "ticker", "/brand/retrieve-by-ticker"},
		{"isin", "US0378331005", "isin", "/brand/retrieve-by-isin"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var hitBrandPath string
			naicsHit := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/brand/retrieve-by-ticker", "/brand/retrieve-by-isin":
					hitBrandPath = r.URL.Path
					_, _ = w.Write([]byte(`{"status":"ok","brand":{"title":"Apple","domain":"apple.com"}}`))
				case "/web/naics":
					naicsHit = true
					if r.URL.Query().Get("input") != "apple.com" {
						t.Fatalf("naics input = %q, want resolved domain", r.URL.Query().Get("input"))
					}
					_, _ = w.Write([]byte(`{"status":"ok","domain":"apple.com","codes":[{"code":"334220","name":"Radio and TV Broadcasting Equipment","confidence":"high"}]}`))
				case "/web/sic":
					_, _ = w.Write([]byte(`{"status":"ok","codes":[{"code":"3571","name":"ELECTRONIC COMPUTERS"}]}`))
				default:
					t.Fatalf("unexpected path %s", r.URL.Path)
				}
			}))
			defer server.Close()
			t.Setenv("CONTEXT_DEV_BASE_URL", server.URL)
			t.Setenv("CONTEXT_DEV_API_KEY", "test-key")

			out, err := runContextDevCommand("ticker-enrich", tc.arg, "--json")
			if err != nil {
				t.Fatalf("ticker-enrich returned error: %v", err)
			}
			var got tickerEnrichOutput
			if err := json.Unmarshal(out.Bytes(), &got); err != nil {
				t.Fatalf("invalid JSON: %v; raw=%s", err, out.String())
			}
			if got.IdentifierType != tc.wantType {
				t.Fatalf("identifier_type = %q, want %q", got.IdentifierType, tc.wantType)
			}
			if hitBrandPath != tc.wantPath {
				t.Fatalf("brand path = %q, want %q", hitBrandPath, tc.wantPath)
			}
			if got.Company["name"] != "Apple" || got.Domain != "apple.com" {
				t.Fatalf("company not resolved: %#v", got)
			}
			if !naicsHit || got.NAICS == nil {
				t.Fatalf("naics not enriched: %#v", got)
			}
		})
	}
}

func TestEstimateAndDryRunDoNotRequireServerOrCredentials(t *testing.T) {
	out, err := runContextDevCommand("source-pack", "--query", "Context dev", "--max-sources", "2", "--estimate", "--json")
	if err != nil {
		t.Fatalf("estimate returned error: %v", err)
	}
	var estimate workflowEstimate
	if err := json.Unmarshal(out.Bytes(), &estimate); err != nil {
		t.Fatalf("invalid estimate JSON: %v; raw=%s", err, out.String())
	}
	if estimate.EstimatedCredits == 0 || len(estimate.PlannedRequests) == 0 {
		t.Fatalf("estimate = %#v", estimate)
	}

	out, err = runContextDevCommand("brand-brief", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("dry-run returned error without args/credentials: %v", err)
	}
	if err := json.Unmarshal(out.Bytes(), &estimate); err != nil {
		t.Fatalf("invalid dry-run JSON: %v; raw=%s", err, out.String())
	}
	if !estimate.DryRun {
		t.Fatalf("dry-run flag not reflected: %#v", estimate)
	}
}

func TestWebsiteChangeDigestStoresSnapshotsUnderStateDir(t *testing.T) {
	home := t.TempDir()
	call := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/web/scrape/markdown":
			call++
			if call <= 1 {
				_, _ = w.Write([]byte(`{"markdown":"First copy https://example.com/a"}`))
				return
			}
			_, _ = w.Write([]byte(`{"markdown":"Second copy https://example.com/b"}`))
		case "/web/styleguide":
			if call <= 1 {
				_, _ = w.Write([]byte(`{"colors":["#111111"],"fonts":["Inter"]}`))
				return
			}
			_, _ = w.Write([]byte(`{"colors":["#222222"],"fonts":["Inter"]}`))
		case "/web/screenshot":
			_, _ = w.Write([]byte(`{"url":"https://cdn.example/screen.png"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	t.Setenv("CONTEXT_DEV_BASE_URL", server.URL)
	t.Setenv("CONTEXT_DEV_API_KEY", "test-key")

	if _, err := runContextDevCommand("--home", home, "website-change-digest", "example.com", "--json"); err != nil {
		t.Fatalf("first digest returned error: %v", err)
	}
	out, err := runContextDevCommand("--home", home, "website-change-digest", "example.com", "--json")
	if err != nil {
		t.Fatalf("second digest returned error: %v", err)
	}
	var got websiteChangeDigest
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v; raw=%s", err, out.String())
	}
	statePrefix := filepath.Join(home, "state")
	if !strings.HasPrefix(got.CurrentSnapshot, statePrefix) || got.PreviousSnapshot == "" {
		t.Fatalf("snapshot paths not under state dir: %#v", got)
	}
	if len(got.ChangedCopy) == 0 || len(got.ChangedLinksFacts) == 0 {
		t.Fatalf("missing expected changes: %#v", got)
	}
}

func TestLeadEnrichBatchReportsRowFailuresAndStrictModeFails(t *testing.T) {
	csvPath := filepath.Join(t.TempDir(), "leads.csv")
	if err := os.WriteFile(csvPath, []byte("domain,name\nok.example,Ok\nbad.example,Bad\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/brand/retrieve" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.URL.Query().Get("domain") == "bad.example" {
			http.Error(w, `{"error":"nope"}`, http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"title":"Ok","website":"https://ok.example"}`))
	}))
	defer server.Close()
	t.Setenv("CONTEXT_DEV_BASE_URL", server.URL)
	t.Setenv("CONTEXT_DEV_API_KEY", "test-key")

	out, err := runContextDevCommand("lead-enrich-batch", csvPath, "--domain-column", "domain", "--json")
	if err != nil {
		t.Fatalf("non-strict batch returned error: %v", err)
	}
	var got leadBatchOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v; raw=%s", err, out.String())
	}
	if got.Summary.Succeeded != 1 || got.Summary.Failed != 1 || got.Rows[1].FailureReason == "" {
		t.Fatalf("batch output = %#v", got)
	}

	out, err = runContextDevCommand("lead-enrich-batch", csvPath, "--domain-column", "domain", "--strict", "--json")
	if err == nil {
		t.Fatal("strict batch should return non-zero")
	}
	if ExitCode(err) != 6 {
		t.Fatalf("ExitCode = %d, want 6", ExitCode(err))
	}
	if !json.Valid(out.Bytes()) {
		t.Fatalf("strict batch should still emit JSON result, got %s", out.String())
	}
}

// TestLeadEnrichBatchResumeRequiresOutput guards against silently reprocessing
// (and re-billing) every row: --resume only skips rows recorded in --output, so
// --resume without --output must be rejected rather than running a full re-enrich.
func TestLeadEnrichBatchResumeRequiresOutput(t *testing.T) {
	t.Parallel()
	csvPath := filepath.Join(t.TempDir(), "leads.csv")
	if err := os.WriteFile(csvPath, []byte("domain\nok.example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := runContextDevCommand("lead-enrich-batch", csvPath, "--domain-column", "domain", "--resume", "--json")
	if err == nil {
		t.Fatal("expected --resume without --output to be rejected")
	}
	if ExitCode(err) != 2 {
		t.Fatalf("ExitCode = %d, want 2", ExitCode(err))
	}
}
