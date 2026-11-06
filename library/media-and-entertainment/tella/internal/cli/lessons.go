// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelLessonsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "lessons",
		Short:       "Work with lessons",
		Example:     "  tella-pp-cli lessons pass vid_example --dry-run",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelLessonsPassCmd(flags))
	cmd.AddCommand(newNovelLessonsShipCmd(flags))
	return cmd
}
