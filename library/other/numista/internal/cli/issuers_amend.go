// Copyright 2026 vinny-pasceri. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/numista/internal/store"
)

// PATCH(amend-2026-05-18: issuer-discovery): adds 'issuers find <query>'
// local-only subcommand and the fuzzy-match helper used by types_search_amend
// to suggest the right issuer slug when the API rejects --issuer.
//
// All edits in this file are hand-written, local-only, and never spend a
// quota call — the discovery path must not silently turn a typo into an API
// hit. The cache is populated by 'numista-pp-cli issuers' (or the broader
// sync pipeline); when empty, both surfaces fail with a clear warm-the-cache
// message rather than auto-fetching.

// issuerRecord is a slim view of an issuer JSON object as stored in the
// local resources table. The store key 'name' carries the slug (e.g.
// 'united-states'), and the JSON field 'name' carries the display label
// (e.g. 'United States'). Numista stores both under the same field name
// in the API response; we use the row id (= store-key 'name') as the
// authoritative slug and a separate 'label' from the JSON only when it
// differs (rare, mostly for territorial subdivisions).
type issuerRecord struct {
	Slug   string
	Label  string
	Parent string
}

// loadIssuersFromCache reads every issuer the local store knows about. Returns
// (nil, nil) when the cache is empty so callers can render the warm-the-cache
// message instead of erroring. The 50000 limit is a generous ceiling well
// above Numista's ~12K issuers across all levels (~250 top-level countries +
// subdivisions); see crawl_issuer.go for the full crawl pattern.
func loadIssuersFromCache() ([]issuerRecord, error) {
	dbPath := defaultDBPath("numista-pp-cli")
	// Distinguish "DB file genuinely missing" (cache not warm — return empty
	// so the caller can render the warm-the-cache message) from "DB file
	// exists but unreadable" (permissions, disk error, corruption — must
	// surface so the user doesn't follow the warm-the-cache advice, spend a
	// quota call on `numista-pp-cli issuers`, and loop). Same fix as PR #688
	// applied to loadCataloguesFromCache; flagged there as Greptile P1.
	if _, err := os.Stat(dbPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("checking issuers cache file %s: %w", dbPath, err)
	}
	s, err := store.OpenReadOnly(dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening issuers cache: %w", err)
	}
	defer s.Close()
	// Pull a big batch — Numista returns ~12K issuers across all levels.
	// store.List caps at the limit parameter; pass a generous ceiling.
	raws, err := s.List("issuers", 50000)
	if err != nil {
		return nil, fmt.Errorf("reading issuers cache: %w", err)
	}
	if len(raws) == 0 {
		return nil, nil
	}
	out := make([]issuerRecord, 0, len(raws))
	for _, raw := range raws {
		var obj map[string]any
		if json.Unmarshal(raw, &obj) != nil {
			continue
		}
		slug := stringField(obj, "name")
		// The JSON 'code' field is sometimes present and authoritative for
		// the slug. Prefer it when set.
		if code := stringField(obj, "code"); code != "" {
			slug = code
		}
		if slug == "" {
			continue
		}
		label := stringField(obj, "label")
		if label == "" {
			label = stringField(obj, "title")
		}
		if label == "" {
			label = slug
		}
		parent := stringField(obj, "parent")
		if parent == "" {
			parent = stringField(obj, "parent_code")
		}
		out = append(out, issuerRecord{Slug: slug, Label: label, Parent: parent})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

// (stringField is defined in collection_value.go; reusing it keeps
// the issuer JSON access shape consistent with the rest of the CLI.)

// normalizeIssuerQuery folds the input into the shape we compare against:
// lowercase, hyphens and underscores collapsed to a single space, repeated
// whitespace collapsed. Matches how a user thinks about the query ("united
// states", "united_states", "United-States" all become "united states").
func normalizeIssuerQuery(q string) string {
	q = strings.ToLower(strings.TrimSpace(q))
	q = strings.ReplaceAll(q, "-", " ")
	q = strings.ReplaceAll(q, "_", " ")
	return strings.Join(strings.Fields(q), " ")
}

// issuerSynonyms maps common short forms to their canonical slugs. This is
// intentionally short and curated — it covers the cases AGENTS.md called out
// explicitly (usa, uk) and a handful of obvious sibling abbreviations. We
// keep this list hand-tuned rather than auto-discovered so a user typing
// "usa" gets a deterministic top hit instead of fuzzy-match drift.
var issuerSynonyms = map[string]string{
	"usa": "united-states",
	"us":  "united-states",
	"uk":  "united-kingdom",
	"gb":  "united-kingdom",
	"uae": "united-arab-emirates",
	"rsa": "south-africa",
	"prc": "china",
	"roc": "taiwan",
	"ussr": "soviet-union",
	"ddr": "germany-democratic-republic",
}

// scoreIssuer ranks a record against the normalized query. Higher is better.
// Scoring rules, in priority order:
//
//  1. Synonym table direct hit: max score (10000) — locks "usa" to united-states.
//  2. Exact slug match: 5000.
//  3. Exact label match: 4500.
//  4. Slug prefix match: 3000 - len(slug) (shorter slugs win on ties).
//  5. Label word prefix on any token: 2000.
//  6. Substring in either slug or label: 1000 + length-of-match bonus.
//  7. No match: 0.
//
// We intentionally avoid Levenshtein here — it makes "morgan" match
// "moravia" and the user said the discovery surface should be predictable.
func scoreIssuer(rec issuerRecord, qNorm string) int {
	if qNorm == "" {
		return 0
	}
	if syn, ok := issuerSynonyms[qNorm]; ok && syn == rec.Slug {
		return 10000
	}
	slugNorm := normalizeIssuerQuery(rec.Slug)
	labelNorm := normalizeIssuerQuery(rec.Label)
	if slugNorm == qNorm {
		return 5000
	}
	if labelNorm == qNorm {
		return 4500
	}
	if strings.HasPrefix(slugNorm, qNorm) {
		return 3000 - len(rec.Slug)
	}
	// Label word-prefix: any token starts with the query
	for _, tok := range strings.Fields(labelNorm) {
		if strings.HasPrefix(tok, qNorm) {
			return 2000 - len(rec.Label)
		}
	}
	if strings.Contains(slugNorm, qNorm) || strings.Contains(labelNorm, qNorm) {
		// Longer matches against shorter labels score higher. Floor at 1 so
		// a pathological-length label never collapses a valid substring
		// match to 0 (which fuzzyFindIssuers filters out). Same fix as PR
		// #688 applied to scoreCatalogue; flagged there as Greptile P2.
		s := 1000 + len(qNorm) - len(rec.Label)/10
		if s < 1 {
			s = 1
		}
		return s
	}
	return 0
}

// fuzzyFindIssuers returns the top n records ranked by scoreIssuer. The
// query is normalized once; ties break on alphabetical slug order so
// repeated calls return stable rankings (important for tests and for
// agent prompts that depend on the top hit).
func fuzzyFindIssuers(records []issuerRecord, query string, n int) []issuerRecord {
	if n <= 0 {
		n = 5
	}
	qNorm := normalizeIssuerQuery(query)
	if qNorm == "" {
		return nil
	}
	type scored struct {
		rec issuerRecord
		s   int
	}
	pool := make([]scored, 0, len(records))
	for _, rec := range records {
		if s := scoreIssuer(rec, qNorm); s > 0 {
			pool = append(pool, scored{rec: rec, s: s})
		}
	}
	sort.SliceStable(pool, func(i, j int) bool {
		if pool[i].s != pool[j].s {
			return pool[i].s > pool[j].s
		}
		return pool[i].rec.Slug < pool[j].rec.Slug
	})
	if len(pool) > n {
		pool = pool[:n]
	}
	out := make([]issuerRecord, len(pool))
	for i, p := range pool {
		out[i] = p.rec
	}
	return out
}

// errIssuersCacheEmpty is returned by callers that want to distinguish
// "no cache rows" from "no matches" — the warm-the-cache hint is the right
// remedy for the former and "try a different query" is the right remedy
// for the latter.
var errIssuersCacheEmpty = errors.New("issuers cache is empty")

// suggestIssuerSlugs is the entry point used by types_search_amend's error
// wrapper. Returns up to 3 suggestions for the user-supplied --issuer value;
// returns errIssuersCacheEmpty so the caller can render the warm-the-cache
// hint instead of a generic "no matches" string. The error never surfaces
// directly to the user — types_search_amend folds it into a single combined
// message.
func suggestIssuerSlugs(userInput string) ([]issuerRecord, error) {
	records, err := loadIssuersFromCache()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, errIssuersCacheEmpty
	}
	return fuzzyFindIssuers(records, userInput, 3), nil
}

// newIssuersFindCmd returns the local-only fuzzy-match command.
//
// Output discipline mirrors the rest of the CLI: machine modes get JSON,
// terminal mode gets a two-column table. The hint about the warm step is
// printed to STDERR only — it never pollutes the JSON payload, so an agent
// piping --json | jq still gets a valid (possibly empty) array on stdout.
func newIssuersFindCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "find <query>",
		Short: "Fuzzy-match an issuer name or short form against the local cache (no API call).",
		Long: "Find an issuer slug (the value you pass to --issuer on 'types search')\n" +
			"by fuzzy-matching a name, short form, or partial slug against the local cache.\n" +
			"\n" +
			"Local only. Never spends a quota call. Run 'numista-pp-cli issuers' first to\n" +
			"populate the cache if it is empty.\n" +
			"\n" +
			"Examples:\n" +
			"  numista-pp-cli issuers find 'united states'\n" +
			"  numista-pp-cli issuers find usa\n" +
			"  numista-pp-cli issuers find 'south africa' --limit 10\n",
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			records, err := loadIssuersFromCache()
			if err != nil {
				return apiErr(err)
			}
			if len(records) == 0 {
				// apiErr (exit 5) rather than usageErr (exit 2): empty cache
				// is a precondition-not-met / data-state error, not a wrong-
				// flag error — usageErr is conventionally reserved for the
				// latter. Greptile P2 on PR #684 review.
				return apiErr(fmt.Errorf(
					"local issuers cache is empty. Run 'numista-pp-cli issuers' to populate it (one API call), " +
						"then re-run 'issuers find'"))
			}
			matches := fuzzyFindIssuers(records, query, limit)
			return renderIssuersFind(cmd, flags, matches)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum number of matches to return")
	return cmd
}

// renderIssuersFind picks the output shape from rootFlags. JSON when any
// machine-format flag is set OR stdout isn't a TTY; otherwise a compact
// human table. Matches the gating used by every other read command in
// this CLI.
func renderIssuersFind(cmd *cobra.Command, flags *rootFlags, matches []issuerRecord) error {
	out := cmd.OutOrStdout()
	// --quiet: suppress all output, exit code communicates result. Mirrors the
	// contract in printOutputWithFlags. Must guard before the CSV / auto-JSON /
	// human-table branches below, since each emits to stdout unconditionally
	// and --json --quiet would otherwise still print via the flags.asJSON gate.
	if flags.quiet {
		return nil
	}
	if flags.csv {
		w := csv.NewWriter(out)
		_ = w.Write([]string{"slug", "label", "parent"})
		for _, m := range matches {
			_ = w.Write([]string{m.Slug, m.Label, m.Parent})
		}
		w.Flush()
		return w.Error()
	}
	// Route JSON through the standard output pipeline so --select and
	// --compact behave identically to every other read command. Previously
	// wrote the envelope directly via json.NewEncoder, which silently
	// bypassed filterFields / compactFields — Greptile P2 on PR #684
	// review.
	if flags.asJSON || (!isTerminal(out) && !flags.csv && !flags.quiet && !flags.plain) {
		items := make([]map[string]any, len(matches))
		for i, m := range matches {
			items[i] = map[string]any{"slug": m.Slug, "label": m.Label, "parent": m.Parent}
		}
		envelope := map[string]any{
			"meta":    map[string]any{"source": "local", "resource_type": "issuers"},
			"results": items,
		}
		data, err := json.Marshal(envelope)
		if err != nil {
			return fmt.Errorf("marshal issuers find envelope: %w", err)
		}
		if flags.selectFields != "" {
			data = filterFields(data, flags.selectFields)
		} else if flags.compact {
			data = compactFields(data)
		}
		return printOutput(out, data, true)
	}
	// Human table
	if len(matches) == 0 {
		// Use cmd.ErrOrStderr() not os.Stderr — every other diagnostic in
		// this CLI goes through the command's error writer, and Cobra-based
		// tests can only capture output that flows through it. Same fix as
		// PR #688 applied to catalogues find; flagged there as Greptile P2.
		fmt.Fprintln(cmd.ErrOrStderr(), "no matches.")
		return nil
	}
	tw := newTabWriter(out)
	fmt.Fprintln(tw, "SLUG\tLABEL\tPARENT")
	for _, m := range matches {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", m.Slug, m.Label, m.Parent)
	}
	return tw.Flush()
}

// attachIssuerAmendments wires the hand-written 'find' subcommand onto the
// generated 'issuers' parent. Called from root.go AFTER
// newIssuersPromotedCmd has been added to rootCmd. Walking the command
// tree (rather than editing promoted_issuers.go directly) keeps the
// generator's DO NOT EDIT contract intact.
func attachIssuerAmendments(rootCmd *cobra.Command, flags *rootFlags) {
	for _, c := range rootCmd.Commands() {
		if c.Name() == "issuers" {
			c.AddCommand(newIssuersFindCmd(flags))
			return
		}
	}
}
