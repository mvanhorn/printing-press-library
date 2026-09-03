// PATCH(library): present the bounded OCR observation loop as intent-level commands.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/semantic"
	"github.com/spf13/cobra"
)

func init() { registerNovelCommand(registerOCRLoopCommands) }

// pp:data-source live
func registerOCRLoopCommands(root *cobra.Command, flags *rootFlags) {
	observe := &cobra.Command{
		Use:         "observe",
		Short:       "Capture a fresh screenshot and OCR observation",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"pp:novel": "true", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return dispatchOCRPresentation(cmd, flags, "observe", nil)
		},
	}

	verify := &cobra.Command{
		Use:         "verify",
		Short:       "Verify exact high-confidence text in a fresh observation",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"pp:novel": "true", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			expected, _ := cmd.Flags().GetString("expect-text")
			return dispatchOCRPresentation(cmd, flags, "verify-text", map[string]any{"text": expected})
		},
	}
	verify.Flags().String("expect-text", "", "exact text expected in the fresh OCR observation")
	_ = verify.MarkFlagRequired("expect-text")

	act := &cobra.Command{
		Use:         "act",
		Short:       "Perform an OCR-observation-gated KVM action",
		Annotations: map[string]string{"pp:novel": "true"},
	}
	act.AddCommand(newOCRClickTextCommand(flags), newOCRPressKeyCommand(flags))

	addNovelCommandIfAbsent(root, observe)
	addNovelCommandIfAbsent(root, verify)
	addNovelCommandIfAbsent(root, act)
}

func newOCRClickTextCommand(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "click-text <text>",
		Short:       "Click one exact high-confidence OCR text match",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"pp:novel": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			observation, err := requiredObservationFlag(cmd, flags)
			if err != nil {
				return err
			}
			return dispatchOCRPresentation(cmd, flags, "click-text", map[string]any{"text": args[0], "observation_id": observation, "write_enabled": true})
		},
	}
	cmd.Flags().String("observation", "", "observation ID returned by kvmctl observe")
	_ = cmd.MarkFlagRequired("observation")
	return cmd
}

func newOCRPressKeyCommand(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "press-key <key>",
		Short:       "Press an allowed key against one fresh OCR observation",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"pp:novel": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			observation, err := requiredObservationFlag(cmd, flags)
			if err != nil {
				return err
			}
			return dispatchOCRPresentation(cmd, flags, "press-key", map[string]any{"key": args[0], "observation_id": observation, "write_enabled": true})
		},
	}
	cmd.Flags().String("observation", "", "observation ID returned by kvmctl observe")
	_ = cmd.MarkFlagRequired("observation")
	return cmd
}

func requiredObservationFlag(cmd *cobra.Command, flags *rootFlags) (string, error) {
	if !flags.yes {
		return "", usageErr(fmt.Errorf("--yes is required for %s (write-gated)", cmd.CommandPath()))
	}
	observation, _ := cmd.Flags().GetString("observation")
	if strings.TrimSpace(observation) == "" {
		return "", usageErr(fmt.Errorf("--observation is required for %s", cmd.CommandPath()))
	}
	return observation, nil
}

func dispatchOCRPresentation(cmd *cobra.Command, flags *rootFlags, operation string, args map[string]any) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	out, err := semantic.Dispatch(cmd.Context(), c, operation, args)
	if err != nil {
		return err
	}
	return flags.printJSON(cmd, json.RawMessage(out))
}
