// pp:novel-static-reference
//
// Curated registry of Magnific (formerly Freepik) AI generation models.
//
// This is a static reference — Magnific's published pricing changes periodically
// and they do not yet expose a /v1/models endpoint that returns the full
// capability matrix. The credit_cost numbers are point-in-time estimates
// from the public docs as of 2026-05; treat them as relative cost guidance,
// not authoritative billing data. The empirical-stats join in `models stats`
// uses your local magnific_tasks rows for the truthful per-model numbers.

package cli

// MagnificModel captures one row of the curated registry.
type MagnificModel struct {
	Slug       string   `json:"slug"`
	Family     string   `json:"family"`
	Capability string   `json:"capability"`
	Endpoint   string   `json:"endpoint"`
	CreditCost float64  `json:"credit_cost"`
	Aspects    []string `json:"aspects,omitempty"`
	Notes      string   `json:"notes,omitempty"`
}

// magnificModels is the curated catalog. Add a row by adding a literal.
// Ordering is by capability, then by family/cost.
var magnificModels = []MagnificModel{
	// Text-to-image (Mystic family)
	{Slug: "mystic", Family: "mystic", Capability: "text-to-image", Endpoint: "/v1/ai/mystic", CreditCost: 12, Aspects: []string{"1:1", "16:9", "9:16", "4:3", "3:4", "2:3", "3:2"}, Notes: "Magnific's exclusive workflow for ultra-realistic images"},

	// Text-to-image (Flux family)
	{Slug: "flux-2-pro", Family: "flux", Capability: "text-to-image", Endpoint: "/v1/ai/text-to-image/flux-2-pro", CreditCost: 10, Aspects: []string{"1:1", "16:9", "9:16"}, Notes: "Professional-grade Flux 2, image-to-image supported"},
	{Slug: "flux-2-turbo", Family: "flux", Capability: "text-to-image", Endpoint: "/v1/ai/text-to-image/flux-2-turbo", CreditCost: 5, Aspects: []string{"1:1", "16:9", "9:16"}, Notes: "Speed-optimized Flux 2"},
	{Slug: "flux-2-klein", Family: "flux", Capability: "text-to-image", Endpoint: "/v1/ai/text-to-image/flux-2-klein", CreditCost: 2, Notes: "Sub-second Flux, up to 4 reference images"},
	{Slug: "flux-pro-v1-1", Family: "flux", Capability: "text-to-image", Endpoint: "/v1/ai/text-to-image/flux-pro-v1-1", CreditCost: 10, Notes: "Premium Flux 1.1"},
	{Slug: "flux-dev", Family: "flux", Capability: "text-to-image", Endpoint: "/v1/ai/text-to-image/flux-dev", CreditCost: 5, Notes: "High-detail Flux Dev"},
	{Slug: "flux-kontext-pro", Family: "flux", Capability: "text-to-image", Endpoint: "/v1/ai/text-to-image/flux-kontext-pro", CreditCost: 8, Notes: "Context-aware Flux with optional image input"},
	{Slug: "hyperflux", Family: "flux", Capability: "text-to-image", Endpoint: "/v1/ai/text-to-image/hyperflux", CreditCost: 3, Notes: "Ultra-fast Flux"},

	// Text-to-image (Seedream)
	{Slug: "seedream-v4", Family: "seedream", Capability: "text-to-image", Endpoint: "/v1/ai/text-to-image/seedream-v4", CreditCost: 6, Notes: "Fast, high-quality"},
	{Slug: "seedream-v4-5", Family: "seedream", Capability: "text-to-image", Endpoint: "/v1/ai/text-to-image/seedream-v4-5", CreditCost: 7, Notes: "Latest Seedream"},

	// Text-to-image (Z-Image, Runway, Imagen, Gemini)
	{Slug: "z-image-turbo", Family: "z-image", Capability: "text-to-image", Endpoint: "/v1/ai/text-to-image/z-image-turbo", CreditCost: 2, Notes: "Fast iteration"},
	{Slug: "runway", Family: "runway", Capability: "text-to-image", Endpoint: "/v1/ai/text-to-image/runway", CreditCost: 9, Notes: "RunWay text-to-image"},
	{Slug: "imagen3", Family: "imagen", Capability: "text-to-image", Endpoint: "/v1/ai/text-to-image/imagen3", CreditCost: 5, Notes: "Google Imagen 3"},
	{Slug: "imagen4-fast", Family: "imagen", Capability: "text-to-image", Endpoint: "/v1/ai/text-to-image/imagen4-fast", CreditCost: 4, Notes: "Imagen 4 Fast"},
	{Slug: "imagen4-ultra", Family: "imagen", Capability: "text-to-image", Endpoint: "/v1/ai/text-to-image/imagen4-ultra", CreditCost: 12, Notes: "Imagen 4 Ultra"},
	{Slug: "gemini-2-5-flash-image-preview", Family: "gemini", Capability: "text-to-image", Endpoint: "/v1/ai/gemini-2-5-flash-image-preview", CreditCost: 6, Notes: "Gemini 2.5 Flash Image"},

	// Image-to-video (Kling)
	{Slug: "kling-v2-1-pro", Family: "kling", Capability: "image-to-video", Endpoint: "/v1/ai/image-to-video/kling-v2-1-pro", CreditCost: 30, Notes: "High-quality Kling 2.1 Pro"},
	{Slug: "kling-v2-5-pro", Family: "kling", Capability: "image-to-video", Endpoint: "/v1/ai/image-to-video/kling-v2-5-pro", CreditCost: 35, Notes: "Enhanced motion quality"},
	{Slug: "kling-v2-6-pro", Family: "kling", Capability: "image-to-video", Endpoint: "/v1/ai/image-to-video/kling-v2-6-pro", CreditCost: 40, Notes: "Latest Kling with motion control"},

	// Image-to-video (MiniMax Hailuo)
	{Slug: "minimax-hailuo-02-1080p", Family: "minimax", Capability: "image-to-video", Endpoint: "/v1/ai/image-to-video/minimax-hailuo-02-1080p", CreditCost: 25, Notes: "Hailuo 02 1080p"},
	{Slug: "minimax-hailuo-2-3-1080p", Family: "minimax", Capability: "image-to-video", Endpoint: "/v1/ai/image-to-video/minimax-hailuo-2-3-1080p", CreditCost: 28, Notes: "Hailuo 2.3"},

	// Image-to-video (WAN, Seedance, PixVerse, RunWay, Veo, LTX)
	{Slug: "wan-v2-5-i2v-1080p", Family: "wan", Capability: "image-to-video", Endpoint: "/v1/ai/image-to-video/wan-v2-5-i2v-1080p", CreditCost: 22, Notes: "WAN 2.5 image-to-video"},
	{Slug: "wan-v2-6-1080p", Family: "wan", Capability: "image-to-video", Endpoint: "/v1/ai/image-to-video/wan-v2-6-1080p", CreditCost: 26, Notes: "WAN 2.6"},
	{Slug: "seedance-pro-1080p", Family: "seedance", Capability: "image-to-video", Endpoint: "/v1/ai/image-to-video/seedance-pro-1080p", CreditCost: 24, Notes: "ByteDance Seedance Pro"},
	{Slug: "runway-gen4-turbo", Family: "runway", Capability: "image-to-video", Endpoint: "/v1/ai/image-to-video/runway-gen4-turbo", CreditCost: 20, Notes: "RunWay Gen4 Turbo"},

	// Text-to-video
	{Slug: "wan-2-5-t2v-1080p", Family: "wan", Capability: "text-to-video", Endpoint: "/v1/ai/text-to-video/wan-2-5-t2v-1080p", CreditCost: 24, Notes: "WAN 2.5 text-to-video 1080p"},
	{Slug: "ltx-2-pro", Family: "ltx", Capability: "text-to-video", Endpoint: "/v1/ai/text-to-video/ltx-2-pro", CreditCost: 18, Notes: "LTX 2.0 Pro text-to-video"},

	// Image editing
	{Slug: "image-upscaler-creative", Family: "magnific", Capability: "image-upscaler", Endpoint: "/v1/ai/image-upscaler", CreditCost: 8, Notes: "Magnific Creative upscaler (the namesake feature)"},
	{Slug: "image-upscaler-precision", Family: "magnific", Capability: "image-upscaler", Endpoint: "/v1/ai/image-upscaler-precision", CreditCost: 6, Notes: "Faithful upscaler"},
	{Slug: "image-relight", Family: "magnific", Capability: "image-edit", Endpoint: "/v1/ai/image-relight", CreditCost: 5, Notes: "Lighting adjustment"},
	{Slug: "image-style-transfer", Family: "magnific", Capability: "image-edit", Endpoint: "/v1/ai/image-style-transfer", CreditCost: 4, Notes: "Style transfer"},
	{Slug: "remove-background", Family: "magnific", Capability: "image-edit", Endpoint: "/v1/ai/beta/remove-background", CreditCost: 1, Notes: "Background removal"},
	{Slug: "image-expand-flux-pro", Family: "magnific", Capability: "image-edit", Endpoint: "/v1/ai/image-expand/flux-pro", CreditCost: 5, Notes: "Expand canvas (Flux Pro)"},
	{Slug: "ideogram-image-edit", Family: "ideogram", Capability: "image-edit", Endpoint: "/v1/ai/ideogram-image-edit", CreditCost: 6, Notes: "Inpainting"},
	{Slug: "image-change-camera", Family: "magnific", Capability: "image-edit", Endpoint: "/v1/ai/image-change-camera", CreditCost: 5, Notes: "Perspective transform"},

	// Audio
	{Slug: "music-generation", Family: "elevenlabs", Capability: "audio", Endpoint: "/v1/ai/music-generation", CreditCost: 10, Notes: "ElevenLabs music"},
	{Slug: "sound-effects", Family: "magnific", Capability: "audio", Endpoint: "/v1/ai/sound-effects", CreditCost: 3, Notes: "Sound effects generation"},
	{Slug: "audio-isolation", Family: "magnific", Capability: "audio", Endpoint: "/v1/ai/audio-isolation", CreditCost: 4, Notes: "SAM Audio isolation"},

	// Utility
	{Slug: "image-to-prompt", Family: "magnific", Capability: "analyze", Endpoint: "/v1/ai/image-to-prompt", CreditCost: 1, Notes: "Reverse caption"},
	{Slug: "image-classifier", Family: "magnific", Capability: "analyze", Endpoint: "/v1/ai/classifier/image", CreditCost: 1, Notes: "AI-vs-real classifier"},
	{Slug: "improve-prompt", Family: "magnific", Capability: "analyze", Endpoint: "/v1/ai/improve-prompt", CreditCost: 1, Notes: "Prompt improver"},
}

// lookupModel returns a model by slug. Linear scan since the catalog is tiny
// and registry lookups are not on a hot path.
func lookupModel(slug string) *MagnificModel {
	for i := range magnificModels {
		if magnificModels[i].Slug == slug {
			return &magnificModels[i]
		}
	}
	return nil
}
