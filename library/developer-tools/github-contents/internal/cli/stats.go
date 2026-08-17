// Copyright 2026 Rick van de Laar and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written command: stats. See internal/ghfetch for the underlying
// address-parsing/tree-walk logic.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/github-contents/internal/ghfetch"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelStatsCmd(flags *rootFlags) *cobra.Command {
	var (
		flagRef string
		flagTop int
	)

	cmd := &cobra.Command{
		Use:   "stats <owner/repo[/path][#ref]>",
		Short: "Size and file-count breakdown of any repo path by subfolder and extension, plus the largest files",
		Long: "Size and file-count breakdown of any repo path by subfolder and extension, plus the largest files, from a single API request.\n" +
			"Use this command for size and file-type breakdowns of a remote repo path. " +
			"Do NOT use it to preview a specific download's file list; use 'plan' instead.",
		Example: "  github-contents-pp-cli stats mjwoon/AI-readings/books --agent --select by_folder",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "target=octocat/Hello-World",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute size/file-count breakdowns by folder and extension for the target")
				return nil
			}
			if len(args) == 0 || args[0] == "" {
				return usageErr(fmt.Errorf("target is required\nUsage: %s <owner/repo[/path][#ref]>", cmd.CommandPath()))
			}
			target := args[0]
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			walk, err := walkGHTarget(ctx, cmd, flags, target, flagRef, nil, nil)
			if err != nil {
				return err
			}
			addr, result := walk.Addr, walk.Result

			stats := ghfetch.ComputeStats(result.Files, addr.Path, flagTop)

			envelope := map[string]any{
				"target":       target,
				"ref":          addr.Ref,
				"by_folder":    stats.ByFolder,
				"by_extension": stats.ByExtension,
				"largest":      stats.Largest,
				"total_files":  stats.TotalFiles,
				"total_bytes":  stats.TotalBytes,
				"human_total":  ghfetch.HumanBytes(stats.TotalBytes),
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "%s @ %s\n", ghfetch.SanitizeTerminal(target), ghfetch.SanitizeTerminal(addr.Ref))
				fmt.Fprintf(cmd.OutOrStdout(), "%d files, %s total\n\n", stats.TotalFiles, ghfetch.HumanBytes(stats.TotalBytes))
				fmt.Fprintln(cmd.OutOrStdout(), "By folder:")
				for _, fs := range stats.ByFolder {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-30s  %5d files  %10s\n", ghfetch.SanitizeTerminal(fs.Folder), fs.Files, ghfetch.HumanBytes(fs.Bytes))
				}
				fmt.Fprintln(cmd.OutOrStdout(), "\nBy extension:")
				for _, es := range stats.ByExtension {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-10s  %5d files  %10s\n", ghfetch.SanitizeTerminal(es.Ext), es.Files, ghfetch.HumanBytes(es.Bytes))
				}
				if len(stats.Largest) > 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "\nLargest files:")
					for _, fst := range stats.Largest {
						fmt.Fprintf(cmd.OutOrStdout(), "  %10s  %s\n", ghfetch.HumanBytes(fst.Size), ghfetch.SanitizeTerminal(fst.Path))
					}
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
		},
	}

	cmd.Flags().StringVar(&flagRef, "ref", "", "Branch, tag, or commit SHA (overrides #ref in the target)")
	cmd.Flags().IntVar(&flagTop, "top", 10, "Number of largest files to list")
	return cmd
}
