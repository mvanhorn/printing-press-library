// Copyright 2026 andrea-m-piovesana. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newStaleCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:   "stale",
		Short: "Open tickets with no activity in N days",
		Example: `  tag-helpdesk-pp-cli stale
  tag-helpdesk-pp-cli stale --days 14 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStore()
			if err != nil {
				return err
			}
			defer db.Close()

			tickets, err := db.StaleTickets(days)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(tickets)
			}
			if len(tickets) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No stale tickets (no activity in last %d days).\n", days)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d tickets with no activity in last %d days:\n\n", len(tickets), days)
			printTicketTable(cmd.OutOrStdout(), tickets)
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 7, "Inactivity threshold in days")
	return cmd
}

func newUnattendedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unattended",
		Short: "Tickets stuck in unattended stages",
		Example: `  tag-helpdesk-pp-cli unattended
  tag-helpdesk-pp-cli unattended --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStore()
			if err != nil {
				return err
			}
			defer db.Close()

			tickets, err := db.UnattendedTickets()
			if err != nil {
				return err
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(tickets)
			}
			if len(tickets) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No unattended tickets.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d unattended tickets:\n\n", len(tickets))
			printTicketTable(cmd.OutOrStdout(), tickets)
			return nil
		},
	}
	return cmd
}

func newOverdueCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:   "overdue",
		Short: "High/urgent priority open tickets older than N days",
		Example: `  tag-helpdesk-pp-cli overdue
  tag-helpdesk-pp-cli overdue --days 3 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStore()
			if err != nil {
				return err
			}
			defer db.Close()

			tickets, err := db.OverdueTickets(days)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(tickets)
			}
			if len(tickets) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No high/urgent tickets older than %d days.\n", days)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d high/urgent tickets older than %d days:\n\n", len(tickets), days)
			printTicketTable(cmd.OutOrStdout(), tickets)
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 5, "Age threshold in days")
	return cmd
}

func newByAgentCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "by-agent",
		Short: "Agent workload: open ticket count and average age",
		Example: `  tag-helpdesk-pp-cli by-agent
  tag-helpdesk-pp-cli by-agent --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStore()
			if err != nil {
				return err
			}
			defer db.Close()

			summaries, err := db.ByAgent()
			if err != nil {
				return err
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(summaries)
			}
			if len(summaries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No open tickets found.")
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "AGENT\tOPEN TICKETS\tAVG AGE (DAYS)")
			for _, s := range summaries {
				name := s.UserName
				if name == "" {
					name = "(unassigned)"
				}
				fmt.Fprintf(w, "%s\t%d\t%.1f\n", name, s.OpenCount, s.AvgAgeDays)
			}
			w.Flush()
			return nil
		},
	}
	return cmd
}

func newByTeamCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "by-team",
		Short: "Open ticket counts grouped by team",
		Example: `  tag-helpdesk-pp-cli by-team
  tag-helpdesk-pp-cli by-team --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStore()
			if err != nil {
				return err
			}
			defer db.Close()

			summaries, err := db.ByTeam()
			if err != nil {
				return err
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(summaries)
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "TEAM\tOPEN TICKETS")
			for _, s := range summaries {
				name := s.TeamName
				if name == "" {
					name = "(no team)"
				}
				fmt.Fprintf(w, "%s\t%d\n", name, s.OpenCount)
			}
			w.Flush()
			return nil
		},
	}
	return cmd
}

func newByCategoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "by-category",
		Short: "Open ticket counts grouped by category",
		Example: `  tag-helpdesk-pp-cli by-category
  tag-helpdesk-pp-cli by-category --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStore()
			if err != nil {
				return err
			}
			defer db.Close()

			summaries, err := db.ByCategory()
			if err != nil {
				return err
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(summaries)
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "CATEGORY\tOPEN TICKETS")
			for _, s := range summaries {
				fmt.Fprintf(w, "%s\t%d\n", s.CategoryName, s.OpenCount)
			}
			w.Flush()
			return nil
		},
	}
	return cmd
}
