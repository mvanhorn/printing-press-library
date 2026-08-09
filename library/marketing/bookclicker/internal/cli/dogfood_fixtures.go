// Copyright 2026 wmiles81 and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: supply live-dogfood fixtures for framework commands that the
// fixture resolver cannot fill on its own. Kept in its own file so
// `generate --force` preserves it.
//
// Why this is needed: resolveCommandPositionals requires a command path with at
// least one more segment than it has positional placeholders, because it walks
// the sibling chain to source a real id. A top-level command that takes a
// positional — `search <query>` — has one segment and one placeholder, so it is
// skipped with "command path [search] has fewer segments than placeholders",
// and its happy-path and json-fidelity probes never run. The promote gate then
// refuses the CLI for "hollow coverage".
//
// An explicit pp:happy-args annotation short-circuits that resolution. The
// positional must be angle-bracketed: parseHappyArgsAnnotation discards a bare
// `query=...`, which would leave the fixture empty and change nothing.

package cli

import "github.com/spf13/cobra"

// dogfoodFixtures maps a command path to the happy-args annotation it needs.
// Values are real queries against synced Bookclicker data, not placeholders, so
// the probe exercises the command rather than proving it can reject nonsense.
var dogfoodFixtures = map[string]string{
	"search": "<query>=romance",
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		for path, args := range dogfoodFixtures {
			cmd, _, err := root.Find([]string{path})
			if err != nil || cmd == nil || cmd.Name() != path {
				continue
			}
			if cmd.Annotations == nil {
				cmd.Annotations = map[string]string{}
			}
			// Never clobber an annotation the generator supplied; a spec-declared
			// fixture is more authoritative than this fallback.
			if _, ok := cmd.Annotations["pp:happy-args"]; !ok {
				cmd.Annotations["pp:happy-args"] = args
			}
		}
	})
}
