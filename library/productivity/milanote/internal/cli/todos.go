// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelTodosCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "todos",
		Short:       "Add, tick and find to-do lists across boards",
		Example:     "  milanote-pp-cli todos open --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:parent-group": "true", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelTodosOpenCmd(flags))
	return cmd
}
