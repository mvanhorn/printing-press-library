// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

package granola

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// armCLISession puts a CLI-owned session in place and points the token cache at
// it, so RefreshAccessToken takes the CLI-owned branch.
func armCLISession(t *testing.T, expiresIn time.Duration) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "granola-pp-cli")
	t.Setenv("GRANOLA_CLI_SESSION_PATH", filepath.Join(dir, "session.json"))
	t.Setenv("GRANOLA_SUPPORT_DIR", t.TempDir())
	t.Setenv("GRANOLA_WORKOS_TOKEN", "")
	t.Setenv("GRANOLA_WORKOS_REFRESH", "")
	if err := SaveCLISession(CLISession{
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		ExpiresAt:    time.Now().Add(expiresIn).UTC(),
		ObtainedAt:   time.Now().UTC(),
		AccountEmail: "someone@example.com",
	}); err != nil {
		t.Fatalf("SaveCLISession: %v", err)
	}
	ResetTokenCache()
	t.Cleanup(ResetTokenCache)
	if _, err := LoadRefreshToken(); err != nil {
		t.Fatalf("LoadRefreshToken: %v", err)
	}
	if src := CurrentTokenSource(); src != TokenSourceCLISession {
		t.Fatalf("want TokenSourceCLISession, got %v", src)
	}
}

// refreshServerSeeingMarker answers refreshes and records, per call, whether the
// in-flight rotation marker was on disk at the moment the exchange happened.
func refreshServerSeeingMarker(t *testing.T, calls *atomic.Int32, markerSeen *atomic.Bool, hold time.Duration) {
	t.Helper()
	orig := refreshClient
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if _, err := os.Stat(rotationMarkerPath()); err == nil {
			markerSeen.Store(true)
		}
		if hold > 0 {
			time.Sleep(hold)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600}`))
	}))
	t.Cleanup(srv.Close)
	SetRefreshHTTPClient(&http.Client{Transport: &rewriteTransport{target: srv.URL}})
	t.Cleanup(func() { SetRefreshHTTPClient(orig) })
}

// refreshServerRefusing arms a server that must never be reached. It keeps
// these tests offline and turns "the code exchanged when it should not have"
// into a clear failure rather than a call to the real endpoint.
func refreshServerRefusing(t *testing.T, calls *atomic.Int32, markerSeen *atomic.Bool) {
	t.Helper()
	orig := refreshClient
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte(`{"error":"this exchange should not have happened"}`))
	}))
	t.Cleanup(srv.Close)
	SetRefreshHTTPClient(&http.Client{Transport: &rewriteTransport{target: srv.URL}})
	t.Cleanup(func() { SetRefreshHTTPClient(orig) })
}

// The marker exists to catch a crash between a spent remote token and a durable
// local write. Written after the exchange it would never witness that window,
// so this asserts the server sees it already on disk.
func TestRefresh_MarksRotationInFlightBeforeExchange(t *testing.T) {
	armCLISession(t, time.Hour)
	var calls atomic.Int32
	var markerSeen atomic.Bool
	refreshServerSeeingMarker(t, &calls, &markerSeen, 0)

	if _, err := RefreshAccessToken("old-refresh"); err != nil {
		t.Fatalf("RefreshAccessToken: %v", err)
	}
	if !markerSeen.Load() {
		t.Error("rotation marker was not on disk during the exchange; an interrupted rotation would be undetectable")
	}
	// A completed rotation must not leave the marker behind, or the next run
	// sends the user to auth login over a refresh that worked.
	if _, err := LoadCLISession(); err != nil {
		t.Errorf("session unusable after a successful rotation: %v", err)
	}
}

// Single-use rotation makes an unserialized refresh unsafe: the loser's
// exchange spends a token the winner already replaced.
func TestRefresh_ConcurrentCallersExchangeOnce(t *testing.T) {
	armCLISession(t, time.Hour)
	var calls atomic.Int32
	var markerSeen atomic.Bool
	refreshServerSeeingMarker(t, &calls, &markerSeen, 40*time.Millisecond)

	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = RefreshAccessToken("old-refresh")
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("caller %d: %v", i, err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("exchanges = %d, want 1; concurrent refreshes spend a single-use chain", got)
	}
}

// A failure that never reached the server left the stored token untouched, so
// reporting an interrupted rotation would send the user to auth login over a
// transport blip.
func TestRefresh_FailedExchangeClearsMarker(t *testing.T) {
	armCLISession(t, time.Hour)
	orig := refreshClient
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"boom"}`))
	}))
	defer srv.Close()
	SetRefreshHTTPClient(&http.Client{Transport: &rewriteTransport{target: srv.URL}})
	defer SetRefreshHTTPClient(orig)

	if _, err := RefreshAccessToken("old-refresh"); err == nil {
		t.Fatal("expected the refresh to fail")
	}
	if _, err := LoadCLISession(); err != nil {
		t.Errorf("a failed exchange must leave the session usable, got %v", err)
	}
	if _, err := os.Stat(rotationMarkerPath()); err == nil {
		t.Error("marker survived a failure that never spent the token")
	}
}

// The one case the marker must survive: the remote exchange succeeded, so the
// presented token may be dead upstream, and no replacement reached disk.
func TestRefresh_UnpersistedRotationStaysDetectable(t *testing.T) {
	armCLISession(t, time.Hour)
	var calls atomic.Int32
	var markerSeen atomic.Bool
	refreshServerSeeingMarker(t, &calls, &markerSeen, 0)

	// Make the durable write fail after the exchange has already happened.
	if err := os.Remove(CLISessionPath()); err != nil {
		t.Fatalf("remove session: %v", err)
	}
	if err := os.MkdirAll(CLISessionPath(), 0o700); err != nil {
		t.Fatalf("occupy session path with a directory: %v", err)
	}

	_, err := RefreshAccessToken("old-refresh")
	if err == nil {
		t.Fatal("a refresh whose persist failed must not report success")
	}
	if _, statErr := os.Stat(rotationMarkerPath()); statErr != nil {
		t.Fatal("marker was cleared after a spent exchange; the unrecoverable window is now invisible")
	}
	// And that state must surface as the actionable error, not as a usable
	// session, once the marker outlives any live rotation.
	os.RemoveAll(CLISessionPath())
	ageRotationMarker(t)
	if _, loadErr := LoadCLISession(); !errors.Is(loadErr, ErrSessionInterrupted) {
		t.Errorf("want ErrSessionInterrupted after an unpersisted rotation, got %v", loadErr)
	}
}

// A caller that defers to another process's rotation must also adopt its
// tokens. Returning them while leaving the local cache holding the rejected
// access token means the 401 retry that triggered the refresh replays the same
// dead credential.
func TestRefresh_WaiterAdoptsRotatedTokensIntoCache(t *testing.T) {
	armCLISession(t, time.Hour)

	// Simulate the winner having already published a rotated session.
	if err := SaveCLISession(CLISession{
		AccessToken:  "winner-access",
		RefreshToken: "winner-refresh",
		ExpiresAt:    time.Now().Add(time.Hour).UTC(),
		ObtainedAt:   time.Now().UTC(),
		AccountEmail: "someone@example.com",
	}); err != nil {
		t.Fatalf("SaveCLISession: %v", err)
	}

	var calls atomic.Int32
	var seenMarker atomic.Bool
	refreshServerRefusing(t, &calls, &seenMarker)

	got, err := RefreshAccessToken("old-refresh")
	if err != nil {
		t.Fatalf("RefreshAccessToken: %v", err)
	}
	if got.AccessToken != "winner-access" {
		t.Fatalf("returned token = %q, want the winner's", got.AccessToken)
	}
	// The cache the next API call reads must hold the winner's token, not the
	// rejected one this caller started with.
	cached, _, err := LoadAccessToken()
	if err != nil {
		t.Fatalf("LoadAccessToken: %v", err)
	}
	if cached != "winner-access" {
		t.Errorf("in-process cache = %q, want winner-access; the retry would replay a rejected token", cached)
	}
}

// Granola's refresh proxy is observed to return the SAME refresh token, so
// detection keyed on that token changing would never fire for the provider this
// CLI actually talks to: the waiter would time out while a valid new access
// token sat on disk.
func TestRefresh_DetectsRotationWhenRefreshTokenIsPreserved(t *testing.T) {
	armCLISession(t, time.Hour)

	// Winner publishes a new access token while preserving the refresh token,
	// exactly as Granola's proxy does.
	if err := SaveCLISession(CLISession{
		AccessToken:  "winner-access",
		RefreshToken: "old-refresh", // unchanged on purpose
		ExpiresAt:    time.Now().Add(time.Hour).UTC(),
		ObtainedAt:   time.Now().Add(time.Second).UTC(),
		AccountEmail: "someone@example.com",
	}); err != nil {
		t.Fatalf("SaveCLISession: %v", err)
	}

	// Arm a server that fails loudly. If the code exchanges instead of
	// recognising the winner, the test fails on this rather than reaching the
	// real endpoint.
	var calls atomic.Int32
	var seenMarker atomic.Bool
	refreshServerRefusing(t, &calls, &seenMarker)

	got, err := RefreshAccessToken("old-refresh")
	if err != nil {
		t.Fatalf("waiter did not recognise a rotation that preserved the refresh token: %v", err)
	}
	if got.AccessToken != "winner-access" {
		t.Errorf("returned %q, want winner-access", got.AccessToken)
	}
	cached, _, err := LoadAccessToken()
	if err != nil {
		t.Fatalf("LoadAccessToken: %v", err)
	}
	if cached != "winner-access" {
		t.Errorf("in-process cache = %q, want winner-access", cached)
	}
	if calls.Load() != 0 {
		t.Errorf("exchanged %d times despite a published winner", calls.Load())
	}
}

// The first generation check and the marker acquire are not atomic. A winner
// that publishes and releases in that gap must still be seen, or this caller
// holds a fresh marker and spends a refresh token that is already consumed.
func TestRefresh_ReChecksAfterAcquiringMarker(t *testing.T) {
	armCLISession(t, time.Hour)
	var calls atomic.Int32
	var seenMarker atomic.Bool
	refreshServerRefusing(t, &calls, &seenMarker)

	// Publish the winner's result only once this caller has passed its first
	// check, so the sole chance to notice is the re-check under the marker.
	orig := beforeAcquireHook
	beforeAcquireHook = func() {
		if err := SaveCLISession(CLISession{
			AccessToken:  "winner-access",
			RefreshToken: "old-refresh",
			ExpiresAt:    time.Now().Add(time.Hour).UTC(),
			ObtainedAt:   time.Now().Add(time.Second).UTC(),
			AccountEmail: "someone@example.com",
		}); err != nil {
			t.Errorf("winner SaveCLISession: %v", err)
		}
	}
	t.Cleanup(func() { beforeAcquireHook = orig })

	got, err := RefreshAccessToken("old-refresh")
	if err != nil {
		t.Fatalf("a rotation published in the check/acquire gap was missed: %v", err)
	}
	if got.AccessToken != "winner-access" {
		t.Errorf("returned %q, want winner-access", got.AccessToken)
	}
	if calls.Load() != 0 {
		t.Errorf("spent the stale token %d times despite a published winner", calls.Load())
	}
	// The marker must not be left behind by the re-check path.
	if _, err := os.Stat(rotationMarkerPath()); err == nil {
		t.Error("marker survived a re-check early return")
	}
}

// Logout must win against an in-flight refresh. A rotation that completes after
// the session is removed would recreate the credential the user just deleted,
// while logout had already reported success.
func TestRefresh_LogoutDuringRotationIsNotReversed(t *testing.T) {
	armCLISession(t, time.Hour)

	orig := refreshClient
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The user logs out while this exchange is in flight. Remove the
		// session directly rather than via ClearCLISession, which would block
		// on the refresh lock this caller holds -- the point here is the
		// cross-process case, where no shared lock exists.
		os.Remove(CLISessionPath())
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"post-logout","refresh_token":"post-logout-r","expires_in":3600}`))
	}))
	defer srv.Close()
	SetRefreshHTTPClient(&http.Client{Transport: &rewriteTransport{target: srv.URL}})
	defer SetRefreshHTTPClient(orig)

	_, err := RefreshAccessToken("old-refresh")
	if !errors.Is(err, ErrSessionLoggedOutDuringRotation) {
		t.Fatalf("want ErrSessionLoggedOutDuringRotation, got %v", err)
	}
	// The credential must stay gone.
	if _, statErr := os.Stat(CLISessionPath()); statErr == nil {
		t.Error("the rotation recreated a session the user had logged out of")
	}
	if _, loadErr := LoadCLISession(); !errors.Is(loadErr, ErrNoCLISession) {
		t.Errorf("want ErrNoCLISession after logout, got %v", loadErr)
	}
	// And no marker is left implying an interrupted rotation on a session that
	// no longer exists.
	if _, mErr := os.Stat(rotationMarkerPath()); mErr == nil {
		t.Error("marker survived a logout-abandoned rotation")
	}
}

// A marker whose directory entry cannot be made durable must not authorize an
// exchange: spending a single-use token with no recoverable record of it is the
// failure the marker exists to prevent.
func TestBeginRotation_RefusesWithoutDurableDirectory(t *testing.T) {
	sessionSandbox(t)
	if err := SaveCLISession(sampleSession()); err != nil {
		t.Fatalf("SaveCLISession: %v", err)
	}
	// The marker's whole job is surviving a crash. If the directory entry cannot
	// be made durable, a crash can lose the marker after the token is already
	// spent, so beginning the rotation at all is worse than refusing it.
	orig := syncDirFn
	syncDirFn = func(string) error { return errors.New("simulated fsync failure") }
	t.Cleanup(func() { syncDirFn = orig })

	if _, err := BeginRotation(); err == nil {
		t.Fatal("BeginRotation succeeded despite an undurable directory; a crash could now lose the marker on a spent token")
	}
	// And it must not leave the marker it could not make durable.
	if _, err := os.Stat(rotationMarkerPath()); err == nil {
		t.Error("a refused rotation left its marker behind")
	}
}

// A desktop-owned source must not touch the marker at all -- it is refused
// before any rotation bookkeeping starts.
func TestRefresh_DesktopSourceNeverMarksRotation(t *testing.T) {
	ResetTokenCache()
	defer ResetTokenCache()
	dir := filepath.Join(t.TempDir(), "granola-pp-cli")
	t.Setenv("GRANOLA_CLI_SESSION_PATH", filepath.Join(dir, "session.json"))
	support := t.TempDir()
	t.Setenv("GRANOLA_SUPPORT_DIR", support)
	t.Setenv("GRANOLA_WORKOS_TOKEN", "")
	t.Setenv("GRANOLA_WORKOS_REFRESH", "")
	if err := os.WriteFile(filepath.Join(support, "supabase.json.enc"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write enc: %v", err)
	}
	writeStoredAccounts(t, support)
	if _, err := LoadRefreshToken(); err != nil {
		t.Fatalf("LoadRefreshToken: %v", err)
	}

	if _, err := RefreshAccessToken("desktop-refresh"); !errors.Is(err, ErrRefreshRefused) {
		t.Fatalf("want ErrRefreshRefused, got %v", err)
	}
	if _, err := os.Stat(rotationMarkerPath()); err == nil {
		t.Error("a refused desktop refresh wrote a rotation marker")
	}
}

// The dedup check compares the stored session against what the caller held when
// it decided to refresh. That snapshot is taken before the lock, so it can land
// in the window after the winner updates the cache and before it releases --
// making the loser compare the winner's new token against itself, conclude
// nothing rotated, and present its own already-spent refresh token a second
// time. Spending a single-use chain twice breaks it permanently, so the spent
// token is tracked exactly rather than inferred from the cache.
func TestRefresh_SpentTokenIsNotPresentedTwice(t *testing.T) {
	armCLISession(t, time.Hour)
	var calls atomic.Int32
	orig := refreshClient
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"rotated-access","refresh_token":"rotated-refresh","expires_in":3600}`))
	}))
	t.Cleanup(srv.Close)
	SetRefreshHTTPClient(&http.Client{Transport: &rewriteTransport{target: srv.URL}})
	t.Cleanup(func() { SetRefreshHTTPClient(orig) })

	if _, err := RefreshAccessToken("chain-token-1"); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	// Re-presenting the consumed token is the lost-snapshot case: the cache now
	// holds the winner's result, so nothing cache-visible says a rotation
	// happened, and only the spent-token record can stop a second exchange.
	if _, err := RefreshAccessToken("chain-token-1"); err != nil {
		t.Fatalf("second refresh with the spent token: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("the spent refresh token was presented again: %d exchanges, want 1", got)
	}
}
