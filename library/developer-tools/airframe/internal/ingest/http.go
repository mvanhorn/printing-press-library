// Copyright 2026 ChrisDrit. Licensed under Apache-2.0. See LICENSE.

// Package ingest downloads, parses, and inserts FAA + NTSB bulk data into
// the local SQLite store. Each source has its own file (faa.go, ntsb.go);
// http.go centralizes the conditional-GET pattern used by both.
package ingest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// UserAgent identifies airframe to upstream servers. The FAA Akamai front
// end is picky: anything that mentions "bot", "compatible", or a github URL
// gets 403'd. A bare "Mozilla/5.0 airframe-pp-cli/0.1.0" passes; we keep
// the airframe suffix so admins can still grep their logs.
const UserAgent = "Mozilla/5.0 airframe-pp-cli/0.1.0"

// PATCH: bound worst-case wall time on a stalled server. http.DefaultClient
// has Timeout=0 (infinite); FAA/NTSB downloads are ~80–90 MB, so 30 minutes
// is a generous ceiling on a healthy link and a finite kill-switch on a
// hung one. The per-request context still controls earlier cancellation.
var downloadClient = &http.Client{Timeout: 30 * time.Minute}

// conditionalDownload GETs url with optional If-Modified-Since. On 304 it
// returns (true, "", nil) signaling skip. On 200 it writes the body to dst
// and returns (false, last-modified, nil). Any other status is an error.
//
// We use GET (not HEAD) because some upstreams — notably the FAA's
// Akamai-fronted CDN — route HEAD requests to a stale error page while GET
// goes to the live IIS origin. If-Modified-Since lets us short-circuit
// without transferring the body when nothing has changed.
func conditionalDownload(ctx context.Context, url, ifModifiedSince string, dst io.Writer) (skipped bool, lastModified string, bytesWritten int64, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, "", 0, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("User-Agent", UserAgent)
	if ifModifiedSince != "" {
		req.Header.Set("If-Modified-Since", ifModifiedSince)
	}
	resp, err := downloadClient.Do(req)
	if err != nil {
		return false, "", 0, fmt.Errorf("requesting %s: %w", url, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		return true, "", 0, nil
	case http.StatusOK, http.StatusPartialContent:
		n, err := io.Copy(dst, resp.Body)
		if err != nil {
			return false, "", n, fmt.Errorf("streaming body: %w", err)
		}
		return false, resp.Header.Get("Last-Modified"), n, nil
	default:
		return false, "", 0, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}
}

// downloadToTempFile is a convenience wrapper that writes the response into
// a freshly-created tmp file. Caller is responsible for removing it.
func downloadToTempFile(ctx context.Context, url, ifModifiedSince, namePattern string) (skipped bool, lastModified string, tmpPath string, bytesWritten int64, err error) {
	f, err := os.CreateTemp("", namePattern)
	if err != nil {
		return false, "", "", 0, fmt.Errorf("creating tmp file: %w", err)
	}
	defer f.Close()

	skipped, lastModified, bytesWritten, err = conditionalDownload(ctx, url, ifModifiedSince, f)
	if err != nil || skipped {
		os.Remove(f.Name())
		return skipped, lastModified, "", bytesWritten, err
	}
	return false, lastModified, f.Name(), bytesWritten, nil
}
