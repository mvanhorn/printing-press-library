// Copyright 2026 Charles Garrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command `drift` — diff the two most recent stored SERP
// snapshots for one query and report per-domain rank movement (moved, new,
// dropped) entirely offline, no API call, no credits. Reads only the local
// store seeded by `google search`. Hand file (no generator header).

package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-do/internal/store"
	"github.com/spf13/cobra"
)

// driftMover is one domain's position change between two snapshots.
type driftMover struct {
	Domain      string `json:"domain"`
	CurrentPos  *int   `json:"current_pos,omitempty"`
	PreviousPos *int   `json:"previous_pos,omitempty"`
	Delta       *int   `json:"delta,omitempty"` // previous - current; positive = moved up
	Status      string `json:"status"`          // moved | new | dropped | unchanged
}

func newNovelDriftCmd(flags *rootFlags) *cobra.Command {
	var gl, hl, googleDomain, device, dbPath string
	cmd := &cobra.Command{
		Use:   "drift <query>",
		Short: "Diff a Google query's two most recent stored SERPs — offline, no credits",
		Long: `Compare the two most recent locally-stored SERP snapshots for a query (and
its locale tuple) and report per-domain rank movement: who moved up or down,
who is newly ranking, and who dropped off. Reads only the local store — no API
call, no credits. Seed it by running 'google search "<query>"' at least twice.`,
		Example: strings.Trim(`
  scrape-do-pp-cli drift "best crm software"
  scrape-do-pp-cli drift "best crm software" --gl us --hl en --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "query=best crm software",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if dryRunOK(flags) {
				return nil
			}
			st, ext, err := openExtras(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			paramHash := serpParamHash(query, gl, hl, googleDomain, device)
			cur, prev, err := ext.TwoLatestSnapshots(cmd.Context(), paramHash)
			if err != nil {
				return err
			}
			if cur == nil || prev == nil {
				note := fmt.Sprintf("need at least two snapshots for %q; run 'scrape-do-pp-cli google search %q' twice (over time) to seed them", query, query)
				payload := map[string]any{"query": query, "param_hash": paramHash, "movers": []driftMover{}, "note": note}
				return emitGov(cmd, flags, payload, note)
			}
			curOrg, err := ext.OrganicForSnapshot(cmd.Context(), cur.ID)
			if err != nil {
				return err
			}
			prevOrg, err := ext.OrganicForSnapshot(cmd.Context(), prev.ID)
			if err != nil {
				return err
			}
			movers := diffOrganic(curOrg, prevOrg)
			onlyChanged := make([]driftMover, 0, len(movers))
			for _, m := range movers {
				if m.Status != "unchanged" {
					onlyChanged = append(onlyChanged, m)
				}
			}
			payload := map[string]any{
				"query":               query,
				"param_hash":          paramHash,
				"current_fetched_at":  cur.FetchedAt,
				"previous_fetched_at": prev.FetchedAt,
				"movers":              onlyChanged,
			}
			if flags.asJSON {
				return flags.printJSON(cmd, payload)
			}
			if len(onlyChanged) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no rank changes for %q between the two latest snapshots\n", query)
				return nil
			}
			rows := make([][]string, 0, len(onlyChanged))
			for _, m := range onlyChanged {
				rows = append(rows, []string{m.Domain, posStr(m.PreviousPos), posStr(m.CurrentPos), deltaStr(m.Delta), m.Status})
			}
			return flags.printTable(cmd, []string{"DOMAIN", "WAS", "NOW", "DELTA", "STATUS"}, rows)
		},
	}
	cmd.Flags().StringVar(&gl, "gl", "", "Country code that keys the snapshot (must match the search)")
	cmd.Flags().StringVar(&hl, "hl", "", "Language code that keys the snapshot (must match the search)")
	cmd.Flags().StringVar(&googleDomain, "google-domain", "", "Google domain that keys the snapshot (must match the search)")
	cmd.Flags().StringVar(&device, "device", "", "Device that keys the snapshot (must match the search)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// diffOrganic compares two flattened organic result sets by domain (best
// position per domain wins) and returns each domain's movement.
func diffOrganic(cur, prev []store.OrganicRow) []driftMover {
	bestPos := func(rows []store.OrganicRow) map[string]int {
		m := map[string]int{}
		for _, r := range rows {
			if r.Domain == "" {
				continue
			}
			if p, ok := m[r.Domain]; !ok || r.Position < p {
				m[r.Domain] = r.Position
			}
		}
		return m
	}
	curMap := bestPos(cur)
	prevMap := bestPos(prev)
	seen := map[string]bool{}
	out := make([]driftMover, 0, len(curMap)+len(prevMap))
	for dom, cp := range curMap {
		seen[dom] = true
		cpv := cp
		m := driftMover{Domain: dom, CurrentPos: &cpv}
		if pp, ok := prevMap[dom]; ok {
			ppv := pp
			d := pp - cp
			m.PreviousPos = &ppv
			m.Delta = &d
			if d == 0 {
				m.Status = "unchanged"
			} else {
				m.Status = "moved"
			}
		} else {
			m.Status = "new"
		}
		out = append(out, m)
	}
	for dom, pp := range prevMap {
		if seen[dom] {
			continue
		}
		ppv := pp
		out = append(out, driftMover{Domain: dom, PreviousPos: &ppv, Status: "dropped"})
	}
	sort.Slice(out, func(i, j int) bool {
		ai := absDelta(out[i])
		aj := absDelta(out[j])
		if ai != aj {
			return ai > aj
		}
		return out[i].Domain < out[j].Domain
	})
	return out
}

// absDelta scores a mover for sorting: bigger movements (and new/dropped) first.
func absDelta(m driftMover) int {
	switch m.Status {
	case "new", "dropped":
		return 1000
	case "moved":
		if m.Delta != nil {
			if *m.Delta < 0 {
				return -*m.Delta
			}
			return *m.Delta
		}
	}
	return 0
}

func posStr(p *int) string {
	if p == nil {
		return "-"
	}
	return strconv.Itoa(*p)
}

func deltaStr(d *int) string {
	if d == nil {
		return "-"
	}
	if *d > 0 {
		return "+" + strconv.Itoa(*d)
	}
	return strconv.Itoa(*d)
}
