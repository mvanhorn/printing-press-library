// Copyright 2026 jrimmer. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

// newIssueCmd creates the top-level 'issue' command group, mirroring gh's UX.
// Novel commands (dashboard, sweep) live here; generated issue CRUD is under 'repos issues'.
func newIssueCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "issue",
		Short: "Work with issues across repos",
		Long: `Work with Forgejo issues. For the full generated API surface see 'fj repos issues'.

Novel commands:
  fj issue dashboard   Cross-repo triage table of your open issues
  fj issue sweep       Find and close stale issues with --dry-run safety`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelIssueDashboardCmd(flags))
	cmd.AddCommand(newNovelIssueSweepCmd(flags))
	return cmd
}

// newPRCmd creates the top-level 'pr' command group.
func newPRCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pr",
		Short: "Work with pull requests",
		Long: `Work with Forgejo pull requests. For the full generated API surface see 'fj repos pulls'.

Novel commands:
  fj pr queue   PRs awaiting your review across all repos`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelPRQueueCmd(flags))
	return cmd
}

// newReleaseCmd creates the top-level 'release' command group.
func newReleaseCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Manage releases and release assets",
		Long: `Manage Forgejo releases. For the full generated API surface see 'fj repos releases'.

Novel commands:
  fj release changelog   Generate a changelog between two tags
  fj release upload      Upload assets with progress and retry`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelReleaseChangelogCmd(flags))
	cmd.AddCommand(newNovelReleaseUploadCmd(flags))
	return cmd
}

// newRunnerCmd creates the top-level 'runner' command group.
func newRunnerCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runner",
		Short: "Manage and monitor Actions runners",
		Long: `Manage Forgejo Actions runners. For per-scope runner commands see 'fj admin', 'fj orgs actions', or 'fj repos actions'.

Novel commands:
  fj runner sweep   Health sweep across all orgs with optional --watch`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelRunnerSweepCmd(flags))
	return cmd
}

// newNotificationCmd creates the top-level 'notification' command group (singular, matching gh).
func newNotificationCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notification",
		Short: "View and manage notifications",
		Long: `View and manage Forgejo notifications. For the full generated notification API see 'fj notifications'.

Novel commands:
  fj notification inbox   Unified inbox with --unread, --since, --limit`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelNotificationInboxCmd(flags))
	return cmd
}

// newCICmd creates the top-level 'ci' command group.
func newCICmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ci",
		Short: "CI/Actions check status",
		Long: `Monitor Forgejo Actions CI runs. For the full generated Actions API see 'fj repos actions'.

Novel commands:
  fj ci status   Pass/fail summary per repo with --watch`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelCIStatusCmd(flags))
	return cmd
}
