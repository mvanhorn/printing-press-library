// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/store"

	"github.com/spf13/cobra"
)

func newNovelInboxCmd(flags *rootFlags) *cobra.Command {
	var flagSince, space, dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "inbox",
		Short: "See new members, unreplied posts, flagged content, form submissions, failed charges and cancellations since a time",
		Long:  "Use this command for what needs attention since a point in time across members, posts, moderation, forms, and payments. Do NOT use this command for paid-seat counts or capacity; use 'seats' instead.",
		Example: strings.Trim(`
  circle-pp-cli inbox --since 24h --agent
  circle-pp-cli inbox --since 1w --space 1000969 --limit 20
  circle-pp-cli inbox --space "Introductions" --db ./circle.db --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Preserve root setup while replacing its generic source rejection with inbox guidance.
			var err error
			if cmd.Parent() != nil && cmd.Root().PersistentPreRunE != nil {
				err = cmd.Root().PersistentPreRunE(cmd, args)
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("inbox reads the local mirror only; run sync first"))
			}
			return err
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "inbox")
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("inbox reads the local mirror only; run sync first"))
			}
			duration, err := cliutil.ParseDurationLoose(flagSince)
			if err != nil || duration <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--since must be a positive duration such as 24h, 7d, or 1w"))
			}
			if len(args) != 0 || limit <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("inbox takes no positional arguments and --limit must be positive"))
			}
			cutoff := time.Now().UTC().Add(-duration)
			if dbPath == "" {
				dbPath = defaultDBPath("circle-pp-cli")
			}
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: circle-pp-cli sync\n", dbPath)
				report := buildInbox(nil, cutoff, space, limit)
				report.setMirror(inboxMirror{Path: dbPath})
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			} else if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return err
			}
			defer func() { _ = db.Close() }()
			hintIfUnsynced(cmd, db, "")
			hintIfStale(cmd, db, "", flags.maxAge)
			records, err := readInboxRecords(ctx, db)
			if err != nil {
				return err
			}
			report := buildInbox(records, cutoff, space, limit)
			mirror, err := readInboxMirror(ctx, db, dbPath)
			if err != nil {
				return err
			}
			report.setMirror(mirror)
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			return printInbox(cmd.OutOrStdout(), report)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "24h", "Show attention items since this duration ago (24h, 7d, or 1w).")
	cmd.Flags().StringVar(&space, "space", "", "Filter unreplied posts by space ID or case-insensitive space name.")
	cmd.Flags().StringVar(&dbPath, "db", "", "Read this local mirror database instead of the default store.")
	cmd.Flags().IntVar(&limit, "limit", 50, "Show at most this many rows in each section.")
	return cmd
}

type inboxMember struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
}

type inboxPost struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SpaceName string `json:"space_name"`
	UserName  string `json:"user_name"`
	URL       string `json:"url"`
	CreatedAt string `json:"created_at"`
}

type inboxFlag struct {
	ID                 string `json:"id"`
	ContentType        string `json:"content_type"`
	ContentID          string `json:"content_id"`
	ReportedReasonType string `json:"reported_reason_type"`
	ReportedByName     string `json:"reported_by_name"`
	ReportedAt         string `json:"reported_at"`
	Status             string `json:"status"`
}

type inboxSubmission struct {
	ID           string `json:"id"`
	ResourceType string `json:"resource_type"`
	CreatedAt    string `json:"created_at"`
}

type inboxCharge struct {
	ID                   string `json:"id"`
	CommunityMemberEmail string `json:"community_member_email"`
	Amount               int64  `json:"amount"`
	AmountDisplay        string `json:"amount_display"`
	Currency             string `json:"currency"`
	Status               string `json:"status"`
	CreatedAt            string `json:"created_at"`
}

type inboxCancellation struct {
	ID                   string `json:"id"`
	CommunityMemberEmail string `json:"community_member_email"`
	PaywallName          string `json:"paywall_name"`
	Status               string `json:"status"`
	EndsAt               string `json:"ends_at"`
}

type inboxReport struct {
	Mirror          inboxMirror         `json:"mirror"`
	Warnings        []string            `json:"warnings,omitempty"`
	Since           string              `json:"since"`
	Counts          map[string]int      `json:"counts"`
	NewMembers      []inboxMember       `json:"new_members"`
	UnrepliedPosts  []inboxPost         `json:"unreplied_posts"`
	FlaggedContent  []inboxFlag         `json:"flagged_content"`
	FormSubmissions []inboxSubmission   `json:"form_submissions"`
	FailedCharges   []inboxCharge       `json:"failed_charges"`
	Cancellations   []inboxCancellation `json:"cancellations"`
}

type inboxMirror struct {
	Path         string `json:"path"`
	Exists       bool   `json:"exists"`
	LastSyncedAt string `json:"last_synced_at"`
}

func (report *inboxReport) setMirror(mirror inboxMirror) {
	report.Mirror = mirror
	if mirror.LastSyncedAt == "" {
		report.Warnings = []string{"mirror not synced; run circle-pp-cli sync"}
	}
}

func readInboxMirror(ctx context.Context, db *store.Store, dbPath string) (inboxMirror, error) {
	mirror := inboxMirror{Path: dbPath, Exists: true}
	var lastSynced sql.NullTime
	err := db.DB().QueryRowContext(ctx, `SELECT last_synced_at FROM sync_state
		WHERE last_synced_at IS NOT NULL ORDER BY last_synced_at DESC LIMIT 1`).Scan(&lastSynced)
	if err == sql.ErrNoRows {
		return mirror, nil
	}
	if err != nil {
		return mirror, err
	}
	if lastSynced.Valid && !lastSynced.Time.IsZero() {
		mirror.LastSyncedAt = lastSynced.Time.UTC().Format(time.RFC3339)
	}
	return mirror, nil
}

func inboxAmountDisplay(cents int64, currency string) string {
	whole, fraction := cents/100, cents%100
	sign := ""
	if cents < 0 {
		sign, whole, fraction = "-", -whole, -fraction
	}
	return strings.TrimSpace(fmt.Sprintf("%s%d.%02d %s", sign, whole, fraction, strings.ToUpper(currency)))
}

type inboxRecord struct {
	ResourceType, ID, Name, Email, CreatedAt                               string
	SpaceID, SpaceName, UserName, URL                                      string
	ContentType, ContentID, ReportedReasonType, ReportedByName, ReportedAt string
	Status, CommunityMemberEmail, Currency, PaywallName, EndsAt            string
	CommentsCount                                                          int
	Amount                                                                 int64
	ScheduledToCancel                                                      bool
}

func readInboxRecords(ctx context.Context, db *store.Store) ([]inboxRecord, error) {
	rows, err := db.DB().QueryContext(ctx, `SELECT resource_type, id,
		COALESCE(json_extract(data, '$.name'), ''), COALESCE(json_extract(data, '$.email'), ''),
		COALESCE(json_extract(data, '$.created_at'), ''), COALESCE(json_extract(data, '$.space_id'), ''),
		COALESCE(json_extract(data, '$.space_name'), ''), COALESCE(json_extract(data, '$.user_name'), ''),
		COALESCE(json_extract(data, '$.url'), ''), COALESCE(json_extract(data, '$.content_type'), ''),
		COALESCE(json_extract(data, '$.content_id'), ''), COALESCE(json_extract(data, '$.reported_reason_type'), ''),
		COALESCE(json_extract(data, '$.reported_by_name'), ''), COALESCE(json_extract(data, '$.reported_at'), ''),
		COALESCE(json_extract(data, '$.status'), ''), COALESCE(json_extract(data, '$.community_member_email'), ''),
		COALESCE(json_extract(data, '$.currency'), ''), COALESCE(json_extract(data, '$.paywall_name'), ''),
		COALESCE(json_extract(data, '$.ends_at'), ''), COALESCE(json_extract(data, '$.comments_count'), 0),
		COALESCE(json_extract(data, '$.amount'), 0), COALESCE(json_extract(data, '$.scheduled_to_cancel'), 0)
		FROM resources`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	records := make([]inboxRecord, 0)
	for rows.Next() {
		var r inboxRecord
		if err := rows.Scan(&r.ResourceType, &r.ID, &r.Name, &r.Email, &r.CreatedAt,
			&r.SpaceID, &r.SpaceName, &r.UserName, &r.URL, &r.ContentType, &r.ContentID,
			&r.ReportedReasonType, &r.ReportedByName, &r.ReportedAt, &r.Status,
			&r.CommunityMemberEmail, &r.Currency, &r.PaywallName, &r.EndsAt,
			&r.CommentsCount, &r.Amount, &r.ScheduledToCancel); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return records, nil
}

func inboxTime(value string) time.Time {
	t, _ := time.Parse(time.RFC3339, value)
	return t
}

func inboxRecordTime(r inboxRecord) time.Time {
	if r.ResourceType == "flagged-contents" {
		return inboxTime(r.ReportedAt)
	}
	if r.ResourceType == "community-member-subscriptions" && r.EndsAt != "" {
		return inboxTime(r.EndsAt)
	}
	return inboxTime(r.CreatedAt)
}

func buildInbox(records []inboxRecord, cutoff time.Time, space string, limit int) inboxReport {
	report := inboxReport{
		Since:      cutoff.UTC().Format(time.RFC3339),
		Counts:     map[string]int{"members": 0, "posts": 0, "comments": 0, "spaces": 0},
		NewMembers: make([]inboxMember, 0), UnrepliedPosts: make([]inboxPost, 0),
		FlaggedContent: make([]inboxFlag, 0), FormSubmissions: make([]inboxSubmission, 0),
		FailedCharges: make([]inboxCharge, 0), Cancellations: make([]inboxCancellation, 0),
	}
	records = append([]inboxRecord(nil), records...)
	sort.SliceStable(records, func(i, j int) bool { return inboxRecordTime(records[i]).After(inboxRecordTime(records[j])) })
	for _, r := range records {
		recent := !inboxTime(r.CreatedAt).IsZero() && !inboxTime(r.CreatedAt).Before(cutoff)
		status := strings.ToLower(strings.TrimSpace(r.Status))
		switch r.ResourceType {
		case "community-members":
			report.Counts["members"]++
			if recent && len(report.NewMembers) < limit {
				report.NewMembers = append(report.NewMembers, inboxMember{r.ID, r.Name, r.Email, r.CreatedAt})
			}
		case "posts":
			report.Counts["posts"]++
			if recent && r.CommentsCount == 0 && (space == "" || r.SpaceID == space || strings.EqualFold(r.SpaceName, space)) && len(report.UnrepliedPosts) < limit {
				report.UnrepliedPosts = append(report.UnrepliedPosts, inboxPost{r.ID, r.Name, r.SpaceName, r.UserName, r.URL, r.CreatedAt})
			}
		case "comments", "spaces":
			report.Counts[r.ResourceType]++
		case "flagged-contents":
			if status != "resolved" && status != "approved" && status != "dismissed" && status != "rejected" && !inboxTime(r.ReportedAt).IsZero() && !inboxTime(r.ReportedAt).Before(cutoff) && len(report.FlaggedContent) < limit {
				report.FlaggedContent = append(report.FlaggedContent, inboxFlag{r.ID, r.ContentType, r.ContentID, r.ReportedReasonType, r.ReportedByName, r.ReportedAt, r.Status})
			}
		case "community-member-charges":
			if recent && (status == "failed" || status == "declined" || status == "requires_payment_method") && len(report.FailedCharges) < limit {
				report.FailedCharges = append(report.FailedCharges, inboxCharge{r.ID, r.CommunityMemberEmail, r.Amount, inboxAmountDisplay(r.Amount, r.Currency), r.Currency, r.Status, r.CreatedAt})
			}
		case "community-member-subscriptions":
			if (status == "canceled" || status == "cancelled" || r.ScheduledToCancel) && !inboxRecordTime(r).IsZero() && !inboxRecordTime(r).Before(cutoff) && len(report.Cancellations) < limit {
				report.Cancellations = append(report.Cancellations, inboxCancellation{r.ID, r.CommunityMemberEmail, r.PaywallName, r.Status, r.EndsAt})
			}
		default:
			if strings.HasSuffix(r.ResourceType, "submissions") && recent && len(report.FormSubmissions) < limit {
				report.FormSubmissions = append(report.FormSubmissions, inboxSubmission{r.ID, r.ResourceType, r.CreatedAt})
			}
		}
	}
	report.Counts["new_members"] = len(report.NewMembers)
	report.Counts["unreplied_posts"] = len(report.UnrepliedPosts)
	report.Counts["flagged_content"] = len(report.FlaggedContent)
	report.Counts["form_submissions"] = len(report.FormSubmissions)
	report.Counts["failed_charges"] = len(report.FailedCharges)
	report.Counts["cancellations"] = len(report.Cancellations)
	return report
}

func printInbox(w io.Writer, report inboxReport) error {
	for _, warning := range report.Warnings {
		if _, err := fmt.Fprintln(w, "Warning:", warning); err != nil {
			return err
		}
	}
	sections := []struct {
		title, header string
		rows          []string
	}{
		{title: "New members", header: "ID\tNAME\tEMAIL\tCREATED"},
		{title: "Unreplied posts", header: "ID\tPOST\tSPACE\tAUTHOR\tCREATED"},
		{title: "Flagged content", header: "ID\tTYPE\tREASON\tREPORTED BY\tREPORTED\tSTATUS"},
		{title: "Form submissions", header: "ID\tRESOURCE\tCREATED"},
		{title: "Failed charges", header: "ID\tEMAIL\tAMOUNT\tSTATUS\tCREATED"},
		{title: "Cancellations", header: "ID\tEMAIL\tPAYWALL\tSTATUS\tENDS"},
	}
	for _, r := range report.NewMembers {
		sections[0].rows = append(sections[0].rows, fmt.Sprintf("%s\t%s\t%s\t%s", r.ID, r.Name, r.Email, r.CreatedAt))
	}
	for _, r := range report.UnrepliedPosts {
		sections[1].rows = append(sections[1].rows, fmt.Sprintf("%s\t%s\t%s\t%s\t%s", r.ID, r.Name, r.SpaceName, r.UserName, r.CreatedAt))
	}
	for _, r := range report.FlaggedContent {
		sections[2].rows = append(sections[2].rows, fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s", r.ID, r.ContentType, r.ReportedReasonType, r.ReportedByName, r.ReportedAt, r.Status))
	}
	for _, r := range report.FormSubmissions {
		sections[3].rows = append(sections[3].rows, fmt.Sprintf("%s\t%s\t%s", r.ID, r.ResourceType, r.CreatedAt))
	}
	for _, r := range report.FailedCharges {
		sections[4].rows = append(sections[4].rows, fmt.Sprintf("%s\t%s\t%s\t%s\t%s", r.ID, r.CommunityMemberEmail, r.AmountDisplay, r.Status, r.CreatedAt))
	}
	for _, r := range report.Cancellations {
		sections[5].rows = append(sections[5].rows, fmt.Sprintf("%s\t%s\t%s\t%s\t%s", r.ID, r.CommunityMemberEmail, r.PaywallName, r.Status, r.EndsAt))
	}
	shown := false
	for _, section := range sections {
		if len(section.rows) == 0 {
			continue
		}
		shown = true
		if _, err := fmt.Fprintf(w, "%s\n", section.title); err != nil {
			return err
		}
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		if _, err := fmt.Fprintln(tw, section.header); err != nil {
			return err
		}
		for _, row := range section.rows {
			if _, err := fmt.Fprintln(tw, row); err != nil {
				return err
			}
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}
	if !shown {
		_, err := fmt.Fprintf(w, "Nothing needs you since %s.\n", report.Since)
		return err
	}
	return nil
}
