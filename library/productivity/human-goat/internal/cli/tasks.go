// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/human-goat/internal/source/taskrabbit"

	"github.com/spf13/cobra"
)

func newTasksCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "tasks",
		Short:       "List and inspect TaskRabbit bookings",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newTasksListCmd(flags))
	cmd.AddCommand(newTasksGetCmd(flags))
	return cmd
}

func newTasksListCmd(flags *rootFlags) *cobra.Command {
	var flagPage int
	var flagPerPage int
	var flagStatus string

	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List your TaskRabbit bookings, filterable by status",
		Example:     "  human-goat-pp-cli tasks list --status active --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list TaskRabbit tasks")
				return nil
			}
			if flagPage <= 0 {
				return usageErr(fmt.Errorf("--page must be positive"))
			}
			if flagPerPage <= 0 {
				return usageErr(fmt.Errorf("--per-page must be positive"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			tr := taskrabbit.New(c)
			bookings, err := tr.ListTasks(cmd.Context(), flagPage, flagPerPage, map[string]any{}, "en-US")
			if err != nil {
				return classifyAPIError(err, flags)
			}

			rows := make([]taskBookingSummary, 0, len(bookings))
			for _, booking := range bookings {
				if flagStatus != "" && !strings.EqualFold(booking.Status, flagStatus) {
					continue
				}
				rows = append(rows, taskBookingSummary{
					ID:     booking.ID,
					Status: booking.Status,
				})
			}
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			tableRows := make([][]string, 0, len(rows))
			for _, row := range rows {
				tableRows = append(tableRows, []string{row.ID, row.Status})
			}
			return flags.printTable(cmd, []string{"ID", "STATUS"}, tableRows)
		},
	}
	cmd.Flags().IntVar(&flagPage, "page", 1, "Page number")
	cmd.Flags().IntVar(&flagPerPage, "per-page", 20, "Results per page")
	cmd.Flags().StringVar(&flagStatus, "status", "", "Filter returned bookings by status")
	return cmd
}

func newTasksGetCmd(flags *rootFlags) *cobra.Command {
	var flagPage int
	var flagPerPage int

	cmd := &cobra.Command{
		Use:         "get <id>",
		Short:       "Get a TaskRabbit booking from the task list",
		Example:     "  human-goat-pp-cli tasks get 35292888 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !commandHasChangedFlags(cmd) {
				return cmd.Help()
			}
			id := strings.TrimSpace(strings.Join(args, " "))
			if dryRunOK(flags) {
				if id == "" {
					id = "<id>"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would get TaskRabbit task %s\n", id)
				return nil
			}
			if id == "" {
				return usageErr(fmt.Errorf("missing id"))
			}
			if flagPage <= 0 {
				return usageErr(fmt.Errorf("--page must be positive"))
			}
			if flagPerPage <= 0 {
				return usageErr(fmt.Errorf("--per-page must be positive"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			tr := taskrabbit.New(c)
			bookings, err := tr.ListTasks(cmd.Context(), flagPage, flagPerPage, map[string]any{}, "en-US")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			for _, booking := range bookings {
				if booking.ID != id {
					continue
				}
				if flags.asJSON || flags.agent {
					if len(booking.Raw) > 0 {
						return printJSONFiltered(cmd.OutOrStdout(), booking.Raw, flags)
					}
					return printJSONFiltered(cmd.OutOrStdout(), booking, flags)
				}
				return flags.printTable(cmd, []string{"ID", "STATUS"}, [][]string{{booking.ID, booking.Status}})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "no booking %s in the first %d results; try --per-page higher\n", id, flagPerPage)
			return nil
		},
	}
	cmd.Flags().IntVar(&flagPage, "page", 1, "Page number")
	cmd.Flags().IntVar(&flagPerPage, "per-page", 20, "Results per page")
	return cmd
}

type taskBookingSummary struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}
