// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command parent: manifest. The check subcommand lives in
// manifest_check.go; the manifest file loader lives in internal/manifest.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelManifestCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "manifest",
		Short:       "manifest subcommands: check",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelManifestCheckCmd(flags))
	return cmd
}
