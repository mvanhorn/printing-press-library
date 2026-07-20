// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
// Novel command: security posture report. Consolidates the IP blocklist,
// blacklisted caller numbers, firewall state, event-log signals, and the
// admin audit log from the local mirror into one attack-surface summary.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type postureReport struct {
	IPBlocklistCount    int             `json:"ip_blocklist_count"`
	BlacklistedNumbers  int             `json:"blacklisted_numbers"`
	EventLogCount       int             `json:"event_log_count"`
	SecuritySignalCount int             `json:"security_signal_count"`
	AuditLogCount       int             `json:"audit_log_count"`
	FirewallState       json.RawMessage `json:"firewall_state,omitempty"`
	SecuritySignals     []string        `json:"security_signals"`
	Note                string          `json:"note,omitempty"`
}

// securityKeywords flag event-log entries that look security-relevant. Matched
// case-insensitively against the flattened JSON of each event.
var securityKeywords = []string{"fail", "denied", "blacklist", "blocklist", "attack", "unauthorized", "ban", "reject", "intrusion", "brute"}

// countSecuritySignals returns how many event objects contain a security
// keyword anywhere in their JSON, plus a small sample of matched messages.
func countSecuritySignals(events []json.RawMessage) (int, []string) {
	count := 0
	sample := []string{}
	for _, e := range events {
		low := strings.ToLower(string(e))
		matched := false
		for _, kw := range securityKeywords {
			if strings.Contains(low, kw) {
				matched = true
				break
			}
		}
		if matched {
			count++
			if len(sample) < 10 {
				var obj map[string]json.RawMessage
				_ = json.Unmarshal(e, &obj)
				msg := firstString(obj, "Message", "Description", "Text", "Source")
				if msg == "" {
					msg = truncate(string(e), 120)
				}
				sample = append(sample, msg)
			}
		}
	}
	return count, sample
}

func newNovelPostureCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "posture",
		Short: "Consolidated security/attack-surface report (blocklist, blacklist, firewall, events, audit)",
		Long: "Aggregate the IP blocklist, blacklisted caller numbers, firewall state, security-relevant\n" +
			"event-log signals, and the admin audit log from the local mirror into one report.\n\n" +
			"Use this command for the consolidated security/attack-surface report. For raw\n" +
			"severity-filtered event rows use the event-logs list command.\n\n" +
			"Sync first: 3cx-xapi-pp-cli sync --resources blocklist,black-list-numbers,firewall,event-logs,report-audit-log",
		Example:     "  3cx-xapi-pp-cli posture --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would build a security posture report from the local mirror")
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
			if !hintIfUnsynced(cmd, db, rtEventLogs) {
				hintIfStale(cmd, db, rtEventLogs, flags.maxAge)
			}

			blocklist, _ := db.List(rtBlocklist, 0)
			blacklist, _ := db.List(rtBlackList, 0)
			events, _ := db.List(rtEventLogs, 0)
			audit, _ := db.List(rtReportAudit, 0)
			firewall, _ := db.List(rtFirewall, 0)

			sigCount, sigSample := countSecuritySignals(events)
			report := postureReport{
				IPBlocklistCount:    len(blocklist),
				BlacklistedNumbers:  len(blacklist),
				EventLogCount:       len(events),
				SecuritySignalCount: sigCount,
				AuditLogCount:       len(audit),
				SecuritySignals:     sigSample,
			}
			if len(firewall) > 0 {
				report.FirewallState = firewall[0]
			}
			total := report.IPBlocklistCount + report.BlacklistedNumbers + report.EventLogCount + report.AuditLogCount + len(firewall)
			if total == 0 {
				report.Note = "no security data in the local mirror; run sync --resources blocklist,black-list-numbers,firewall,event-logs,report-audit-log"
			}

			if machineOut(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintln(w, bold("Security posture"))
			fmt.Fprintf(w, "  IP blocklist entries:   %d\n", report.IPBlocklistCount)
			fmt.Fprintf(w, "  blacklisted numbers:    %d\n", report.BlacklistedNumbers)
			fmt.Fprintf(w, "  event-log entries:      %d\n", report.EventLogCount)
			fmt.Fprintf(w, "  security signals:       %s\n", func() string {
				if sigCount > 0 {
					return red(fmt.Sprintf("%d", sigCount))
				}
				return green("0")
			}())
			fmt.Fprintf(w, "  admin audit entries:    %d\n", report.AuditLogCount)
			for _, s := range sigSample {
				fmt.Fprintf(w, "    - %s\n", truncate(s, 100))
			}
			if report.Note != "" {
				fmt.Fprintln(w, report.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}
