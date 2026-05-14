// Copyright 2026 alex-puckhaber. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// PushPress gap-stub commands — per the user briefing protocol:
//   "flag the gaps — do NOT silently skip or fake the data."
//
// PushPress maintains two parallel APIs:
//   - /v3 — the public, documented Platform API (20 endpoints; what this CLI ships)
//   - /v2 — the internal dashboard API (rich; consumed by app.pushpress.com SPA)
//
// The commands in this file correspond to dashboard features that live in /v2
// but have NO /v3 counterpart. They ship as Cobra commands so an agent reading
// `--help` knows the feature category EXISTS as a concept and is being
// deliberately surfaced — not silently dropped. Every stub returns exit code 6
// (typed exit: feature gated on follow-up) and explains where to unblock it.
//
// To un-stub: run the documented /v2 browser-sniff follow-up (see this run's
// $DISCOVERY_DIR/v2-paths.md for the enumerated 52 /v2 paths captured from the
// dashboard SPA bundle).

const (
	// Typed exit for stub commands. Distinct from auth errors (4) and
	// not-found (3) so an agent scripting around this CLI can branch on it.
	stubExitGated = 6
)

// stubGapMsg returns the standard error message every stub emits. Keeps the
// wording uniform so an agent regex-matching on output gets one consistent
// signal.
func stubGapMsg(cmdPath, neededEndpoint, followUp string) string {
	return fmt.Sprintf(
		"`%s` is not supported by PushPress /v3.\n\n"+
			"What's missing: %s\n"+
			"Follow-up: %s\n\n"+
			"This command exists as a placeholder per the gap-flag protocol so agents reading --help know the\n"+
			"feature category is recognized but not yet wired. Re-run the press with /v2 browser-sniff to enable it.",
		cmdPath, neededEndpoint, followUp,
	)
}

// runStub is the shared body for every gap-stub command. It prints the gap
// message to stderr and returns a cliError carrying exit code 6, which the
// generator's ExitCode() in root.go routes to os.Exit().
func runStub(cmd *cobra.Command, fullPath, neededEndpoint, followUp string) error {
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	fmt.Fprintln(cmd.ErrOrStderr(), stubGapMsg(fullPath, neededEndpoint, followUp))
	return &cliError{code: stubExitGated, err: fmt.Errorf("`%s` not supported by /v3 — see message above", fullPath)}
}

// ---------- plans (S1, S2, S3) ----------

func newPlansStubCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "plans",
		Short:       "Membership plans — /v2-gated; run plans <subcommand> --help for the specific gap",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(&cobra.Command{
		Use:                "list",
		Short:              "List all membership plans (NOT IN /v3 — /v2/plans)",
		Annotations:        map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "6"},
		DisableFlagParsing: false,
		RunE: func(c *cobra.Command, args []string) error {
			return runStub(c, "plans list",
				"/v3 has no plans endpoint family. The dashboard's plan catalog lives at /v2/plans.",
				"Browser-sniff app.pushpress.com (logged-in), capture the Settings → Plans request, then regenerate.",
			)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:         "members <plan-id>",
		Short:       "List members on a specific plan (NOT IN /v3 — /v2/plans + /v2/client)",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "6"},
		RunE: func(c *cobra.Command, args []string) error {
			return runStub(c, "plans members <plan-id>",
				"/v3 doesn't expose plan-to-member relationship. Dashboard uses /v2/plans + /v2/client filtered by planId.",
				"Same /v2 browser-sniff path as `plans list`.",
			)
		},
	})
	return cmd
}

// ---------- mrr (S3) ----------

func newMRRStubCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "mrr",
		Short:       "MRR / ARR snapshots — /v2-gated",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(&cobra.Command{
		Use:         "today",
		Short:       "Monthly recurring revenue snapshot for today (NOT IN /v3 — /v2/billing, /v2/subscription)",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "6"},
		RunE: func(c *cobra.Command, args []string) error {
			return runStub(c, "mrr today",
				"/v3 has no billing or subscription endpoints. MRR lives at /v2/billing + /v2/subscription.",
				"Browser-sniff app.pushpress.com Billing tab, capture the MRR dashboard call.",
			)
		},
	})
	return cmd
}

// ---------- signups (S4) ----------

func newSignupsStubCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "signups",
		Short:       "Recent signups — /v3 lacks the source-attribution field",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(&cobra.Command{
		Use:         "recent",
		Short:       "Recent signups (NOT IN /v3 — /v3 Customer schema has no `dateAdded` field)",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "6"},
		RunE: func(c *cobra.Command, args []string) error {
			return runStub(c, "signups recent",
				"PushPress /v3 Customer schema exposes id/name/email/phone/address/profileImage only. No dateAdded, no source attribution.",
				"Browser-sniff the dashboard's Activity feed (/v2/activity) to capture signups with source labels.",
			)
		},
	})
	return cmd
}

// ---------- cancellations (S5) ----------

func newCancellationsStubCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "cancellations",
		Short:       "Recent cancellations + freeze-vs-cancel distinction — /v2-gated",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(&cobra.Command{
		Use:         "recent",
		Short:       "Recent cancellations with reason and freeze-vs-cancel split (NOT IN /v3)",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "6"},
		RunE: func(c *cobra.Command, args []string) error {
			return runStub(c, "cancellations recent",
				"/v3 has no cancellation, freeze, or subscription-status endpoint. PushPress publishes a Churn report on /v2/billing.",
				"Browser-sniff the dashboard Churn report; capture the underlying API call.",
			)
		},
	})
	return cmd
}

// ---------- classes (S6, S7) ----------

func newClassesStubCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "classes",
		Short:       "Classes / sessions — /v2-gated (use `class-mix` for the read-only signal /v3 does expose)",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(&cobra.Command{
		Use:         "list",
		Short:       "List class definitions (NOT IN /v3 — class names appear only inside check-in records)",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "6"},
		RunE: func(c *cobra.Command, args []string) error {
			return runStub(c, "classes list",
				"/v3 has no class-definition endpoint. The class name is embedded in ClassCheckin records — see `class-mix --help`.",
				"Browser-sniff /v2/calendar for class definitions, schedules, and instructor assignments.",
			)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:         "roster <class-id>",
		Short:       "List registrants for a specific class session (NOT IN /v3)",
		Example:     "  pushpress-pp-cli classes roster 550e8400-e29b-41d4-a716-446655440000",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "6"},
		RunE: func(c *cobra.Command, args []string) error {
			return runStub(c, "classes roster <class-id>",
				"/v3 has no class-roster endpoint. Dashboard reads /v2/calendar with the session id.",
				"Same /v2 browser-sniff path as `classes list`.",
			)
		},
	})
	return cmd
}

// ---------- leads (S8) ----------

func newLeadsStubCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "leads",
		Short:       "Leads (prospects, not yet members) — /v2-gated",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(&cobra.Command{
		Use:         "list",
		Short:       "List leads with source + conversion status (NOT IN /v3)",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "6"},
		RunE: func(c *cobra.Command, args []string) error {
			return runStub(c, "leads list",
				"/v3 treats every contact as a Customer with no lead/member distinction.",
				"Browser-sniff /v2/client filtered by lead status to get the lead pipeline.",
			)
		},
	})
	return cmd
}

// ---------- tasks (S9) ----------

func newTasksStubCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "tasks",
		Short:       "Staff tasks attached to members — /v2-gated",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(&cobra.Command{
		Use:         "list",
		Short:       "List staff tasks (NOT IN /v3 — /v2/task)",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "6"},
		RunE: func(c *cobra.Command, args []string) error {
			return runStub(c, "tasks list",
				"/v3 has no task surface. Dashboard task list lives at /v2/task.",
				"Browser-sniff /v2/task to capture the task model.",
			)
		},
	})
	return cmd
}

// ---------- notes (S10) ----------

func newNotesStubCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "notes",
		Short:       "Staff notes attached to members — /v2-gated",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(&cobra.Command{
		Use:         "list",
		Short:       "List recent staff notes (NOT IN /v3)",
		Example:     "  pushpress-pp-cli notes list",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "6"},
		RunE: func(c *cobra.Command, args []string) error {
			return runStub(c, "notes list",
				"/v3 has no notes surface. Dashboard notes live alongside /v2/task or /v2/communications.",
				"Browser-sniff /v2/communications to capture notes/comments.",
			)
		},
	})
	return cmd
}

// ---------- cohort (S11 — moved here from transcendence) ----------

func newCohortStubCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cohort",
		Short: "Signup-month retention curve (NOT IN /v3 — Customer schema has no dateAdded field)",
		Long: "Originally planned as a transcendence feature but moved to gap-stubs after discovering that PushPress /v3 " +
			"Customer schema does NOT expose `dateAdded` (only id/name/email/phone/address/profileImage). Cohort retention " +
			"requires a signup date and cannot be computed without it.",
		Example:     "  pushpress-pp-cli cohort --month 2026-04   # will print the /v2 follow-up note",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "6"},
		RunE: func(c *cobra.Command, args []string) error {
			return runStub(c, "cohort",
				"/v3 Customer schema lacks `dateAdded`. Cohort retention needs the signup date to bucket customers by month.",
				"Browser-sniff /v2/client (the dashboard's Customer model) — it exposes dateAdded.",
			)
		},
	}
	return cmd
}
