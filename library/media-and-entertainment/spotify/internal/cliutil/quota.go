// Copyright 2026 Rob Zehner and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(amend-2026-08-08: quota circuit breaker + limiter ceiling persistence)
// Library-side addition, not generator output. Spotify's Web API enforces
// endpoint-class-scoped daily quotas whose 429s carry hours-scale Retry-After
// values (observed: Retry-After: 67209 on /search while /me kept returning
// 200). The generated retry loop clamps every Retry-After to MaxRetryWait
// (60s), so a multi-hour quota block became three futile 60s sleeps per
// invocation and the real reset time never reached the user. This file adds:
//
//   - RetryAfterUncapped: the unclamped parse, so callers can distinguish a
//     genuine short rate limit (keep the clamped retry loop) from a quota
//     block (fail fast with the wall-clock reset).
//   - A small persisted quota store (<cache-dir>/quota.json) so every new
//     process fails fast against a known block instead of re-burning the
//     retry budget.
//   - AdaptiveLimiter ceiling persistence, so the discovered safe request
//     rate survives process restarts instead of being re-learned via 429s.

package cliutil

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// QuotaBlockThreshold separates genuine short rate limits from quota blocks.
// A Retry-After above this is treated as an endpoint-class quota block:
// retrying is guaranteed-futile noise, so the client records the block and
// fails fast until the reset time passes.
const QuotaBlockThreshold = 5 * time.Minute

// RetryAfterUncapped parses an HTTP Retry-After header exactly like
// RetryAfter (delta-seconds, HTTP-date, Unix epoch s/ms) but WITHOUT the
// MaxRetryWait clamp. Returns 0 when the header is missing or unparseable so
// callers fall back to the clamped default path.
func RetryAfterUncapped(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}
	header := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if header == "" {
		return 0
	}
	if value, err := strconv.ParseInt(header, 10, 64); err == nil {
		if value <= 0 {
			return 0
		}
		if wait := retryAfterEpochWait(value); wait > 0 {
			return wait
		}
		return time.Duration(value) * time.Second
	}
	if t, err := http.ParseTime(header); err == nil {
		if wait := time.Until(t); wait > 0 {
			return wait
		}
	}
	return 0
}

// QuotaBlockError signals that an endpoint class is under a persisted quota
// block. Distinct from RateLimitError: no retries were attempted because the
// block is hours-scale, not seconds-scale.
type QuotaBlockError struct {
	URL          string
	Class        string
	BlockedUntil time.Time
}

func (e *QuotaBlockError) Error() string {
	remaining := time.Until(e.BlockedUntil).Round(time.Second)
	msg := fmt.Sprintf("quota exhausted (HTTP 429) for /%s endpoints", e.Class)
	if e.URL != "" {
		msg += fmt.Sprintf(" (%s)", e.URL)
	}
	msg += fmt.Sprintf(": blocked until %s (%s from now)",
		e.BlockedUntil.Local().Format("2006-01-02 15:04 MST"), remaining)
	msg += "; not retrying — the block is quota-scale, not rate-limit-scale." +
		" Other endpoint classes may still work; run 'spotify-pp-cli auth status' to see active blocks."
	return msg
}

// QuotaBlock is one persisted endpoint-class block.
type QuotaBlock struct {
	Class        string    `json:"class"`
	BlockedUntil time.Time `json:"blocked_until"`
	RecordedAt   time.Time `json:"recorded_at"`
}

type quotaState struct {
	Blocks         map[string]QuotaBlock `json:"blocks,omitempty"`
	LimiterCeiling float64               `json:"limiter_ceiling,omitempty"`
}

var (
	quotaMu       sync.Mutex
	quotaIdentity string
)

// SetQuotaIdentity scopes persisted quota state to the credential the quota
// is actually enforced against. Spotify quota blocks are per-APP (client ID),
// not per user account — observed directly: a block followed the app while
// /me on the same account kept returning 200. So two accounts sharing one
// client ID correctly share a block, but switching to a different developer
// app must not inherit the old app's block. The identity is a short hash of
// the client ID; empty falls back to the unscoped legacy filename.
func SetQuotaIdentity(clientID string) {
	quotaMu.Lock()
	defer quotaMu.Unlock()
	if clientID == "" {
		quotaIdentity = ""
		return
	}
	h := sha256.Sum256([]byte(clientID))
	quotaIdentity = hex.EncodeToString(h[:8])
}

// quotaStatePathLocked returns the identity-scoped state path. Caller must
// hold quotaMu (quotaIdentity is guarded by it).
func quotaStatePathLocked() (string, error) {
	dir, err := CacheDir()
	if err != nil {
		return "", err
	}
	name := "quota.json"
	if quotaIdentity != "" {
		name = "quota-" + quotaIdentity + ".json"
	}
	return filepath.Join(dir, name), nil
}

// loadQuotaStateLocked reads quota state fresh from disk on EVERY call —
// deliberately no process-lifetime cache. Concurrent CLI processes share the
// state file, and a cached snapshot would make each process's save erase
// blocks recorded by the others (last-writer-wins on a stale base). With
// fresh read-modify-write under an atomic rename, a block recorded by
// another process is visible here immediately and survives this process's
// next save. The residual race is the microseconds between read and rename
// of two truly simultaneous writers; the worst case is one redundant live
// request that gets a real 429 and re-records the block. flock is
// deliberately omitted: x/sys is only an indirect dependency and go.mod
// tidy policy drops direct usage. Fail-open: missing or corrupt file =
// empty state. Caller must hold quotaMu.
func loadQuotaStateLocked() *quotaState {
	state := &quotaState{Blocks: map[string]QuotaBlock{}}
	if path, err := quotaStatePathLocked(); err == nil {
		if data, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(data, state)
			if state.Blocks == nil {
				state.Blocks = map[string]QuotaBlock{}
			}
		}
	}
	return state
}

func saveQuotaStateLocked(state *quotaState) {
	path, err := quotaStatePathLocked()
	if err != nil {
		return
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return
	}
	_ = AtomicWritePrivateFile(path, data, 0o600, 0o700)
}

// QuotaGuardEnabled reports whether the persisted quota circuit breaker is
// active. Disabled under verify/dogfood environments so mock-server test
// runs can never trip or consult a real on-disk block.
func QuotaGuardEnabled() bool {
	return !IsVerifyEnv() && !IsDogfoodEnv()
}

// QuotaClassForPath maps a request path to its quota endpoint class — the
// first path segment ("search", "artists", "me", ...). Spotify quota blocks
// are endpoint-scoped: /me can return 200 while /search and /artists 429.
func QuotaClassForPath(path string) string {
	p := strings.TrimPrefix(path, "/")
	if i := strings.IndexByte(p, '?'); i >= 0 {
		p = p[:i]
	}
	if i := strings.IndexByte(p, '/'); i >= 0 {
		p = p[:i]
	}
	if p == "" {
		return "root"
	}
	return p
}

// QuotaBlockedUntil reports whether the endpoint class is under an active
// persisted block. Expired entries are pruned lazily.
func QuotaBlockedUntil(class string) (time.Time, bool) {
	quotaMu.Lock()
	defer quotaMu.Unlock()
	state := loadQuotaStateLocked()
	block, ok := state.Blocks[class]
	if !ok {
		return time.Time{}, false
	}
	if time.Now().After(block.BlockedUntil) {
		delete(state.Blocks, class)
		saveQuotaStateLocked(state)
		return time.Time{}, false
	}
	return block.BlockedUntil, true
}

// RecordQuotaBlock persists an endpoint-class quota block.
func RecordQuotaBlock(class string, until time.Time) {
	quotaMu.Lock()
	defer quotaMu.Unlock()
	state := loadQuotaStateLocked()
	state.Blocks[class] = QuotaBlock{
		Class:        class,
		BlockedUntil: until,
		RecordedAt:   time.Now(),
	}
	saveQuotaStateLocked(state)
}

// RecordLimiterCeiling persists the adaptive limiter's learned rate ceiling
// (req/s) so future sessions start below it instead of re-discovering it
// via 429s. Zero/negative ceilings are ignored.
func RecordLimiterCeiling(ceiling float64) {
	if ceiling <= 0 {
		return
	}
	quotaMu.Lock()
	defer quotaMu.Unlock()
	state := loadQuotaStateLocked()
	if state.LimiterCeiling == ceiling {
		return
	}
	state.LimiterCeiling = ceiling
	saveQuotaStateLocked(state)
}

// PersistedLimiterCeiling returns the stored rate ceiling, or 0 when none.
func PersistedLimiterCeiling() float64 {
	quotaMu.Lock()
	defer quotaMu.Unlock()
	return loadQuotaStateLocked().LimiterCeiling
}

// ActiveQuotaBlocks returns the currently-active blocks sorted by class,
// pruning expired entries. Used by `auth status` and `doctor` so the user
// can see endpoint-scoped block state instead of inferring health from a
// working /me call.
func ActiveQuotaBlocks() []QuotaBlock {
	quotaMu.Lock()
	defer quotaMu.Unlock()
	state := loadQuotaStateLocked()
	now := time.Now()
	pruned := false
	var out []QuotaBlock
	for class, block := range state.Blocks {
		if now.After(block.BlockedUntil) {
			delete(state.Blocks, class)
			pruned = true
			continue
		}
		out = append(out, block)
	}
	if pruned {
		saveQuotaStateLocked(state)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Class < out[j].Class })
	return out
}

// Ceiling returns the limiter's discovered rate ceiling (0 when none yet).
// Nil-safe like every other AdaptiveLimiter method.
func (l *AdaptiveLimiter) Ceiling() float64 {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.ceiling
}

// SeedCeiling installs a previously-learned rate ceiling (req/s) into the
// limiter, lowering the current rate under it when needed. No-op for nil
// limiters or non-positive ceilings.
func (l *AdaptiveLimiter) SeedCeiling(ceiling float64) {
	if l == nil || ceiling <= 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ceiling = ceiling
	if capped := ceiling * 0.9; l.rate > capped {
		if capped < l.floor {
			capped = l.floor
		}
		l.rate = capped
	}
}
