// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

func TestBuildGenerateBody_Custom(t *testing.T) {
	body := buildGenerateBody(generateInput{
		createMode: "custom",
		mv:         "chirp-fenix",
		title:      "Night Drive",
		tags:       "synthwave",
		prompt:     "la la la",
	})
	if body.Metadata.CreateMode != "custom" {
		t.Errorf("create_mode = %q, want custom", body.Metadata.CreateMode)
	}
	if body.Mv != "chirp-fenix" {
		t.Errorf("mv = %q, want chirp-fenix", body.Mv)
	}
	if body.CoverClipID != nil {
		t.Errorf("cover_clip_id = %v, want nil", *body.CoverClipID)
	}
	if body.ContinueClipID != nil {
		t.Errorf("continue_clip_id = %v, want nil", *body.ContinueClipID)
	}
	if body.NegativeTags != "" {
		t.Errorf("negative_tags = %q, want empty string", body.NegativeTags)
	}
	if body.Prompt != "la la la" {
		t.Errorf("prompt = %q, want la la la", body.Prompt)
	}
	if body.GenerationType != "TEXT" {
		t.Errorf("generation_type = %q, want TEXT", body.GenerationType)
	}
	if body.OverrideFields == nil {
		t.Errorf("override_fields must be present (empty slice), got nil")
	}
	if body.TransactionUUID == "" || body.Metadata.CreateSessionToken == "" {
		t.Errorf("transaction_uuid and create_session_token must be populated")
	}
	if body.Metadata.ControlSliders != nil {
		t.Errorf("control_sliders should be nil when no sliders set")
	}
}

func TestBuildGenerateBody_NegativeTags(t *testing.T) {
	body := buildGenerateBody(generateInput{
		createMode:   "custom",
		mv:           "chirp-fenix",
		tags:         "synthwave",
		negativeTags: "country, banjo",
		prompt:       "la la la",
	})
	if body.NegativeTags != "country, banjo" {
		t.Errorf("negative_tags = %q, want %q", body.NegativeTags, "country, banjo")
	}
}

func TestBuildGenerateBody_WebSchemaPlaceholders(t *testing.T) {
	body := buildGenerateBody(generateInput{
		createMode: "custom",
		mv:         "chirp-fenix",
		prompt:     "la la la",
	})
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal generate body: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal generate body: %v", err)
	}
	for _, key := range []string{
		"user_uploaded_images_b64",
		"artist_clip_id",
		"artist_start_s",
		"artist_end_s",
	} {
		value, ok := got[key]
		if !ok {
			t.Fatalf("generate body missing web schema placeholder %q", key)
		}
		if value != nil {
			t.Fatalf("generate body placeholder %q = %v, want null", key, value)
		}
	}
}

func TestBuildGenerateBody_Cover(t *testing.T) {
	body := buildGenerateBody(generateInput{
		createMode:  "cover",
		mv:          "chirp-fenix",
		title:       "Acoustic",
		coverClipID: "clip-123",
	})
	if body.Metadata.CreateMode != "cover" {
		t.Errorf("create_mode = %q, want cover", body.Metadata.CreateMode)
	}
	if body.CoverClipID == nil || *body.CoverClipID != "clip-123" {
		t.Errorf("cover_clip_id = %v, want clip-123", body.CoverClipID)
	}
	if body.ContinueClipID != nil {
		t.Errorf("continue_clip_id should be nil for cover")
	}
}

func TestBuildGenerateBody_Extend(t *testing.T) {
	at := 120.0
	body := buildGenerateBody(generateInput{
		createMode:     "custom",
		mv:             "chirp-fenix",
		continueClipID: "clip-xyz",
		continueAt:     &at,
	})
	if body.ContinueClipID == nil || *body.ContinueClipID != "clip-xyz" {
		t.Errorf("continue_clip_id = %v, want clip-xyz", body.ContinueClipID)
	}
	if body.ContinueAt == nil || *body.ContinueAt != 120.0 {
		t.Errorf("continue_at = %v, want 120", body.ContinueAt)
	}
	if body.CoverClipID != nil {
		t.Errorf("cover_clip_id should be nil for extend")
	}
}

func TestBuildGenerateBody_Remaster(t *testing.T) {
	body := buildGenerateBody(generateInput{
		createMode:  "remaster",
		mv:          "chirp-flounder",
		coverClipID: "clip-rm",
	})
	if body.Metadata.CreateMode != "remaster" {
		t.Errorf("create_mode = %q, want remaster", body.Metadata.CreateMode)
	}
	if body.Mv != "chirp-flounder" {
		t.Errorf("mv = %q, want chirp-flounder (remaster key)", body.Mv)
	}
	if body.CoverClipID == nil || *body.CoverClipID != "clip-rm" {
		t.Errorf("cover_clip_id = %v, want clip-rm", body.CoverClipID)
	}
}

func TestBuildGenerateBody_Sliders(t *testing.T) {
	w := 0.5
	body := buildGenerateBody(generateInput{
		createMode: "custom",
		mv:         "chirp-fenix",
		weirdness:  &w,
	})
	if body.Metadata.ControlSliders == nil {
		t.Fatalf("control_sliders should be set when weirdness provided")
	}
	if body.Metadata.ControlSliders.WeirdnessConstraint != 0.5 {
		t.Errorf("weirdness_constraint = %v, want 0.5", body.Metadata.ControlSliders.WeirdnessConstraint)
	}
}

func TestBuildGenerateBody_Variation(t *testing.T) {
	// Unset: the variation field must be omitted entirely so the default
	// body stays byte-identical to the known-good flow.
	plain := buildGenerateBody(generateInput{createMode: "custom", mv: "chirp-fenix"})
	if plain.Metadata.Variation != nil {
		t.Errorf("variation should be nil when unset, got %q", *plain.Metadata.Variation)
	}

	v := "high"
	body := buildGenerateBody(generateInput{createMode: "custom", mv: "chirp-fenix", variation: &v})
	if body.Metadata.Variation == nil || *body.Metadata.Variation != "high" {
		t.Errorf("variation = %v, want high", body.Metadata.Variation)
	}
}

func TestVariationPtr(t *testing.T) {
	if p, err := variationPtr(""); err != nil || p != nil {
		t.Errorf("empty variation = (%v, %v), want (nil, nil)", p, err)
	}
	for _, v := range []string{"high", "Normal", " subtle "} {
		p, err := variationPtr(v)
		if err != nil || p == nil {
			t.Errorf("variationPtr(%q) = (%v, %v), want valid pointer", v, p, err)
		}
	}
	if _, err := variationPtr("loud"); err == nil {
		t.Errorf("expected error for invalid variation 'loud'")
	}
}

func TestResolveModel(t *testing.T) {
	mv, err := resolveModel("", sunoGenerateModels, sunoGenerateModelOrder)
	if err != nil || mv != "chirp-fenix" {
		t.Errorf("default model = %q (err %v), want chirp-fenix", mv, err)
	}
	mv, err = resolveModel("v5", sunoGenerateModels, sunoGenerateModelOrder)
	if err != nil || mv != "chirp-crow" {
		t.Errorf("v5 = %q (err %v), want chirp-crow", mv, err)
	}
	if _, err := resolveModel("v9", sunoGenerateModels, sunoGenerateModelOrder); err == nil {
		t.Errorf("expected error for unknown model v9")
	}
	mv, err = resolveModel("v5.5", sunoRemasterModels, sunoRemasterModelOrder)
	if err != nil || mv != "chirp-flounder" {
		t.Errorf("remaster v5.5 = %q (err %v), want chirp-flounder", mv, err)
	}
}
