package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestDoctorReportsMissingEnvWithoutSecrets(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "")
	var out bytes.Buffer
	if err := run([]string{"doctor"}, &out, &bytes.Buffer{}); err != nil {
		t.Fatalf("doctor failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "CLOUDFLARE_API_TOKEN") || !strings.Contains(got, "CLOUDFLARE_ACCOUNT_ID") {
		t.Fatalf("expected missing env names, got %s", got)
	}
}

func TestRegisterRequiresTypedConfirmationBeforeAuth(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "")
	var out bytes.Buffer
	err := run([]string{"domain-register", "--domain", "example.com"}, &out, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--confirm-domain") {
		t.Fatalf("expected confirmation error, got %v", err)
	}
}

func TestRegisterConfirmationMustMatchDomain(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("CLOUDFLARE_ACCOUNT_ID", "")
	err := run([]string{"domain-register", "--domain", "example.com", "--confirm-domain", "other.com"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "must exactly match") {
		t.Fatalf("expected mismatch error, got %v", err)
	}
}

func TestDomainCheckRequiresDomain(t *testing.T) {
	err := run([]string{"domain-check"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "requires --domain") {
		t.Fatalf("expected --domain error, got %v", err)
	}
}

func TestDomainSearchRequiresQuery(t *testing.T) {
	err := run([]string{"domain-search"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "requires --query") {
		t.Fatalf("expected --query error, got %v", err)
	}
}

func TestVersion(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"version"}, &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) == "" {
		t.Fatal("expected version output")
	}
}

func TestMainDoesNotRequireEnvForHelp(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	var out bytes.Buffer
	if err := run([]string{"help"}, &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Cloudflare Registrar domain CLI") {
		t.Fatal(out.String())
	}
}
