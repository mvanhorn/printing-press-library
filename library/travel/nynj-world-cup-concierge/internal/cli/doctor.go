// Copyright 2026 USER and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	opts := defaultSourceOptions()
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Validate that the public source pages and Concierge prompt API are reachable and that extraction returns the expected categories.",
		Example: `  nynj-world-cup-concierge-pp-cli doctor --agent
  nynj-world-cup-concierge-pp-cli doctor --pretty`,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := buildPayload(opts)
			if err != nil {
				_ = printJSONWithIndent(map[string]any{
					"status": "failed",
					"source": "nynj-world-cup-concierge",
					"error":  err.Error(),
				}, opts.Pretty && !opts.Agent)
				return exitError{code: 1}
			}
			status, code := doctorPayload(data)
			if err := printJSONWithIndent(status, opts.Pretty && !opts.Agent); err != nil {
				return err
			}
			if code != 0 {
				return exitError{code: code}
			}
			return nil
		},
	}
	bindCommonSourceFlags(cmd, &opts)
	return cmd
}
