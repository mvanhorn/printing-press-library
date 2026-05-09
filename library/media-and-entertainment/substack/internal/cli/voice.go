// Phase 3 hand-authored novel command. Not generator-emitted.

package cli

import "github.com/spf13/cobra"

func newVoiceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "voice",
		Short: "Mechanical voice fingerprint — sentence length, em-dash rate, hook patterns",
	}
	cmd.AddCommand(newVoiceFingerprintCmd(flags))
	return cmd
}
