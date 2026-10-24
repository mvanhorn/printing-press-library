// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package appstore

// The governor's memory across invocations. Each CLI run is a new process,
// so a limiter or a tripped governor that lives only in memory forgets a 429
// the moment the command exits, and the next invocation sends at once. On
// 2026-10-23 apps.apple.com throttled 2 of 6 uncached page fetches with gaps
// of 10 s and 76 s between invocations. The state file below keeps, for
// every process that shares it (one per --home / state directory):
//   - the send slot each rate class reserved last, so a new process paces
//     from the previous process's last request, not from zero;
//   - the latest send slot of any class, for the combined --rate-limit cap;
//   - a per-host hold after a 403/429: nothing more is sent to that host
//     until max(Retry-After, ThrottleHold) has passed.
// Reservations happen under a file lock, so concurrent processes (an MCP
// host running several tools at once) take distinct slots.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil"
)

const (
	// ThrottleHold is the shortest time the governor keeps a host closed after
	// Apple answered 403 or 429. Retry-After (13-19 s observed) is far shorter
	// than the edge's memory: on 2026-10-23 a page request 2 minutes after a
	// 429 passed, and one 76 s after two more page requests was throttled
	// again.
	ThrottleHold = 5 * time.Minute
	// maxHoldAhead bounds a stored hold: a hold or send slot further ahead
	// than this comes from a skewed clock or a damaged file and is ignored,
	// so it cannot stall every later run.
	maxHoldAhead = time.Hour
	// governorStateVersion is the state file format.
	governorStateVersion = 1
)

// waitNoticeAfter is the shortest pacing wait announced through
// Options.Notice (a variable so tests need not wait 3 s).
var waitNoticeAfter = 3 * time.Second

// GovernorStateFile is the state file name inside the CLI's state directory.
const GovernorStateFile = "appstore-governor.json"

// HostHold is a throttle the governor remembers: Apple answered Status for
// URL at At, and no request goes to Host until Until.
type HostHold struct {
	Host       string    `json:"host"`
	Until      time.Time `json:"until"`
	At         time.Time `json:"at"`
	Status     int       `json:"status"`
	URL        string    `json:"url"`
	RetryAfter int64     `json:"retryAfterSeconds,omitempty"`
}

// governorState is the shared pacing and throttle memory.
type governorState struct {
	Version  int                     `json:"version"`
	LastSend map[RateClass]time.Time `json:"lastSend,omitempty"`
	LastAny  time.Time               `json:"lastAny,omitempty"`
	Holds    map[string]HostHold     `json:"holds,omitempty"`
}

func (s *governorState) init() {
	if s.LastSend == nil {
		s.LastSend = map[RateClass]time.Time{}
	}
	if s.Holds == nil {
		s.Holds = map[string]HostHold{}
	}
	s.Version = governorStateVersion
}

// activeHold returns the hold on host that is still in force at now.
func (s *governorState) activeHold(host string, now time.Time) (HostHold, bool) {
	h, ok := s.Holds[host]
	if !ok || !now.Before(h.Until) || h.Until.Sub(now) > maxHoldAhead {
		return HostHold{}, false
	}
	return h, true
}

// prune drops holds and send slots older than a day, and ones from the far
// future (a skewed clock), so the file stays small and sane.
func (s *governorState) prune(now time.Time) {
	for host, h := range s.Holds {
		if now.Sub(h.Until) > 24*time.Hour || h.Until.Sub(now) > maxHoldAhead {
			delete(s.Holds, host)
		}
	}
	for class, at := range s.LastSend {
		if now.Sub(at) > 24*time.Hour || at.Sub(now) > maxHoldAhead {
			delete(s.LastSend, class)
		}
	}
	if now.Sub(s.LastAny) > 24*time.Hour || s.LastAny.Sub(now) > maxHoldAhead {
		s.LastAny = time.Time{}
	}
}

// governor serializes reservations in this process and, when path is set,
// across processes through a locked JSON file. A state file that cannot be
// read is treated as empty; one that cannot be locked or written turns the
// governor into an in-memory one for the rest of the process (a broken state
// directory must not break the command).
type governor struct {
	mu   sync.Mutex
	path string
	mem  governorState
	// fileErr is the first lock or write failure (the governor then works
	// in memory only).
	fileErr error
}

func newGovernor(path string) *governor {
	g := &governor{path: strings.TrimSpace(path)}
	g.mem.init()
	return g
}

// stateErr is the state-file failure that turned the governor memory-only
// (holds and send slots then cover this process only), if any.
func (g *governor) stateErr() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.fileErr
}

// HoldRetryAfter reads a 403/429 answer's Retry-After (delta-seconds or an
// HTTP-date) for a throttle hold. Unlike cliutil.RetryAfter, which caps a
// retry sleep at a minute, it keeps Apple's full value, bounded by
// maxHoldAhead (a hold further ahead would be pruned as a clock error). It is
// 0 when the header is absent, unparsable or not in the future.
func HoldRetryAfter(h http.Header, now time.Time) time.Duration {
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return 0
	}
	var d time.Duration
	if secs, err := strconv.ParseInt(v, 10, 64); err == nil {
		if secs >= int64(maxHoldAhead/time.Second) {
			return maxHoldAhead
		}
		d = time.Duration(secs) * time.Second
	} else if t, err := http.ParseTime(v); err == nil {
		d = t.Sub(now)
	}
	if d <= 0 {
		return 0
	}
	return min(d, maxHoldAhead)
}

// update runs fn on the current state; fn reports whether it changed it.
func (g *governor) update(fn func(st *governorState) bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.path == "" || g.fileErr != nil {
		fn(&g.mem)
		return
	}
	if err := os.MkdirAll(filepath.Dir(g.path), 0o700); err != nil {
		g.fileErr = err
		fn(&g.mem)
		return
	}
	var ran bool
	err := cliutil.WithFileLock(g.path, func() error {
		st := readGovernorState(g.path)
		ran = true
		if !fn(&st) {
			g.mem = st
			return nil
		}
		st.prune(time.Now())
		g.mem = st
		data, err := json.MarshalIndent(st, "", "  ")
		if err != nil {
			return err
		}
		return cliutil.AtomicWritePrivateFile(g.path, append(data, '\n'), 0o600, 0o700)
	})
	if err != nil {
		g.fileErr = err
		if !ran {
			fn(&g.mem)
		}
	}
}

// readGovernorState reads the state file; a missing or damaged file is an
// empty state.
func readGovernorState(path string) governorState {
	var st governorState
	data, err := os.ReadFile(path) // #nosec G304 -- the CLI's own state file.
	if err == nil && json.Unmarshal(data, &st) != nil {
		st = governorState{}
	}
	if st.Version != 0 && st.Version != governorStateVersion {
		st = governorState{}
	}
	st.init()
	return st
}

// PacingError reports that the next send slot for a host lies past the
// request's deadline (the root --timeout) or past Options.WaitUntil; nothing
// was sent. It unwraps to context.DeadlineExceeded.
type PacingError struct {
	Host     string
	Class    RateClass
	Wait     time.Duration
	Interval time.Duration
}

func (e *PacingError) Error() string {
	return fmt.Sprintf("the next %s request may go only in %s (%s requests are paced %s apart, across runs), later than this run may wait; nothing was sent",
		e.Host, roundUpSecond(e.Wait), classLabel(e.Class), roundUpSecond(e.Interval))
}

func (e *PacingError) Unwrap() error { return context.DeadlineExceeded }

// classLabel names a rate class for messages.
func classLabel(class RateClass) string {
	switch class {
	case RatePages:
		return "apps.apple.com page"
	case RateStoreAPI:
		return "apps.apple.com/api"
	case RateSearchAPI:
		return "iTunes Search API"
	case RateITunes:
		return "itunes.apple.com"
	case RateHints:
		return "search-hints"
	}
	return string(class)
}

func roundUpSecond(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	return ((d + time.Second - 1) / time.Second) * time.Second
}

// holdError is the error for a request the governor refuses because Apple
// throttled its host earlier (in this run or an earlier one).
func holdError(h HostHold, target string, now time.Time) error {
	remaining := roundUpSecond(h.Until.Sub(now))
	body := fmt.Sprintf("Apple answered HTTP %d for %s at %s", h.Status, h.URL, FormatTime(h.At))
	if h.RetryAfter > 0 {
		body += fmt.Sprintf(" (Retry-After %ds)", h.RetryAfter)
	}
	body += fmt.Sprintf("; the governor holds every request to %s until %s, so nothing was sent", h.Host, FormatTime(h.Until))
	return &cliutil.RateLimitError{URL: target, RetryAfter: remaining, Body: body}
}

// StatePath is the governor's state file ("" when the client keeps its
// pacing and throttle memory in this process only).
func (c *Client) StatePath() string { return c.gov.path }

// HoldFor reports the throttle hold in force for host, if any (tests,
// diagnostics).
func (c *Client) HoldFor(host string) (HostHold, bool) {
	var h HostHold
	var ok bool
	c.gov.update(func(st *governorState) bool {
		h, ok = st.activeHold(strings.ToLower(host), time.Now())
		return false
	})
	return h, ok
}

// recordHold remembers a 403/429 from host: nothing more goes there until
// max(retryAfter, ThrottleHold) from now. It returns the hold.
func (c *Client) recordHold(host string, status int, rawURL string, retryAfter time.Duration) HostHold {
	now := time.Now()
	hold := ThrottleHold
	if retryAfter > hold {
		hold = retryAfter
	}
	h := HostHold{
		Host:       strings.ToLower(host),
		Until:      now.Add(hold),
		At:         now,
		Status:     status,
		URL:        rawURL,
		RetryAfter: int64(retryAfter / time.Second),
	}
	c.gov.update(func(st *governorState) bool {
		if prev, ok := st.Holds[h.Host]; ok && prev.Until.After(h.Until) && prev.Until.Sub(now) <= maxHoldAhead {
			h.Until = prev.Until
		}
		st.Holds[h.Host] = h
		return true
	})
	return h
}

// Govern applies this client's governor to a request another HTTP client is
// about to send to an App Store host (the generated internal/client, which
// the MCP server's typed tools use): it refuses the request while this run is
// stopped or a throttle hold for the host is in force (*cliutil.RateLimitError),
// fails fast with *PacingError when the next send slot lies past ctx's
// deadline, and otherwise waits for that slot, shared with every App Store
// client using the same state file. A URL outside Apple's App Store hosts
// passes at once.
func (c *Client) Govern(ctx context.Context, u *url.URL) error {
	class, err := ClassifyURL(u)
	if err != nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := c.governorErr(); err != nil {
		return err
	}
	if err := c.pace(ctx, u, class); err != nil {
		return err
	}
	c.countRequest(class)
	return nil
}

// RecordThrottle remembers a 403/429 that another HTTP client received from
// an App Store host (see Govern): the host is held for max(retryAfter,
// ThrottleHold) across processes, and this client stops. It returns the hold.
func (c *Client) RecordThrottle(u *url.URL, status int, retryAfter time.Duration) HostHold {
	hold := c.recordHold(u.Hostname(), status, u.String(), retryAfter)
	c.trip(&cliutil.RateLimitError{
		URL:        u.String(),
		RetryAfter: roundUpSecond(time.Until(hold.Until)),
		Body:       c.throttleText(status, retryAfter, hold, nil),
	})
	return hold
}
