// Copyright 2026 Rick van de Laar and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written support shared by fetch/plan/verify/sync-dir/stats/tarball/
// releases-download — the seven live commands built on internal/ghfetch.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/github-contents/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/github-contents/internal/ghfetch"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/github-contents/internal/store"
	"github.com/spf13/cobra"
)

// classifyGHTreeError converts a WalkTree (or rate-limit-probe) error
// touching a repo/path into an actionable CLI error. GitHub returns an
// identical 404 for "no such repo/path" AND "private repo without auth" —
// there is no way to distinguish them from the response alone — so the
// hint always names both possibilities and points at GITHUB_TOKEN. Non-404
// errors (401/403/429/5xx) fall through to the generic classifyAPIError,
// which already covers those with GITHUB_TOKEN/GH_TOKEN hints.
func classifyGHTreeError(err error, flags *rootFlags, target string) error {
	var apiErrTyped *client.APIError
	if errors.As(err, &apiErrTyped) && apiErrTyped.StatusCode == http.StatusNotFound {
		return notFoundErr(fmt.Errorf("%s not found (or the path doesn't exist in this repo)\nhint: private repos return 404 without auth — set GITHUB_TOKEN", target))
	}
	return classifyAPIError(err, flags)
}

// resolveGHAddress parses target into a ghfetch.Address and applies an
// explicit --ref flag override, which wins over any "#ref" suffix already
// present in target.
func resolveGHAddress(target, refFlag string) (ghfetch.Address, error) {
	addr, err := ghfetch.ParseAddress(target)
	if err != nil {
		return ghfetch.Address{}, err
	}
	if refFlag != "" {
		addr.Ref = refFlag
	}
	return addr, nil
}

// downloadPhaseCtx returns the context for a bulk byte-transfer phase
// (fetch/sync-dir file downloads, tarball, release assets). The global
// --timeout default (60s) is sized for JSON API calls and must not cap a
// multi-GB download — a 1.92 GB fetch would die mid-transfer at exactly
// the 60s mark otherwise (observed: 64/118 files then "context deadline
// exceeded" on every remaining file). Rule: when the user did NOT
// explicitly set --timeout, the download phase runs unbounded-with-cancel
// (Ctrl-C and parent cancellation still propagate); an explicitly-set
// --timeout is honored for the whole run, downloads included. The
// walk/API phase stays on boundCtx regardless — the default deadline is
// right for JSON calls.
func downloadPhaseCtx(cmd *cobra.Command, flags *rootFlags) (context.Context, context.CancelFunc) {
	var timeout time.Duration
	if flags != nil {
		timeout = flags.timeout
	}
	return deadlineForTransfer(cmd.Context(), timeout, cmd.Flags().Changed("timeout"))
}

// deadlineForTransfer is the pure context-selection rule behind
// downloadPhaseCtx, extracted for unit testing: an explicit positive
// timeout becomes a deadline; anything else is cancel-only.
func deadlineForTransfer(parent context.Context, timeout time.Duration, explicit bool) (context.Context, context.CancelFunc) {
	if explicit && timeout > 0 {
		return context.WithTimeout(parent, timeout)
	}
	return context.WithCancel(parent)
}

// ghWalk bundles everything the tree-walking commands share after their
// common preamble: the resolved address (Ref filled in from the walk), the
// raw walk result, the glob-filtered file list, and the API client for
// follow-up calls (downloads, rate-limit probes).
type ghWalk struct {
	Addr   ghfetch.Address
	Result *ghfetch.WalkResult
	Files  []ghfetch.TreeFile
	Client *client.Client
}

// walkGHTarget is the shared preamble of the five tree-walking commands
// (fetch/plan/verify/sync-dir/stats): parse the target (+ --ref override),
// warn once about invalid globs, build the API client, walk the tree,
// classify walk errors, and glob-filter the listing. Errors are already
// CLI-classified (usageErr / notFoundErr / classifyAPIError) — callers
// return them as-is.
func walkGHTarget(ctx context.Context, cmd *cobra.Command, flags *rootFlags, target, refFlag string, includes, excludes []string) (*ghWalk, error) {
	addr, err := resolveGHAddress(target, refFlag)
	if err != nil {
		return nil, usageErr(fmt.Errorf("%w\nUsage: %s", err, cmd.UseLine()))
	}
	warnInvalidGlobs(cmd.ErrOrStderr(), includes, excludes)
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	result, err := ghfetch.WalkTree(ctx, c, addr)
	if err != nil {
		return nil, classifyGHTreeError(err, flags, target)
	}
	addr.Ref = result.Ref
	return &ghWalk{
		Addr:   addr,
		Result: result,
		Files:  filterWalkFiles(result.Files, addr, includes, excludes),
		Client: c,
	}, nil
}

// warnInvalidGlobs prints one stderr note (not one per file) when any
// --include/--exclude pattern fails to compile, per ghfetch.MatchGlobs'
// silent-no-match contract for bad patterns.
func warnInvalidGlobs(w io.Writer, includes, excludes []string) {
	var bad []string
	bad = append(bad, ghfetch.InvalidGlobs(includes)...)
	bad = append(bad, ghfetch.InvalidGlobs(excludes)...)
	if len(bad) == 0 {
		return
	}
	fmt.Fprintf(w, "warning: ignoring invalid glob pattern(s), treated as never-matching: %s\n", strings.Join(bad, ", "))
}

// filterWalkFiles applies --include/--exclude glob filtering to a WalkTree
// result, matching each file's path relative to addr.Path.
func filterWalkFiles(files []ghfetch.TreeFile, addr ghfetch.Address, includes, excludes []string) []ghfetch.TreeFile {
	if len(includes) == 0 && len(excludes) == 0 {
		return files
	}
	out := make([]ghfetch.TreeFile, 0, len(files))
	for _, f := range files {
		if ghfetch.MatchGlobs(f.RelTo(addr.Path), includes, excludes) {
			out = append(out, f)
		}
	}
	return out
}

// storeWritethroughCap bounds how many tree entries fetch/plan persist to
// the local store in one call, so an enormous repo listing cannot balloon
// a single command invocation's local-store write.
const storeWritethroughCap = 20000

// persistTreeEntries best-effort writes tree listings into the local store
// under resource_type "trees" so the framework `search` command can answer
// "which repo path had file X" offline. Every failure mode here is
// non-fatal to the calling command: a single summarizing stderr note is
// printed and the command's own exit code is unaffected.
func persistTreeEntries(addr ghfetch.Address, files []ghfetch.TreeFile) {
	if len(files) == 0 {
		return
	}
	db, err := store.Open(defaultDBPath("github-contents-pp-cli"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "note: local store unavailable, skipping offline-search write-through: %v\n", err)
		return
	}
	defer db.Close()

	n := len(files)
	if n > storeWritethroughCap {
		n = storeWritethroughCap
	}
	var firstErr error
	failures := 0
	for _, f := range files[:n] {
		id := fmt.Sprintf("%s/%s@%s:%s", addr.Owner, addr.Repo, addr.Ref, f.Path)
		data, marshalErr := json.Marshal(map[string]any{
			"path":  f.Path,
			"size":  f.Size,
			"sha":   f.SHA,
			"owner": addr.Owner,
			"repo":  addr.Repo,
			"ref":   addr.Ref,
		})
		if marshalErr != nil {
			failures++
			if firstErr == nil {
				firstErr = marshalErr
			}
			continue
		}
		if err := db.Upsert("trees", id, data); err != nil {
			failures++
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	if failures > 0 {
		fmt.Fprintf(os.Stderr, "note: %d/%d tree entries failed to write to the local store (offline search may be incomplete): %v\n", failures, n, firstErr)
	}
}

// diffEntry pairs a remote TreeFile with its path relative to the fetch
// root (the same relative path used for local filesystem joins).
type diffEntry struct {
	Rel  string
	File ghfetch.TreeFile
}

// dirDiff is the outcome of comparing a local directory against a remote
// WalkTree listing: which remote files match locally by blob SHA, which
// differ, which are missing locally, which local files have no remote
// counterpart, and which remote paths were skipped as unsafe to represent
// locally (absolute / traversal / drive-or-stream syntax — the same paths
// a fetch would refuse to write).
type dirDiff struct {
	Matched []diffEntry
	Changed []diffEntry
	Missing []diffEntry
	Extra   []string
	Unsafe  []string
}

// diffLocalDir compares localDir's contents against remoteFiles (already
// glob-filtered by the caller if desired — includes/excludes here only
// gate which local-only files count as "extra", matching what a
// subsequent fetch/sync-dir with the same filters would have written).
func diffLocalDir(localDir string, addr ghfetch.Address, remoteFiles []ghfetch.TreeFile, includes, excludes []string) (*dirDiff, error) {
	d := &dirDiff{}

	remoteRel := make(map[string]ghfetch.TreeFile, len(remoteFiles))
	for _, f := range remoteFiles {
		rel, err := ghfetch.SafeRelPath(f.RelTo(addr.Path))
		if err != nil {
			// Not silently droppable: a remote path fetch would refuse to
			// write means the local dir CANNOT faithfully mirror the remote,
			// so it must surface (verify reports ok=false on it).
			d.Unsafe = append(d.Unsafe, f.Path)
			continue
		}
		remoteRel[rel] = f
	}

	for rel, f := range remoteRel {
		localPath := filepath.Join(localDir, filepath.FromSlash(rel))
		info, statErr := os.Stat(localPath)
		if statErr != nil || info.IsDir() {
			d.Missing = append(d.Missing, diffEntry{Rel: rel, File: f})
			continue
		}
		localSHA, shaErr := ghfetch.GitBlobSHAFile(localPath)
		if shaErr != nil || localSHA != f.SHA {
			d.Changed = append(d.Changed, diffEntry{Rel: rel, File: f})
			continue
		}
		d.Matched = append(d.Matched, diffEntry{Rel: rel, File: f})
	}

	localRelPaths, err := listLocalFiles(localDir)
	if err != nil {
		return nil, err
	}
	for _, rel := range localRelPaths {
		if _, ok := remoteRel[rel]; ok {
			continue
		}
		if len(includes) > 0 || len(excludes) > 0 {
			if !ghfetch.MatchGlobs(rel, includes, excludes) {
				continue
			}
		}
		d.Extra = append(d.Extra, rel)
	}

	sort.Slice(d.Matched, func(i, j int) bool { return d.Matched[i].Rel < d.Matched[j].Rel })
	sort.Slice(d.Changed, func(i, j int) bool { return d.Changed[i].Rel < d.Changed[j].Rel })
	sort.Slice(d.Missing, func(i, j int) bool { return d.Missing[i].Rel < d.Missing[j].Rel })
	sort.Strings(d.Extra)
	sort.Strings(d.Unsafe)
	return d, nil
}

// listLocalFiles walks root and returns every regular file's path relative
// to root, slash-separated. In-flight or crash-orphaned download temps —
// legacy "X.partial" names AND ghfetch.StreamToFile's randomized
// "X.partial-NNNN" names — plus ".git" directories are skipped so they
// never surface as spurious "extra" entries in verify/sync-dir.
func listLocalFiles(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".partial") || strings.Contains(filepath.Base(p), ".partial-") {
			return nil
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return relErr
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
