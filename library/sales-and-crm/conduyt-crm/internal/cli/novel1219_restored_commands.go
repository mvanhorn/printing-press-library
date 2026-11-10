// Copyright 2026 Paul Taramona and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import "github.com/spf13/cobra"

// PATCH(novel1219-restored-commands): the four commands the public library entry serves and the 4.34.2 reprint
// dropped (Greptile PR #2450 round 2 P1) are attached to their generated parent groups through the novel-command
// hook, so a reprint that regenerates reports.go, dialer.go and automations.go keeps them. Each parent is resolved
// with root.Find and each child attached with a direct addNovelCommandIfAbsent call: the library's static SKILL.md
// verifier reads exactly that shape, so the README examples for these commands resolve in CI.
func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		if reportsCmd, _, err := root.Find([]string{"reports"}); err == nil && reportsCmd != root && reportsCmd.Name() == "reports" {
			addNovelCommandIfAbsent(reportsCmd, newNovelReportsCompareCmd(flags))
			addNovelCommandIfAbsent(reportsCmd, newNovelReportsScorecardCmd(flags))
		}
		if dialerCmd, _, err := root.Find([]string{"dialer"}); err == nil && dialerCmd != root && dialerCmd.Name() == "dialer" {
			addNovelCommandIfAbsent(dialerCmd, newNovelDialerCoverageCmd(flags))
		}
		if automationsCmd, _, err := root.Find([]string{"automations"}); err == nil && automationsCmd != root && automationsCmd.Name() == "automations" {
			addNovelCommandIfAbsent(automationsCmd, newNovelAutomationsHoursAuditCmd(flags))
		}
	})
}
