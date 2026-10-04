// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cobratree

import "strings"

// These optional learning reads intentionally open/migrate the ordinary
// operator store or record/prune local events. Keep their CLI behavior and
// current data or explicit store errors, with truthful MCP local-write hints.
func carstayLearningReadHasLocalEffects(path []string) bool {
	switch strings.Join(path, " ") {
	case "recall", "learnings list", "learnings candidates", "learnings stats", "playbook list":
		return true
	}
	return false
}
