// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: see .printing-press-patches/ for context. Hand-authored, not
// generator output — regen-merge preserves this file.

// pp:data-source computed

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// driftTrackedResources lists the config-shaping resources drift compares.
// Each entry names the local table (or, when empty, a generic-resources
// resource_type — see genericResourceRows) this resource is actually synced
// into on this API.
var driftTrackedResources = []struct {
	label        string
	table        string
	resourceType string
}{
	{label: "networks", table: "networks"},
	{label: "wifi", table: "wifi"},
	{label: "dns", table: "dns"},
	{label: "firewall_zones", resourceType: "v1_sites_firewall_zones"},
	{label: "firewall_policies", resourceType: "v1_sites_firewall_policies"},
}

type driftChange struct {
	Resource string `json:"resource"`
	ID       string `json:"id"`
	Kind     string `json:"kind"` // added, removed, changed
}

type driftView struct {
	Site           string        `json:"site"`
	BaselineAt     *time.Time    `json:"baseline_at,omitempty"`
	SinceRequested string        `json:"since_requested,omitempty"`
	FirstRun       bool          `json:"first_run"`
	Changes        []driftChange `json:"changes"`
	Note           string        `json:"note,omitempty"`
}

func newNovelDriftCmd(flags *rootFlags) *cobra.Command {
	var flagSite string
	var flagSince string

	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Show what changed in site config (networks, firewall, wifi, DNS) since the last sync snapshot.",
		Long: "Compares the current locally synced state against a snapshot " +
			"captured the last time 'drift' itself ran for this site, then " +
			"advances the snapshot forward. The API has no config-versioning or " +
			"audit-trail endpoint, so this command maintains its own snapshot on " +
			"disk rather than querying history from the API. The first run for a " +
			"site has nothing to compare against and reports first_run=true with " +
			"an empty change list (that captured state becomes the baseline for " +
			"the next run) — that is expected, not an error. Run 'unifi-pp-cli " +
			"sync' before each 'drift' to pick up the latest config.",
		Example:     "  unifi-pp-cli drift --site default --since 24h --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "drift")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath := defaultDBPath("unifi-pp-cli")
			db, err := openNovelStore(ctx, dbPath)
			if err != nil {
				return err
			}
			if db == nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: unifi-pp-cli sync\n", dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), driftView{Changes: []driftChange{}}, flags)
				}
				return nil
			}
			defer db.Close()

			siteID, siteName, err := resolveSiteIDLocal(ctx, db.DB(), flagSite)
			if err != nil {
				if isNoLocalDataYet(err) {
					fmt.Fprintf(cmd.ErrOrStderr(), "%s\nrun: unifi-pp-cli sync\n", err)
					if !wantsHumanTable(cmd.OutOrStdout(), flags) {
						return printJSONFiltered(cmd.OutOrStdout(), driftView{Changes: []driftChange{}}, flags)
					}
					return nil
				}
				return err
			}

			snapDir, err := novelSnapshotDir()
			if err != nil {
				return err
			}

			view := driftView{Site: siteName, SinceRequested: flagSince, Changes: []driftChange{}}
			firstRunAny := false

			for _, res := range driftTrackedResources {
				var curr map[string]json.RawMessage
				if res.table != "" {
					curr, err = resourceRows(ctx, db.DB(), res.table, siteID)
				} else {
					curr, err = genericResourceRows(ctx, db.DB(), res.resourceType, siteID)
				}
				if err != nil {
					return err
				}

				path := snapshotPath(snapDir, res.label, siteID)
				prevSnap, loadErr := loadResourceSnapshot(path)
				if loadErr != nil {
					return fmt.Errorf("loading %s snapshot: %w", res.label, loadErr)
				}
				if prevSnap == nil {
					firstRunAny = true
				} else {
					if view.BaselineAt == nil || prevSnap.CapturedAt.Before(*view.BaselineAt) {
						t := prevSnap.CapturedAt
						view.BaselineAt = &t
					}
					prev := prevSnap.Entities
					for id := range curr {
						if _, ok := prev[id]; !ok {
							view.Changes = append(view.Changes, driftChange{Resource: res.label, ID: id, Kind: "added"})
						} else if string(prev[id]) != string(curr[id]) {
							view.Changes = append(view.Changes, driftChange{Resource: res.label, ID: id, Kind: "changed"})
						}
					}
					for id := range prev {
						if _, ok := curr[id]; !ok {
							view.Changes = append(view.Changes, driftChange{Resource: res.label, ID: id, Kind: "removed"})
						}
					}
				}

				if saveErr := saveResourceSnapshot(path, resourceSnapshot{CapturedAt: time.Now().UTC(), Entities: curr}); saveErr != nil {
					return fmt.Errorf("saving %s snapshot: %w", res.label, saveErr)
				}
			}

			view.FirstRun = firstRunAny
			if firstRunAny && view.BaselineAt == nil {
				view.Note = "no prior snapshot for one or more resources; baseline captured now, re-run drift later to see changes"
			} else if len(view.Changes) == 0 {
				view.Note = "no changes detected since the last drift run"
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			if len(view.Changes) == 0 {
				fmt.Fprintln(w, view.Note)
				return nil
			}
			for _, c := range view.Changes {
				fmt.Fprintf(w, "%-8s %-18s %s\n", c.Kind, c.Resource, c.ID)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSite, "site", "", "Site id, internalReference, or name (default: the only synced site)")
	cmd.Flags().StringVar(&flagSince, "since", "", "Informational only: echoed in output. Actual comparison is always against the last drift run's snapshot, not a fixed lookback window.")
	return cmd
}
