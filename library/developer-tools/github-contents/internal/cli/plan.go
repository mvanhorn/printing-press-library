// Copyright 2026 Rick van de Laar and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written command: plan. See internal/ghfetch for the underlying
// address-parsing/tree-walk logic.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/github-contents/internal/ghfetch"
	"github.com/spf13/cobra"
)

// maxLFSProbes bounds how many small candidate files plan will fetch blob
// content for to confirm an LFS pointer. Only files at or under
// ghfetch.LFSMaxPointerSize bytes are probed — real LFS pointer files are
// always small plain-text stubs — so a books-and-PDFs repo full of large
// files never spends an extra API request here, while the cap protects
// against a repo with thousands of tiny files.
const maxLFSProbes = 50

// pp:data-source live
func newNovelPlanCmd(flags *rootFlags) *cobra.Command {
	var (
		flagRef     string
		flagInclude []string
		flagExclude []string
		flagTop     int
	)

	cmd := &cobra.Command{
		Use:   "plan <owner/repo[/path][#ref]>",
		Short: "Preview exactly what a fetch would download — file list, sizes, total bytes, API-request cost vs your remaining quota",
		Long: "Preview exactly what a fetch would download — file list, sizes, total bytes, API-request cost vs your remaining quota, " +
			"and LFS-pointer warnings — before spending any bandwidth.\n" +
			"Use this command to preview what 'fetch' would download (files, sizes, total, API cost) without writing anything. " +
			"Do NOT use it to compare an already-downloaded local directory against the remote; use 'verify' instead.",
		Example: "  github-contents-pp-cli plan mjwoon/AI-readings/books --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "target=octocat/Hello-World",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would preview the file list, sizes, total bytes, and API cost for the target")
				return nil
			}
			if len(args) == 0 || args[0] == "" {
				return usageErr(fmt.Errorf("target is required\nUsage: %s <owner/repo[/path][#ref]>", cmd.CommandPath()))
			}
			target := args[0]
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			walk, err := walkGHTarget(ctx, cmd, flags, target, flagRef, flagInclude, flagExclude)
			if err != nil {
				return err
			}
			addr, result, files := walk.Addr, walk.Result, walk.Files

			type fileEntry struct {
				Path string `json:"path"`
				Size int64  `json:"size"`
				SHA  string `json:"sha"`
			}
			var totalBytes int64
			entries := make([]fileEntry, 0, len(files))
			for _, f := range files {
				totalBytes += f.Size
				entries = append(entries, fileEntry{Path: f.RelTo(addr.Path), Size: f.Size, SHA: f.SHA})
			}

			largest := append([]fileEntry(nil), entries...)
			sort.Slice(largest, func(i, j int) bool { return largest[i].Size > largest[j].Size })
			top := flagTop
			if top <= 0 {
				top = 10
			}
			if top > len(largest) {
				top = len(largest)
			}
			largest = largest[:top]

			lfsPointers := detectLFSPointers(ctx, walk.Client, addr, files)

			// api_cost is what a subsequent `fetch` of this exact selection
			// would spend: WalkTree's own request count, zero per file (raw
			// CDN downloads don't count against the REST API quota). The
			// rate-limit probe and LFS-detection probes below are plan's own
			// overhead, not part of that projected cost.
			apiCost := result.APIRequests
			remaining, limit, rlErr := fetchRateLimit(ctx, walk.Client)
			if rlErr != nil {
				remaining, limit = -1, -1
			}

			persistTreeEntries(addr, result.Files)

			envelope := map[string]any{
				"target":             target,
				"ref":                addr.Ref,
				"file_count":         len(files),
				"total_bytes":        totalBytes,
				"human_total":        ghfetch.HumanBytes(totalBytes),
				"api_cost":           apiCost,
				"remaining_quota":    remaining,
				"quota_limit":        limit,
				"largest":            largest,
				"lfs_pointers":       lfsPointers,
				"truncated":          result.Truncated,
				"skipped_symlinks":   result.SkippedSymlinks,
				"skipped_submodules": result.SkippedSubmodules,
				"files":              entries,
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "%s @ %s\n", ghfetch.SanitizeTerminal(target), ghfetch.SanitizeTerminal(addr.Ref))
				fmt.Fprintf(cmd.OutOrStdout(), "%d files, %s total, %d API request(s) to fetch, %d/%d quota remaining\n",
					len(files), ghfetch.HumanBytes(totalBytes), apiCost, remaining, limit)
				if len(largest) > 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "\nLargest files:")
					for _, e := range largest {
						fmt.Fprintf(cmd.OutOrStdout(), "  %10s  %s\n", ghfetch.HumanBytes(e.Size), ghfetch.SanitizeTerminal(e.Path))
					}
				}
				if len(lfsPointers) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "\nwarning: %d possible LFS pointer file(s) detected\n", len(lfsPointers))
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
		},
	}

	cmd.Flags().StringVar(&flagRef, "ref", "", "Branch, tag, or commit SHA (overrides #ref in the target)")
	cmd.Flags().StringSliceVar(&flagInclude, "include", nil, "Glob pattern(s) to include (repeatable; empty = all)")
	cmd.Flags().StringSliceVar(&flagExclude, "exclude", nil, "Glob pattern(s) to exclude (repeatable)")
	cmd.Flags().IntVar(&flagTop, "top", 10, "Number of largest files to list")
	return cmd
}

// fetchRateLimit reads the caller's current API quota via GET /rate_limit.
func fetchRateLimit(ctx context.Context, c ghfetch.API) (remaining, limit int, err error) {
	data, err := c.Get(ctx, "/rate_limit", nil)
	if err != nil {
		return 0, 0, err
	}
	var resp struct {
		Resources struct {
			Core struct {
				Limit     int `json:"limit"`
				Remaining int `json:"remaining"`
			} `json:"core"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, 0, err
	}
	return resp.Resources.Core.Remaining, resp.Resources.Core.Limit, nil
}

// detectLFSPointers confirms Git LFS pointer files among the smallest
// candidates (at or under ghfetch.LFSMaxPointerSize — the LFS spec's max
// pointer size) by fetching their real content via the shared blobs-API
// helper — a genuine check, not a size-only heuristic — bounded by
// maxLFSProbes so a repo with many tiny files can't blow up plan's API
// cost. Probe failures are silently skipped; this is a best-effort preview
// feature, not fetch's authoritative post-download LFS detection.
func detectLFSPointers(ctx context.Context, c ghfetch.API, addr ghfetch.Address, files []ghfetch.TreeFile) []string {
	var pointers []string
	probes := 0
	for _, f := range files {
		if f.Size <= 0 || f.Size > ghfetch.LFSMaxPointerSize {
			continue
		}
		if probes >= maxLFSProbes {
			break
		}
		probes++
		raw, err := ghfetch.FetchBlobBytes(ctx, c, addr, f.SHA)
		if err != nil {
			continue
		}
		if ghfetch.IsLFSPointer(raw) {
			pointers = append(pointers, f.RelTo(addr.Path))
		}
	}
	return pointers
}
