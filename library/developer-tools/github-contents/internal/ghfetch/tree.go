// Copyright 2026 Rick van de Laar and contributors. Licensed under Apache-2.0. See LICENSE.

package ghfetch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
)

// API is the minimal surface WalkTree needs from an HTTP client. The
// generated *client.Client satisfies this directly (its Get method has
// this exact signature), so no adapter is needed in production; tests
// supply a small fake.
type API interface {
	Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error)
}

// TreeFile describes one downloadable blob entry from a repo tree. Path is
// the FULL path from the repo root (as GitHub's recursive tree API
// returns it) — use RelTo to get a path relative to the fetch root.
type TreeFile struct {
	Path string
	SHA  string
	Size int64
	Mode string
}

// RelTo returns f.Path relative to basePath (the Address.Path a WalkTree
// call was scoped to). When basePath is empty (whole-repo fetch), the full
// path is already relative to the fetch root and is returned unchanged.
// When f.Path IS basePath (a single-file target), the file's own basename
// is returned rather than an empty string.
//
// The prefix check is segment-boundary-aware (mirroring underPath): a path
// like "booksmore/x" is NOT under basePath "books" and is returned
// unchanged, rather than being mangled to "more/x" by a naive TrimPrefix.
func (f TreeFile) RelTo(basePath string) string {
	if basePath == "" {
		return f.Path
	}
	if f.Path == basePath {
		return path.Base(f.Path)
	}
	if strings.HasPrefix(f.Path, basePath+"/") {
		return f.Path[len(basePath)+1:]
	}
	return f.Path
}

// WalkResult is the outcome of walking a repo tree scoped to an Address.
type WalkResult struct {
	Files             []TreeFile
	Truncated         bool
	SkippedSymlinks   []string
	SkippedSubmodules []string
	Ref               string
	APIRequests       int
}

// treeEntry mirrors one element of GitHub's git tree API response.
type treeEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
	Size int64  `json:"size"`
}

type treeResponse struct {
	SHA       string      `json:"sha"`
	Truncated bool        `json:"truncated"`
	Tree      []treeEntry `json:"tree"`
}

// errRequestCapHit is an internal sentinel used by the bounded-BFS fallback
// to unwind once maxBFSRequests is reached without treating it as a hard
// error — the caller marks the result Truncated and returns what it has.
var errRequestCapHit = errors.New("ghfetch: bfs request cap hit")

// maxBFSRequests bounds the bounded-BFS fallback (used when the initial
// recursive=1 tree response itself comes back truncated) so a pathological
// or enormous repo cannot spin the walker indefinitely.
const maxBFSRequests = 500

// WalkTree lists every downloadable blob under addr.Path (or the whole
// repo, when addr.Path is empty) at addr.Ref (or the repo's default branch,
// when addr.Ref is empty). It tries GitHub's recursive tree listing first
// (one API request after ref resolution); if that response is itself
// truncated (huge repos), it falls back to a bounded breadth-first walk
// that fetches one tree level per API call, capped at maxBFSRequests total
// requests across the whole call.
func WalkTree(ctx context.Context, api API, addr Address) (*WalkResult, error) {
	result := &WalkResult{Ref: addr.Ref}

	ref := addr.Ref
	if ref == "" {
		data, err := api.Get(ctx, fmt.Sprintf("/repos/%s/%s", url.PathEscape(addr.Owner), url.PathEscape(addr.Repo)), nil)
		result.APIRequests++
		if err != nil {
			return result, err
		}
		var repoMeta struct {
			DefaultBranch string `json:"default_branch"`
		}
		if err := json.Unmarshal(data, &repoMeta); err != nil {
			return result, fmt.Errorf("parsing repo metadata: %w", err)
		}
		if repoMeta.DefaultBranch == "" {
			return result, fmt.Errorf("repo %s/%s has no default branch", addr.Owner, addr.Repo)
		}
		ref = repoMeta.DefaultBranch
	}
	result.Ref = ref

	data, err := api.Get(ctx, treeURL(addr, ref), map[string]string{"recursive": "1"})
	result.APIRequests++
	if err != nil {
		return result, err
	}
	var resp treeResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return result, fmt.Errorf("parsing tree response: %w", err)
	}
	if resp.Truncated {
		return walkTreeBFS(ctx, api, addr, ref, result)
	}
	applyRecursiveEntries(result, addr.Path, resp.Tree)
	return result, nil
}

func treeURL(addr Address, shaOrRef string) string {
	return fmt.Sprintf("/repos/%s/%s/git/trees/%s", url.PathEscape(addr.Owner), url.PathEscape(addr.Repo), url.PathEscape(shaOrRef))
}

// applyRecursiveEntries classifies entries from a recursive=1 tree listing
// (whose Path fields are already full repo-root-relative paths), keeping
// only those under basePath.
func applyRecursiveEntries(result *WalkResult, basePath string, entries []treeEntry) {
	for _, e := range entries {
		if !underPath(e.Path, basePath) {
			continue
		}
		classifyEntry(result, e.Path, e)
	}
}

func underPath(entryPath, basePath string) bool {
	if basePath == "" {
		return true
	}
	return entryPath == basePath || strings.HasPrefix(entryPath, basePath+"/")
}

// classifyEntry buckets one tree entry (at its already-resolved fullPath)
// into Files, SkippedSymlinks, or SkippedSubmodules. Any blob whose mode
// is not 120000 (symlink) is downloadable — this deliberately includes
// legacy/non-canonical modes like 100664, not just 100644/100755.
func classifyEntry(result *WalkResult, fullPath string, e treeEntry) {
	switch {
	case e.Type == "commit":
		result.SkippedSubmodules = append(result.SkippedSubmodules, fullPath)
	case e.Mode == "120000":
		result.SkippedSymlinks = append(result.SkippedSymlinks, fullPath)
	case e.Type == "blob":
		result.Files = append(result.Files, TreeFile{Path: fullPath, SHA: e.SHA, Size: e.Size, Mode: e.Mode})
	}
}

// walkTreeBFS is the fallback used when GitHub's recursive=1 response is
// itself truncated. It resolves the subtree at addr.Path one path segment
// at a time (one non-recursive API call per segment), then breadth-first
// walks every subtree from there, one API call per subtree, until either
// the tree is exhausted or maxBFSRequests is reached (in which case
// result.Truncated is set and the partial result is returned).
func walkTreeBFS(ctx context.Context, api API, addr Address, ref string, result *WalkResult) (*WalkResult, error) {
	startEntries, err := resolveSubtreeAtPath(ctx, api, addr, ref, result)
	if err != nil {
		return result, err
	}

	type queueItem struct {
		prefix  string
		entries []treeEntry
	}
	var queue []queueItem
	if len(startEntries) > 0 {
		queue = append(queue, queueItem{prefix: addr.Path, entries: startEntries})
	}

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]
		for _, e := range item.entries {
			fullPath := e.Path
			if item.prefix != "" {
				fullPath = item.prefix + "/" + e.Path
			}
			if e.Type != "tree" {
				classifyEntry(result, fullPath, e)
				continue
			}
			if result.APIRequests >= maxBFSRequests {
				result.Truncated = true
				continue
			}
			data, err := api.Get(ctx, treeURL(addr, e.SHA), nil)
			result.APIRequests++
			if err != nil {
				return result, fmt.Errorf("walking subtree %s: %w", fullPath, err)
			}
			var sub treeResponse
			if err := json.Unmarshal(data, &sub); err != nil {
				return result, fmt.Errorf("parsing subtree %s: %w", fullPath, err)
			}
			if sub.Truncated {
				result.Truncated = true
			}
			queue = append(queue, queueItem{prefix: fullPath, entries: sub.Tree})
		}
	}
	return result, nil
}

// resolveSubtreeAtPath descends from ref's root tree to addr.Path, fetching
// one tree level per path segment (non-recursive GET, one API call each).
// It returns the entries at addr.Path's own level for walkTreeBFS to
// continue from. If addr.Path resolves to a single blob rather than a
// directory, that file is appended directly to result.Files and a nil
// slice is returned (nothing further to descend into).
func resolveSubtreeAtPath(ctx context.Context, api API, addr Address, ref string, result *WalkResult) ([]treeEntry, error) {
	fetchTree := func(shaOrRef string) (treeResponse, error) {
		if result.APIRequests >= maxBFSRequests {
			result.Truncated = true
			return treeResponse{}, errRequestCapHit
		}
		data, err := api.Get(ctx, treeURL(addr, shaOrRef), nil)
		result.APIRequests++
		if err != nil {
			return treeResponse{}, err
		}
		var tr treeResponse
		if err := json.Unmarshal(data, &tr); err != nil {
			return treeResponse{}, fmt.Errorf("parsing tree response: %w", err)
		}
		if tr.Truncated {
			result.Truncated = true
		}
		return tr, nil
	}

	root, err := fetchTree(ref)
	if err != nil {
		if errors.Is(err, errRequestCapHit) {
			return nil, nil
		}
		return nil, err
	}
	if addr.Path == "" {
		return root.Tree, nil
	}

	entries := root.Tree
	segments := strings.Split(addr.Path, "/")
	for i, seg := range segments {
		var found *treeEntry
		for j := range entries {
			if entries[j].Path == seg {
				found = &entries[j]
				break
			}
		}
		if found == nil {
			return nil, fmt.Errorf("path %q not found in repo tree (segment %q missing)", addr.Path, seg)
		}
		last := i == len(segments)-1
		if found.Type != "tree" {
			if !last {
				return nil, fmt.Errorf("path %q traverses through non-directory entry %q", addr.Path, seg)
			}
			// Route through classifyEntry so a symlink or submodule target
			// lands in the Skipped* buckets instead of being downloaded as
			// a regular file (mirrors the recursive path's behavior).
			classifyEntry(result, addr.Path, *found)
			return nil, nil
		}
		sub, err := fetchTree(found.SHA)
		if err != nil {
			if errors.Is(err, errRequestCapHit) {
				return nil, nil
			}
			return nil, err
		}
		entries = sub.Tree
		if last {
			return entries, nil
		}
	}
	return entries, nil
}
