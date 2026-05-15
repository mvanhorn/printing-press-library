// Copyright 2026 andrea-m-piovesana. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/project-management/tag-helpdesk/internal/odoo"
	"github.com/mvanhorn/printing-press-library/library/project-management/tag-helpdesk/internal/store"
)

var priorityLabel = map[string]string{
	"0": "Low",
	"1": "Medium",
	"2": "High",
	"3": "Very High",
}

var reTicketNumber = regexp.MustCompile(`(?i)^[A-Z]+\d+$`)

func newListCmd(flags *rootFlags) *cobra.Command {
	var closed bool
	var open bool
	var unattended bool
	var priority string
	var team string
	var assignee string
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List helpdesk tickets from local cache",
		Long:  "List tickets from the local SQLite cache. Run 'sync' first to populate.\nFilter by status, priority, team, or assignee.",
		Example: `  tag-helpdesk-pp-cli list
  tag-helpdesk-pp-cli list --open --priority 2
  tag-helpdesk-pp-cli list --team Support --limit 20
  tag-helpdesk-pp-cli list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStore()
			if err != nil {
				return err
			}
			defer db.Close()

			f := store.Filter{Limit: limit}
			trueVal := true
			falseVal := false
			if open {
				f.Closed = &falseVal
				f.Active = &trueVal
			} else if closed {
				f.Closed = &trueVal
			}
			if unattended {
				f.Unattended = &trueVal
			}
			if priority != "" {
				f.Priority = priority
			}
			if team != "" {
				f.TeamName = team
			}
			if assignee != "" {
				f.UserName = assignee
			}

			tickets, err := db.ListTickets(f)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(tickets)
			}
			printTicketTable(cmd.OutOrStdout(), tickets)
			return nil
		},
	}
	cmd.Flags().BoolVar(&open, "open", false, "Show only open tickets (default behaviour)")
	cmd.Flags().BoolVar(&closed, "closed", false, "Show only closed tickets")
	cmd.Flags().BoolVar(&unattended, "unattended", false, "Show only unattended tickets")
	cmd.Flags().StringVar(&priority, "priority", "", "Filter by priority: 0=Low, 1=Medium, 2=High, 3=Very High")
	cmd.Flags().StringVar(&team, "team", "", "Filter by team name (substring match)")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Filter by assigned user (substring match)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max tickets to return (0 = all)")
	return cmd
}

func newGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <id|number>",
		Short: "Get full detail for a ticket by ID or ticket number",
		Example: `  tag-helpdesk-pp-cli get 42
  tag-helpdesk-pp-cli get T0042
  tag-helpdesk-pp-cli get 42 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStore()
			if err != nil {
				return err
			}
			defer db.Close()

			var ticket *store.TicketRow
			ref := args[0]
			if reTicketNumber.MatchString(ref) {
				ticket, err = db.GetTicketByNumber(strings.ToUpper(ref))
			} else {
				id, parseErr := strconv.Atoi(ref)
				if parseErr != nil {
					return fmt.Errorf("invalid ticket reference %q: must be an integer ID or ticket number (e.g. T0042)", ref)
				}
				ticket, err = db.GetTicket(id)
			}
			if err != nil {
				return err
			}

			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(ticket)
			}
			printTicketDetail(cmd.OutOrStdout(), ticket)
			return nil
		},
	}
	return cmd
}

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search across ticket content",
		Example: `  tag-helpdesk-pp-cli search "printer offline"
  tag-helpdesk-pp-cli search "mario rossi" --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStore()
			if err != nil {
				return err
			}
			defer db.Close()

			tickets, err := db.SearchTickets(args[0], limit)
			if err != nil {
				return fmt.Errorf("FTS search error: %w\nTip: FTS5 requires exact words or prefix* patterns", err)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(tickets)
			}
			printTicketTable(cmd.OutOrStdout(), tickets)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results")
	return cmd
}

// --- helper printers ---

func printTicketTable(out io.Writer, tickets []store.TicketRow) {
	if len(tickets) == 0 {
		fmt.Fprintln(out, "No tickets found.")
		return
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNUMBER\tPRIORITY\tSTATUS\tASSIGNEE\tTEAM\tTITLE")
	for _, t := range tickets {
		status := "open"
		if t.Closed {
			status = "closed"
		} else if t.Unattended {
			status = "unattended"
		}
		assignee := t.UserName
		if assignee == "" {
			assignee = "(unassigned)"
		}
		title := t.Name
		if len(title) > 50 {
			title = title[:47] + "..."
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
			t.ID, t.Number, priorityLabel[t.Priority], status, assignee, t.TeamName, title)
	}
	w.Flush()
}

func printTicketDetail(out io.Writer, t *store.TicketRow) {
	fmt.Fprintf(out, "Ticket:      #%d — %s\n", t.ID, t.Number)
	fmt.Fprintf(out, "Title:       %s\n", t.Name)
	fmt.Fprintf(out, "Priority:    %s\n", priorityLabel[t.Priority])
	fmt.Fprintf(out, "Stage:       %s\n", t.StageName)
	fmt.Fprintf(out, "Kanban:      %s\n", t.KanbanState)
	fmt.Fprintf(out, "Closed:      %v\n", t.Closed)
	fmt.Fprintf(out, "Unattended:  %v\n", t.Unattended)
	fmt.Fprintf(out, "Team:        %s\n", t.TeamName)
	assignee := t.UserName
	if assignee == "" {
		assignee = "(unassigned)"
	}
	fmt.Fprintf(out, "Assignee:    %s\n", assignee)
	fmt.Fprintf(out, "Customer:    %s <%s>\n", t.PartnerName, t.PartnerEmail)
	fmt.Fprintf(out, "Category:    %s\n", t.CategoryName)
	fmt.Fprintf(out, "Channel:     %s\n", t.ChannelName)
	fmt.Fprintf(out, "Assigned:    %s\n", t.AssignedDate)
	fmt.Fprintf(out, "Last update: %s\n", t.WriteDate)
	fmt.Fprintf(out, "Stage since: %s\n", t.LastStageUpdate)
	if t.Closed {
		fmt.Fprintf(out, "Closed at:   %s\n", t.ClosedDate)
	}
	fmt.Fprintf(out, "\nDescription:\n%s\n", t.DescriptionText)
}

func newAnalyzeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analyze <id|number>",
		Short: "Structured ticket detail + chatter for Claude analysis",
		Long: `Fetches full ticket detail from the local store and chatter messages live from Odoo.
Outputs structured JSON suitable for piping to Claude.

  tag-helpdesk-pp-cli analyze 42 | claude -p "Summarize this helpdesk ticket and suggest next steps"`,
		Example: `  tag-helpdesk-pp-cli analyze 42
  tag-helpdesk-pp-cli analyze HEL00123`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStore()
			if err != nil {
				return err
			}
			defer db.Close()

			var ticket *store.TicketRow
			ref := args[0]
			if reTicketNumber.MatchString(ref) {
				ticket, err = db.GetTicketByNumber(strings.ToUpper(ref))
			} else {
				id, parseErr := strconv.Atoi(ref)
				if parseErr != nil {
					return fmt.Errorf("invalid ticket reference %q", ref)
				}
				ticket, err = db.GetTicket(id)
			}
			if err != nil {
				return err
			}

			// Fetch chatter live from Odoo for freshest messages
			oc, err := odoo.NewFromEnv()
			var messages []map[string]interface{}
			if err == nil {
				if authErr := oc.Authenticate(); authErr == nil {
					messages, _ = oc.GetTicketMessages(ticket.ID)
				}
			}

			// Build Claude-friendly output
			out := map[string]interface{}{
				"ticket":   ticket,
				"messages": messages,
				"meta": map[string]string{
					"priority":    priorityLabel[ticket.Priority],
					"stage":       ticket.StageName,
					"last_update": ticket.WriteDate,
				},
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		},
	}
	return cmd
}

func newBulkAnalyzeCmd(flags *rootFlags) *cobra.Command {
	var closed bool
	var limit int

	cmd := &cobra.Command{
		Use:   "bulk-analyze",
		Short: "Dump all open tickets as JSON lines for batch Claude analysis",
		Long: `Outputs one JSON object per line (JSONL format) for all open tickets.
Each line is a complete ticket record with flattened fields.

  tag-helpdesk-pp-cli bulk-analyze | claude -p "Which tickets need urgent attention?"`,
		Example: `  tag-helpdesk-pp-cli bulk-analyze
  tag-helpdesk-pp-cli bulk-analyze --all --limit 500`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStore()
			if err != nil {
				return err
			}
			defer db.Close()

			falseVal := false
			trueVal := true
			f := store.Filter{Limit: limit}
			if !closed {
				f.Closed = &falseVal
				f.Active = &trueVal
			}

			tickets, err := db.ListTickets(f)
			if err != nil {
				return err
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			for _, t := range tickets {
				if err := enc.Encode(t); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&closed, "all", false, "Include closed tickets")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max tickets (0 = all)")
	return cmd
}

func newSummaryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Structured KPI snapshot of open tickets for Claude pipelines",
		Long: `Outputs a structured JSON summary of helpdesk KPIs from the local cache.
Designed for piping to Claude for analysis:

  tag-helpdesk-pp-cli summary | claude -p "What is the state of our helpdesk?"`,
		Example: `  tag-helpdesk-pp-cli summary
  tag-helpdesk-pp-cli summary --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStore()
			if err != nil {
				return err
			}
			defer db.Close()

			kpis, err := db.GetKPIs()
			if err != nil {
				return err
			}
			byAgent, _ := db.ByAgent()
			byTeam, _ := db.ByTeam()
			byCategory, _ := db.ByCategory()

			out := map[string]interface{}{
				"kpis":        kpis,
				"by_agent":    byAgent,
				"by_team":     byTeam,
				"by_category": byCategory,
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		},
	}
	return cmd
}
