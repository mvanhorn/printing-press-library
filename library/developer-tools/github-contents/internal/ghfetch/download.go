// Copyright 2026 Rick van de Laar and contributors. Licensed under Apache-2.0. See LICENSE.

package ghfetch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/github-contents/internal/cliutil"
)

// responseHeaderTimeout guards the request/handshake phase of a streamed
// download: if the server hasn't started answering within this window the
// request fails fast. Deliberately NOT an http.Client.Timeout — that would
// cap total transfer time and kill any file that takes longer than the
// window to stream, no matter how healthy the connection is.
const responseHeaderTimeout = 30 * time.Second

// defaultConcurrency is used when Downloader.Concurrency is <= 0.
const defaultConcurrency = 8

// NewStreamingHTTPClient returns an http.Client tuned for streamed byte
// transfers (raw CDN files, tarballs, release assets): connection, TLS,
// and response-header timeouts protect against dead servers, but there
// is NO overall Client.Timeout, so total transfer time is unbounded — a
// multi-GB body streams for as long as it needs. Safety comes from
// context cancellation (caller-controlled) plus the per-file size/SHA
// checks. The transport clones http.DefaultTransport so proxy settings,
// dial timeouts (30s), and TLS handshake timeout (10s) are preserved.
func NewStreamingHTTPClient() *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.ResponseHeaderTimeout = responseHeaderTimeout
	return &http.Client{Transport: tr}
}

// Downloader streams TreeFile content to a local directory. It downloads
// from GitHub's raw CDN by default (fast, no JSON/base64 overhead), falling
// back once per file to the git blobs API on 404/403 (private repos, or
// CDN propagation lag on a just-pushed commit).
//
// A fresh, dedicated HTTPClient is used rather than the generated
// internal/client.Client's underlying transport: raw.githubusercontent.com
// responses are streamed binary bytes, not JSON API payloads, and the
// generated client's response path (caching, JSON sanitization, binary
// base64-envelope wrapping) is built for the latter. Reusing the same
// http.Client would still route every raw-CDN byte through code designed
// to buffer and reshape whole response bodies in memory.
type Downloader struct {
	Concurrency int
	Force       bool
	Flatten     bool
	Limiter     *cliutil.AdaptiveLimiter
	Token       string // full Authorization header value, e.g. "Bearer ghp_..."; empty = unauthenticated
	HTTPClient  *http.Client
	APIClient   API // optional: used for the one-shot git-blobs-API fallback on 404/403 from the raw CDN
}

// FileFailure records one file's download failure without aborting the
// rest of the batch.
type FileFailure struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// DownloadReport summarizes a Downloader.Download call. Downloaded and
// Skipped never double-count a file that also appears in Failures.
type DownloadReport struct {
	Downloaded  int           `json:"downloaded"`
	Skipped     int           `json:"skipped"`
	Bytes       int64         `json:"bytes"`
	Failures    []FileFailure `json:"failures,omitempty"`
	LFSPointers []string      `json:"lfs_pointers,omitempty"`
}

// downloadJob is a single planned download: the source TreeFile, the
// safety-checked slash-relative path used both as its FanoutRun source
// identity and (in non-flatten mode) as the destination's subpath, and the
// final absolute local destination.
type downloadJob struct {
	file     TreeFile
	relPath  string
	destPath string
}

type fileOutcome struct {
	status     string // "downloaded" | "skipped"
	bytes      int64
	lfsPointer bool
}

// Download fetches every file in files (as returned by WalkTree, optionally
// filtered by MatchGlobs) into destDir. addr.Ref must already be resolved
// (WalkResult.Ref, not the possibly-empty Address.Ref a caller started
// with) since it is used to build the raw-CDN URL.
func (d *Downloader) Download(ctx context.Context, addr Address, files []TreeFile, destDir string) (*DownloadReport, error) {
	jobs, planFailures := d.planJobs(addr, files, destDir)

	concurrency := d.Concurrency
	if concurrency < 1 {
		concurrency = defaultConcurrency
	}
	httpClient := d.HTTPClient
	if httpClient == nil {
		httpClient = NewStreamingHTTPClient()
	}

	results, errs := cliutil.FanoutRun(ctx, jobs,
		func(j downloadJob) string { return j.relPath },
		func(ctx context.Context, j downloadJob) (fileOutcome, error) {
			return d.downloadOne(ctx, httpClient, addr, j)
		},
		cliutil.WithConcurrency(concurrency),
	)

	report := &DownloadReport{Failures: planFailures}
	for _, r := range results {
		switch r.Value.status {
		case "downloaded":
			report.Downloaded++
			report.Bytes += r.Value.bytes
			if r.Value.lfsPointer {
				report.LFSPointers = append(report.LFSPointers, r.Source)
			}
		case "skipped":
			report.Skipped++
		}
	}
	for _, e := range errs {
		report.Failures = append(report.Failures, FileFailure{Path: e.Source, Error: e.Err.Error()})
	}
	return report, nil
}

// planJobs computes, single-threaded and up front, the destination path
// for every file — so flatten-mode collision disambiguation is
// deterministic instead of racing across worker goroutines. A file whose
// remote path is unsafe to represent locally (traversal, absolute,
// drive/volume syntax) is diverted into the returned failures instead of
// aborting the whole batch — one hostile or host-incompatible filename
// must not brick the download of every clean sibling.
func (d *Downloader) planJobs(addr Address, files []TreeFile, destDir string) ([]downloadJob, []FileFailure) {
	type planned struct {
		file TreeFile
		rel  string
		base string
	}
	var failures []FileFailure
	tmp := make([]planned, 0, len(files))
	for _, f := range files {
		rel, err := SafeRelPath(f.RelTo(addr.Path))
		if err != nil {
			failures = append(failures, FileFailure{Path: f.Path, Error: err.Error()})
			continue
		}
		tmp = append(tmp, planned{file: f, rel: rel, base: path.Base(rel)})
	}

	jobs := make([]downloadJob, 0, len(tmp))
	if !d.Flatten {
		for _, t := range tmp {
			jobs = append(jobs, downloadJob{file: t.file, relPath: t.rel, destPath: filepath.Join(destDir, filepath.FromSlash(t.rel))})
		}
		return jobs, failures
	}

	baseCounts := map[string]int{}
	for _, t := range tmp {
		baseCounts[t.base]++
	}
	used := map[string]int{}
	for _, t := range tmp {
		name := t.base
		if baseCounts[t.base] > 1 {
			dir := path.Dir(t.rel)
			if dir != "." && dir != "" {
				name = strings.ReplaceAll(dir, "/", "-") + "-" + t.base
			}
		}
		if n := used[name]; n > 0 {
			ext := path.Ext(name)
			stem := strings.TrimSuffix(name, ext)
			name = fmt.Sprintf("%s-%d%s", stem, n+1, ext)
		}
		used[name]++
		jobs = append(jobs, downloadJob{file: t.file, relPath: t.rel, destPath: filepath.Join(destDir, name)})
	}
	return jobs, failures
}

// prefixCapture is an io.Writer that retains only the first cap bytes it
// sees and discards the rest without erroring. Used as a TeeReader sink
// for the LFS-pointer probe: IsLFSPointer only needs the file's prefix,
// and an LFS-tracked entry's raw-CDN body can be arbitrarily large, so an
// unbounded buffer would balloon memory.
type prefixCapture struct {
	buf []byte
	cap int
}

func (p *prefixCapture) Write(b []byte) (int, error) {
	if remaining := p.cap - len(p.buf); remaining > 0 {
		if len(b) <= remaining {
			p.buf = append(p.buf, b...)
		} else {
			p.buf = append(p.buf, b[:remaining]...)
		}
	}
	return len(b), nil
}

func (d *Downloader) downloadOne(ctx context.Context, httpClient *http.Client, addr Address, j downloadJob) (fileOutcome, error) {
	out := fileOutcome{}
	if !d.Force {
		if info, statErr := os.Stat(j.destPath); statErr == nil && !info.IsDir() {
			if localSHA, shaErr := GitBlobSHAFile(j.destPath); shaErr == nil && localSHA == j.file.SHA {
				out.status = "skipped"
				return out, nil
			}
		}
	}

	d.Limiter.Wait()
	body, err := d.fetchBytes(ctx, httpClient, addr, j.file)
	if err != nil {
		return out, err
	}
	defer body.Close()

	// Small tree entries might be LFS pointers; capture the stream's prefix
	// as it goes to disk so no second read is needed. The capture is capped
	// (see prefixCapture) because an LFS-tracked entry's raw-CDN body is
	// the RESOLVED object, which can be arbitrarily large.
	var lfsProbe *prefixCapture
	var reader io.Reader = body
	if j.file.Size > 0 && j.file.Size <= LFSMaxPointerSize {
		lfsProbe = &prefixCapture{cap: len(gitLFSPointerPrefix)}
		reader = io.TeeReader(body, lfsProbe)
	}

	written, streamErr := StreamToFile(reader, j.destPath, j.file.Size)
	if streamErr != nil {
		// LFS-tracked file: the tree records the pointer's size (small),
		// but the raw CDN serves the resolved object, so the byte counts
		// disagree. Recover via the blobs API, which returns the pointer
		// bytes as git stores them — write those and flag the path.
		// Genuine mismatches (expected size beyond any possible pointer)
		// stay hard failures.
		if errors.Is(streamErr, ErrSizeMismatch) && j.file.Size > 0 && j.file.Size <= LFSMaxPointerSize && d.APIClient != nil {
			raw, blobErr := FetchBlobBytes(ctx, d.APIClient, addr, j.file.SHA)
			if blobErr != nil {
				return out, fmt.Errorf("downloading %s: size mismatch and blob-API recovery failed: %w", j.file.Path, blobErr)
			}
			// A size check against len(raw) would be a tautology here;
			// verify the recovered payload by its git blob SHA instead.
			if shaErr := checkBlobSHA(raw, j.file.SHA); shaErr != nil {
				return out, fmt.Errorf("downloading %s: blob-API recovery returned wrong content: %w", j.file.Path, shaErr)
			}
			if _, writeErr := StreamToFile(bytes.NewReader(raw), j.destPath, int64(len(raw))); writeErr != nil {
				return out, fmt.Errorf("downloading %s: %w", j.file.Path, writeErr)
			}
			out.status = "downloaded"
			out.bytes = int64(len(raw))
			out.lfsPointer = IsLFSPointer(raw)
			return out, nil
		}
		return out, fmt.Errorf("downloading %s: %w", j.file.Path, streamErr)
	}

	out.status = "downloaded"
	out.bytes = written
	if lfsProbe != nil {
		out.lfsPointer = IsLFSPointer(lfsProbe.buf)
	}
	return out, nil
}

// fetchBytes returns the byte stream for file's content: raw CDN first,
// falling back once to the git blobs API on 404/403.
func (d *Downloader) fetchBytes(ctx context.Context, httpClient *http.Client, addr Address, file TreeFile) (io.ReadCloser, error) {
	ref := addr.Ref
	if ref == "" {
		ref = "HEAD"
	}
	rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s",
		url.PathEscape(addr.Owner), url.PathEscape(addr.Repo), url.PathEscape(ref), EscapePath(file.Path))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if d.Token != "" {
		req.Header.Set("Authorization", d.Token)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", file.Path, err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return resp.Body, nil
	case http.StatusTooManyRequests:
		defer resp.Body.Close()
		return nil, &cliutil.RateLimitError{URL: rawURL, RetryAfter: cliutil.RetryAfter(resp)}
	case http.StatusNotFound, http.StatusForbidden:
		_ = resp.Body.Close()
		return d.fetchBlobFallback(ctx, addr, file)
	default:
		defer resp.Body.Close()
		return nil, fmt.Errorf("downloading %s: HTTP %d from raw CDN", file.Path, resp.StatusCode)
	}
}

// fetchBlobFallback resolves file content via the shared blobs-API helper
// (FetchBlobBytes) when the raw CDN can't serve it directly.
func (d *Downloader) fetchBlobFallback(ctx context.Context, addr Address, file TreeFile) (io.ReadCloser, error) {
	if d.APIClient == nil || file.SHA == "" {
		return nil, fmt.Errorf("downloading %s: not found on raw CDN and no blob-API fallback available\nhint: private repos return 404 without auth — set GITHUB_TOKEN", file.Path)
	}
	raw, err := FetchBlobBytes(ctx, d.APIClient, addr, file.SHA)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: raw CDN failed and blob-API fallback failed: %w", file.Path, err)
	}
	if shaErr := checkBlobSHA(raw, file.SHA); shaErr != nil {
		return nil, fmt.Errorf("downloading %s: blob-API fallback returned wrong content: %w", file.Path, shaErr)
	}
	return io.NopCloser(bytes.NewReader(raw)), nil
}
