// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package granola

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola/safestorage"
)

// WorkOSClientID is the Granola desktop client's WorkOS application client
// id. It is hardcoded across the community ecosystem (getprobo, granola.py,
// granola-mcp) and kept here for reference / ecosystem compatibility.
// The active refresh flow now uses GranolaRefreshEndpoint directly and no
// longer sends this client_id in the request body.
const WorkOSClientID = "client_01HJK46TGGY2DFQ2NX9P9XYJZN"

// GranolaRefreshEndpoint is Granola desktop's refresh endpoint. Modern
// Granola tokens are refreshed through Granola's API rather than by calling
// WorkOS directly; the older WorkOS client_id flow can return invalid_client
// for current desktop sessions.
const GranolaRefreshEndpoint = "https://api.granola.ai/v1/refresh-access-token"

// granolaSupportDir is the macOS support directory for Granola.
func granolaSupportDir() string {
	if v := os.Getenv("GRANOLA_SUPPORT_DIR"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "Granola")
}

// supabaseJSONPath returns the path to the supabase.json token file.
func supabaseJSONPath() string {
	return filepath.Join(granolaSupportDir(), "supabase.json")
}

// storedAccountsPath returns the path to the stored-accounts.json fallback.
func storedAccountsPath() string {
	return filepath.Join(granolaSupportDir(), "stored-accounts.json")
}

// workosTokens is the inner shape of the (stringified) workos_tokens blob.
type workosTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ObtainedAt   int64  `json:"obtained_at"` // millis since epoch
	ExpiresIn    int    `json:"expires_in"`  // seconds
	TokenType    string `json:"token_type"`
	SignInMethod string `json:"sign_in_method"`
	ExternalID   string `json:"external_id"`
	SessionID    string `json:"session_id"`
}

// supabaseFile is the top-level shape of supabase.json. workos_tokens is a
// JSON-stringified blob, hence the json.RawMessage indirection.
type supabaseFile struct {
	SessionID    string          `json:"session_id"`
	UserInfo     json.RawMessage `json:"user_info"`
	WorkOSTokens json.RawMessage `json:"workos_tokens"`
}

// storedAccountsFile is the top-level shape of stored-accounts.json.
type storedAccountsFile struct {
	Accounts json.RawMessage `json:"accounts"`
}

// storedAccount is one entry in the (stringified) accounts array.
type storedAccount struct {
	Email    string          `json:"email"`
	UserID   string          `json:"userId"`
	SavedAt  string          `json:"savedAt"`
	Tokens   json.RawMessage `json:"tokens"`
	UserInfo json.RawMessage `json:"userInfo"`
}

// LoadAccessToken returns the current access token + its expiry time.
// It returns the in-memory cached token if RefreshAccessToken has been
// called this session and minted a newer one; otherwise it reads
// supabase.json.enc (preferred), then plaintext supabase.json, then
// stored-accounts.json, then env GRANOLA_WORKOS_TOKEN.
//
// The returned expiry is computed from obtained_at + expires_in. Callers
// SHOULD compare against time.Now() and call RefreshAccessToken if it has
// passed. The reverse path (network call) is what the InternalClient does
// on 401 from the API itself; both paths share the in-process cache.
//
// PATCH(encrypted-cache): the load now tracks where the token came from,
// so RefreshAccessToken can enforce D6 (refuse to rotate a refresh token
// read from the encrypted supabase store).
func LoadAccessToken() (string, time.Time, error) {
	tokenMu.Lock()
	defer tokenMu.Unlock()
	if cachedAccess != "" && !cachedExpiry.IsZero() {
		return cachedAccess, cachedExpiry, nil
	}
	tok, src, err := loadTokensRaw()
	if err != nil {
		return "", time.Time{}, err
	}
	cachedAccess = tok.AccessToken
	cachedRefresh = tok.RefreshToken
	cachedExpiry = tok.expiry()
	cachedSource = src
	return cachedAccess, cachedExpiry, nil
}

// LoadRefreshToken returns the refresh token (read-only).
func LoadRefreshToken() (string, error) {
	tokenMu.Lock()
	defer tokenMu.Unlock()
	if cachedRefresh != "" {
		return cachedRefresh, nil
	}
	tok, src, err := loadTokensRaw()
	if err != nil {
		return "", err
	}
	cachedAccess = tok.AccessToken
	cachedRefresh = tok.RefreshToken
	cachedExpiry = tok.expiry()
	cachedSource = src
	return cachedRefresh, nil
}

// TokenSource enumerates where the in-process cached token came from.
// PATCH(encrypted-cache): RefreshAccessToken consults this to enforce D6
// (read-only against the encrypted supabase store) - refreshing a token
// read from supabase.json.enc would invalidate Granola desktop's stored
// refresh token next time the desktop tries to refresh.
type TokenSource int

const (
	TokenSourceUnknown TokenSource = iota
	// TokenSourceEnvOverride: GRANOLA_WORKOS_TOKEN was set. User opt-in
	// to manage refresh themselves; D6 allows refresh on this source.
	TokenSourceEnvOverride
	// TokenSourcePlaintextSupabase: pre-encryption Granola wrote a
	// plaintext supabase.json. Refresh allowed on this source for the
	// same reason - the user is on legacy Granola where rotation is
	// not coordinated with the desktop's encrypted store.
	TokenSourcePlaintextSupabase
	// TokenSourceEncryptedSupabase: token came from supabase.json.enc.
	// D6: refresh refused on this source to avoid signing the user out
	// of Granola desktop.
	TokenSourceEncryptedSupabase
	// TokenSourcePlaintextSupabaseDesktopFallback: supabase.json.enc was
	// present but unavailable because Keychain access failed, so the token
	// was read from plaintext supabase.json. Treat this as desktop-owned
	// for D6 because the plaintext and encrypted files may share the same
	// single-use refresh token.
	TokenSourcePlaintextSupabaseDesktopFallback
	// TokenSourceStoredAccounts: stored-accounts.json fallback. D6: refresh
	// refused whenever supabase.json.enc is present, because the desktop
	// owns the account store on any install that also has an encrypted
	// store - and a desktop key migration means the load now falls through
	// to this file on exactly those installs. Refresh stays allowed on
	// genuinely legacy installs that never had an encrypted store.
	TokenSourceStoredAccounts
	// PATCH(cli-owned-workos-session): TokenSourceCLISession: a session this
	// CLI obtained itself through the device authorization grant and stores
	// under its own data directory. This is the one chain the CLI owns, so D6
	// does not apply -- refreshing it cannot sign the desktop app or the
	// browser out, because neither holds it.
	TokenSourceCLISession
)

// in-process token cache. Survives across multiple calls in one CLI run.
var (
	tokenMu       sync.Mutex
	cachedAccess  string
	cachedRefresh string
	cachedExpiry  time.Time
	cachedSource  TokenSource // PATCH(encrypted-cache): see TokenSource
	// refreshClient is the HTTP client used for refresh calls. Tests may
	// override it with a transport that mocks WorkOS responses.
	refreshClient = &http.Client{Timeout: 15 * time.Second}
	// refreshMu serializes the exchange-and-persist for the CLI-owned chain.
	// Separate from tokenMu on purpose: tokenMu guards short in-process reads
	// and must not be held across an HTTP round trip, while this one must span
	// the whole rotation because the chain is single-use.
	refreshMu sync.Mutex
)

// ErrRefreshRefused is returned by RefreshAccessToken when the in-memory
// token came from a store Granola desktop owns. Refreshing would invalidate
// the rotated token the desktop still holds on disk, signing the user
// out. Callers should surface the contained message to the user and
// suggest waking Granola desktop. See plan D6. Refusals for sources other
// than the encrypted store wrap this sentinel, so match it with errors.Is.
var ErrRefreshRefused = errors.New("safestorage: refresh refused for encrypted source - open Granola desktop briefly to refresh, then retry")

// refreshRefusedError names the desktop-owned source that blocked a refresh
// while still matching ErrRefreshRefused via errors.Is, so callers keep the
// same "wake Granola desktop" handling regardless of which store the token
// was read from.
type refreshRefusedError struct{ source string }

func (e *refreshRefusedError) Error() string {
	return fmt.Sprintf("safestorage: refresh refused for %s source - open Granola desktop briefly to refresh, then retry", e.source)
}

func (e *refreshRefusedError) Unwrap() error { return ErrRefreshRefused }

// refreshRefusalFor returns the D6 refusal for a desktop-owned token source,
// or nil when refresh is allowed.
//
// PATCH(d6-read-only-applies-to-all-desktop-token-sources): stored-accounts.json
// is desktop-owned too whenever supabase.json.enc sits beside it. A Granola
// desktop key migration made that encrypted store undecryptable here, so the
// stored-accounts fallback is the live path on current installs and its
// refresh token is the single-use one the desktop still holds - rotating it
// signs the desktop out. Gating on the encrypted store's presence rather than
// refusing outright preserves refresh for genuinely legacy installs that never
// had one.
func refreshRefusalFor(src TokenSource) error {
	switch src {
	case TokenSourceCLISession:
		// PATCH(cli-owned-workos-session): never refused. The CLI owns this
		// chain outright; no other client holds it, so rotating it cannot
		// strand anyone. Every desktop-owned arm below is unchanged.
		return nil
	case TokenSourceEncryptedSupabase, TokenSourcePlaintextSupabaseDesktopFallback:
		return ErrRefreshRefused
	case TokenSourceStoredAccounts:
		if _, err := os.Stat(supabaseJSONPath() + ".enc"); err == nil {
			return &refreshRefusedError{source: "stored-accounts"}
		}
	}
	return nil
}

// sessionGeneration identifies a published version of the stored session.
//
// Keying rotation detection on the refresh token changing was wrong: Granola's
// refresh proxy is observed to return the SAME refresh token, so a waiter would
// never see the winner's result and would time out despite a valid new access
// token sitting on disk. ObtainedAt advances on every persisted rotation
// regardless, and the access token is carried too so a same-instant rotation is
// still caught.
type sessionGeneration struct {
	accessToken string
}

// beforeAcquireHook fires between the first generation check and the marker
// acquire. Nil in production; a test sets it to simulate a winner publishing in
// exactly that window, which is otherwise not reproducible.
var beforeAcquireHook func()

// heldSessionGeneration describes the credential this process currently holds.
func heldSessionGeneration() sessionGeneration {
	tokenMu.Lock()
	defer tokenMu.Unlock()
	return sessionGeneration{accessToken: cachedAccess}
}

// ErrSessionLoggedOutDuringRotation reports that the session was removed while
// a refresh was in flight. The rotation is abandoned rather than completed:
// finishing it would recreate the credential logout just deleted.
var ErrSessionLoggedOutDuringRotation = errors.New("granola: session was logged out during rotation")

// adoptRotated installs another process's rotation result into this process's
// token cache.
//
// Returning the winner's tokens without this leaves cachedAccess holding the
// token the server already rejected, so the 401 retry that triggered the
// refresh would replay the same dead credential and fail again. The caller is
// refreshing precisely because its cached token is no good.
func adoptRotated(r RefreshAccessTokenResponse) RefreshAccessTokenResponse {
	tokenMu.Lock()
	defer tokenMu.Unlock()
	cachedAccess = r.AccessToken
	if r.RefreshToken != "" {
		cachedRefresh = r.RefreshToken
	}
	if r.ExpiresIn > 0 {
		cachedExpiry = time.Now().Add(time.Duration(r.ExpiresIn) * time.Second)
	}
	return r
}

// awaitRotatedCLISession waits, briefly and bounded, for the process holding
// the rotation marker to publish its result. A short wait is right: the holder
// is mid-HTTP-exchange, and the alternative -- returning immediately -- pushes a
// spurious failure at a caller whose session is about to be perfectly valid.
// Giving up returns false rather than exchanging, because spending a single-use
// chain twice is the outcome this whole path exists to prevent.
func awaitRotatedCLISession(seen sessionGeneration) (RefreshAccessTokenResponse, bool) {
	const (
		window = 10 * time.Second
		step   = 100 * time.Millisecond
	)
	deadline := time.Now().Add(window)
	for time.Now().Before(deadline) {
		time.Sleep(step)
		if fresh, ok := rotatedCLISessionToken(seen); ok {
			return fresh, true
		}
		// The holder cleared its marker without rotating -- it failed
		// harmlessly, so there is nothing to wait for.
		if _, err := os.Stat(rotationMarkerPath()); err != nil {
			// One more read before giving up. A save renames the session and
			// then removes the marker, so the holder can publish in the gap
			// between the read above and this stat -- and giving up here would
			// report "rotation in progress" while the fresh token it produced is
			// already sitting on disk.
			if fresh, ok := rotatedCLISessionToken(seen); ok {
				return fresh, true
			}
			return RefreshAccessTokenResponse{}, false
		}
	}
	return RefreshAccessTokenResponse{}, false
}

// rotatedCLISessionToken reports whether the stored session has advanced past
// the generation this caller snapshotted, and returns it when so. That is the
// signal that another goroutine or process won the race; reusing its result is
// what stops the loser from spending a single-use chain twice.
// lastConsumedRefresh is the refresh token most recently spent by a completed
// exchange in this process. Guarded by refreshMu. It exists so a caller holding
// that exact token can recognise it as already spent even when the cache
// comparison cannot see the rotation.
var (
	consumedMu          sync.Mutex
	lastConsumedRefresh string
)

// recordConsumedRefresh and isConsumedRefresh guard the ledger with their own
// mutex rather than refreshMu. ResetTokenCache clears it and is itself called
// from inside the refreshMu-held logout branch, so reusing refreshMu here would
// deadlock the very path that abandons a rotation.
func recordConsumedRefresh(tok string) {
	consumedMu.Lock()
	lastConsumedRefresh = tok
	consumedMu.Unlock()
}

func isConsumedRefresh(tok string) bool {
	consumedMu.Lock()
	defer consumedMu.Unlock()
	return tok != "" && tok == lastConsumedRefresh
}

func rotatedCLISessionToken(seen sessionGeneration) (RefreshAccessTokenResponse, bool) {
	s, err := loadCLISessionUnmarked()
	if err != nil || s.AccessToken == "" {
		return RefreshAccessTokenResponse{}, false
	}
	// Advanced when the stored credential is not the one this caller holds.
	advanced := seen.accessToken != "" && s.AccessToken != seen.accessToken
	if !advanced {
		return RefreshAccessTokenResponse{}, false
	}
	// Deliberately no freshness gate. "Someone else rotated" and "the result
	// looks fresh" are different questions, and answering the first with the
	// second fails in the dangerous direction: a winner whose response omitted
	// expires_in publishes a new access token over a stale expiry, and the loser
	// would read that as "no rotation happened" and exchange the same single-use
	// refresh token a second time. A token that turns out to be expired costs a
	// 401 and a retry; a double exchange breaks the chain for good.
	remaining := time.Until(s.ExpiresAt)
	if s.ExpiresAt.IsZero() || remaining < 0 {
		remaining = 0
	}
	return RefreshAccessTokenResponse{
		AccessToken:  s.AccessToken,
		RefreshToken: s.RefreshToken,
		ExpiresIn:    int(remaining.Seconds()),
	}, true
}

// SetRefreshHTTPClient swaps the HTTP client used for WorkOS refreshes.
// Tests use this to inject mocked transports.
func SetRefreshHTTPClient(c *http.Client) {
	tokenMu.Lock()
	defer tokenMu.Unlock()
	refreshClient = c
}

// ResetTokenCache clears the in-process token cache. Tests call this to
// force re-reading the on-disk source.
func ResetTokenCache() {
	tokenMu.Lock()
	defer tokenMu.Unlock()
	cachedAccess = ""
	cachedRefresh = ""
	cachedExpiry = time.Time{}
	cachedSource = TokenSourceUnknown
	recordConsumedRefresh("")
}

// CurrentTokenSource returns the origin of the in-process cached token.
// Returns TokenSourceUnknown if no token has been loaded this session.
// PATCH(encrypted-cache): doctor and internalapi consult this to decide
// whether refresh is allowed (D6).
func CurrentTokenSource() TokenSource {
	tokenMu.Lock()
	defer tokenMu.Unlock()
	return cachedSource
}

func (t workosTokens) expiry() time.Time {
	if t.ObtainedAt == 0 || t.ExpiresIn == 0 {
		return time.Time{}
	}
	obtained := time.UnixMilli(t.ObtainedAt)
	return obtained.Add(time.Duration(t.ExpiresIn) * time.Second)
}

// loadTokensRaw reads the on-disk token, trying the env override first,
// then supabase.json (.enc preferred), then stored-accounts.json.
// PATCH(encrypted-cache): returns the source alongside the token so the
// refresh path can enforce D6.
func loadTokensRaw() (workosTokens, TokenSource, error) {
	// Env-var override path (power users + tests + CI smoke).
	if v := os.Getenv("GRANOLA_WORKOS_TOKEN"); v != "" {
		// Synthesize an expiry far in the future.
		return workosTokens{
			AccessToken:  v,
			RefreshToken: os.Getenv("GRANOLA_WORKOS_REFRESH"),
			ObtainedAt:   time.Now().UnixMilli(),
			ExpiresIn:    3600,
			TokenType:    "Bearer",
		}, TokenSourceEnvOverride, nil
	}

	// PATCH(cli-owned-workos-session): the CLI's own session outranks every
	// desktop-owned probe below. It is the only chain the CLI may refresh, and
	// on a post-migration install it is the only one that still works at all --
	// supabase.json.enc needs the entitlement-gated DEK, and the plaintext
	// files Granola stopped writing are stale fossils. The env override stays
	// ahead of it so a deliberate GRANOLA_WORKOS_TOKEN still wins.
	if s, err := LoadCLISession(); err == nil {
		expiresIn := 0
		if !s.ExpiresAt.IsZero() {
			expiresIn = int(time.Until(s.ExpiresAt).Seconds())
		}
		return workosTokens{
			AccessToken:  s.AccessToken,
			RefreshToken: s.RefreshToken,
			ObtainedAt:   s.ObtainedAt.UnixMilli(),
			ExpiresIn:    expiresIn,
			TokenType:    "Bearer",
		}, TokenSourceCLISession, nil
	} else if !errors.Is(err, ErrNoCLISession) {
		// A corrupt session, an interrupted rotation, or unsafe permissions are
		// states the user must be told about. Falling through to the desktop
		// probes here would replace an actionable message with a confusing
		// "no token found" three probes later.
		return workosTokens{}, TokenSourceUnknown, err
	}

	if tok, src, err := loadFromSupabaseJSON(); err == nil {
		return tok, src, nil
	}
	if tok, err := loadFromStoredAccountsJSON(); err == nil {
		return tok, TokenSourceStoredAccounts, nil
	}
	return workosTokens{}, TokenSourceUnknown, fmt.Errorf("no Granola token found in supabase.json (.enc or plaintext) or stored-accounts.json; sign into the Granola desktop app or set GRANOLA_WORKOS_TOKEN")
}

// PATCH(encrypted-cache): supabase.json.enc preferred over plaintext.
// Returns the parsed token plus a TokenSource flag so the caller knows
// whether D6's refresh-refusal applies.
func loadFromSupabaseJSON() (workosTokens, TokenSource, error) {
	// Prefer the .enc sibling when present, but do not let a blocked
	// Keychain prompt make headless/agent runs report "no token" when a
	// legacy/plaintext supabase.json fallback is also available. Other
	// encrypted-store errors still surface instead of silently falling back.
	encPath := supabaseJSONPath() + ".enc"
	if _, err := os.Stat(encPath); err == nil {
		tok, src, encErr := loadFromSupabaseEnc(encPath)
		if encErr == nil {
			return tok, src, nil
		}
		if errors.Is(encErr, safestorage.ErrKeyUnavailable) {
			plainPath := supabaseJSONPath()
			if _, statErr := os.Stat(plainPath); statErr == nil {
				if tok, _, plainErr := loadFromSupabasePlain(plainPath); plainErr == nil {
					return tok, TokenSourcePlaintextSupabaseDesktopFallback, nil
				} else {
					return workosTokens{}, TokenSourceUnknown, fmt.Errorf("supabase.json.enc: Keychain unavailable; supabase.json fallback also failed: %w", plainErr)
				}
			}
		}
		return workosTokens{}, TokenSourceUnknown, encErr
	}
	return loadFromSupabasePlain(supabaseJSONPath())
}

func loadFromSupabaseEnc(path string) (workosTokens, TokenSource, error) {
	cipher, err := os.ReadFile(path)
	if err != nil {
		return workosTokens{}, TokenSourceUnknown, err
	}
	plain, err := safestorage.Decrypt(cipher)
	if err != nil {
		if errors.Is(err, safestorage.ErrKeyUnavailable) {
			return workosTokens{}, TokenSourceUnknown, fmt.Errorf("supabase.json.enc: Keychain access unavailable (sign into Granola desktop or run `granola-pp-cli sync` to authorize): %w", err)
		}
		return workosTokens{}, TokenSourceUnknown, fmt.Errorf("supabase.json.enc: decrypt failed: %w", err)
	}
	defer safestorage.ZeroBytes(plain)
	tok, err := parseSupabaseBlob(plain, path)
	if err != nil {
		return workosTokens{}, TokenSourceUnknown, err
	}
	return tok, TokenSourceEncryptedSupabase, nil
}

func loadFromSupabasePlain(path string) (workosTokens, TokenSource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return workosTokens{}, TokenSourceUnknown, err
	}
	tok, err := parseSupabaseBlob(data, path)
	if err != nil {
		return workosTokens{}, TokenSourceUnknown, err
	}
	return tok, TokenSourcePlaintextSupabase, nil
}

func parseSupabaseBlob(data []byte, source string) (workosTokens, error) {
	var top supabaseFile
	if err := json.Unmarshal(data, &top); err != nil {
		return workosTokens{}, fmt.Errorf("%s: %w", source, err)
	}
	if len(top.WorkOSTokens) == 0 {
		return workosTokens{}, fmt.Errorf("%s: workos_tokens missing", source)
	}
	raw := unwrapStringifiedJSON(top.WorkOSTokens)
	var tok workosTokens
	if err := json.Unmarshal(raw, &tok); err != nil {
		return workosTokens{}, fmt.Errorf("%s: parsing workos_tokens: %w", source, err)
	}
	if tok.AccessToken == "" {
		return workosTokens{}, fmt.Errorf("%s: empty access_token", source)
	}
	return tok, nil
}

func loadFromStoredAccountsJSON() (workosTokens, error) {
	data, err := os.ReadFile(storedAccountsPath())
	if err != nil {
		return workosTokens{}, err
	}
	var top storedAccountsFile
	if err := json.Unmarshal(data, &top); err != nil {
		return workosTokens{}, err
	}
	if len(top.Accounts) == 0 {
		return workosTokens{}, fmt.Errorf("stored-accounts.json: accounts missing")
	}
	raw := unwrapStringifiedJSON(top.Accounts)
	var accts []storedAccount
	if err := json.Unmarshal(raw, &accts); err != nil {
		return workosTokens{}, fmt.Errorf("stored-accounts.json: parsing accounts: %w", err)
	}
	if len(accts) == 0 {
		return workosTokens{}, fmt.Errorf("stored-accounts.json: no accounts")
	}
	// Iterate every account; keep the newest by SavedAt with a parseable
	// tokens blob.
	var best workosTokens
	var bestSaved string
	for _, a := range accts {
		if len(a.Tokens) == 0 {
			continue
		}
		inner := unwrapStringifiedJSON(a.Tokens)
		var tok workosTokens
		if err := json.Unmarshal(inner, &tok); err != nil {
			continue
		}
		if tok.AccessToken == "" {
			continue
		}
		if a.SavedAt > bestSaved {
			best = tok
			bestSaved = a.SavedAt
		}
	}
	if best.AccessToken == "" {
		return workosTokens{}, fmt.Errorf("stored-accounts.json: no usable tokens")
	}
	return best, nil
}

// RefreshAccessTokenResponse is the parsed body from a Granola refresh call.
type RefreshAccessTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	// Granola returns the new expiry as expires_in (seconds) when present.
	ExpiresIn int `json:"expires_in"`
}

// workosLimiter paces Granola refresh-token calls. The endpoint is hit at most
// once per CLI invocation under normal conditions; the limiter is here for the
// pathological case where a caller burst-refreshes (and so the typed 429
// contract below is exercised by the AdaptiveLimiter as well).
var workosLimiter = cliutil.NewAdaptiveLimiter(2.0)

// RefreshAccessToken exchanges the current refresh token for a new
// access/refresh pair. Granola/WorkOS rotates refresh tokens single-use per
// getprobo's findings; the caller MUST persist the new refresh token if
// it intends to refresh again (we cache it in-process only - we do not
// write back to Granola's files).
//
// PATCH(encrypted-cache): refuses to refresh when the in-memory token came
// from a store Granola desktop owns - supabase.json.enc, plaintext
// supabase.json read as a fallback because the encrypted store was
// Keychain-blocked, or stored-accounts.json on an install that has an
// encrypted store (D6). Refreshing would mint a new refresh_token and
// invalidate the one Granola desktop still has on disk, signing the user out
// next time the desktop tries to refresh. The env override path
// (GRANOLA_WORKOS_TOKEN) is opt-in: power users who set it accept the
// desktop-sign-out trade-off. Legacy plaintext supabase.json and
// stored-accounts.json installs with no encrypted store still allow refresh.
func RefreshAccessToken(refreshToken string) (RefreshAccessTokenResponse, error) {
	// Hold tokenMu across the D6 check so a concurrent loadTokensRaw cannot
	// flip cachedSource to a desktop-owned source between the read and
	// the network call. Single-CLI-invocation processes don't see this race
	// in practice; long-running agents that call sync concurrently would.
	tokenMu.Lock()
	src := cachedSource
	if refusal := refreshRefusalFor(src); refusal != nil {
		tokenMu.Unlock()
		return RefreshAccessTokenResponse{}, refusal
	}
	tokenMu.Unlock()

	// PATCH(cli-owned-workos-session): serialize the whole exchange-and-persist
	// for the one chain the CLI owns, and mark it in flight first.
	//
	// tokenMu deliberately does not cover the network call -- holding it there
	// would block every concurrent reader for the duration of an HTTP round
	// trip. But an unserialized refresh is unsafe for a single-use chain: two
	// callers both exchange, the loser gets invalid_grant, and the session is
	// stranded with no recovery but a fresh login. A dedicated lock gives the
	// serialization without the read stall.
	//
	// The marker is written before the exchange rather than after, because the
	// unrecoverable window is precisely between a successful remote exchange
	// and a durable local write. A marker written afterwards would never
	// witness the crash it exists to detect.
	// exchanged flips once the remote call has returned a usable token, i.e.
	// once the presented refresh token may already be spent upstream.
	exchanged := false
	var rotationHandle RotationHandle
	if src == TokenSourceCLISession {
		// The reference point is what THIS caller holds, not what is on disk:
		// by the time we look, a winner may already have published, and a disk
		// snapshot would compare that result against itself and miss it. The
		// cached access token is the credential this caller was using when it
		// decided to refresh, so "stored differs from mine" is exactly the
		// question worth asking -- and it stays true when the provider
		// preserves the refresh token, which Granola's proxy is observed to do.
		// Snapshotted before the lock on purpose: it must record what this
		// caller was using when it decided to refresh, not what the winner
		// published while this caller was blocked. Sampling under the lock reads
		// the winner's own result, making "did anyone rotate?" compare the new
		// token against itself -- every loser then concludes nothing happened.
		seen := heldSessionGeneration()
		refreshMu.Lock()
		defer refreshMu.Unlock()
		// The snapshot above can still be taken in the narrow window after the
		// winner updates the cache and before it releases the lock, which would
		// hide the rotation. That case is caught exactly rather than
		// approximately: the winner records which refresh token it consumed, so
		// a caller presenting that same token knows its own credential is spent
		// and must adopt the winner's result instead of exchanging again.
		if isConsumedRefresh(refreshToken) {
			if stored, err := loadCLISessionUnmarked(); err == nil && stored.AccessToken != "" {
				remaining := time.Until(stored.ExpiresAt)
				if stored.ExpiresAt.IsZero() || remaining < 0 {
					remaining = 0
				}
				return adoptRotated(RefreshAccessTokenResponse{
					AccessToken:  stored.AccessToken,
					RefreshToken: stored.RefreshToken,
					ExpiresIn:    int(remaining.Seconds()),
				}), nil
			}
		}
		// Another goroutine may have completed a refresh while this one waited.
		// Detect that by the stored refresh token no longer matching the one
		// this caller presented, and hand back the winner's result rather than
		// spending the chain again. Keying on rotation rather than on remaining
		// validity matters: a caller refreshing after a 401 holds a token the
		// server has already rejected, and must not be told it is still good.
		if fresh, ok := rotatedCLISessionToken(seen); ok {
			return adoptRotated(fresh), nil
		}
		// Test seam: lets a test publish a winner in the check/acquire gap.
		if beforeAcquireHook != nil {
			beforeAcquireHook()
		}
		handle, err := BeginRotation()
		if errors.Is(err, ErrRotationInProgress) {
			// Another process holds the marker. Never exchange alongside it:
			// the chain is single-use, so a second exchange spends the token it
			// is rotating. Wait for it to publish, then reuse its result.
			if fresh, ok := awaitRotatedCLISession(seen); ok {
				return adoptRotated(fresh), nil
			}
			return RefreshAccessTokenResponse{}, err
		}
		if err != nil {
			return RefreshAccessTokenResponse{}, err
		}
		rotationHandle = handle
		// Registered before the re-check below, so an early return there still
		// releases the marker. The marker must survive exactly one outcome: the
		// remote exchange succeeded and the local write did not. That is the
		// unrecoverable window -- the presented token is dead upstream and no
		// replacement reached disk. Every other path left the stored token
		// untouched, so clearing it avoids sending the user to auth login over
		// a transport blip. On success SaveCLISession has already removed it.
		defer func() {
			if !exchanged {
				rotationHandle.Abort()
			}
		}()
		// Re-check now that the marker is held. The first check and the acquire
		// are not atomic: a winner can persist its session and drop the marker
		// in the gap between them, which would leave this caller holding a
		// fresh marker and about to spend a refresh token already consumed.
		// Checking again under the marker closes that window.
		if fresh, ok := rotatedCLISessionToken(seen); ok {
			return adoptRotated(fresh), nil
		}
	}

	body, _ := json.Marshal(map[string]string{
		"refresh_token": refreshToken,
	})
	req, err := http.NewRequest("POST", GranolaRefreshEndpoint, bytes.NewReader(body))
	if err != nil {
		return RefreshAccessTokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", granolaUserAgent)
	req.Header.Set("X-Client-Version", granolaClientVersion)
	req.Header.Set("X-Granola-Platform", granolaPlatform)
	workosLimiter.Wait()
	resp, err := refreshClient.Do(req)
	if err != nil {
		return RefreshAccessTokenResponse{}, fmt.Errorf("granola refresh: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	// Typed 429 handling — surface WorkOS throttling as cliutil.RateLimitError
	// so the caller can distinguish "rate limited" from "auth failed."
	if resp.StatusCode == http.StatusTooManyRequests {
		workosLimiter.OnRateLimit()
		wait := cliutil.RetryAfter(resp)
		return RefreshAccessTokenResponse{}, &cliutil.RateLimitError{
			URL:        GranolaRefreshEndpoint,
			RetryAfter: wait,
			Body:       string(respBody),
		}
	}
	workosLimiter.OnSuccess()
	if resp.StatusCode != http.StatusOK {
		return RefreshAccessTokenResponse{}, fmt.Errorf("granola refresh: status %d: %s", resp.StatusCode, string(respBody))
	}
	// A 200 is proof the endpoint rotated, so the presented token is spent from
	// here on regardless of what the body turns out to contain. Flipping this
	// on parse success instead would let a truncated or malformed body clear
	// the marker on an already-consumed token -- deleting the only evidence of
	// the state the marker exists to record.
	exchanged = true
	// Recorded at the same moment for the same reason: the server has committed,
	// so this refresh token is spent. A concurrent caller holding it must adopt
	// the result rather than present it again. Written under refreshMu, which
	// this function holds for its whole body.
	recordConsumedRefresh(refreshToken)
	var parsed RefreshAccessTokenResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return RefreshAccessTokenResponse{}, fmt.Errorf("granola refresh: parse response: %w", err)
	}
	if parsed.AccessToken == "" {
		return RefreshAccessTokenResponse{}, fmt.Errorf("granola refresh: empty access_token in response")
	}
	// Cache the new pair in-process.
	tokenMu.Lock()
	cachedAccess = parsed.AccessToken
	if parsed.RefreshToken != "" {
		cachedRefresh = parsed.RefreshToken
	}
	if parsed.ExpiresIn > 0 {
		cachedExpiry = time.Now().Add(time.Duration(parsed.ExpiresIn) * time.Second)
	}
	tokenMu.Unlock()

	// PATCH(cli-owned-workos-session): persist the rotated pair for a
	// CLI-owned session before returning.
	//
	// Granola's refresh proxy was observed returning the same refresh token
	// rather than a rotated one, so this is not currently a
	// consume-and-replace exchange. Persist anyway: the WorkOS endpoint behind
	// it *is* single-use, and if the proxy ever starts passing that through, an
	// unpersisted replacement would strand the session with no recovery but a
	// fresh login. Failing loudly beats returning a token whose refresh half
	// was silently lost.
	if src == TokenSourceCLISession {
		if err := persistRotatedCLISession(parsed, rotationHandle); err != nil {
			if errors.Is(err, ErrSessionLoggedOutDuringRotation) {
				// Logout won. The marker only exists to flag a session that
				// needs recovering, and there is no longer a session -- leaving
				// it would tell the user to re-login to repair something they
				// deliberately deleted. Clear it, and drop the in-process cache
				// too: the tokens just exchanged are still in memory, and
				// leaving them there would keep serving a credential the user
				// removed from disk.
				rotationHandle.Abort()
				ResetTokenCache()
			}
			return RefreshAccessTokenResponse{}, err
		}
		// The replacement is durable, so the window the marker guards is closed.
		rotationHandle.Complete()
	}
	return parsed, nil
}

// persistRotatedCLISession writes a refreshed pair back to the CLI-owned
// session file, preserving the account identity fields the refresh response
// does not carry.
func persistRotatedCLISession(parsed RefreshAccessTokenResponse, h RotationHandle) error {
	// Unmarked: this runs inside the refresh lock with our own in-flight marker
	// set, so the marked loader would refuse to read the session being replaced.
	s, err := loadCLISessionUnmarked()
	if errors.Is(err, ErrNoCLISession) {
		// The session vanished mid-rotation, which means `auth logout` ran.
		// Writing here would resurrect the credential the user just removed and
		// report logout as having succeeded, so abandon the rotation instead.
		// The exchanged token is discarded; the user is logged out, which is
		// what they asked for.
		return ErrSessionLoggedOutDuringRotation
	}
	if err != nil {
		return fmt.Errorf("granola refresh: reload session before persist: %w", err)
	}
	s.AccessToken = parsed.AccessToken
	if parsed.RefreshToken != "" {
		s.RefreshToken = parsed.RefreshToken
	}
	if parsed.ExpiresIn > 0 {
		s.ExpiresAt = time.Now().Add(time.Duration(parsed.ExpiresIn) * time.Second).UTC()
	}
	s.ObtainedAt = time.Now().UTC()
	if err := SaveCLISession(s); err != nil {
		return fmt.Errorf("granola refresh: persist rotated session: %w", err)
	}
	// The existence check above only closes the in-process race, which refreshMu
	// already covers. Across processes a logout can land between that check and
	// the rename, and the rename would then recreate the credential after logout
	// reported success. Re-read the revocation epoch now that the file is
	// published: if it moved, a logout happened somewhere inside this rotation,
	// so remove what was just written rather than leave the user signed in.
	if readRevocationEpoch() != h.Epoch() {
		if rmErr := os.Remove(CLISessionPath()); rmErr != nil && !os.IsNotExist(rmErr) {
			return fmt.Errorf("granola refresh: logout raced this rotation and the resurrected session could not be removed: %w", rmErr)
		}
		return ErrSessionLoggedOutDuringRotation
	}
	return nil
}
