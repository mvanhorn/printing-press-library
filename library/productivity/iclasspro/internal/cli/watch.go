// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call

package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/internal/icp"
	"github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/internal/store"

	"github.com/spf13/cobra"
)

type watchObservation struct {
	Key       string `json:"key"`
	Kind      string `json:"entity_kind"`
	EntityID  int    `json:"entity_id"`
	Name      string `json:"name"`
	Openings  int    `json:"openings"`
	Previous  *int   `json:"previous_openings,omitempty"`
	Status    string `json:"status"`
	Opened    bool   `json:"opened"`
	Changed   bool   `json:"changed"`
	PortalURL string `json:"portal_url,omitempty"`
}

type watchReport struct {
	Account   string             `json:"account"`
	Checks    int                `json:"checks"`
	Watched   int                `json:"entities_watched"`
	Openings  []watchObservation `json:"openings"`
	Changes   []watchObservation `json:"changes"`
	Recorded  bool               `json:"recorded"`
	Note      string             `json:"note,omitempty"`
	Gate      string             `json:"gate,omitempty"`
	CheckedAt string             `json:"checked_at"`
}

func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	var (
		classID  string
		campID   string
		interval string
		follow   bool
		checks   int
		dbPath   string
		quietWin bool
	)

	cmd := &cobra.Command{
		Use:   "watch [account]",
		Short: "Get told the moment a spot frees up in a class or camp that is currently full.",
		Long: strings.Trim(`
Poll an account's live catalog and report availability transitions.

Use this command to be alerted when a spot frees up in a class or camp that is
currently full. Do NOT use it to find registration that has not opened yet; use
'opens-soon' instead.

Every check is compared against the newest observation already in the local
mirror and is itself recorded, so the transition survives across invocations.
A single check is the default; --follow polls until interrupted or until
--checks is reached.`, "\n"),
		Example: strings.Trim(`
  iclasspro-pp-cli watch scottsdalegymnastics --agent
  iclasspro-pp-cli watch scottsdalegymnastics --class 7653
  iclasspro-pp-cli watch scaq --follow --interval 5m --checks 12`, "\n"),
		// Deliberately NOT mcp:read-only: every check appends an openings observation
		// to the local store (RecordICPOpenings below), which is what makes drift and
		// fill-rate possible. Same classification as `sync`.
		Annotations: map[string]string{"pp:happy-args": "<account>=scaq"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "watch")
			}
			account, err := icpRequireAccount(args)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}

			var wantClass, wantCamp int
			if strings.TrimSpace(classID) != "" {
				wantClass, err = strconv.Atoi(strings.TrimSpace(classID))
				if err != nil {
					return usageErr(fmt.Errorf("--class %q is not a numeric class id", classID))
				}
			}
			if strings.TrimSpace(campID) != "" {
				wantCamp, err = strconv.Atoi(strings.TrimSpace(campID))
				if err != nil {
					return usageErr(fmt.Errorf("--camp %q is not a numeric camp id", campID))
				}
			}

			every := time.Minute * 5
			if strings.TrimSpace(interval) != "" {
				every, err = cliutil.ParseDurationLoose(interval)
				if err != nil {
					return usageErr(fmt.Errorf("--interval %q is not a duration (try 5m, 30s, 1h): %w", interval, err))
				}
				if every <= 0 {
					return usageErr(fmt.Errorf("--interval must be positive"))
				}
			}
			if !follow {
				checks = 1
			}
			if checks <= 0 {
				checks = 1
			}
			// Verify and live-dogfood runs are bounded; never loop under either.
			if cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() {
				checks = 1
				follow = false
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := icpDBPath(dbPath)
			var report watchReport
			for i := 0; i < checks; i++ {
				if i > 0 {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(every):
					}
				}
				report, err = icpWatchOnce(ctx, c, account, path, wantClass, wantCamp)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				report.Checks = i + 1

				if wantsHumanTable(cmd.OutOrStdout(), flags) {
					icpRenderWatch(cmd, report, quietWin)
				} else if i == checks-1 {
					return printJSONFiltered(cmd.OutOrStdout(), report, flags)
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&classID, "class", "", "Watch only this class id")
	cmd.Flags().StringVar(&campID, "camp", "", "Watch only this camp id")
	cmd.Flags().StringVar(&interval, "interval", "5m", "Delay between checks when --follow is set (e.g. 5m, 30s)")
	cmd.Flags().BoolVar(&follow, "follow", false, "Keep polling instead of checking once")
	cmd.Flags().IntVar(&checks, "checks", 1, "Maximum checks to perform when --follow is set")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	cmd.Flags().BoolVar(&quietWin, "only-changes", false, "Print only transitions, not the full availability list")
	return cmd
}

// icpWatchOnce performs one live check, compares it to the newest recorded
// observation, and records the result.
func icpWatchOnce(ctx context.Context, c *client.Client, account, dbPath string, classID, campID int) (watchReport, error) {
	report := watchReport{
		Account:   account,
		Openings:  make([]watchObservation, 0),
		Changes:   make([]watchObservation, 0),
		CheckedAt: icpNow().Format(time.RFC3339),
	}

	coll, err := icpCollect(ctx, c, account, icpCollectOptions{
		IncludeClasses: campID == 0,
		IncludeCamps:   classID == 0,
		MaxPages:       10,
		PageSize:       100,
	})
	if err != nil {
		return report, err
	}
	report.Gate = string(coll.Gate)
	if coll.Gate == icpGateSignIn || coll.Gate == icpGateNotFound {
		report.Note = icpGateNote(account, coll.Gate)
	}

	ents := make([]icp.Entity, 0, len(coll.Entities))
	for _, e := range coll.Entities {
		if classID > 0 && !(e.Kind == icp.KindClass && e.ID == classID) {
			continue
		}
		if campID > 0 && !(e.Kind == icp.KindCamp && e.ID == campID) {
			continue
		}
		ents = append(ents, e)
	}
	report.Watched = len(ents)
	if (classID > 0 || campID > 0) && len(ents) == 0 {
		report.Note = "the requested id was not found in this account's live catalog"
		return report, nil
	}

	// Previous observations come from the mirror when it exists; a first run
	// simply has no baseline and reports current state without transitions.
	prev := map[string]int{}
	if _, statErr := icpStat(dbPath); statErr == nil {
		s, oerr := store.OpenWithContext(ctx, dbPath)
		if oerr == nil {
			defer func() { _ = s.Close() }()
			hist, herr := s.ICPHistory(ctx, account, icpNow().AddDate(0, 0, -365))
			if herr == nil {
				for _, h := range hist {
					prev[fmt.Sprintf("%s/%s/%d", h.Account, h.Kind, h.EntityID)] = h.Openings
				}
			}
		}
	}

	obs := make([]store.ICPObservation, 0, len(ents))
	for _, e := range ents {
		o := watchObservation{
			Key: e.Key(), Kind: e.Kind, EntityID: e.ID, Name: e.Name,
			Openings: e.Openings, Status: e.Status(), PortalURL: e.PortalURL,
		}
		if p, seen := prev[e.Key()]; seen {
			pc := p
			o.Previous = &pc
			o.Changed = p != e.Openings
			o.Opened = p == 0 && e.Openings > 0
		}
		if e.Openings > 0 {
			report.Openings = append(report.Openings, o)
		}
		if o.Changed {
			report.Changes = append(report.Changes, o)
		}
		data, merr := icpMarshal(e)
		if merr != nil {
			return report, merr
		}
		obs = append(obs, store.ICPObservation{
			Account: e.Account, Kind: e.Kind, EntityID: e.ID, Name: e.Name,
			Openings: e.Openings, FutureOpen: e.FutureOpenings,
			HasOpenings: e.HasOpenings, AllowWaitlist: e.AllowWaitlist, Data: data,
		})
	}

	if len(obs) > 0 {
		if err := icpEnsureParentDir(dbPath); err == nil {
			if s, oerr := store.OpenWithContext(ctx, dbPath); oerr == nil {
				defer func() { _ = s.Close() }()
				// Openings history only. Watch can be scoped to one class, so it
				// must never write a catalog snapshot; see RecordICPOpenings.
				if rerr := s.RecordICPOpenings(ctx, icpNow(), obs); rerr == nil {
					report.Recorded = true
				}
			}
		}
	}
	return report, nil
}

func icpRenderWatch(cmd *cobra.Command, r watchReport, onlyChanges bool) {
	if r.Note != "" {
		fmt.Fprintln(cmd.ErrOrStderr(), "note:", r.Note)
	}
	stamp := time.Now().Format("15:04:05")
	if len(r.Changes) > 0 {
		for _, o := range r.Changes {
			verb := "changed"
			if o.Opened {
				verb = "OPENED"
			}
			from := "?"
			if o.Previous != nil {
				from = strconv.Itoa(*o.Previous)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s %s: %s -> %d  %s\n",
				stamp, verb, truncate(o.Name, 44), from, o.Openings, o.PortalURL)
		}
		return
	}
	if onlyChanges {
		return
	}
	fmt.Fprintf(cmd.OutOrStdout(), "[%s] %d of %d watched entities have openings; no change since the last observation.\n",
		stamp, len(r.Openings), r.Watched)
}
