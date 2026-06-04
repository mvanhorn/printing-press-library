// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseBriefToShots(t *testing.T) {
	// Platforms → one shot per platform with the platform's default aspect.
	shots := parseBriefToShots("hero", []string{"instagram", "tiktok"}, nil)
	if len(shots) != 2 || shots[0].AspectRatio != "4:5" || shots[1].AspectRatio != "9:16" {
		t.Fatalf("platform shots = %#v", shots)
	}
	// Platforms + explicit aspect → aspect overrides default.
	shots = parseBriefToShots("hero", []string{"instagram"}, []string{"16:9"})
	if len(shots) != 1 || shots[0].AspectRatio != "16:9" {
		t.Fatalf("aspect override = %#v", shots)
	}
	// Platforms × multiple aspects → full N×M cross-product, mirroring
	// buildPackShots so plan output matches what pack produces.
	shots = parseBriefToShots("hero", []string{"instagram", "tiktok"}, []string{"16:9", "1:1"})
	if len(shots) != 4 {
		t.Fatalf("platform×aspect cross-product = %d shots, want 4: %#v", len(shots), shots)
	}

	// Aspects only → one shot per aspect, no platform.
	shots = parseBriefToShots("hero", nil, []string{"1:1", "9:16"})
	if len(shots) != 2 || shots[0].Platform != "" {
		t.Fatalf("aspect-only = %#v", shots)
	}
	// Neither → single shot.
	shots = parseBriefToShots("hero", nil, nil)
	if len(shots) != 1 {
		t.Fatalf("single = %#v", shots)
	}
}

func TestBriefHasCues(t *testing.T) {
	if !briefHasCues("a tiktok video") {
		t.Error("platform name should be a cue")
	}
	if !briefHasCues("render at 16:9") {
		t.Error("aspect ratio should be a cue")
	}
	if briefHasCues("just a vibe") {
		t.Error("no cues expected")
	}
}

func TestParseShotsFromText(t *testing.T) {
	text := "Here you go:\n[{\"prompt\":\"a\",\"platform\":\"instagram\"}]\nThanks!"
	shots, err := parseShotsFromText(text)
	if err != nil || len(shots) != 1 || shots[0].Prompt != "a" {
		t.Fatalf("parse = %v %#v", err, shots)
	}
	if _, err := parseShotsFromText("no array here"); err == nil {
		t.Fatalf("expected error for no array")
	}
	if _, err := parseShotsFromText(`[{"platform":"x"}]`); err == nil {
		t.Fatalf("expected error for shot missing prompt")
	}
}

func TestPlanBriefFallbackToParser(t *testing.T) {
	// planner=llm but no planner-model → deterministic fallback with warning.
	shots, used, warnings, err := planBrief(context.Background(), &rootFlags{}, "llm",
		planBriefFlags{platforms: []string{"instagram"}}, "helm brief")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if used != "fallback-parser" {
		t.Fatalf("planner_used = %q, want fallback-parser", used)
	}
	if len(warnings) == 0 {
		t.Fatalf("expected a warning about missing planner model")
	}
	if len(shots) != 1 {
		t.Fatalf("shots = %#v", shots)
	}
}

func TestLLMBriefToShotsWithMock(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/predictions/t1/result" {
			_, _ = w.Write([]byte(`{"data":{"id":"t1","status":"completed","outputs":["[{\"prompt\":\"shot one\",\"platform\":\"instagram\"}]"]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"id":"t1","status":"created"}}`))
	}))
	defer ts.Close()
	c := newTestClient(ts.URL)
	shots, err := llmBriefToShots(context.Background(), c, "text-model", "make a hero", false)
	if err != nil {
		t.Fatalf("llm planner: %v", err)
	}
	if len(shots) != 1 || shots[0].Prompt != "shot one" {
		t.Fatalf("shots = %#v", shots)
	}
}

func TestPriceShotStatuses(t *testing.T) {
	// 200 with price → priceOK.
	okSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"price":1.25}}`))
	}))
	defer okSrv.Close()
	price, status := priceShot(context.Background(), newTestClient(okSrv.URL), "m", nil)
	if status != priceOK || price != 1.25 {
		t.Fatalf("ok: status=%v price=%v", status, price)
	}

	// 200 without a price → priceUnavailable.
	unavail := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"message":"pricing unavailable"}}`))
	}))
	defer unavail.Close()
	_, status = priceShot(context.Background(), newTestClient(unavail.URL), "m", nil)
	if status != priceUnavailable {
		t.Fatalf("unavailable: status=%v", status)
	}

	// 5xx with no cache → priceError.
	errSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer errSrv.Close()
	t.Setenv("WAVESPEED_ARCHIVE_DB", t.TempDir()+"/none.db")
	_, status = priceShot(context.Background(), newTestClient(errSrv.URL), "m", nil)
	if status != priceError {
		t.Fatalf("5xx no-cache: status=%v", status)
	}
}

func TestPickModelForIntent(t *testing.T) {
	models := []byte(`{"data":[
		{"model_id":"wavespeed-ai/flux-dev","type":"image","description":"text to image"},
		{"model_id":"wavespeed-ai/hunyuan-video","type":"video","description":"text to video"}
	]}`)
	pick, _ := pickModelForIntent(models, "make a reel video")
	if pick != "wavespeed-ai/hunyuan-video" {
		t.Fatalf("video intent picked %q", pick)
	}
	pick, _ = pickModelForIntent(models, "a product image")
	if pick != "wavespeed-ai/flux-dev" {
		t.Fatalf("image intent picked %q", pick)
	}
}
