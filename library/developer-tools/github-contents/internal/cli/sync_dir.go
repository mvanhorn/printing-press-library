// Copyright 2026 Rick van de Laar and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written command: sync-dir. See internal/ghfetch for the underlying
// address-parsing/tree-walk/download logic.

package cli

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/github-contents/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/github-contents/internal/ghfetch"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelSyncDirCmd(flags *rootFlags) *cobra.Command {
	var (
		flagRef         string
		flagInclude     []string
		flagExclude     []string
		flagConcurrency int
		flagForce       bool
	)

	cmd := &cobra.Command{
		Use:   "sync-dir <local-dir> <owner/repo[/path][#ref]>",
		Short: "Update an existing downloaded directory in place, fetching only files that changed upstream or are missing locally",
		Long: "Update an existing downloaded directory in place, fetching only files that changed upstream or are missing locally.\n" +
			"Use this command to update an existing downloaded directory in place, fetching only changed or new files. " +
			"Do NOT use it for a first-time download into an empty directory; use 'fetch' instead.",
		Example:     "  github-contents-pp-cli sync-dir ./books mjwoon/AI-readings/books --agent",
		Annotations: map[string]string{"pp:happy-args": "dir=/tmp/pp-ghc-dogfood/fetch;target=octocat/Hello-World"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("both <local-dir> and <owner/repo[/path][#ref]> are required\nUsage: %s <local-dir> <owner/repo[/path][#ref]>", cmd.CommandPath()))
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch only changed or missing files into <local-dir>")
				return nil
			}
			localDir, target := args[0], args[1]
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would sync %s from %s\n", localDir, target)
				return nil
			}
			if _, statErr := os.Stat(localDir); statErr != nil {
				return usageErr(fmt.Errorf("local directory %q not found: %w\nhint: use 'fetch %s --out %s' for a first-time download", localDir, statErr, target, localDir))
			}

			// Walk/API phase: --timeout-bounded (the default 60s is right
			// for the 1-2 JSON calls a walk makes).
			walkCtx, cancelWalk := boundCtx(cmd.Context(), flags)
			defer cancelWalk()

			walk, err := walkGHTarget(walkCtx, cmd, flags, target, flagRef, flagInclude, flagExclude)
			if err != nil {
				return err
			}
			addr := walk.Addr

			diff, err := diffLocalDir(localDir, addr, walk.Files, flagInclude, flagExclude)
			if err != nil {
				return apiErr(fmt.Errorf("reading local directory %q: %w", localDir, err))
			}

			// attempted is the set of diff entries actually handed to the
			// downloader — updated/added below are computed from THIS set
			// (minus failures), so a dogfood-mode truncation can never
			// report phantom successes for entries that were never tried.
			attempted := make([]diffEntry, 0, len(diff.Changed)+len(diff.Missing))
			attempted = append(attempted, diff.Changed...)
			attempted = append(attempted, diff.Missing...)
			if cliutil.IsDogfoodEnv() && len(attempted) > 5 {
				attempted = attempted[:5]
			}
			toFetch := make([]ghfetch.TreeFile, 0, len(attempted))
			for _, e := range attempted {
				toFetch = append(toFetch, e.File)
			}

			dl := &ghfetch.Downloader{
				Concurrency: flagConcurrency,
				Force:       flagForce,
				Limiter:     cliutil.NewAdaptiveLimiterAuto(4),
				Token:       walk.Client.Config.AuthHeader(),
				APIClient:   walk.Client,
			}
			// Download phase: exempt from the DEFAULT --timeout (60s would
			// kill a large re-sync mid-stream); see downloadPhaseCtx.
			dlCtx, cancelDl := downloadPhaseCtx(cmd, flags)
			defer cancelDl()
			report, dlErr := dl.Download(dlCtx, addr, toFetch, localDir)
			if dlErr != nil {
				return apiErr(dlErr)
			}

			failedRel := make(map[string]bool, len(report.Failures))
			for _, f := range report.Failures {
				failedRel[f.Path] = true
			}
			changedRel := make(map[string]bool, len(diff.Changed))
			for _, e := range diff.Changed {
				changedRel[e.Rel] = true
			}
			updated, added := 0, 0
			for _, e := range attempted {
				if failedRel[e.Rel] {
					continue
				}
				if changedRel[e.Rel] {
					updated++
				} else {
					added++
				}
			}

			envelope := map[string]any{
				"updated":        updated,
				"added":          added,
				"unchanged":      len(diff.Matched),
				"unsafe":         diff.Unsafe,
				"bytes":          report.Bytes,
				"fetch_failures": report.Failures,
			}
			if err := printJSONFiltered(cmd.OutOrStdout(), envelope, flags); err != nil {
				return err
			}
			if len(report.Failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d downloads failed\n", len(report.Failures), len(toFetch))
				return apiErr(fmt.Errorf("%d of %d downloads failed", len(report.Failures), len(toFetch)))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagRef, "ref", "", "Branch, tag, or commit SHA (overrides #ref in the target)")
	cmd.Flags().StringSliceVar(&flagInclude, "include", nil, "Glob pattern(s) to include (repeatable; empty = all)")
	cmd.Flags().StringSliceVar(&flagExclude, "exclude", nil, "Glob pattern(s) to exclude (repeatable)")
	cmd.Flags().IntVar(&flagConcurrency, "concurrency", 8, "Number of concurrent downloads")
	cmd.Flags().BoolVar(&flagForce, "force", false, "Re-download files even if a local copy already matches by blob SHA")
	return cmd
}
