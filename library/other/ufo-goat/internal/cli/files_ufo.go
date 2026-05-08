// Custom files parent command that wires the UFO-specific subcommands.
package cli

import (
	"github.com/spf13/cobra"
)

func newUFOFilesParentCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "files",
		Short: "Declassified UAP files (PDFs, videos, images) from FBI, DoD, NASA, State Department",
	}

	cmd.AddCommand(newUFOFilesGetCmd(flags))
	cmd.AddCommand(newUFOFilesListCmd(flags))
	cmd.AddCommand(newUFOFilesSearchCmd(flags))
	return cmd
}

// newUFOSearchShortcutCmd creates a top-level "search" command that delegates
// to the files search functionality without needing "files search".
func newUFOSearchShortcutCmd(flags *rootFlags) *cobra.Command {
	cmd := newUFOFilesSearchCmd(flags)
	cmd.Use = "search <query>"
	cmd.Short = "Search across all declassified UAP files (shortcut for 'files search')"
	return cmd
}
