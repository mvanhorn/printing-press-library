// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelHotelsAliasCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "alias",
		Short:       "Give a memorable name to a TravelClick hotel ID instead of memorizing a 6-digit number.",
		Example:     "  travelclick-pp-cli hotels alias add made-nyc 102306",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE:        parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newNovelHotelsAliasAddCmd(flags))
	cmd.AddCommand(newNovelHotelsAliasListCmd(flags))
	cmd.AddCommand(newNovelHotelsAliasRemoveCmd(flags))

	return cmd
}

func newNovelHotelsAliasAddCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "add <alias> <hotel_id>",
		Short:   "Add a new hotel alias",
		Example: "  travelclick-pp-cli hotels alias add made-nyc 102306",
		Annotations: map[string]string{
			"mcp:read-only": "false",
			"pp:happy-args": "alias=made-nyc;hotel_id=102306",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "hotels alias add")
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("both <alias> and <hotel_id> are required"))
			}
			alias := args[0]
			hotelID := args[1]

			// Validate hotelID is numeric
			for _, r := range hotelID {
				if r < '0' || r > '9' {
					return usageErr(fmt.Errorf("hotel_id must be numeric, got %q", hotelID))
				}
			}

			// Validate --data-source is local
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}

			db, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			if err := db.UpsertHotelAlias(cmd.Context(), alias, hotelID); err != nil {
				return err
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"status":   "success",
					"alias":    alias,
					"hotel_id": hotelID,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Successfully added alias %q for hotel %s\n", alias, hotelID)
			return nil
		},
	}
	return cmd
}

func newNovelHotelsAliasListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List all hotel aliases",
		Example: "  travelclick-pp-cli hotels alias list",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "hotels alias list")
			}

			// Validate --data-source is local
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}

			db, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			list, err := db.ListHotelAliases(cmd.Context())
			if err != nil {
				return err
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), list, flags)
			}

			if len(list) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No hotel aliases found.")
				return nil
			}

			var rows [][]string
			for _, item := range list {
				rows = append(rows, []string{item.Alias, item.HotelID, item.CreatedAt})
			}
			return flags.printTable(cmd, []string{"ALIAS", "HOTEL_ID", "CREATED_AT"}, rows)
		},
	}
	return cmd
}

func newNovelHotelsAliasRemoveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove <alias>",
		Short:   "Remove a hotel alias",
		Example: "  travelclick-pp-cli hotels alias remove made-nyc",
		Annotations: map[string]string{
			"mcp:read-only": "false",
			"pp:happy-args": "alias=made-nyc",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "hotels alias remove")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<alias> is required"))
			}
			alias := args[0]

			// Validate --data-source is local
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}

			db, err := openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			removed, err := db.RemoveHotelAlias(cmd.Context(), alias)
			if err != nil {
				return err
			}

			if flags.asJSON {
				status := "not_found"
				if removed {
					status = "success"
				}
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"status": status,
					"alias":  alias,
				}, flags)
			}

			if removed {
				fmt.Fprintf(cmd.OutOrStdout(), "Successfully removed alias %q\n", alias)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Alias %q already removed or not found\n", alias)
			}
			return nil
		},
	}
	return cmd
}
