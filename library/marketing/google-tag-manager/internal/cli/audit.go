// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// audit: run a hygiene battery over a mirrored container.

package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

type auditReport struct {
	Snapshot string         `json:"snapshot"`
	Counts   map[string]int `json:"counts"`
	Findings []auditFinding `json:"findings"`
}

func newNovelAuditCmd(flags *rootFlags) *cobra.Command {
	var dbPath, container, failOn string

	cmd := &cobra.Command{
		Use:         "audit",
		Short:       "Run hygiene checks over a mirrored container",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3"},
		Long: `Audit a mirrored container for hygiene problems the GTM console cannot
surface: dead tags (no firing trigger), orphan triggers, unused variables,
tracking tags missing Consent Mode settings, paused tags, Custom HTML tags, and
tags firing on All Pages.

Exit code 3 is returned when findings at or above --fail-on are present
(use --fail-on high to gate a CI publish). Run 'pull' first.`,
		Example: `  # Audit the most recent snapshot
  google-tag-manager-pp-cli audit --json

  # Fail a CI step on any high-severity finding
  google-tag-manager-pp-cli audit --fail-on high`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would audit the mirrored container")
				return nil
			}
			if failOn != "" && failOn != "high" && failOn != "warning" {
				return usageErr(fmt.Errorf("--fail-on must be 'high' or 'warning', got %q", failOn))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			s, snaps, ok, err := gtmReadSnapshots(ctx, cmd, flags, gtmDBPath(dbPath))
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			defer s.Close()

			snap, err := resolveSnapshot(snaps, container)
			if err != nil {
				return usageErr(err)
			}
			entities, err := snapshotEntities(ctx, s.DB(), snap.ID, "")
			if err != nil {
				return err
			}
			findings := runAudit(entities)
			counts := map[string]int{"high": 0, "warning": 0, "info": 0}
			for _, f := range findings {
				counts[f.Severity]++
			}
			report := auditReport{Snapshot: snap.Label, Counts: counts, Findings: findings}

			if err := gtmEmit(cmd, flags, report, func(w io.Writer) {
				fmt.Fprintf(w, "Audit of %s — %d high, %d warning, %d info\n",
					snap.Label, counts["high"], counts["warning"], counts["info"])
				tw := newTabWriter(w)
				for _, f := range findings {
					fmt.Fprintf(tw, "  [%s]\t%s\t%s\t%s\n", f.Severity, f.Check, f.Entity, f.Message)
				}
				tw.Flush()
			}); err != nil {
				return err
			}

			switch failOn {
			case "high":
				if counts["high"] > 0 {
					return &cliError{code: 3, err: fmt.Errorf("%d high-severity finding(s)", counts["high"])}
				}
			case "warning":
				if counts["high"]+counts["warning"] > 0 {
					return &cliError{code: 3, err: fmt.Errorf("%d finding(s) at or above warning", counts["high"]+counts["warning"])}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&container, "snapshot", "", "Snapshot ref to audit: live, workspace:<id>, container:<id>, version:<id>, or a snapshot id (default: most recent in the active container)")
	cmd.Flags().StringVar(&failOn, "fail-on", "", "Exit 3 when findings at this severity exist: high | warning")
	return cmd
}
