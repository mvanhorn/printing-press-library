package config

import (
	"strings"
	"testing"
)

func TestSettingsFromEnvParsesPythonContract(t *testing.T) {
	got, err := SettingsFromEnv(map[string]string{
		"KVMCTL_URL": " https://kvm.example/ ", "KVMCTL_TOKEN": "tok",
		"KVMCTL_CA_BUNDLE": "/tmp/ca.pem", "KVMCTL_INSECURE": "true",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != "https://kvm.example/" || got.Token != "tok" || got.CABundle != "/tmp/ca.pem" || got.Verify {
		t.Fatalf("settings = %+v", got)
	}
}

func TestSettingsFromEnvRequiresURLAndAuth(t *testing.T) {
	if _, err := SettingsFromEnv(map[string]string{}); err == nil {
		t.Fatal("expected URL error")
	}
	_, err := SettingsFromEnv(map[string]string{"KVMCTL_URL": "https://x", "KVMCTL_USER": "u"})
	if err == nil || !strings.Contains(err.Error(), "password") {
		t.Fatalf("err = %v", err)
	}
}

func TestSettingsStringRedactsSecrets(t *testing.T) {
	got := Settings{URL: "https://x", Token: "secret-token", User: "u", Password: "secret-pass", CABundle: "/ca", Verify: true}.String()
	if strings.Contains(got, "secret-token") || strings.Contains(got, "secret-pass") {
		t.Fatalf("leaked: %s", got)
	}
}
