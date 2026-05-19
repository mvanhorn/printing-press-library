// Copyright 2026 joltsconsulting. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/adminbyrequest/internal/store"
	"github.com/spf13/cobra"
)

type complianceRow struct {
	ID            string `json:"id"`
	RequestTime   string `json:"request_time"`
	Type          string `json:"type"`
	Status        string `json:"status"`
	User          string `json:"user"`
	Computer      string `json:"computer"`
	Application   string `json:"application"`
	Reason        string `json:"reason"`
	ApprovedBy    string `json:"approved_by"`
	DeniedBy      string `json:"denied_by"`
	DeniedReason  string `json:"denied_reason"`
	AuditLogLink  string `json:"auditlog_link"`
}

func newReportCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Render compliance reports from synced data",
	}
	cmd.AddCommand(newReportComplianceCmd(flags))
	return cmd
}

func newReportComplianceCmd(flags *rootFlags) *cobra.Command {
	var sinceStr, untilStr, userFilter, computerFilter, format, dbPath string
	cmd := &cobra.Command{
		Use:     "compliance",
		Short:   "Render audit log entries for a user or computer over a window",
		Long:    "Read locally-synced audit log entries inside the window and emit them as Markdown, CSV, or JSON. Filter by --user (matches AZUREAD\\X or full name) and/or --computer.",
		Example: "  adminbyrequest-pp-cli report compliance --since 2026-01-01 --user CHRISCOOMBES --format md",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			since, err := parseLooseDate(sinceStr)
			if err != nil {
				return fmt.Errorf("invalid --since %q: %w", sinceStr, err)
			}
			until := time.Now()
			if untilStr != "" {
				until, err = parseLooseDate(untilStr)
				if err != nil {
					return fmt.Errorf("invalid --until %q: %w", untilStr, err)
				}
			}
			if dbPath == "" {
				dbPath = defaultDBPath("adminbyrequest-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store at %s: %w (run sync first)", dbPath, err)
			}
			defer db.Close()

			where := []string{
				"COALESCE(request_time, start_time, '') >= ?",
				"COALESCE(request_time, start_time, '') <= ?",
			}
			argv := []interface{}{
				since.Format("2006-01-02T15:04:05"),
				until.Format("2006-01-02T15:04:05"),
			}
			if userFilter != "" {
				where = append(where, "(json_extract(data, '$.user.account') LIKE ? OR json_extract(data, '$.user.fullName') LIKE ?)")
				argv = append(argv, "%"+userFilter+"%", "%"+userFilter+"%")
			}
			if computerFilter != "" {
				where = append(where, "json_extract(data, '$.computer.name') LIKE ?")
				argv = append(argv, "%"+computerFilter+"%")
			}
			q := `SELECT
				  COALESCE(id,''),
				  COALESCE(request_time, start_time, ''),
				  COALESCE(type,''),
				  COALESCE(status,''),
				  COALESCE(json_extract(data, '$.user.account'), ''),
				  COALESCE(json_extract(data, '$.computer.name'), ''),
				  COALESCE(json_extract(data, '$.application.name'), ''),
				  COALESCE(reason,''),
				  COALESCE(approved_by,''),
				  COALESCE(denied_by,''),
				  COALESCE(denied_reason,''),
				  COALESCE(auditlog_link,'')
				 FROM auditlog
				 WHERE ` + strings.Join(where, " AND ") + `
				 ORDER BY COALESCE(request_time, start_time, '') ASC`
			rows, err := db.DB().QueryContext(cmd.Context(), q, argv...)
			if err != nil {
				return fmt.Errorf("querying auditlog: %w", err)
			}
			defer rows.Close()

			var data []complianceRow
			for rows.Next() {
				var r complianceRow
				if err := rows.Scan(&r.ID, &r.RequestTime, &r.Type, &r.Status, &r.User,
					&r.Computer, &r.Application, &r.Reason, &r.ApprovedBy, &r.DeniedBy,
					&r.DeniedReason, &r.AuditLogLink); err != nil {
					return err
				}
				data = append(data, r)
			}
			// rows.Err() is non-negotiable here: a compliance report missing
			// entries is worse than an error, because auditors may rely on it
			// for completeness. Fail loudly if the scan was truncated.
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating auditlog rows: %w", err)
			}
			if data == nil {
				data = []complianceRow{}
			}

			out := cmd.OutOrStdout()
			switch strings.ToLower(format) {
			case "md", "markdown":
				return writeComplianceMarkdown(out, data, since, until, userFilter, computerFilter)
			case "csv":
				return writeComplianceCSV(out, data)
			case "json":
				return printJSONFiltered(out, data, flags)
			default:
				if flags.asJSON || !isTerminal(out) {
					return printJSONFiltered(out, data, flags)
				}
				return writeComplianceMarkdown(out, data, since, until, userFilter, computerFilter)
			}
		},
	}
	cmd.Flags().StringVar(&sinceStr, "since", "", "Start of window (YYYY-MM-DD or RFC3339, required)")
	cmd.Flags().StringVar(&untilStr, "until", "", "End of window (defaults to now)")
	cmd.Flags().StringVar(&userFilter, "user", "", "Filter by user account or full name (substring match)")
	cmd.Flags().StringVar(&computerFilter, "computer", "", "Filter by computer name (substring match)")
	cmd.Flags().StringVar(&format, "format", "", "Output format: md (default for terminal), csv, json")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard CLI location)")
	return cmd
}

func parseLooseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{
		"2006-01-02",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z07:00",
		time.RFC3339,
		time.RFC3339Nano,
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("not a recognised date")
}

func writeComplianceMarkdown(w io.Writer, rows []complianceRow, since, until time.Time, user, computer string) error {
	fmt.Fprintf(w, "# Admin By Request Compliance Report\n\n")
	fmt.Fprintf(w, "- Window: %s -> %s\n", since.Format("2006-01-02"), until.Format("2006-01-02"))
	if user != "" {
		fmt.Fprintf(w, "- User filter: `%s`\n", user)
	}
	if computer != "" {
		fmt.Fprintf(w, "- Computer filter: `%s`\n", computer)
	}
	fmt.Fprintf(w, "- Total entries: %d\n\n", len(rows))
	fmt.Fprintf(w, "| Time | Type | Status | User | Computer | Application | Approved By | Denied By | Reason |\n")
	fmt.Fprintf(w, "|------|------|--------|------|----------|-------------|-------------|-----------|--------|\n")
	for _, r := range rows {
		fmt.Fprintf(w, "| %s | %s | %s | %s | %s | %s | %s | %s | %s |\n",
			r.RequestTime, mdSafe(r.Type), mdSafe(r.Status), mdSafe(r.User),
			mdSafe(r.Computer), mdSafe(r.Application), mdSafe(r.ApprovedBy),
			mdSafe(r.DeniedBy), mdSafe(firstNonEmpty(r.Reason, r.DeniedReason)))
	}
	return nil
}

func writeComplianceCSV(w io.Writer, rows []complianceRow) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()
	if err := cw.Write([]string{"id", "request_time", "type", "status", "user", "computer",
		"application", "reason", "approved_by", "denied_by", "denied_reason", "auditlog_link"}); err != nil {
		return err
	}
	for _, r := range rows {
		if err := cw.Write([]string{r.ID, r.RequestTime, r.Type, r.Status, r.User, r.Computer,
			r.Application, r.Reason, r.ApprovedBy, r.DeniedBy, r.DeniedReason, r.AuditLogLink}); err != nil {
			return err
		}
	}
	return nil
}

func mdSafe(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	return strings.ReplaceAll(s, "\n", " ")
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

