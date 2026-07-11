// Copyright 2026 erikgunawans and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: khatam reading plan (finish the Qur'an in N days) with local progress.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/education/quranku/internal/client"
	"github.com/mvanhorn/printing-press-library/library/education/quranku/internal/store"
)

type qkSurahMeta struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	NumberOfVerses int    `json:"numberOfVerses"`
}

type qkPlan struct {
	Days            int    `json:"days"`
	Started         string `json:"started"`
	TotalVerses     int    `json:"total_verses"`
	PerDay          int    `json:"per_day"`
	CompletedVerses int    `json:"completed_verses"`
	// LastAdvanced is the UTC date (YYYY-MM-DD) of the most recent 'plan
	// advance'. It makes advancing idempotent per day so a retry can't skip a
	// reading range. Empty on plans created before this field existed.
	LastAdvanced string `json:"last_advanced,omitempty"`
}

// qkSurahCacheKey is the plan-private resource type used to cache surah
// metadata fetched live by the plan commands. It is deliberately separate from
// the shared "surah" cache that 'sync' owns, so a plan fetch never overwrites
// or downgrades richer synced surah rows.
const qkSurahCacheKey = "plan_surah_meta"

// qkReadSurahMeta loads valid surah metadata rows of the given resource type.
func qkReadSurahMeta(s *store.Store, resourceType string) []qkSurahMeta {
	rows, err := s.List(resourceType, 0)
	if err != nil {
		return nil
	}
	var metas []qkSurahMeta
	for _, r := range rows {
		var m qkSurahMeta
		if json.Unmarshal(r, &m) == nil && m.ID > 0 && m.NumberOfVerses > 0 {
			metas = append(metas, m)
		}
	}
	return metas
}

// qkLoadSurahMeta returns ordered surah metadata (id, name, verse count),
// preferring the shared synced "surah" cache, then the plan-private cache,
// then one live fetch of the surah list which it caches plan-privately so
// later offline plan calls do not refetch and fail.
func qkLoadSurahMeta(ctx context.Context, c *client.Client, s *store.Store) ([]qkSurahMeta, error) {
	metas := qkReadSurahMeta(s, "surah")
	if len(metas) < 114 {
		if cached := qkReadSurahMeta(s, qkSurahCacheKey); len(cached) >= 114 {
			metas = cached
		}
	}
	if len(metas) < 114 {
		raw, err := c.Get(ctx, "/surahs", nil)
		if err != nil {
			return nil, fmt.Errorf("fetching surah list: %w", err)
		}
		var resp struct {
			Data []qkSurahMeta `json:"data"`
		}
		if err := json.Unmarshal(raw, &resp); err != nil {
			return nil, fmt.Errorf("parsing surah list: %w", err)
		}
		metas = resp.Data
		// Cache into the plan-private resource so later offline plan calls read
		// it locally, without touching the shared "surah" cache that 'sync'
		// owns. Best-effort: a cache write failure must not break the command.
		for _, m := range metas {
			if m.ID <= 0 {
				continue
			}
			if item, mErr := json.Marshal(m); mErr == nil {
				_ = s.Upsert(qkSurahCacheKey, strconv.Itoa(m.ID), item)
			}
		}
	}
	sort.Slice(metas, func(i, j int) bool { return metas[i].ID < metas[j].ID })
	return metas, nil
}

// qkGlobalToRef maps a 1-based global verse index to "surah:verse".
func qkGlobalToRef(metas []qkSurahMeta, global int) string {
	remaining := global
	for _, m := range metas {
		if remaining <= m.NumberOfVerses {
			return fmt.Sprintf("%d:%d", m.ID, remaining)
		}
		remaining -= m.NumberOfVerses
	}
	return ""
}

func qkTotalVerses(metas []qkSurahMeta) int {
	total := 0
	for _, m := range metas {
		total += m.NumberOfVerses
	}
	return total
}

func qkLoadPlan(s *store.Store) (*qkPlan, error) {
	raw, err := s.Get("plan_state", "current")
	if err != nil {
		return nil, nil
	}
	var p qkPlan
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// pp:data-source computed
func newNovelPlanCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Build and follow a plan to finish the Qur'an in N days, tracking progress locally.",
		Long: "Create a khatam reading plan that finishes the Qur'an in N days, then track your daily " +
			"progress. Use 'plan today' to see today's reading and 'plan advance' to record it as done.",
		Example:     "  quranku-pp-cli plan start --days 30",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newPlanStartCmd(flags))
	cmd.AddCommand(newPlanStatusCmd(flags))
	cmd.AddCommand(newPlanTodayCmd(flags))
	cmd.AddCommand(newPlanAdvanceCmd(flags))
	return cmd
}

func newPlanStartCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:         "start",
		Short:       "Start a new khatam plan finishing in --days days",
		Example:     "  quranku-pp-cli plan start --days 30",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return qkDryRun(cmd, flags, "start a reading plan")
			}
			if days < 1 {
				return qkInputError(cmd, flags, "--days must be at least 1")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			s, err := qkOpenStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			metas, err := qkLoadSurahMeta(ctx, c, s)
			if err != nil {
				return err
			}
			total := qkTotalVerses(metas)
			if total == 0 {
				return fmt.Errorf("could not determine total verse count")
			}
			perDay := (total + days - 1) / days
			p := qkPlan{
				Days:            days,
				Started:         time.Now().UTC().Format("2006-01-02"),
				TotalVerses:     total,
				PerDay:          perDay,
				CompletedVerses: 0,
			}
			b, _ := json.Marshal(p)
			if err := s.Upsert("plan_state", "current", b); err != nil {
				return err
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), p, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "started a %d-day plan: ~%d verses/day (%d total)\n", days, perDay, total)
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 0, "number of days to finish the Qur'an in")
	return cmd
}

func newPlanStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "status",
		Short:       "Show plan progress and whether you are on track",
		Example:     "  quranku-pp-cli plan status --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return qkDryRun(cmd, flags, "show plan status")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			s, err := qkOpenStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			p, err := qkLoadPlan(s)
			if err != nil {
				return err
			}
			if p == nil {
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "null")
					return nil
				}
				fmt.Fprintln(cmd.OutOrStdout(), "no active plan; run: quranku-pp-cli plan start --days 30")
				return nil
			}
			daysElapsed := 0
			if started, perr := time.Parse("2006-01-02", p.Started); perr == nil {
				daysElapsed = int(time.Since(started).Hours()/24) + 1
			}
			expected := p.PerDay * daysElapsed
			if expected > p.TotalVerses {
				expected = p.TotalVerses
			}
			onTrack := p.CompletedVerses >= expected
			view := map[string]any{
				"days":             p.Days,
				"day_number":       daysElapsed,
				"completed_verses": p.CompletedVerses,
				"total_verses":     p.TotalVerses,
				"expected_verses":  expected,
				"on_track":         onTrack,
				"percent":          fmt.Sprintf("%.1f", float64(p.CompletedVerses)/float64(p.TotalVerses)*100),
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			status := "on track"
			if !onTrack {
				status = fmt.Sprintf("behind by %d verses", expected-p.CompletedVerses)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "day %d/%d — %d/%d verses (%.1f%%), %s\n",
				daysElapsed, p.Days, p.CompletedVerses, p.TotalVerses,
				float64(p.CompletedVerses)/float64(p.TotalVerses)*100, status)
			return nil
		},
	}
}

func newPlanTodayCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "today",
		Short:       "Show the verse range to read today",
		Example:     "  quranku-pp-cli plan today --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return qkDryRun(cmd, flags, "show today's reading")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			s, err := qkOpenStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			p, err := qkLoadPlan(s)
			if err != nil {
				return err
			}
			if p == nil {
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "null")
					return nil
				}
				fmt.Fprintln(cmd.OutOrStdout(), "no active plan; run: quranku-pp-cli plan start --days 30")
				return nil
			}
			if p.CompletedVerses >= p.TotalVerses {
				if flags.asJSON || flags.agent {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"done": true}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "khatam complete — الحمد لله")
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			metas, err := qkLoadSurahMeta(ctx, c, s)
			if err != nil {
				return err
			}
			start := p.CompletedVerses + 1
			end := p.CompletedVerses + p.PerDay
			if end > p.TotalVerses {
				end = p.TotalVerses
			}
			fromRef := qkGlobalToRef(metas, start)
			toRef := qkGlobalToRef(metas, end)
			view := map[string]any{
				"from":         fromRef,
				"to":           toRef,
				"verses_today": end - start + 1,
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "today: read %s to %s (%d verses)\n", fromRef, toRef, end-start+1)
			return nil
		},
	}
}

func newPlanAdvanceCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "advance",
		Short:       "Record today's reading as done and advance progress",
		Example:     "  quranku-pp-cli plan advance",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return qkDryRun(cmd, flags, "advance the plan")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			s, err := qkOpenStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			p, err := qkLoadPlan(s)
			if err != nil {
				return err
			}
			if p == nil {
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "null")
					return nil
				}
				fmt.Fprintln(cmd.OutOrStdout(), "no active plan; run: quranku-pp-cli plan start --days 30")
				return nil
			}
			today := time.Now().UTC().Format("2006-01-02")
			if p.LastAdvanced == today {
				// Already advanced today: a duplicate/retried call must not
				// skip another reading range. Report current state unchanged.
				if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), p, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "already recorded today's reading: %d/%d verses read\n", p.CompletedVerses, p.TotalVerses)
				return nil
			}
			p.LastAdvanced = today
			p.CompletedVerses += p.PerDay
			if p.CompletedVerses > p.TotalVerses {
				p.CompletedVerses = p.TotalVerses
			}
			b, _ := json.Marshal(p)
			if err := s.Upsert("plan_state", "current", b); err != nil {
				return err
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), p, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "advanced: %d/%d verses read\n", p.CompletedVerses, p.TotalVerses)
			return nil
		},
	}
}
