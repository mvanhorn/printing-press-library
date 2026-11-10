// Copyright 2026 Paul Taramona and contributors. Licensed under Apache-2.0. See LICENSE.

package config

// PATCH(novel1219-auth-compat): read-only, global-only legacy auth (orchestrator decision after Codex novel1219-r10):
// env precedence; earlier saved keys honoured only in the global credentials file and the config document, only when
// no current credential of any kind exists, through the existing checked-descriptor loaders; profile-file earlier keys
// ignored with a notice; the generated set-token and logout unchanged; nothing written or deleted by this change; no
// secret in any output.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/cliutil"
)

const (
	legacyTokenA = "LEGACY-BEARER-AUTH-SECRET-1111"
	legacyTokenB = "LEGACY-API-KEY-SECRET-2222"
	currentToken = "CURRENT-CRM-BEARER-SECRET-3333"
	headerToken  = "Bearer AUTH-HEADER-SECRET-4444"
	defaultBase  = "https://conduyt.app/api/v1"
)

// isolateLegacyAuth isolates every credential source and captures both notice writers.
func isolateLegacyAuth(t *testing.T) *bytes.Buffer {
	t.Helper()
	isolateCredentials(t)
	setOrUnset(t, "CONDUYT_CRM_BEARER_AUTH", "")
	setOrUnset(t, "CONDUYT_BEARER_AUTH", "")
	setOrUnset(t, "CONDUYT_API_KEY", "")
	var buf bytes.Buffer
	prevCfg, prevCli := legacyAuthNoticeWriter, cliutil.LegacyNoticeWriter
	legacyAuthNoticeWriter, cliutil.LegacyNoticeWriter = &buf, &buf
	legacyAuthNoticeOnce = sync.Map{}
	t.Cleanup(func() {
		legacyAuthNoticeWriter, cliutil.LegacyNoticeWriter = prevCfg, prevCli
		legacyAuthNoticeOnce = sync.Map{}
	})
	return &buf
}

// blankEnv marks an env var that is set to the empty string (as opposed to "", unset).
const blankEnv = "<blank>"

func setOrUnset(t *testing.T, name, value string) {
	t.Helper()
	t.Setenv(name, "")
	switch value {
	case "":
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	case blankEnv:
	default:
		t.Setenv(name, value)
	}
}

func writeOwnerOnly(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertNoSecret(t *testing.T, where, text string) {
	t.Helper()
	for _, s := range []string{legacyTokenA, legacyTokenB, currentToken, headerToken} {
		if strings.Contains(text, s) {
			t.Fatalf("%s leaks a secret value: %q", where, text)
		}
	}
}

func assertUnchanged(t *testing.T, path, body string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != body {
		t.Fatalf("%s was changed (err %v)", filepath.Base(path), err)
	}
}

func profilePaths(t *testing.T) (configPath, credsPath string) {
	t.Helper()
	configPath = filepath.Join(t.TempDir(), "profile", "config.toml")
	credsPath, err := cliutil.CredentialsFilePathForConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	return configPath, credsPath
}

func globalCredsPath(t *testing.T) string {
	t.Helper()
	p, err := cliutil.CredentialsFilePath()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLegacyAuthEnvPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name               string
		crm, bearer, apiKy string
		want, wantSource   string
		wantNotice         string
	}{
		{"current only", currentToken, "", "", currentToken, "env:CONDUYT_CRM_BEARER_AUTH", ""},
		{"current beats legacy", currentToken, legacyTokenA, legacyTokenB, currentToken, "env:CONDUYT_CRM_BEARER_AUTH", ""},
		{"bearer auth fallback", "", legacyTokenA, "", legacyTokenA, "env:CONDUYT_BEARER_AUTH", "CONDUYT_BEARER_AUTH is deprecated"},
		{"bearer auth beats api key", "", legacyTokenA, legacyTokenB, legacyTokenA, "env:CONDUYT_BEARER_AUTH", "CONDUYT_BEARER_AUTH is deprecated"},
		{"api key fallback", "", "", legacyTokenB, legacyTokenB, "env:CONDUYT_API_KEY", "CONDUYT_API_KEY is deprecated"},
		{"none", "", "", "", "", "", ""},
		{"blank current blocks legacy", blankEnv, legacyTokenA, legacyTokenB, "", "", ""},
		{"blank bearer auth blocks api key", "", blankEnv, legacyTokenB, "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			notices := isolateLegacyAuth(t)
			setOrUnset(t, "CONDUYT_CRM_BEARER_AUTH", tc.crm)
			setOrUnset(t, "CONDUYT_BEARER_AUTH", tc.bearer)
			setOrUnset(t, "CONDUYT_API_KEY", tc.apiKy)
			cfg, err := Load(filepath.Join(t.TempDir(), "config.toml"))
			if err != nil {
				t.Fatal(err)
			}
			if cfg.ConduytCrmBearerAuth != tc.want || cfg.AuthSource != tc.wantSource {
				t.Fatalf("token match=%t source=%q, want source %q", cfg.ConduytCrmBearerAuth == tc.want, cfg.AuthSource, tc.wantSource)
			}
			if tc.want != "" && cfg.AuthHeader() != "Bearer "+tc.want {
				t.Fatal("AuthHeader does not carry the selected token")
			}
			if tc.wantNotice == "" && notices.Len() != 0 || tc.wantNotice != "" && strings.Count(notices.String(), "\n") != 1 || !strings.Contains(notices.String(), tc.wantNotice) {
				t.Fatalf("notices = %q, want %q once", notices.String(), tc.wantNotice)
			}
			assertNoSecret(t, "notice", notices.String())
		})
	}
}

// Every current credential kind outranks an earlier key, in the same document and across documents; an earlier key in
// the global credentials file is used only when no current credential of any kind exists anywhere.
func TestLegacyAuthRanksBelowEveryCurrentKind(t *testing.T) {
	legacy := "bearer_auth = \"" + legacyTokenA + "\"\napi_key = \"" + legacyTokenB + "\"\n"
	for _, tc := range []struct {
		name, globalExtra, configDoc string
		wantLegacy                   bool
	}{
		{"nothing current: global earlier key used", "", "", true},
		{"global auth_header", "auth_header = \"" + headerToken + "\"\n", "", false},
		{"global crm_bearer_auth", "crm_bearer_auth = \"" + currentToken + "\"\n", "", false},
		{"global crm_bearer_auth empty (cleared)", "crm_bearer_auth = \"\"\n", "", false},
		{"global access_token", "access_token = \"ACCESS-SECRET-8888\"\n", "", false},
		{"global refresh_token", "refresh_token = \"REFRESH-SECRET-8888\"\n", "", false},
		{"global client_id", "client_id = \"client\"\n", "", false},
		{"global client_secret", "client_secret = \"CLIENT-SECRET-8888\"\n", "", false},
		{"config access_token", "", "access_token = \"ACCESS-SECRET-8888\"\n", false},
		{"config crm_bearer_auth", "", "crm_bearer_auth = \"" + currentToken + "\"\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			notices := isolateLegacyAuth(t)
			global := "base_url = \"" + defaultBase + "\"\n" + tc.globalExtra + legacy
			writeOwnerOnly(t, globalCredsPath(t), global)
			configPath := filepath.Join(t.TempDir(), "config.toml")
			if tc.configDoc != "" {
				writeOwnerOnly(t, configPath, tc.configDoc)
			}
			cfg, err := Load(configPath)
			if err != nil {
				t.Fatal(err)
			}
			usedLegacy := strings.Contains(cfg.AuthHeader(), legacyTokenA) || strings.Contains(cfg.AuthHeader(), legacyTokenB)
			if usedLegacy != tc.wantLegacy {
				t.Fatalf("legacy used=%t, want %t (source %q)", usedLegacy, tc.wantLegacy, cfg.CredentialSource)
			}
			if tc.wantLegacy && (cfg.AuthHeader() != "Bearer "+legacyTokenA || cfg.CredentialSource != "credentials file" || !strings.Contains(notices.String(), globalCredsPath(t))) {
				t.Fatalf("global earlier key: header ok=%t source=%q notices=%q", cfg.AuthHeader() == "Bearer "+legacyTokenA, cfg.CredentialSource, notices.String())
			}
			assertNoSecret(t, "notice", notices.String())
			assertUnchanged(t, globalCredsPath(t), global)
		})
	}
}

// The config document's earlier key is honoured only when nothing current exists anywhere (a global current
// credential outranks it); the document is never changed by the load.
func TestLegacyAuthConfigDocumentKey(t *testing.T) {
	t.Run("alone", func(t *testing.T) {
		notices := isolateLegacyAuth(t)
		path := filepath.Join(t.TempDir(), "config.toml")
		body := "api_key = \"" + legacyTokenB + "\"\n"
		writeOwnerOnly(t, path, body)
		cfg, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.AuthHeader() != "Bearer "+legacyTokenB || cfg.CredentialSource != "config-kind path" || !strings.Contains(notices.String(), path) {
			t.Fatalf("config earlier key: source=%q notices=%q", cfg.CredentialSource, notices.String())
		}
		assertUnchanged(t, path, body)
		assertNoSecret(t, "notice", notices.String())
	})
	t.Run("below a global current credential", func(t *testing.T) {
		isolateLegacyAuth(t)
		path := filepath.Join(t.TempDir(), "config.toml")
		writeOwnerOnly(t, path, "bearer_auth = \""+legacyTokenA+"\"\n")
		writeOwnerOnly(t, globalCredsPath(t), "base_url = \""+defaultBase+"\"\naccess_token = \"ACCESS-SECRET-8888\"\n")
		cfg, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.AuthHeader() != "Bearer ACCESS-SECRET-8888" {
			t.Fatal("a config earlier key outranked a global current credential")
		}
	})
	t.Run("cleared current key in the same document", func(t *testing.T) {
		isolateLegacyAuth(t)
		path := filepath.Join(t.TempDir(), "config.toml")
		writeOwnerOnly(t, path, "crm_bearer_auth = \"\"\nbearer_auth = \""+legacyTokenA+"\"\n")
		cfg, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.AuthHeader() != "" {
			t.Fatal("an empty crm_bearer_auth in the config document resurrected an earlier key")
		}
	})
	t.Run("untrusted document refused", func(t *testing.T) {
		isolateLegacyAuth(t)
		path := filepath.Join(t.TempDir(), "config.toml")
		writeOwnerOnly(t, path, "bearer_auth = \""+legacyTokenA+"\"\n")
		if err := os.Chmod(path, 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.AuthHeader() != "" || !cfg.HasCredentialRefusals() {
			t.Fatal("an untrusted config document's earlier key was used")
		}
	})
}

// Earlier keys inside a profile credentials file are never honoured; the loader prints one notice naming the file and
// the generated re-save command; the global file decides as it would without them.
func TestLegacyAuthProfileFileKeysIgnoredWithNotice(t *testing.T) {
	for _, withGlobal := range []bool{false, true} {
		notices := isolateLegacyAuth(t)
		configPath, credsPath := profilePaths(t)
		body := "base_url = \"" + defaultBase + "\"\nbearer_auth = \"" + legacyTokenA + "\"\n"
		writeOwnerOnly(t, credsPath, body)
		if withGlobal {
			writeOwnerOnly(t, globalCredsPath(t), "base_url = \""+defaultBase+"\"\ncrm_bearer_auth = \""+currentToken+"\"\n")
		}
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatal(err)
		}
		want := ""
		if withGlobal {
			want = "Bearer " + currentToken
		}
		if cfg.AuthHeader() != want {
			t.Fatalf("withGlobal=%t: a profile earlier key was honoured", withGlobal)
		}
		if !strings.Contains(notices.String(), credsPath) || !strings.Contains(notices.String(), cliutil.LegacyResaveCommand) {
			t.Fatalf("withGlobal=%t: notice %q does not name the file and the re-save command", withGlobal, notices.String())
		}
		assertUnchanged(t, credsPath, body)
		assertNoSecret(t, "notice", notices.String())
	}
}

// After the generated set-token the saved current token outranks every earlier key, so the new token is sent.
func TestLegacyAuthGeneratedSetTokenWins(t *testing.T) {
	const rotated = "ROTATED-SET-TOKEN-SECRET-5555"
	for _, where := range []string{"global", "config"} {
		t.Run(where, func(t *testing.T) {
			isolateLegacyAuth(t)
			configPath := filepath.Join(t.TempDir(), "config.toml")
			if where == "global" {
				writeOwnerOnly(t, globalCredsPath(t), "base_url = \""+defaultBase+"\"\napi_key = \""+legacyTokenB+"\"\n")
			} else {
				writeOwnerOnly(t, configPath, "bearer_auth = \""+legacyTokenA+"\"\n")
			}
			cfg, err := Load(configPath)
			if err != nil {
				t.Fatal(err)
			}
			cfg.AuthHeaderVal = ""
			if err := cfg.SaveTokens("", "", rotated, "", time.Time{}); err != nil { // exactly what auth set-token does
				t.Fatal(err)
			}
			after, err := Load(configPath)
			if err != nil {
				t.Fatal(err)
			}
			if after.AuthHeader() != "Bearer "+rotated {
				t.Fatalf("after the generated set-token the earlier key is still sent (%s)", where)
			}
		})
	}
}

// After the generated logout the files that held honoured earlier keys are cleared, so nothing is found.
func TestLegacyAuthGeneratedLogoutClears(t *testing.T) {
	isolateLegacyAuth(t)
	configPath := filepath.Join(t.TempDir(), "config.toml")
	writeOwnerOnly(t, configPath, "bearer_auth = \""+legacyTokenA+"\"\n")
	writeOwnerOnly(t, globalCredsPath(t), "base_url = \""+defaultBase+"\"\napi_key = \""+legacyTokenB+"\"\n")
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.ClearTokens(); err != nil { // exactly what auth logout does
		t.Fatal(err)
	}
	after, err := Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if after.AuthHeader() != "" || after.ConduytCrmBearerAuth != "" {
		t.Fatal("a credential survived the generated logout")
	}
}

func TestLegacyAuthUnsafeOrAliasedGlobalFileIsNotUsed(t *testing.T) {
	t.Run("loose mode", func(t *testing.T) {
		isolateLegacyAuth(t)
		writeOwnerOnly(t, globalCredsPath(t), "base_url = \""+defaultBase+"\"\nbearer_auth = \""+legacyTokenA+"\"\n")
		if err := os.Chmod(globalCredsPath(t), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(filepath.Join(t.TempDir(), "config.toml"))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.AuthHeader() != "" {
			t.Fatal("a loose global file's earlier key was used")
		}
	})
	t.Run("symlink", func(t *testing.T) {
		isolateLegacyAuth(t)
		target := filepath.Join(t.TempDir(), "elsewhere.toml")
		writeOwnerOnly(t, target, "base_url = \""+defaultBase+"\"\nbearer_auth = \""+legacyTokenA+"\"\n")
		if err := os.MkdirAll(filepath.Dir(globalCredsPath(t)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, globalCredsPath(t)); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(filepath.Join(t.TempDir(), "config.toml"))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.AuthHeader() != "" {
			t.Fatal("a symlinked global file's earlier key was used")
		}
	})
}

func TestLegacyAuthSavesNeverWriteLegacyKeys(t *testing.T) {
	isolateLegacyAuth(t)
	if err := cliutil.SaveCredentials(&cliutil.Credentials{BaseURL: defaultBase, ConduytCrmBearerAuth: currentToken, LegacyBearerAuth: legacyTokenA, LegacyToken: legacyTokenA, LegacyKey: "bearer_auth"}); err != nil {
		t.Fatal(err)
	}
	persisted, err := os.ReadFile(globalCredsPath(t))
	if err != nil {
		t.Fatal(err)
	}
	doc := strings.ReplaceAll(string(persisted), "crm_bearer_auth", "")
	if strings.Contains(doc, "bearer_auth") || strings.Contains(doc, "api_key") || strings.Contains(string(persisted), legacyTokenA) || strings.Contains(string(persisted), "Legacy") {
		t.Fatalf("a save wrote an earlier key or internal field:\n%s", persisted)
	}
}

// This change adds no file write or delete: the profile writer files are gone and the compatibility code calls no
// file-writing or file-removing API.
func TestLegacyAuthAddsNoFileWriteOrDelete(t *testing.T) {
	for _, gone := range []string{"../cliutil/creds_profile_write_unix.go", "../cliutil/creds_profile_write_windows.go"} {
		if _, err := os.Stat(gone); !os.IsNotExist(err) {
			t.Fatalf("%s still exists", gone)
		}
	}
	src, err := os.ReadFile("legacy_auth.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range []string{"os.WriteFile", "os.Remove", "os.RemoveAll", "os.Rename", "os.Create", "os.CreateTemp", "os.OpenFile", "os.Mkdir", "os.MkdirAll", "AtomicWritePrivateFile", "SaveCredentials", "RemoveCredentials", ".save()", "Unlinkat", "Renameat"} {
		if strings.Contains(string(src), call) {
			t.Fatalf("legacy_auth.go calls %s", call)
		}
	}
	for _, name := range []string{"SetProfileBearer", "ScrubLegacyCredentialKeys", "RemoveProfileCredentials", "SaveBearerToken"} {
		for _, f := range []string{"legacy_auth.go", "config.go", "../cliutil/credentials.go", "../cli/auth.go"} {
			raw, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), name) {
				t.Fatalf("%s still references the reverted writer %s", f, name)
			}
		}
	}
}

// ---- Codex novel1219-r11 regression (r12): one sticky rule ----

// Once any trusted source declares the current key (present even empty) or CONDUYT_CRM_BEARER_AUTH is set (even blank),
// earlier-key fallback is off for every source for the rest of the resolution; with the current key absent
// everywhere the earlier key is still used.
func TestLegacyAuthDeclaredCurrentKeyDisablesEveryLegacySource(t *testing.T) {
	globalLegacy := "base_url = \"" + defaultBase + "\"\nbearer_auth = \"" + legacyTokenA + "\"\n"
	t.Run("config crm_bearer_auth empty + global legacy", func(t *testing.T) {
		isolateLegacyAuth(t)
		path := filepath.Join(t.TempDir(), "config.toml")
		writeOwnerOnly(t, path, "crm_bearer_auth = \"\"\n")
		writeOwnerOnly(t, globalCredsPath(t), globalLegacy)
		cfg, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.AuthHeader() != "" {
			t.Fatal("a cleared config key let the global earlier key through")
		}
	})
	t.Run("explicit profile with an empty current bearer + global legacy", func(t *testing.T) {
		isolateLegacyAuth(t)
		configPath, credsPath := profilePaths(t)
		writeOwnerOnly(t, credsPath, "base_url = \""+defaultBase+"\"\ncrm_bearer_auth = \"\"\n")
		writeOwnerOnly(t, globalCredsPath(t), globalLegacy)
		cfg, err := Load(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.AuthHeader() != "" {
			t.Fatal("an empty profile bearer fell through to the global earlier key")
		}
	})
	t.Run("blank CONDUYT_CRM_BEARER_AUTH + file legacy", func(t *testing.T) {
		for _, where := range []string{"global", "config"} {
			isolateLegacyAuth(t)
			path := filepath.Join(t.TempDir(), "config.toml")
			if where == "global" {
				writeOwnerOnly(t, globalCredsPath(t), globalLegacy)
			} else {
				writeOwnerOnly(t, path, "bearer_auth = \""+legacyTokenA+"\"\n")
			}
			setOrUnset(t, "CONDUYT_CRM_BEARER_AUTH", blankEnv)
			cfg, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.AuthHeader() != "" {
				t.Fatalf("a blank CONDUYT_CRM_BEARER_AUTH let the %s earlier key through", where)
			}
		}
	})
	t.Run("declared current key + legacy env name", func(t *testing.T) {
		isolateLegacyAuth(t)
		path := filepath.Join(t.TempDir(), "config.toml")
		writeOwnerOnly(t, path, "crm_bearer_auth = \"\"\n")
		setOrUnset(t, "CONDUYT_BEARER_AUTH", legacyTokenA)
		cfg, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.AuthHeader() != "" {
			t.Fatal("a cleared config key let a legacy env name through")
		}
	})
	t.Run("untrusted declaration does not count", func(t *testing.T) {
		isolateLegacyAuth(t)
		path := filepath.Join(t.TempDir(), "config.toml")
		writeOwnerOnly(t, path, "crm_bearer_auth = \"\"\n")
		if err := os.Chmod(path, 0o644); err != nil {
			t.Fatal(err)
		}
		writeOwnerOnly(t, globalCredsPath(t), globalLegacy)
		cfg, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.AuthHeader() != "Bearer "+legacyTokenA {
			t.Fatal("an untrusted document's declaration disabled the trusted global earlier key")
		}
	})
	t.Run("absent everywhere: legacy still used", func(t *testing.T) {
		isolateLegacyAuth(t)
		writeOwnerOnly(t, globalCredsPath(t), globalLegacy)
		cfg, err := Load(filepath.Join(t.TempDir(), "config.toml"))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.AuthHeader() != "Bearer "+legacyTokenA {
			t.Fatal("with the current key absent everywhere the global earlier key was not used")
		}
	})
}

// Codex novel1219-r12 P1: a config document holding another current credential must still declare, so a legacy env
// name is never installed over it. The rule lives in one place (legacyAllowed).
func TestLegacyAuthCurrentCredentialInDocumentDisablesLegacyEnv(t *testing.T) {
	cases := []struct {
		name, doc, env, want string
	}{
		{"config access_token + CONDUYT_BEARER_AUTH", "access_token = \"" + currentToken + "\"\n", "CONDUYT_BEARER_AUTH", currentToken},
		{"config crm_bearer_auth set + CONDUYT_BEARER_AUTH", "crm_bearer_auth = \"" + currentToken + "\"\n", "CONDUYT_BEARER_AUTH", currentToken},
		{"config auth_header + CONDUYT_API_KEY", "auth_header = \"" + headerToken + "\"\n", "CONDUYT_API_KEY", headerToken},
		{"config crm_bearer_auth empty + CONDUYT_API_KEY", "crm_bearer_auth = \"\"\n", "CONDUYT_API_KEY", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isolateLegacyAuth(t)
			path := filepath.Join(t.TempDir(), "config.toml")
			writeOwnerOnly(t, path, tc.doc)
			setOrUnset(t, tc.env, legacyTokenB)
			cfg, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			got := cfg.AuthHeader()
			if strings.Contains(got, legacyTokenB) || strings.HasPrefix(cfg.AuthSource, "env:") {
				t.Fatalf("%s was used over a declared current credential (source %q)", tc.env, cfg.AuthSource)
			}
			if tc.want == "" && got != "" {
				t.Fatal("a cleared current key still produced a credential")
			}
			if tc.want != "" && !strings.Contains(got, tc.want) {
				t.Fatal("the declared current credential was not used")
			}
		})
	}
	t.Run("global file access_token + CONDUYT_BEARER_AUTH", func(t *testing.T) {
		isolateLegacyAuth(t)
		writeOwnerOnly(t, globalCredsPath(t), "base_url = \""+defaultBase+"\"\naccess_token = \""+currentToken+"\"\n")
		setOrUnset(t, "CONDUYT_BEARER_AUTH", legacyTokenB)
		cfg, err := Load(filepath.Join(t.TempDir(), "config.toml"))
		if err != nil {
			t.Fatal(err)
		}
		if got := cfg.AuthHeader(); strings.Contains(got, legacyTokenB) || !strings.Contains(got, currentToken) {
			t.Fatal("a legacy env name was used over the global file's current credential")
		}
	})
	t.Run("no current credential anywhere: legacy env used", func(t *testing.T) {
		isolateLegacyAuth(t)
		setOrUnset(t, "CONDUYT_BEARER_AUTH", legacyTokenB)
		cfg, err := Load(filepath.Join(t.TempDir(), "config.toml"))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.AuthHeader() != "Bearer "+legacyTokenB || cfg.AuthSource != "env:CONDUYT_BEARER_AUTH" {
			t.Fatal("with no current credential the legacy env name was not used")
		}
	})
}
