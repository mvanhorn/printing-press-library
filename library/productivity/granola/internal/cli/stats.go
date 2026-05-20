// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newStatsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Aggregate stats over the local store",
	}
	cmd.AddCommand(newStatsFrequencyCmd(flags))
	cmd.AddCommand(newStatsDurationCmd(flags))
	cmd.AddCommand(newStatsAttendeesCmd(flags))
	cmd.AddCommand(newStatsCalendarCmd(flags))
	return cmd
}

func newStatsFrequencyCmd(flags *rootFlags) *cobra.Command {
	var bucket, since string
	cmd := &cobra.Command{
		Use:   "frequency",
		Short: "Meetings per day/week/month bucket",
		Example: `  # Meetings per week for the last 90 days (default bucket)
  granola-pp-cli stats frequency --since 90d

  # Daily counts, JSON
  granola-pp-cli stats frequency --bucket day --since 30d --json

  # Monthly counts for the year
  granola-pp-cli stats frequency --bucket month --since 2026-01-01`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if bucket == "" {
				bucket = "week"
			}
			var sinceArg time.Time
			if since != "" {
				var err error
				sinceArg, err = parseAnyDate(since)
				if err != nil {
					return usageErr(err)
				}
			}
			s, err := openGranolaStoreRead(cmd.Context())
			if err != nil {
				return err
			}
			if s == nil {
				return notFoundErr(fmt.Errorf("no local data — run sync"))
			}
			defer s.Close()

			fmtStr := "%Y-%W"
			switch bucket {
			case "day":
				fmtStr = "%Y-%m-%d"
			case "month":
				fmtStr = "%Y-%m"
			case "week":
				fmtStr = "%Y-W%W"
			default:
				return usageErr(fmt.Errorf("invalid --bucket %q (day|week|month)", bucket))
			}
			q := fmt.Sprintf(`SELECT strftime(%q, started_at) AS bkt, COUNT(*) FROM meetings WHERE started_at IS NOT NULL AND started_at <> ''`, fmtStr)
			args2 := []any{}
			if !sinceArg.IsZero() {
				q += ` AND started_at >= ?`
				args2 = append(args2, sinceArg.UTC().Format("2006-01-02T15:04:05Z"))
			}
			q += ` GROUP BY bkt ORDER BY bkt ASC`
			rows, err := s.DB().Query(q, args2...)
			if err != nil {
				return err
			}
			defer rows.Close()
			out := []map[string]any{}
			for rows.Next() {
				var bkt string
				var cnt int
				if err := rows.Scan(&bkt, &cnt); err != nil {
					return err
				}
				out = append(out, map[string]any{"bucket": bkt, "count": cnt})
			}
			return emitJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&bucket, "bucket", "", "Bucket: day | week | month (default week)")
	cmd.Flags().StringVar(&since, "since", "", "Start date")
	return cmd
}

func newStatsDurationCmd(flags *rootFlags) *cobra.Command {
	var by string
	cmd := &cobra.Command{
		Use:   "duration",
		Short: "Total meeting duration aggregated by participant or calendar or template",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if by == "" {
				by = "participant"
			}
			s, err := openGranolaStoreRead(cmd.Context())
			if err != nil {
				return err
			}
			if s == nil {
				return notFoundErr(fmt.Errorf("no local data"))
			}
			defer s.Close()
			var q string
			switch by {
			case "participant":
				q = `SELECT a.email, COUNT(DISTINCT m.id), COALESCE(SUM(
					(strftime('%s', m.ended_at) - strftime('%s', m.started_at)) / 60.0
				), 0) FROM meetings m
				JOIN attendees a ON a.meeting_id = m.id
				WHERE m.started_at IS NOT NULL AND m.ended_at IS NOT NULL
				GROUP BY a.email ORDER BY 3 DESC`
			case "calendar":
				q = `SELECT COALESCE(calendar_event_id,'none'), COUNT(*), COALESCE(SUM(
					(strftime('%s', ended_at) - strftime('%s', started_at)) / 60.0
				), 0) FROM meetings
				WHERE started_at IS NOT NULL AND ended_at IS NOT NULL
				GROUP BY calendar_event_id ORDER BY 3 DESC`
			case "template":
				q = `SELECT COALESCE(creation_source,'unknown'), COUNT(*), COALESCE(SUM(
					(strftime('%s', ended_at) - strftime('%s', started_at)) / 60.0
				), 0) FROM meetings
				WHERE started_at IS NOT NULL AND ended_at IS NOT NULL
				GROUP BY creation_source ORDER BY 3 DESC`
			default:
				return usageErr(fmt.Errorf("invalid --by %q (participant|calendar|template)", by))
			}
			rows, err := s.DB().Query(q)
			if err != nil {
				return err
			}
			defer rows.Close()
			out := []map[string]any{}
			for rows.Next() {
				var key string
				var count int
				var totalMin float64
				if err := rows.Scan(&key, &count, &totalMin); err != nil {
					return err
				}
				out = append(out, map[string]any{
					"key":           key,
					"meeting_count": count,
					"total_minutes": totalMin,
				})
			}
			return emitJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&by, "by", "", "Aggregate by: participant | calendar | template (default participant)")
	return cmd
}

func newStatsAttendeesCmd(flags *rootFlags) *cobra.Command {
	var top int
	cmd := &cobra.Command{
		Use:   "attendees",
		Short: "Top N attendees by meeting count",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if top <= 0 {
				top = 20
			}
			s, err := openGranolaStoreRead(cmd.Context())
			if err != nil {
				return err
			}
			if s == nil {
				return notFoundErr(fmt.Errorf("no local data"))
			}
			defer s.Close()
			rows, err := s.DB().Query(`SELECT email, MAX(name), COUNT(DISTINCT meeting_id) AS c FROM attendees GROUP BY email ORDER BY c DESC LIMIT ?`, top)
			if err != nil {
				return err
			}
			defer rows.Close()
			out := []map[string]any{}
			for rows.Next() {
				var email, name string
				var c int
				if err := rows.Scan(&email, &name, &c); err != nil {
					return err
				}
				out = append(out, map[string]any{"email": email, "name": name, "meeting_count": c})
			}
			return emitJSON(cmd, flags, out)
		},
	}
	cmd.Flags().IntVar(&top, "top", 20, "Top N attendees")
	return cmd
}

func newStatsCalendarCmd(flags *rootFlags) *cobra.Command {
	var topDomains, topEmails bool
	var top int
	cmd := &cobra.Command{
		Use:   "calendar",
		Short: "Top calendar attendees/domains",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if top <= 0 {
				top = 20
			}
			s, err := openGranolaStoreRead(cmd.Context())
			if err != nil {
				return err
			}
			if s == nil {
				return notFoundErr(fmt.Errorf("no local data"))
			}
			defer s.Close()
			var q string
			if topDomains {
				q = `SELECT substr(email, instr(email,'@')+1) AS d, COUNT(DISTINCT meeting_id) c
					FROM attendees WHERE email LIKE '%@%' GROUP BY d ORDER BY c DESC LIMIT ?`
			} else if topEmails {
				q = `SELECT email, COUNT(DISTINCT meeting_id) c FROM attendees GROUP BY email ORDER BY c DESC LIMIT ?`
			} else {
				topDomains = true
				q = `SELECT substr(email, instr(email,'@')+1) AS d, COUNT(DISTINCT meeting_id) c
					FROM attendees WHERE email LIKE '%@%' GROUP BY d ORDER BY c DESC LIMIT ?`
			}
			rows, err := s.DB().Query(q, top)
			if err != nil {
				return err
			}
			defer rows.Close()
			out := []map[string]any{}
			for rows.Next() {
				var k string
				var c int
				if err := rows.Scan(&k, &c); err != nil {
					return err
				}
				key := "email"
				if topDomains {
					key = "domain"
				}
				out = append(out, map[string]any{key: k, "meeting_count": c})
			}
			return emitJSON(cmd, flags, out)
		},
	}
	cmd.Flags().BoolVar(&topDomains, "top-domains", false, "Top email domains")
	cmd.Flags().BoolVar(&topEmails, "top-emails", false, "Top email addresses")
	cmd.Flags().IntVar(&top, "top", 20, "Top N rows")
	return cmd
}
