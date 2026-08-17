// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/internal/icp"
	"github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/internal/store"

	"github.com/spf13/cobra"
)

// syncResult is the machine-readable outcome of one sync.
type syncResult struct {
	Account          string   `json:"account"`
	Locations        int      `json:"locations"`
	Classes          int      `json:"classes"`
	Camps            int      `json:"camps"`
	Pages            int      `json:"pages_fetched"`
	RunID            int64    `json:"run_id"`
	DBPath           string   `json:"db_path"`
	Gate             string   `json:"gate"`
	Truncated        bool     `json:"truncated"`
	SnapshotRecorded bool     `json:"snapshot_recorded"`
	Note             string   `json:"note,omitempty"`
	Warnings         []string `json:"warnings"`
}

// icpRecordSyncObservations promotes complete whole-catalog walks to snapshots.
// Page-capped, resource-scoped, and access-gated walks still add useful
// openings history, but they must never become the newest catalog snapshot:
// drift would interpret every unseen row as removed.
func icpRecordSyncObservations(ctx context.Context, s *store.Store, account string, at time.Time, obs []store.ICPObservation, snapshotComplete bool) (int64, error) {
	if !snapshotComplete {
		return 0, s.RecordICPOpenings(ctx, at, obs)
	}
	runID, err := s.StartICPRun(ctx, account, at)
	if err != nil {
		return 0, err
	}
	if err := s.RecordICPObservations(ctx, runID, at, obs); err != nil {
		return 0, err
	}
	return runID, nil
}

func icpSyncSnapshotComplete(wantClasses, wantCamps, truncated, gated bool) bool {
	return wantClasses && wantCamps && !truncated && !gated
}

func newIclassproSyncCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath    string
		resources string
		maxPages  int
		pageSize  int
		keepRuns  int
	)

	cmd := &cobra.Command{
		Use:   "sync [account]",
		Short: "Mirror an account's catalog into the local database",
		Long: strings.Trim(`
Walk every active location of an iClassPro account and record its classes and
camps in the local SQLite mirror.

Each run also appends an openings observation for every entity. That history is
what makes 'drift', 'fill-rate', and 'watch' possible: the portal API is
present-tense only, so nothing upstream can tell you what a number used to be.
Run sync at least twice, spaced apart, before expecting those commands to have
anything to compare.

Only a complete classes-and-camps walk replaces the authoritative snapshot.
Runs limited by --resources or --max-pages, or interrupted by a customer
sign-in gate, still update openings history and the search cache without making
omitted records look deleted.`, "\n"),
		Example: strings.Trim(`
  iclasspro-pp-cli sync scottsdalegymnastics
  iclasspro-pp-cli sync scottsdalegymnastics --resources classes,camps
  iclasspro-pp-cli sync scaq --resources classes --max-pages 3`, "\n"),
		Annotations: map[string]string{"pp:happy-args": "<account>=scaq;--resources=classes,camps;--max-pages=1"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				if !cliutil.IsDogfoodEnv() {
					return cmd.Help()
				}
				// Printing Press primes local-history commands by invoking a bare
				// sync in an isolated HOME before the live dogfood matrix. Keep the
				// normal interactive behavior (help) while giving that harness one
				// bounded, public catalog to mirror.
				args = []string{"scaq"}
				resources = "classes,camps"
				maxPages = 1
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "sync")
			}
			account, err := icpRequireAccount(args)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			wantClasses, wantCamps := true, true
			if strings.TrimSpace(resources) != "" {
				wantClasses, wantCamps = false, false
				for _, r := range strings.Split(resources, ",") {
					switch strings.ToLower(strings.TrimSpace(r)) {
					case "classes", "class":
						wantClasses = true
					case "camps", "camp", "events":
						wantCamps = true
					case "":
					default:
						return usageErr(fmt.Errorf("unknown resource %q; supported: classes, camps", strings.TrimSpace(r)))
					}
				}
				if !wantClasses && !wantCamps {
					return usageErr(fmt.Errorf("--resources selected nothing; supported: classes, camps"))
				}
			}

			// Live dogfood runs against the real portal under a flat per-command
			// timeout, so curtail the walk rather than paging a large gym fully.
			if cliutil.IsDogfoodEnv() && maxPages > 1 {
				maxPages = 1
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			coll, err := icpCollect(ctx, c, account, icpCollectOptions{
				IncludeClasses: wantClasses,
				IncludeCamps:   wantCamps,
				MaxPages:       maxPages,
				PageSize:       pageSize,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			res := syncResult{
				Account:   account,
				Locations: len(coll.Locations),
				Pages:     coll.Pages,
				DBPath:    icpDBPath(dbPath),
				Gate:      string(coll.Gate),
				Truncated: coll.Truncated,
				Note:      icpGateNote(account, coll.Gate),
				Warnings:  coll.Warnings,
			}
			snapshotComplete := icpSyncSnapshotComplete(wantClasses, wantCamps, coll.Truncated, coll.Gated)
			if !wantClasses || !wantCamps {
				res.Note = "catalog snapshot unchanged because --resources did not include both classes and camps"
			}
			for _, e := range coll.Entities {
				if e.Kind == icp.KindCamp {
					res.Camps++
				} else {
					res.Classes++
				}
			}

			if len(coll.Entities) > 0 {
				if err := icpEnsureParentDir(res.DBPath); err != nil {
					return err
				}
				s, err := store.OpenWithContext(ctx, res.DBPath)
				if err != nil {
					return fmt.Errorf("opening local mirror at %s: %w", res.DBPath, err)
				}
				defer func() { _ = s.Close() }()

				now := time.Now().UTC()
				obs := make([]store.ICPObservation, 0, len(coll.Entities))
				for _, e := range coll.Entities {
					data, merr := json.Marshal(e)
					if merr != nil {
						return fmt.Errorf("encoding %s: %w", e.Key(), merr)
					}
					obs = append(obs, store.ICPObservation{
						Account: e.Account, Kind: e.Kind, EntityID: e.ID, Name: e.Name,
						Openings: e.Openings, FutureOpen: e.FutureOpenings,
						HasOpenings: e.HasOpenings, AllowWaitlist: e.AllowWaitlist,
						Data: data,
					})
				}
				runID, err := icpRecordSyncObservations(ctx, s, account, now, obs, snapshotComplete)
				if err != nil {
					return err
				}
				res.RunID = runID
				res.SnapshotRecorded = runID > 0

				// The generic `resources` table backs the shared FTS index. It is
				// written after the observation transaction commits, never inside
				// it: Upsert opens its own write transaction and SQLite allows a
				// single writer.
				for i, e := range coll.Entities {
					resourceType := fmt.Sprintf("%s/%ss", e.Account, e.Kind)
					if err := s.Upsert(resourceType, fmt.Sprint(e.ID), obs[i].Data); err != nil {
						return fmt.Errorf("caching %s: %w", e.Key(), err)
					}
				}
				if res.SnapshotRecorded {
					if err := s.PruneICPRuns(ctx, account, keepRuns); err != nil {
						return err
					}
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			if res.Note != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "note:", res.Note)
			}
			for _, w := range res.Warnings {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning:", w)
			}
			if len(coll.Entities) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Synced %s: nothing to record.\n", account)
				return nil
			}
			if !snapshotComplete {
				reason := "the scan hit --max-pages"
				if !wantClasses || !wantCamps {
					reason = "--resources did not include both classes and camps"
				}
				fmt.Fprintf(cmd.OutOrStdout(),
					"Observed %s: %d locations, %d classes, %d camps across %d requests; catalog snapshot unchanged because %s.\n",
					account, res.Locations, res.Classes, res.Camps, res.Pages, reason)
				fmt.Fprintf(cmd.OutOrStdout(), "Local mirror: %s\n", res.DBPath)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"Synced %s: %d locations, %d classes, %d camps across %d requests (run %d)\n",
				account, res.Locations, res.Classes, res.Camps, res.Pages, res.RunID)
			fmt.Fprintf(cmd.OutOrStdout(), "Local mirror: %s\n", res.DBPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	cmd.Flags().StringVar(&resources, "resources", "", "Comma-separated resources to observe: classes, camps; only both together record a snapshot (default both)")
	cmd.Flags().IntVar(&maxPages, "max-pages", 20, "Maximum result pages to fetch per location and camp type")
	cmd.Flags().IntVar(&pageSize, "page-size", 100, "Results requested per page")
	cmd.Flags().IntVar(&keepRuns, "keep-runs", 20, "Number of historical snapshots to retain per account")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newIclassproSyncCmd(flags))
	})
}
