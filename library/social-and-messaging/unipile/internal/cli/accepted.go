// Copyright 2026 fuushyn and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type acceptedEntry struct {
	MemberID    string `json:"member_id"`
	Name        string `json:"name"`
	Headline    string `json:"headline,omitempty"`
	ProfileURL  string `json:"profile_url,omitempty"`
	ConnectedAt string `json:"connected_at,omitempty"`
	DaysAgo     int    `json:"days_ago"`
	HasChat     bool   `json:"has_chat"`
}

func newNovelAcceptedCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath      string
		since       string
		limit       int
		includeSent bool
	)
	cmd := &cobra.Command{
		Use:   "accepted",
		Short: "New connections you have not messaged yet",
		Long: strings.Trim(`
Lists LinkedIn connections formed inside the --since window that you have never
exchanged a message with, computed by diffing the synced relation set against
your synced conversations.

Unipile's own guidance warns that polling the relations endpoint on a fixed
schedule looks like automation; this reads the local mirror instead.

Use this command to build the post-acceptance follow-up queue. Do NOT use it to
send the follow-up; pipe the ids into 'chats send-message'.`, "\n"),
		Example: strings.Trim(`
  unipile-pp-cli accepted --since 7d --agent
  unipile-pp-cli accepted --since 30d --include-messaged
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--since=7d;--limit=5",
			"pp:typed-exit-codes": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "accepted")
			}
			now := time.Now().UTC()
			cutoff, err := parseWindow(now, since)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--since %q is not a valid duration (try 7d, 2w, 48h)", since))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			entries := make([]acceptedEntry, 0)
			db, ok, err := novelStore(cmd, flags, dbPath, entries)
			if err != nil || !ok {
				return err
			}
			defer db.Close()

			relations, err := loadRelations(ctx, db)
			if err != nil {
				return err
			}
			chats, err := loadChats(ctx, db, false)
			if err != nil {
				return err
			}
			_, inbound, outbound, err := loadLastMessages(ctx, db)
			if err != nil {
				return err
			}
			// A member counts as "already talked to" when any chat with them
			// carries at least one message in either direction.
			talked := make(map[string]bool, len(chats))
			for _, c := range chats {
				if c.AttendeeID == "" {
					continue
				}
				if inbound[c.ID]+outbound[c.ID] > 0 {
					talked[c.AttendeeID] = true
				}
			}

			for _, r := range relations {
				if r.CreatedAt == "" {
					continue
				}
				when, perr := time.Parse(time.RFC3339, r.CreatedAt)
				if perr != nil {
					continue
				}
				if !cutoff.IsZero() && when.Before(cutoff) {
					continue
				}
				hasChat := talked[r.MemberID]
				if hasChat && !includeSent {
					continue
				}
				entries = append(entries, acceptedEntry{
					MemberID:    r.MemberID,
					Name:        r.Name,
					Headline:    r.Headline,
					ProfileURL:  r.ProfileURL,
					ConnectedAt: r.CreatedAt,
					DaysAgo:     int(now.Sub(when).Hours() / 24),
					HasChat:     hasChat,
				})
			}
			sort.Slice(entries, func(i, j int) bool { return entries[i].ConnectedAt > entries[j].ConnectedAt })
			if limit > 0 && len(entries) > limit {
				entries = entries[:limit]
			}

			emptyMirrorHint(ctx, cmd, db, len(entries))
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), entries, flags)
			}
			if len(entries) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No un-messaged connections formed in the last %s.\n", since)
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "DAYS AGO\tNAME\tHEADLINE\tPROFILE")
			for _, e := range entries {
				fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", e.DaysAgo, truncate(e.Name, 26), truncate(e.Headline, 46), e.ProfileURL)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory)")
	cmd.Flags().StringVar(&since, "since", "7d", "how far back to look for new connections (7d, 2w, 48h)")
	cmd.Flags().IntVar(&limit, "limit", 50, "maximum connections to return")
	cmd.Flags().BoolVar(&includeSent, "include-messaged", false, "also include connections you have already exchanged messages with")
	return cmd
}
