// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored business logic for here.now novel ("transcendence") features.
// This file is NOT generated and must survive a regeneration merge. It holds
// the shared logic behind: publish dir, publish resume, claims (+ expiring +
// redeem), drives sync, drives diff, and usage. The thin newNovel*Cmd cobra
// wrappers in the per-feature files call into the functions here.
package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/here-now/internal/client"
	"github.com/mvanhorn/printing-press-library/library/cloud/here-now/internal/store"
)

// anonClaimTTL is the lifetime here.now grants an anonymous publish before it
// expires. Used to compute the locally-recorded expiry when the server does
// not return one.
const anonClaimTTL = 24 * time.Hour

// inlineTextMaxBytes is the size ceiling for inlining a file's content into the
// publish request rather than uploading it via a presigned URL. Files at or
// under this size whose content sniffs as text are inlined.
const inlineTextMaxBytes = 64 * 1024

// Free-plan ceilings used by the usage meter.
const (
	freePlanSiteLimit        = 500
	freePlanDriveBytesLimit  = 10 * 1024 * 1024 * 1024 // 10 GiB
	freePlanDriveLimit       = 1
	freePlanPublishesPerHour = 60
)

// resolveDBPath returns the effective SQLite path: the explicit --db value if
// set, otherwise the canonical default for this CLI.
func resolveDBPath(explicit string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	return defaultDBPath("here-now-pp-cli")
}

// openHereNowStore opens (creating if needed) the local store at the resolved
// path and ensures the novel-feature tables exist. The caller owns Close.
func openHereNowStore(ctx context.Context, explicit string) (*store.Store, error) {
	path := resolveDBPath(explicit)
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create store directory: %w", err)
		}
	}
	s, err := store.OpenWithContext(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("open local store at %s: %w", path, err)
	}
	if err := s.EnsureHereNowTables(); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

// localFile is one file discovered by walking a publish/sync source directory.
type localFile struct {
	// RelPath is the forward-slash path relative to the walk root.
	RelPath string
	// AbsPath is the absolute filesystem path.
	AbsPath string
	Size    int64
	// ContentType is the sniffed MIME type.
	ContentType string
}

// walkDir collects every regular file under root, recording each one's
// forward-slash relative path, size, and sniffed content type. Symlinks and
// directories are skipped. The returned slice is sorted by RelPath for stable
// output.
func walkDir(root string) ([]localFile, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}
	var files []localFile
	err = filepath.Walk(root, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if fi.IsDir() || !fi.Mode().IsRegular() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		ct, ctErr := sniffContentType(path, fi.Size())
		if ctErr != nil {
			return ctErr
		}
		files = append(files, localFile{
			RelPath:     rel,
			AbsPath:     path,
			Size:        fi.Size(),
			ContentType: ct,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].RelPath < files[j].RelPath })
	return files, nil
}

// sniffContentType detects a file's MIME type from its first 512 bytes via
// http.DetectContentType, falling back to an extension-based guess. Used to
// decide inline-vs-upload in publish dir and to set Content-Type on uploads.
func sniffContentType(path string, size int64) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	head := make([]byte, 512)
	n, err := f.Read(head)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	ct := http.DetectContentType(head[:n])
	if ext := strings.ToLower(filepath.Ext(path)); ext != "" {
		// http.DetectContentType is byte-oriented and misclassifies common
		// web assets (CSS as text/plain, JS as text/plain). Prefer the
		// extension mapping for those so published sites serve correctly.
		switch ext {
		case ".css":
			ct = "text/css; charset=utf-8"
		case ".js", ".mjs":
			ct = "text/javascript; charset=utf-8"
		case ".json":
			ct = "application/json"
		case ".svg":
			ct = "image/svg+xml"
		case ".html", ".htm":
			ct = "text/html; charset=utf-8"
		}
	}
	return ct, nil
}

// isInlineable reports whether a file should be embedded directly in the
// publish request: it must sniff as text and be at or under the inline size
// ceiling.
func (f localFile) isInlineable() bool {
	return f.Size <= inlineTextMaxBytes && strings.HasPrefix(f.ContentType, "text/")
}

// sha256File returns the lowercase hex sha256 digest of a file's contents.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// --- publish request/response shapes (subset of the OpenAPI schema) ---

// publishFileReq is one entry in the publish request's files[] array. The
// here.now API takes a descriptor for EVERY file (path + size, plus optional
// contentType/hash) and decides server-side which ones need a presigned upload
// — there is no inline-content field on this endpoint. size is required.
type publishFileReq struct {
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType,omitempty"`
	Hash        string `json:"hash,omitempty"`
}

type publishCreateReq struct {
	Files      []publishFileReq `json:"files"`
	ClaimToken string           `json:"claimToken,omitempty"`
	SpaMode    bool             `json:"spaMode,omitempty"`
	Password   string           `json:"password,omitempty"`
}

type uploadTarget struct {
	Path    string            `json:"path"`
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

type publishUpload struct {
	VersionID   string         `json:"versionId"`
	Uploads     []uploadTarget `json:"uploads"`
	Skipped     []string       `json:"skipped"`
	FinalizeURL string         `json:"finalizeUrl"`
}

type publishCreateResp struct {
	Slug             string        `json:"slug"`
	SiteURL          string        `json:"siteUrl"`
	Status           string        `json:"status"`
	IsLive           bool          `json:"isLive"`
	RequiresFinalize bool          `json:"requiresFinalize"`
	Upload           publishUpload `json:"upload"`
	ClaimToken       string        `json:"claimToken"`
	ClaimURL         string        `json:"claimUrl"`
	ExpiresAt        string        `json:"expiresAt"`
	Anonymous        bool          `json:"anonymous"`
}

type finalizeReq struct {
	VersionID string `json:"versionId"`
}

// --- publish dir ---

// publishDirOptions configures a publish-dir run.
type publishDirOptions struct {
	Dir      string
	Anon     bool
	Slug     string
	Password string
	SPA      bool
}

// publishPlan is the classification result for a directory: the per-file
// descriptors for the publish request, a source-file lookup for the presigned
// PUTs the server requests back, the byte total, and an informational split of
// how many files the local heuristic considers small-text ("inline-eligible")
// vs binary/large — surfaced only in the --dry-run preview. The here.now API
// itself decides which files need an upload, so every file is sent as a
// descriptor regardless of the local classification.
type publishPlan struct {
	Files          []publishFileReq
	src            map[string]localFile // RelPath -> source file, for the live PUT
	TotalSize      int64
	InlineEligible int // small-text files (informational, dry-run preview only)
	BinaryOrLarge  int
}

// classifyForPublish walks the directory and builds a size-bearing descriptor
// for every file (path, size, contentType, sha256), recording a source-file
// lookup for the presigned uploads the server requests. It also tallies an
// informational small-text-vs-binary split for the dry-run preview.
func classifyForPublish(dir string) (*publishPlan, error) {
	files, err := walkDir(dir)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no files found under %s", dir)
	}
	plan := &publishPlan{src: map[string]localFile{}}
	for _, f := range files {
		plan.TotalSize += f.Size
		if f.isInlineable() {
			plan.InlineEligible++
		} else {
			plan.BinaryOrLarge++
		}
		hash, hashErr := sha256File(f.AbsPath)
		if hashErr != nil {
			return nil, hashErr
		}
		plan.Files = append(plan.Files, publishFileReq{
			Path:        f.RelPath,
			Size:        f.Size,
			ContentType: f.ContentType,
			Hash:        hash,
		})
		plan.src[f.RelPath] = f
	}
	return plan, nil
}

// uploadBytes PUTs a file's contents to a presigned target URL using a plain
// net/http client (the target is external object storage, not the here.now
// API). It sets the headers the target requires plus Content-Type, and expects
// a 200 or 204.
func uploadBytes(ctx context.Context, target uploadTarget, src localFile, timeout time.Duration) error {
	data, err := os.ReadFile(src.AbsPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", src.AbsPath, err)
	}
	method := target.Method
	if method == "" {
		method = http.MethodPut
	}
	req, err := http.NewRequestWithContext(ctx, method, target.URL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("build upload request for %s: %w", src.RelPath, err)
	}
	for k, v := range target.Headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", src.ContentType)
	}
	hc := &http.Client{Timeout: timeout}
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("upload %s: %w", src.RelPath, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("upload %s: unexpected status %d: %s", src.RelPath, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// runPublishDir executes the full publish-dir flow against the live API:
// classify -> POST /api/v1/publish -> PUT each upload target -> POST finalize.
// It records progress into the local store (publish_state, and claim_vault for
// anonymous publishes). On a finalize failure after uploads succeeded it
// records finalized=0 and returns an error mentioning `publish resume`.
func runPublishDir(ctx context.Context, c *client.Client, db *store.Store, opts publishDirOptions, timeout time.Duration) (*publishCreateResp, error) {
	plan, err := classifyForPublish(opts.Dir)
	if err != nil {
		return nil, err
	}

	// --anon must publish WITHOUT credentials so the server returns a claim
	// token and a 24h-expiring anonymous site. The here.now publish endpoint's
	// security is [{},{"bearerAuth":[]}] (auth optional). The client always
	// sends the configured Bearer token, which the server treats as an
	// authenticated publish (no claim token). Blank the client's auth material
	// for this run so the request goes out unauthenticated.
	if opts.Anon && c.Config != nil {
		c.Config.AuthHeaderVal = ""
		c.Config.AccessToken = ""
		c.Config.HerenowApiKey = ""
	}

	body := publishCreateReq{
		Files:    plan.Files,
		SpaMode:  opts.SPA,
		Password: opts.Password,
	}
	// An anonymous re-publish can reuse a stored claim token to keep the same
	// slug; not required, so this is best-effort and only when --slug is given.
	if opts.Anon && opts.Slug != "" {
		if rec, gerr := db.GetClaim(opts.Slug); gerr == nil && rec != nil && rec.ClaimToken != "" {
			body.ClaimToken = rec.ClaimToken
		}
	}

	raw, status, err := c.Post(ctx, "/api/v1/publish", body)
	if err != nil {
		return nil, fmt.Errorf("publish create failed (HTTP %d): %w", status, err)
	}
	var resp publishCreateResp
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode publish response: %w", err)
	}
	if resp.Slug == "" {
		return nil, fmt.Errorf("publish response missing slug: %s", string(raw))
	}

	// Persist the anonymous claim token IMMEDIATELY, before uploads, state
	// writes, or finalize. The create-publish response returns the claim token
	// exactly once and it is unrecoverable if lost; saving it first protects it
	// against any downstream failure (disk full on SavePublishState, finalize
	// error). The site already exists server-side at this point and can be made
	// permanent later via `claims redeem`.
	if opts.Anon && resp.ClaimToken != "" {
		expiresAt := resp.ExpiresAt
		if expiresAt == "" {
			expiresAt = time.Now().UTC().Add(anonClaimTTL).Format(time.RFC3339)
		}
		if cerr := db.SaveClaim(store.ClaimRecord{
			Slug:        resp.Slug,
			ClaimToken:  resp.ClaimToken,
			URL:         resp.SiteURL,
			PublishedAt: time.Now().UTC().Format(time.RFC3339),
			ExpiresAt:   expiresAt,
			Claimed:     false,
		}); cerr != nil {
			return &resp, cerr
		}
	}

	// PUT every upload target the server requested before finalizing.
	for _, target := range resp.Upload.Uploads {
		src, ok := plan.src[target.Path]
		if !ok {
			return nil, fmt.Errorf("server requested upload for %q but it was not in the local plan", target.Path)
		}
		if uerr := uploadBytes(ctx, target, src, timeout); uerr != nil {
			return nil, uerr
		}
	}

	uploadsJSON, err := json.Marshal(resp.Upload.Uploads)
	if err != nil {
		return nil, fmt.Errorf("record upload targets for %q: %w", resp.Slug, err)
	}
	now := time.Now().UTC().Format(time.RFC3339)

	// Persist the pre-finalize state so `publish resume` can recover if
	// finalize fails below.
	preState := store.PublishStateRecord{
		Slug:        resp.Slug,
		VersionID:   resp.Upload.VersionID,
		Dir:         opts.Dir,
		UploadsJSON: string(uploadsJSON),
		Finalized:   false,
		CreatedAt:   now,
	}
	if serr := db.SavePublishState(preState); serr != nil {
		return nil, serr
	}

	if resp.RequiresFinalize {
		if ferr := finalizePublish(ctx, c, resp.Slug, resp.Upload.VersionID); ferr != nil {
			return &resp, fmt.Errorf("uploads succeeded but finalize failed for %q; run 'here-now-pp-cli publish resume %s' to complete: %w", resp.Slug, resp.Slug, ferr)
		}
	}
	if merr := db.MarkFinalized(resp.Slug); merr != nil {
		return &resp, merr
	}

	return &resp, nil
}

// finalizePublish POSTs the finalize request for a slug/version.
func finalizePublish(ctx context.Context, c *client.Client, slug, versionID string) error {
	path := fmt.Sprintf("/api/v1/publish/%s/finalize", slug)
	_, status, err := c.Post(ctx, path, finalizeReq{VersionID: versionID})
	if err != nil {
		return fmt.Errorf("finalize %q (HTTP %d): %w", slug, status, err)
	}
	return nil
}

// --- claims ---

// claimView is the display projection of a claim_vault row with computed
// time-remaining.
type claimView struct {
	Slug        string `json:"slug"`
	URL         string `json:"url"`
	PublishedAt string `json:"published_at"`
	ExpiresAt   string `json:"expires_at"`
	Remaining   string `json:"remaining"`
	Claimed     bool   `json:"claimed"`
}

// toClaimView converts a stored record into its display projection, computing
// the remaining time until expiry relative to now.
func toClaimView(rec store.ClaimRecord, now time.Time) claimView {
	return claimView{
		Slug:        rec.Slug,
		URL:         rec.URL,
		PublishedAt: rec.PublishedAt,
		ExpiresAt:   rec.ExpiresAt,
		Remaining:   remainingUntil(rec.ExpiresAt, now),
		Claimed:     rec.Claimed,
	}
}

// remainingUntil renders the human-readable duration until an RFC3339 expiry,
// or "expired" / "claimed-permanent" style markers. Unparseable timestamps
// render as "unknown".
func remainingUntil(expiresAt string, now time.Time) string {
	if strings.TrimSpace(expiresAt) == "" {
		return "permanent"
	}
	t, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return "unknown"
	}
	d := t.Sub(now)
	if d <= 0 {
		return "expired"
	}
	return roundDuration(d).String()
}

// roundDuration trims a duration to a readable granularity (minutes for short,
// hours otherwise).
func roundDuration(d time.Duration) time.Duration {
	if d < time.Hour {
		return d.Round(time.Minute)
	}
	return d.Round(time.Hour)
}

// listClaimViews returns every vault row as a display projection.
func listClaimViews(db *store.Store, now time.Time) ([]claimView, error) {
	recs, err := db.ListClaims()
	if err != nil {
		return nil, err
	}
	out := make([]claimView, 0, len(recs))
	for _, r := range recs {
		out = append(out, toClaimView(r, now))
	}
	return out, nil
}

// listExpiringClaims returns unclaimed vault rows expiring at or before
// now+within, soonest first.
func listExpiringClaims(db *store.Store, within time.Duration, now time.Time) ([]claimView, error) {
	recs, err := db.ListClaims()
	if err != nil {
		return nil, err
	}
	cutoff := now.Add(within)
	var out []claimView
	for _, r := range recs {
		if r.Claimed {
			continue
		}
		t, perr := time.Parse(time.RFC3339, r.ExpiresAt)
		if perr != nil {
			continue
		}
		if !t.After(cutoff) {
			out = append(out, toClaimView(r, now))
		}
	}
	return out, nil
}

// claimSiteReq is the POST body for claiming an anonymous site.
type claimSiteReq struct {
	ClaimToken string `json:"claimToken"`
}

// redeemClaim claims an anonymous site by POSTing its stored token to
// /api/v1/publish/{slug}/claim, then marks the local row claimed on success.
func redeemClaim(ctx context.Context, c *client.Client, db *store.Store, slug string) (json.RawMessage, error) {
	rec, err := db.GetClaim(slug)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, fmt.Errorf("no claim recorded for slug %q; only anonymous publishes made by this CLI are in the vault", slug)
	}
	if rec.ClaimToken == "" {
		return nil, fmt.Errorf("claim record for %q has no claim token; cannot redeem", slug)
	}
	path := fmt.Sprintf("/api/v1/publish/%s/claim", slug)
	raw, status, err := c.Post(ctx, path, claimSiteReq{ClaimToken: rec.ClaimToken})
	if err != nil {
		return nil, fmt.Errorf("claim %q (HTTP %d): %w", slug, status, err)
	}
	if merr := db.MarkClaimed(slug); merr != nil {
		return raw, merr
	}
	return raw, nil
}

// --- drive sync / diff ---

// driveFileMeta is the subset of a DriveFile we need for diffing.
type driveFileMeta struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type driveFileListResp struct {
	Files      []driveFileMeta `json:"files"`
	NextCursor *string         `json:"nextCursor"`
}

// driveDiff is the result of comparing a local directory against a Drive's
// current files: which to upload (new or changed), which are unchanged, and
// which exist only remotely (delete candidates).
type driveDiff struct {
	Upload    []localFile
	Unchanged []localFile
	Delete    []string
}

// listDriveFiles fetches the full file list for a Drive, following cursor
// pagination, and returns a path->meta map. It bypasses the response
// cache (GetNoCache) so drive sync/diff always compute against the
// live remote state; a stale cached list would let sync skip files
// that changed externally and report success while leaving the Drive
// out of sync.
func listDriveFiles(ctx context.Context, c *client.Client, driveID string) (map[string]driveFileMeta, error) {
	out := map[string]driveFileMeta{}
	cursor := ""
	path := fmt.Sprintf("/api/v1/drives/%s/files", driveID)
	for {
		params := map[string]string{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, err := c.GetNoCache(ctx, path, params)
		if err != nil {
			return nil, fmt.Errorf("list drive files: %w", err)
		}
		var resp driveFileListResp
		if uerr := json.Unmarshal(raw, &resp); uerr != nil {
			return nil, fmt.Errorf("decode drive file list: %w", uerr)
		}
		for _, f := range resp.Files {
			out[f.Path] = f
		}
		if resp.NextCursor == nil || *resp.NextCursor == "" {
			break
		}
		cursor = *resp.NextCursor
	}
	return out, nil
}

// computeDriveDiff walks the local directory, hashes each file, and compares
// against the remote file map. includeDeletes controls whether remote-only
// files are reported as delete candidates.
func computeDriveDiff(dir string, remote map[string]driveFileMeta, includeDeletes bool) (*driveDiff, error) {
	files, err := walkDir(dir)
	if err != nil {
		return nil, err
	}
	diff := &driveDiff{}
	localSeen := map[string]bool{}
	for _, f := range files {
		localSeen[f.RelPath] = true
		hash, herr := sha256File(f.AbsPath)
		if herr != nil {
			return nil, herr
		}
		rf, ok := remote[f.RelPath]
		if !ok || normalizeSHA(rf.SHA256) != hash {
			diff.Upload = append(diff.Upload, f)
		} else {
			diff.Unchanged = append(diff.Unchanged, f)
		}
	}
	if includeDeletes {
		for path := range remote {
			if !localSeen[path] {
				diff.Delete = append(diff.Delete, path)
			}
		}
		sort.Strings(diff.Delete)
	}
	return diff, nil
}

// normalizeSHA strips an optional "sha256:" prefix and lowercases the digest so
// remote and locally-computed hashes compare equal regardless of formatting.
func normalizeSHA(s string) string {
	s = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(s)), "sha256:")
	return s
}

// --- drive upload (presigned) shapes ---

type driveUploadCreateReq struct {
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType,omitempty"`
	SHA256      string `json:"sha256,omitempty"`
}

// driveUploadCreateResp decodes the presigned-upload response. The OpenAPI
// spec names the presigned URL `url`, but the LIVE API returns it as
// `uploadUrl` (with the spec name absent). We decode both and prefer
// uploadUrl, falling back to url, so the upload PUT always gets a real target.
type driveUploadCreateResp struct {
	UploadID  string            `json:"uploadId"`
	Method    string            `json:"method"`
	URL       string            `json:"url"`
	UploadURL string            `json:"uploadUrl"`
	Headers   map[string]string `json:"headers"`
}

// presignedURL returns the effective presigned upload target, preferring the
// live API's uploadUrl over the spec-named url.
func (r driveUploadCreateResp) presignedURL() string {
	if r.UploadURL != "" {
		return r.UploadURL
	}
	return r.URL
}

type driveFinalizeReq struct {
	UploadID string `json:"uploadId"`
	Path     string `json:"path"`
}

// uploadFileToDrive runs the three-step drive upload for one file: create
// upload (presigned URL) -> PUT bytes -> finalize.
func uploadFileToDrive(ctx context.Context, c *client.Client, driveID string, f localFile, timeout time.Duration) error {
	hash, err := sha256File(f.AbsPath)
	if err != nil {
		return err
	}
	createPath := fmt.Sprintf("/api/v1/drives/%s/files/uploads", driveID)
	raw, status, err := c.Post(ctx, createPath, driveUploadCreateReq{
		Path:        f.RelPath,
		Size:        f.Size,
		ContentType: f.ContentType,
		SHA256:      hash,
	})
	if err != nil {
		return fmt.Errorf("create drive upload for %s (HTTP %d): %w", f.RelPath, status, err)
	}
	var up driveUploadCreateResp
	if uerr := json.Unmarshal(raw, &up); uerr != nil {
		return fmt.Errorf("decode drive upload response: %w", uerr)
	}
	target := uploadTarget{Path: f.RelPath, Method: up.Method, URL: up.presignedURL(), Headers: up.Headers}
	if perr := uploadBytes(ctx, target, f, timeout); perr != nil {
		return perr
	}
	finPath := fmt.Sprintf("/api/v1/drives/%s/files/finalize", driveID)
	if _, fstatus, ferr := c.Post(ctx, finPath, driveFinalizeReq{UploadID: up.UploadID, Path: f.RelPath}); ferr != nil {
		return fmt.Errorf("finalize drive upload for %s (HTTP %d): %w", f.RelPath, fstatus, ferr)
	}
	return nil
}

// deleteDriveFile removes a remote file from a Drive by path.
func deleteDriveFile(ctx context.Context, c *client.Client, driveID, path string) error {
	delPath := fmt.Sprintf("/api/v1/drives/%s/files/%s", driveID, escapeDrivePath(path))
	if _, status, err := c.Delete(ctx, delPath); err != nil {
		return fmt.Errorf("delete drive file %s (HTTP %d): %w", path, status, err)
	}
	return nil
}

// escapeDrivePath percent-escapes each path segment so a Drive path with
// subdirectories maps to a safe URL path without escaping the slashes between
// segments.
func escapeDrivePath(p string) string {
	parts := strings.Split(p, "/")
	for i, seg := range parts {
		parts[i] = url.PathEscape(seg)
	}
	return strings.Join(parts, "/")
}

// driveSyncResult counts the outcome of a drives sync run.
type driveSyncResult struct {
	Uploaded int `json:"uploaded"`
	Skipped  int `json:"skipped"`
	Deleted  int `json:"deleted"`
}

// runDriveSync uploads changed/new files and (when del is set) removes
// remote-only files, returning the per-category counts.
func runDriveSync(ctx context.Context, c *client.Client, driveID, dir string, del bool, timeout time.Duration) (*driveSyncResult, error) {
	remote, err := listDriveFiles(ctx, c, driveID)
	if err != nil {
		return nil, err
	}
	diff, err := computeDriveDiff(dir, remote, del)
	if err != nil {
		return nil, err
	}
	res := &driveSyncResult{Skipped: len(diff.Unchanged)}
	for _, f := range diff.Upload {
		if uerr := uploadFileToDrive(ctx, c, driveID, f, timeout); uerr != nil {
			return res, uerr
		}
		res.Uploaded++
	}
	if del {
		for _, p := range diff.Delete {
			if derr := deleteDriveFile(ctx, c, driveID, p); derr != nil {
				return res, derr
			}
			res.Deleted++
		}
	}
	return res, nil
}

// --- usage meter ---

// usageMetric is one free-plan dimension's used/limit reading.
type usageMetric struct {
	Used      int64  `json:"used"`
	Limit     int64  `json:"limit"`
	Pct       int    `json:"pct"`
	OverLimit bool   `json:"over_limit"`
	Note      string `json:"note,omitempty"`
}

// usageReport is the full local free-plan rollup emitted by `usage`.
type usageReport struct {
	Sites           usageMetric `json:"sites"`
	DriveBytes      usageMetric `json:"drive_bytes"`
	Drives          usageMetric `json:"drives"`
	PublishesLast1h usageMetric `json:"publishes_last_1h"`
}

// newMetric builds a usageMetric, computing percentage and over-limit flag.
func newMetric(used, limit int64, note string) usageMetric {
	pct := 0
	if limit > 0 {
		pct = int(used * 100 / limit)
	}
	return usageMetric{
		Used:      used,
		Limit:     limit,
		Pct:       pct,
		OverLimit: limit > 0 && used > limit,
		Note:      note,
	}
}

// buildUsageReport assembles the local free-plan meter. Site count and recent
// publish cadence come from the local store; drive count and byte totals come
// from the live API when a client is available. A nil client (no auth) yields
// drive metrics with an explanatory note rather than an error.
func buildUsageReport(ctx context.Context, c *client.Client, db *store.Store, now time.Time) (*usageReport, error) {
	siteCount, err := db.Count("publishes")
	if err != nil {
		return nil, fmt.Errorf("count local sites: %w", err)
	}
	recentPublishes, err := db.RecentPublishCount(now.Add(-time.Hour).Format(time.RFC3339))
	if err != nil {
		return nil, err
	}

	report := &usageReport{
		Sites:           newMetric(int64(siteCount), freePlanSiteLimit, "from local mirror"),
		PublishesLast1h: newMetric(int64(recentPublishes), freePlanPublishesPerHour, "from local publish log"),
	}

	driveCount, driveBytes, dnote := collectDriveUsage(ctx, c, db)
	report.Drives = newMetric(driveCount, freePlanDriveLimit, dnote)
	report.DriveBytes = newMetric(driveBytes, freePlanDriveBytesLimit, dnote)
	return report, nil
}

// collectDriveUsage returns (driveCount, totalBytes, note). It queries the live
// API for the drive list and per-drive file sizes; on any auth/network error it
// falls back to the local drives mirror and annotates the note. Both the drive
// list and the per-drive file lists are read uncached (GetNoCache) so the usage
// meter reflects current state right after a sync rather than a cached snapshot.
func collectDriveUsage(ctx context.Context, c *client.Client, db *store.Store) (int64, int64, string) {
	if c == nil {
		count, _ := db.Count("drives")
		return int64(count), 0, "drive bytes need auth; drive count from local mirror"
	}
	raw, err := c.GetNoCache(ctx, "/api/v1/drives", nil)
	if err != nil {
		count, _ := db.Count("drives")
		return int64(count), 0, "live drive stats unavailable (auth required); drive count from local mirror"
	}
	var listResp struct {
		Drives []struct {
			ID string `json:"id"`
		} `json:"drives"`
	}
	if uerr := json.Unmarshal(raw, &listResp); uerr != nil {
		count, _ := db.Count("drives")
		return int64(count), 0, "could not parse live drive list; drive count from local mirror"
	}
	var totalBytes int64
	for _, d := range listResp.Drives {
		files, ferr := listDriveFiles(ctx, c, d.ID)
		if ferr != nil {
			continue
		}
		for _, f := range files {
			totalBytes += f.Size
		}
	}
	return int64(len(listResp.Drives)), totalBytes, "live"
}

// --- sites stale ---

// staleSite is a site from the local mirror not updated within the threshold.
type staleSite struct {
	Slug      string `json:"slug"`
	SiteURL   string `json:"siteUrl"`
	UpdatedAt string `json:"updatedAt"`
	AgeDays   int    `json:"age_days"`
}

// listStaleSites reads the local publishes mirror and returns sites whose
// updatedAt is older than `days` days, oldest first.
func listStaleSites(db *store.Store, days int, now time.Time) ([]staleSite, error) {
	rows, err := db.List("publishes", 100000)
	if err != nil {
		return nil, fmt.Errorf("read local sites: %w", err)
	}
	threshold := now.Add(-time.Duration(days) * 24 * time.Hour)
	var out []staleSite
	for _, raw := range rows {
		var rec struct {
			Slug      string `json:"slug"`
			SiteURL   string `json:"siteUrl"`
			UpdatedAt string `json:"updatedAt"`
		}
		if uerr := json.Unmarshal(raw, &rec); uerr != nil {
			continue
		}
		t, perr := time.Parse(time.RFC3339, rec.UpdatedAt)
		if perr != nil {
			continue
		}
		if t.Before(threshold) {
			out = append(out, staleSite{
				Slug:      rec.Slug,
				SiteURL:   rec.SiteURL,
				UpdatedAt: rec.UpdatedAt,
				AgeDays:   int(now.Sub(t).Hours() / 24),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AgeDays > out[j].AgeDays })
	return out, nil
}
