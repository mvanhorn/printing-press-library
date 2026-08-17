// Copyright 2026 Rick van de Laar and contributors. Licensed under Apache-2.0. See LICENSE.

// Package ghfetch holds pure, table-tested logic for the hand-written
// fetch/plan/verify/sync-dir/stats commands: address parsing, path
// escaping/safety, git blob SHA hashing, glob matching, and the repo-tree
// walker. Filesystem/network side effects that DO belong here (WalkTree's
// API calls, the Downloader's byte streaming) are kept behind small
// interfaces so the pure logic stays unit-testable without a live server.
package ghfetch

import (
	"fmt"
	"net/url"
	"strings"
)

// Address identifies a location inside a GitHub repository: an owner/repo
// pair, an optional path within the repo (empty means the whole repo), and
// an optional ref (branch, tag, or commit SHA; empty means "resolve the
// repo's default branch").
type Address struct {
	Owner string
	Repo  string
	Path  string
	Ref   string
}

// ParseAddress parses a user-supplied target string into an Address. Six
// input shapes are accepted:
//
//	owner/repo
//	owner/repo/sub/path
//	owner/repo#ref
//	owner/repo/sub/path#ref
//	https://github.com/owner/repo
//	https://github.com/owner/repo/tree/<ref>/<path...>
//	https://github.com/owner/repo/blob/<ref>/<path...>
//
// A trailing "#ref" suffix is accepted on ANY of the above forms (including
// the tree/blob URL forms, where it overrides the ref segment already
// present in the URL path). Trailing slashes are trimmed before parsing;
// empty owner or repo is rejected.
//
// Limitation: for /tree/<ref>/... and /blob/<ref>/... URLs, the ref is taken
// as the single path segment immediately following "tree"/"blob" — GitHub's
// own URL shape has no unambiguous delimiter between a ref and the path that
// follows it. A branch name containing "/" (e.g. "feature/foo") is NOT
// resolvable from such a URL; the segment "feature" would be misread as the
// ref and "foo/..." as the path. Use an explicit "#ref" suffix or the
// command's --ref flag instead when the branch name contains a slash.
func ParseAddress(arg string) (Address, error) {
	trimmed := strings.TrimSpace(arg)
	if trimmed == "" {
		return Address{}, fmt.Errorf("address is empty")
	}

	var explicitRef string
	if idx := strings.IndexByte(trimmed, '#'); idx >= 0 {
		explicitRef = strings.TrimSpace(trimmed[idx+1:])
		trimmed = trimmed[:idx]
	}
	trimmed = strings.TrimRight(trimmed, "/")
	if trimmed == "" {
		return Address{}, fmt.Errorf("address is empty after stripping #ref")
	}

	var (
		addr Address
		err  error
	)
	if looksLikeGitHubURL(trimmed) {
		addr, err = parseGitHubURL(trimmed)
	} else {
		addr, err = parseShorthand(trimmed)
	}
	if err != nil {
		return Address{}, err
	}
	if explicitRef != "" {
		addr.Ref = explicitRef
	}
	if addr.Owner == "" || addr.Repo == "" {
		return Address{}, fmt.Errorf("could not determine owner/repo from %q", arg)
	}
	return addr, nil
}

func looksLikeGitHubURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func parseShorthand(s string) (Address, error) {
	parts := strings.Split(s, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return Address{}, fmt.Errorf("expected owner/repo[/path], got %q", s)
	}
	addr := Address{Owner: parts[0], Repo: parts[1]}
	if len(parts) > 2 {
		addr.Path = strings.Join(parts[2:], "/")
	}
	return addr, nil
}

func parseGitHubURL(raw string) (Address, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return Address{}, fmt.Errorf("invalid URL %q: %w", raw, err)
	}
	if !strings.EqualFold(u.Hostname(), "github.com") {
		return Address{}, fmt.Errorf("unsupported host %q: only github.com URLs are supported", u.Hostname())
	}
	segs := splitPathSegments(u.Path)
	if len(segs) < 2 || segs[0] == "" || segs[1] == "" {
		return Address{}, fmt.Errorf("URL %q is missing an owner/repo path", raw)
	}
	addr := Address{Owner: segs[0], Repo: segs[1]}
	if len(segs) == 2 {
		return addr, nil
	}
	kind := segs[2]
	if kind != "tree" && kind != "blob" {
		// Unrecognized third segment (e.g. "issues", "actions"); no ref/path
		// to extract, but owner/repo alone is still a valid address.
		return addr, nil
	}
	if len(segs) < 4 || segs[3] == "" {
		return Address{}, fmt.Errorf("URL %q has %q but no ref segment", raw, kind)
	}
	addr.Ref = segs[3]
	if len(segs) > 4 {
		addr.Path = strings.Join(segs[4:], "/")
	}
	return addr, nil
}

// splitPathSegments splits a URL path on "/", dropping empty segments
// produced by leading/trailing/duplicate slashes. net/url has already
// percent-decoded u.Path, so segments are plain text.
func splitPathSegments(p string) []string {
	raw := strings.Split(p, "/")
	segs := make([]string, 0, len(raw))
	for _, s := range raw {
		if s != "" {
			segs = append(segs, s)
		}
	}
	return segs
}

// EscapePath percent-encodes each "/"-separated segment of p independently
// and rejoins them with "/". GitHub's raw CDN 404s on unescaped spaces and
// other reserved characters in a path (e.g. "books/machine learning/foo
// bar.pdf"); escaping segment-by-segment keeps the "/" separators intact
// while making every segment's literal characters (including unicode) safe
// for use in a URL path.
func EscapePath(p string) string {
	if p == "" {
		return ""
	}
	segs := strings.Split(p, "/")
	for i, s := range segs {
		segs[i] = url.PathEscape(s)
	}
	return strings.Join(segs, "/")
}
