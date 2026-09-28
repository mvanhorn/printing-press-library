package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

// Domain diagnostics remain separate from usable result envelopes, including
// partial refresh output. Human invocations retain Cobra's ordinary errors.
func tripMachineDiagnostic(cmd *cobra.Command, flags *rootFlags, err error) {
	if err == nil || !(flags.agent || flags.asJSON) {
		return
	}
	cmd.SilenceErrors = true
	code := ExitCode(err)
	if isCobraUsageError(err) {
		code = 2
	}
	encoder := json.NewEncoder(cmd.ErrOrStderr())
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(map[string]any{"error": err.Error(), "code": code})
}

func wrapTripMachineErrors(cmd *cobra.Command, flags *rootFlags) {
	if args := cmd.Args; args != nil {
		cmd.Args = func(cmd *cobra.Command, values []string) error {
			err := args(cmd, values)
			tripMachineDiagnostic(cmd, flags, err)
			return err
		}
	}
	if run := cmd.RunE; run != nil {
		cmd.RunE = func(cmd *cobra.Command, values []string) error {
			err := run(cmd, values)
			tripMachineDiagnostic(cmd, flags, err)
			return err
		}
	}
	for _, child := range cmd.Commands() {
		wrapTripMachineErrors(child, flags)
	}
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		for _, cmd := range root.Commands() {
			switch cmd.Name() {
			case "find", "show", "areas", "cuisines", "lists":
				wrapTripMachineErrors(cmd, flags)
			}
		}
	})
}
