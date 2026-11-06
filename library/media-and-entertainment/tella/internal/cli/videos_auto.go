// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import (
	"fmt"
	"github.com/spf13/cobra"
)

func newNovelVideosAutoCmd(flags *rootFlags) *cobra.Command {
	var o autoOptions
	var apply, layouts bool
	cmd := &cobra.Command{Use: "auto <video>", Short: "Plan automatic edits; --apply saves recovery state, edits and returns a preview", Args: workflowArgs(1), Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "live", "pp:preview-happy-path": "true", "pp:happy-args": "--fillers=true"}, Example: "  tella-pp-cli videos auto vid_example --fillers --silences natural --zooms --zoom-style subtle\n  tella-pp-cli videos auto vid_example --crop --apply"}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		if layouts {
			return usageErr(fmt.Errorf("no public auto-layout endpoint: use videos storyboard / videos apply-storyboard for a manual layout plan, or agent-chats start --yes"))
		}
		if err := o.validate(); err != nil {
			return usageErr(err)
		}
		if !o.Zooms && !o.Crop && !o.Fillers && o.Silences == "" {
			return usageErr(fmt.Errorf("select --zooms, --crop, --fillers or --silences"))
		}
		if workflowOffline(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "videos auto")
		}
		c, err := workflowClient(flags)
		if err != nil {
			return err
		}
		p, err := planAutomatics(cmd.Context(), c, args[0], o)
		if err != nil {
			return err
		}
		if flags.dryRun || !apply {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "plan": p, "recovery_limits": lessonRecoveryLimit}, flags)
		}
		if len(p.Plans) == 0 {
			return fmt.Errorf("video has no clips; no automatic edits can be applied")
		}
		r, err := saveAutoRecovery(p)
		if err != nil {
			return err
		}
		if err = applyAutomatics(cmd.Context(), c, p, r); err != nil {
			return workflowFailure(cmd, flags, r, err)
		}
		preview, err := createLessonPreview(cmd.Context(), c, args[0])
		if err != nil {
			return workflowFailure(cmd, flags, r, err)
		}
		if flags.asJSON || flags.selectFields != "" || flags.compact {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"applied": true, "recovery": r, "preview": preview}, flags)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Recovery: %s\n%s\n", r.State, r.Limits)
		return printPreview(cmd, flags, preview)
	}
	cmd.Flags().BoolVar(&o.Zooms, "zooms", false, "Generate tracking zooms, preserving existing zooms")
	cmd.Flags().StringVar(&o.ZoomStyle, "zoom-style", "", "gentle, subtle, moderate, focused, intense (slow/medium/fast aliases)")
	cmd.Flags().BoolVar(&o.Crop, "crop", false, "Apply detected screen crop")
	cmd.Flags().BoolVar(&layouts, "layouts", false, "Request layouts (unsupported; fails before any writes)")
	cmd.Flags().BoolVar(&o.Fillers, "fillers", false, "Remove filler words")
	cmd.Flags().StringVar(&o.Silences, "silences", "", "natural, fast or faster")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply planned edits; global --dry-run wins")
	return cmd
}
