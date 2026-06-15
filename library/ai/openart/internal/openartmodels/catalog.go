// Curated catalog of OpenArt video / image / audio models.
//
// OpenArt does not expose a public model-listing endpoint; the suite UI
// embeds the catalog in static JS bundles. This file mirrors the
// observed catalog (browser-sniff capture from 2026-05-13).
//
// Refresh by re-running the picker scan in Phase 1.7 of the press SKILL
// when OpenArt adds or removes models.
package openartmodels

// FormType is the suffix of an OpenArt capability_id (e.g. "text2video").
type FormType string

const (
	FormText2Video  FormType = "text2video"
	FormImage2Video FormType = "image2video"
	FormText2Image  FormType = "text2image"
	FormImage2Image FormType = "image2image"
	FormLipSync     FormType = "lipsync"
	FormMotionSync  FormType = "motionsync"
	FormEditImage   FormType = "edit-image"
	FormEditVideo   FormType = "edit-video"
)

// Family is the high-level capability family.
type Family string

const (
	FamilyVideo Family = "video"
	FamilyImage Family = "image"
	FamilyAudio Family = "audio"
)

// CapabilityPattern templates for the requestFormId that gets POSTed to
// /forms/creations/. Video models use "<slug>:<form-type>" (the slug-and-form
// pattern); image models use a fixed prefix observed via browser-sniff:
// "create-image:reference:<slug>". Send these distinct patterns; the
// internal capabilityId field on the resource record (e.g.
// "nano-banana:text2image") is NOT what gets POSTed — that's the resolved
// internal id in OpenArt's records, and posting it 404s.
//
// Verified via 2026-05-13 browser-sniff against nano-banana.
const (
	CapabilityPatternImage = "create-image:reference:%s"
)

// Model describes one OpenArt model entry.
type Model struct {
	// Slug is the URL-safe identifier (e.g. "byte-plus-seedance-2", "kling2-6").
	Slug string
	// DisplayName is how the OpenArt UI labels it.
	DisplayName string
	// Vendor is the underlying inference provider.
	Vendor string
	// Family is video / image / audio.
	Family Family
	// Description is a one-liner from the suite UI.
	Description string
	// SupportedForms enumerates the form-types the model exposes
	// (text2video, image2video, etc.).
	SupportedForms []FormType
	// Resolutions lists output resolutions the model supports
	// (e.g. "480p", "720p", "1080p", "4K").
	Resolutions []string
	// DurationMinSec / DurationMaxSec bound the duration range in seconds.
	DurationMinSec int
	DurationMaxSec int
	// HasAudio is true when the model can emit audio.
	HasAudio bool
	// SupportsReference is true when the model accepts visual references.
	SupportsReference bool
	// SupportsStartEndFrame is true when the model accepts start/end keyframes.
	SupportsStartEndFrame bool
	// SupportsMultiShots is true when the model produces multi-shot output.
	SupportsMultiShots bool
	// CreditsPerVideoDefault is the credits cost for one generation at the
	// model's default settings. For video, that's typically 720p, mid-range
	// duration. For image, it's the per-image cost at the default
	// resolution/aspect-ratio. Used by `cost estimate` and `models cheapest`
	// for ranking and forecasting.
	// 0 means "unknown — verify via /suite/api/topaz/estimate".
	CreditsPerVideoDefault int
	// Tier is "fast" for low-latency models; empty otherwise.
	Tier string
	// Recommended is true when the model appears in the picker's "Recommended" group.
	Recommended bool
	// PixelResolutions lists output sizes for image-family models
	// (e.g. "1024x1024", "1536x1024"). Empty for video models.
	PixelResolutions []string
	// AspectRatios lists supported aspect-ratio strings for image-family
	// models (e.g. "1:1", "16:9", "9:16"). Empty for video models which use
	// the standard Resolutions field.
	AspectRatios []string
	// Experimental marks an entry whose submit contract has NOT been
	// end-to-end verified via browser-sniff. The CLI prints a warning when
	// a user submits against an experimental model — it may 404 or require
	// fields the catalog does not yet know about. Verified models have
	// Experimental=false.
	Experimental bool
}

// Capability returns the requestFormId that should be POSTed to
// /suite/api/forms/creations/<id>. Family-aware: video uses
// "<slug>:<form-type>", image uses "create-image:reference:<slug>".
//
// The form parameter is honored for video (text2video / image2video / etc.)
// but is ignored for image — image models route through a single fixed
// requestFormId pattern regardless of whether the user asked for
// text2image or image2image (which is encoded in body fields like
// `visualReferences`, not in the URL).
func (m Model) Capability(form FormType) string {
	if m.Family == FamilyImage {
		return "create-image:reference:" + m.Slug
	}
	return m.Slug + ":" + string(form)
}

// Catalog is the curated list of OpenArt models. Refresh from the suite
// model picker when the upstream catalog changes.
var Catalog = []Model{
	{
		Slug:                   "byte-plus-seedance-2",
		DisplayName:            "Seedance 2.0",
		Vendor:                 "BytePlus",
		Family:                 FamilyVideo,
		Description:            "Cinematic videos with audio & multi-shots",
		SupportedForms:         []FormType{FormText2Video, FormImage2Video},
		Resolutions:            []string{"480p", "720p", "1080p"},
		DurationMinSec:         4,
		DurationMaxSec:         15,
		HasAudio:               true,
		SupportsReference:      true,
		SupportsStartEndFrame:  true,
		SupportsMultiShots:     true,
		CreditsPerVideoDefault: 800,
		Recommended:            true,
	},
	{
		Slug:                   "kling3-omni",
		DisplayName:            "Kling 3.0 Omni",
		Vendor:                 "Kling",
		Family:                 FamilyVideo,
		Description:            "Enhanced multimodal references",
		SupportedForms:         []FormType{FormText2Video, FormImage2Video},
		Resolutions:            []string{"720p", "1080p", "4K"},
		DurationMinSec:         3,
		DurationMaxSec:         15,
		HasAudio:               true,
		SupportsReference:      true,
		SupportsMultiShots:     true,
		CreditsPerVideoDefault: 600,
		Recommended:            true,
	},
	{
		Slug:                   "kling3",
		DisplayName:            "Kling 3.0",
		Vendor:                 "Kling",
		Family:                 FamilyVideo,
		Description:            "Enhanced audio, consistency & multi-shots",
		SupportedForms:         []FormType{FormText2Video, FormImage2Video},
		Resolutions:            []string{"720p", "1080p", "4K"},
		DurationMinSec:         3,
		DurationMaxSec:         15,
		HasAudio:               true,
		SupportsStartEndFrame:  true,
		SupportsMultiShots:     true,
		CreditsPerVideoDefault: 500,
		Recommended:            true,
	},
	{
		Slug:                   "happyhorse",
		DisplayName:            "HappyHorse",
		Vendor:                 "HappyHorse",
		Family:                 FamilyVideo,
		Description:            "Cinematic videos with multi-shot narrative & native audio",
		SupportedForms:         []FormType{FormImage2Video},
		Resolutions:            []string{"720p", "1080p"},
		DurationMinSec:         3,
		DurationMaxSec:         15,
		HasAudio:               true,
		CreditsPerVideoDefault: 400,
	},
	{
		Slug:                   "wan2-7",
		DisplayName:            "Wan 2.7",
		Vendor:                 "Wan",
		Family:                 FamilyVideo,
		Description:            "Cinematic videos with audio & start/end frame control",
		SupportedForms:         []FormType{FormText2Video, FormImage2Video},
		Resolutions:            []string{"720p", "1080p"},
		DurationMinSec:         2,
		DurationMaxSec:         15,
		HasAudio:               true,
		SupportsStartEndFrame:  true,
		CreditsPerVideoDefault: 350,
	},
	{
		Slug:                   "veo3-1",
		DisplayName:            "Veo 3.1",
		Vendor:                 "Google DeepMind",
		Family:                 FamilyVideo,
		Description:            "High-fidelity videos with audio & 4K",
		SupportedForms:         []FormType{FormText2Video, FormImage2Video},
		Resolutions:            []string{"720p", "1080p", "4K"},
		DurationMinSec:         4,
		DurationMaxSec:         8,
		HasAudio:               true,
		SupportsReference:      true,
		SupportsStartEndFrame:  true,
		CreditsPerVideoDefault: 1200,
	},
	{
		Slug:                   "byte-plus-seedance-1-5-pro",
		DisplayName:            "Seedance 1.5 Pro",
		Vendor:                 "BytePlus",
		Family:                 FamilyVideo,
		Description:            "Cinematic videos with audio & multi-shots (older Seedance line)",
		SupportedForms:         []FormType{FormText2Video, FormImage2Video},
		Resolutions:            []string{"480p", "720p", "1080p"},
		DurationMinSec:         4,
		DurationMaxSec:         12,
		HasAudio:               true,
		SupportsStartEndFrame:  true,
		SupportsMultiShots:     true,
		CreditsPerVideoDefault: 400,
	},
	{
		Slug:                   "grok-imagine",
		DisplayName:            "Grok Imagine",
		Vendor:                 "xAI",
		Family:                 FamilyVideo,
		Description:            "Fast generation of cinematic videos",
		SupportedForms:         []FormType{FormText2Video, FormImage2Video},
		Resolutions:            []string{"480p", "720p"},
		DurationMinSec:         1,
		DurationMaxSec:         15,
		HasAudio:               true,
		CreditsPerVideoDefault: 150,
		Tier:                   "fast",
	},
	{
		Slug:                   "kling2-6",
		DisplayName:            "Kling 2.6",
		Vendor:                 "Kling",
		Family:                 FamilyVideo,
		Description:            "Cinematic videos with audio & voice",
		SupportedForms:         []FormType{FormImage2Video},
		Resolutions:            []string{"720p", "1080p"},
		DurationMinSec:         5,
		DurationMaxSec:         10,
		HasAudio:               true,
		SupportsStartEndFrame:  true,
		CreditsPerVideoDefault: 100,
	},
	{
		Slug:                   "wan2-6",
		DisplayName:            "Wan 2.6",
		Vendor:                 "Wan",
		Family:                 FamilyVideo,
		Description:            "Cinematic videos with audio & multi-shots",
		SupportedForms:         []FormType{FormText2Video, FormImage2Video},
		Resolutions:            []string{"720p", "1080p"},
		DurationMinSec:         5,
		DurationMaxSec:         15,
		HasAudio:               true,
		SupportsMultiShots:     true,
		CreditsPerVideoDefault: 250,
	},

	// Image-family models. The submit-contract for `nano-banana` was
	// verified end-to-end via browser-sniff on 2026-05-13 (request body,
	// response envelope, polling URL, completion shape). Other entries are
	// marked Experimental: true — they share the same body shape in the JS
	// bundle but were not individually exercised against the live API.
	// Submitting against an Experimental model surfaces a warning in the
	// CLI; refresh-and-verify by running an actual gen and removing the
	// flag.
	{
		Slug:                   "nano-banana",
		DisplayName:            "Nano Banana",
		Vendor:                 "Nano Banana",
		Family:                 FamilyImage,
		Description:            "Fast image generation with reference support",
		SupportedForms:         []FormType{FormText2Image, FormImage2Image},
		PixelResolutions:       []string{"1024x1024", "1024x1536", "1536x1024"},
		AspectRatios:           []string{"1:1", "9:16", "16:9", "3:4", "4:3"},
		SupportsReference:      true,
		CreditsPerVideoDefault: 50, // observed: nano-banana submit consumed ~50 credits
		Tier:                   "fast",
	},
	{
		Slug:                   "nano-banana-pro",
		DisplayName:            "Nano Banana Pro",
		Vendor:                 "Nano Banana",
		Family:                 FamilyImage,
		Description:            "Higher-quality variant of Nano Banana",
		SupportedForms:         []FormType{FormText2Image, FormImage2Image},
		PixelResolutions:       []string{"1024x1024", "1536x1024", "1024x1536"},
		AspectRatios:           []string{"1:1", "9:16", "16:9"},
		SupportsReference:      true,
		CreditsPerVideoDefault: 120,
		Experimental:           true,
	},
	{
		Slug:                   "gpt-image-2",
		DisplayName:            "GPT Image 2",
		Vendor:                 "OpenAI",
		Family:                 FamilyImage,
		Description:            "OpenAI's image-2 model with reference support",
		SupportedForms:         []FormType{FormText2Image, FormImage2Image},
		PixelResolutions:       []string{"1024x1024", "1024x1536", "1536x1024"},
		AspectRatios:           []string{"1:1", "9:16", "16:9", "3:4", "4:3"},
		SupportsReference:      true,
		CreditsPerVideoDefault: 200,
		Experimental:           true,
	},
	{
		Slug:                   "gpt-image-1-5",
		DisplayName:            "GPT Image 1.5",
		Vendor:                 "OpenAI",
		Family:                 FamilyImage,
		Description:            "OpenAI's image-1.5 model",
		SupportedForms:         []FormType{FormText2Image, FormImage2Image},
		PixelResolutions:       []string{"1024x1024", "1024x1536", "1536x1024"},
		AspectRatios:           []string{"1:1", "16:9", "9:16"},
		SupportsReference:      true,
		CreditsPerVideoDefault: 150,
		Experimental:           true,
	},
	{
		Slug:                   "flux-2-pro",
		DisplayName:            "FLUX 2 Pro",
		Vendor:                 "Black Forest Labs",
		Family:                 FamilyImage,
		Description:            "FLUX 2 Pro from Black Forest Labs",
		SupportedForms:         []FormType{FormText2Image},
		PixelResolutions:       []string{"1024x1024", "1024x1536", "1536x1024", "2048x2048"},
		AspectRatios:           []string{"1:1", "16:9", "9:16", "3:4", "4:3"},
		CreditsPerVideoDefault: 100,
		Experimental:           true,
	},
	{
		Slug:                   "flux-1-1-pro",
		DisplayName:            "FLUX 1.1 Pro",
		Vendor:                 "Black Forest Labs",
		Family:                 FamilyImage,
		Description:            "FLUX 1.1 Pro from Black Forest Labs",
		SupportedForms:         []FormType{FormText2Image},
		PixelResolutions:       []string{"1024x1024", "1024x1536", "1536x1024"},
		AspectRatios:           []string{"1:1", "16:9", "9:16"},
		CreditsPerVideoDefault: 80,
		Experimental:           true,
	},
	{
		Slug:                   "byte-plus-seedream-4",
		DisplayName:            "Seedream 4",
		Vendor:                 "BytePlus",
		Family:                 FamilyImage,
		Description:            "BytePlus Seedream 4 image model",
		SupportedForms:         []FormType{FormText2Image, FormImage2Image},
		PixelResolutions:       []string{"1024x1024", "1024x1536", "1536x1024"},
		AspectRatios:           []string{"1:1", "16:9", "9:16", "3:4", "4:3"},
		SupportsReference:      true,
		CreditsPerVideoDefault: 100,
		Experimental:           true,
	},
	{
		Slug:                   "byte-plus-seedream-4-5",
		DisplayName:            "Seedream 4.5",
		Vendor:                 "BytePlus",
		Family:                 FamilyImage,
		Description:            "BytePlus Seedream 4.5 image model",
		SupportedForms:         []FormType{FormText2Image, FormImage2Image},
		PixelResolutions:       []string{"1024x1024", "1024x1536", "1536x1024"},
		AspectRatios:           []string{"1:1", "16:9", "9:16"},
		SupportsReference:      true,
		CreditsPerVideoDefault: 130,
		Experimental:           true,
	},
	{
		Slug:                   "google-imagen-4",
		DisplayName:            "Imagen 4",
		Vendor:                 "Google DeepMind",
		Family:                 FamilyImage,
		Description:            "Google Imagen 4 image model",
		SupportedForms:         []FormType{FormText2Image},
		PixelResolutions:       []string{"1024x1024", "1024x1536", "1536x1024"},
		AspectRatios:           []string{"1:1", "16:9", "9:16", "3:4", "4:3"},
		CreditsPerVideoDefault: 250,
		Experimental:           true,
	},
	{
		Slug:                   "qwen-image-max",
		DisplayName:            "Qwen Image Max",
		Vendor:                 "Alibaba Qwen",
		Family:                 FamilyImage,
		Description:            "Alibaba Qwen Image Max model",
		SupportedForms:         []FormType{FormText2Image},
		PixelResolutions:       []string{"1024x1024", "1024x1536", "1536x1024"},
		AspectRatios:           []string{"1:1", "16:9", "9:16"},
		CreditsPerVideoDefault: 90,
		Experimental:           true,
	},
}

// FindBySlug returns the model with the given slug, or nil.
func FindBySlug(slug string) *Model {
	for i := range Catalog {
		if Catalog[i].Slug == slug {
			return &Catalog[i]
		}
	}
	return nil
}

// FindByDisplayName does a case-insensitive match against DisplayName.
func FindByDisplayName(name string) *Model {
	if name == "" {
		return nil
	}
	for i := range Catalog {
		if equalFold(Catalog[i].DisplayName, name) {
			return &Catalog[i]
		}
	}
	return nil
}

// Resolve accepts either a slug or a display-name shorthand and returns
// the matching model. Recognised shorthands include "seedance2",
// "seedance-2", "seedance 2", "kling 2.6", "grok", "veo3", and so on.
func Resolve(input string) *Model {
	if input == "" {
		return nil
	}
	if m := FindBySlug(input); m != nil {
		return m
	}
	if m := FindByDisplayName(input); m != nil {
		return m
	}
	norm := normalize(input)
	for i := range Catalog {
		if normalize(Catalog[i].Slug) == norm || normalize(Catalog[i].DisplayName) == norm {
			return &Catalog[i]
		}
	}
	// Looser match: substring on slug or display name.
	for i := range Catalog {
		if hasNormalized(Catalog[i].Slug, norm) || hasNormalized(Catalog[i].DisplayName, norm) {
			return &Catalog[i]
		}
	}
	return nil
}

// FilterVideo returns the subset of Catalog with Family==FamilyVideo.
func FilterVideo() []Model {
	out := make([]Model, 0, len(Catalog))
	for _, m := range Catalog {
		if m.Family == FamilyVideo {
			out = append(out, m)
		}
	}
	return out
}

// FilterImage returns the subset of Catalog with Family==FamilyImage.
func FilterImage() []Model {
	out := make([]Model, 0, len(Catalog))
	for _, m := range Catalog {
		if m.Family == FamilyImage {
			out = append(out, m)
		}
	}
	return out
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func normalize(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z':
			out = append(out, c+32)
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			out = append(out, c)
		}
	}
	return string(out)
}

func hasNormalized(haystack, needle string) bool {
	h := normalize(haystack)
	if h == "" || needle == "" {
		return false
	}
	for i := 0; i+len(needle) <= len(h); i++ {
		if h[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// EstimateCredits returns a credits-per-video estimate for the requested
// duration / count / resolution. The catalog's CreditsPerVideoDefault is
// pegged at the model's default settings; this scales linearly on
// duration and applies a 1.5× multiplier for 1080p and 2.0× for 4K.
//
// The result is a hint, not a guarantee. The authoritative cost is
// returned by /suite/api/topaz/estimate when available.
func (m Model) EstimateCredits(durationSec, count int, resolution string) int {
	if count <= 0 {
		count = 1
	}
	base := m.CreditsPerVideoDefault
	if base == 0 {
		return 0
	}
	// Image-family branch: no duration scaling; resolution multiplier is
	// pixel-area-based. Default is 1024x1024; 1024x1536/1536x1024 = 1.5×;
	// 2048x2048 = 4×. The video-shape resolution strings ("720p", "1080p",
	// "4K") are video-only.
	if m.Family == FamilyImage {
		mult := 1
		switch resolution {
		case "1024x1536", "1536x1024":
			mult = 3
		case "2048x2048":
			mult = 8
		default:
			// Unknown / 1024x1024 / empty falls through at 1×.
			return base * count
		}
		return base * mult / 2 * count
	}
	defaultDuration := defaultDurationFor(m)
	if defaultDuration == 0 {
		defaultDuration = 5
	}
	if durationSec <= 0 {
		durationSec = defaultDuration
	}
	// Linear scale on duration (some upstream models price differently;
	// users should round-trip through topaz/estimate when precision matters).
	per := base * durationSec / defaultDuration
	switch resolution {
	case "1080p":
		per = per * 3 / 2
	case "4K", "4k":
		per = per * 2
	}
	return per * count
}

func defaultDurationFor(m Model) int {
	if m.DurationMinSec == 0 && m.DurationMaxSec == 0 {
		return 5
	}
	mid := (m.DurationMinSec + m.DurationMaxSec) / 2
	if mid < m.DurationMinSec {
		return m.DurationMinSec
	}
	return mid
}
