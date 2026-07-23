//go:build !windows

// Copyright 2026 Kent Martin and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored companion to creds_perms_heal_windows.go.

package cliutil

// CredsPermsIssueIsInheritedOnly always returns false on Unix: mode bits are
// explicit state, so an over-permissive file is deliberate (or a copy the
// operator must chmod back) and the fail-closed refusal stands.
func CredsPermsIssueIsInheritedOnly(string) bool { return false }
