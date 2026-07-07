// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

func printJSONValue(cmd *cobra.Command, out any) error {
	b, _ := json.Marshal(out)
	return printOutput(cmd.OutOrStdout(), b, true)
}
