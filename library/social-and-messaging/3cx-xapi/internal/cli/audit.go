// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
// Novel command: config integrity audit. Flags dangling references in the
// PBX config graph — ring-group members, queue agents, and inbound-rule
// destinations that point at numbers which are not live extensions/queues/
// ring groups. Pure local joins over the synced mirror.

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type auditReport struct {
	Findings       []auditFinding `json:"findings"`
	DanglingCount  int            `json:"dangling_count"`
	ScannedUsers   int            `json:"scanned_users"`
	ScannedRing    int            `json:"scanned_ring_groups"`
	ScannedQueues  int            `json:"scanned_queues"`
	ScannedInbound int            `json:"scanned_inbound_rules"`
	ValidDNCount   int            `json:"valid_dn_count"`
	Note           string         `json:"note,omitempty"`
}

func newNovelAuditCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Flag config references that point at deleted or non-existent extensions/queues/ring groups",
		Long: "Walk the synced config graph and report every ring-group member, queue agent, and\n" +
			"inbound-rule destination that references a number which is not a live extension,\n" +
			"queue, or ring group. Reads the local mirror only.\n\n" +
			"Use this command for graph-wide dangling-reference integrity. Do NOT use it for\n" +
			"time-based config drift (use 'diff') or for one extension's routing paths (use 'trace').\n\n" +
			"Sync first (expand members so memberships are checked):\n" +
			"  3cx-xapi-pp-cli sync --resources users,ring-groups,queues,inbound-rules,receptionists \\\n" +
			"    --resource-param 'ring-groups:$expand=Members' --resource-param 'queues:$expand=Agents'",
		Example:     "  3cx-xapi-pp-cli audit --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would audit the local config graph for dangling references")
				return nil
			}
			db, ok, err := openLocalMirror(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, rtUsers) {
				hintIfStale(cmd, db, rtUsers, flags.maxAge)
			}

			valid, err := dnNumberSet(db)
			if err != nil {
				return fmt.Errorf("building DN set: %w", err)
			}
			ringGroups, err := listObjects(db, rtRingGroups)
			if err != nil {
				return err
			}
			queues, err := listObjects(db, rtQueues)
			if err != nil {
				return err
			}
			inbound, err := listObjects(db, rtInboundRules)
			if err != nil {
				return err
			}
			users, err := listObjects(db, rtUsers)
			if err != nil {
				return err
			}

			membersMissing := hintMembersNotExpanded(cmd, ringGroups, queues)
			findings := findDanglingRefs(valid, ringGroups, queues, inbound)
			sort.SliceStable(findings, func(i, j int) bool {
				if findings[i].Kind != findings[j].Kind {
					return findings[i].Kind < findings[j].Kind
				}
				return findings[i].Ref < findings[j].Ref
			})
			if findings == nil {
				findings = []auditFinding{}
			}

			report := auditReport{
				Findings:       findings,
				DanglingCount:  len(findings),
				ScannedUsers:   len(users),
				ScannedRing:    len(ringGroups),
				ScannedQueues:  len(queues),
				ScannedInbound: len(inbound),
				ValidDNCount:   len(valid),
			}
			if len(valid) == 0 {
				report.Note = "no extensions/queues/ring groups in the local mirror; run sync first"
			} else if membersMissing {
				report.Note = "ring-group/queue memberships were NOT checked: re-sync with --resource-param 'ring-groups:$expand=Members' --resource-param 'queues:$expand=Agents' for a complete audit"
			} else if len(findings) == 0 {
				report.Note = "no dangling references found"
			}

			if machineOut(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Config integrity audit (%d valid DNs scanned)\n", report.ValidDNCount)
			fmt.Fprintf(w, "  ring groups: %d   queues: %d   inbound rules: %d\n\n", report.ScannedRing, report.ScannedQueues, report.ScannedInbound)
			if len(findings) == 0 {
				fmt.Fprintln(w, green("No dangling references found."))
				if report.Note != "" {
					fmt.Fprintln(w, report.Note)
				}
				return nil
			}
			fmt.Fprintf(w, "%s\n", red(fmt.Sprintf("%d dangling reference(s):", len(findings))))
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "KIND\tOWNER\tMISSING REF")
			for _, f := range findings {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", f.Kind, f.ObjectKey, f.Ref)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}
