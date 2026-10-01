// Copyright 2026 smoochy and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

type blockingCookieJar struct {
	http.CookieJar
	entered chan struct{}
	release chan struct{}
	blocked atomic.Bool
}

func (j *blockingCookieJar) Cookies(u *url.URL) []*http.Cookie {
	if !j.blocked.Swap(true) {
		close(j.entered)
		<-j.release
	}
	return j.CookieJar.Cookies(u)
}

func TestSessionInvalidationCannotOverwriteNewSession(t *testing.T) {
	t.Setenv("SYNOLOGY_DATA_DIR", t.TempDir())
	manager := newSessionManager(time.Second, "", "https://synology.example", false)
	manager.SetSession(dsmLoginResult{SID: "old-synthetic"})
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	blocking := &blockingCookieJar{CookieJar: jar, entered: make(chan struct{}), release: make(chan struct{})}
	manager.jar = blocking
	invalidateDone := make(chan struct{})
	go func() {
		manager.Invalidate()
		close(invalidateDone)
	}()
	select {
	case <-blocking.entered:
	case <-time.After(time.Second):
		t.Fatal("invalidation did not reach persistence")
	}
	setDone := make(chan struct{})
	go func() {
		manager.SetSession(dsmLoginResult{SID: "new-synthetic"})
		close(setDone)
	}()
	// With an unlocked persistence step, SetSession can write the new token
	// while the older invalidation is paused, then lose it to the old write.
	// Allow both schedules; the final disk state must match the new token.
	select {
	case <-setDone:
	case <-time.After(100 * time.Millisecond):
	}
	close(blocking.release)
	select {
	case <-invalidateDone:
	case <-time.After(time.Second):
		t.Fatal("invalidation remained blocked")
	}
	select {
	case <-setDone:
	case <-time.After(time.Second):
		t.Fatal("new session remained blocked")
	}
	if manager.Token() != "new-synthetic" {
		t.Fatal("new session was lost in memory")
	}
	data, err := os.ReadFile(manager.sessionFilePath())
	if err != nil {
		t.Fatal(err)
	}
	var record sessionRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	if record.Token != "new-synthetic" {
		t.Fatal("older invalidation overwrote the new session on disk")
	}
}
