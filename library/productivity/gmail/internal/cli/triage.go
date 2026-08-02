// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: unread triage summary grouped by sender and category.
// Absorbs gws +triage with agent-shaped output.
package cli

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/gmail/internal/gmailmail"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newTriageCmd(flags))
	})
}

type triageSender struct {
	Sender   string       `json:"sender"`
	Count    int          `json:"count"`
	Newest   string       `json:"newest"`
	Subjects []string     `json:"subjects"`
	Messages []messageRow `json:"messages,omitempty"`
}

type triageOutput struct {
	Query      string         `json:"query"`
	Total      int            `json:"total_unread"`
	Categories map[string]int `json:"categories,omitempty"`
	Senders    []triageSender `json:"senders"`
}

var triageCategories = []string{"personal", "social", "promotions", "updates", "forums"}

func newTriageCmd(flags *rootFlags) *cobra.Command {
	var query string
	var limit int
	var detail bool

	cmd := &cobra.Command{
		Use:   "triage",
		Short: "Unread summary grouped by sender and category",
		Long: `Fetches unread inbox mail and groups it by sender with per-category
counts, so an agent (or you) can decide what to read, archive, or reply to
in one pass.`,
		Example: strings.Trim(`
  gmail-pp-cli triage
  gmail-pp-cli triage --agent
  gmail-pp-cli triage --query "is:unread category:primary" --detail`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "summarize unread mail by sender and category")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			msgs, skipped, err := gmailSearchMetadata(cmd.Context(), c, query, limit)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			warnSkipped(cmd, skipped)
			out := triageOutput{Query: query, Total: len(msgs), Categories: map[string]int{}}
			bySender := map[string]*triageSender{}
			for i := range msgs {
				m := &msgs[i]
				sender := gmailmail.ExtractEmail(m.Header("From"))
				if sender == "" {
					sender = "(unknown)"
				}
				s, ok := bySender[sender]
				if !ok {
					s = &triageSender{Sender: sender}
					bySender[sender] = s
				}
				s.Count++
				if t, ok := m.InternalTime(); ok {
					ts := t.Local().Format("2006-01-02 15:04")
					if ts > s.Newest {
						s.Newest = ts
					}
				}
				if len(s.Subjects) < 3 {
					s.Subjects = append(s.Subjects, m.Header("Subject"))
				}
				for _, cat := range triageCategories {
					if m.HasLabel("CATEGORY_" + strings.ToUpper(cat)) {
						out.Categories[cat]++
						break
					}
				}
				if detail {
					rows := toMessageRows([]gmailmail.Message{*m})
					s.Messages = append(s.Messages, rows...)
				}
			}
			for _, s := range bySender {
				out.Senders = append(out.Senders, *s)
			}
			sort.Slice(out.Senders, func(i, j int) bool {
				if out.Senders[i].Count != out.Senders[j].Count {
					return out.Senders[i].Count > out.Senders[j].Count
				}
				return out.Senders[i].Sender < out.Senders[j].Sender
			})
			if wantsJSONOutput(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%d unread message(s)\n", out.Total)
			if len(out.Categories) > 0 {
				var parts []string
				for _, cat := range triageCategories {
					if n := out.Categories[cat]; n > 0 {
						parts = append(parts, fmt.Sprintf("%s %d", cat, n))
					}
				}
				fmt.Fprintln(w, "categories:", strings.Join(parts, ", "))
			}
			fmt.Fprintln(w)
			tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "COUNT\tSENDER\tNEWEST\tSUBJECTS")
			for _, s := range out.Senders {
				fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", s.Count, s.Sender, s.Newest, truncateCell(strings.Join(s.Subjects, " | "), 70))
			}
			tw.Flush()
			return nil
		},
	}
	cmd.Flags().StringVar(&query, "query", "is:unread in:inbox", "Gmail query defining the triage set")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum messages to summarize")
	cmd.Flags().BoolVar(&detail, "detail", false, "Include per-message rows under each sender")
	return cmd
}
