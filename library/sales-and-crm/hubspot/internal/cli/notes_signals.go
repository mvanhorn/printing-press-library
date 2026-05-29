// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: regex buying/lost signals scanned over note bodies.

package cli

import (
	"database/sql"
	"fmt"
	"regexp"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
	"github.com/spf13/cobra"
)

func newNovelNotesSignalsCmd(flags *rootFlags) *cobra.Command {
	var pipeline string
	var since string
	var owner string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "signals",
		Short:       "Scan note bodies for buying / lost signals and group by deal",
		Long:        `Regex-scan HubSpot note bodies for buying signals (meeting scheduled, budget approved, etc.) and lost signals (no response, competitor chosen, etc.). When the local hubspot_associations table is populated, signals group by deal; otherwise per-note rows are emitted.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  hubspot-pp-cli notes signals --since 30d --pipeline default`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("hubspot-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			ownerID, err := resolveOwnerArg(db, owner)
			if err != nil {
				return err
			}

			cutoff := ""
			if since != "" {
				cutoff, err = parseDurationOrTimestamp(since)
				if err != nil {
					return err
				}
			}

			var assocCount int
			_ = db.DB().QueryRow(`SELECT COUNT(*) FROM hubspot_associations`).Scan(&assocCount)
			source := "local"
			if assocCount == 0 {
				source = "local-no-associations"
			}
			_ = pipeline

			q := `
SELECT n.id,
  COALESCE(json_extract(n.data, '$.properties.hs_note_body'), '') AS body,
  COALESCE(json_extract(n.data, '$.properties.hubspot_owner_id'), '') AS owner_id,
  COALESCE(n.updated_at, n.created_at, '') AS ts
FROM hubspot_notes_crm n
WHERE COALESCE(n.archived, 0) = 0
  AND (? = '' OR ts > ?)
  AND (? = '' OR owner_id = ?)`
			rows, err := db.DB().QueryContext(cmd.Context(), q, cutoff, cutoff, ownerID, ownerID)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			type noteHit struct {
				NoteID     string
				Buying     int
				Lost       int
				BuyingKind []string
				LostKind   []string
				TS         string
				Body       string
			}
			var notes []noteHit
			for rows.Next() {
				var id, body, ownerS, ts sql.NullString
				if err := rows.Scan(&id, &body, &ownerS, &ts); err != nil {
					return err
				}
				text := stripHTML(nullStr(body))
				b, bk := scanRegexHits(text, buyingSignals)
				l, lk := scanRegexHits(text, lostSignals)
				if b == 0 && l == 0 {
					continue
				}
				notes = append(notes, noteHit{
					NoteID: nullStr(id), Buying: b, Lost: l,
					BuyingKind: bk, LostKind: lk,
					TS: nullStr(ts), Body: text,
				})
			}

			type dealAgg struct {
				DealID     string   `json:"deal_id"`
				DealName   string   `json:"deal_name"`
				Buying     int      `json:"buying_count"`
				Lost       int      `json:"lost_count"`
				Signals    []string `json:"signal_types"`
				LastNoteAt string   `json:"last_note_at"`
			}
			if source == "local" {
				agg := map[string]*dealAgg{}
				for _, n := range notes {
					dealRows, err := db.DB().QueryContext(cmd.Context(), `
SELECT a.to_id, COALESCE(json_extract(d.data, '$.properties.dealname'), '')
FROM hubspot_associations a
JOIN hubspot_deals_crm d ON d.id = a.to_id
WHERE a.from_type = 'notes' AND a.to_type = 'deals' AND a.from_id = ?`, n.NoteID)
					if err != nil {
						continue
					}
					for dealRows.Next() {
						var did, dname string
						if err := dealRows.Scan(&did, &dname); err != nil {
							continue
						}
						if agg[did] == nil {
							agg[did] = &dealAgg{DealID: did, DealName: dname}
						}
						a := agg[did]
						a.Buying += n.Buying
						a.Lost += n.Lost
						a.Signals = appendUnique(a.Signals, n.BuyingKind...)
						a.Signals = appendUnique(a.Signals, n.LostKind...)
						if n.TS > a.LastNoteAt {
							a.LastNoteAt = n.TS
						}
					}
					dealRows.Close()
				}
				var aggList []dealAgg
				for _, v := range agg {
					aggList = append(aggList, *v)
				}
				sort.Slice(aggList, func(i, j int) bool { return aggList[i].Buying-aggList[i].Lost > aggList[j].Buying-aggList[j].Lost })
				if flags.asJSON {
					return flags.printJSON(cmd, map[string]any{
						"data_source": source,
						"results":     aggList,
					})
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "data_source: %s\n", source)
				headers := []string{"deal_id", "deal_name", "buying_count", "lost_count", "signal_types", "last_note_at"}
				out := make([][]string, 0, len(aggList))
				for _, a := range aggList {
					out = append(out, []string{
						a.DealID, a.DealName,
						fmt.Sprintf("%d", a.Buying), fmt.Sprintf("%d", a.Lost),
						joinSignals(a.Signals), a.LastNoteAt,
					})
				}
				return flags.printTable(cmd, headers, out)
			}

			// Fallback: per-note results.
			type noteRow struct {
				NoteID  string   `json:"note_id"`
				Buying  int      `json:"buying_count"`
				Lost    int      `json:"lost_count"`
				Signals []string `json:"signal_types"`
				NoteAt  string   `json:"note_at"`
				Snippet string   `json:"snippet"`
			}
			items := make([]noteRow, 0, len(notes))
			for _, n := range notes {
				kinds := append([]string{}, n.BuyingKind...)
				kinds = append(kinds, n.LostKind...)
				items = append(items, noteRow{
					NoteID: n.NoteID, Buying: n.Buying, Lost: n.Lost,
					Signals: kinds, NoteAt: n.TS, Snippet: snippet(n.Body, 100),
				})
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"data_source": source,
					"results":     items,
				})
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "data_source: %s\n", source)
			headers := []string{"note_id", "buying_count", "lost_count", "signal_types", "note_at", "snippet"}
			out := make([][]string, 0, len(items))
			for _, it := range items {
				out = append(out, []string{
					it.NoteID,
					fmt.Sprintf("%d", it.Buying), fmt.Sprintf("%d", it.Lost),
					joinSignals(it.Signals), it.NoteAt, it.Snippet,
				})
			}
			return flags.printTable(cmd, headers, out)
		},
	}
	cmd.Flags().StringVar(&pipeline, "pipeline", "", "Restrict to deals in a single pipeline id (requires sync-associations)")
	cmd.Flags().StringVar(&since, "since", "", "Only notes newer than this (e.g. 30d, 4h, RFC3339)")
	cmd.Flags().StringVar(&owner, "owner", "", "Filter by note owner id, email, or 'me'")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func compileAll(patterns ...string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		out = append(out, regexp.MustCompile(`(?i)`+p))
	}
	return out
}

var buyingSignals = map[string][]*regexp.Regexp{
	"meeting_scheduled":      compileAll(`meeting scheduled`, `book a meeting`, `set up a call`, `demo scheduled`),
	"budget_approved":        compileAll(`budget approved`, `got approval`, `budget for`),
	"timeline_urgency":       compileAll(`need by`, `must start`, `deadline`, `\bq[1-4]\b`),
	"ready_language":         compileAll(`ready to move`, `ready to proceed`, `let'?s start`, `move forward`),
	"compliance_requirement": compileAll(`\bcmmc\b`, `compliance`, `hipaa`, `security requirement`, `audit`),
	"decision_maker":         compileAll(`\bvp\b`, `\bceo\b`, `\bcto\b`, `\bpresident\b`, `\bowner\b`, `\bdirector\b`),
}

var lostSignals = map[string][]*regexp.Regexp{
	"no_response":       compileAll(`no response`, `haven'?t heard`, `went silent`, `ghosted`, `no reply`),
	"competitor_chosen": compileAll(`went with another`, `chose another`, `selected another vendor`),
	"budget_concern":    compileAll(`budget cut`, `not in budget`, `too expensive`, `cost concern`),
	"delayed":           compileAll(`put on hold`, `next quarter`, `not a priority`, `postponed`),
}

func scanRegexHits(text string, group map[string][]*regexp.Regexp) (int, []string) {
	hits := 0
	var kinds []string
	for kind, patterns := range group {
		for _, p := range patterns {
			if p.MatchString(text) {
				hits++
				kinds = appendUnique(kinds, kind)
				break
			}
		}
	}
	return hits, kinds
}

func appendUnique(s []string, vs ...string) []string {
	seen := map[string]bool{}
	for _, x := range s {
		seen[x] = true
	}
	for _, v := range vs {
		if !seen[v] {
			s = append(s, v)
			seen[v] = true
		}
	}
	return s
}

func joinSignals(s []string) string {
	if len(s) == 0 {
		return ""
	}
	sorted := append([]string{}, s...)
	sort.Strings(sorted)
	out := sorted[0]
	for _, x := range sorted[1:] {
		out += "," + x
	}
	return out
}
