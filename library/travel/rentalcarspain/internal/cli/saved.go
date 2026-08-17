// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: named saved searches (feeds watch and drift).

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/store"

	"github.com/spf13/cobra"
)

func newNovelSavedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "saved",
		Short: "Name and store a recurring Malaga search so watch and drift can re-run it",
		Long:  "Manage named searches kept in the local store. A saved search records a location code, dates and driver age under a name that 'watch' and 'drift' can re-run.",
		Example: "  rentalcarspain-pp-cli saved add agp-august MAL02 20/08/2026 27/08/2026\n" +
			"  rentalcarspain-pp-cli saved list --agent\n" +
			"  rentalcarspain-pp-cli saved remove agp-august",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newSavedAddCmd(flags))
	cmd.AddCommand(newSavedListCmd(flags))
	cmd.AddCommand(newSavedRemoveCmd(flags))
	return cmd
}

func newSavedAddCmd(flags *rootFlags) *cobra.Command {
	var driverAge int
	var dropoffCode string
	cmd := &cobra.Command{
		Use:         "add <name> <location-code> <pickup> <dropoff>",
		Short:       "Save a named search",
		Example:     "  rentalcarspain-pp-cli saved add agp-august MAL02 20/08/2026 27/08/2026",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 4 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("saved add needs <name> <location-code> <pickup> <dropoff>"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := store.OpenWithContext(ctx, defaultDBPath("rentalcarspain-pp-cli"))
			if err != nil {
				return configErr(err)
			}
			defer db.Close()
			ss := store.SavedSearch{
				Name: args[0], LocationCode: args[1], DropoffCode: dropoffCode,
				Pickup: args[2], Dropoff: args[3], DriverAge: driverAge,
			}
			if err := db.AddSavedSearch(ctx, ss); err != nil {
				return apiErr(err)
			}
			if wantsMachineOutput(flags) || flags.asJSON {
				b, _ := json.Marshal(map[string]any{"saved": ss})
				return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved %q: %s %s → %s (age %d)\n", ss.Name, ss.LocationCode, ss.Pickup, ss.Dropoff, ss.DriverAge)
			return nil
		},
	}
	cmd.Flags().IntVar(&driverAge, "driver-age", 35, "Driver age (used for eligibility/validation; under-25 surcharges are charged at the counter, not in the quote)")
	cmd.Flags().StringVar(&dropoffCode, "dropoff-code", "", "Dropoff location code (defaults to pickup)")
	return cmd
}

func newSavedListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List saved searches",
		Example:     "  rentalcarspain-pp-cli saved list --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := store.OpenWithContext(ctx, defaultDBPath("rentalcarspain-pp-cli"))
			if err != nil {
				return configErr(err)
			}
			defer db.Close()
			list, err := db.ListSavedSearches(ctx)
			if err != nil {
				return apiErr(err)
			}
			if wantsMachineOutput(flags) || flags.asJSON {
				b, _ := json.Marshal(map[string]any{"count": len(list), "saved": list})
				return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
			}
			w := cmd.OutOrStdout()
			if len(list) == 0 {
				fmt.Fprintln(w, "No saved searches. Add one: rentalcarspain-pp-cli saved add <name> <code> <pickup> <dropoff>")
				return nil
			}
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "NAME\tLOCATION\tPICKUP\tDROPOFF\tAGE")
			for _, s := range list {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\n", s.Name, s.LocationCode, s.Pickup, s.Dropoff, s.DriverAge)
			}
			return tw.Flush()
		},
	}
	return cmd
}

func newSavedRemoveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "remove <name>",
		Short:       "Remove a saved search",
		Example:     "  rentalcarspain-pp-cli saved remove agp-august",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("saved remove needs <name>"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := store.OpenWithContext(ctx, defaultDBPath("rentalcarspain-pp-cli"))
			if err != nil {
				return configErr(err)
			}
			defer db.Close()
			removed, err := db.RemoveSavedSearch(ctx, args[0])
			if err != nil {
				return apiErr(err)
			}
			if wantsMachineOutput(flags) || flags.asJSON {
				b, _ := json.Marshal(map[string]any{"name": args[0], "removed": removed})
				return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
			}
			if !removed {
				fmt.Fprintf(cmd.OutOrStdout(), "No saved search named %q\n", args[0])
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %q\n", args[0])
			return nil
		},
	}
	return cmd
}

// parseNightsFlag is shared by dates; kept here to avoid a tiny extra file.
func parseNightsFlag(s string) (int, error) {
	if s == "" {
		return 7, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("--nights must be a positive integer")
	}
	return n, nil
}
