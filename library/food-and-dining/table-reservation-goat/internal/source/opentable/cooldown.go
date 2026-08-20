// OpenTable's Akamai bot defense periodically flags the Surf TLS fingerprint
// after a burst of requests (or after specific anti-bot signals trip), and
// the resulting 403 lasts much longer than a normal 429 backoff — minutes
// to hours. The 429-aware limiter doesn't recover from this because it's
// not a rate-limit signal at all: a 403 means "we don't trust this
// connection," not "you're going too fast."
//
// This file adds three things on top of the limiter:
//
//   1. A typed `*BotDetectionError` distinct from `*cliutil.RateLimitError`
//      so callers can render a useful "Akamai cooldown — retry after X"
//      message instead of an opaque 403.
//   2. A disk-persisted cooldown that survives across CLI invocations.
//      Without it, every fresh `goat`/`earliest`/`watch tick` invocation
//      would hit the home-page bootstrap, get 403'd, and waste 30s of the
//      user's time before failing.
//   3. Exponential backoff per consecutive 403 (5 min → 10 min → 20 min
//      → 40 min → cap at 60 min) so escalation is automatic but bounded.
//
// `Bootstrap()` and `do429Aware()` consult `loadCooldown()` before firing
// any HTTP request and call `setCooldown()` when they observe a 403.

package opentable

// PATCH: cross-network-source-clients — see .printing-press-patches.json for the change-set rationale.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// BotDetectionKind enumerates the recovery shapes for an anti-bot
// block. The CLI-layer JSON encoder surfaces this verbatim so agents
// can branch on it without parsing free-text Reason strings (issue
// #406 failure 5).
//
//	BotKindSessionBlocked — session-wide block (Bootstrap 403, all
//	  downstream ops will fail). Recovery: `auth login --chrome` to
//	  refresh Akamai cookies. The CLI sets a disk-persisted cooldown
//	  so subsequent invocations fast-fail rather than re-hitting the
//	  wall.
//
//	BotKindOperationBlocked — single-opname WAF rule (e.g. Akamai
//	  specifically blocks `opname=RestaurantsAvailability`). Sibling
//	  ops on the same session still work. Recovery: pass a numeric
//	  OpenTable ID instead (bypasses Autocomplete entirely), use the
//	  chromedp escape hatch, or surface the venue URL so the user
//	  can click through.
type BotDetectionKind string

const (
	// BotKindSessionBlocked: the entire OT session is shadow-banned.
	// All ops will fail until cookies are refreshed.
	BotKindSessionBlocked BotDetectionKind = "session_blocked"

	// BotKindOperationBlocked: a single GraphQL opname is blocked at
	// the WAF tier. Other ops on the same session work fine.
	BotKindOperationBlocked BotDetectionKind = "operation_blocked"
)

// BotDetectionError signals a persistent anti-bot block (403 from the home
// page or another well-known endpoint). Distinct from `cliutil.RateLimitError`
// because the remediation is different: rate-limit recovery is "back off
// briefly," bot-detection recovery is "wait minutes-to-hours and possibly
// rotate the fingerprint."
//
// The Kind field discriminates between session-wide and operation-
// specific blocks so agents can branch on the recovery strategy
// without parsing Reason. Issue #406 failure 5: prior versions buried
// this distinction in Reason text only — agents that didn't parse it
// couldn't tell whether to refresh cookies or pivot to a different op.
type BotDetectionError struct {
	Kind   BotDetectionKind
	URL    string
	Status int
	Until  time.Time // when the cached cooldown expires; zero means "now"
	Streak int       // how many consecutive 403s have been observed
	Reason string    // human-readable cause
}

func (e *BotDetectionError) Error() string {
	wait := time.Until(e.Until).Round(time.Second)
	if wait < 0 {
		wait = 0
	}
	switch e.Kind {
	case BotKindOperationBlocked:
		return fmt.Sprintf("opentable: operation blocked by Akamai WAF (status=%d) — the specific GraphQL opname is on a blocklist, but sibling ops still work; reason: %s; recovery: use a numeric restaurant ID via `restaurants list` to bypass Autocomplete, or use the chromedp escape hatch (set TABLE_RESERVATION_GOAT_OT_CHROME_DEBUG_URL)",
			e.Status, e.Reason)
	default:
		// Default and BotKindSessionBlocked: full session cooldown.
		return fmt.Sprintf("opentable: anti-bot cooldown (kind=session_blocked, status=%d, streak=%d) — retry after %s (%s); reason: %s; recovery: close Chrome briefly and run `auth login --chrome` to refresh Akamai cookies",
			e.Status, e.Streak, wait, e.Until.Format(time.RFC3339), e.Reason)
	}
}

// cooldownState is the on-disk shape. Persisted to a per-user cache file so
// a fresh CLI invocation can fast-fail rather than re-hit the bot-blocked
// home page.
type cooldownState struct {
	Until  time.Time `json:"until"`
	Streak int       `json:"streak"`
	Reason string    `json:"reason"`
}

// cooldownPath returns the on-disk cooldown file path. Honors
// $TABLE_RESERVATION_GOAT_CONFIG_DIR for parity with the auth.SessionPath.
func cooldownPath() (string, error) {
	if env := os.Getenv("TABLE_RESERVATION_GOAT_CONFIG_DIR"); env != "" {
		return filepath.Join(env, "opentable-cooldown.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "table-reservation-goat-pp-cli", "opentable-cooldown.json"), nil
}

// loadActiveCooldown reads the on-disk cooldown and returns a typed
// `*BotDetectionError` if a cooldown is currently active. Returns nil
// when there is no cooldown or the cooldown has expired (and removes
// the stale file). nil-safe to call on every request.
func loadActiveCooldown() *BotDetectionError {
	path, err := cooldownPath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil // file doesn't exist — no cooldown
	}
	var s cooldownState
	if err := json.Unmarshal(data, &s); err != nil {
		// Corrupt file — remove it and treat as no cooldown.
		_ = os.Remove(path)
		return nil
	}
	if time.Now().After(s.Until) {
		// Stale — clean up so future runs don't keep reading it.
		_ = os.Remove(path)
		return nil
	}
	return &BotDetectionError{
		Kind:   BotKindSessionBlocked,
		URL:    Origin + "/",
		Status: 403,
		Until:  s.Until,
		Streak: s.Streak,
		Reason: s.Reason,
	}
}

// setCooldown writes a cooldown to disk with exponential backoff based on
// the previous streak. Floor 5min, ceiling 60min, doubling per consecutive
// 403. Idempotent within a single process — repeated calls during the same
// burst extend `Until` but do not pile up.
func setCooldown(reason string) (*BotDetectionError, error) {
	path, err := cooldownPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("creating cooldown directory: %w", err)
	}
	prevStreak := 0
	if existing := loadActiveCooldown(); existing != nil {
		prevStreak = existing.Streak
	}
	streak := prevStreak + 1
	wait := 5 * time.Minute
	for i := 1; i < streak; i++ {
		wait *= 2
		if wait > 60*time.Minute {
			wait = 60 * time.Minute
			break
		}
	}
	until := time.Now().Add(wait)
	state := cooldownState{
		Until:  until,
		Streak: streak,
		Reason: reason,
	}
	js, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling cooldown: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, js, 0o600); err != nil {
		return nil, fmt.Errorf("writing cooldown: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return nil, fmt.Errorf("renaming cooldown file: %w", err)
	}
	return &BotDetectionError{
		Kind:   BotKindSessionBlocked,
		URL:    Origin + "/",
		Status: 403,
		Until:  until,
		Streak: streak,
		Reason: reason,
	}, nil
}

// clearCooldown removes the on-disk cooldown file. Called when a successful
// 200 response indicates the cooldown has lifted, so the next 403 starts a
// fresh streak count rather than escalating from a stale prior streak.
func clearCooldown() {
	path, err := cooldownPath()
	if err != nil {
		return
	}
	_ = os.Remove(path)
}

// IsBotDetection reports whether err is or wraps a *BotDetectionError.
// Callers use this to render a friendlier message than the generic 403
// string. Mirrors the `errors.As` pattern.
func IsBotDetection(err error) (*BotDetectionError, bool) {
	var bde *BotDetectionError
	if errors.As(err, &bde) {
		return bde, true
	}
	return nil, false
}
