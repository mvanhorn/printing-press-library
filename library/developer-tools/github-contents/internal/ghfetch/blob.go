// Copyright 2026 Rick van de Laar and contributors. Licensed under Apache-2.0. See LICENSE.

package ghfetch

import (
	"bytes"
	"context"
	"crypto/sha1" // #nosec G505 -- git's own blob-object hashing algorithm, not used for security
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
)

// gitLFSPointerPrefix is the literal marker byte string every Git LFS
// pointer file begins with (see
// https://github.com/git-lfs/git-lfs/blob/main/docs/spec.md).
const gitLFSPointerPrefix = "version https://git-lfs.github.com/spec/v1"

// LFSMaxPointerSize is the Git LFS spec's maximum size of a pointer file
// (https://github.com/git-lfs/git-lfs/blob/main/docs/spec.md: pointer
// files are small key-value text blobs, bounded at 1024 bytes). It is the
// single threshold both plan's LFS probe and the downloader's LFS handling
// key on: a tree entry at or under this size MIGHT be an LFS pointer;
// anything larger cannot be.
const LFSMaxPointerSize = 1024

// GitBlobSHA computes the git blob object SHA-1 for a stream of size bytes,
// matching `git hash-object`'s algorithm: sha1("blob {size}\x00" + content).
// size must be the exact byte length of r's remaining content; a mismatch
// produces a SHA that will not match git's own value for the same bytes.
func GitBlobSHA(r io.Reader, size int64) (string, error) {
	h := sha1.New() // #nosec G401 -- git blob SHAs are SHA-1 by protocol; integrity check against GitHub's own values, not a security primitive
	if _, err := fmt.Fprintf(h, "blob %d\x00", size); err != nil {
		return "", err
	}
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// GitBlobSHAFile computes the git blob SHA-1 of the file at path, using its
// on-disk size. Used to compare a local file against a remote TreeFile.SHA
// without re-downloading.
func GitBlobSHAFile(path string) (string, error) {
	f, err := os.Open(path) // #nosec G304 -- caller-controlled local path (fetch/verify/sync-dir dest)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory, not a file", path)
	}
	return GitBlobSHA(f, info.Size())
}

// IsLFSPointer reports whether head (the first bytes of a file, or the
// whole file for small files) is a Git LFS pointer file rather than real
// blob content. LFS pointer files are small plain-text files; a genuine
// binary/text blob would need an implausible coincidence to share this
// exact prefix.
func IsLFSPointer(head []byte) bool {
	return bytes.HasPrefix(head, []byte(gitLFSPointerPrefix))
}

// checkBlobSHA verifies that raw's git blob SHA-1 equals want. Used after
// blob-API fetches so a wrong or corrupted payload can never be written
// to disk as if it were the tree-listed blob (a size check alone is a
// tautology there — the expected size IS the payload's own length).
func checkBlobSHA(raw []byte, want string) error {
	got, err := GitBlobSHA(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("blob content SHA mismatch: got %s, want %s", got, want)
	}
	return nil
}

// FetchBlobBytes fetches a blob's raw content via the git blobs API
// (GET /repos/{o}/{r}/git/blobs/{sha}) and base64-decodes it. This is the
// shared blob-content path used by the downloader's raw-CDN fallback and
// plan's LFS-pointer probe. Unlike the raw CDN, the blobs API always
// returns the blob as git stores it — for an LFS-tracked file that is the
// pointer text, never the resolved object.
func FetchBlobBytes(ctx context.Context, api API, addr Address, sha string) ([]byte, error) {
	if sha == "" {
		return nil, fmt.Errorf("blob SHA is empty")
	}
	data, err := api.Get(ctx, fmt.Sprintf("/repos/%s/%s/git/blobs/%s", url.PathEscape(addr.Owner), url.PathEscape(addr.Repo), url.PathEscape(sha)), nil)
	if err != nil {
		return nil, err
	}
	var blob struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal(data, &blob); err != nil {
		return nil, fmt.Errorf("parsing blob response: %w", err)
	}
	if blob.Encoding != "base64" {
		return nil, fmt.Errorf("blob API returned unsupported encoding %q", blob.Encoding)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(blob.Content, "\n", ""))
	if err != nil {
		return nil, fmt.Errorf("decoding blob content: %w", err)
	}
	return raw, nil
}
