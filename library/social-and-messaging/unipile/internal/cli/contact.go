// Copyright 2026 fuushyn and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type contactChat struct {
	ChatID      string `json:"chat_id"`
	Provider    string `json:"provider"`
	Messages    int    `json:"messages"`
	FromThem    int    `json:"from_them"`
	FromYou     int    `json:"from_you"`
	LastMessage string `json:"last_message,omitempty"`
	LastAt      string `json:"last_at,omitempty"`
	LastFrom    string `json:"last_from,omitempty"`
}

type contactView struct {
	Query        string        `json:"query"`
	Name         string        `json:"name,omitempty"`
	MemberID     string        `json:"member_id,omitempty"`
	Headline     string        `json:"headline,omitempty"`
	ProfileURL   string        `json:"profile_url,omitempty"`
	Connected    bool          `json:"connected"`
	ConnectedAt  string        `json:"connected_at,omitempty"`
	InviteSentAt string        `json:"invitation_sent_at,omitempty"`
	InviteGotAt  string        `json:"invitation_received_at,omitempty"`
	Chats        []contactChat `json:"chats"`
	Candidates   []string      `json:"other_matches,omitempty"`
	Note         string        `json:"note,omitempty"`
}

// nameMatchRank scores how well a stored name answers the query, lowest first:
// 0 exact, 1 prefix, 2 word-boundary, 3 loose substring, -1 no match. Sorting
// alphabetically instead made any earlier surname outrank an exact hit, so
// `contact "Michael"` answered with "Ahan Vincent Michael".
func nameMatchRank(name, query string) int {
	n := strings.ToLower(strings.TrimSpace(name))
	q := strings.ToLower(strings.TrimSpace(query))
	if n == "" || q == "" {
		return -1
	}
	switch {
	case n == q:
		return 0
	case strings.HasPrefix(n, q):
		return 1
	}
	for _, word := range strings.Fields(n) {
		if strings.HasPrefix(word, q) {
			return 2
		}
	}
	if strings.Contains(n, q) {
		return 3
	}
	return -1
}

func newNovelContactCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "contact [name-or-identifier]",
		Short: "Everything the local mirror knows about one person",
		Long: strings.Trim(`
Joins connections, invitation history, conversations, and message counts for a
single human, matched by name, LinkedIn public identifier, or member id.

Today this takes six API calls and a hand-written join.

Use this command before writing to someone. Do NOT use it to fetch a live
LinkedIn profile; use 'users get'.`, "\n"),
		Example: strings.Trim(`
  unipile-pp-cli contact "Ada Lovelace" --agent
  unipile-pp-cli contact ada-lovelace
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "query=example",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "contact")
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a name, public identifier, or member id is required"))
			}
			query := strings.TrimSpace(args[0])
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			view := contactView{Query: query, Chats: make([]contactChat, 0)}
			db, ok, err := novelStore(cmd, flags, dbPath, view)
			if err != nil || !ok {
				return err
			}
			defer db.Close()

			relations, err := loadRelations(ctx, db)
			if err != nil {
				return err
			}
			needle := strings.ToLower(query)
			type rankedRelation struct {
				row  relationRow
				rank int
			}
			ranked := make([]rankedRelation, 0)
			for _, r := range relations {
				rank := nameMatchRank(r.Name, query)
				// An id match is as good as an exact name match.
				if strings.EqualFold(r.PublicID, query) || strings.EqualFold(r.MemberID, query) {
					rank = 0
				}
				if rank < 0 {
					continue
				}
				ranked = append(ranked, rankedRelation{row: r, rank: rank})
			}
			sort.SliceStable(ranked, func(i, j int) bool {
				if ranked[i].rank != ranked[j].rank {
					return ranked[i].rank < ranked[j].rank
				}
				return ranked[i].row.Name < ranked[j].row.Name
			})
			matches := make([]relationRow, 0, len(ranked))
			for _, r := range ranked {
				matches = append(matches, r.row)
			}

			var target relationRow
			if len(matches) > 0 {
				target = matches[0]
				view.Connected = true
				view.Name = target.Name
				view.MemberID = target.MemberID
				view.Headline = target.Headline
				view.ProfileURL = target.ProfileURL
				view.ConnectedAt = target.CreatedAt
				for _, m := range matches[1:] {
					view.Candidates = append(view.Candidates, m.Name)
				}
			}

			// Invitation history matches on the invited user's name or public id
			// even when the person never became a connection.
			for _, rt := range []string{"users-invite-sent", "users-invite-received"} {
				rows, qerr := db.QueryContext(ctx, `
					SELECT COALESCE(json_extract(data,'$.invited_user'),''),
					       COALESCE(json_extract(data,'$.invited_user_id'),''),
					       COALESCE(json_extract(data,'$.invited_user_public_id'),''),
					       COALESCE(json_extract(data,'$.parsed_datetime'),'')
					FROM resources WHERE resource_type = ?`, rt)
				if qerr != nil {
					return fmt.Errorf("reading %s: %w", rt, qerr)
				}
				var bestName, bestAt string
				for rows.Next() {
					var name, id, publicID, ts sql.NullString
					if serr := rows.Scan(&name, &id, &publicID, &ts); serr != nil {
						_ = rows.Close()
						return fmt.Errorf("scanning %s: %w", rt, serr)
					}
					hit := strings.Contains(strings.ToLower(name.String), needle) ||
						strings.EqualFold(publicID.String, query) ||
						strings.EqualFold(id.String, query)
					if target.MemberID != "" && id.String == target.MemberID {
						hit = true
					}
					if hit && ts.String > bestAt {
						bestAt, bestName = ts.String, name.String
					}
				}
				if rerr := rows.Err(); rerr != nil {
					_ = rows.Close()
					return fmt.Errorf("iterating %s: %w", rt, rerr)
				}
				if cerr := rows.Close(); cerr != nil {
					return fmt.Errorf("closing %s rows: %w", rt, cerr)
				}
				if bestAt != "" {
					if rt == "users-invite-sent" {
						view.InviteSentAt = bestAt
					} else {
						view.InviteGotAt = bestAt
					}
					if view.Name == "" {
						view.Name = bestName
					}
				}
			}

			chats, err := loadChats(ctx, db, false)
			if err != nil {
				return err
			}
			attendees, err := loadAttendees(ctx, db)
			if err != nil {
				return err
			}
			last, inbound, outbound, err := loadLastMessages(ctx, db)
			if err != nil {
				return err
			}
			for _, c := range chats {
				// Match the resolved counterpart, not just the chat title: a
				// one-to-one chat often stores no name and resolves the person
				// only through the attendee index, so title-only matching made
				// contact miss people who are plainly visible in inbox/thread.
				who := counterpartName(c, relations, attendees)
				nameMatch := strings.Contains(strings.ToLower(c.Name), needle) ||
					nameMatchRank(who, query) >= 0
				idMatch := view.MemberID != "" && c.AttendeeID == view.MemberID
				if !nameMatch && !idMatch {
					continue
				}
				// A chat hit is enough to know the person exists locally, even
				// when they are not a connection and were never invited.
				if view.Name == "" && who != "" && who != "(unknown)" {
					view.Name = who
				}
				cc := contactChat{
					ChatID:   c.ID,
					Provider: c.AccountType,
					FromThem: inbound[c.ID],
					FromYou:  outbound[c.ID],
					Messages: inbound[c.ID] + outbound[c.ID],
				}
				if m, hit := last[c.ID]; hit {
					cc.LastMessage = truncate(strings.ReplaceAll(m.Text, "\n", " "), 160)
					cc.LastAt = m.Timestamp.UTC().Format(time.RFC3339)
					cc.LastFrom = "them"
					if m.FromMe {
						cc.LastFrom = "you"
					}
				}
				view.Chats = append(view.Chats, cc)
			}
			sort.Slice(view.Chats, func(i, j int) bool { return view.Chats[i].LastAt > view.Chats[j].LastAt })

			synced := mirrorPopulated(ctx, db)
			if !view.Connected && view.InviteSentAt == "" && view.InviteGotAt == "" && len(view.Chats) == 0 {
				if synced {
					view.Note = fmt.Sprintf("no local record matching %q; widen the query or sync more resources", query)
				} else {
					view.Note = "the local mirror is empty; run 'unipile-pp-cli sync' before looking anyone up"
				}
			}

			// Nothing matched: report it as not-found (exit 3) rather than a
			// successful empty answer, so callers can branch on the exit code.
			// An empty mirror is not a failed lookup, so only a real miss on a
			// populated mirror earns the not-found exit code.
			notFound := view.Note != "" && synced
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
				if notFound {
					return notFoundErr(fmt.Errorf("no local record matching %q", query))
				}
				return nil
			}
			if notFound {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return notFoundErr(fmt.Errorf("no local record matching %q", query))
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s\n", view.Name)
			if view.Headline != "" {
				fmt.Fprintf(out, "%s\n", view.Headline)
			}
			if view.ProfileURL != "" {
				fmt.Fprintf(out, "%s\n", view.ProfileURL)
			}
			fmt.Fprintln(out)
			fmt.Fprintf(out, "connected:          %t", view.Connected)
			if view.ConnectedAt != "" {
				fmt.Fprintf(out, " (since %s)", view.ConnectedAt)
			}
			fmt.Fprintln(out)
			if view.InviteSentAt != "" {
				fmt.Fprintf(out, "invitation sent:    %s\n", view.InviteSentAt)
			}
			if view.InviteGotAt != "" {
				fmt.Fprintf(out, "invitation from:    %s\n", view.InviteGotAt)
			}
			if len(view.Chats) == 0 {
				fmt.Fprintln(out, "conversations:      none in the local mirror")
			} else {
				fmt.Fprintln(out, "\nCONVERSATIONS")
				tw := newTabWriter(out)
				fmt.Fprintln(tw, "PROVIDER\tMSGS\tFROM YOU\tFROM THEM\tLAST")
				for _, c := range view.Chats {
					fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%s\n", c.Provider, c.Messages, c.FromYou, c.FromThem, truncate(c.LastMessage, 50))
				}
				if err := tw.Flush(); err != nil {
					return err
				}
			}
			if len(view.Candidates) > 0 {
				fmt.Fprintf(out, "\nother matches: %s\n", strings.Join(view.Candidates, ", "))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory)")
	return cmd
}
