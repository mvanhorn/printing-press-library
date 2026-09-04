// Copyright 2026 Paul Gradeff and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: watch a saved list of public bodies.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/monitoring/utah-pmn/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source auto
func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "watch",
		Short:       "Track a saved list of public bodies and report their meetings",
		Long:        "Maintain a watchlist of public bodies (e.g. Delta City Council, Millard County Planning Commission) and report just their upcoming meetings. Subcommands: add, remove, list, check.",
		Example:     "  utah-pmn-pp-cli watch add \"Millard County Planning Commission\"",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWatchAddCmd(flags))
	cmd.AddCommand(newWatchRemoveCmd(flags))
	cmd.AddCommand(newWatchListCmd(flags))
	cmd.AddCommand(newWatchCheckCmd(flags))
	return cmd
}

// openWatchDB opens the store and ensures the hand-authored tables exist.
func openWatchDB(ctx context.Context, flagDB string) (*store.Store, error) {
	dbPath := flagDB
	if dbPath == "" {
		dbPath = defaultDBPath("utah-pmn-pp-cli")
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	if err := ensurePMNTables(ctx, db.DB()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("preparing tables: %w", err)
	}
	return db, nil
}

func newWatchAddCmd(flags *rootFlags) *cobra.Command {
	var flagDB string
	cmd := &cobra.Command{
		Use:         "add <body>",
		Short:       "Add a public body to the watchlist",
		Example:     "  utah-pmn-pp-cli watch add \"Delta City Council\"",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "body=Delta City Council"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				b, _ := json.Marshal(map[string]any{"dry_run": true})
				return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a body name is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openWatchDB(ctx, flagDB)
			if err != nil {
				return err
			}
			defer db.Close()
			name := strings.ToLower(strings.TrimSpace(strings.Join(args, " ")))
			if err := watchAdd(ctx, db.DB(), name, time.Now().Format(time.RFC3339)); err != nil {
				return fmt.Errorf("adding body: %w", err)
			}
			b, _ := json.Marshal(map[string]string{"added": name})
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path (default: standard cache location)")
	return cmd
}

func newWatchRemoveCmd(flags *rootFlags) *cobra.Command {
	var flagDB string
	cmd := &cobra.Command{
		Use:         "remove <body>",
		Short:       "Remove a public body from the watchlist",
		Example:     "  utah-pmn-pp-cli watch remove \"Delta City Council\"",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "body=Delta City Council"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				b, _ := json.Marshal(map[string]any{"dry_run": true})
				return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a body name is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openWatchDB(ctx, flagDB)
			if err != nil {
				return err
			}
			defer db.Close()
			name := strings.ToLower(strings.TrimSpace(strings.Join(args, " ")))
			n, err := watchRemove(ctx, db.DB(), name)
			if err != nil {
				return fmt.Errorf("removing body: %w", err)
			}
			b, _ := json.Marshal(map[string]any{"removed": name, "rows": n})
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path (default: standard cache location)")
	return cmd
}

func newWatchListCmd(flags *rootFlags) *cobra.Command {
	var flagDB string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List the public bodies on the watchlist",
		Example:     "  utah-pmn-pp-cli watch list --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openWatchDB(ctx, flagDB)
			if err != nil {
				return err
			}
			defer db.Close()
			names, err := watchList(ctx, db.DB())
			if err != nil {
				return fmt.Errorf("listing watchlist: %w", err)
			}
			if names == nil {
				names = []string{}
			}
			b, _ := json.Marshal(names)
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path (default: standard cache location)")
	return cmd
}

func newWatchCheckCmd(flags *rootFlags) *cobra.Command {
	var flagDB string
	var flagLocation string
	var flagDays int
	var flagLimit int
	cmd := &cobra.Command{
		Use:         "check",
		Short:       "Report upcoming meetings for watched bodies",
		Long:        "Sweep notices and keep only meetings whose body matches the watchlist. With --location, scans one ZIP/city; without it, sweeps all Millard County towns.",
		Example:     "  utah-pmn-pp-cli watch check --days 45 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would report meetings for watched bodies")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openWatchDB(ctx, flagDB)
			if err != nil {
				return err
			}
			defer db.Close()
			watched, err := watchList(ctx, db.DB())
			if err != nil {
				return fmt.Errorf("reading watchlist: %w", err)
			}
			if len(watched) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "watchlist is empty; add bodies with: utah-pmn-pp-cli watch add \"<body name>\"")
				if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			start, end := dateWindow(flagDays)
			var notices []pmnNotice
			if flagLocation != "" {
				notices, err = fetchNotices(ctx, c, flagLocation, start, end, flagLimit)
				sortNoticesByDate(notices)
			} else {
				notices, err = sweepLocations(ctx, c, millardCityNames(), start, end, flagLimit)
			}
			if err != nil {
				return classifyAPIError(err, flags)
			}
			out := make([]pmnNotice, 0)
			for _, n := range notices {
				body := strings.ToLower(n.PublicBodyName + " " + n.EntityName)
				for _, w := range watched {
					if w != "" && strings.Contains(body, w) {
						out = append(out, n)
						break
					}
				}
			}
			b, err := json.Marshal(out)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path (default: standard cache location)")
	cmd.Flags().StringVar(&flagLocation, "location", "", "ZIP or city to scan (default: all Millard County towns)")
	cmd.Flags().IntVar(&flagDays, "days", 45, "Days ahead to scan from today")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Max notices per location before matching")
	return cmd
}
