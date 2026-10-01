// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// Policy-file I/O. The policy endpoints need things the generated JSON client
// does not expose: the raw HuJSON body (comments intact), the ETag response
// header, and an If-Match request header on writes. Local backups live under
// the CLI state directory, one HuJSON file plus a JSON sidecar per snapshot,
// filed per credential and tailnet selector (see policyBackupScope).

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/client"
	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/config"
	"github.com/mvanhorn/printing-press-library/library/cloud/tailscale/internal/tsadmin"
)

type tsRawResponse struct {
	Status int
	Header http.Header
	Body   []byte
}

// tsRawRequest sends one authenticated request through the generated client's
// HTTP client (same config, auth, transport, and redirect guard) and returns
// the raw response. GETs retry on 429/502/503/504; writes never retry. Under
// the verify harness, writes short-circuit without dialing.
func tsRawRequest(ctx context.Context, c *client.Client, method, path string, body []byte, headers map[string]string) (*tsRawResponse, error) {
	if method != http.MethodGet && cliutil.IsVerifyEnv() && !cliutil.IsVerifyLiveHTTPEnv() {
		return &tsRawResponse{Status: http.StatusOK, Header: http.Header{}}, nil
	}
	auth, err := tsRequireAuth(c.Config)
	if err != nil {
		return nil, err
	}
	hc := *c.HTTPClient
	if hc.Timeout <= 0 {
		hc.Timeout = 60 * time.Second
	}
	target := strings.TrimSuffix(c.RequestBaseURL(), "/") + path
	attempts := 1
	if method == http.MethodGet {
		attempts = 4
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		var rdr io.Reader
		if body != nil {
			rdr = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, target, rdr)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", auth)
		req.Header.Set("User-Agent", "tailscale-pp-cli")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := hc.Do(req)
		if err != nil {
			lastErr = err
			if method != http.MethodGet {
				return nil, apiErr(fmt.Errorf("%s %s: %w", method, path, err))
			}
			if !tsSleep(ctx, tsBackoff(attempt, nil)) {
				break
			}
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		_ = resp.Body.Close() // body fully read above; a close error changes nothing
		if readErr != nil {
			return nil, apiErr(fmt.Errorf("reading %s %s: %w", method, path, readErr))
		}
		out := &tsRawResponse{Status: resp.StatusCode, Header: resp.Header, Body: data}
		retryable := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusGatewayTimeout
		if method == http.MethodGet && retryable && attempt < attempts-1 {
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			if !tsSleep(ctx, tsBackoff(attempt, resp)) {
				break
			}
			continue
		}
		return out, nil
	}
	return nil, apiErr(fmt.Errorf("%s %s: %v", method, path, lastErr))
}

// tsBackoff honors Retry-After (seconds or HTTP date) when the server sends
// it, and otherwise backs off exponentially from 500ms.
func tsBackoff(attempt int, resp *http.Response) time.Duration {
	if resp != nil && strings.TrimSpace(resp.Header.Get("Retry-After")) != "" {
		return cliutil.RetryAfter(resp)
	}
	return time.Duration(1<<attempt) * 500 * time.Millisecond
}

func tsSleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// tsHTTPError converts a non-2xx raw response into a typed CLI error.
func tsHTTPError(what string, r *tsRawResponse) error {
	var e struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(r.Body, &e)
	msg := strings.TrimSpace(cliutil.SanitizeErrorBody(e.Message))
	if msg == "" {
		msg = strings.TrimSpace(cliutil.SanitizeErrorBody(string(r.Body)))
	}
	err := fmt.Errorf("%s: HTTP %d: %s", what, r.Status, msg)
	switch {
	case r.Status == http.StatusUnauthorized || r.Status == http.StatusForbidden:
		return authErr(fmt.Errorf("%w\nhint: the token or OAuth client lacks the needed scope (policy_file for writes, policy_file:read for reads)", err))
	case r.Status == http.StatusNotFound:
		return notFoundErr(err)
	case r.Status == http.StatusPreconditionFailed:
		return apiErr(fmt.Errorf("%w\nthe policy file changed after it was read, so nothing was written; re-run to plan against the current version", err))
	case r.Status == http.StatusTooManyRequests:
		return rateLimitErr(err)
	default:
		return apiErr(err)
	}
}

// policySnapshot is a policy file plus its ETag.
type policySnapshot struct {
	Text []byte
	ETag string
}

func fetchPolicyHuJSON(ctx context.Context, c *client.Client, tailnet string) (policySnapshot, error) {
	r, err := tsRawRequest(ctx, c, http.MethodGet, tailnetPath(tailnet, "/acl"), nil, map[string]string{"Accept": "application/hujson"})
	if err != nil {
		return policySnapshot{}, err
	}
	if r.Status != http.StatusOK {
		return policySnapshot{}, tsHTTPError("reading policy file", r)
	}
	return policySnapshot{Text: r.Body, ETag: r.Header.Get("ETag")}, nil
}

// validatePolicy sends a candidate policy (standardized to JSON) to
// /acl/validate. Validation never changes the tailnet.
func validatePolicy(ctx context.Context, c *client.Client, tailnet string, policy []byte) (tsadmin.ValidateResult, error) {
	std, err := tsadmin.StandardizeJSON(policy)
	if err != nil {
		return tsadmin.ValidateResult{}, usageErr(fmt.Errorf("candidate policy is not valid HuJSON: %w", err))
	}
	r, err := tsRawRequest(ctx, c, http.MethodPost, tailnetPath(tailnet, "/acl/validate"), std, map[string]string{"Content-Type": "application/json", "Accept": "application/json"})
	if err != nil {
		return tsadmin.ValidateResult{}, err
	}
	if r.Status == http.StatusUnauthorized || r.Status == http.StatusForbidden || r.Status == http.StatusNotFound {
		return tsadmin.ValidateResult{}, tsHTTPError("validating policy file", r)
	}
	return tsadmin.InterpretValidate(r.Status, r.Body), nil
}

// writePolicy posts the HuJSON policy with If-Match so a concurrent change is
// never overwritten. It returns the new ETag.
func writePolicy(ctx context.Context, c *client.Client, tailnet string, policy []byte, etag string) (string, error) {
	if strings.TrimSpace(etag) == "" {
		return "", apiErr(errors.New("refusing to write the policy file without an ETag from the read; re-run the command"))
	}
	r, err := tsRawRequest(ctx, c, http.MethodPost, tailnetPath(tailnet, "/acl"), policy, map[string]string{
		"Content-Type": "application/hujson",
		"Accept":       "application/hujson",
		"If-Match":     etag,
	})
	if err != nil {
		return "", err
	}
	if r.Status != http.StatusOK {
		return "", tsHTTPError("writing policy file", r)
	}
	return r.Header.Get("ETag"), nil
}

// policyBackup describes one local snapshot.
type policyBackup struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Tailnet   string `json:"tailnet"`
	Scope     string `json:"scope"`
	ETag      string `json:"etag"`
	CreatedAt string `json:"created_at"`
	Reason    string `json:"reason"`
	SHA256    string `json:"sha256"`
	Bytes     int    `json:"bytes"`
}

var unsafeFileChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// backupIDRe matches IDs written by savePolicyBackup. Anything else in a
// sidecar is ignored, so a crafted sidecar cannot point outside the directory.
var backupIDRe = regexp.MustCompile(`^[0-9]{8}T[0-9]{6}-[0-9]{9}Z-[A-Za-z0-9_-]+$`)

// policyBackupScope names the store a backup is filed under: a fingerprint of
// the credential requests are sent with plus the tailnet selector. The default
// selector "-" means "the credential's own tailnet", which changes when the
// credential does, so the selector alone cannot key the store. A credential
// belongs to exactly one tailnet, so a backup filed under one credential is
// never listed or restored by ID under a credential for another tailnet. The
// scope is computed locally; listing and restoring never depend on another
// API call. After a key rotation older backups stay under the old scope and
// can be restored by file path.
func policyBackupScope(c *client.Client, tailnet string) (string, error) {
	fp := tsCredentialFingerprint(c.Config)
	if fp == "" {
		_, err := tsRequireAuth(c.Config)
		if err == nil {
			err = authErr(errors.New("no credential to file policy backups under"))
		}
		return "", err
	}
	return "credential-" + fp + "-" + tailnet, nil
}

// tsCredentialFingerprint is a short digest of the API base URL plus the
// credential requests are actually sent with, following the same precedence
// as Config.AuthHeader. A minted OAuth token changes every run, so OAuth uses
// the client ID and a digest of its secret instead; that also gives the same
// value before and after the token is minted. It never contains the
// credential itself.
func tsCredentialFingerprint(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	header := strings.TrimSpace(cfg.AuthHeader())
	base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	material := ""
	if header != "" {
		material = "auth-header\x00" + base + "\x00" + header
	}
	// OAuth applies when nothing stronger is configured and the header is
	// either not minted yet or was minted from the OAuth client.
	if strings.TrimSpace(cfg.AuthHeaderVal) == "" && strings.TrimSpace(cfg.TailscaleApiKey) == "" &&
		(header == "" || strings.HasPrefix(cfg.AuthSource, "oauth:")) {
		if id, secret := tsOAuthCredentials(); id != "" && secret != "" {
			sec := sha256.Sum256([]byte(secret))
			material = "oauth-client\x00" + base + "\x00" + id + "\x00" + hex.EncodeToString(sec[:])
		}
	}
	if material == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(material))
	return hex.EncodeToString(sum[:8])
}

// isPolicyFileRef reports whether a restore argument names a file to restore
// as given (no backup scope needed to load it). "latest" and anything shaped
// like a backup ID always mean a managed backup, even if a file with that
// name exists in the working directory; pass ./name to restore such a file.
func isPolicyFileRef(ref string) bool {
	ref = strings.TrimSpace(ref)
	id := strings.TrimSuffix(strings.TrimSuffix(ref, ".json"), ".hujson")
	if strings.EqualFold(id, "latest") || backupIDRe.MatchString(id) {
		return false
	}
	st, err := os.Stat(ref)
	return err == nil && !st.IsDir()
}

// policyBackupDir returns the directory for one backup scope. The name is a
// readable slug plus a short hash of the exact scope, so distinct scopes never
// share a directory and no scope can produce "." or "..".
func policyBackupDir(scope string) (string, error) {
	base, err := cliutil.StateDir()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(scope) == "" {
		return "", errors.New("policy backup scope is empty")
	}
	slug := strings.Trim(unsafeFileChars.ReplaceAllString(scope, "_"), "._")
	if len(slug) > 48 {
		slug = slug[:48]
	}
	if slug == "" {
		slug = "scope"
	}
	sum := sha256.Sum256([]byte(scope))
	return filepath.Join(base, "policy-backups", slug+"-"+hex.EncodeToString(sum[:4])), nil
}

// isRegularFile reports whether path is a regular file, not following
// symlinks, so a planted link cannot pull an outside file into a restore.
func isRegularFile(path string) bool {
	st, err := os.Lstat(path)
	return err == nil && st.Mode().IsRegular()
}

func withinDir(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// savePolicyBackup writes text plus a sidecar describing it. Files are 0600
// in a 0700 directory because the policy file names users and devices.
func savePolicyBackup(scope, tailnet string, snap policySnapshot, reason string) (policyBackup, error) {
	dir, err := policyBackupDir(scope)
	if err != nil {
		return policyBackup{}, err
	}
	now := time.Now().UTC()
	sum := sha256.Sum256(snap.Text)
	id := fmt.Sprintf("%sT%s-%09dZ-%s", now.Format("20060102"), now.Format("150405"), now.Nanosecond(), strings.ReplaceAll(unsafeFileChars.ReplaceAllString(reason, "-"), ".", "-"))
	b := policyBackup{
		ID:        id,
		Path:      filepath.Join(dir, id+".hujson"),
		Tailnet:   tailnet,
		Scope:     scope,
		ETag:      snap.ETag,
		CreatedAt: now.Format(time.RFC3339),
		Reason:    reason,
		SHA256:    hex.EncodeToString(sum[:]),
		Bytes:     len(snap.Text),
	}
	if err := cliutil.AtomicWritePrivateFile(b.Path, snap.Text, 0o600, 0o700); err != nil {
		return policyBackup{}, fmt.Errorf("writing policy backup: %w", err)
	}
	meta, _ := json.MarshalIndent(b, "", "  ")
	if err := cliutil.AtomicWritePrivateFile(filepath.Join(dir, id+".json"), meta, 0o600, 0o700); err != nil {
		return policyBackup{}, fmt.Errorf("writing policy backup metadata: %w", err)
	}
	return b, nil
}

// listPolicyBackups returns the backups filed under scope, newest first. A
// sidecar recording any other scope is skipped even if it sits in this
// directory.
func listPolicyBackups(scope string) ([]policyBackup, error) {
	dir, err := policyBackupDir(scope)
	if err != nil {
		return nil, err
	}
	out := make([]policyBackup, 0)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.Type().IsRegular() || !strings.HasSuffix(e.Name(), ".json") {
			continue // directories and symlinks are never managed backups
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name())) // #nosec G304 -- dir is the CLI-owned backup directory; the name comes from its own ReadDir listing, filtered to regular .json files
		if err != nil {
			continue
		}
		var b policyBackup
		if json.Unmarshal(raw, &b) != nil || !backupIDRe.MatchString(b.ID) || e.Name() != b.ID+".json" || b.Scope != scope {
			continue
		}
		b.Path = filepath.Join(dir, b.ID+".hujson")
		if !withinDir(dir, b.Path) || !isRegularFile(b.Path) {
			continue
		}
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

// loadPolicyBackup resolves a backup by ID (within scope) or by file path. A
// path is the caller's explicit choice and is not scope-checked.
func loadPolicyBackup(scope, tailnet, ref string) (policyBackup, []byte, error) {
	ref = strings.TrimSpace(ref)
	if isPolicyFileRef(ref) {
		text, err := os.ReadFile(filepath.Clean(ref))
		if err != nil {
			return policyBackup{}, nil, err
		}
		return policyBackup{ID: filepath.Base(ref), Path: ref, Tailnet: tailnet, Bytes: len(text)}, text, nil
	}
	backups, err := listPolicyBackups(scope)
	if err != nil {
		return policyBackup{}, nil, err
	}
	id := strings.TrimSuffix(strings.TrimSuffix(ref, ".json"), ".hujson")
	if strings.EqualFold(id, "latest") {
		if len(backups) == 0 {
			return policyBackup{}, nil, notFoundErr(fmt.Errorf("no policy backups yet for tailnet %q; they are taken before every policy write", tailnet))
		}
		id = backups[0].ID
	}
	if !backupIDRe.MatchString(id) {
		return policyBackup{}, nil, notFoundErr(fmt.Errorf("no policy backup %q for tailnet %q; pass an ID from 'tailscale-pp-cli policy restore --list' or a path to a HuJSON file", ref, tailnet))
	}
	for _, b := range backups {
		if b.ID == id {
			if !isRegularFile(b.Path) {
				return policyBackup{}, nil, notFoundErr(fmt.Errorf("policy backup %q is not a regular file", ref))
			}
			text, err := os.ReadFile(filepath.Clean(b.Path))
			if err != nil {
				return policyBackup{}, nil, err
			}
			return b, text, nil
		}
	}
	return policyBackup{}, nil, notFoundErr(fmt.Errorf("no policy backup %q for tailnet %q; run 'tailscale-pp-cli policy restore --list'", ref, tailnet))
}

// policyDiffView is the diff summary shared by add-entry and restore.
type policyDiffView struct {
	Added   []tsadmin.DiffLine `json:"added"`
	Removed []tsadmin.DiffLine `json:"removed"`
	Text    string             `json:"unified"`
}

func buildPolicyDiff(before, after []byte) policyDiffView {
	lines := tsadmin.LineDiff(string(before), string(after))
	added, removed := tsadmin.DiffChanges(lines)
	return policyDiffView{Added: added, Removed: removed, Text: tsadmin.UnifiedDiff(lines, 2)}
}
