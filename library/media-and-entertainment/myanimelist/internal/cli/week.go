// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type weekSlot struct {
	LocalDay  string `json:"local_day"`
	LocalTime string `json:"local_time"`
	JSTSlot   string `json:"jst_slot,omitempty"`
	Kind      string `json:"kind"`
	ID        int    `json:"id"`
	Title     string `json:"title,omitempty"`
	Progress  int    `json:"progress,omitempty"`
	Next      int    `json:"next_episode,omitempty"`
	Collision bool   `json:"collision"`
}

type weekView struct {
	Slots    []weekSlot `json:"slots"`
	Timezone string     `json:"timezone"`
	Entries  int        `json:"library_entries"`
	Note     string     `json:"note,omitempty"`
}

var broadcastRE = regexp.MustCompile(`(?i)(sunday|monday|tuesday|wednesday|thursday|friday|saturday)s?\s+at\s+(\d{1,2}):(\d{2})`)

// newNovelWeekCmd builds a timezone-correct weekly grid from the local library.
// MyAnimeList publishes broadcast slots in JST only, so every viewer converts
// by hand; this joins the local library with each title's slot and flags
// same-hour collisions.
func newNovelWeekCmd(flags *rootFlags) *cobra.Command {
	var dbPath, kind string
	var all bool
	cmd := &cobra.Command{
		Use:   "week",
		Short: "Show this week's schedule for the shows you track, in your timezone",
		Long: "Use this command for a timezone-correct weekly grid built from titles in your local library, including broadcast-slot collisions.\n" +
			"Do NOT use this command for a flat list of everything airing in the next N days; use 'airing' instead.",
		Example: "  myanimelist-pp-cli week",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "auto",
			"pp:happy-args":       "--db=:memory:",
			"pp:typed-exit-codes": "0,3",
			"pp:novel-scaffold":   "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "week")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			dbPath = malDBPath(flags, dbPath)
			if !malStoreExists(dbPath) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: myanimelist-pp-cli track add 52991 --status watching --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), weekView{Slots: make([]weekSlot, 0), Timezone: localZone()}, flags)
				}
				return nil
			}
			db, err := malOpenStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			entries, err := malLoadLibrary(ctx, db, kind)
			if err != nil {
				return err
			}
			if !all {
				filtered := entries[:0]
				for _, e := range entries {
					if e.Status == "watching" {
						filtered = append(filtered, e)
					}
				}
				entries = filtered
			}
			if isDogfoodEnv() && len(entries) > 2 {
				entries = entries[:2]
			}
			// A broadcast slot is published only on the live title page. Under
			// an explicit --data-source local the command must stay offline, so
			// entries are reported without a schedule instead of silently
			// fetching live data the user asked it not to fetch.
			localOnly := flags != nil && flags.dataSource == "local"
			slots := make([]weekSlot, 0, len(entries))
			if localOnly {
				for _, e := range entries {
					slots = append(slots, weekSlot{
						Kind: e.Kind, ID: e.ID, Title: e.Title,
						Progress: e.Progress, Next: e.Progress + 1,
						LocalDay: "unscheduled",
					})
				}
			} else {
				// One client for the whole grid: newClient reads the config file
				// and builds a fresh rate limiter on every call, so creating it
				// inside the loop would re-read config and reset the pacer once
				// per tracked title.
				c, cerr := flags.newClient()
				if cerr != nil {
					return cerr
				}
				for _, e := range entries {
					detail, derr := malDetailWith(ctx, c, e.Kind, e.ID)
					if derr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not read %s %d: %v\n", e.Kind, e.ID, derr)
						continue
					}
					slot := weekSlot{Kind: e.Kind, ID: e.ID, Title: detail.Title, Progress: e.Progress, Next: e.Progress + 1}
					if day, clock, ok := convertBroadcast(detail.Broadcast, time.Now()); ok {
						slot.LocalDay, slot.LocalTime, slot.JSTSlot = day, clock, detail.Broadcast
					} else {
						slot.LocalDay = "unscheduled"
						slot.JSTSlot = detail.Broadcast
					}
					slots = append(slots, slot)
				}
			}
			// Collisions: two tracked shows in the same local day and hour. A
			// slot without a local time has no schedule to collide with, so it is
			// excluded; otherwise every unscheduled entry would share the same
			// empty key and be falsely flagged against the others.
			counts := map[string]int{}
			for _, s := range slots {
				if s.LocalTime == "" {
					continue
				}
				counts[s.LocalDay+" "+s.LocalTime]++
			}
			for i := range slots {
				if counts[slots[i].LocalDay+" "+slots[i].LocalTime] > 1 {
					slots[i].Collision = true
				}
			}
			sort.SliceStable(slots, func(i, j int) bool {
				if slots[i].LocalDay != slots[j].LocalDay {
					return dayOrder(slots[i].LocalDay) < dayOrder(slots[j].LocalDay)
				}
				return slots[i].LocalTime < slots[j].LocalTime
			})
			view := weekView{Slots: slots, Timezone: localZone(), Entries: len(entries)}
			switch {
			case localOnly && len(slots) > 0:
				view.Note = "--data-source local: broadcast times are only published on the live title page, so no local schedule is available; drop the flag for the live grid"
			case len(slots) == 0:
				view.Note = "no locally tracked shows matched; add one with `myanimelist-pp-cli track add <id> --status watching`"
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(slots) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			if localOnly {
				fmt.Fprintf(cmd.ErrOrStderr(), "note: %s\n", view.Note)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Weekly grid (%s)\n", view.Timezone)
			table := make([]map[string]any, 0, len(slots))
			for _, s := range slots {
				mark := ""
				if s.Collision {
					mark = "collision"
				}
				table = append(table, map[string]any{"day": s.LocalDay, "time": s.LocalTime, "show": s.Title, "next_ep": s.Next, "flag": mark})
			}
			return printAutoTable(cmd.OutOrStdout(), table)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	cmd.Flags().StringVar(&kind, "kind", "", "Only include library entries of this kind (anime or manga)")
	cmd.Flags().BoolVar(&all, "all", false, "Include every library status, not just watching")
	return cmd
}

func localZone() string {
	z, _ := time.Now().Zone()
	if name := time.Now().Location().String(); name != "" && name != "Local" {
		return name
	}
	return z
}

func dayOrder(day string) int {
	switch strings.ToLower(day) {
	case "monday":
		return 1
	case "tuesday":
		return 2
	case "wednesday":
		return 3
	case "thursday":
		return 4
	case "friday":
		return 5
	case "saturday":
		return 6
	case "sunday":
		return 7
	}
	return 8
}

// convertBroadcast turns a MyAnimeList JST broadcast slot ("Saturdays at 01:00
// (JST)") into the viewer's local weekday and clock time.
func convertBroadcast(broadcast string, now time.Time) (string, string, bool) {
	m := broadcastRE.FindStringSubmatch(broadcast)
	if m == nil {
		return "", "", false
	}
	hour, _ := strconv.Atoi(m[2])
	minute, _ := strconv.Atoi(m[3])
	weekday := strings.Title(strings.ToLower(m[1]))
	jst := time.FixedZone("JST", 9*3600)
	// Anchor on the next matching JST weekday at the broadcast clock, then
	// render that instant in the local zone so DST is handled by the runtime.
	offset := (dayOrder(weekday) - dayOrder(now.In(jst).Weekday().String()) + 7) % 7
	anchor := now.In(jst).AddDate(0, 0, offset)
	slot := time.Date(anchor.Year(), anchor.Month(), anchor.Day(), hour, minute, 0, 0, jst)
	local := slot.In(time.Local)
	return local.Weekday().String(), local.Format("15:04"), true
}
