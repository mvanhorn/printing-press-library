// Copyright 2026 Matias Sanchez Moises and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

const version = "0.1.0"

type rootFlags struct {
	asJSON         bool
	compact        bool
	noColor        bool
	agent          bool
	libraryPath    string
	messagesDBPath string
}

func Execute() error {
	f := &rootFlags{}

	root := &cobra.Command{
		Use:   "icloud-pp-cli",
		Short: "Query your Apple iCloud data from the command line",
		Long: `icloud-pp-cli gives AI agents and power users direct access to Apple iCloud data
stored locally on macOS — Photos library storage analysis today, with Contacts
and more coming.

All reads are local. No network calls, no iCloud API token required.

Pipe any command for automatic JSON output.`,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if f.agent {
				f.asJSON = true
				f.compact = true
				f.noColor = true
			}
			return nil
		},
	}

	root.PersistentFlags().BoolVar(&f.asJSON, "json", false, "Output as JSON")
	root.PersistentFlags().BoolVar(&f.compact, "compact", false, "Minimal fields only")
	root.PersistentFlags().BoolVar(&f.noColor, "no-color", false, "Disable colored output")
	root.PersistentFlags().BoolVar(&f.agent, "agent", false, "Agent-friendly mode: --json --compact --no-color")
	root.PersistentFlags().StringVar(&f.libraryPath, "library", "", "Path to Photos.sqlite (default: ~/Pictures/Photos Library.photoslibrary/database/Photos.sqlite)")
	// PATCH(messages-db-flag-root-scope): registered at root so `doctor` and any
	// future sibling command can read the override. Previously scoped to the
	// `messages` group only, which made `doctor --messages-db ...` exit with
	// `unknown flag` despite the doctor body reading f.messagesDBPath.
	root.PersistentFlags().StringVar(&f.messagesDBPath, "messages-db", "", "Path to chat.db (default: ~/Library/Messages/chat.db)")

	root.AddCommand(newPhotosCmd(f))
	root.AddCommand(newMessagesCmd(f))
	root.AddCommand(newContactsCmd(f))
	root.AddCommand(newDoctorCmd(f))

	return root.Execute()
}

func ExitCode(err error) int {
	var ce *cliError
	if errors.As(err, &ce) {
		return ce.code
	}
	return 1
}
