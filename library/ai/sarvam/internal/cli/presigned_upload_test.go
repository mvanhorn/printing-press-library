// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestValidatePresignedUploadURL(t *testing.T) {
	for _, rawURL := range []string{
		"https://storage.example.test/container/file.wav?sig=fixture",
		"HTTPS://storage.example.test/file.pdf?sig=fixture",
	} {
		if err := validatePresignedUploadURL(rawURL); err != nil {
			t.Errorf("validatePresignedUploadURL(%q) = %v", rawURL, err)
		}
	}
	for _, rawURL := range []string{
		"http://storage.example.test/file.wav?sig=fixture",
		"/relative/file.wav",
		"https:///missing-host",
		"https://user:pass@storage.example.test/file.wav",
	} {
		if err := validatePresignedUploadURL(rawURL); err == nil {
			t.Errorf("validatePresignedUploadURL(%q) unexpectedly succeeded", rawURL)
		}
	}
}

func TestPresignedUploadHTTPClientRefusesRedirectWithoutMutatingBase(t *testing.T) {
	baseRedirectCalled := false
	base := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			baseRedirectCalled = true
			return nil
		},
	}
	upload := presignedUploadHTTPClient(base, time.Minute)
	if upload == base {
		t.Fatal("presignedUploadHTTPClient returned the auth-aware base client")
	}
	if upload.Timeout != base.Timeout {
		t.Fatalf("upload timeout = %s, want %s", upload.Timeout, base.Timeout)
	}
	if err := upload.CheckRedirect(nil, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("upload redirect error = %v, want http.ErrUseLastResponse", err)
	}
	if baseRedirectCalled {
		t.Fatal("upload client invoked the base client's redirect hook")
	}
	if err := base.CheckRedirect(nil, nil); err != nil || !baseRedirectCalled {
		t.Fatalf("base redirect hook changed: called=%v err=%v", baseRedirectCalled, err)
	}
}

func TestPresignedUploadHTTPClientBoundsNilBase(t *testing.T) {
	upload := presignedUploadHTTPClient(nil, 9*time.Second)
	if upload.Timeout != 9*time.Second {
		t.Fatalf("upload timeout = %s, want 9s", upload.Timeout)
	}
	if err := upload.CheckRedirect(nil, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("upload redirect error = %v, want http.ErrUseLastResponse", err)
	}
}
