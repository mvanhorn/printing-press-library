// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
)

func newMeetingsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "meetings",
		Short: "List, fetch, delete and restore Granola meetings",
		Long: `Granola meetings stored locally (after sync) and live via the internal API.

The 'meetings list' command queries the synced SQLite store; 'meetings get'
prefers the live API but falls back to cache; mutating commands always
require an authenticated live call.`,
	}
	cmd.AddCommand(newMeetingsListCmd(flags))
	cmd.AddCommand(newMeetingsGetCmd(flags))
	cmd.AddCommand(newMeetingsFetchBatchCmd(flags))
	cmd.AddCommand(newMeetingsDeleteCmd(flags))
	cmd.AddCommand(newMeetingsRestoreCmd(flags))
	return cmd
}

type meetingRow struct {
	ID                  string   `json:"id"`
	Title               string   `json:"title"`
	StartedAt           string   `json:"started_at,omitempty"`
	EndedAt             string   `json:"ended_at,omitempty"`
	CreatedAt           string   `json:"created_at,omitempty"`
	UpdatedAt           string   `json:"updated_at,omitempty"`
	WorkspaceID         string   `json:"workspace_id,omitempty"`
	TranscriptAvailable bool     `json:"transcript_available"`
	Attendees           []string `json:"attendees,omitempty"`
	DeletedAt           string   `json:"deleted_at,omitempty"`
	CalendarEventID     string   `json:"calendar_event_id,omitempty"`
	CreationSource      string   `json:"creation_source,omitempty"`
}

func newMeetingsListCmd(flags *rootFlags) *cobra.Command {
	var last, since, until, participant, query string
	var limit, offset int
	var deleted bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List meetings from the local store",
		Long: `Reads from the synced SQLite store. Run 'granola-pp-cli sync' first.

Filters compose: --last 7d, --since DATE, --until DATE, --participant EMAIL,
--query TEXT (FTS over title+notes_plain), --deleted (include soft-deleted).`,
		Example: `  # Recent meetings, table form
  granola-pp-cli meetings list --last 7d

  # Meetings with a specific attendee, JSON
  granola-pp-cli meetings list --participant alice@example.com --json

  # Full-text search over title + notes
  granola-pp-cli meetings list --query "Q2 planning" --last 90d

  # Include soft-deleted meetings
  granola-pp-cli meetings list --deleted --last 30d`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			from, to, err := parseTimeWindow(last, since, until)
			if err != nil {
				return usageErr(err)
			}
			rows, err := loadMeetings(cmd.Context(), from, to, participant, query, deleted, limit, offset)
			if err != nil {
				return err
			}
			if flags.asJSON || flags.agent {
				return emitJSON(cmd, flags, rows)
			}
			tbl := make([][]string, 0, len(rows))
			for _, r := range rows {
				tbl = append(tbl, []string{r.ID, r.Title, r.StartedAt, fmt.Sprintf("%v", r.TranscriptAvailable)})
			}
			return flags.printTable(cmd, []string{"ID", "TITLE", "STARTED_AT", "TRANSCRIBED"}, tbl)
		},
	}
	cmd.Flags().StringVar(&last, "last", "", "Time window (e.g. 7d, 24h, 4w)")
	cmd.Flags().StringVar(&since, "since", "", "Start date (RFC3339 or YYYY-MM-DD)")
	cmd.Flags().StringVar(&until, "until", "", "End date (RFC3339 or YYYY-MM-DD)")
	cmd.Flags().StringVar(&participant, "participant", "", "Filter by attendee email (substring match)")
	cmd.Flags().StringVar(&query, "query", "", "Full-text query on title + notes")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to return (0 = unlimited)")
	cmd.Flags().IntVar(&offset, "offset", 0, "Rows to skip")
	cmd.Flags().BoolVar(&deleted, "deleted", false, "Include soft-deleted meetings")
	return cmd
}

func loadMeetings(ctx context.Context, from, to time.Time, participant, query string, includeDeleted bool, limit, offset int) ([]meetingRow, error) {
	s, err := openGranolaStoreRead(ctx)
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, notFoundErr(fmt.Errorf("no local data — run 'granola-pp-cli sync' first"))
	}
	defer s.Close()

	var whereParts []string
	var args []any
	if !from.IsZero() {
		whereParts = append(whereParts, "started_at >= ?")
		args = append(args, from.UTC().Format("2006-01-02T15:04:05Z"))
	}
	if !to.IsZero() {
		whereParts = append(whereParts, "started_at <= ?")
		args = append(args, to.UTC().Format("2006-01-02T15:04:05Z"))
	}
	if !includeDeleted {
		whereParts = append(whereParts, "(deleted_at IS NULL OR deleted_at = '')")
	}
	if participant != "" {
		whereParts = append(whereParts, "id IN (SELECT meeting_id FROM attendees WHERE email LIKE ? OR name LIKE ?)")
		args = append(args, "%"+strings.ToLower(participant)+"%", "%"+participant+"%")
	}
	if query != "" {
		// Use FTS5 LIKE-fallback for safety; FTS MATCH would require us to
		// re-rank.
		whereParts = append(whereParts, "(title LIKE ? OR notes_plain LIKE ?)")
		args = append(args, "%"+query+"%", "%"+query+"%")
	}
	q := `SELECT id, title, started_at, ended_at, created_at, updated_at, workspace_id, transcript_available, COALESCE(deleted_at,''), COALESCE(calendar_event_id,''), COALESCE(creation_source,'') FROM meetings`
	if len(whereParts) > 0 {
		q += " WHERE " + strings.Join(whereParts, " AND ")
	}
	q += " ORDER BY started_at DESC"
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)
	} else if offset > 0 {
		q += fmt.Sprintf(" LIMIT -1 OFFSET %d", offset)
	}
	rows, err := s.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query meetings: %w", err)
	}
	defer rows.Close()
	var out []meetingRow
	for rows.Next() {
		var r meetingRow
		var transcribed int
		if err := rows.Scan(&r.ID, &r.Title, &r.StartedAt, &r.EndedAt, &r.CreatedAt, &r.UpdatedAt, &r.WorkspaceID, &transcribed, &r.DeletedAt, &r.CalendarEventID, &r.CreationSource); err != nil {
			return nil, err
		}
		r.TranscriptAvailable = transcribed == 1
		// Attendees (one extra query — fine for paged result sizes).
		ar, err := s.DB().QueryContext(ctx, `SELECT email FROM attendees WHERE meeting_id = ? ORDER BY email ASC`, r.ID)
		if err == nil {
			for ar.Next() {
				var e string
				if err := ar.Scan(&e); err == nil && e != "" {
					r.Attendees = append(r.Attendees, e)
				}
			}
			ar.Close()
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func newMeetingsGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Get one meeting by id",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id := args[0]
			// Prefer cache for richness (notes, transcript flag).
			c, err := openGranolaCache()
			if err == nil {
				if d := c.DocumentByID(id); d != nil {
					out := flattenDocForJSON(c, d)
					return emitJSON(cmd, flags, out)
				}
			}
			// Fallback: store.
			s, err := openGranolaStoreRead(cmd.Context())
			if err == nil && s != nil {
				s.Close()
				rows, err := loadMeetings(cmd.Context(), time.Time{}, time.Time{}, "", "", true, 0, 0)
				if err == nil {
					for _, r := range rows {
						if r.ID == id {
							return emitJSON(cmd, flags, r)
						}
					}
				}
			}
			// Fallback: live.
			if flags.dataSource != "local" {
				ic, ierr := granola.NewInternalClient()
				if ierr == nil {
					docs, derr := ic.GetDocumentsBatch([]string{id})
					if derr == nil && len(docs) > 0 {
						return emitJSON(cmd, flags, flattenDocForJSON(nil, &docs[0]))
					}
				}
			}
			return notFoundErr(fmt.Errorf("meeting %s not found", id))
		},
	}
	return cmd
}

func newMeetingsFetchBatchCmd(flags *rootFlags) *cobra.Command {
	var idsCSV string
	cmd := &cobra.Command{
		Use:   "fetch-batch",
		Short: "Fetch multiple meetings by id via the internal API",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if idsCSV == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ids := splitAndTrim(idsCSV)
			ic, err := granola.NewInternalClient()
			if err != nil {
				return authErr(err)
			}
			docs, err := ic.GetDocumentsBatch(ids)
			if err != nil {
				return apiErr(err)
			}
			out := make([]any, 0, len(docs))
			for _, d := range docs {
				d := d
				out = append(out, flattenDocForJSON(nil, &d))
			}
			return emitJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&idsCSV, "ids", "", "Comma-separated meeting ids")
	return cmd
}

func newMeetingsDeleteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Soft-delete a meeting (sets deleted_at)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			id := args[0]
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), `{"dry_run":true,"action":"delete","id":%q}`+"\n", id)
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), `{"verify":true,"action":"delete","id":%q}`+"\n", id)
				return nil
			}
			ic, err := granola.NewInternalClient()
			if err != nil {
				return authErr(err)
			}
			if err := ic.DeleteDocument(id); err != nil {
				return apiErr(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), `{"deleted":true,"id":%q}`+"\n", id)
			return nil
		},
	}
	return cmd
}

func newMeetingsRestoreCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore <id>",
		Short: "Restore a soft-deleted meeting",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			id := args[0]
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), `{"dry_run":true,"action":"restore","id":%q}`+"\n", id)
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), `{"verify":true,"action":"restore","id":%q}`+"\n", id)
				return nil
			}
			ic, err := granola.NewInternalClient()
			if err != nil {
				return authErr(err)
			}
			if err := ic.RestoreDocument(id); err != nil {
				return apiErr(err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), `{"restored":true,"id":%q}`+"\n", id)
			return nil
		},
	}
	return cmd
}

// flattenDocForJSON builds a public-shaped record for emission. Pass a
// non-nil cache to include meetingsMetadata attendees; live-API records
// already carry their attendees in doc.people.
func flattenDocForJSON(c *granola.Cache, d *granola.Document) map[string]any {
	out := map[string]any{
		"id":              d.ID,
		"title":           d.Title,
		"created_at":      d.CreatedAt,
		"updated_at":      d.UpdatedAt,
		"workspace_id":    d.WorkspaceID,
		"creation_source": d.CreationSource,
		"valid_meeting":   d.ValidMeeting,
	}
	if d.DeletedAt != nil {
		out["deleted_at"] = *d.DeletedAt
	}
	if d.GoogleCalendarEvent != nil {
		out["calendar_event_id"] = d.GoogleCalendarEvent.ID
		out["calendar_summary"] = d.GoogleCalendarEvent.Summary
		out["calendar_html_link"] = d.GoogleCalendarEvent.HtmlLink
	}
	if d.NotesPlain != "" {
		out["notes_plain"] = d.NotesPlain
	}
	if d.NotesMarkdown != "" {
		out["notes_markdown"] = d.NotesMarkdown
	}
	// Attendees: prefer meetingsMetadata when we have a cache reference.
	if c != nil {
		if md := c.MeetingMetadataByID(d.ID); md != nil {
			out["attendees"] = md.Attendees
		}
	}
	if _, hasAtt := out["attendees"]; !hasAtt && d.People != nil {
		out["attendees"] = d.People.Attendees
	}
	if len(d.Notes) > 0 {
		out["notes_tiptap"] = json.RawMessage(d.Notes)
	}
	return out
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Reference sql for store usage.
var _ = sql.ErrNoRows
