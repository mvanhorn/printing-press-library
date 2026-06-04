// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source live
//
// Hand-built request body for Suno's generation endpoint
// POST /api/generate/v2-web/. The upstream API requires the FULL body on
// every call — null/empty placeholders are mandatory, the server rejects a
// trimmed body. This file defines the typed body plus the model-key tables
// and the body builders shared by generate/describe/extend/cover/remaster.

package cli

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Generate model keys: CLI value -> mv wire value.
var sunoGenerateModels = map[string]string{
	"v5.5":  "chirp-fenix",
	"v5":    "chirp-crow",
	"v4.5+": "chirp-bluejay",
	"v4.5":  "chirp-auk",
	"v4":    "chirp-v4",
	"v3.5":  "chirp-v3-5",
	"v3":    "chirp-v3-0",
	"v2":    "chirp-v2-xxl-alpha",
}

// sunoGenerateModelOrder preserves a stable display order for the models table.
var sunoGenerateModelOrder = []string{"v5.5", "v5", "v4.5+", "v4.5", "v4", "v3.5", "v3", "v2"}

// Remaster model keys: CLI value -> mv wire value.
var sunoRemasterModels = map[string]string{
	"v5.5":  "chirp-flounder",
	"v5":    "chirp-carp",
	"v4.5+": "chirp-bass",
}

var sunoRemasterModelOrder = []string{"v5.5", "v5", "v4.5+"}

const defaultGenerateModel = "v5.5"

// sunoControlSliders carries the optional weirdness/style-weight knobs. Sent
// only when at least one slider is set; otherwise control_sliders stays nil.
type sunoControlSliders struct {
	WeirdnessConstraint float64 `json:"weirdness_constraint"`
	StyleWeight         float64 `json:"style_weight"`
}

// sunoGenerateMetadata is the metadata sub-object of the generation body.
type sunoGenerateMetadata struct {
	WebClientPathname          string              `json:"web_client_pathname"`
	IsMaxMode                  bool                `json:"is_max_mode"`
	IsMumble                   bool                `json:"is_mumble"`
	CreateMode                 string              `json:"create_mode"`
	UserTier                   string              `json:"user_tier"`
	CreateSessionToken         string              `json:"create_session_token"`
	DisableVolumeNormalization bool                `json:"disable_volume_normalization"`
	ControlSliders             *sunoControlSliders `json:"control_sliders"`
	// Variation is the advanced "how different from the prompt" preset
	// (high/normal/subtle). Best-effort: omitted entirely unless --variation
	// is set, so the default body stays byte-identical to the known-good flow.
	Variation *string `json:"variation,omitempty"`
}

// sunoGenerateBody is the full POST /api/generate/v2-web/ request body. Every
// field is always serialized; *string / *float64 fields emit JSON null when
// nil, which the upstream API requires as explicit placeholders for the
// optional reference fields (cover_clip_id, persona_id, continue_*). Title and
// Tags are the exception: the API requires them to be JSON strings (a null
// title is rejected before the captcha gate), so they are always non-nil — see
// alwaysStrPtr.
type sunoGenerateBody struct {
	Token                  *string              `json:"token"`
	GenerationType         string               `json:"generation_type"`
	Title                  *string              `json:"title"`
	Tags                   *string              `json:"tags"`
	NegativeTags           string               `json:"negative_tags"`
	Mv                     string               `json:"mv"`
	Prompt                 string               `json:"prompt"`
	MakeInstrumental       bool                 `json:"make_instrumental"`
	Metadata               sunoGenerateMetadata `json:"metadata"`
	OverrideFields         []string             `json:"override_fields"`
	CoverClipID            *string              `json:"cover_clip_id"`
	CoverStartS            *float64             `json:"cover_start_s"`
	CoverEndS              *float64             `json:"cover_end_s"`
	PersonaID              *string              `json:"persona_id"`
	ContinueClipID         *string              `json:"continue_clip_id"`
	ContinuedAlignedPrompt *string              `json:"continued_aligned_prompt"`
	ContinueAt             *float64             `json:"continue_at"`
	TransactionUUID        string               `json:"transaction_uuid"`
}

// strPtr returns a pointer to s, or nil when s is empty. Used for the optional
// reference fields (cover_clip_id, persona_id, continue_clip_id) that the
// upstream API genuinely wants as JSON null when absent.
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// alwaysStrPtr returns a pointer to s even when s is empty, so the field
// serializes as a JSON string ("") rather than null. title and tags must use
// this: the upstream API rejects a null title with
// 422 [{"loc":["body","params","title"],"msg":"Input should be a valid string"}]
// before the captcha gate is ever evaluated, which broke every inspiration-mode
// (describe) request that did not pass an explicit --title. The browser sends ""
// for both fields in inspiration mode.
func alwaysStrPtr(s string) *string {
	return &s
}

// generateInput collects the resolved, validated parameters that the body
// builder turns into a sunoGenerateBody. Empty/zero values map to nil/absent.
type generateInput struct {
	createMode   string // "custom" | "inspiration" | "cover" | "remaster"
	mv           string // wire model key
	title        string
	tags         string
	prompt       string // lyrics for custom, description for inspiration
	instrumental bool
	personaID    string
	token        string // hCaptcha token; empty -> nil

	coverClipID    string
	continueClipID string
	continueAt     *float64

	weirdness      *float64 // 0..1
	styleInfluence *float64 // 0..1
	variation      *string  // "high" | "normal" | "subtle"; nil when unset
}

// validVariations are the accepted --variation presets.
var validVariations = map[string]bool{"high": true, "normal": true, "subtle": true}

// variationPtr validates a --variation value and returns a pointer to the
// lowercased value, or nil when empty. Returns an error for unrecognized input.
func variationPtr(v string) (*string, error) {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return nil, nil
	}
	if !validVariations[v] {
		return nil, fmt.Errorf("invalid --variation %q: must be high, normal, or subtle", v)
	}
	return &v, nil
}

// buildGenerateBody assembles the full request body from validated input. A
// fresh create_session_token and transaction_uuid (UUID v4) are minted on
// every call.
func buildGenerateBody(in generateInput) sunoGenerateBody {
	meta := sunoGenerateMetadata{
		WebClientPathname:          "/create",
		IsMaxMode:                  false,
		IsMumble:                   false,
		CreateMode:                 in.createMode,
		UserTier:                   "",
		CreateSessionToken:         uuid.NewString(),
		DisableVolumeNormalization: false,
	}
	if in.weirdness != nil || in.styleInfluence != nil {
		sliders := &sunoControlSliders{}
		if in.weirdness != nil {
			sliders.WeirdnessConstraint = *in.weirdness
		}
		if in.styleInfluence != nil {
			sliders.StyleWeight = *in.styleInfluence
		}
		meta.ControlSliders = sliders
	}
	meta.Variation = in.variation

	return sunoGenerateBody{
		Token:            strPtr(in.token),
		GenerationType:   "TEXT",
		Title:            alwaysStrPtr(in.title),
		Tags:             alwaysStrPtr(in.tags),
		NegativeTags:     "",
		Mv:               in.mv,
		Prompt:           in.prompt,
		MakeInstrumental: in.instrumental,
		Metadata:         meta,
		OverrideFields:   []string{},
		CoverClipID:      strPtr(in.coverClipID),
		PersonaID:        strPtr(in.personaID),
		ContinueClipID:   strPtr(in.continueClipID),
		ContinueAt:       in.continueAt,
		TransactionUUID:  uuid.NewString(),
	}
}

// appendTag folds an additional descriptor (e.g. "male vocals") into a tags
// string, comma-separating when tags is non-empty.
func appendTag(tags, extra string) string {
	if extra == "" {
		return tags
	}
	if strings.TrimSpace(tags) == "" {
		return extra
	}
	return tags + ", " + extra
}

// vocalTag maps a --vocal value to the tag descriptor appended to tags.
// Returns "" for an empty/unknown value (caller validates).
func vocalTag(vocal string) string {
	switch strings.ToLower(strings.TrimSpace(vocal)) {
	case "male":
		return "male vocals"
	case "female":
		return "female vocals"
	default:
		return ""
	}
}
