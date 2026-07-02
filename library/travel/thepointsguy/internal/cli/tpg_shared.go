// Copyright 2026 megumikuo and contributors. Licensed under Apache-2.0. See LICENSE.
// Shared helpers for the hand-authored Points Guy commands: engine client
// construction, local-store access, output emission, and program-name resolution.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/thepointsguy/internal/store"
	"github.com/mvanhorn/printing-press-library/library/travel/thepointsguy/internal/tpg"
)

// newTPGClient builds the Points Guy engine client using the root rate limit.
func newTPGClient(flags *rootFlags) *tpg.Client {
	return tpg.New(flags.rateLimit)
}

// tpgDBPath resolves the local SQLite mirror path.
func tpgDBPath() string { return defaultDBPath("thepointsguy-pp-cli") }

// emitJSON writes v as JSON honoring --select/--compact/--csv/--quiet.
func emitJSON(cmd *cobra.Command, flags *rootFlags, v any) error {
	return printJSONFiltered(cmd.OutOrStdout(), v, flags)
}

// resource type keys in the local store.
const (
	rtValuations = "valuations"
	rtCards      = "cards"
	rtArticles   = "articles"
)

var reNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// programAliases expand common shorthands to their canonical program wording so
// fuzzy matching succeeds for everyday names an agent or user would type.
var programAliases = []struct{ from, to string }{
	{"amex", "american express"},
	{"aadvantage", "american airlines aadvantage"},
	{" mr", " membership rewards"},
	{" ur", " ultimate rewards"},
	{" ty", " thankyou"},
}

// normProgram lowercases, expands aliases, and strips punctuation.
func normProgram(s string) string {
	low := " " + strings.ToLower(s) + " "
	for _, a := range programAliases {
		low = strings.ReplaceAll(low, a.from, a.to)
	}
	return strings.Trim(reNonAlnum.ReplaceAllString(low, " "), " ")
}

// sigTokens returns significant (length>=3) tokens of a normalized string.
func sigTokens(norm string) []string {
	var out []string
	for _, t := range strings.Fields(norm) {
		if len(t) >= 3 {
			out = append(out, t)
		}
	}
	return out
}

// loadValuations reads the mirrored valuations for the most recent month in the
// store. Returns them keyed by normalized program name plus the flat slice.
func loadValuations(st *store.Store) (map[string]tpg.Valuation, []tpg.Valuation, error) {
	raws, err := st.List(rtValuations, 100000)
	if err != nil {
		return nil, nil, err
	}
	all := make([]tpg.Valuation, 0, len(raws))
	latestMonthByProg := map[string]tpg.Valuation{}
	for _, r := range raws {
		var v tpg.Valuation
		if json.Unmarshal(r, &v) != nil || v.Program == "" {
			continue
		}
		all = append(all, v)
		// Keep the entry with the newest month per program.
		key := normProgram(v.Program)
		if cur, ok := latestMonthByProg[key]; !ok || v.Month > cur.Month {
			latestMonthByProg[key] = v
		}
	}
	return latestMonthByProg, all, nil
}

// resolveValuation finds a program's current valuation by fuzzy name match:
// exact key, then substring, then token-subset (all target tokens present in
// the program name). On ambiguity or miss it returns close candidates.
func resolveValuation(byProg map[string]tpg.Valuation, program string) (tpg.Valuation, []string, bool) {
	target := normProgram(program)
	if v, ok := byProg[target]; ok {
		return v, nil, true
	}
	// Substring match either direction.
	var subMatches []tpg.Valuation
	for key, v := range byProg {
		if strings.Contains(key, target) || strings.Contains(target, key) {
			subMatches = append(subMatches, v)
		}
	}
	if len(subMatches) == 1 {
		return subMatches[0], nil, true
	}

	// Token-subset: programs containing every significant target token.
	targetTokens := sigTokens(target)
	var full []tpg.Valuation
	bestPartial := 0
	var partialCands []string
	if len(targetTokens) > 0 {
		for key, v := range byProg {
			matched := 0
			for _, t := range targetTokens {
				if strings.Contains(key, t) {
					matched++
				}
			}
			if matched == len(targetTokens) {
				full = append(full, v)
			}
			if matched > bestPartial {
				bestPartial = matched
				partialCands = []string{v.Program}
			} else if matched == bestPartial && matched > 0 {
				partialCands = append(partialCands, v.Program)
			}
		}
	}
	if len(full) == 1 {
		return full[0], nil, true
	}
	if len(full) > 1 {
		// Prefer the shortest (closest) program name among full matches.
		best := full[0]
		for _, v := range full[1:] {
			if len(v.Program) < len(best.Program) {
				best = v
			}
		}
		return best, nil, true
	}

	// No confident match: surface candidates.
	cands := partialCands
	if len(cands) == 0 {
		for _, v := range byProg {
			cands = append(cands, v.Program)
		}
	}
	sort.Strings(cands)
	if len(cands) > 12 {
		cands = cands[:12]
	}
	return tpg.Valuation{}, cands, false
}

// truncRunes truncates s to at most max runes (not bytes), appending an
// ellipsis, so multibyte characters like ® are never split.
func truncRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return string(r[:max])
	}
	return string(r[:max-1]) + "…"
}

// dollarsFromPoints returns the estimated USD value of a points balance at a
// given cents-per-point rate.
func dollarsFromPoints(points float64, centsPerPoint float64) float64 {
	return points * centsPerPoint / 100.0
}

// centsPerPoint returns the effective cents-per-point of a redemption worth
// cashValue dollars for pointsUsed points.
func centsPerPoint(cashValue, pointsUsed float64) float64 {
	if pointsUsed == 0 {
		return 0
	}
	return cashValue / pointsUsed * 100.0
}

// missingMirrorHint prints a stderr hint and empty JSON when the store is absent.
func missingMirrorHint(cmd *cobra.Command, flags *rootFlags, resources string) {
	fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror found\nrun: thepointsguy-pp-cli sync --resources %s\n", resources)
	if flags.asJSON || flags.agent {
		fmt.Fprintln(cmd.OutOrStdout(), "[]")
	}
}

// persistValuations upserts a valuations snapshot into the store, keyed by
// program+month so historical months accumulate for `valuations drift`.
func persistValuations(st *store.Store, vals []tpg.Valuation) {
	for _, v := range vals {
		id := normProgram(v.Program) + "|" + v.Month
		data, err := json.Marshal(v)
		if err != nil {
			continue
		}
		_ = st.Upsert(rtValuations, id, data)
	}
}

// currentValuations resolves the current valuations map + month. Respects
// --data-source: "local" reads only the store; otherwise it fetches live (and
// opportunistically persists a snapshot), falling back to the store on error.
// The returned store may be nil when --data-source=live and no mirror exists.
func currentValuations(cmd *cobra.Command, flags *rootFlags, c *tpgClientCtx) (map[string]tpg.Valuation, string, error) {
	if flags.dataSource == "local" {
		st, err := openTPGStore()
		if err != nil {
			return nil, "", err
		}
		defer st.Close()
		byProg, all, err := loadValuations(st)
		if err != nil {
			return nil, "", err
		}
		if len(all) == 0 {
			return nil, "", fmt.Errorf("no valuations in local store; run: thepointsguy-pp-cli sync --resources valuations")
		}
		return byProg, firstMonth(all), nil
	}
	vals, month, err := c.client.Valuations(c.ctx)
	if err != nil {
		// Fall back to any mirrored snapshot.
		if st, oerr := openTPGStore(); oerr == nil {
			defer st.Close()
			if byProg, all, lerr := loadValuations(st); lerr == nil && len(all) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "live valuations unavailable (%v); using local mirror\n", err)
				return byProg, firstMonth(all), nil
			}
		}
		return nil, "", err
	}
	// Opportunistically persist for offline + drift history. Close errors on a
	// best-effort cache write are non-fatal and deliberately ignored.
	if st, oerr := openTPGStore(); oerr == nil {
		persistValuations(st, vals)
		_ = st.Close()
	}
	byProg := map[string]tpg.Valuation{}
	for _, v := range vals {
		byProg[normProgram(v.Program)] = v
	}
	return byProg, month, nil
}

func firstMonth(vals []tpg.Valuation) string {
	if len(vals) == 0 {
		return ""
	}
	return vals[0].Month
}

// openTPGStore opens (creating if needed) the local SQLite mirror.
func openTPGStore() (*store.Store, error) {
	return store.Open(tpgDBPath())
}

// tpgClientCtx bundles the engine client with a bounded context.
type tpgClientCtx struct {
	client *tpg.Client
	ctx    context.Context
}
