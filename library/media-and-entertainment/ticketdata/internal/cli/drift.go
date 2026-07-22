// pp:data-source local
// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/ticketdata/internal/store"
	"github.com/spf13/cobra"
)

type driftMove struct {
	EventID       string  `json:"event_id"`
	Title         string  `json:"title"`
	PreviousPrice float64 `json:"previous_price"`
	CurrentPrice  float64 `json:"current_price"`
	Delta         float64 `json:"delta"`
	ChangePct     float64 `json:"change_pct"`
	Direction     string  `json:"direction"`
}

type driftTargetHit struct {
	EventID      string  `json:"event_id"`
	Title        string  `json:"title"`
	CurrentPrice float64 `json:"current_price"`
	Target       float64 `json:"target"`
	Hit          bool    `json:"hit"`
}

type driftView struct {
	Moved      []driftMove      `json:"moved"`
	TargetHits []driftTargetHit `json:"target_hits"`
}

func newNovelDriftCmd(flags *rootFlags) *cobra.Command {
	var threshold float64
	var targets []string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "drift",
		Short:       "Diff the two most recent snapshots per watched event, flag floors that moved past a threshold",
		Long:        "Use `drift` for what moved since the last sync and for price-target alerts. For a full current snapshot use `board`; for one event's history use `stats`.",
		Example:     "  ticketdata-pp-cli drift --threshold 10 --target 22323960=150 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return usageErr(fmt.Errorf("drift does not accept arguments"))
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare the two latest local snapshots")
				return nil
			}
			if threshold < 0 {
				return usageErr(fmt.Errorf("--threshold must be non-negative"))
			}
			targetMap, err := parseDriftTargets(targets)
			if err != nil {
				return usageErr(err)
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			watches, err := db.ListWatch(cmd.Context())
			if err != nil {
				return err
			}

			// Batch every snapshot read (moves need 2 latest per watched event,
			// targets need 1 latest) into a single query over the union of IDs.
			idSet := make(map[string]struct{}, len(watches)+len(targetMap))
			eventIDs := make([]string, 0, len(watches)+len(targetMap))
			addID := func(id string) {
				if _, ok := idSet[id]; !ok {
					idSet[id] = struct{}{}
					eventIDs = append(eventIDs, id)
				}
			}
			for _, watch := range watches {
				addID(watch.EventID)
			}
			for id := range targetMap {
				addID(id)
			}
			snapsByEvent, err := db.LatestSnapshotsForEvents(cmd.Context(), eventIDs, 2)
			if err != nil {
				return err
			}

			titles := make(map[string]string, len(watches))
			moved := make([]driftMove, 0)
			for _, watch := range watches {
				titles[watch.EventID] = watch.Title
				snaps := snapsByEvent[watch.EventID]
				if len(snaps) < 2 {
					continue
				}
				cur, prev := snaps[0], snaps[1]
				delta := cur.GetInPrice - prev.GetInPrice
				pct := 0.0
				if prev.GetInPrice > 0 {
					pct = delta / prev.GetInPrice * 100
				}
				if math.Abs(pct) >= threshold {
					direction := "flat"
					if delta > 0 {
						direction = "up"
					} else if delta < 0 {
						direction = "down"
					}
					moved = append(moved, driftMove{EventID: watch.EventID, Title: watch.Title, PreviousPrice: prev.GetInPrice, CurrentPrice: cur.GetInPrice, Delta: delta, ChangePct: pct, Direction: direction})
				}
			}
			hits := make([]driftTargetHit, 0)
			for id, target := range targetMap {
				snaps := snapsByEvent[id]
				if len(snaps) == 0 || snaps[0].GetInPrice <= 0 || snaps[0].GetInPrice > target {
					continue
				}
				hits = append(hits, driftTargetHit{EventID: id, Title: titles[id], CurrentPrice: snaps[0].GetInPrice, Target: target, Hit: true})
			}
			sort.Slice(hits, func(i, j int) bool { return hits[i].EventID < hits[j].EventID })
			if len(moved) == 0 && len(hits) == 0 {
				hintIfUnsynced(cmd, db, "")
			}
			view := driftView{Moved: moved, TargetHits: hits}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return printDriftTable(cmd, view)
		},
	}
	cmd.Flags().Float64Var(&threshold, "threshold", 5, "Minimum percent movement to report")
	cmd.Flags().StringArrayVar(&targets, "target", nil, "Price target as eventID=price; repeatable")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath("ticketdata-pp-cli"), "SQLite database file path")
	return cmd
}

func parseDriftTargets(raw []string) (map[string]float64, error) {
	targets := make(map[string]float64, len(raw))
	for _, entry := range raw {
		id, priceText, ok := strings.Cut(entry, "=")
		if !ok || strings.TrimSpace(id) == "" || strings.TrimSpace(priceText) == "" {
			return nil, fmt.Errorf("--target must look like eventID=price")
		}
		price, err := strconv.ParseFloat(strings.TrimSpace(priceText), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid target price %q", priceText)
		}
		targets[strings.TrimSpace(id)] = price
	}
	return targets, nil
}

func printDriftTable(cmd *cobra.Command, view driftView) error {
	if len(view.Moved) == 0 && len(view.TargetHits) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no drift or target hits found")
		return nil
	}
	tw := newTabWriter(cmd.OutOrStdout())
	fmt.Fprintln(tw, "TYPE\tEVENT\tTITLE\tPREVIOUS\tCURRENT\tDELTA\tCHANGE\tTARGET")
	for _, row := range view.Moved {
		fmt.Fprintf(tw, "move\t%s\t%s\t%.2f\t%.2f\t%.2f\t%.2f%%\t\n", row.EventID, truncate(row.Title, 38), row.PreviousPrice, row.CurrentPrice, row.Delta, row.ChangePct)
	}
	for _, row := range view.TargetHits {
		fmt.Fprintf(tw, "target\t%s\t%s\t\t%.2f\t\t\t%.2f\n", row.EventID, truncate(row.Title, 38), row.CurrentPrice, row.Target)
	}
	return tw.Flush()
}
