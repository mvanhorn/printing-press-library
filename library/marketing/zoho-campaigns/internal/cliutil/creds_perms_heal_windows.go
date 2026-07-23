//go:build windows

// Copyright 2026 Kent Martin and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored companion to the credentials perms guard. Distinguishes
// "this file landed in a directory and inherited its ACEs" (an unintentional
// copy — safe to re-restrict and load) from "someone explicitly granted broad
// access" (deliberate broadening — the token may be compromised; refuse and
// re-mint, matching the generated fail-closed design).

package cliutil

import (
	"strings"

	"golang.org/x/sys/windows"
)

// CredsPermsIssueIsInheritedOnly reports whether the file's perms refusal is
// caused solely by inherited allow-ACEs. Explicit broad grants, foreign
// owners, or unparseable descriptors return false so the caller keeps the
// fail-closed refusal.
func CredsPermsIssueIsInheritedOnly(path string) bool {
	sd, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.OWNER_SECURITY_INFORMATION,
	)
	if err != nil {
		return false
	}
	me, err := currentUserSID()
	if err != nil {
		return false
	}
	sddl := sd.String()
	om := ownerRe.FindStringSubmatch(sddl)
	if om == nil || !ownerAllowed(om[1], me) {
		return false // foreign-owned file: never heal
	}
	return aclIssuesAreInheritedOnly(sddl, me)
}

// aclIssuesAreInheritedOnly walks the DACL's ACEs: every allow-ACE for a
// principal outside the allowed set (current user, Administrators, SYSTEM)
// must carry the inherited (ID) flag. Any explicit broad allow-ACE, or any
// unparseable ACE, disqualifies healing.
func aclIssuesAreInheritedOnly(sddl, currentUserSID string) bool {
	i := strings.Index(sddl, "D:")
	if i < 0 {
		return false
	}
	rest := sddl[i+2:]
	if strings.HasPrefix(rest, "NO_ACCESS_CONTROL") {
		return false
	}
	if j := strings.Index(rest, ")S:"); j >= 0 {
		rest = rest[:j+1]
	}
	for _, m := range aceRe.FindAllStringSubmatch(rest, -1) {
		fields := strings.Split(m[1], ";")
		if len(fields) < 6 {
			return false
		}
		aceType := strings.ToUpper(strings.TrimSpace(fields[0]))
		flags := strings.ToUpper(strings.TrimSpace(fields[1]))
		trustee := strings.ToUpper(strings.TrimSpace(fields[5]))
		if aceType != "A" {
			continue // deny ACEs cannot broaden access
		}
		if ownerAllowed(trustee, currentUserSID) {
			continue
		}
		if !strings.Contains(flags, "ID") {
			return false // explicit grant to a disallowed principal
		}
	}
	return true
}
