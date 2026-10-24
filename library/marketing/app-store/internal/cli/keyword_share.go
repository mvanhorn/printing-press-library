// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"

	"github.com/spf13/cobra"
)

type keywordSharePos struct {
	Term             string `json:"term"`
	Position         *int   `json:"position"`
	PreviousPosition *int   `json:"previousPosition"`
}

type keywordShareRow struct {
	AppID     string `json:"appId"`
	Title     string `json:"title"`
	Target    bool   `json:"target"`
	TermsHeld int    `json:"termsHeld"`
	// TermsHeldComparable counts held terms that also have an earlier
	// capture, so TermsHeldComparable - TermsHeldPrevious == Change.
	TermsHeldComparable int               `json:"termsHeldComparable"`
	TermsHeldPrevious   int               `json:"termsHeldPrevious"`
	Change              int               `json:"change"`
	AvgPosition         *float64          `json:"avgPosition"`
	BestPosition        *int              `json:"bestPosition"`
	Terms               []keywordSharePos `json:"terms"`
}

type keywordShareTermMeta struct {
	Term               string  `json:"term"`
	Platform           string  `json:"platform"`
	Source             string  `json:"source"`
	LastCapturedAt     string  `json:"lastCapturedAt"`
	PreviousCapturedAt *string `json:"previousCapturedAt"`
	TopStored          int     `json:"topStored"`
	ResultCount        int     `json:"resultCount"`
}

type keywordShareGap struct {
	Term       string   `json:"term"`
	TargetRank *int     `json:"targetRank"`
	Leaders    []string `json:"leaders"`
}

type keywordShareTarget struct {
	AppID     string            `json:"app_id"`
	TermsHeld int               `json:"terms_held"`
	HeldTerms []keywordSharePos `json:"held_terms"`
	GapTerms  []keywordShareGap `json:"gap_terms"`
}

type keywordShareMeta struct {
	Storefront      string                 `json:"storefront"`
	Platform        string                 `json:"platform"`
	Source          string                 `json:"source"`
	Top             int                    `json:"top"`
	TermsCounted    int                    `json:"terms_counted"`
	ComparableTerms int                    `json:"comparable_terms"`
	Terms           []keywordShareTermMeta `json:"terms"`
	TermsMissing    []string               `json:"terms_missing"`
	TotalApps       int                    `json:"total_apps"`
	Limit           int                    `json:"limit"`
	Truncated       bool                   `json:"truncated"`
	Unweighted      bool                   `json:"unweighted"`
	Target          *keywordShareTarget    `json:"target"`
	Note            string                 `json:"note"`
}

type keywordShareView struct {
	Meta    keywordShareMeta  `json:"meta"`
	Results []keywordShareRow `json:"results"`
}

type keywordShareOpts struct {
	terms      []string
	storefront string
	platform   string
	source     string
	top        int
	limit      int
	target     *appstore.AppRef
}

const keywordShareUnweightedNote = "Counts are unweighted: Apple publishes no search volume, so every tracked term counts once."

func newNovelKeywordShareCmd(flags *rootFlags) *cobra.Command {
	var flagTop, flagLimit int
	var flagApp, flagPlatform, flagSource, flagDB string

	cmd := &cobra.Command{
		Use:   "keyword-share [term...]",
		Short: "See which apps hold the top search results across tracked terms and how that share moved since the last capture",
		Long:  "Use this to see which apps hold the top search results across a set of tracked terms and how that share moved since the previous capture. Do NOT use this for one app's position on one term over time; use 'keyword-history' instead, and to capture a fresh result list use 'keyword-rank'.",
		Example: strings.Trim(`
  app-store-pp-cli keyword-share "music streaming" "podcast player" --country us --top 10 --app 324684580 --agent
  app-store-pp-cli keyword-share --country gb --top 5 --json
  app-store-pp-cli keyword-share spotify music --app com.spotify.client`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "term=spotify;--top=10;--app=324684580",
			// Any text is a valid search term; an untracked term is an empty
			// local result (exit 0, listed in meta.terms_missing), not an error.
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "keyword-share")
			}
			opts := keywordShareOpts{top: flagTop, limit: flagLimit}
			seen := map[string]bool{}
			for _, a := range args {
				term := store.NormalizeKeywordTerm(a)
				if term == "" {
					return usageErr(fmt.Errorf("empty search term; pass terms such as \"music streaming\" or omit them to use every tracked term"))
				}
				if !seen[term] {
					seen[term] = true
					opts.terms = append(opts.terms, term)
				}
			}
			if flagTop < 1 || flagTop > 200 {
				return usageErr(fmt.Errorf("--top must be between 1 and 200, got %d", flagTop))
			}
			if flagLimit < 0 {
				return usageErr(fmt.Errorf("--limit must be 0 (no limit) or more, got %d", flagLimit))
			}
			cc, err := appStoreCountry(flags)
			if err != nil {
				return err
			}
			opts.storefront = cc
			if strings.TrimSpace(flagPlatform) != "" {
				if opts.platform, err = parsePlatform(flagPlatform); err != nil {
					return err
				}
			}
			if opts.source, err = keywordShareSource(flagSource); err != nil {
				return err
			}
			if strings.TrimSpace(flagApp) != "" {
				ref, err := parseAppRef(flagApp)
				if err != nil {
					return err
				}
				opts.target = &ref
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// Without a store, name the terms that were asked for: they are the
			// missing ones, and the capture hint uses the first of them.
			hintTerm, hintApp := "music streaming", " --app 324684580"
			if len(opts.terms) > 0 {
				hintTerm, hintApp = opts.terms[0], ""
			}
			if strings.TrimSpace(flagApp) != "" {
				hintApp = " --app " + strings.TrimSpace(flagApp)
			}
			hint := fmt.Sprintf("capture result lists first, e.g. 'app-store-pp-cli keyword-rank %q%s --country %s', on more than one day to see movement", hintTerm, hintApp, cc)
			st, ok, err := s3bOpenStore(cmd, flags, hint, func(note string) any {
				meta := newKeywordShareMeta(opts)
				meta.TermsMissing = append(meta.TermsMissing, opts.terms...)
				meta.Note = keywordShareUnweightedNote + " " + note
				return keywordShareView{Meta: meta, Results: make([]keywordShareRow, 0)}
			})
			if err != nil || !ok {
				return err
			}
			defer func() { _ = st.Close() }()

			view, err := buildKeywordShare(ctx, st, opts)
			if err != nil {
				return s3bLocalReadErr(err)
			}
			stamps := make([]string, 0, len(view.Meta.Terms))
			for _, t := range view.Meta.Terms {
				stamps = append(stamps, t.LastCapturedAt)
			}
			warnStale(cmd, flags, s3bNewestOf(stamps...), s3bNow(), "keyword capture", fmt.Sprintf("app-store-pp-cli keyword-rank <term> --country %s for each term", cc))
			return renderKeywordShare(cmd, flags, view)
		},
	}
	cmd.Flags().IntVar(&flagTop, "top", 10, "Count a term as held when the app ranks in the top N stored results (1-200)")
	cmd.Flags().StringVar(&flagApp, "app", "", "Target app (id, id123, store URL or bundle id): marks its row and lists the terms competitors hold that it lacks")
	cmd.Flags().StringVar(&flagPlatform, "platform", "", "Only count captures for this platform (iphone, ipad, mac); default any")
	cmd.Flags().StringVar(&flagSource, "source", "", "Only count captures from this source: itunes (Search API) or web (opt-in web search page); default any")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum apps to return, most terms held first (0 = no limit; the --app target is always kept)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Local snapshot store path (default: the CLI data directory)")
	return cmd
}

func keywordShareSource(raw string) (string, error) {
	// One keyword-source vocabulary for every keyword reader.
	return s3aKeywordSource(raw)
}

func newKeywordShareMeta(opts keywordShareOpts) keywordShareMeta {
	return keywordShareMeta{
		Storefront:   opts.storefront,
		Platform:     opts.platform,
		Source:       opts.source,
		Top:          opts.top,
		Terms:        make([]keywordShareTermMeta, 0),
		TermsMissing: make([]string, 0),
		Limit:        opts.limit,
		Unweighted:   true,
	}
}

// keywordShareTerm is one counted term: its latest capture's top-N ranks and
// the previous comparable capture's (same platform and source), if any.
type keywordShareTerm struct {
	meta     keywordShareTermMeta
	latest   map[string]int
	previous map[string]int // nil when the term has one capture
	leaders  []string
	captures []store.KeywordRank
}

// topPositions maps app ids to 1-based positions within the first n stored
// results; a targeted capture's own rank fills in when its list is shorter.
func topPositions(k store.KeywordRank, n int) map[string]int {
	out := map[string]int{}
	for i, id := range k.TopIDs {
		if i >= n {
			break
		}
		if id = strings.TrimSpace(id); id != "" {
			if _, dup := out[id]; !dup {
				out[id] = i + 1
			}
		}
	}
	if k.AppID != "" && k.Rank > 0 && k.Rank <= n {
		if _, ok := out[k.AppID]; !ok {
			out[k.AppID] = k.Rank
		}
	}
	return out
}

func buildKeywordShare(ctx context.Context, st *store.Store, opts keywordShareOpts) (keywordShareView, error) {
	view := keywordShareView{Meta: newKeywordShareMeta(opts), Results: make([]keywordShareRow, 0)}
	all, err := store.KeywordSeries(ctx, st, store.KeywordFilter{Storefront: opts.storefront, Platform: opts.platform, Source: opts.source})
	if err != nil {
		return view, err
	}
	byTerm := map[string][]store.KeywordRank{}
	for _, k := range all {
		byTerm[k.Term] = append(byTerm[k.Term], k)
	}
	terms := opts.terms
	if len(terms) == 0 {
		terms = s3bSortedKeys(byTerm)
	}

	counted := make([]*keywordShareTerm, 0, len(terms))
	shortLists := 0
	for _, term := range terms {
		caps := byTerm[term]
		if len(caps) == 0 {
			view.Meta.TermsMissing = append(view.Meta.TermsMissing, term)
			continue
		}
		latest := caps[len(caps)-1]
		t := &keywordShareTerm{latest: topPositions(latest, opts.top), captures: caps}
		t.meta = keywordShareTermMeta{Term: term, Platform: latest.Platform, Source: latest.Source, LastCapturedAt: s3bFormat(latest.CapturedAt),
			TopStored: len(latest.TopIDs), ResultCount: latest.ResultCount}
		for i := len(caps) - 2; i >= 0; i-- {
			p := caps[i]
			if p.CapturedAt.Before(latest.CapturedAt) && p.Platform == latest.Platform && p.Source == latest.Source {
				t.previous = topPositions(p, opts.top)
				t.meta.PreviousCapturedAt = s3bTimePtr(p.CapturedAt)
				break
			}
		}
		for i, id := range latest.TopIDs {
			if i >= 3 {
				break
			}
			t.leaders = append(t.leaders, id)
		}
		if t.meta.TopStored < opts.top {
			shortLists++
		}
		counted = append(counted, t)
		view.Meta.Terms = append(view.Meta.Terms, t.meta)
	}
	view.Meta.TermsCounted = len(counted)

	type agg struct {
		now, prev map[string]int
	}
	apps := map[string]*agg{}
	get := func(id string) *agg {
		a := apps[id]
		if a == nil {
			a = &agg{now: map[string]int{}, prev: map[string]int{}}
			apps[id] = a
		}
		return a
	}
	comparable := map[string]bool{}
	for _, t := range counted {
		if t.previous != nil {
			comparable[t.meta.Term] = true
		}
		for id, pos := range t.latest {
			get(id).now[t.meta.Term] = pos
		}
		for id, pos := range t.previous {
			get(id).prev[t.meta.Term] = pos
		}
	}
	view.Meta.ComparableTerms = len(comparable)

	var targetID, unresolvedTarget string
	if opts.target != nil {
		ids, missing, err := s3bResolveLocalApps(ctx, st, []appstore.AppRef{*opts.target})
		if err != nil {
			return view, err
		}
		if len(ids) > 0 {
			targetID = ids[0]
			get(targetID)
		} else if len(missing) > 0 {
			unresolvedTarget = missing[0]
		}
	}
	titles, err := s3bAppTitles(ctx, st)
	if err != nil {
		return view, err
	}

	rows := make([]keywordShareRow, 0, len(apps))
	for id, a := range apps {
		row := keywordShareRow{AppID: id, Title: titles[id], Target: id == targetID, Terms: make([]keywordSharePos, 0)}
		row.TermsHeld = len(a.now)
		row.TermsHeldPrevious = len(a.prev)
		heldComparable := 0
		sum, best := 0, 0
		for term, pos := range a.now {
			if comparable[term] {
				heldComparable++
			}
			sum += pos
			if best == 0 || pos < best {
				best = pos
			}
		}
		row.TermsHeldComparable = heldComparable
		row.Change = heldComparable - row.TermsHeldPrevious
		if row.TermsHeld > 0 {
			avg := s3bRound2(float64(sum) / float64(row.TermsHeld))
			row.AvgPosition = &avg
			b := best
			row.BestPosition = &b
		}
		termSet := map[string]bool{}
		for term := range a.now {
			termSet[term] = true
		}
		for term := range a.prev {
			termSet[term] = true
		}
		for _, term := range s3bSortedKeys(termSet) {
			p := keywordSharePos{Term: term}
			if pos, ok := a.now[term]; ok {
				v := pos
				p.Position = &v
			}
			if pos, ok := a.prev[term]; ok {
				v := pos
				p.PreviousPosition = &v
			}
			row.Terms = append(row.Terms, p)
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.TermsHeld != b.TermsHeld {
			return a.TermsHeld > b.TermsHeld
		}
		if (a.AvgPosition == nil) != (b.AvgPosition == nil) {
			return a.AvgPosition != nil
		}
		if a.AvgPosition != nil && *a.AvgPosition != *b.AvgPosition {
			return *a.AvgPosition < *b.AvgPosition
		}
		if a.TermsHeldPrevious != b.TermsHeldPrevious {
			return a.TermsHeldPrevious > b.TermsHeldPrevious
		}
		return a.AppID < b.AppID
	})
	view.Meta.TotalApps = len(rows)
	if opts.limit > 0 && len(rows) > opts.limit {
		kept := append(make([]keywordShareRow, 0, opts.limit+1), rows[:opts.limit]...)
		for _, r := range rows[opts.limit:] {
			if r.Target {
				kept = append(kept, r)
			}
		}
		rows = kept
		view.Meta.Truncated = true
	}
	view.Results = rows

	if targetID != "" {
		view.Meta.Target = keywordShareTargetMeta(targetID, apps[targetID].now, apps[targetID].prev, counted)
	}

	note := keywordShareUnweightedNote
	switch {
	case len(all) == 0:
		// The hint names what was asked for: the first requested term, the
		// --app target and the storefront.
		hintTerm, hintApp := "music streaming", ""
		if len(opts.terms) > 0 {
			hintTerm = opts.terms[0]
		}
		if opts.target != nil {
			ref := opts.target.ID
			if ref == "" {
				ref = opts.target.BundleID
			}
			hintApp = " --app " + ref
		}
		note += fmt.Sprintf(" No keyword captures are stored for storefront %s; capture result lists with 'keyword-rank %q%s --country %s' or 'search-store %q --country %s'.", opts.storefront, hintTerm, hintApp, opts.storefront, hintTerm, opts.storefront)
	case len(counted) == 0:
		note += fmt.Sprintf(" None of the requested terms (%s) has a stored capture in storefront %s; tracked terms there: %s.", strings.Join(view.Meta.TermsMissing, ", "), opts.storefront, strings.Join(s3bFirstN(s3bSortedKeys(byTerm), 20), ", "))
	default:
		note += fmt.Sprintf(" %d term(s) counted; change compares the %d term(s) with an earlier capture (same platform and source).", len(counted), len(comparable))
		if len(view.Meta.TermsMissing) > 0 {
			note += fmt.Sprintf(" No stored capture for: %s.", strings.Join(view.Meta.TermsMissing, ", "))
		}
		if shortLists > 0 {
			note += fmt.Sprintf(" %d term(s) stored fewer than %d results, so only their stored ranks count.", shortLists, opts.top)
		}
	}
	if unresolvedTarget != "" {
		note += fmt.Sprintf(" Target bundle id %s is not in the local store, so no target row is shown; pass its numeric id or capture it with 'app-store-pp-cli app %s'.", unresolvedTarget, unresolvedTarget)
	}
	view.Meta.Note = note
	return view, nil
}

func keywordShareTargetMeta(targetID string, now, prev map[string]int, counted []*keywordShareTerm) *keywordShareTarget {
	t := &keywordShareTarget{AppID: targetID, TermsHeld: len(now), HeldTerms: make([]keywordSharePos, 0), GapTerms: make([]keywordShareGap, 0)}
	for _, term := range counted {
		name := term.meta.Term
		if pos, ok := now[name]; ok {
			p := keywordSharePos{Term: name}
			v := pos
			p.Position = &v
			if pp, ok := prev[name]; ok {
				w := pp
				p.PreviousPosition = &w
			}
			t.HeldTerms = append(t.HeldTerms, p)
			continue
		}
		if len(term.latest) == 0 {
			continue
		}
		gap := keywordShareGap{Term: name, Leaders: append(make([]string, 0, 3), term.leaders...)}
		// The target's own latest tracked rank for this term, when one was captured.
		for i := len(term.captures) - 1; i >= 0; i-- {
			k := term.captures[i]
			if k.AppID == targetID && k.Platform == term.meta.Platform && k.Source == term.meta.Source {
				if k.Rank > 0 {
					r := k.Rank
					gap.TargetRank = &r
				}
				break
			}
		}
		t.GapTerms = append(t.GapTerms, gap)
	}
	return t
}

// s3bFirstN returns at most n items of list.
func s3bFirstN(list []string, n int) []string {
	if len(list) <= n {
		return list
	}
	return list[:n]
}

func renderKeywordShare(cmd *cobra.Command, flags *rootFlags, view keywordShareView) error {
	w := cmd.OutOrStdout()
	if !wantsHumanTable(w, flags) {
		return printJSONFiltered(w, view, flags)
	}
	if len(view.Results) == 0 {
		_, _ = fmt.Fprintln(w, view.Meta.Note)
		return nil
	}
	rows := make([][]string, 0, len(view.Results))
	for _, r := range view.Results {
		mark := ""
		if r.Target {
			mark = "target"
		}
		// "compared" (held among terms with an earlier capture) minus "prev"
		// is "change"; with no earlier capture at all there is nothing to
		// compare, shown as a dash rather than 0.
		compared, prev, change := fmt.Sprint(r.TermsHeldComparable), fmt.Sprint(r.TermsHeldPrevious), fmt.Sprintf("%+d", r.Change)
		if view.Meta.ComparableTerms == 0 {
			compared, prev, change = "-", "-", "-"
		}
		rows = append(rows, []string{r.AppID, s3bTruncate(r.Title, 32), fmt.Sprint(r.TermsHeld), compared, prev,
			change, s3bOptFloat(r.AvgPosition), s3bOptRank(r.BestPosition), mark})
	}
	_, _ = fmt.Fprintf(w, "Top %d share across %d term(s) in %s\n\n", view.Meta.Top, view.Meta.TermsCounted, view.Meta.Storefront)
	if err := s3bPrintTable(w, []string{"app", "title", "held", "compared", "prev", "change", "avg pos", "best", ""}, rows); err != nil {
		return err
	}
	if t := view.Meta.Target; t != nil && len(t.GapTerms) > 0 {
		_, _ = fmt.Fprintf(w, "\nTerms where competitors hold the top %d and %s does not:\n", view.Meta.Top, t.AppID)
		for _, g := range t.GapTerms {
			rank := "not found"
			if g.TargetRank != nil {
				rank = fmt.Sprintf("rank %d", *g.TargetRank)
			}
			_, _ = fmt.Fprintf(w, "  %s (%s; leaders %s)\n", g.Term, rank, strings.Join(g.Leaders, ", "))
		}
	}
	_, _ = fmt.Fprintln(w, "\n"+view.Meta.Note)
	return nil
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNovelKeywordShareCmd(flags))
	})
}
