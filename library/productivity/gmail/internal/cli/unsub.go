// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: unsubscribe audit. Joins stored List-Unsubscribe headers with
// per-sender volume and unread ratio to rank which lists to kill, and emits
// each list's unsubscribe target for agent execution. Gmail exposes the header
// only one message at a time; no competitor mines it corpus-wide.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local
package cli

import (
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type unsubRow struct {
	Sender       string   `json:"sender"`
	ListID       string   `json:"list_id,omitempty"`
	Count        int      `json:"count"`
	Unread       int      `json:"unread"`
	UnreadRatio  float64  `json:"unread_ratio"`
	HTTPTarget   string   `json:"http_target,omitempty"`
	MailtoTarget string   `json:"mailto_target,omitempty"`
	Unvalidated  []string `json:"unvalidated_targets,omitempty"`
}

// parseUnsubTargets splits a List-Unsubscribe header value like
// `<mailto:leave@example.com>, <https://x.com/u?id=1>` into its targets.
//
// The header is chosen by the sender and this command's output is meant to be
// acted on (often by an agent), so targets are validated rather than echoed:
// only http(s) with a routable public host and parsable mailto: addresses are
// returned. Anything else is reported separately as unvalidated.
func parseUnsubTargets(header string) (httpTarget, mailtoTarget string, unvalidated []string) {
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		part = strings.TrimPrefix(part, "<")
		part = strings.TrimSuffix(part, ">")
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		u, err := url.Parse(part)
		if err != nil {
			unvalidated = append(unvalidated, part)
			continue
		}
		switch strings.ToLower(u.Scheme) {
		case "http", "https":
			if httpTarget == "" && isPublicHost(u.Hostname()) {
				httpTarget = part
			} else if httpTarget == "" {
				unvalidated = append(unvalidated, part)
			}
		case "mailto":
			addr := strings.TrimPrefix(part, u.Scheme+":")
			if idx := strings.IndexByte(addr, '?'); idx >= 0 {
				addr = addr[:idx]
			}
			if mailtoTarget == "" {
				if _, err := mail.ParseAddress(addr); err == nil {
					mailtoTarget = part
				} else {
					unvalidated = append(unvalidated, part)
				}
			}
		default:
			unvalidated = append(unvalidated, part)
		}
	}
	return httpTarget, mailtoTarget, unvalidated
}

// isPublicHost rejects loopback, link-local, and private-range literals so an
// unsubscribe "target" cannot point an agent at an internal service.
func isPublicHost(host string) bool {
	if host == "" {
		return false
	}
	h := strings.ToLower(host)
	if h == "localhost" || strings.HasSuffix(h, ".localhost") || strings.HasSuffix(h, ".internal") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
			ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return false
		}
	}
	return true
}

func newNovelUnsubCmd(flags *rootFlags) *cobra.Command {
	var minCount int
	var limit int
	var readThreshold float64
	var dbPath string

	cmd := &cobra.Command{
		Use:   "unsub",
		Short: "Rank mailing lists by volume and never-read ratio, with unsubscribe targets",
		Long: `Mines List-Unsubscribe headers across the local mirror, ranks lists by
message volume and how rarely you read them, and emits each list's
unsubscribe URL or mailto target. Feed the --agent output to an agent to
act on the URLs, or open them yourself.`,
		Example: strings.Trim(`
  gmail-pp-cli unsub --min-count 10
  gmail-pp-cli unsub --min-count 10 --agent
  gmail-pp-cli unsub --never-read 0.8`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "rank mailing lists from the local mirror")
			}
			db, err := openGmailStore(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "messages") {
				hintIfStale(cmd, db, "messages", flags.maxAge)
			}
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT COALESCE(json_extract(data,'$.from_email'),'(unknown)') AS sender,
				       COALESCE(MAX(json_extract(data,'$.list_id')),'') AS list_id,
				       COUNT(*) AS cnt,
				       SUM(CASE WHEN json_extract(data,'$.unread') THEN 1 ELSE 0 END) AS unread,
				       MAX(json_extract(data,'$.list_unsubscribe')) AS unsub_header
				FROM messages
				WHERE json_extract(data,'$.list_unsubscribe') IS NOT NULL
				GROUP BY sender
				HAVING cnt >= ?
				   AND CAST(unread AS REAL) / cnt >= ?
				ORDER BY cnt DESC
				LIMIT ?`, minCount, readThreshold, limit)
			if err != nil {
				return fmt.Errorf("querying unsubscribe candidates: %w", err)
			}
			defer rows.Close()
			var out []unsubRow
			for rows.Next() {
				var r unsubRow
				var header string
				if err := rows.Scan(&r.Sender, &r.ListID, &r.Count, &r.Unread, &header); err != nil {
					return fmt.Errorf("scanning unsub row: %w", err)
				}
				if r.Count > 0 {
					r.UnreadRatio = float64(r.Unread) / float64(r.Count)
				}
				r.HTTPTarget, r.MailtoTarget, r.Unvalidated = parseUnsubTargets(header)
				out = append(out, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if wantsJSONOutput(cmd, flags) {
				if out == nil {
					out = []unsubRow{}
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no mailing lists over the thresholds; is the local mirror populated? (gmail-pp-cli pull)")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "COUNT\tUNREAD%\tSENDER\tUNSUBSCRIBE")
			for _, r := range out {
				target := r.HTTPTarget
				if target == "" {
					target = r.MailtoTarget
				}
				fmt.Fprintf(tw, "%d\t%.0f%%\t%s\t%s\n", r.Count, r.UnreadRatio*100, truncateCell(r.Sender, 36), truncateCell(target, 60))
			}
			tw.Flush()
			return nil
		},
	}
	cmd.Flags().IntVar(&minCount, "min-count", 5, "Only lists with at least this many messages")
	cmd.Flags().IntVar(&limit, "limit", 30, "Number of lists to show")
	cmd.Flags().Float64Var(&readThreshold, "never-read", 0, "Only lists whose unread ratio is at least this (0..1)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: resolved data directory data.db)")
	return cmd
}
