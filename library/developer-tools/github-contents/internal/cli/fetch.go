// Copyright 2026 Rick van de Laar and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written command: fetch. See internal/ghfetch for the underlying
// address-parsing/tree-walk/download logic.

package cli

import (
	"fmt"
	"path"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/github-contents/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/github-contents/internal/ghfetch"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newFetchCmd(flags *rootFlags) *cobra.Command {
	var (
		flagOut         string
		flagRef         string
		flagInclude     []string
		flagExclude     []string
		flagFlatten     bool
		flagForce       bool
		flagConcurrency int
		flagMaxFiles    int
	)

	cmd := &cobra.Command{
		Use:   "fetch <owner/repo[/path][#ref]>",
		Short: "Download a GitHub folder without cloning — recursive, structure-preserving",
		Long: "Download a GitHub folder without cloning — recursive, structure-preserving, and resumable.\n" +
			"Accepts owner/repo, owner/repo/sub/path, an optional #ref suffix, or a github.com tree/blob URL.",
		Example: strings.Trim(`
  github-contents-pp-cli fetch mjwoon/AI-readings/books --out ./books
  github-contents-pp-cli fetch mjwoon/AI-readings/books --include "*.pdf" --out ./pdfs
  github-contents-pp-cli fetch octocat/Hello-World --out /tmp/hw --agent
`, "\n"),
		Annotations: map[string]string{
			"pp:happy-args": "target=octocat/Hello-World;--out=/tmp/pp-ghc-dogfood/fetch",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would download the target repo/path recursively and write files under --out")
				return nil
			}
			if len(args) == 0 || args[0] == "" {
				return usageErr(fmt.Errorf("target is required\nUsage: %s <owner/repo[/path][#ref]>", cmd.CommandPath()))
			}
			target := args[0]
			addr, err := resolveGHAddress(target, flagRef)
			if err != nil {
				return usageErr(fmt.Errorf("%w\nUsage: %s <owner/repo[/path][#ref]>", err, cmd.CommandPath()))
			}
			outDir := flagOut
			if outDir == "" {
				if addr.Path != "" {
					outDir = path.Base(addr.Path)
				} else {
					outDir = addr.Repo
				}
			}
			if cliutil.IsVerifyEnv() {
				// Deliberately no directory claim here: the effective out
				// dir can still change after the walk (single-file --out
				// correction below), which never runs in verify mode.
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch %s\n", target)
				return nil
			}

			// Walk/API phase: --timeout-bounded (the default 60s is right
			// for the 1-2 JSON calls a walk makes).
			walkCtx, cancelWalk := boundCtx(cmd.Context(), flags)
			defer cancelWalk()

			walk, err := walkGHTarget(walkCtx, cmd, flags, target, flagRef, flagInclude, flagExclude)
			if err != nil {
				return err
			}
			addr = walk.Addr

			// Single-file target with a defaulted --out: without this, the
			// default (basename of addr.Path) nests the file inside a
			// directory named after itself (README.md/README.md). Write to
			// the current directory instead.
			if flagOut == "" && addr.Path != "" && len(walk.Result.Files) == 1 && walk.Result.Files[0].Path == addr.Path {
				outDir = "."
			}

			files := walk.Files
			if flagMaxFiles > 0 && len(files) > flagMaxFiles {
				files = files[:flagMaxFiles]
			}
			if cliutil.IsDogfoodEnv() && len(files) > 5 {
				files = files[:5]
			}

			dl := &ghfetch.Downloader{
				Concurrency: flagConcurrency,
				Force:       flagForce,
				Flatten:     flagFlatten,
				Limiter:     cliutil.NewAdaptiveLimiterAuto(4),
				Token:       walk.Client.Config.AuthHeader(),
				APIClient:   walk.Client,
			}
			// Download phase: exempt from the DEFAULT --timeout (60s would
			// kill a multi-GB fetch mid-stream); see downloadPhaseCtx.
			dlCtx, cancelDl := downloadPhaseCtx(cmd, flags)
			defer cancelDl()
			report, dlErr := dl.Download(dlCtx, addr, files, outDir)
			if dlErr != nil {
				return apiErr(dlErr)
			}

			persistTreeEntries(addr, walk.Result.Files)

			envelope := map[string]any{
				"target":             target,
				"ref":                addr.Ref,
				"out_dir":            outDir,
				"files_total":        len(files),
				"downloaded":         report.Downloaded,
				"skipped":            report.Skipped,
				"bytes":              report.Bytes,
				"truncated":          walk.Result.Truncated,
				"lfs_pointers":       report.LFSPointers,
				"skipped_symlinks":   walk.Result.SkippedSymlinks,
				"skipped_submodules": walk.Result.SkippedSubmodules,
				"fetch_failures":     report.Failures,
				"api_requests":       walk.Result.APIRequests,
			}
			if err := printJSONFiltered(cmd.OutOrStdout(), envelope, flags); err != nil {
				return err
			}
			if len(report.Failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d downloads failed\n", len(report.Failures), len(files))
				return apiErr(fmt.Errorf("%d of %d downloads failed", len(report.Failures), len(files)))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagOut, "out", "", "Local destination directory (default: basename of path, or repo name)")
	cmd.Flags().StringVar(&flagRef, "ref", "", "Branch, tag, or commit SHA (overrides #ref in the target)")
	cmd.Flags().StringSliceVar(&flagInclude, "include", nil, "Glob pattern(s) to include (repeatable; empty = all)")
	cmd.Flags().StringSliceVar(&flagExclude, "exclude", nil, "Glob pattern(s) to exclude (repeatable)")
	cmd.Flags().BoolVar(&flagFlatten, "flatten", false, "Write all files directly into --out, without subfolders")
	cmd.Flags().BoolVar(&flagForce, "force", false, "Re-download files even if a local copy already matches by blob SHA")
	cmd.Flags().IntVar(&flagConcurrency, "concurrency", 8, "Number of concurrent downloads")
	cmd.Flags().IntVar(&flagMaxFiles, "max-files", 0, "Maximum number of files to download (0 = unlimited)")

	return cmd
}
