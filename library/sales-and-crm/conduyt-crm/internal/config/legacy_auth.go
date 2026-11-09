// Copyright 2026 Paul Taramona and contributors. Licensed under Apache-2.0. See LICENSE.

package config

// PATCH(novel1219-auth-compat): auth compatibility with installs of the earlier library entry: READ-ONLY and
// GLOBAL-ONLY (orchestrator decision after Codex novel1219-r10). This change writes and deletes nothing; set-token and
// logout are exactly as generated.
//
//   - Env: CONDUYT_CRM_BEARER_AUTH, then CONDUYT_BEARER_AUTH, then CONDUYT_API_KEY. A set name decides (blank =
//     cleared, never falls through); an unresolved MCP placeholder does not count as set. A legacy name prints one
//     deprecation line. Logout names the env var EffectiveAuthEnv resolves.
//   - Saved keys bearer_auth / api_key are honoured only in the files the generated logout clears: the global
//     credentials file (logout removes it) and the config document (every save marshals only current fields, so
//     logout's save drops them). They are read through the existing checked-descriptor loaders, only when the
//     document holds no current credential of any kind (crm_bearer_auth present even empty = cleared), and used only
//     when no current credential of any kind was found anywhere: below auth_header, crm_bearer_auth, access_token and
//     refresh/client credentials. After the generated set-token the saved current token outranks them; after the
//     generated logout they are gone with the files.
//   - THE RULE (one place: legacyAllowed): earlier keys and earlier env names are considered only when NO current
//     credential of any kind was declared by any trusted source (config document, profile file, global file, env);
//     otherwise never. A declaration is any current field with a value (auth_header, crm_bearer_auth, access_token,
//     refresh_token, client_id, client_secret), crm_bearer_auth present even empty, or CONDUYT_CRM_BEARER_AUTH set
//     even blank. Once declared, legacyDisabled stays set for the rest of the resolution.
//   - Earlier keys inside a PROFILE credentials file are not honoured: the loader prints one notice naming the file
//     and the generated re-save command.
//   - No secret value is ever printed.

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/cliutil"
	"github.com/pelletier/go-toml/v2"
)

// legacyAuthEnvNames are the env vars the earlier entry read, in fallback order after CONDUYT_CRM_BEARER_AUTH.
var legacyAuthEnvNames = []string{"CONDUYT_BEARER_AUTH", "CONDUYT_API_KEY"}

// legacyResaveCommand re-saves the token under the current key.
const legacyResaveCommand = cliutil.LegacyResaveCommand

// legacyAuthNoticeWriter receives the one-line notices; tests replace it.
var legacyAuthNoticeWriter io.Writer = os.Stderr

var legacyAuthNoticeOnce sync.Map

func legacyAuthNotice(key, format string, args ...any) {
	if _, seen := legacyAuthNoticeOnce.LoadOrStore(key, true); seen {
		return
	}
	fmt.Fprintf(legacyAuthNoticeWriter, "notice: "+format+"\n", args...)
}

// legacyKeySource is the saved earlier key that supplied the token.
type legacyKeySource struct {
	Path  string
	Key   string
	Token string
}

// applyLegacyConfigKey runs inside parseConfigData over the bytes of the checked descriptor: a config document with no
// current credential of any kind (crm_bearer_auth present even empty, auth_header, access_token, refresh_token,
// client_id, client_secret) but an earlier key puts that token in the bearer field for the duration of the parse, so
// acceptOpenedConfig's permission refusal and untrusted-routing drop apply to it as to any credential;
// deferLegacyConfigToken then moves it out of the way of every current source.
func (c *Config) applyLegacyConfigKey(data []byte, path string) {
	// The declaration check runs FIRST, before any early return: a document holding any current credential (a
	// non-empty current field, or crm_bearer_auth present even empty) declares, and once it is accepted as trusted
	// deferLegacyConfigToken turns that into legacyDisabled for every source, env names included.
	var keys map[string]any
	keysErr := toml.Unmarshal(data, &keys)
	_, bearerPresent := keys["crm_bearer_auth"]
	if c.hasCredentialFields() || bearerPresent || keysErr != nil {
		c.currentKeyDeclared = c.hasCredentialFields() || bearerPresent
		return
	}
	var doc struct {
		BearerAuth string `toml:"bearer_auth"`
		APIKey     string `toml:"api_key"`
	}
	if toml.Unmarshal(data, &doc) != nil {
		return
	}
	key, token := "", ""
	switch {
	case strings.TrimSpace(doc.BearerAuth) != "":
		key, token = "bearer_auth", doc.BearerAuth
	case strings.TrimSpace(doc.APIKey) != "":
		key, token = "api_key", doc.APIKey
	default:
		return
	}
	c.ConduytCrmBearerAuth = token
	c.legacyKey = &legacyKeySource{Path: path, Key: key, Token: token}
}

// deferLegacyConfigToken runs right after the config document is read: a token that came from an earlier config key
// and survived the trust checks is held back, so the credentials files' current fields are read as if it were not
// there.
func (c *Config) deferLegacyConfigToken() {
	if c.currentKeyDeclared || envNameDecides("CONDUYT_CRM_BEARER_AUTH") {
		c.legacyDisabled = true // sticky for the rest of the resolution
	}
	src := c.legacyKey
	c.legacyKey = nil
	if src == nil || c.ConduytCrmBearerAuth != src.Token {
		return
	}
	c.ConduytCrmBearerAuth = ""
	c.legacyConfigToken = src
}

// legacyAllowed is the single rule every earlier-key and earlier-env-name use goes through: true only while no
// trusted source has declared a current credential of any kind. legacyDisabled records the declarations seen while
// reading the config document and the credentials files (sticky); CONDUYT_CRM_BEARER_AUTH set (even blank) and any
// current field holding a value are declarations too.
func (c *Config) legacyAllowed() bool {
	return !c.legacyDisabled && !envNameDecides("CONDUYT_CRM_BEARER_AUTH") && !c.hasCredentialFields()
}

// resolveLegacyTokens runs after the credentials files: an earlier-key token (the config document's first, then the
// global credentials file's, as the earlier entry read them) is used only when no current credential of any kind was
// found, so it ranks below auth_header, crm_bearer_auth, access_token and refresh/client credentials.
func (c *Config) resolveLegacyTokens() {
	cfgSrc, globalSrc := c.legacyConfigToken, c.legacyGlobalToken
	c.legacyConfigToken, c.legacyGlobalToken = nil, nil
	if !c.legacyAllowed() {
		return
	}
	// The token is held in legacyToken, never in a persisted field, so the generated set-token and logout saves can
	// not write it back as a current key.
	switch {
	case cfgSrc != nil:
		c.legacyToken, c.legacyKey = cfgSrc.Token, cfgSrc
		c.AuthSource, c.CredentialSource = "config", c.configOwner
	case globalSrc != nil:
		c.legacyToken, c.legacyKey = globalSrc.Token, globalSrc
		c.AuthSource, c.CredentialSource = "config", "credentials file"
	}
}

// noteLegacyKey prints one notice when an earlier saved key supplied the token still in effect.
func (c *Config) noteLegacyKey() {
	src := c.legacyKey
	if src == nil || c.legacyToken != src.Token || c.hasCredentialFields() {
		// an env token (or any current credential) supersedes it
		c.legacyKey = nil
		return
	}
	legacyAuthNotice("key:"+src.Path, "%s saves the token under the earlier key %s; it is used only while no current credential exists. Save the token again with '%s'", src.Path, src.Key, legacyResaveCommand)
}

// envNameDecides reports whether an env var is set to a real value, including an empty or blank one: a set name
// decides, so a blank value means "explicitly cleared" and never falls through to a later name. An unset name, or an
// unresolved MCP bundle placeholder (a host that did not configure it), does not decide.
func envNameDecides(name string) bool {
	v, set := os.LookupEnv(name)
	return set && !cliutil.UnresolvedUserConfigPlaceholder(v)
}

// legacyEnvBearer returns the legacy env var that supplies the token, with its value: the first legacy name that is
// set decides, and a blank one supplies nothing.
func legacyEnvBearer() (name, value string) {
	for _, n := range legacyAuthEnvNames {
		if !envNameDecides(n) {
			continue
		}
		if v := cliutil.EnvOverride(n); strings.TrimSpace(v) != "" {
			return n, v
		}
		return "", ""
	}
	return "", ""
}

// applyLegacyEnvBearer is the env fallback: CONDUYT_CRM_BEARER_AUTH (applied by Load) wins; otherwise
// CONDUYT_BEARER_AUTH, then CONDUYT_API_KEY, each with a one-line deprecation notice.
func (c *Config) applyLegacyEnvBearer() {
	if !c.legacyAllowed() { // a declared current credential of any kind, from any trusted source or the env, decides
		return
	}
	name, v := legacyEnvBearer()
	if name == "" {
		return
	}
	c.ConduytCrmBearerAuth = v
	c.markEnvOverride("ConduytCrmBearerAuth")
	c.AuthSource = "env:" + name
	c.CredentialSource = "env:" + name
	legacyAuthNotice("env:"+name, "%s is deprecated; set CONDUYT_CRM_BEARER_AUTH instead (the value is used for now)", name)
}

// EffectiveAuthEnv names the env var that supplies the token under Load's own rules, or "" when none does:
// CONDUYT_CRM_BEARER_AUTH decides when it is set (an unresolved MCP placeholder does not count as set) and supplies a
// token when its effective value is non-empty; otherwise the first set legacy name decides. Logout reports exactly
// this name.
func EffectiveAuthEnv() string {
	if envNameDecides("CONDUYT_CRM_BEARER_AUTH") {
		if cliutil.EnvOverride("CONDUYT_CRM_BEARER_AUTH") != "" {
			return "CONDUYT_CRM_BEARER_AUTH"
		}
		return ""
	}
	name, _ := legacyEnvBearer()
	return name
}
