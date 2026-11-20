package client

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
)

// EtsyQuota is the most recent daily budget advertised by Etsy.
type EtsyQuota struct {
	LimitPerDay    int       `json:"limit_per_day"`
	RemainingToday int       `json:"remaining_today"`
	LimitPerSecond int       `json:"limit_per_second"`
	ObservedAt     time.Time `json:"observed_at"`
	RawRemaining   int       `json:"raw_remaining"`
	RawObservedAt  time.Time `json:"raw_observed_at"`
}

var etsyQuotaState struct {
	sync.Mutex
	quotas        map[string]EtsyQuota
	persistErrors map[string]error
	directory     string
}

func quotaHeader(h http.Header, key string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(h.Get(key)))
	return n, err == nil && n >= 0
}

// etsy-ext: cache identity survives unavailable state directories.
var etsyQuotaNow = time.Now
var etsyQuotaMergeHook func()

func quotaIdentity(key string) string {
	key, _, _ = strings.Cut(strings.TrimSpace(key), ":")
	if key == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(key))
	return fmt.Sprintf("etsy-quota-%x.json", sum[:6])
}

// quotaPath contains only a hash of the API keystring.
func quotaPath(key string) string {
	id := quotaIdentity(key)
	if id == "" {
		return ""
	}
	dir, err := cliutil.StateDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, id)
}

// EtsyQuotaPersistError exposes best-effort persistence failures to doctor only.
func EtsyQuotaPersistError(key string) error {
	etsyQuotaState.Lock()
	defer etsyQuotaState.Unlock()
	return etsyQuotaState.persistErrors[quotaIdentity(key)]
}
func quotaPersistError(id string, err error) {
	etsyQuotaState.Lock()
	defer etsyQuotaState.Unlock()
	if etsyQuotaState.persistErrors == nil {
		etsyQuotaState.persistErrors = make(map[string]error)
	}
	etsyQuotaState.persistErrors[id] = err
}

// etsy-ext: a deliberate state-directory switch starts a separate cache namespace.
// An unavailable directory retains the in-process observations.
func quotaCacheDirectory(path string) {
	if path == "" {
		return
	}
	dir := filepath.Dir(path)
	etsyQuotaState.Lock()
	defer etsyQuotaState.Unlock()
	if etsyQuotaState.directory != dir {
		etsyQuotaState.directory = dir
		etsyQuotaState.quotas = nil
		etsyQuotaState.persistErrors = nil
	}
}

// ObserveEtsyQuota records quota headers best-effort, without affecting requests.
func ObserveEtsyQuota(h http.Header, key string) {
	observeEtsyQuota(context.Background(), h, key, etsyQuotaNow().UTC())
}

// observeEtsyQuota receives the response timestamp before any lock acquisition.
func observeEtsyQuota(ctx context.Context, h http.Header, key string, observedAt time.Time) {
	remaining, ok := quotaHeader(h, "x-remaining-today")
	path := quotaPath(key)
	quotaCacheDirectory(path)
	if !ok || quotaIdentity(key) == "" {
		return
	}
	q := EtsyQuota{RemainingToday: remaining, ObservedAt: observedAt}
	q.LimitPerDay, _ = quotaHeader(h, "x-limit-per-day")
	q.LimitPerSecond, _ = quotaHeader(h, "x-limit-per-second")
	observeEtsyQuotaRecord(ctx, path, q, quotaIdentity(key))
}

// etsy-ext: rolling validity and raw observations prevent merged minima ratcheting.
func quotaUsable(q EtsyQuota) bool {
	age := etsyQuotaNow().Sub(q.ObservedAt)
	return !q.ObservedAt.IsZero() && age < 24*time.Hour && age >= -time.Minute
}
func mergeEtsyQuota(a, b EtsyQuota) EtsyQuota {
	if !quotaUsable(a) {
		a = EtsyQuota{}
	}
	if !quotaUsable(b) {
		b = EtsyQuota{}
	}
	if a.ObservedAt.IsZero() {
		return b
	}
	if b.ObservedAt.IsZero() {
		return a
	}
	if a.ObservedAt.After(b.ObservedAt) {
		a, b = b, a
	}
	if a.RawObservedAt.IsZero() {
		a.RawObservedAt, a.RawRemaining = a.ObservedAt, a.RemainingToday
	}
	if b.RawObservedAt.IsZero() {
		b.RawObservedAt, b.RawRemaining = b.ObservedAt, b.RemainingToday
	}
	if b.LimitPerDay == 0 {
		b.LimitPerDay = a.LimitPerDay
	}
	if b.LimitPerSecond == 0 {
		b.LimitPerSecond = a.LimitPerSecond
	}
	if b.RawObservedAt.Sub(a.RawObservedAt) <= 5*time.Second {
		if b.RawObservedAt.Equal(a.RawObservedAt) && b.RawRemaining == a.RawRemaining {
			b.RemainingToday = min(a.RemainingToday, b.RemainingToday)
		} else {
			b.RemainingToday = min(a.RawRemaining, b.RawRemaining)
		}
	}
	return b
}

func observeEtsyQuotaRecord(ctx context.Context, path string, q EtsyQuota, id string) {
	if !quotaUsable(q) {
		return
	}
	q.RawObservedAt, q.RawRemaining = q.ObservedAt, q.RemainingToday
	fallback := func() {
		etsyQuotaState.Lock()
		defer etsyQuotaState.Unlock()
		old, _ := loadEtsyQuotaLocked(path, id)
		cacheEtsyQuotaLocked(id, mergeEtsyQuota(old, q))
	}
	if path == "" {
		quotaPersistError(id, fmt.Errorf("state directory unavailable"))
		fallback()
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		quotaPersistError(id, err)
		fallback()
		return
	}
	// Never hold the cache mutex while waiting for a file lock.
	acquired, err := withEtsyQuotaLock(ctx, path, func() error {
		etsyQuotaState.Lock()
		defer etsyQuotaState.Unlock()
		old, _ := loadEtsyQuotaLocked(path, id)
		if etsyQuotaMergeHook != nil {
			etsyQuotaMergeHook()
		}
		q = mergeEtsyQuota(old, q)
		cacheEtsyQuotaLocked(id, q)
		data, err := json.Marshal(q)
		if err != nil {
			return err
		}
		return cliutil.AtomicWritePrivateFile(path, data, 0600, 0700)
	})
	quotaPersistError(id, err)
	if !acquired {
		fallback()
	}
}

// LoadEtsyQuota accepts observations within a rolling 24h window, including across UTC midnight.
// Reads are lock-free across processes because writers rename atomically.
func LoadEtsyQuota(key string) (EtsyQuota, bool) {
	id := quotaIdentity(key)
	if id == "" {
		return EtsyQuota{}, false
	}
	path := quotaPath(key)
	quotaCacheDirectory(path)
	if path == "" {
		quotaPersistError(id, fmt.Errorf("state directory unavailable"))
	}
	etsyQuotaState.Lock()
	defer etsyQuotaState.Unlock()
	return loadEtsyQuotaLocked(path, id)
}
func loadEtsyQuotaLocked(path, id string) (EtsyQuota, bool) {
	var q EtsyQuota
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &q)
	}
	q = mergeEtsyQuota(q, etsyQuotaState.quotas[id])
	if !quotaUsable(q) {
		return EtsyQuota{}, false
	}
	return q, true
}

// DailyQuotaExhaustedError identifies Etsy's daily quota exhaustion.
type DailyQuotaExhaustedError struct{ Cause error }

func (e *DailyQuotaExhaustedError) Error() string {
	return "Etsy daily request quota exhausted (x-remaining-today: 0); retry when Etsy reports available quota"
}

func (e *DailyQuotaExhaustedError) Unwrap() error { return e.Cause }

// IsDailyQuotaExhausted reports whether the error chain contains daily exhaustion.
func IsDailyQuotaExhausted(err error) bool {
	var exhausted *DailyQuotaExhaustedError
	return errors.As(err, &exhausted)
}

func cacheEtsyQuotaLocked(path string, q EtsyQuota) {
	if etsyQuotaState.quotas == nil {
		etsyQuotaState.quotas = make(map[string]EtsyQuota)
	}
	etsyQuotaState.quotas[path] = q
}
