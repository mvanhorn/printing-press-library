package hfx

import "testing"

func TestDetectQuant(t *testing.T) {
	cases := []struct {
		name   string
		want   string
		family string
		ok     bool
	}{
		{"qwen2.5-7b-instruct-Q4_K_M.gguf", "Q4_K_M", "K", true},
		{"qwen2.5-7b-instruct-IQ4_NL.gguf", "IQ4_NL", "I", true},
		{"qwen2.5-7b-instruct-UD-Q4_K_XL.gguf", "UD-Q4_K_XL", "UD", true},
		{"qwen2.5-7b-instruct-Q8_0.gguf", "Q8_0", "Legacy", true},
		{"qwen2.5-7b-instruct-F16.gguf", "F16", "F", true},
		{"qwen2.5-7b-instruct-MXFP4.gguf", "MXFP4", "MXFP", true},
		{"qwen2.5-7b-instruct.safetensors", "", "", false},
		{"random-file.txt", "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := DetectQuant(c.name)
			if ok != c.ok {
				t.Fatalf("DetectQuant ok = %v, want %v", ok, c.ok)
			}
			if !ok {
				return
			}
			if got.Code != c.want {
				t.Errorf("Code = %q, want %q", got.Code, c.want)
			}
			if got.Family != c.family {
				t.Errorf("Family = %q, want %q", got.Family, c.family)
			}
		})
	}
}

func TestIsTrustedUploader(t *testing.T) {
	cases := []struct {
		user string
		want bool
	}{
		{"unsloth", true},
		{"bartowski", true},
		{"mradermacher", true},
		{"BARTOWSKI", true}, // case-insensitive
		{"randomperson", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsTrustedUploader(c.user); got != c.want {
			t.Errorf("IsTrustedUploader(%q) = %v, want %v", c.user, got, c.want)
		}
	}
}

func TestClassifyFeature_MoE(t *testing.T) {
	cfg := ConfigJSON{NumExperts: 8, NumExpertsPerTok: 2}
	cls := ClassifyFeature("moe", cfg, "")
	if !cls.Detected {
		t.Fatal("expected MoE detected")
	}
	if cls.Confidence != "high" {
		t.Errorf("Confidence = %q, want high", cls.Confidence)
	}
}

func TestClassifyFeature_GQA(t *testing.T) {
	cfg := ConfigJSON{NumAttentionHeads: 32, NumKeyValueHeads: 8}
	cls := ClassifyFeature("gqa", cfg, "")
	if !cls.Detected {
		t.Fatal("expected GQA detected")
	}
}
