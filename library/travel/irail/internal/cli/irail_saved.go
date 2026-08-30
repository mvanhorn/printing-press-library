// Copyright 2026 Olivier and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored command group: saved.
//
// Named route shortcuts, absorbing commandtrein's `shortcut add/list`. Stored
// in SQLite rather than a config file so `observe` can pick them up and build
// history for the routes you actually travel.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/irail/internal/store"
)

func newIrailSavedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "saved",
		Short: "Named route shortcuts reused by observe and the planning commands",
		Long: "Stores named shortcuts such as 'commute' = Ghent-Sint-Pieters -> Brussels-Central.\n\n" +
			"A bare 'observe' records every saved route, which is what makes 'punctuality'\n" +
			"accumulate history for the journeys you actually take.",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newIrailSavedAddCmd(flags), newIrailSavedListCmd(flags), newIrailSavedRemoveCmd(flags))
	return cmd
}

func newIrailSavedAddCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "add [name] [from] [to]",
		Short: "Save a named route or station shortcut",
		Long: "Saves a shortcut. Provide a destination for a route, or omit it to save a\n" +
			"station whose board should be observed.",
		Example: `  irail-pp-cli saved add commute Ghent-Sint-Pieters Brussels-Central
  irail-pp-cli saved add home Leuven`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a name and at least an origin station are required"))
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would save shortcut %q\n", args[0])
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if dbPath == "" {
				dbPath = defaultDBPath("irail-pp-cli")
			}

			route := store.SavedRoute{
				Name:        args[0],
				FromStation: resolveStationName(args[1]),
				CreatedAt:   time.Now().Unix(),
			}
			if len(args) > 2 {
				route.ToStation = resolveStationName(args[2])
			}

			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer func() { _ = db.Close() }()
			if err := db.EnsureIrailSchema(ctx); err != nil {
				return err
			}
			if err := db.SaveRoute(ctx, route); err != nil {
				return err
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), route, flags)
			}
			if route.ToStation != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "saved %q: %s -> %s\n", route.Name, route.FromStation, route.ToStation)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "saved %q: %s\n", route.Name, route.FromStation)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newIrailSavedListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List every saved route and station shortcut",
		Example:     "  irail-pp-cli saved list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list saved shortcuts")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			routes, err := loadSavedRoutes(ctx, dbPath)
			if err != nil {
				return err
			}
			if routes == nil {
				routes = []store.SavedRoute{}
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), routes, flags)
			}
			if len(routes) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(),
					"no shortcuts saved yet; add one with: irail-pp-cli saved add commute <from> <to>")
				return nil
			}
			for _, r := range routes {
				if r.ToStation != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "%-16s %s -> %s\n", r.Name, r.FromStation, r.ToStation)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-16s %s\n", r.Name, r.FromStation)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newIrailSavedRemoveCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "remove [name]",
		Short:       "Delete a saved shortcut",
		Example:     "  irail-pp-cli saved remove commute",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("the shortcut name is required"))
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would remove shortcut %q\n", args[0])
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if dbPath == "" {
				dbPath = defaultDBPath("irail-pp-cli")
			}

			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer func() { _ = db.Close() }()
			if err := db.EnsureIrailSchema(ctx); err != nil {
				return err
			}
			removed, err := db.DeleteSavedRoute(ctx, args[0])
			if err != nil {
				return err
			}
			if !removed {
				return notFoundErr(fmt.Errorf("no shortcut named %q", args[0]))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %q\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
