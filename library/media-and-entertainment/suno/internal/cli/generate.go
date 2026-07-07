// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"time"

	"github.com/spf13/cobra"
)

// newGenerateCmd is the parent for music/lyrics/video generation. It is visible
// in the top-level --help because creating a track is the CLI's primary use case;
// every subcommand resolves under it.
func newGenerateCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Create tracks: music, lyrics, and video generation jobs",
		RunE:  parentNoSubcommandRunE(flags),
	}

	// Hand-authored, captcha-aware generation/transform commands.
	cmd.AddCommand(newSunoGenerateCreateCmd(flags))
	cmd.AddCommand(newSunoDescribeCmd(flags))
	cmd.AddCommand(newSunoExtendCmd(flags))
	cmd.AddCommand(newSunoCoverCmd(flags))
	cmd.AddCommand(newSunoRemasterCmd(flags))

	// Spec endpoint subcommands.
	cmd.AddCommand(newGenerateConcatCmd(flags))
	cmd.AddCommand(newGenerateLyricsCmd(flags))
	cmd.AddCommand(newGenerateLyricsStatusCmd(flags))
	cmd.AddCommand(newGenerateVideoStatusCmd(flags))

	// Adaptive captcha-gate retry, opt-in, inherited by every generation
	// subcommand. Suno gates generation adaptively (422 token_validation_failed);
	// the gate reopens after a cooldown (minutes to hours). With --wait-for-gate a
	// gated submit backs off and retries until it clears or --gate-timeout
	// elapses; off by default so scripted callers stay single-shot. These compose
	// with the piloted-Chrome auto-solver: the solver runs first and
	// --wait-for-gate rides out any residual gate, while under --no-captcha they
	// drive the passive, no-browser fallback on their own.
	cmd.PersistentFlags().Bool(flagWaitForGate, false, "On a captcha gate (422 token_validation_failed), back off and retry until the gate reopens or --gate-timeout elapses")
	cmd.PersistentFlags().Duration(flagGateTimeout, 30*time.Minute, "Maximum time to keep retrying when --wait-for-gate is set")
	return cmd
}
