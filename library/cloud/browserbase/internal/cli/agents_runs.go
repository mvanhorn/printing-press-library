// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command. Implemented body; generate --force preserves this file.
// pp:data-source live

package cli

import (
	"github.com/spf13/cobra"
)

// dogfoodFixtureRunID is a real run on the dogfood account used by
// pp:happy-args annotations so the live matrix exercises run-scoped commands
// with a valid ID instead of the spec's placeholder UUID.
const dogfoodFixtureRunID = "52f6b13d-eb27-436d-86ff-356b2fd01697"

func newNovelAgentsRunsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:     "runs",
		Short:   "List, get, create, and stop agent runs, plus structural diffing",
		Example: "  browserbase-pp-cli agents runs diff 52f6b13d-eb27-436d-86ff-356b2fd01697 2d310606-42fa-483c-9a7b-7102a85ddb09 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelAgentsRunsDiffCmd(flags))
	return cmd
}

func init() {
	// The flat generated run commands (agents runs-get / runs-list /
	// runs-messages / runs-stop / runs-create) are registered directly in
	// agents.go and remain the canonical run surface. Point the run-scoped
	// ones at a real fixture run ID so live dogfood happy-path probes don't
	// 404 on the spec's placeholder UUID.
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		for _, flat := range []string{"runs-get", "runs-messages"} {
			cmd, _, err := root.Find([]string{"agents", flat})
			if err != nil || cmd == nil {
				continue
			}
			if cmd.Annotations == nil {
				cmd.Annotations = map[string]string{}
			}
			cmd.Annotations["pp:happy-args"] = "runId=" + dogfoodFixtureRunID
		}
		// The generated `feedback` parent has no Examples section, which the
		// dogfood help check requires, and accepts any free-form text (so it
		// has no meaningful error path for invalid input). Add both
		// non-invasively.
		if fb, _, err := root.Find([]string{"feedback"}); err == nil && fb != nil {
			if fb.Example == "" {
				fb.Example = "  browserbase-pp-cli feedback list\n  browserbase-pp-cli feedback \"found a bug in sessions run\""
			}
			if fb.Annotations == nil {
				fb.Annotations = map[string]string{}
			}
			fb.Annotations["pp:no-error-path-probe"] = "true"
		}
	})
}
