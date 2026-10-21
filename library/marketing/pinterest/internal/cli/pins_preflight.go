// Copyright 2026 Elizabeth Stone and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source computed

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/pinterest/internal/store"
)

// Field limits from the v5 PinCreate schema.
var pinFieldLimits = []struct {
	name string
	max  int
	get  func(p preflightProposal) string
}{
	{"title", 100, func(p preflightProposal) string { return p.Title }},
	{"description", 800, func(p preflightProposal) string { return p.Description }},
	{"alt_text", 500, func(p preflightProposal) string { return p.AltText }},
	{"link", 2048, func(p preflightProposal) string { return p.Link }},
}

type preflightProposal struct {
	Board       string `json:"board"`
	Section     string `json:"section"`
	Title       string `json:"title"`
	Description string `json:"description"`
	AltText     string `json:"alt_text"`
	Link        string `json:"link"`
	ImageURL    string `json:"image_url"`
}

type preflightResult struct {
	OK         bool           `json:"ok"`
	Errors     []string       `json:"errors"`
	Warnings   []string       `json:"warnings"`
	CreateBody map[string]any `json:"create_body"`
}

// preflightPin validates one proposal against spec limits and the synced
// boards/sections. With no synced boards, board checks degrade to warnings.
func preflightPin(p preflightProposal, boards []localBoard, sections []map[string]string) preflightResult {
	res := preflightResult{Errors: make([]string, 0), Warnings: make([]string, 0)}
	boardID := ""
	switch {
	case strings.TrimSpace(p.Board) == "":
		res.Errors = append(res.Errors, "board_required")
	case len(boards) == 0:
		// No synced boards to resolve against: a numeric value is passed through as
		// an id; a name cannot become board_id, so the body is not postable yet.
		if isNumericID(p.Board) {
			res.Warnings = append(res.Warnings, "board_unverified")
			boardID = p.Board
		} else {
			res.Errors = append(res.Errors, "board_unresolved")
		}
	default:
		matches := make([]localBoard, 0, 1)
		for _, b := range boards {
			if b.ID == p.Board || strings.EqualFold(b.Name, p.Board) {
				matches = append(matches, b)
			}
		}
		switch len(matches) {
		case 0:
			res.Errors = append(res.Errors, "board_not_found")
		case 1:
			boardID = matches[0].ID
		default:
			res.Errors = append(res.Errors, "board_ambiguous")
		}
	}
	sectionID := ""
	if p.Section != "" {
		found := false
		for _, s := range sections {
			if (boardID == "" || s["board_id"] == "" || s["board_id"] == boardID) &&
				(s["id"] == p.Section || strings.EqualFold(s["name"], p.Section)) {
				sectionID, found = s["id"], true
				break
			}
		}
		if !found {
			// Pinterest needs a numeric section id; an unresolved name is not postable.
			if isNumericID(p.Section) {
				res.Warnings = append(res.Warnings, "section_unverified")
				sectionID = p.Section
			} else {
				res.Errors = append(res.Errors, "section_unresolved")
			}
		}
	}
	for _, f := range pinFieldLimits {
		if utf8.RuneCountInString(f.get(p)) > f.max {
			res.Errors = append(res.Errors, f.name+"_too_long")
		}
	}
	if strings.TrimSpace(p.ImageURL) == "" {
		res.Errors = append(res.Errors, "image_url_required")
	} else if !isHTTPURL(p.ImageURL) {
		res.Errors = append(res.Errors, "image_url_invalid")
	}
	if p.Link != "" && !isHTTPURL(p.Link) {
		res.Errors = append(res.Errors, "link_invalid")
	}
	if strings.TrimSpace(p.AltText) == "" {
		res.Warnings = append(res.Warnings, "missing_alt_text")
	}
	if strings.TrimSpace(p.Title) == "" {
		res.Warnings = append(res.Warnings, "missing_title")
	}
	if p.Link == "" {
		res.Warnings = append(res.Warnings, "missing_link")
	} else if !hasUTM(p.Link) {
		res.Warnings = append(res.Warnings, "link_without_utm")
	}
	body := map[string]any{"media_source": map[string]string{"source_type": "image_url", "url": p.ImageURL}}
	for k, v := range map[string]string{"board_id": boardID, "board_section_id": sectionID, "title": p.Title,
		"description": p.Description, "alt_text": p.AltText, "link": p.Link} {
		if v != "" {
			body[k] = v
		}
	}
	res.CreateBody = body
	res.OK = len(res.Errors) == 0
	return res
}

func newNovelPinsPreflightCmd(flags *rootFlags) *cobra.Command {
	var p preflightProposal
	var dbPath string
	var fromStdin, strict bool

	cmd := &cobra.Command{
		Use:   "preflight",
		Short: "Validate a proposed Pin offline and print a ready-to-send create body",
		Long: "Use this command to check a Pin before creating it.\n" +
			"Do NOT use this command for Pins that already exist; use 'pins audit' instead.\n\n" +
			"Checks spec field limits, resolves the board and section by name from the local store,\n" +
			"and flags missing alt text or UTM tags. It never calls the API and never writes.",
		Example: strings.Trim(`
  pinterest-pp-cli pins preflight --board "Recipes" --title "Weeknight dinner ideas" --alt-text "Bowl of pasta on a table" --link "https://example.com/blog/meal-prep?utm_source=pinterest" --image-url https://example.com/img/meal-prep.jpg --agent
  cat batch.ndjson | pinterest-pp-cli pins preflight --stdin --json`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "computed",
			// With --strict, exit 2 after printing when any proposal has errors.
			"pp:typed-exit-codes": "0,2",
			"pp:happy-args":       "--board=example-board;--title=Weeknight dinner ideas;--alt-text=Bowl of pasta on a table;--link=https://example.com/blog/meal-prep?utm_source=pinterest;--image-url=https://example.com/img/meal-prep.jpg",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "pins preflight")
			}
			if !fromStdin && (p.Board == "" || p.ImageURL == "") {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--board and --image-url are required (or pass proposals with --stdin)"))
			}
			proposals := []preflightProposal{p}
			if fromStdin {
				var err error
				if proposals, err = readPreflightNDJSON(cmd); err != nil {
					return err
				}
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if dbPath == "" {
				dbPath = defaultDBPath("pinterest-pp-cli")
			}
			boards := []localBoard{}
			var sections []map[string]string
			if _, statErr := os.Stat(dbPath); statErr == nil {
				db, err := store.OpenReadOnlyContext(ctx, dbPath)
				if err != nil {
					return fmt.Errorf("opening database: %w", err)
				}
				defer db.Close()
				hintIfUnsynced(cmd, db, "boards")
				if boards, err = loadSyncedBoards(ctx, db); err != nil {
					return err
				}
				sections = loadSyncedSections(ctx, db)
			}
			results := make([]preflightResult, 0, len(proposals))
			failed := 0
			for _, prop := range proposals {
				r := preflightPin(prop, boards, sections)
				if !r.OK {
					failed++
				}
				results = append(results, r)
			}
			w := cmd.OutOrStdout()
			if !wantsHumanTable(w, flags) {
				var err error
				if fromStdin {
					err = printJSONFiltered(w, results, flags)
				} else {
					err = printJSONFiltered(w, results[0], flags)
				}
				if err != nil {
					return err
				}
				return preflightExit(strict, failed, len(results))
			}
			for i, r := range results {
				status := "OK"
				if !r.OK {
					status = "FAIL"
				}
				fmt.Fprintf(w, "#%d %s errors=%s warnings=%s\n", i+1, status, strings.Join(r.Errors, ","), strings.Join(r.Warnings, ","))
			}
			if failed > 0 {
				fmt.Fprintf(w, "\n%d of %d proposals have errors; fix them before 'pins create'.\n", failed, len(results))
			}
			return preflightExit(strict, failed, len(results))
		},
	}
	cmd.Flags().StringVar(&p.Board, "board", "", "Target board name or id (resolved from the local store)")
	cmd.Flags().StringVar(&p.Section, "section", "", "Optional board section name or id within the board")
	cmd.Flags().StringVar(&p.Title, "title", "", "Proposed Pin title (max 100 characters)")
	cmd.Flags().StringVar(&p.Description, "description", "", "Proposed Pin description (max 800 characters)")
	cmd.Flags().StringVar(&p.AltText, "alt-text", "", "Accessible image description (max 500 characters)")
	cmd.Flags().StringVar(&p.Link, "link", "", "Destination URL, ideally with utm_* tracking parameters")
	cmd.Flags().StringVar(&p.ImageURL, "image-url", "", "Public http(s) URL of the Pin image")
	cmd.Flags().BoolVar(&strict, "strict", false, "Exit 2 when any proposal has errors (for preflight && pins create chains)")
	cmd.Flags().BoolVar(&fromStdin, "stdin", false, "Read NDJSON proposals (board, section, title, description, alt_text, link, image_url) from stdin")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite store (defaults to the CLI data dir)")
	return cmd
}

// preflightExit fails the command (exit 2) after the report is printed when
// --strict is set and any proposal has blocking errors.
func preflightExit(strict bool, failed, total int) error {
	if !strict || failed == 0 {
		return nil
	}
	return usageErr(fmt.Errorf("%d of %d Pin proposals failed preflight", failed, total))
}

func readPreflightNDJSON(cmd *cobra.Command) ([]preflightProposal, error) {
	out := make([]preflightProposal, 0)
	sc := bufio.NewScanner(cmd.InOrStdin())
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" {
			continue
		}
		var p preflightProposal
		if err := json.Unmarshal([]byte(text), &p); err != nil {
			return nil, usageErr(fmt.Errorf("stdin line %d: invalid JSON proposal: %w", line, err))
		}
		out = append(out, p)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("reading stdin: %w", err)
	}
	if len(out) == 0 {
		return nil, usageErr(fmt.Errorf("--stdin given but no proposals were read"))
	}
	return out, nil
}

func isNumericID(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
