package config

import "testing"

func TestResolveCaptchaProfile_PrecedenceAndDefault(t *testing.T) {
	cfg := &Config{}
	if got := cfg.ResolveCaptchaProfile(""); got != "default" {
		t.Fatalf("empty -> default, got %q", got)
	}
	if got := cfg.ResolveCaptchaProfile("work"); got != "work" {
		t.Fatalf("flag should win, got %q", got)
	}
	cfg.Captcha = &CaptchaConfig{DefaultProfile: "studio"}
	if got := cfg.ResolveCaptchaProfile(""); got != "studio" {
		t.Fatalf("config default, got %q", got)
	}
}

func TestEnsureCaptchaProfile_AssignsDistinctPorts(t *testing.T) {
	cfg := &Config{}
	a := cfg.EnsureCaptchaProfile("default")
	b := cfg.EnsureCaptchaProfile("work")
	if a.CDPPort == b.CDPPort {
		t.Fatalf("profiles must get distinct ports: %d == %d", a.CDPPort, b.CDPPort)
	}
	if a.CDPPort != 9233 {
		t.Fatalf("first profile should start at 9233, got %d", a.CDPPort)
	}
	if b.CDPPort != 9234 {
		t.Fatalf("second profile should be 9234, got %d", b.CDPPort)
	}
	if again := cfg.EnsureCaptchaProfile("default"); again.CDPPort != 9233 {
		t.Fatalf("re-ensure changed port: %d", again.CDPPort)
	}
}
