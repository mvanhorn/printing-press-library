// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Wiring helpers for hand-authored (novel) commands. Each novel command
// file registers itself from its own init() through registerNovelCommand so
// a forced regeneration of root.go cannot drop it, and so no two extensions
// have to coordinate edits to the same generated file.

package cli

import "github.com/spf13/cobra"

// addNovelCommandIfAbsent attaches cmd to parent unless a command with the
// same name is already attached. Generated roots may already wire a novel
// command directly; the hook then becomes a no-op instead of registering a
// duplicate that would make Cobra's name resolution ambiguous.
func addNovelCommandIfAbsent(parent, cmd *cobra.Command) {
	if parent == nil || cmd == nil {
		return
	}
	for _, existing := range parent.Commands() {
		if existing.Name() == cmd.Name() {
			return
		}
	}
	parent.AddCommand(cmd)
}

// findNovelParent resolves an existing parent command by its path segments
// (e.g. {"users"}), returning nil when the parent is not registered.
func findNovelParent(root *cobra.Command, path []string) *cobra.Command {
	if root == nil {
		return nil
	}
	found, _, err := root.Find(path)
	if err != nil || found == nil || found == root {
		return nil
	}
	return found
}
