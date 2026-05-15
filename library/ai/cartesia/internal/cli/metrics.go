// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

// newMetricsCmd registers the top-level `metrics` parent. Note: an
// `agents metrics` parent already exists nested under agents; this is a
// separate top-level command housing cross-cutting metric analytics
// (e.g. trend).
func newMetricsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "Aggregate analytics across metric results",
	}
	cmd.AddCommand(newMetricsTrendCmd(flags))
	return cmd
}
