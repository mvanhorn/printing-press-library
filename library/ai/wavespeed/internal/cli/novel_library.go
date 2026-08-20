// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/wavespeed/internal/store"
	"github.com/spf13/cobra"
)

func newLibraryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "library",
		Short: "Query and manage the local generation library",
		Long:  "The library records every novel-command generation. Search, filter, tag, export, and roll up cost across brand/model/platform/tag.",
	}
	cmd.AddCommand(
		newLibraryListCmd(flags),
		newLibrarySearchCmd(flags),
		newLibraryShowCmd(flags),
		newLibraryTagCmd(flags),
		newLibraryExportCmd(flags),
		newLibraryCostReportCmd(flags),
	)
	return cmd
}

func newLibraryListCmd(flags *rootFlags) *cobra.Command {
	var (
		brand, platform, model, tag, since string
		limit                              int
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recorded generations (newest first)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openLibrary()
			if err != nil {
				return configErr(err)
			}
			defer s.Close()
			sinceTime, err := parseSince(since)
			if err != nil {
				return usageErr(err)
			}
			gens, err := s.ListGenerations(libraryFilter(brand, platform, model, tag, sinceTime, limit))
			if err != nil {
				return apiErr(err)
			}
			env := newEnvelope("library list")
			for _, g := range gens {
				env.Results = append(env.Results, g)
			}
			env.RecommendedAction = fmt.Sprintf("count: %d", len(gens))
			return emitEnvelope(cmd.OutOrStdout(), env)
		},
	}
	cmd.Flags().StringVar(&brand, "brand", "", "Filter by brand")
	cmd.Flags().StringVar(&platform, "platform", "", "Filter by platform")
	cmd.Flags().StringVar(&model, "model", "", "Filter by model ID")
	cmd.Flags().StringVar(&tag, "tag", "", "Filter by tag")
	cmd.Flags().StringVar(&since, "since", "", "Only since (e.g. 30d, 24h, or 2026-05-01)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum rows")
	return cmd
}

func newLibrarySearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search generation prompts (FTS5)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openLibrary()
			if err != nil {
				return configErr(err)
			}
			defer s.Close()
			query := strings.Join(args, " ")
			gens, err := s.SearchGenerations(query, limit)
			if err != nil {
				return usageErr(fmt.Errorf("search failed (check FTS5 query syntax): %w", err))
			}
			env := newEnvelope("library search")
			for _, g := range gens {
				env.Results = append(env.Results, g)
			}
			env.RecommendedAction = fmt.Sprintf("count: %d", len(gens))
			return emitEnvelope(cmd.OutOrStdout(), env)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum rows")
	return cmd
}

func newLibraryShowCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show one generation with its tags",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openLibrary()
			if err != nil {
				return configErr(err)
			}
			defer s.Close()
			g, err := s.GetGeneration(args[0])
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return notFoundErr(fmt.Errorf("generation %q not found", args[0]))
				}
				return apiErr(err)
			}
			env := newEnvelope("library show")
			env.Results = []any{g}
			return emitEnvelope(cmd.OutOrStdout(), env)
		},
	}
}

func newLibraryTagCmd(flags *rootFlags) *cobra.Command {
	var add, remove []string
	cmd := &cobra.Command{
		Use:   "tag <id>",
		Short: "Add or remove tags on a generation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(add) == 0 && len(remove) == 0 {
				return usageErr(fmt.Errorf("pass --add and/or --remove"))
			}
			s, err := openLibrary()
			if err != nil {
				return configErr(err)
			}
			defer s.Close()
			if _, err := s.GetGeneration(args[0]); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return notFoundErr(fmt.Errorf("generation %q not found", args[0]))
				}
				return apiErr(err)
			}
			for _, t := range add {
				if err := s.AddTag(args[0], t); err != nil {
					return apiErr(err)
				}
			}
			for _, t := range remove {
				if err := s.RemoveTag(args[0], t); err != nil {
					return apiErr(err)
				}
			}
			tags, err := s.TagsFor(args[0])
			if err != nil {
				return apiErr(err)
			}
			env := newEnvelope("library tag")
			env.Results = []any{map[string]any{"id": args[0], "tags": tags}}
			return emitEnvelope(cmd.OutOrStdout(), env)
		},
	}
	cmd.Flags().StringSliceVar(&add, "add", nil, "Tags to add")
	cmd.Flags().StringSliceVar(&remove, "remove", nil, "Tags to remove")
	return cmd
}

func newLibraryExportCmd(flags *rootFlags) *cobra.Command {
	var (
		brand, platform, model, tag, since string
		limit                              int
	)
	cmd := &cobra.Command{
		Use:   "export <dir>",
		Short: "Export matching generations as JSON files",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return notFoundErr(fmt.Errorf("export target %q is not writable: %w", dir, err))
			}
			s, err := openLibrary()
			if err != nil {
				return configErr(err)
			}
			defer s.Close()
			sinceTime, err := parseSince(since)
			if err != nil {
				return usageErr(err)
			}
			gens, err := s.ListGenerations(libraryFilter(brand, platform, model, tag, sinceTime, limit))
			if err != nil {
				return apiErr(err)
			}
			written := 0
			for _, g := range gens {
				path := filepath.Join(dir, g.ID+".json")
				data, _ := json.MarshalIndent(g, "", "  ")
				if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
					return notFoundErr(fmt.Errorf("writing %s: %w", path, err))
				}
				written++
			}
			env := newEnvelope("library export")
			env.Results = []any{map[string]any{"dir": dir, "exported": written}}
			return emitEnvelope(cmd.OutOrStdout(), env)
		},
	}
	cmd.Flags().StringVar(&brand, "brand", "", "Filter by brand")
	cmd.Flags().StringVar(&platform, "platform", "", "Filter by platform")
	cmd.Flags().StringVar(&model, "model", "", "Filter by model ID")
	cmd.Flags().StringVar(&tag, "tag", "", "Filter by tag")
	cmd.Flags().StringVar(&since, "since", "", "Only since (e.g. 30d, 24h, or 2026-05-01)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows (0 = all)")
	return cmd
}

func newLibraryCostReportCmd(flags *rootFlags) *cobra.Command {
	var since, groupBy string
	cmd := &cobra.Command{
		Use:   "cost-report",
		Short: "Roll up cost grouped by brand, model, platform, or tag",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if groupBy == "" {
				groupBy = "brand"
			}
			s, err := openLibrary()
			if err != nil {
				return configErr(err)
			}
			defer s.Close()
			sinceTime, err := parseSince(since)
			if err != nil {
				return usageErr(err)
			}
			rows, err := s.CostReport(sinceTime, groupBy)
			if err != nil {
				return usageErr(err)
			}
			var total float64
			for _, r := range rows {
				total += r.TotalCost
			}
			env := newEnvelope("library cost-report")
			env.CostSpent = total
			env.Results = []any{map[string]any{"group_by": groupBy, "rows": rows, "total_cost": total}}
			return emitEnvelope(cmd.OutOrStdout(), env)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Only since (e.g. 30d, 24h, or 2026-05-01)")
	cmd.Flags().StringVar(&groupBy, "group-by", "brand", "Grouping: brand|model|platform|tag")
	return cmd
}

func libraryFilter(brand, platform, model, tag string, since time.Time, limit int) store.GenerationFilter {
	return store.GenerationFilter{Brand: brand, Platform: platform, Model: model, Tag: tag, Since: since, Limit: limit}
}

// parseSince parses a duration shorthand ("30d", "24h", "90m") or a date
// ("2026-05-01"). Empty string returns the zero time (no lower bound).
func parseSince(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	if len(s) > 1 {
		unit := s[len(s)-1]
		if n, err := strconv.Atoi(s[:len(s)-1]); err == nil {
			switch unit {
			case 'd':
				return time.Now().Add(-time.Duration(n) * 24 * time.Hour), nil
			case 'h':
				return time.Now().Add(-time.Duration(n) * time.Hour), nil
			case 'm':
				return time.Now().Add(-time.Duration(n) * time.Minute), nil
			}
		}
	}
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid --since %q (use 30d, 24h, 90m, or YYYY-MM-DD)", s)
}
