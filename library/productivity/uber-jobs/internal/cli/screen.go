// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"unicode"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/uberjobs"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNovelScreenCmd(flags))
	})
}

// screenEvidence is one phrase checked against one posting.
type screenEvidence struct {
	Phrase   string  `json:"phrase"`
	Kind     string  `json:"kind"`
	Matched  bool    `json:"matched"`
	Strength *string `json:"strength"`
	Sentence *string `json:"sentence"`
}

// screenRow is a posting with its verdict and the evidence behind it.
type screenRow struct {
	uberjobs.Posting
	Verdict  string           `json:"verdict"`
	Evidence []screenEvidence `json:"evidence"`
}

var screenVerdicts = map[string]bool{"keep": true, "drop": true, "unscreened": true, "all": true}

func newNovelScreenCmd(flags *rootFlags) *cobra.Command {
	var pf postingFlags
	var require, exclude []string
	var verdict string
	cmd := &cobra.Command{
		Use:   "screen",
		Short: "Keep or drop postings by phrases in their description and show the sentence each phrase matched",
		Long: strings.Trim(`
Screen postings by phrases in their plain-text description, with the evidence sentence

A posting is dropped when any --exclude phrase appears or any --require phrase
is missing, and kept otherwise; a posting without a description is unscreened.
Matching is case-insensitive over the HTML-stripped description. Each evidence
entry names the phrase, whether it matched, its strength (word when it matched
on word boundaries, substring when only inside a longer word), and the sentence
it matched, so a negation such as "no fluent Dutch needed" can be judged.

auto screens the local store when it holds a complete sync, and otherwise reads
the matching postings live once (two requests). The country, team, keyword,
and recency filters narrow the candidates first. --verdict picks which rows
to return (keep by default); meta.extra counts every verdict.`, "\n"),
		Example: strings.Trim(`
  uber-jobs-pp-cli screen --country NLD --posted-within 7d --exclude "fluent Dutch" --verdict all --agent
  uber-jobs-pp-cli screen --country USA --exclude sponsorship --verdict drop --json
  uber-jobs-pp-cli screen --country GBR --require "stakeholder management" --exclude "security clearance"`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "auto",
			"pp:happy-args":  "--country=NLD;--exclude=fluent Dutch",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) > 0 {
				return usageErr(fmt.Errorf("screen takes no arguments; pass phrases with --require or --exclude"))
			}
			f, err := pf.filters()
			if err != nil {
				return err
			}
			ds, err := resolveDataSource(cmd, flags, "auto", "live", "local")
			if err != nil {
				return err
			}
			v := strings.ToLower(strings.TrimSpace(verdict))
			if !screenVerdicts[v] {
				return usageErr(fmt.Errorf("--verdict must be keep, drop, unscreened, or all, got %q", verdict))
			}
			req, exc := cleanPhrases(require), cleanPhrases(exclude)
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			now := nowUTC()
			resolved, last, err := resolveLocalFirst(ctx, ds, pf.dbPath, 0, now)
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, describeSource(flags, resolved, pf.dbPath, f)+fmt.Sprintf(", then screen descriptions (require %d, exclude %d phrases)", len(req), len(exc)))
			}
			if len(req) == 0 && len(exc) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("give at least one --require or --exclude phrase"))
			}
			env, rows, err := gatherPostings(ctx, flags, resolved, pf.dbPath, f, now)
			if err != nil {
				return err
			}
			if resolved == "local" && env.Meta.Note == "" {
				env.Meta.Note = localFirstNote("local", last)
			}
			env.Meta.Command = "screen"
			sortPostings(rows, f.Sort)
			env.Meta.Note = joinNote(env.Meta.Note, secondaryCountryNote(rows, f.Countries))
			counts := map[string]int{"keep": 0, "drop": 0, "unscreened": 0}
			picked := make([]screenRow, 0, len(rows))
			for _, p := range rows {
				r := screenPosting(p, req, exc)
				counts[r.Verdict]++
				if v == "all" || r.Verdict == v {
					picked = append(picked, r)
				}
			}
			if v == "all" {
				picked = orderByVerdict(picked)
			}
			env.Meta.Extra = map[string]any{"candidates": len(rows), "keep": counts["keep"], "drop": counts["drop"], "unscreened": counts["unscreened"], "verdict": v}
			if v == "keep" && counts["drop"] > 0 {
				env.Meta.Note = strings.TrimSpace(env.Meta.Note + fmt.Sprintf(" %d dropped; rerun with --verdict drop to read the matched sentences", counts["drop"]))
			}
			env.Hits = len(picked)
			page := pageScreenRows(picked, pf.offset, pf.limit)
			env.Returned = len(page)
			env.Results = page
			return printEnvelope(cmd, flags, env, func(w io.Writer) error {
				tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
				fmt.Fprintln(tw, "ID\tVERDICT\tCOUNTRY\tTITLE\tEVIDENCE")
				for _, r := range page {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.ID, r.Verdict, deref(r.CountryCode, "-"), r.Title, evidenceSummary(r.Evidence))
				}
				if err := tw.Flush(); err != nil {
					return err
				}
				fmt.Fprintf(w, "\n%d of %d shown; keep %d, drop %d, unscreened %d (source: %s)\n", len(page), len(picked), counts["keep"], counts["drop"], counts["unscreened"], env.Meta.Source)
				if env.Meta.Note != "" {
					fmt.Fprintln(w, "note:", env.Meta.Note)
				}
				return nil
			})
		},
	}
	registerPostingFlags(cmd, &pf, true, false)
	cmd.Flags().StringArrayVar(&require, "require", nil, "Keep only postings whose description contains this phrase (repeatable; all must match)")
	cmd.Flags().StringArrayVar(&exclude, "exclude", nil, "Drop postings whose description contains this phrase (repeatable; any match drops)")
	cmd.Flags().StringVar(&verdict, "verdict", "keep", "Rows to return: keep, drop, unscreened, or all")
	addDataSourceFlag(cmd, "auto", "Where to read: auto (the local store when it holds a complete sync, else live), live, or local")
	return cmd
}

// cleanPhrases trims phrases, collapses inner whitespace, and drops blanks.
func cleanPhrases(in []string) []string {
	out := make([]string, 0, len(in))
	for _, p := range in {
		if p = strings.Join(strings.Fields(p), " "); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// screenPosting checks every phrase against one posting's description.
func screenPosting(p uberjobs.Posting, require, exclude []string) screenRow {
	row := screenRow{Posting: p, Evidence: make([]screenEvidence, 0, len(require)+len(exclude))}
	if p.Description == nil || strings.TrimSpace(*p.Description) == "" {
		row.Verdict = "unscreened"
		return row
	}
	sentences := uberjobs.Sentences(*p.Description)
	flat := strings.Join(strings.Fields(*p.Description), " ")
	drop := false
	for _, phrase := range exclude {
		ev := matchPhrase(phrase, "exclude", sentences, flat)
		drop = drop || ev.Matched
		row.Evidence = append(row.Evidence, ev)
	}
	for _, phrase := range require {
		ev := matchPhrase(phrase, "require", sentences, flat)
		drop = drop || !ev.Matched
		row.Evidence = append(row.Evidence, ev)
	}
	row.Verdict = "keep"
	if drop {
		row.Verdict = "drop"
	}
	return row
}

// matchPhrase finds the phrase case-insensitively, preferring a sentence where
// it matches on word boundaries. A phrase that spans a sentence break still
// matches, with a window of surrounding text as its evidence.
func matchPhrase(phrase, kind string, sentences []string, flat string) screenEvidence {
	ev := screenEvidence{Phrase: phrase, Kind: kind}
	needle := strings.ToLower(phrase)
	var substringHit *string
	for _, s := range sentences {
		lower := strings.ToLower(s)
		if !strings.Contains(lower, needle) {
			continue
		}
		sentence := s
		if wordMatch(lower, needle) {
			strength := "word"
			ev.Matched, ev.Strength, ev.Sentence = true, &strength, &sentence
			return ev
		}
		if substringHit == nil {
			substringHit = &sentence
		}
	}
	if substringHit != nil {
		strength := "substring"
		ev.Matched, ev.Strength, ev.Sentence = true, &strength, substringHit
		return ev
	}
	lower := strings.ToLower(flat)
	if i := strings.Index(lower, needle); i >= 0 {
		strength := "substring"
		if wordMatch(lower, needle) {
			strength = "word"
		}
		window := snippet(flat, i, len(needle))
		ev.Matched, ev.Strength, ev.Sentence = true, &strength, &window
	}
	return ev
}

// wordMatch reports whether needle occurs in hay with non-word runes (or the
// text edge) on both sides.
func wordMatch(hay, needle string) bool {
	for start := 0; start <= len(hay)-len(needle); {
		i := strings.Index(hay[start:], needle)
		if i < 0 {
			return false
		}
		i += start
		end := i + len(needle)
		if boundaryBefore(hay, i) && boundaryAfter(hay, end) {
			return true
		}
		start = i + 1
	}
	return false
}

func boundaryBefore(s string, i int) bool {
	if i == 0 {
		return true
	}
	r := []rune(s[:i])
	return !isWordRune(r[len(r)-1])
}

func boundaryAfter(s string, end int) bool {
	if end >= len(s) {
		return true
	}
	for _, r := range s[end:] {
		return !isWordRune(r)
	}
	return true
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// snippet returns about 80 bytes of context around a match, cut on rune edges.
func snippet(s string, at, n int) string {
	start, end := at-80, at+n+80
	if start < 0 {
		start = 0
	}
	if end > len(s) {
		end = len(s)
	}
	for start > 0 && !utf8RuneStart(s[start]) {
		start--
	}
	for end < len(s) && !utf8RuneStart(s[end]) {
		end++
	}
	return strings.TrimSpace(s[start:end])
}

func utf8RuneStart(b byte) bool { return b&0xC0 != 0x80 }

// orderByVerdict lists keep, then unscreened, then drop, keeping the sorted
// order inside each group.
func orderByVerdict(rows []screenRow) []screenRow {
	out := make([]screenRow, 0, len(rows))
	for _, v := range []string{"keep", "unscreened", "drop"} {
		for _, r := range rows {
			if r.Verdict == v {
				out = append(out, r)
			}
		}
	}
	return out
}

func pageScreenRows(rows []screenRow, offset, limit int) []screenRow {
	if offset >= len(rows) {
		return []screenRow{}
	}
	rows = rows[offset:]
	if limit > 0 && limit < len(rows) {
		rows = rows[:limit]
	}
	out := make([]screenRow, len(rows))
	copy(out, rows)
	return out
}

func evidenceSummary(ev []screenEvidence) string {
	parts := make([]string, 0, len(ev))
	for _, e := range ev {
		mark := "no"
		if e.Matched {
			mark = "yes"
		}
		parts = append(parts, fmt.Sprintf("%s %q: %s", e.Kind, e.Phrase, mark))
	}
	return strings.Join(parts, "; ")
}
