// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

// TestCommandSurface_Keyless pins the v2 command surface to the keyless
// set. v2 is a $0/no-key fork of v1; the inherited RapidAPI REST command
// subtree (publication / user / medium-unofficial-search / api / the
// promoted_* family) must not be registered, while the four Tier-0
// commands, the local-store novels, and the framework commands must
// remain. This guards the prune against silent regressions in either
// direction (a removed command sneaking back, or a kept command being
// dropped by accident).
func TestCommandSurface_Keyless(t *testing.T) {
	have := map[string]bool{}
	for _, c := range RootCmd().Commands() {
		have[c.Name()] = true
	}

	// Inherited RapidAPI surface — every one of these needs MEDIUM_API_KEY
	// at runtime and so has no place in the keyless v2 binary.
	removed := []string{
		"publication", "user", "medium-unofficial-search", "api",
		"archived-articles", "article", "latestposts", "list",
		"recommended-feed", "recommended-users", "related-tags",
		"root-tags", "tag", "top-writers", "topfeeds",
		// RapidAPI-bound framework commands removed for the keyless build.
		"sync", "tail", "import", "workflow",
	}
	for _, name := range removed {
		if have[name] {
			t.Errorf("command %q is still registered; the RapidAPI surface must be removed for the keyless v2 binary", name)
		}
	}

	// The keyless surface that must survive the prune.
	kept := []string{
		// Tier-0 direct-Medium commands.
		"feed", "read", "search",
		// Local-store novels.
		"author-archive", "author-compare", "corpus", "digest",
		// Framework.
		"doctor", "auth", "agent-context", "profile", "feedback",
		"which", "analytics", "version",
	}
	for _, name := range kept {
		if !have[name] {
			t.Errorf("command %q must remain registered in the keyless v2 surface", name)
		}
	}
}
