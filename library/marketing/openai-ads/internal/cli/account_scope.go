package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/marketing/openai-ads/internal/config"
)

// activeConfigPath mirrors the resolved --config flag so local-store path
// resolution can load the same credentials the API client will authenticate
// with. It is set once from the root PersistentPreRunE, alongside the existing
// --home override, and read by accountScopeSuffix.
//
// A package-level value is used deliberately: defaultDBPath is called from 16
// generated call sites that have no access to rootFlags, and threading a new
// parameter through all of them would mean editing generated code that regen
// would overwrite.
var (
	activeConfigPathMu sync.RWMutex
	activeConfigPath   string
)

// setActiveConfigPath records the --config value for account scoping.
func setActiveConfigPath(path string) {
	activeConfigPathMu.Lock()
	activeConfigPath = path
	activeConfigPathMu.Unlock()
}

func configPathForScope() string {
	activeConfigPathMu.RLock()
	defer activeConfigPathMu.RUnlock()
	return activeConfigPath
}

// accountScopeSuffix returns a short, non-reversible discriminator derived from
// the *effective* Ads credential, or "" when no credential is resolvable.
//
// Each OpenAI Ads API key is scoped to exactly one ad account (see
// https://developers.openai.com/ads/api-reference/authentication). Without a
// per-credential local store, pointing the CLI at a second ad account would
// read, mix, and extend the previous account's campaigns, ads, and snapshot
// history under the same data.db.
//
// The credential is resolved through config.Load using the same config path the
// command itself will use, so every supported auth path participates in
// scoping: an environment variable, a token persisted by 'auth set-token' into
// the credentials file, and an explicit --config pointing at another account's
// credentials all select their own database. config.Load applies the documented
// precedence (credentials file first, environment override last), so this
// mirrors exactly what the API client will send.
//
// The suffix is a truncated SHA-256 of the credential: it never reveals the
// key, and it is stable for as long as that key is in use. Rotating a key
// starts a fresh mirror, which is the conservative outcome — a stale mirror is
// re-syncable with one 'sync', whereas cross-account contamination is not
// detectable after the fact.
func accountScopeSuffix() string {
	cred := effectiveAdsCredential()
	if cred == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(cred))
	return hex.EncodeToString(sum[:])[:12]
}

// effectiveAdsCredential returns the Ads API key the client would actually
// authenticate with, or "" when none is configured. Failures to load config are
// non-fatal: an unscoped database is the pre-existing behavior and is
// preferable to blocking every local command on a config parse error.
func effectiveAdsCredential() string {
	cfg, err := config.Load(configPathForScope())
	if err != nil || cfg == nil {
		return ""
	}
	// Hash the same secret AuthHeader() actually sends. auth set-token
	// persists AccessToken (and optionally AuthHeaderVal); hashing only
	// OpenaiAdsApiKey left a stored-token login for a second ad account
	// on the shared data.db.
	if v := strings.TrimSpace(cfg.AuthHeaderVal); v != "" {
		return v
	}
	if v := strings.TrimSpace(cfg.OpenaiAdsApiKey); v != "" {
		return v
	}
	if v := strings.TrimSpace(cfg.OpenaiAdsConversionsApiKey); v != "" {
		return v
	}
	if v := strings.TrimSpace(cfg.AccessToken); v != "" {
		return v
	}
	return ""
}

// scopedDBFilename returns the account-scoped SQLite filename.
func scopedDBFilename() string {
	if suffix := accountScopeSuffix(); suffix != "" {
		return "data-" + suffix + ".db"
	}
	return "data.db"
}
