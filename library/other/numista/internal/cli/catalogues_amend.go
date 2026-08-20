// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

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

// PATCH(amend-2026-05-18: catalogue-discovery): adds 'catalogues find <query>'
// local-only subcommand for fuzzy-matching a Numista reference catalogue against
// the local cache. Complements 'issuers find' (PR #684) and closes
// pp-numista/AGENTS.md Priority 4 — agents starting from a PCGS cert can verify
// that catalogue id=1856 is PCGS without spending a quota call or scrolling the
// 3,106-row 'catalogues' dump.
//
// All edits in this file are hand-written, local-only, and never spend a quota
// call. The cache is populated by 'numista-pp-cli catalogues' (one API call);
// when empty, 'find' fails with a clear warm-the-cache message rather than
// auto-fetching, matching the discipline established by 'issuers find'.

// catalogueRecord is a slim view of a catalogue JSON object as stored in the
// local resources table. Numista's /catalogues response shape is
// {id (int), code, title, author, publisher, isbn13}; we project the four
// fields the find surface uses and drop the rest. The 'extra' field holds
// the publisher-or-author label rendered next to title in the human table.
type catalogueRecord struct {
	ID        int
	Code      string
	Title     string
	Author    string
	Publisher string
}

// extra returns the secondary identifier shown alongside title in the human
// table. Author wins when present (most catalogues are author-named);
// publisher is the fallback for institutional references like PCGS where
// "author" is empty.
func (r catalogueRecord) extra() string {
	if r.Author != "" {
		return r.Author
	}
	return r.Publisher
}

// loadCataloguesFromCache reads every catalogue the local store knows about.
// Returns (nil, nil) when the cache is empty so callers can render the
// warm-the-cache message instead of erroring. Numista has ~3,100 catalogues;
// the 5000 limit covers the full set with headroom.
func loadCataloguesFromCache() ([]catalogueRecord, error) {
	dbPath := defaultDBPath("numista-pp-cli")
	// Distinguish "DB file genuinely missing" (cache not warm — empty result
	// is the right answer; caller renders the warm-the-cache message) from
	// "DB file exists but unreadable" (permissions, disk error, corruption —
	// must surface so the user doesn't follow the warm-the-cache advice,
	// spend a quota call on `numista-pp-cli catalogues`, and loop). Greptile
	// P1 on PR #688 review.
	if _, err := os.Stat(dbPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("checking catalogues cache file %s: %w", dbPath, err)
	}
	s, err := store.OpenReadOnly(dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening catalogues cache: %w", err)
	}
	defer s.Close()
	raws, err := s.List("catalogues", 5000)
	if err != nil {
		return nil, fmt.Errorf("reading catalogues cache: %w", err)
	}
	if len(raws) == 0 {
		return nil, nil
	}
	out := make([]catalogueRecord, 0, len(raws))
	for _, raw := range raws {
		var obj map[string]any
		if json.Unmarshal(raw, &obj) != nil {
			continue
		}
		// id arrives as float64 from the generic JSON decoder; cast back to
		// int. Skip rows missing an id — the find surface needs it for the
		// definitive 'numista-pp-cli types search --catalogue <id>' lookup
		// the cookbook recommends.
		id, ok := intFromAny(obj["id"])
		if !ok {
			continue
		}
		code := stringField(obj, "code")
		title := stringField(obj, "title")
		if code == "" && title == "" {
			continue
		}
		out = append(out, catalogueRecord{
			ID:        id,
			Code:      code,
			Title:     title,
			Author:    stringField(obj, "author"),
			Publisher: stringField(obj, "publisher"),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Code != out[j].Code {
			return out[i].Code < out[j].Code
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// intFromAny coerces a JSON-decoded value into an int. Numeric JSON values
// decode as float64 through encoding/json's default map[string]any path, so
// the generic int conversion has to handle both float64 (the common case)
// and json.Number (set if a future caller installs a Decoder with UseNumber).
func intFromAny(v any) (int, bool) {
	switch x := v.(type) {
	case float64:
		return int(x), true
	case int:
		return x, true
	case int64:
		return int(x), true
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return int(i), true
		}
	case string:
		// Some upstream rows may store numeric ids as strings during
		// migration; tolerate it and fall back to atoi.
		var n int
		if _, err := fmt.Sscanf(x, "%d", &n); err == nil {
			return n, true
		}
	}
	return 0, false
}

// normalizeCatalogueQuery folds the input into the shape we compare against.
// Lowercased, diacritics stripped (so 'schon' matches 'Schön'), hyphens and
// underscores collapsed to spaces, repeated whitespace collapsed.
//
// Diacritic folding is targeted at the Latin-script catalogue corpus —
// German umlauts, French accents, and Eastern-European combining marks
// that show up in author and publisher names (Schön, Krämer, Müller).
// We do it manually rather than pulling golang.org/x/text/unicode/norm
// because the cost of one new module dep for a discovery surface this
// small is not worth it, and the explicit table is auditable.
func normalizeCatalogueQuery(q string) string {
	q = strings.ToLower(strings.TrimSpace(q))
	q = foldDiacritics(q)
	q = strings.ReplaceAll(q, "-", " ")
	q = strings.ReplaceAll(q, "_", " ")
	return strings.Join(strings.Fields(q), " ")
}

// foldDiacritics rewrites the common Latin-script accented letters to their
// unaccented ASCII equivalents. The table is intentionally compact and
// covers the diacritics that actually appear in the Numista catalogue
// corpus (German, French, Spanish, Scandinavian, Eastern European). A user
// who types 'schon' should still hit 'Schön'; a user who types 'kramer'
// should still hit 'Krämer'.
func foldDiacritics(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		// German / Scandinavian
		case 'ä', 'á', 'à', 'â', 'ã', 'å', 'ā', 'ă', 'ą':
			b.WriteRune('a')
		case 'ë', 'é', 'è', 'ê', 'ē', 'ĕ', 'ė', 'ę':
			b.WriteRune('e')
		case 'ï', 'í', 'ì', 'î', 'ī', 'į':
			b.WriteRune('i')
		case 'ö', 'ó', 'ò', 'ô', 'õ', 'ø', 'ō', 'ŏ', 'ő':
			b.WriteRune('o')
		case 'ü', 'ú', 'ù', 'û', 'ū', 'ŭ', 'ů', 'ű':
			b.WriteRune('u')
		case 'ý', 'ÿ':
			b.WriteRune('y')
		case 'ñ', 'ń', 'ň':
			b.WriteRune('n')
		case 'ç', 'ć', 'č':
			b.WriteRune('c')
		case 'ß':
			b.WriteString("ss")
		case 'ł':
			b.WriteRune('l')
		case 'š', 'ś':
			b.WriteRune('s')
		case 'ž', 'ź', 'ż':
			b.WriteRune('z')
		case 'ř':
			b.WriteRune('r')
		case 'ť':
			b.WriteRune('t')
		case 'ď':
			b.WriteRune('d')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// scoreCatalogue ranks a record against the normalized query. Higher is
// better. Scoring rules in priority order:
//
//  1. Exact code match (case-insensitive): 5000 — locks 'find KM' to id=3.
//  2. Exact title match: 4500.
//  3. Code prefix match: 3000 - len(code) (shorter codes win on ties).
//  4. Title word-prefix on any token: 2000 - len(title).
//  5. Substring in code OR title OR author OR publisher: 1000 + len(query)
//     - len(title)/10 (longer titles score slightly lower to bias toward
//     concise canonical references).
//  6. No match: 0.
//
// Author/publisher matches at tier 5 are how 'find krause' surfaces KM
// (publisher=Krause Publications) and 'find yeoman' surfaces Y (author=
// Richard Sperry Yeoman). Tier 5 is intentionally last so a query that
// matches a canonical code or title still wins over a query that only
// matches deep metadata.
func scoreCatalogue(rec catalogueRecord, qNorm string) int {
	if qNorm == "" {
		return 0
	}
	codeNorm := normalizeCatalogueQuery(rec.Code)
	titleNorm := normalizeCatalogueQuery(rec.Title)
	authorNorm := normalizeCatalogueQuery(rec.Author)
	publisherNorm := normalizeCatalogueQuery(rec.Publisher)

	if codeNorm == qNorm {
		return 5000
	}
	if titleNorm == qNorm {
		return 4500
	}
	if strings.HasPrefix(codeNorm, qNorm) {
		return 3000 - len(rec.Code)
	}
	for _, tok := range strings.Fields(titleNorm) {
		if strings.HasPrefix(tok, qNorm) {
			return 2000 - len(rec.Title)
		}
	}
	if strings.Contains(codeNorm, qNorm) ||
		strings.Contains(titleNorm, qNorm) ||
		strings.Contains(authorNorm, qNorm) ||
		strings.Contains(publisherNorm, qNorm) {
		// Floor at 1 so a pathological-length title never collapses a valid
		// substring match to 0 (which fuzzyFindCatalogues filters out).
		// Numista titles are short in practice but the formula needs a
		// guarantee. Greptile P2 on PR #688 review.
		s := 1000 + len(qNorm) - len(rec.Title)/10
		if s < 1 {
			s = 1
		}
		return s
	}
	return 0
}

// fuzzyFindCatalogues returns the top n records ranked by scoreCatalogue.
// Query normalized once; ties break on alphabetical code (then id) so
// repeated calls return stable rankings — important for tests and for
// agent prompts that depend on the top hit.
func fuzzyFindCatalogues(records []catalogueRecord, query string, n int) []catalogueRecord {
	if n <= 0 {
		n = 5
	}
	qNorm := normalizeCatalogueQuery(query)
	if qNorm == "" {
		return nil
	}
	type scored struct {
		rec catalogueRecord
		s   int
	}
	pool := make([]scored, 0, len(records))
	for _, rec := range records {
		if s := scoreCatalogue(rec, qNorm); s > 0 {
			pool = append(pool, scored{rec: rec, s: s})
		}
	}
	// Tiebreak ordering, applied in this priority when scores collide:
	//   1. Lower numista id wins — Numista assigns ids in roughly canonical
	//      order (id=3 is KM, id=9 is Y, id=24 is Schön). For a query like
	//      'krause' where KM (id=3) and Baker (id=2375) both score in the
	//      publisher-substring tier, the canonical world-coin reference is
	//      what the agent wants.
	//   2. Alphabetical by code, as a stable fallback for deterministic
	//      tests when ids are unavailable (shouldn't happen in practice).
	sort.SliceStable(pool, func(i, j int) bool {
		if pool[i].s != pool[j].s {
			return pool[i].s > pool[j].s
		}
		if pool[i].rec.ID != pool[j].rec.ID {
			return pool[i].rec.ID < pool[j].rec.ID
		}
		return pool[i].rec.Code < pool[j].rec.Code
	})
	if len(pool) > n {
		pool = pool[:n]
	}
	out := make([]catalogueRecord, len(pool))
	for i, p := range pool {
		out[i] = p.rec
	}
	return out
}

// newCataloguesFindCmd returns the local-only fuzzy-match command.
//
// Output discipline mirrors 'issuers find': machine modes get JSON, terminal
// mode gets a column-aligned table, and the warm-the-cache hint never lands
// on stdout in machine modes so 'find pcgs --json | jq' stays clean for
// agents.
func newCataloguesFindCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "find <query>",
		Short: "Fuzzy-match a reference catalogue by code, title, author, or publisher (no API call).",
		Long: "Find a Numista reference catalogue id by fuzzy-matching against the local cache.\n" +
			"Returns id, code, title, and author-or-publisher for the top matches.\n" +
			"\n" +
			"Local only. Never spends a quota call. Run 'numista-pp-cli catalogues' first\n" +
			"to populate the cache (one API call) if it is empty.\n" +
			"\n" +
			"Use the returned id with 'types search --catalogue <id> --number <ref>' to do\n" +
			"a direct cross-walk from a third-party catalogue reference (e.g. PCGSNo, KM#) to\n" +
			"a Numista N#.\n" +
			"\n" +
			"Examples:\n" +
			"  numista-pp-cli catalogues find pcgs            # → id=1856 (PCGS CoinFacts)\n" +
			"  numista-pp-cli catalogues find krause          # → id=3 (KM, Standard Catalog of World Coins)\n" +
			"  numista-pp-cli catalogues find yeoman --limit 10\n",
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			records, err := loadCataloguesFromCache()
			if err != nil {
				return apiErr(err)
			}
			if len(records) == 0 {
				// apiErr (exit 5) rather than usageErr (exit 2): empty cache
				// is a precondition-not-met / data-state error, not a wrong-
				// flag error — usageErr is conventionally reserved for the
				// latter. Same fix applied to issuers find in PR #684 after
				// Greptile P2 review.
				return apiErr(fmt.Errorf(
					"local catalogues cache is empty. Run 'numista-pp-cli catalogues' to populate it (one API call), " +
						"then re-run 'catalogues find'"))
			}
			matches := fuzzyFindCatalogues(records, query, limit)
			return renderCataloguesFind(cmd, flags, matches)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum number of matches to return")
	return cmd
}

// renderCataloguesFind picks the output shape from rootFlags. JSON when any
// machine-format flag is set OR stdout isn't a TTY; otherwise a compact
// human table. Matches the gating used by 'issuers find' and every other
// read command in this CLI.
func renderCataloguesFind(cmd *cobra.Command, flags *rootFlags, matches []catalogueRecord) error {
	out := cmd.OutOrStdout()
	if flags.csv {
		w := csv.NewWriter(out)
		_ = w.Write([]string{"id", "code", "title", "author", "publisher"})
		for _, m := range matches {
			_ = w.Write([]string{fmt.Sprintf("%d", m.ID), m.Code, m.Title, m.Author, m.Publisher})
		}
		w.Flush()
		return w.Error()
	}
	// Route JSON through the standard output pipeline so --select and
	// --compact behave identically to every other read command. Previously
	// wrote the envelope directly via json.NewEncoder, which silently
	// bypassed filterFields / compactFields — same fix applied to issuers
	// find in PR #684 after Greptile P2 review.
	if flags.asJSON || (!isTerminal(out) && !flags.csv && !flags.quiet && !flags.plain) {
		items := make([]map[string]any, len(matches))
		for i, m := range matches {
			items[i] = map[string]any{
				"id":        m.ID,
				"code":      m.Code,
				"title":     m.Title,
				"author":    m.Author,
				"publisher": m.Publisher,
			}
		}
		envelope := map[string]any{
			"meta":    map[string]any{"source": "local", "resource_type": "catalogues"},
			"results": items,
		}
		data, err := json.Marshal(envelope)
		if err != nil {
			return fmt.Errorf("marshal catalogues find envelope: %w", err)
		}
		if flags.selectFields != "" {
			data = filterFields(data, flags.selectFields)
		} else if flags.compact {
			data = compactFields(data)
		}
		return printOutput(out, data, true)
	}
	if len(matches) == 0 {
		// Use cmd.ErrOrStderr() not os.Stderr — every other diagnostic in
		// this CLI goes through the command's error writer, and Cobra-based
		// tests can only capture output that flows through it. Greptile P2
		// on PR #688 review.
		fmt.Fprintln(cmd.ErrOrStderr(), "no matches.")
		return nil
	}
	tw := newTabWriter(out)
	fmt.Fprintln(tw, "ID\tCODE\tTITLE\tAUTHOR/PUBLISHER")
	for _, m := range matches {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", m.ID, m.Code, m.Title, m.extra())
	}
	return tw.Flush()
}

// attachCatalogueAmendments wires the hand-written 'find' subcommand onto
// the generated 'catalogues' parent. Called from root.go AFTER
// newCataloguesPromotedCmd has been added to rootCmd. Walking the command
// tree (rather than editing promoted_catalogues.go directly) keeps the
// generator's DO NOT EDIT contract intact and preserves the existing
// zero-arg 'catalogues' invocation as a shortcut for 'catalogues get'.
func attachCatalogueAmendments(rootCmd *cobra.Command, flags *rootFlags) {
	for _, c := range rootCmd.Commands() {
		if c.Name() == "catalogues" {
			c.AddCommand(newCataloguesFindCmd(flags))
			return
		}
	}
}
