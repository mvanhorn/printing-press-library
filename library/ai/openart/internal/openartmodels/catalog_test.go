package openartmodels

import "testing"

func TestResolveBySlug(t *testing.T) {
	m := Resolve("byte-plus-seedance-2")
	if m == nil {
		t.Fatal("expected to find byte-plus-seedance-2 by slug")
	}
	if m.DisplayName != "Seedance 2.0" {
		t.Fatalf("unexpected display name: %q", m.DisplayName)
	}
}

func TestResolveByDisplayName(t *testing.T) {
	m := Resolve("Kling 2.6")
	if m == nil || m.Slug != "kling2-6" {
		t.Fatalf("expected kling2-6 from 'Kling 2.6', got %v", m)
	}
}

func TestResolveByShorthand(t *testing.T) {
	cases := map[string]string{
		"seedance2":     "byte-plus-seedance-2",
		"seedance 2":    "byte-plus-seedance-2",
		"seedance-2":    "byte-plus-seedance-2",
		"kling 2.6":     "kling2-6",
		"kling26":       "kling2-6",
		"grok":          "grok-imagine",
		"grok imagine":  "grok-imagine",
		"veo3":          "veo3-1",
		"veo 3.1":       "veo3-1",
	}
	for input, wantSlug := range cases {
		got := Resolve(input)
		if got == nil {
			t.Errorf("Resolve(%q) returned nil, want %s", input, wantSlug)
			continue
		}
		if got.Slug != wantSlug {
			t.Errorf("Resolve(%q) = %s, want %s", input, got.Slug, wantSlug)
		}
	}
}

func TestCapability(t *testing.T) {
	m := FindBySlug("byte-plus-seedance-2")
	if m == nil {
		t.Fatal("model lookup failed")
	}
	if got := m.Capability(FormText2Video); got != "byte-plus-seedance-2:text2video" {
		t.Errorf("capability = %q", got)
	}
}

func TestEstimateCreditsScalesOnDurationAndResolution(t *testing.T) {
	m := FindBySlug("byte-plus-seedance-2")
	if m == nil {
		t.Fatal("model lookup failed")
	}
	mid := defaultDurationFor(*m)
	base := m.EstimateCredits(mid, 1, "720p")
	if base != m.CreditsPerVideoDefault {
		t.Errorf("at default duration/720p, estimate = %d, want %d", base, m.CreditsPerVideoDefault)
	}
	doubled := m.EstimateCredits(mid*2, 1, "720p")
	if doubled != base*2 {
		t.Errorf("doubling duration: estimate = %d, want %d", doubled, base*2)
	}
	hd := m.EstimateCredits(mid, 1, "1080p")
	if hd != base*3/2 {
		t.Errorf("1080p multiplier: estimate = %d, want %d", hd, base*3/2)
	}
	four := m.EstimateCredits(mid, 1, "4K")
	if four != base*2 {
		t.Errorf("4K multiplier: estimate = %d, want %d", four, base*2)
	}
	bulk := m.EstimateCredits(mid, 4, "720p")
	if bulk != base*4 {
		t.Errorf("count multiplier: estimate = %d, want %d", bulk, base*4)
	}
}

func TestFilterVideo(t *testing.T) {
	video := FilterVideo()
	if len(video) == 0 {
		t.Fatal("expected at least one video model in catalog")
	}
	for _, m := range video {
		if m.Family != FamilyVideo {
			t.Errorf("FilterVideo returned non-video model: %s", m.Slug)
		}
	}
}

func TestFilterImage(t *testing.T) {
	image := FilterImage()
	if len(image) < 5 {
		t.Fatalf("expected at least 5 image models in catalog, got %d", len(image))
	}
	for _, m := range image {
		if m.Family != FamilyImage {
			t.Errorf("FilterImage returned non-image model: %s", m.Slug)
		}
	}
}

func TestImageCapability(t *testing.T) {
	m := FindBySlug("nano-banana")
	if m == nil {
		t.Fatal("nano-banana not in catalog")
	}
	if got := m.Capability(FormText2Image); got != "create-image:reference:nano-banana" {
		t.Errorf("nano-banana Capability = %q; want create-image:reference:nano-banana", got)
	}
	if got := m.Capability(FormImage2Image); got != "create-image:reference:nano-banana" {
		t.Errorf("nano-banana Capability(image2image) = %q; want create-image:reference:nano-banana", got)
	}
}

func TestImageVsVideoCapability(t *testing.T) {
	v := FindBySlug("byte-plus-seedance-2")
	if v == nil {
		t.Fatal("byte-plus-seedance-2 not in catalog")
	}
	if got := v.Capability(FormText2Video); got != "byte-plus-seedance-2:text2video" {
		t.Errorf("video Capability changed: %q", got)
	}
}

func TestImageEstimateCreditsIgnoresDuration(t *testing.T) {
	m := FindBySlug("nano-banana")
	if m == nil {
		t.Fatal("nano-banana not in catalog")
	}
	base := m.EstimateCredits(0, 1, "1024x1024")
	withDuration := m.EstimateCredits(10, 1, "1024x1024")
	if base != withDuration {
		t.Errorf("image EstimateCredits should ignore duration: base=%d withDuration=%d", base, withDuration)
	}
	bigger := m.EstimateCredits(0, 1, "1536x1024")
	if bigger <= base {
		t.Errorf("1536x1024 should cost more than 1024x1024: 1024=%d, 1536=%d", base, bigger)
	}
	multi := m.EstimateCredits(0, 4, "1024x1024")
	if multi != base*4 {
		t.Errorf("count multiplier broken on image: 1=%d, 4=%d", base, multi)
	}
}

func TestExperimentalFlag(t *testing.T) {
	nano := FindBySlug("nano-banana")
	if nano == nil || nano.Experimental {
		t.Errorf("nano-banana should be verified (Experimental=false)")
	}
	gpt := FindBySlug("gpt-image-2")
	if gpt == nil || !gpt.Experimental {
		t.Errorf("gpt-image-2 should be Experimental=true")
	}
}
