// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// HAND-WRITTEN — not generated. MyFitnessPal-specific HTTP transport.
//
// Two responsibilities:
//
//  1. Rewrite the request host. Paths starting with /v2/ are served by
//     api.myfitnesspal.com; everything else stays on www.myfitnesspal.com.
//     The generator's client uses a single base URL (www) so this transport
//     fixes that up at the wire layer.
//
//  2. Inject MyFitnessPal-specific headers that no public spec advertises but
//     that the live API requires:
//       - mfp-client-id: "mfp-main-js" on every request (constant; the web
//         client identifier captured in HAR analysis).
//       - mfp-user-id: numeric account id, attached to api.myfitnesspal.com
//         requests only. Resolved on first /user/auth_token call and cached
//         in ~/.config/myfitnesspal-pp-cli/mfp-state.json.
//       - origin: "https://www.myfitnesspal.com" on api.myfitnesspal.com
//         requests (the v2 surface enforces same-site CORS).

package client

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// MFPTransport wraps an http.RoundTripper to apply MFP host rewriting and
// header injection. Construct with NewMFPTransport.
type MFPTransport struct {
	Underlying http.RoundTripper

	mu        sync.RWMutex
	cachedUID string
	statePath string
}

// NewMFPTransport returns a RoundTripper that wraps next with MFP-specific
// header and host handling. statePath is the path to the JSON file where the
// cached numeric user id lives; pass an empty string to disable caching.
func NewMFPTransport(next http.RoundTripper, statePath string) *MFPTransport {
	if next == nil {
		next = http.DefaultTransport
	}
	t := &MFPTransport{Underlying: next, statePath: statePath}
	t.loadState()
	return t
}

// SetUserID stores the numeric MFP user id. Called by the auth-token flow
// after successfully bootstrapping a token from /user/auth_token. The cached
// id is persisted to disk so subsequent commands skip the bootstrap call.
func (t *MFPTransport) SetUserID(uid string) {
	t.mu.Lock()
	t.cachedUID = uid
	t.mu.Unlock()
	t.saveState()
}

// UserID returns the cached numeric user id, or the empty string if not yet
// resolved.
func (t *MFPTransport) UserID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.cachedUID
}

// RoundTrip implements http.RoundTripper.
func (t *MFPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL != nil && req.URL.Host == "www.myfitnesspal.com" {
		path := req.URL.Path
		if strings.HasPrefix(path, "/v2/") {
			req = req.Clone(req.Context())
			req.URL.Host = "api.myfitnesspal.com"
			req.Host = "api.myfitnesspal.com"
		}
	}

	req.Header.Set("mfp-client-id", "mfp-main-js")

	if req.URL != nil && req.URL.Host == "api.myfitnesspal.com" {
		if req.Header.Get("origin") == "" && req.Header.Get("Origin") == "" {
			req.Header.Set("Origin", "https://www.myfitnesspal.com")
		}
		if req.Header.Get("Referer") == "" {
			req.Header.Set("Referer", "https://www.myfitnesspal.com/")
		}
		if uid := t.UserID(); uid != "" {
			req.Header.Set("mfp-user-id", uid)
		}
	}

	return t.Underlying.RoundTrip(req)
}

type mfpState struct {
	UserID string `json:"user_id,omitempty"`
}

func (t *MFPTransport) loadState() {
	if t.statePath == "" {
		return
	}
	data, err := os.ReadFile(t.statePath)
	if err != nil {
		return
	}
	var s mfpState
	if err := json.Unmarshal(data, &s); err != nil {
		return
	}
	t.mu.Lock()
	t.cachedUID = s.UserID
	t.mu.Unlock()
}

func (t *MFPTransport) saveState() {
	if t.statePath == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(t.statePath), 0o700); err != nil {
		return
	}
	data, err := json.Marshal(mfpState{UserID: t.UserID()})
	if err != nil {
		return
	}
	tmp := t.statePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, t.statePath)
}
