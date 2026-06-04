// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const lookupProfileFixture = `{
  "name": "Ada Lovelace",
  "bio": "Mathematician",
  "bioFrom": "wikipedia",
  "location": "London",
  "timeZone": "Europe/London",
  "avatar": "https://example.com/ada.jpg",
  "twitterHandle": "ada",
  "memberSince": "2018-01-01T00:00:00Z",
  "links": [{"url": "https://example.com/ada", "title": "Home"}],
  "salesData": {"sources": [{"name": "clearbit"}]}
}`

// fakeJPEG is a minimal byte blob standing in for a photo payload.
var fakeJPEG = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}

func lookupTestServer(t *testing.T, profileStatus int, profileBody string, photoStatus int, photoBody []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/v2/profile"):
			if profileStatus != 200 {
				http.Error(w, `{"error":"boom"}`, profileStatus)
				return
			}
			fmt.Fprint(w, profileBody)
		case strings.Contains(r.URL.Path, "/contact/") && strings.HasSuffix(r.URL.Path, "/photo"):
			if photoStatus != 200 {
				http.Error(w, "no photo", photoStatus)
				return
			}
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write(photoBody)
		default:
			http.Error(w, "unexpected path: "+r.URL.Path, 404)
		}
	}))
}

func TestLookup_HappyPath(t *testing.T) {
	srv := lookupTestServer(t, 200, lookupProfileFixture, 404, nil)
	defer srv.Close()
	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	stdout, _, err := executeCmd(t, "--config", configPath, "--json", "lookup", "ada@example.com")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	var prof contactProfile
	if err := json.Unmarshal([]byte(stdout), &prof); err != nil {
		t.Fatalf("parse output: %v\n%s", err, stdout)
	}
	if prof.Name != "Ada Lovelace" {
		t.Errorf("name = %q want Ada Lovelace", prof.Name)
	}
	if prof.Location != "London" {
		t.Errorf("location = %q want London", prof.Location)
	}
	if prof.TwitterHandle != "ada" {
		t.Errorf("twitterHandle = %q want ada", prof.TwitterHandle)
	}
}

func TestLookup_JSONSelect(t *testing.T) {
	srv := lookupTestServer(t, 200, lookupProfileFixture, 404, nil)
	defer srv.Close()
	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	stdout, _, err := executeCmd(t, "--config", configPath, "--json", "--select", "name,location", "lookup", "ada@example.com")
	if err != nil {
		t.Fatalf("lookup --select: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(stdout), &m); err != nil {
		t.Fatalf("parse: %v\n%s", err, stdout)
	}
	if _, ok := m["name"]; !ok {
		t.Errorf("expected name in selected output: %s", stdout)
	}
	if _, ok := m["bio"]; ok {
		t.Errorf("bio should be filtered out by --select name,location: %s", stdout)
	}
}

func TestLookup_PhotoSuccess(t *testing.T) {
	srv := lookupTestServer(t, 200, lookupProfileFixture, 200, fakeJPEG)
	defer srv.Close()
	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	out := filepath.Join(t.TempDir(), "ada.jpg")
	_, stderr, err := executeCmd(t, "--config", configPath, "--json", "lookup", "ada@example.com", "--photo", out)
	if err != nil {
		t.Fatalf("lookup --photo: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("photo not written: %v", err)
	}
	if len(got) != len(fakeJPEG) {
		t.Errorf("photo size = %d want %d", len(got), len(fakeJPEG))
	}
	if !strings.Contains(stderr, "wrote") {
		t.Errorf("expected wrote-confirmation on stderr, got %q", stderr)
	}
}

func TestLookup_PhotoNotFound(t *testing.T) {
	srv := lookupTestServer(t, 200, lookupProfileFixture, 404, nil)
	defer srv.Close()
	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	out := filepath.Join(t.TempDir(), "missing.jpg")
	_, stderr, err := executeCmd(t, "--config", configPath, "--json", "lookup", "ada@example.com", "--photo", out)
	if err != nil {
		t.Fatalf("lookup --photo (404) should not error: %v", err)
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Errorf("no file should be written when photo is 404")
	}
	if !strings.Contains(stderr, "no photo on file") {
		t.Errorf("expected 'no photo on file' message, got %q", stderr)
	}
}

func TestLookup_SparseProfile(t *testing.T) {
	srv := lookupTestServer(t, 200, `{"name":"Solo Name"}`, 404, nil)
	defer srv.Close()
	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	stdout, _, err := executeCmd(t, "--config", configPath, "--json", "lookup", "solo@example.com")
	if err != nil {
		t.Fatalf("lookup sparse: %v", err)
	}
	var prof contactProfile
	if err := json.Unmarshal([]byte(stdout), &prof); err != nil {
		t.Fatalf("parse: %v\n%s", err, stdout)
	}
	if prof.Name != "Solo Name" {
		t.Errorf("name = %q want Solo Name", prof.Name)
	}
	// Email backfilled from the argument when the response omits it.
	if prof.Email != "solo@example.com" {
		t.Errorf("email = %q want solo@example.com (backfilled)", prof.Email)
	}
}

func TestLookup_MalformedEmail(t *testing.T) {
	configPath, _ := withConfigPath(t)
	_, _, err := executeCmd(t, "--config", configPath, "lookup", "notanemail")
	if err == nil {
		t.Fatalf("expected usage error for malformed email")
	}
	if !strings.Contains(err.Error(), "not a valid email") {
		t.Errorf("error = %q want a 'not a valid email' usage message", err.Error())
	}
}

func TestLookup_PhotoEndpointError_ProfileStillEmitted(t *testing.T) {
	// Photo endpoint returns 500; the profile was already fetched and must
	// still be emitted (P1 regression guard: a photo failure must not
	// discard the profile).
	srv := lookupTestServer(t, 200, lookupProfileFixture, 500, nil)
	defer srv.Close()
	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	out := filepath.Join(t.TempDir(), "ada.jpg")
	stdout, stderr, err := executeCmd(t, "--config", configPath, "--json", "lookup", "ada@example.com", "--photo", out)
	if err != nil {
		t.Fatalf("photo endpoint failure must be non-fatal, got: %v", err)
	}
	var prof contactProfile
	if jerr := json.Unmarshal([]byte(stdout), &prof); jerr != nil {
		t.Fatalf("profile must still be emitted on stdout: %v\n%s", jerr, stdout)
	}
	if prof.Name != "Ada Lovelace" {
		t.Errorf("name = %q want Ada Lovelace (profile discarded?)", prof.Name)
	}
	if !strings.Contains(stderr, "could not download photo") {
		t.Errorf("expected photo-failure warning on stderr, got %q", stderr)
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Errorf("no file should be written when the photo fetch failed")
	}
}

func TestLookup_ProfileServerError(t *testing.T) {
	srv := lookupTestServer(t, 500, "", 404, nil)
	defer srv.Close()
	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	_, _, err := executeCmd(t, "--config", configPath, "--json", "lookup", "ada@example.com")
	if err == nil {
		t.Fatalf("expected error on HTTP 500")
	}
}
