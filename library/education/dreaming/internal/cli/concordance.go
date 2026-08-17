// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local
// concordance: keyword-in-context (KWIC) search over the cached transcript
// corpus. Supports plain words/phrases (FTS5), raw regex, and heuristic
// Spanish verb-tense presets, scoped by catalog metadata. Hand-built headline
// feature. Build the corpus first with `transcript sync`.

package cli

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/education/dreaming/internal/config"
	"github.com/mvanhorn/printing-press-library/library/education/dreaming/internal/store"

	"github.com/spf13/cobra"
)

func newNovelConcordanceCmd(flags *rootFlags) *cobra.Command {
	var flagTense string
	var flagRegex string
	var flagLevel string
	var flagGuide string
	var flagDialect string
	var flagLimit int
	var dbOverride string

	cmd := &cobra.Command{
		Use:   "concordance [query]",
		Short: "Search every cached transcript for a word, phrase, regex, or verb-tense pattern (KWIC)",
		Long: "Search the cached transcript corpus and return each hit in context with the\n" +
			"video, level, guide, and cue timestamp. Provide a word/phrase (FTS5), a\n" +
			"--regex, or a --tense preset. Verb-tense presets are heuristic regexes over\n" +
			"Spanish conjugation endings (they over-match nouns sharing endings), useful\n" +
			"as a first-order filter. Given several of query/--regex/--tense, hits must\n" +
			"match all of them. Build the corpus first with 'transcript sync'.\n\n" +
			"Tense presets: " + tensePresetNames(),
		Example: strings.Trim(`
  dreaming-pp-cli concordance "entonces"
  dreaming-pp-cli concordance --tense subjunctive-imperfect --level intermediate
  dreaming-pp-cli concordance --regex "\bhab(ía|ían)\b" --guide "Pablo"
  dreaming-pp-cli concordance "me gusta" --agent --select context,video_title,timestamp
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			query := ""
			if len(args) > 0 {
				query = strings.TrimSpace(strings.Join(args, " "))
			}
			// Require at least one of: query, --regex, --tense.
			if query == "" && flagRegex == "" && flagTense == "" {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("provide a search term, --regex, or --tense (one of: %s)", tensePresetNames()))
			}
			if flagTense != "" {
				if _, ok := tensePresets[flagTense]; !ok {
					return usageErr(fmt.Errorf("unknown --tense %q: must be one of %s", flagTense, tensePresetNames()))
				}
			}
			if dryRunOK(flags) {
				return nil
			}

			db, err := openDreamingStoreAt(cmd.Context(), dbOverride)
			if err != nil {
				return err
			}
			defer db.Close()
			vids, _, err := db.TranscriptStats()
			if err != nil {
				return err
			}
			if vids == 0 {
				return notFoundErr(fmt.Errorf("transcript corpus is empty — run 'dreaming-pp-cli transcript sync' first"))
			}

			// Load config once so each hit can carry the canonical web URL for
			// the video (honors a custom base_url if the host moved).
			cfg, cerr := config.Load(flags.configPath)
			if cerr != nil {
				return configErr(cerr)
			}

			limit := flagLimit
			if limit <= 0 {
				limit = 50
			}
			filter := store.TranscriptFilter{Level: flagLevel, Guide: flagGuide, Dialect: flagDialect, Limit: limit}

			var hits []store.ConcordanceHit
			switch {
			case flagRegex != "" || flagTense != "":
				// When both are given, hits must match both — mirroring how a
				// plain query ANDs with either flag below.
				var patterns []*regexp.Regexp
				if flagRegex != "" {
					re, cerr := regexp.Compile(flagRegex)
					if cerr != nil {
						return usageErr(fmt.Errorf("invalid --regex %q: %w", flagRegex, cerr))
					}
					patterns = append(patterns, re)
				}
				if flagTense != "" {
					re, cerr := regexp.Compile(tensePresets[flagTense])
					if cerr != nil {
						return fmt.Errorf("compiling --tense %q preset: %w", flagTense, cerr)
					}
					patterns = append(patterns, re)
				}
				all, serr := db.AllCuesForScan(filter)
				if serr != nil {
					return serr
				}
				for _, h := range all {
					matched := true
					for _, re := range patterns {
						if !re.MatchString(h.Text) {
							matched = false
							break
						}
					}
					if !matched {
						continue
					}
					// If a plain query was ALSO given, require it as a substring.
					if query != "" && !strings.Contains(strings.ToLower(h.Text), strings.ToLower(query)) {
						continue
					}
					h.Timestamp = formatTimestamp(h.StartMS)
					h.VideoURL = cfg.VideoWebURL(h.VideoID)
					hits = append(hits, h)
					if len(hits) >= limit {
						break
					}
				}
			default:
				ftsHits, serr := db.SearchTranscriptFTS(ftsQuery(query), filter)
				if serr != nil {
					return serr
				}
				for _, h := range ftsHits {
					h.Timestamp = formatTimestamp(h.StartMS)
					h.VideoURL = cfg.VideoWebURL(h.VideoID)
					hits = append(hits, h)
				}
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), hits, flags)
			}
			if len(hits) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matches in the cached corpus.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "TIMESTAMP\tLEVEL\tGUIDE\tVIDEO\tCONTEXT\tURL")
			for _, h := range hits {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", h.Timestamp, h.Level, truncate(h.Guide, 16), truncate(h.VideoTitle, 28), truncate(h.Text, 70), h.VideoURL)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&flagTense, "tense", "", "Spanish verb-tense preset (one of: "+tensePresetNames()+")")
	cmd.Flags().StringVar(&flagRegex, "regex", "", "Raw RE2 regex to match against cue text")
	cmd.Flags().StringVar(&flagLevel, "level", "", "Scope to a catalog level (superbeginner, beginner, intermediate, advanced)")
	cmd.Flags().StringVar(&flagGuide, "guide", "", "Scope to a guide/teacher")
	cmd.Flags().StringVar(&flagDialect, "dialect", "", "Scope to a dialect/accent")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Max hits to return")
	cmd.Flags().StringVar(&dbOverride, "db", "", "Database path (default: ~/.local/share/dreaming-pp-cli/data.db)")
	return cmd
}

// ftsQuery phrase-quotes the query so FTS5 treats it as a literal phrase:
// spaces mean "phrase" rather than implicit AND, and special characters
// (*, -, +, parens, AND/OR/NOT/NEAR) match literally instead of surfacing a
// cryptic "fts5: syntax error". Power users who want pattern matching have
// --regex.
func ftsQuery(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return q
	}
	return `"` + strings.ReplaceAll(q, `"`, `""`) + `"`
}
