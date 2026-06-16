// Copyright 2026 Nik and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

func emitBinaryPage(cmd *cobra.Command, flags *rootFlags, path string, data []byte) error {
	if flags.quiet {
		return nil
	}
	if flags.asJSON || flags.agent || flags.csv || flags.compact || flags.plain || flags.selectFields != "" {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"path":    path,
			"content": string(data),
			"bytes":   len(data),
		}, flags)
	}
	_, err := cmd.OutOrStdout().Write(data)
	return err
}
