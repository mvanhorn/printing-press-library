// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Library patch: keep provider credentials off presigned storage requests.

package cli

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// validatePresignedUploadURL refuses plaintext or malformed destinations before
// a document or audio file leaves the machine. Sarvam's upload URLs are HTTPS;
// accepting HTTP would expose user content and the signed query string.
func validatePresignedUploadURL(rawURL string) error {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("invalid presigned upload URL")
	}
	if !strings.EqualFold(u.Scheme, "https") || u.Host == "" {
		return fmt.Errorf("presigned upload URL must be an absolute HTTPS URL")
	}
	if u.User != nil {
		return fmt.Errorf("presigned upload URL must not contain user information")
	}
	return nil
}

// presignedUploadHTTPClient reuses the generated client's timeout and
// transport, but deliberately drops its cookie jar and redirect hook. The
// generated hook adds api-subscription-key on same-host redirects, which is
// correct for Sarvam API calls but must never run for a third-party storage
// URL. Redirects are returned to the caller as 3xx responses so uploads cannot
// silently move to a different destination.
func presignedUploadHTTPClient(base *http.Client, fallbackTimeout time.Duration) *http.Client {
	if base == nil {
		return &http.Client{
			Timeout:       fallbackTimeout,
			CheckRedirect: refusePresignedUploadRedirect,
		}
	}
	clone := *base
	clone.Jar = nil
	clone.CheckRedirect = refusePresignedUploadRedirect
	if clone.Timeout <= 0 {
		clone.Timeout = fallbackTimeout
	}
	return &clone
}

func refusePresignedUploadRedirect(_ *http.Request, _ []*http.Request) error {
	return http.ErrUseLastResponse
}
