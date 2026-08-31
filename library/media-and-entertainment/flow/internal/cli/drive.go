// Copyright 2026 github-actionsbot and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelDriveCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "drive",
		Short: "Drive ingestion",
		// The live dogfood matrix tests every command's own declared Example
		// field independently of its children's pp:happy-args overrides, so
		// a parent Example showing an illustrative (non-existent) path fails
		// as a real invocation even though "drive import" itself is
		// correctly annotated and passing. This mirrors the child's own
		// pp:happy-args fixture with an explicit --dry-run so it's both a
		// real working example and safe (and valid JSON) to actually
		// execute, including with --json appended.
		Example:     "  flow-pp-cli drive import --folder-id testdata/dogfood-fixtures/images --tag-scene --project a1b2c3d4-e5f6-47a8-9b0c-1d2e3f4a5b6c --dry-run",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelDriveImportCmd(flags))
	return cmd
}
