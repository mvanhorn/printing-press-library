// Commands: score, gen (suggest/mix/affix/blend/hack/rhyme/permute), similar, socials
package cli

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/domain-goat/internal/gen"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/domain-goat/internal/scoring"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/domain-goat/internal/source/rdap"
)

func newScoreCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "score <domain...>",
		Short: "Score a domain on brandability (length, syllables, dictionary, TLD prestige).",
		Example: `  domain-goat-pp-cli score kindred.io
  domain-goat-pp-cli score kindred.io lumen.ai novella.studio --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "fqdns": args})
			}
			fqdns, err := normalizeAll(args)
			if err != nil {
				return usageErr(err)
			}
			scores := make([]scoring.Score, 0, len(fqdns))
			for _, f := range fqdns {
				scores = append(scores, scoring.Compute(f))
			}
			if wantJSON(cmd, flags) {
				return emitJSON(cmd, flags, scores)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "DOMAIN\tTOTAL\tLEN\tSYL\tDICT\tHACK\tPRESTIGE")
			for _, s := range scores {
				fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%v\t%v\t%d\n", s.FQDN, s.Total, s.Length, s.Syllables, s.DictWord, s.HackStyle, s.TLDPrestige)
			}
			return tw.Flush()
		},
	}
	return cmd
}

func newGenCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gen",
		Short: "Offline domain-name generators (suggest, mix, affix, blend, hack, rhyme).",
		Long:  "Composable generators that produce candidate names from seed words.\nUse `gen suggest` for the kitchen-sink combo or pick a specific generator.",
	}
	cmd.AddCommand(newGenSuggestCmd(flags))
	cmd.AddCommand(newGenAffixCmd(flags))
	cmd.AddCommand(newGenBlendCmd(flags))
	cmd.AddCommand(newGenMixCmd(flags))
	cmd.AddCommand(newGenHackCmd(flags))
	cmd.AddCommand(newGenRhymeCmd(flags))
	return cmd
}

func newGenSuggestCmd(flags *rootFlags) *cobra.Command {
	var seeds, seedsFile, tldsCSV string
	var count int
	var availableOnly bool
	var includeScore bool
	var maxRenewal float64
	cmd := &cobra.Command{
		Use:   "suggest",
		Short: "All generators combined: affix + blend + mix + hack + rhyme.",
		Example: `  domain-goat-pp-cli gen suggest --seeds kindred,studio --tlds com,io,ai --count 50
  domain-goat-pp-cli gen suggest --seeds brand --available-only --count 30 --json
  domain-goat-pp-cli gen suggest --seeds-file seeds.txt --tlds com,io --max-renewal 50`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			seedList := splitCSV(seeds)
			if seedsFile != "" {
				more, err := readNamesFile(seedsFile)
				if err == nil {
					seedList = append(seedList, more...)
				}
			}
			if len(seedList) == 0 {
				return usageErr(fmt.Errorf("--seeds or --seeds-file required"))
			}
			tlds := joinTLDs(tldsCSV)
			if len(tlds) == 0 {
				tlds = []string{"com", "io", "ai", "app", "co", "dev"}
			}
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "seeds": seedList, "tlds": tlds, "count": count})
			}
			labels := map[string]struct{}{}
			for _, s := range seedList {
				labels[s] = struct{}{}
				for _, v := range gen.Affix(s, nil, nil) {
					labels[v] = struct{}{}
				}
				for _, v := range gen.Rhyme(s) {
					labels[v] = struct{}{}
				}
			}
			if len(seedList) >= 2 {
				for i := 0; i < len(seedList); i++ {
					for j := i + 1; j < len(seedList); j++ {
						for _, v := range gen.Blend(seedList[i], seedList[j]) {
							labels[v] = struct{}{}
						}
					}
				}
				for _, v := range gen.Mix(seedList) {
					labels[v] = struct{}{}
				}
			}
			// hack on each seed
			for _, s := range seedList {
				for _, v := range gen.Hack(s) {
					labels[v] = struct{}{}
				}
			}
			labelList := make([]string, 0, len(labels))
			for l := range labels {
				if !strings.Contains(l, ".") {
					labelList = append(labelList, l)
				}
			}
			fqdns := []string{}
			for _, l := range labelList {
				for _, t := range tlds {
					fqdns = append(fqdns, l+"."+t)
				}
			}
			// include hack-style FQDNs directly
			for l := range labels {
				if strings.Contains(l, ".") {
					fqdns = append(fqdns, l)
				}
			}
			// dedupe
			sort.Strings(fqdns)
			fqdns = dedupeStrings(fqdns)

			type SuggestRow struct {
				FQDN      string         `json:"fqdn"`
				Score     *scoring.Score `json:"score,omitempty"`
				Available *bool          `json:"available,omitempty"`
			}
			rows := make([]SuggestRow, 0, len(fqdns))
			for _, f := range fqdns {
				row := SuggestRow{FQDN: f}
				if includeScore {
					sc := scoring.Compute(f)
					row.Score = &sc
				}
				rows = append(rows, row)
			}
			// PATCH(gen-suggest-score-sort-before-filter): sort by score BEFORE the --available-only RDAP loop so the count early-exit returns the top-N highest-scoring available domains, not the first-N alphabetically. gen.Generate emits candidates alphabetically; the prior order biased --include-score=true users to first-N-alphabetical rather than top-N-by-score.
			// Sort by score BEFORE the availability filter so that the
			// availableOnly loop's `count` early-exit returns the top N
			// highest-scoring available domains rather than the first N
			// alphabetical ones (gen.Generate emits its candidates in
			// alphabetical FQDN order).
			if includeScore {
				sort.Slice(rows, func(i, j int) bool {
					return rows[i].Score.Total > rows[j].Score.Total
				})
			}
			// availability filter (optional, slow — only when explicitly requested)
			if availableOnly {
				ctx, cancel := context.WithTimeout(cmd.Context(), 90*time.Second)
				defer cancel()
				filtered := make([]SuggestRow, 0, len(rows))
				for _, r := range rows {
					if len(filtered) >= count && count > 0 {
						break
					}
					res, err := rdap.Lookup(ctx, r.FQDN)
					if err != nil || res == nil {
						continue
					}
					if !res.Available {
						continue
					}
					avail := true
					r.Available = &avail
					filtered = append(filtered, r)
				}
				rows = filtered
			}
			if maxRenewal > 0 {
				// PATCH(gen-suggest-max-renewal-strict): return an error when openStore fails instead of silently dropping the filter. The previous `if err == nil` wrapper returned every candidate unfiltered when the store couldn't open — users got results that violated their --max-renewal ceiling.
				// If the store can't open we MUST return — silently skipping
				// the filter would hand the user every candidate including the
				// ones whose Porkbun renewal exceeds their stated budget. The
				// user passed --max-renewal as a hard ceiling; we honour it or
				// we error out, but we never silently relax it.
				s, err := openStore(cmd.Context())
				if err != nil {
					return apiErr(fmt.Errorf("--max-renewal requires the local store: %w", err))
				}
				defer s.Close()
				filtered := make([]SuggestRow, 0, len(rows))
				for _, r := range rows {
					p, _ := s.PricingForFQDN(cmd.Context(), r.FQDN)
					if p == nil || p.Renewal == 0 {
						continue
					}
					if p.Renewal > maxRenewal {
						continue
					}
					filtered = append(filtered, r)
				}
				rows = filtered
			}
			if count > 0 && len(rows) > count {
				rows = rows[:count]
			}
			if wantJSON(cmd, flags) {
				return emitJSON(cmd, flags, rows)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			if includeScore {
				fmt.Fprintln(tw, "DOMAIN\tSCORE")
				for _, r := range rows {
					s := 0
					if r.Score != nil {
						s = r.Score.Total
					}
					fmt.Fprintf(tw, "%s\t%d\n", r.FQDN, s)
				}
			} else {
				fmt.Fprintln(tw, "DOMAIN")
				for _, r := range rows {
					fmt.Fprintln(tw, r.FQDN)
				}
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&seeds, "seeds", "", "Comma-separated seed words")
	cmd.Flags().StringVar(&seedsFile, "seeds-file", "", "Read additional seeds from file (one per line, or '-' for stdin)")
	cmd.Flags().StringVar(&tldsCSV, "tlds", "com,io,ai,app,co,dev", "Comma-separated TLDs to combine with each label")
	cmd.Flags().IntVar(&count, "count", 50, "Max number of candidates to emit (0 = no limit)")
	cmd.Flags().BoolVar(&availableOnly, "available-only", false, "Only emit FQDNs that pass RDAP availability check (slower)")
	cmd.Flags().BoolVar(&includeScore, "include-score", true, "Attach brandability score, sort by score")
	cmd.Flags().Float64Var(&maxRenewal, "max-renewal", 0, "Filter out FQDNs whose Porkbun renewal price exceeds this (requires `pricing sync` first)")
	return cmd
}

func newGenAffixCmd(flags *rootFlags) *cobra.Command {
	var seed, prefixCSV, suffixCSV, tldsCSV string
	cmd := &cobra.Command{
		Use:         "affix",
		Short:       "Prefix/suffix combos around one seed.",
		Example:     `  domain-goat-pp-cli gen affix --seed brand --prefixes get,my,go --suffixes -hq,-ly,-app --tlds io,ai`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if seed == "" {
				return usageErr(fmt.Errorf("--seed required"))
			}
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "seed": seed})
			}
			labels := gen.Affix(seed, splitCSV(prefixCSV), splitCSV(suffixCSV))
			tlds := joinTLDs(tldsCSV)
			if len(tlds) == 0 {
				return emitJSON(cmd, flags, labels)
			}
			fqdns := gen.Pair(labels, tlds)
			return emitJSON(cmd, flags, fqdns)
		},
	}
	cmd.Flags().StringVar(&seed, "seed", "", "Base word")
	cmd.Flags().StringVar(&prefixCSV, "prefixes", "", "Comma-separated prefixes (default: get,my,go,use,try,join,the,with)")
	cmd.Flags().StringVar(&suffixCSV, "suffixes", "", "Comma-separated suffixes (default: hq,ly,io,app,hub,labs,studio,kit,now,co)")
	cmd.Flags().StringVar(&tldsCSV, "tlds", "", "Optional TLDs to combine with each label")
	return cmd
}

func newGenBlendCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "blend <a> <b>",
		Short:       "Portmanteau-style blends of two seed words.",
		Example:     `  domain-goat-pp-cli gen blend snap apple`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "a": args[0], "b": args[1]})
			}
			return emitJSON(cmd, flags, gen.Blend(args[0], args[1]))
		},
	}
	return cmd
}

func newGenMixCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "mix <seed...>",
		Short:       "Combine multiple seeds with internal joins.",
		Example:     `  domain-goat-pp-cli gen mix kindred studio voice`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "seeds": args})
			}
			return emitJSON(cmd, flags, gen.Mix(args))
		},
	}
	return cmd
}

func newGenHackCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "hack <word>",
		Short:       "Split a word into hack-style domains (kub.es, del.icio.us).",
		Example:     `  domain-goat-pp-cli gen hack delicious`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "word": args[0]})
			}
			return emitJSON(cmd, flags, gen.Hack(args[0]))
		},
	}
	return cmd
}

func newGenRhymeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "rhyme <word>",
		Short:       "Generate rhyming variants by swapping the leading consonant cluster.",
		Example:     `  domain-goat-pp-cli gen rhyme brand`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "word": args[0]})
			}
			return emitJSON(cmd, flags, gen.Rhyme(args[0]))
		},
	}
	return cmd
}

func newSimilarCmd(flags *rootFlags) *cobra.Command {
	var typesCSV string
	var limit int
	cmd := &cobra.Command{
		Use:   "similar <fqdn>",
		Short: "Generate typosquat / similar-name variations (dnstwist-style).",
		Long: `Apply one or more permutation algorithms to a base FQDN:
omission, insertion, replacement, transposition, repetition, vowel-swap,
hyphenation, addition, tld-swap, homoglyph, bitsquatting, subdomain.`,
		Example: `  domain-goat-pp-cli similar kindred.io
  domain-goat-pp-cli similar kindred.io --types vowel-swap,tld-swap --json
  domain-goat-pp-cli similar example.com --types homoglyph,bitsquatting --limit 50`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "fqdn": args[0]})
			}
			// Permutation works on any string; skip strict TLD validation so
			// non-canonical inputs (e.g., bare hostnames, internal labels)
			// still produce useful typo variants.
			base := strings.ToLower(strings.TrimSpace(args[0]))
			perms := gen.Permute(base, splitCSV(typesCSV))
			if limit > 0 && len(perms) > limit {
				perms = perms[:limit]
			}
			if wantJSON(cmd, flags) {
				return emitJSON(cmd, flags, perms)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "DOMAIN\tKIND")
			for _, p := range perms {
				fmt.Fprintf(tw, "%s\t%s\n", p.FQDN, p.Kind)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&typesCSV, "types", "", "Comma-separated permutation kinds (default: all)")
	cmd.Flags().IntVar(&limit, "limit", 200, "Max results")
	return cmd
}

func newSocialsCmd(flags *rootFlags) *cobra.Command {
	var twitterOnly bool
	cmd := &cobra.Command{
		Use:   "socials <handle>",
		Short: "Check whether a handle is taken on common social platforms.",
		Long: `Sends a HEAD request to public profile URLs. No auth required.
A 404 response is treated as "available"; 2xx as "taken"; other codes as unknown.`,
		Example: `  domain-goat-pp-cli socials kindred
  domain-goat-pp-cli socials kindred --twitter --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			handle := strings.ToLower(args[0])
			if dryRunOK(flags) {
				return emitJSON(cmd, flags, map[string]any{"dry_run": true, "handle": handle})
			}
			profiles := []struct {
				Name string
				URL  string
			}{
				{"twitter", "https://twitter.com/" + handle},
				{"github", "https://github.com/" + handle},
				{"instagram", "https://www.instagram.com/" + handle + "/"},
			}
			if twitterOnly {
				profiles = profiles[:1]
			}
			type SocialResult struct {
				Platform  string `json:"platform"`
				URL       string `json:"url"`
				Status    int    `json:"status"`
				Available bool   `json:"available"`
				Note      string `json:"note,omitempty"`
			}
			out := make([]SocialResult, 0, len(profiles))
			c := &http.Client{Timeout: 6 * time.Second}
			for _, p := range profiles {
				r := SocialResult{Platform: p.Name, URL: p.URL}
				// PATCH(socials-request-error-handling): capture http.NewRequestWithContext error, record it on the result row, and skip the request instead of dereferencing nil req. URL-unsafe handles previously made url.Parse fail and req.Header.Set panicked on the next line.
				// NewRequestWithContext parses p.URL; a handle with URL-unsafe
				// characters (e.g. a space) makes req nil. Surface the parse
				// error on the result row instead of dereferencing nil.
				req, reqErr := http.NewRequestWithContext(cmd.Context(), http.MethodHead, p.URL, nil)
				if reqErr != nil {
					r.Note = reqErr.Error()
				} else {
					req.Header.Set("User-Agent", "Mozilla/5.0 domain-goat-pp-cli/1.0")
					resp, err := c.Do(req)
					if err != nil {
						r.Note = err.Error()
					} else {
						r.Status = resp.StatusCode
						r.Available = resp.StatusCode == 404
						resp.Body.Close()
					}
				}
				if handle == "" {
					r.Available = false
				}
				if p.Name == "twitter" && len(handle) > 15 {
					r.Note = "twitter handles are limited to 15 chars"
					r.Available = false
				}
				out = append(out, r)
			}
			if wantJSON(cmd, flags) {
				return emitJSON(cmd, flags, out)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "PLATFORM\tSTATUS\tAVAILABLE\tNOTE")
			for _, r := range out {
				fmt.Fprintf(tw, "%s\t%d\t%v\t%s\n", r.Platform, r.Status, r.Available, r.Note)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&twitterOnly, "twitter", false, "Only check Twitter/X")
	return cmd
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func dedupeStrings(xs []string) []string {
	if len(xs) == 0 {
		return xs
	}
	out := xs[:0]
	seen := map[string]struct{}{}
	for _, x := range xs {
		if _, ok := seen[x]; ok {
			continue
		}
		seen[x] = struct{}{}
		out = append(out, x)
	}
	return out
}
