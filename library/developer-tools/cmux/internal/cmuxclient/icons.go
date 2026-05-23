// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cmuxclient

import (
	"strings"
	"unicode"
)

// State is the canonical agent state derived from a status entry value and
// the surface title. See ecosystem-manager's cookbook for the original rule.
type State string

const (
	StateIdle     State = "idle"
	StateWorking  State = "working"
	StateAwaiting State = "awaiting"
	StateStranded State = "stranded"
	StateUnknown  State = "unknown"
)

// brailleSpinnerRunes is the set of braille glyphs cmux uses for the
// "actively working a turn" title icon.
var brailleSpinnerRunes = map[rune]bool{
	'⠐': true, '⠂': true, '⠈': true, '⠠': true, '⠄': true, '⠁': true, '⠆': true, '⠰': true,
	'⠘': true, '⠨': true, '⠌': true, '⠡': true,
}

// awaitingGlyphs are the title icons that mean "Claude has stopped and is
// awaiting input" (✻ ✶ ✳).
var awaitingGlyphs = map[rune]bool{
	'✻': true, '✶': true, '✳': true,
}

// IconState reads only the title's leading glyph and returns the canonical
// state. Returns StateUnknown when the title is empty or has no recognized
// icon.
func IconState(title string) State {
	for _, r := range title {
		if unicode.IsSpace(r) {
			continue
		}
		if brailleSpinnerRunes[r] {
			return StateWorking
		}
		if awaitingGlyphs[r] {
			return StateAwaiting
		}
		// First non-space rune is not a recognized icon — treat as no-icon
		// idle terminal pane.
		return StateIdle
	}
	return StateUnknown
}

// CanonicalState applies the cookbook's icon-priority rule: when the JSON
// statusEntry value and the title icon disagree, trust the icon. statusValue
// is the workspace-level statusEntry's `value` (e.g. "Running" or
// "Needs input"); titles is every surface title in the workspace.
//
// The `_strandedCount` argument is ignored for now: cmux's `in_window=false`
// flag indicates "this workspace is not the currently-displayed one in the
// active window" rather than the cookbook's "stranded surface" failure
// mode, so the per-workspace surface-health count is not a reliable signal
// for the canonical state column. Callers still get the raw stranded count
// from `surface-health` if they need it; we just don't fold it into the
// agent-state classification.
func CanonicalState(statusValue string, titles []string, _strandedCount int) State {
	// Title-icon priority: if any surface title encodes a state, that wins.
	hasWorking := false
	hasAwaiting := false
	for _, t := range titles {
		switch IconState(t) {
		case StateWorking:
			hasWorking = true
		case StateAwaiting:
			hasAwaiting = true
		}
	}
	if hasWorking {
		return StateWorking
	}
	if hasAwaiting {
		return StateAwaiting
	}
	// Fall back to status entry value.
	v := strings.TrimSpace(strings.ToLower(statusValue))
	switch v {
	case "running":
		return StateWorking
	case "needs input", "awaiting", "waiting":
		return StateAwaiting
	case "":
		return StateIdle
	}
	return StateUnknown
}
