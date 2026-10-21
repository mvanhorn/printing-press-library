// Copyright 2026 Elizabeth Stone and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/pinterest/internal/store"
)

var pinAuditIssueNames = []string{"missing_alt_text", "missing_link", "missing_title", "missing_description", "link_without_utm"}

type auditedPin struct {
	PinID     string   `json:"pin_id"`
	BoardID   string   `json:"board_id"`
	BoardName string   `json:"board_name"`
	Title     string   `json:"title"`
	Link      string   `json:"link"`
	Issues    []string `json:"issues"`
}

type pinAuditReport struct {
	ScannedPins    int          `json:"scanned_pins"`
	PinsWithIssues int          `json:"pins_with_issues"`
	Issues         []auditedPin `json:"issues"`
}

type boardAuditRow struct {
	BoardID            string   `json:"board_id"`
	BoardName          string   `json:"board_name"`
	Pins               int      `json:"pins"`
	MissingAltText     int      `json:"missing_alt_text"`
	MissingLink        int      `json:"missing_link"`
	MissingTitle       int      `json:"missing_title"`
	MissingDescription int      `json:"missing_description"`
	LinkWithoutUTM     int      `json:"link_without_utm"`
	PinIDsWithIssues   []string `json:"pin_ids_with_issues"`
}

// pinIssues returns the hygiene problems for one Pin, in a stable order.
func pinIssues(pin map[string]any) []string {
	issues := make([]string, 0)
	link := fieldString(pin, "link")
	if fieldString(pin, "alt_text") == "" {
		issues = append(issues, "missing_alt_text")
	}
	if link == "" {
		issues = append(issues, "missing_link")
	}
	if fieldString(pin, "title") == "" {
		issues = append(issues, "missing_title")
	}
	if fieldString(pin, "description") == "" {
		issues = append(issues, "missing_description")
	}
	if link != "" && !hasUTM(link) {
		issues = append(issues, "link_without_utm")
	}
	return issues
}

// auditPins applies the board/issue filters and returns the per-pin report
// plus per-board rollups (every scanned board, including clean ones).
func auditPins(pins []map[string]any, boards []localBoard, boardFilter, issueFilter string, limit int) (pinAuditReport, []boardAuditRow) {
	names := make(map[string]string, len(boards))
	for _, b := range boards {
		names[b.ID] = b.Name
	}
	report := pinAuditReport{Issues: make([]auditedPin, 0)}
	byBoard := map[string]*boardAuditRow{}
	for _, pin := range pins {
		boardID := fieldString(pin, "board_id")
		boardName := names[boardID]
		if boardFilter != "" && !strings.EqualFold(boardFilter, boardID) && !strings.EqualFold(boardFilter, boardName) {
			continue
		}
		report.ScannedPins++
		row := byBoard[boardID]
		if row == nil {
			row = &boardAuditRow{BoardID: boardID, BoardName: boardName, PinIDsWithIssues: make([]string, 0)}
			byBoard[boardID] = row
		}
		row.Pins++
		issues := pinIssues(pin)
		if issueFilter != "" {
			kept := make([]string, 0, 1)
			for _, is := range issues {
				if is == issueFilter {
					kept = append(kept, is)
				}
			}
			issues = kept
		}
		if len(issues) == 0 {
			continue
		}
		for _, is := range issues {
			switch is {
			case "missing_alt_text":
				row.MissingAltText++
			case "missing_link":
				row.MissingLink++
			case "missing_title":
				row.MissingTitle++
			case "missing_description":
				row.MissingDescription++
			case "link_without_utm":
				row.LinkWithoutUTM++
			}
		}
		pinID := fieldString(pin, "id")
		row.PinIDsWithIssues = append(row.PinIDsWithIssues, pinID)
		report.PinsWithIssues++
		if len(report.Issues) < limit {
			report.Issues = append(report.Issues, auditedPin{
				PinID: pinID, BoardID: boardID, BoardName: boardName,
				Title: fieldString(pin, "title"), Link: fieldString(pin, "link"), Issues: issues,
			})
		}
	}
	grouped := make([]boardAuditRow, 0, len(byBoard))
	for _, row := range byBoard {
		grouped = append(grouped, *row)
	}
	sort.Slice(grouped, func(i, j int) bool {
		if len(grouped[i].PinIDsWithIssues) != len(grouped[j].PinIDsWithIssues) {
			return len(grouped[i].PinIDsWithIssues) > len(grouped[j].PinIDsWithIssues)
		}
		return grouped[i].BoardID < grouped[j].BoardID
	})
	return report, grouped
}

func newNovelPinsAuditCmd(flags *rootFlags) *cobra.Command {
	var groupBy, board, issue, dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Find published Pins missing alt text, a link, a title, or UTM tracking",
		Long: "Use this command to find problems in Pins already published and synced.\n" +
			"Do NOT use this command to check a Pin you have not created yet; use 'pins preflight' instead.\n\n" +
			"Reads the local store only; run 'pinterest-pp-cli sync' first.",
		Example: strings.Trim(`
  pinterest-pp-cli pins audit --group-by board --agent
  pinterest-pp-cli pins audit --issue missing_alt_text --limit 50 --json
  pinterest-pp-cli pins audit --board "Recipes" --agent`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "--group-by=board",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "pins audit")
			}
			if groupBy != "" && groupBy != "board" {
				return usageErr(fmt.Errorf("--group-by must be \"board\" or omitted, got %q", groupBy))
			}
			if issue != "" && !containsString(pinAuditIssueNames, issue) {
				return usageErr(fmt.Errorf("--issue must be one of %s, got %q", strings.Join(pinAuditIssueNames, ", "), issue))
			}
			if limit < 1 {
				return usageErr(fmt.Errorf("--limit must be at least 1, got %d", limit))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if dbPath == "" {
				dbPath = defaultDBPath("pinterest-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: pinterest-pp-cli sync --resources pins,boards --db %s\n", dbPath, dbPath)
				report, grouped := auditPins(nil, nil, "", "", limit)
				return emitPinAudit(cmd, flags, report, grouped, groupBy)
			}
			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "pins") {
				hintIfStale(cmd, db, "pins", 0)
			}
			pins, err := loadSyncedPins(ctx, db)
			if err != nil {
				return err
			}
			boards, err := loadSyncedBoards(ctx, db)
			if err != nil {
				return err
			}
			report, grouped := auditPins(pins, boards, board, issue, limit)
			return emitPinAudit(cmd, flags, report, grouped, groupBy)
		},
	}
	cmd.Flags().StringVar(&board, "board", "", "Only audit Pins on this synced board (name or id)")
	cmd.Flags().StringVar(&groupBy, "group-by", "", "Group results per board instead of per Pin (only \"board\")")
	cmd.Flags().StringVar(&issue, "issue", "", "Only report one issue type, e.g. missing_alt_text or link_without_utm")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum number of per-Pin issue rows to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite store (defaults to the CLI data dir)")
	return cmd
}

func emitPinAudit(cmd *cobra.Command, flags *rootFlags, report pinAuditReport, grouped []boardAuditRow, groupBy string) error {
	w := cmd.OutOrStdout()
	if groupBy == "board" {
		if !wantsHumanTable(w, flags) {
			return printJSONFiltered(w, grouped, flags)
		}
		if len(grouped) == 0 {
			fmt.Fprintln(w, "No synced Pins to audit.")
			return nil
		}
		fmt.Fprintf(w, "%-20s %-28s %5s %5s %5s %5s %5s %5s\n", "BOARD_ID", "BOARD", "PINS", "ALT", "LINK", "TITLE", "DESC", "UTM")
		for _, r := range grouped {
			fmt.Fprintf(w, "%-20s %-28s %5d %5d %5d %5d %5d %5d\n", r.BoardID, truncateRunes(r.BoardName, 28), r.Pins,
				r.MissingAltText, r.MissingLink, r.MissingTitle, r.MissingDescription, r.LinkWithoutUTM)
		}
		return nil
	}
	if !wantsHumanTable(w, flags) {
		return printJSONFiltered(w, report, flags)
	}
	if len(report.Issues) == 0 {
		fmt.Fprintf(w, "No issues found across %d synced Pins.\n", report.ScannedPins)
		return nil
	}
	for _, p := range report.Issues {
		fmt.Fprintf(w, "%s\t%s\t%s\n", p.PinID, p.BoardName, strings.Join(p.Issues, ","))
	}
	fmt.Fprintf(w, "\n%d of %d Pins have issues.\n", report.PinsWithIssues, report.ScannedPins)
	return nil
}
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
