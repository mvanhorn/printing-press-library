// Copyright 2026 dstevens. Licensed under Apache-2.0.

package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// PATCH(auto-refresh): central dispatcher for the pre-command refresh.
// Whenever any command runs (outside the skip list and any opt-out),
// PersistentPreRunE calls runAutoRefresh as its first data action so
// downstream reads see current data without the user remembering to
// run `sync` first. Refresh is best-effort: failures print a one-line
// warning and the underlying command proceeds against whatever data
// the local store already has.

// noRefreshCommands lists command leaves and ancestors whose presence
// in the running command's lineage suppresses auto-refresh entirely.
// Walked via cmd.Parent() chain in shouldSkipAutoRefresh.
//
// Each entry is justified:
//
//   sync, sync-api: would recurse (auto-refresh calls them)
//   auth          : refresh requires auth — chicken/egg
//   doctor        : doctor reports current state; auto-refresh would
//                   side-effect that state (write SyncState) before
//                   doctor reads it
//   help, version, completion, agent-context, which:
//                 : no data dependency
//   profile       : profile management is local-only
//   feedback      : feedback is local-only (or remote-but-not-data)
//
// Names match cobra Use:; aliases (e.g. "sync-api") are matched as-is.
var noRefreshCommands = map[string]struct{}{
	"sync":          {},
	"sync-api":      {},
	"auth":          {},
	"doctor":        {},
	"help":          {},
	"version":       {},
	"completion":    {},
	"agent-context": {},
	"profile":       {},
	"feedback":      {},
	"which":         {},
}

// refreshSurface labels which auth path a refreshResult came from so
// the provenance line can distinguish "cache=ok api=skipped" from
// "cache=failed api=ok". Stable strings — used in tests.
const (
	refreshSurfaceCache = "cache"
	refreshSurfaceAPI   = "api"
)

// refreshPlan captures which auth surfaces auto-refresh should hit
// for this invocation. Both, one, or neither may be true; an empty
// plan is a legitimate "no auth configured" state and produces no
// refresh and no provenance line.
type refreshPlan struct {
	cache bool
	api   bool
}

func (p refreshPlan) empty() bool { return !p.cache && !p.api }

// refreshResult is the per-surface outcome auto-refresh produces.
// Errors are captured here rather than returned upward — refresh
// must never block the user's command.
type refreshResult struct {
	surface  string
	ok       bool
	rows     int
	duration time.Duration
	err      error
}

// runAutoRefresh is the entry point the PersistentPreRunE hook calls.
// It is best-effort end-to-end: any panic, error, or unexpected state
// is captured into a refreshResult and the function returns without
// propagating an error. The caller's command always proceeds.
//
// Declared as a variable so integration tests can swap in a spy that
// records call count without invoking the real cache/API sync paths.
// Production reads runAutoRefreshImpl directly.
var runAutoRefresh = runAutoRefreshImpl

func runAutoRefreshImpl(cmd *cobra.Command, flags *rootFlags) {
	if cmd == nil || flags == nil {
		return
	}
	if shouldSkipAutoRefresh(cmd) {
		return
	}
	if autoRefreshOptedOut(flags) {
		return
	}
	plan := detectRefreshPlan(flags)
	if plan.empty() {
		return
	}

	ctx, cancel := autoRefreshContext(cmd.Context(), flags.timeout)
	defer cancel()

	results := plan.run(ctx, flags)
	if shouldEmitProvenance(flags, cmd) {
		emitProvenanceLine(cmd.ErrOrStderr(), results)
	}
}

// autoRefreshContext returns a derived context with a deadline if the
// parent context has none and the user-configured timeout is positive.
// Without this, a hung refresh could pin the entire command — the
// user's command should still be able to run on stale data within a
// reasonable bound.
func autoRefreshContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	if timeout <= 0 {
		return context.WithCancel(parent)
	}
	if _, hasDeadline := parent.Deadline(); hasDeadline {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, timeout)
}

// shouldSkipAutoRefresh returns true when the command's lineage
// includes any name in noRefreshCommands. Walks via cmd.Parent()
// so deep subcommands like "auth login" or "profile save" are
// matched on the ancestor's name.
func shouldSkipAutoRefresh(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if _, skip := noRefreshCommands[c.Name()]; skip {
			return true
		}
	}
	return false
}

// autoRefreshOptedOut applies a two-tier precedence: --no-refresh=true
// (possibly set by a profile via ApplyProfileToFlags during Phase 5.3 of
// root.go's PersistentPreRunE) wins; otherwise GRANOLA_NO_AUTO_REFRESH
// decides. Refresh is on by default.
//
// PATCH(autorefresh-doc-comment-honest): a bool flag carries no
// "was-explicitly-set-to-false" signal at this call site, so the env var
// runs whenever flags.noRefresh is false regardless of whether the user
// or a profile said `--no-refresh=false` or simply left the flag at its
// default. That means a profile setting `no_refresh: false` cannot
// override a truthy GRANOLA_NO_AUTO_REFRESH — env beats the explicit
// "leave it on" profile value. Captured here so the doc matches the code.
func autoRefreshOptedOut(flags *rootFlags) bool {
	if flags.noRefresh {
		return true
	}
	if envBoolish(os.Getenv("GRANOLA_NO_AUTO_REFRESH")) {
		return true
	}
	return false
}

// envBoolish matches the convention the rest of the CLI uses for
// boolean env vars (see how the existing feedback-auto-send env is
// parsed): "1", "true", "yes" (case-insensitive) are truthy; anything
// else is falsy.
func envBoolish(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes":
		return true
	}
	return false
}

// detectRefreshPlan inspects auth surfaces in the user's environment
// and config and decides which sync paths auto-refresh should run.
// Both are independent: a user with desktop cache + API key gets both;
// a user with only one gets one; a user with neither gets nothing.
func detectRefreshPlan(flags *rootFlags) refreshPlan {
	return refreshPlan{
		cache: encryptedCachePresent(),
		api:   apiKeyConfigured(flags),
	}
}

// encryptedCachePresent returns true when Granola's encrypted cache
// file exists at the expected path. We do not attempt to decrypt — that
// would require a Keychain prompt every time the hook fires. A present
// file is the strongest signal we can read cheaply that auto-refresh
// has something to do; if decrypt later fails, runCacheSync records
// that on the SyncState the same way doctor does, and the provenance
// line shows "cache=failed".
//
// Reuses the support-dir resolver from doctor_encrypted_store.go to
// honor GRANOLA_SUPPORT_DIR overrides.
func encryptedCachePresent() bool {
	supportDir := granolaSupportDirFromEnv()
	if _, err := os.Stat(supportDir); err != nil {
		return false
	}
	encPath := filepath.Join(supportDir, "cache-v6.json.enc")
	if fileExists(encPath) {
		return true
	}
	// Pre-encryption installs ship cache-v6.json (plaintext). Treat
	// those as refreshable too — runCacheSync's openGranolaCache
	// handles both shapes.
	plainPath := filepath.Join(supportDir, "cache-v6.json")
	return fileExists(plainPath)
}

// run executes the refresh plan, calling whichever sync cores the
// plan selected. Cache sync runs first because it is fast and local
// (file decrypt + SQLite upsert); api sync is slower (HTTP) so an
// already-stale-but-warm cache is available even if api sync hangs.
// Each result captures errors locally so the caller never sees an
// error return — best-effort is enforced at the type level.
func (p refreshPlan) run(ctx context.Context, flags *rootFlags) []refreshResult {
	var out []refreshResult
	if p.cache {
		res, err := runCacheSync(ctx)
		// Cache refresh is "ok" when the decrypt + SQLite upsert
		// succeeded, even if document-API hydration was unable to
		// reach /v2/get-documents (HydrateErr) or the sync_state
		// marker write failed (StateWriteErr). Both are non-fatal:
		// the caller's command still gets fresh cached transcripts,
		// folders, panels, recipes, and chats. Provenance line will
		// show "cache=ok" with the rows count; doctor surfaces the
		// hydrate/state-write detail.
		out = append(out, refreshResult{
			surface:  refreshSurfaceCache,
			ok:       err == nil,
			rows:     res.TotalRows(),
			duration: res.Duration,
			err:      err,
		})
	}
	if p.api {
		res, err := runApiSync(ctx, flags)
		out = append(out, refreshResult{
			surface:  refreshSurfaceAPI,
			ok:       err == nil,
			rows:     res.TotalRows(),
			duration: res.Duration,
			err:      err,
		})
	}
	return out
}

// shouldEmitProvenance decides whether the one-line stderr summary
// fires. Suppressed under any mode that suggests the caller is a
// machine (--agent, --json, --compact, --quiet) or when stderr is
// not a TTY (CI logs, piped consumers). The TTY check is the most
// important — agents often forget to pass --agent.
func shouldEmitProvenance(flags *rootFlags, _ *cobra.Command) bool {
	if flags.agent || flags.asJSON || flags.compact || flags.quiet {
		return false
	}
	return stderrIsTerminal()
}

// stderrIsTerminal is a tiny seam so tests can probe shouldEmitProvenance
// without mucking with os.Stderr. Production checks the character-device
// bit via os.Stderr.Stat() — pipes and files do not have ModeCharDevice
// set, terminals do. Stdlib-only; avoids pulling in golang.org/x/term.
var stderrIsTerminal = func() bool {
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// emitProvenanceLine writes the one-line summary to the provided
// writer. Format:
//
//   auto-refresh: cache=ok (1.2s, 47 docs)  api=ok (820ms, 12 docs)
//
// Failures render as cache=failed: <short reason>. Surfaces not in
// the plan are omitted entirely (no "api=skipped" noise for users
// without an API key).
func emitProvenanceLine(w io.Writer, results []refreshResult) {
	if len(results) == 0 {
		return
	}
	parts := make([]string, 0, len(results))
	for _, r := range results {
		parts = append(parts, formatRefreshFragment(r))
	}
	fmt.Fprintln(w, "auto-refresh: "+strings.Join(parts, "  "))
}

// formatRefreshFragment renders a single surface's outcome.
// Externalized so the provenance line composition is trivial to test
// per-surface.
func formatRefreshFragment(r refreshResult) string {
	dur := formatRefreshDuration(r.duration)
	if r.ok {
		return fmt.Sprintf("%s=ok (%s, %d rows)", r.surface, dur, r.rows)
	}
	if r.err == nil {
		// ok=false with no err is a programming bug; show something
		// truthful rather than a misleading "ok".
		return fmt.Sprintf("%s=failed (%s)", r.surface, dur)
	}
	return fmt.Sprintf("%s=failed: %s (%s)", r.surface, shortErr(r.err), dur)
}

// formatRefreshDuration renders durations as humans read them:
// sub-second → "230ms", sub-minute → "1.2s". Avoid Go's default
// "1.234567s" which puts microsecond precision on a refresh line
// nobody will read at that resolution.
func formatRefreshDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// shortErr trims an error message to a single short line for
// inclusion in the provenance string. Keeps long upstream wrapped
// errors from blowing up the line width.
func shortErr(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if idx := strings.IndexAny(msg, "\n\r"); idx >= 0 {
		msg = msg[:idx]
	}
	if len(msg) > 80 {
		msg = msg[:77] + "..."
	}
	return msg
}
