// Copyright 2026 github-actionsbot and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelScriptCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "script",
		Short:       "Audio-drama pipeline",
		Example:     "  flow-pp-cli script draft-prompts recap_script.json --images-dir ./seed-images --out episode3-queue.json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelScriptDraftPromptsCmd(flags))
	return cmd
}
