package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestLoadAcceptsShortR365EnvAliases(t *testing.T) {
	t.Setenv("R365_ODATA_USERNAME", "tenant\\user")
	t.Setenv("R365_ODATA_PASSWORD", "secret-password-value")
	t.Setenv("R365_ODATA_BASE_URL", "https://example.test/views")
	t.Setenv("RESTAURANT365_ODATA_USERNAME", "")
	t.Setenv("RESTAURANT365_ODATA_PASSWORD", "")
	t.Setenv("RESTAURANT365_ODATA_BASE_URL", "")

	cfg, err := Load(t.TempDir() + "/missing.toml")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.BaseURL != "https://example.test/views" {
		t.Fatalf("BaseURL=%q", cfg.BaseURL)
	}
	if cfg.AuthSource != "env:R365_ODATA_PASSWORD" {
		t.Fatalf("AuthSource=%q", cfg.AuthSource)
	}
	auth := cfg.AuthHeader()
	if !strings.HasPrefix(auth, "Basic ") {
		t.Fatalf("AuthHeader=%q, want Basic header", auth)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != "tenant\\user:secret-password-value" {
		t.Fatalf("decoded auth=%q", decoded)
	}
}

func TestSaveBasicCredentialsPersistsUsernameAndPassword(t *testing.T) {
	path := t.TempDir() + "/config.toml"
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if err := cfg.SaveBasicCredentials("tenant\\user", "secret-password-value"); err != nil {
		t.Fatalf("SaveBasicCredentials returned error: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load after save returned error: %v", err)
	}
	if loaded.Restaurant365OdataUsername != "tenant\\user" {
		t.Fatalf("username=%q", loaded.Restaurant365OdataUsername)
	}
	if loaded.Restaurant365OdataPassword != "secret-password-value" {
		t.Fatalf("password=%q", loaded.Restaurant365OdataPassword)
	}
	if !strings.HasPrefix(loaded.AuthHeader(), "Basic ") {
		t.Fatalf("AuthHeader=%q, want Basic header", loaded.AuthHeader())
	}
}
