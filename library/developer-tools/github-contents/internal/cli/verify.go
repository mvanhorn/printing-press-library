// Copyright 2026 Rick van de Laar and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written command: verify. See internal/ghfetch for the underlying
// address-parsing/tree-walk/blob-SHA logic.

package cli

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/github-contents/internal/ghfetch"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelVerifyCmd(flags *rootFlags) *cobra.Command {
	var (
		flagRef     string
		flagInclude []string
		flagExclude []string
	)

	cmd := &cobra.Command{
		Use:   "verify <local-dir> <owner/repo[/path][#ref]>",
		Short: "Check whether a previously downloaded directory still matches the remote at a ref — match/changed/missing/extra per file",
		Long: "Check whether a previously downloaded directory still matches the remote at a ref — match/changed/missing/extra per file — without re-downloading anything.\n" +
			"Use this command to check whether a previously downloaded directory matches the remote at a ref. " +
			"Do NOT use it to download the differences; use 'sync-dir' instead.",
		Example: "  github-contents-pp-cli verify ./books mjwoon/AI-readings/books --agent",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "dir=/tmp/pp-ghc-dogfood/fetch;target=octocat/Hello-World",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("both <local-dir> and <owner/repo[/path][#ref]> are required\nUsage: %s <local-dir> <owner/repo[/path][#ref]>", cmd.CommandPath()))
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare <local-dir> against the remote target and report matched/changed/missing/extra")
				return nil
			}
			localDir, target := args[0], args[1]
			if _, statErr := os.Stat(localDir); statErr != nil {
				return usageErr(fmt.Errorf("local directory %q not found: %w", localDir, statErr))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			walk, err := walkGHTarget(ctx, cmd, flags, target, flagRef, flagInclude, flagExclude)
			if err != nil {
				return err
			}
			addr := walk.Addr

			diff, err := diffLocalDir(localDir, addr, walk.Files, flagInclude, flagExclude)
			if err != nil {
				return apiErr(fmt.Errorf("reading local directory %q: %w", localDir, err))
			}

			changed := relPaths(diff.Changed)
			missing := relPaths(diff.Missing)
			// Unsafe remote paths (traversal / drive-or-stream syntax) mean
			// the local dir CANNOT faithfully mirror the remote — a fetch
			// would refuse to write them — so they always fail the match.
			ok := len(changed) == 0 && len(missing) == 0 && len(diff.Extra) == 0 && len(diff.Unsafe) == 0
			checked := len(diff.Matched) + len(diff.Changed) + len(diff.Missing)

			// This is a report command: it always returns nil (exit 0)
			// regardless of ok, since a real diff is a successful, complete
			// answer to "does this match" — not a command failure.
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "%s vs %s @ %s\n",
					ghfetch.SanitizeTerminal(localDir), ghfetch.SanitizeTerminal(target), ghfetch.SanitizeTerminal(addr.Ref))
				fmt.Fprintf(cmd.OutOrStdout(), "matched=%d changed=%d missing=%d extra=%d unsafe=%d checked=%d ok=%t\n",
					len(diff.Matched), len(changed), len(missing), len(diff.Extra), len(diff.Unsafe), checked, ok)
				return nil
			}
			envelope := map[string]any{
				"ok":      ok,
				"matched": len(diff.Matched),
				"changed": changed,
				"missing": missing,
				"extra":   diff.Extra,
				"unsafe":  diff.Unsafe,
				"checked": checked,
			}
			return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
		},
	}

	cmd.Flags().StringVar(&flagRef, "ref", "", "Branch, tag, or commit SHA (overrides #ref in the target)")
	cmd.Flags().StringSliceVar(&flagInclude, "include", nil, "Glob pattern(s) to include (repeatable; empty = all)")
	cmd.Flags().StringSliceVar(&flagExclude, "exclude", nil, "Glob pattern(s) to exclude (repeatable)")
	return cmd
}

// relPaths extracts the Rel field from a slice of diffEntry, for JSON
// output that only needs the path (not the full remote TreeFile).
func relPaths(entries []diffEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Rel)
	}
	return out
}
