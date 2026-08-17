package config

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestSaveBasicCredentialsPersistsCompletePair(t *testing.T) {
	t.Setenv("DATAFORSEO_LOGIN", "")
	t.Setenv("DATAFORSEO_PASSWORD", "")
	path := t.TempDir() + "/config.toml"
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveBasicCredentials("account@example.com", "secret-password-value"); err != nil {
		t.Fatalf("SaveBasicCredentials: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DataforseoDocumentationUsername != "account@example.com" {
		t.Fatalf("login = %q", loaded.DataforseoDocumentationUsername)
	}
	if loaded.DataforseoDocumentationPassword != "secret-password-value" {
		t.Fatalf("password = %q", loaded.DataforseoDocumentationPassword)
	}
	encoded := strings.TrimPrefix(loaded.AuthHeader(), "Basic ")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != "account@example.com:secret-password-value" {
		t.Fatalf("decoded Authorization = %q", decoded)
	}
}

func TestClearTokensClearsBasicAndOAuthCredentials(t *testing.T) {
	t.Setenv("DATAFORSEO_LOGIN", "")
	t.Setenv("DATAFORSEO_PASSWORD", "")
	path := t.TempDir() + "/config.toml"
	cfg := &Config{
		Path:                            path,
		AuthHeaderVal:                   "Bearer legacy",
		AccessToken:                     "access",
		RefreshToken:                    "refresh",
		TokenExpiry:                     time.Now().Add(time.Hour),
		ClientID:                        "client-id",
		ClientSecret:                    "client-secret",
		DataforseoDocumentationUsername: "login",
		DataforseoDocumentationPassword: "password",
	}
	if err := cfg.ClearTokens(); err != nil {
		t.Fatalf("ClearTokens: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.AuthHeader() != "" || loaded.AuthHeaderVal != "" || loaded.AccessToken != "" || loaded.RefreshToken != "" ||
		loaded.ClientID != "" || loaded.ClientSecret != "" || loaded.DataforseoDocumentationUsername != "" || loaded.DataforseoDocumentationPassword != "" || !loaded.TokenExpiry.IsZero() {
		t.Fatalf("credentials remain after logout: %#v", loaded)
	}
}
