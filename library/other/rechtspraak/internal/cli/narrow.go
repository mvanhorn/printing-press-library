// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0.

package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/rechtspraak/internal/rechtspraak"
)

func newNovelNarrowCmd(flags *rootFlags) *cobra.Command {
	var (
		flagKeyword  []string
		flagExclude  []string
		flagPhrase   []string
		flagRegex    []string
		flagFile     string
		flagSummary  bool
		flagFull     bool
		flagAnnotate bool
	)

	cmd := &cobra.Command{
		Use:   "narrow [< ecli-list]",
		Short: "Narrow an ECLI list locally by keyword, exclusion, phrase, or regex",
		Long: `Read ECLIs from stdin (one per line, or JSON output from search/watch/sync),
fetch each decision's metadata + inhoudsindicatie + body, and apply local
filters. The upstream API has no free-text search; this is where keyword
narrowing happens for long-tail Dutch legal terms.

  --keyword  required term (case-insensitive) — repeat for AND
  --exclude  forbidden term (case-insensitive) — repeat for NOT
  --phrase   required exact phrase
  --regex    must match Go-syntax regex (per https://pkg.go.dev/regexp/syntax)
  --summary-only  match only against inhoudsindicatie (not body)
  --full-only    match only against the uitspraak body
  --file F   read ECLIs from F instead of stdin
  --annotate-count   print "N survivors of M" to stderr after the run`,
		Example: `  rechtspraak-pp-cli search --court HR --from 2024-01-01 --json | jq -r '.[] .ecli' | rechtspraak-pp-cli narrow --keyword huurprijs --exclude "kort geding"
  rechtspraak-pp-cli narrow --keyword belastingrecht --phrase "omkering bewijslast" --file eclis.txt`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if flagSummary && flagFull {
				return fmt.Errorf("--summary-only and --full-only are mutually exclusive")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			var src io.Reader
			if flagFile != "" {
				f, err := os.Open(flagFile)
				if err != nil {
					return fmt.Errorf("open %s: %w", flagFile, err)
				}
				defer f.Close()
				src = f
			} else if len(args) == 0 {
				// Read from stdin by default — narrow is a pipe filter.
				src = cmd.InOrStdin()
			} else {
				return cmd.Help()
			}
			eclis := readECLIs(src)
			if len(eclis) == 0 {
				if flagAnnotate {
					fmt.Fprintln(cmd.ErrOrStderr(), "narrow: no input ECLIs")
				}
				return nil
			}
			regexes := make([]*regexp.Regexp, 0, len(flagRegex))
			for _, r := range flagRegex {
				re, err := regexp.Compile(r)
				if err != nil {
					return fmt.Errorf("bad --regex %q: %w", r, err)
				}
				regexes = append(regexes, re)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			http := mustHTTP()
			survivors := make([]string, 0, len(eclis))
			for _, ecli := range eclis {
				// Honour context cancellation before each potentially
				// expensive HTTP call so --timeout / Ctrl-C / agent
				// deadline doesn't silently truncate the result set as
				// "false" survivors.
				if err := ctx.Err(); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "narrow: scan interrupted (%v); %d of %d ECLIs processed.\n", err, len(survivors), len(eclis))
					return err
				}
				ok, err := narrowMatch(ctx, http, ecli, flagKeyword, flagExclude, flagPhrase, regexes, flagSummary, flagFull)
				if err != nil {
					if flagAnnotate {
						fmt.Fprintf(cmd.ErrOrStderr(), "narrow: skip %s (%v)\n", ecli, err)
					}
					continue
				}
				if ok {
					survivors = append(survivors, ecli)
				}
			}
			if flagAnnotate {
				fmt.Fprintf(cmd.ErrOrStderr(), "narrow: %d survivors of %d\n", len(survivors), len(eclis))
			}
			if shouldEmitJSON(cmd.OutOrStdout(), flags) {
				return writeJSONOut(cmd.OutOrStdout(), map[string]any{
					"input_count":    len(eclis),
					"survivor_count": len(survivors),
					"ecli":           survivors,
				})
			}
			for _, e := range survivors {
				fmt.Fprintln(cmd.OutOrStdout(), e)
			}
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&flagKeyword, "keyword", nil, "Required term (AND, repeatable, case-insensitive)")
	cmd.Flags().StringSliceVar(&flagExclude, "exclude", nil, "Forbidden term (NOT, repeatable, case-insensitive)")
	cmd.Flags().StringSliceVar(&flagPhrase, "phrase", nil, "Required exact phrase (repeatable)")
	cmd.Flags().StringSliceVar(&flagRegex, "regex", nil, "Go-syntax regex that must match (repeatable)")
	cmd.Flags().StringVar(&flagFile, "file", "", "Read ECLIs from this file instead of stdin")
	cmd.Flags().BoolVar(&flagSummary, "summary-only", false, "Match only against inhoudsindicatie")
	cmd.Flags().BoolVar(&flagFull, "full-only", false, "Match only against the uitspraak body")
	cmd.Flags().BoolVar(&flagAnnotate, "annotate-count", false, "Print survivor counts to stderr")
	return cmd
}

// readECLIs accepts four input shapes:
//   - pretty- or single-line JSON array of strings: ["ECLI:...","ECLI:..."]
//   - pretty- or single-line JSON object with "entries" or "ecli" field
//     (matches what `uitspraken search --agent` and `watch --agent` emit)
//   - one JSON object per line with an "ecli" field (JSONL)
//   - one ECLI per line, optionally followed by whitespace + title
//
// Pretty-printed JSON breaks line-by-line scanners, so the function fully
// buffers stdin and dispatches on the first non-whitespace byte.
func readECLIs(r io.Reader) []string {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil
	}
	trimmed := bytes.TrimSpace(buf)
	if len(trimmed) == 0 {
		return nil
	}
	switch trimmed[0] {
	case '[':
		if eclis, ok := decodeJSONArray(trimmed); ok {
			return eclis
		}
	case '{':
		if eclis, ok := decodeJSONObject(trimmed); ok {
			return eclis
		}
	}
	// Fall through to line-by-line: handles JSONL, ECLI-per-line, and the
	// "ECLI  title" output from `uitspraken search` text mode.
	return decodeLines(buf)
}

func decodeJSONArray(b []byte) ([]string, bool) {
	// Try []string first; fall back to []SearchEntry shape.
	var raw []string
	if err := json.Unmarshal(b, &raw); err == nil {
		return raw, true
	}
	var entries []struct {
		ECLI string `json:"ecli"`
	}
	if err := json.Unmarshal(b, &entries); err == nil {
		out := make([]string, 0, len(entries))
		for _, e := range entries {
			if e.ECLI != "" {
				out = append(out, e.ECLI)
			}
		}
		if len(out) > 0 {
			return out, true
		}
	}
	return nil, false
}

func decodeJSONObject(b []byte) ([]string, bool) {
	// Common shapes:
	//   {"ecli":"ECLI:..."}                         — single decision
	//   {"ecli":["ECLI:...", ...]}                  — narrow's own output
	//   {"entries":[{"ecli":"ECLI:..."}, ...]}      — search / watch envelope
	var probe struct {
		ECLIString string          `json:"-"`
		ECLI       json.RawMessage `json:"ecli"`
		Entries    []struct {
			ECLI string `json:"ecli"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(b, &probe); err != nil {
		return nil, false
	}
	var out []string
	if len(probe.ECLI) > 0 {
		var asString string
		if err := json.Unmarshal(probe.ECLI, &asString); err == nil && asString != "" {
			out = append(out, asString)
		} else {
			var asArr []string
			if err := json.Unmarshal(probe.ECLI, &asArr); err == nil {
				out = append(out, asArr...)
			}
		}
	}
	for _, e := range probe.Entries {
		if e.ECLI != "" {
			out = append(out, e.ECLI)
		}
	}
	if len(out) > 0 {
		return out, true
	}
	return nil, false
}

func decodeLines(buf []byte) []string {
	var out []string
	sc := bufio.NewScanner(bytes.NewReader(buf))
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		// JSONL: one object per line with an "ecli" field.
		if strings.HasPrefix(line, "{") {
			var obj struct {
				ECLI string `json:"ecli"`
			}
			if err := json.Unmarshal([]byte(line), &obj); err == nil && obj.ECLI != "" {
				out = append(out, obj.ECLI)
				continue
			}
		}
		// Tolerate "ECLI title" / "ECLI  Court, date, zaak" output from search
		// command's text mode — first whitespace-delimited token is the ECLI.
		if idx := strings.IndexAny(line, " \t"); idx > 0 && strings.HasPrefix(line, "ECLI:") {
			line = line[:idx]
		}
		out = append(out, line)
	}
	return out
}

func narrowMatch(ctx context.Context, http *rechtspraak.HTTP, ecli string, keywords, excludes, phrases []string, regexes []*regexp.Regexp, summaryOnly, fullOnly bool) (bool, error) {
	if _, err := rechtspraak.ParseECLI(ecli); err != nil {
		return false, err
	}
	d, err := http.Get(ctx, ecli, false)
	if err != nil {
		return false, err
	}
	corpus := matchCorpus(d, summaryOnly, fullOnly)
	low := strings.ToLower(corpus)
	for _, kw := range keywords {
		if !strings.Contains(low, strings.ToLower(kw)) {
			return false, nil
		}
	}
	for _, ex := range excludes {
		if strings.Contains(low, strings.ToLower(ex)) {
			return false, nil
		}
	}
	for _, p := range phrases {
		if !strings.Contains(corpus, p) {
			return false, nil
		}
	}
	for _, re := range regexes {
		if !re.MatchString(corpus) {
			return false, nil
		}
	}
	return true, nil
}

func matchCorpus(d *rechtspraak.Decision, summaryOnly, fullOnly bool) string {
	switch {
	case summaryOnly:
		return d.Summary
	case fullOnly:
		return d.Body
	default:
		return d.Title + "\n" + d.Summary + "\n" + d.Body
	}
}
