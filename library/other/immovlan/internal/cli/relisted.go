// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source computed

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/immovlan"
	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/store"
)

type relistRef struct {
	ID        string   `json:"id"`
	URL       string   `json:"url"`
	FirstSeen string   `json:"first_seen"`
	LastSeen  string   `json:"last_seen"`
	CreatedAt string   `json:"created_at,omitempty"`
	Price     *float64 `json:"price,omitempty"`
	GoneAt    string   `json:"gone_at,omitempty"`
}

type relistedView struct {
	Results []relistGroup `json:"results"`
	Note    string        `json:"note,omitempty"`
}

type relistGroup struct {
	MatchedBy      string      `json:"matched_by"` // address or photos
	Key            string      `json:"key"`
	Locality       string      `json:"locality"`
	Street         string      `json:"street,omitempty"`
	Refs           []relistRef `json:"references"`
	Current        string      `json:"current_reference"`
	CurrentURL     string      `json:"current_url"`
	FirstListed    string      `json:"first_listed"`
	TrueDaysOnMkt  int         `json:"true_days_on_market"`
	ShownDaysOnMkt *int        `json:"shown_days_on_market,omitempty"`
	FirstPrice     *float64    `json:"first_price,omitempty"`
	CurrentPrice   *float64    `json:"current_price,omitempty"`
	PriceChangePct *float64    `json:"price_change_pct,omitempty"`
	FlaggedAsNew   bool        `json:"flagged_as_new"`
}

func newNovelRelistedCmd(flags *rootFlags) *cobra.Command {
	var flagSince, flagPostcode, dbPath string
	var flagLimit int
	cmd := &cobra.Command{
		Use:   "relisted",
		Short: "Listings that came back under a new reference, with their true first-seen date and price at each reference",
		Long: `Agencies re-list a property under a fresh reference, which resets the "Nouveau"
ribbon and the publication date on Immovlan. relisted groups stored listings by
normalised address (or by identical photos after enrich) across different
references and reports the earliest first-seen date, the true days on market and
the price at each reference. Two references only form a group when they share
the deal (a sale that comes back as a rental is not a re-listing) and when the older
one had left Immovlan before the newer appeared, or when they describe the same
home (same type and bedrooms, surface within 10 %) and were published at least a
week apart; sibling units in one building and duplicate ads posted together are
not re-listings. References are ordered by the site's publication date when the
detail page gave one, else by when this store first saw them.
Use this command to see listings that came back under a new reference. Do NOT
use it for price cuts on the same reference; use 'drops'. 'watch' reports new
and gone listings per saved search but does not link a gone listing to its
re-listing.`,
		Example: strings.Trim(`
  immovlan-pp-cli relisted --since 90d --postcode 1030 --agent
  immovlan-pp-cli relisted --limit 20`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "computed", "pp:happy-args": "--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "detect re-listed properties in the local store")
			}
			if err := rejectDataSource(flags, "local"); err != nil {
				return usageErr(err)
			}
			flags.agentSource = "local"
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cutoff, err := sinceCutoff(flagSince)
			if err != nil {
				return err
			}
			f := store.ListingFilter{IncludeGone: true}
			if f.PostalCodes, err = parsePostcodes(flagPostcode); err != nil {
				return err
			}
			groups := []relistGroup{}
			note := ""
			if path, ok := localStoreExists(dbPath); ok {
				db, err := openVlanStore(ctx, path)
				if err != nil {
					return err
				}
				defer db.Close()
				rows, err := db.QueryVlanListings(ctx, f)
				if err != nil {
					return err
				}
				groups = groupRelisted(rows, time.Now())
			} else {
				note = noStoreNote(path, "find")
			}
			out := []relistGroup{}
			for _, g := range groups {
				if cutoff != "" && g.Refs[len(g.Refs)-1].FirstSeen < cutoff {
					continue
				}
				out = append(out, g)
			}
			sort.SliceStable(out, func(i, j int) bool { return out[i].TrueDaysOnMkt > out[j].TrueDaysOnMkt })
			if flagLimit > 0 && len(out) > flagLimit {
				out = out[:flagLimit]
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printView(cmd.OutOrStdout(), relistedView{Results: out, Note: note}, flags)
			}
			w := cmd.OutOrStdout()
			if len(out) == 0 {
				fmt.Fprintln(w, "No re-listed properties detected (needs the same address or photos under two references; run enrich to add addresses).")
			} else {
				tw := newTabWriter(w)
				fmt.Fprintln(tw, "CURRENT\tLOCALITY\tSTREET\tREFS\tFIRST LISTED\tTRUE DAYS\tSHOWN\tFIRST €\tNOW €\tCHANGE\tNEW?")
				for _, g := range out {
					chg := "-"
					if g.PriceChangePct != nil {
						chg = fmt.Sprintf("%+.1f%%", *g.PriceChangePct)
					}
					fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\t%d\t%s\t%s\t%s\t%s\t%v\n", g.Current, termSafe(g.Locality), termSafe(truncate(g.Street, 26)), len(g.Refs), shortDate(g.FirstListed), g.TrueDaysOnMkt, intStr(g.ShownDaysOnMkt), fmtPtrEUR(g.FirstPrice), fmtPtrEUR(g.CurrentPrice), chg, g.FlaggedAsNew)
				}
				_ = tw.Flush()
			}
			if note != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Only re-listings whose newest reference appeared in this window (e.g. 90d)")
	cmd.Flags().StringVar(&flagPostcode, "postcode", "", "Only these postal codes, comma-separated")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum rows")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path")
	_ = cmd.Flags().MarkHidden("db")
	return cmd
}

// minRelistGap is how far apart two still-live references must have been
// published to count as a re-listing rather than a duplicate ad or a sibling
// unit posted the same week.
const minRelistGap = 7 * 24 * time.Hour

// listedAt is when a reference was published: the site's created_at when the
// detail page gave one, else the first time this store saw it. Zero when
// neither parses.
func listedAt(m store.StoredListing) time.Time {
	for _, s := range []string{m.CreatedAt, m.FirstSeen} {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// sameHome reports whether two references at one address can be the same
// property: the older one had gone before the newer appeared, or type,
// bedrooms and surface (±10 %) agree and the two were published at least
// minRelistGap apart. Concurrently-live sibling units in a building and
// duplicate ads posted together fail these tests.
func sameHome(older, newer store.StoredListing) bool {
	if older.Deal != "" && newer.Deal != "" && older.Deal != newer.Deal {
		return false // a sale that comes back as a rental is not a re-listing
	}
	if older.GoneAt != "" && older.GoneAt <= newer.FirstSeen {
		return true
	}
	if listedAt(newer).Sub(listedAt(older)) < minRelistGap {
		return false // both live and posted within a week: a duplicate ad or a sibling, not a comeback
	}
	if older.Type != "" && newer.Type != "" && older.Type != newer.Type {
		return false
	}
	if older.Bedrooms != nil && newer.Bedrooms != nil && *older.Bedrooms != *newer.Bedrooms {
		return false
	}
	if older.Surface != nil && newer.Surface != nil {
		lo, hi := *older.Surface*0.9, *older.Surface*1.1
		if *newer.Surface < lo || *newer.Surface > hi {
			return false
		}
	}
	return older.Type != "" || older.Bedrooms != nil || older.Surface != nil
}

// groupRelisted links references that share an address key or a photo hash
// and pass sameHome, oldest first.
func groupRelisted(rows []store.StoredListing, now time.Time) []relistGroup {
	byKey := map[string][]store.StoredListing{}
	keyKind := map[string]string{}
	for _, r := range rows {
		if r.AddrKey != "" {
			byKey["a:"+r.AddrKey] = append(byKey["a:"+r.AddrKey], r)
			keyKind["a:"+r.AddrKey] = "address"
		}
		if r.PhotoHash != "" {
			byKey["p:"+r.PhotoHash] = append(byKey["p:"+r.PhotoHash], r)
			keyKind["p:"+r.PhotoHash] = "photos"
		}
	}
	placed := map[string]bool{} // IDs already reported; a photo chain fully inside an address group is the same home
	out := []relistGroup{}
	// Address keys first (a: sorts before p:) so a pair matched both ways is
	// reported as an address match; sorted keys keep the output stable.
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		members := byKey[key]
		ids := map[string]bool{}
		uniq := []store.StoredListing{}
		for _, m := range members {
			if !ids[m.ID] {
				ids[m.ID] = true
				uniq = append(uniq, m)
			}
		}
		// Order by publication date, not scrape order: on a first scrape every
		// reference shares one first_seen, and only created_at tells which
		// listing is the comeback.
		sort.SliceStable(uniq, func(i, j int) bool {
			ti, tj := listedAt(uniq[i]), listedAt(uniq[j])
			if !ti.Equal(tj) {
				return ti.Before(tj)
			}
			return uniq[i].ID < uniq[j].ID
		})
		// One address can hold several homes: chain each reference to the
		// first chain whose tail it can be the same home as, else start one.
		chains := [][]store.StoredListing{}
		for _, m := range uniq {
			placed := false
			for i := range chains {
				if sameHome(chains[i][len(chains[i])-1], m) {
					chains[i] = append(chains[i], m)
					placed = true
					break
				}
			}
			if !placed {
				chains = append(chains, []store.StoredListing{m})
			}
		}
		for _, chain := range chains {
			if len(chain) < 2 {
				continue
			}
			covered := true
			for _, m := range chain {
				if !placed[m.ID] {
					covered = false
				}
			}
			if covered {
				continue
			}
			for _, m := range chain {
				placed[m.ID] = true
			}
			out = append(out, relistGroupFrom(keyKind[key], key[2:], chain, now)) // key[2:] strips the a:/p: prefix
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Current < out[j].Current })
	return out
}

// relistGroupFrom builds one relistGroup from an oldest-first chain of
// references sharing an address key or photo hash.
func relistGroupFrom(kind, key string, chain []store.StoredListing, now time.Time) relistGroup {
	g := relistGroup{MatchedBy: kind, Key: key}
	for _, m := range chain {
		g.Refs = append(g.Refs, relistRef{ID: m.ID, URL: m.URL, FirstSeen: m.FirstSeen, LastSeen: m.LastSeen, CreatedAt: m.CreatedAt, Price: m.Price, GoneAt: m.GoneAt})
	}
	first, last := chain[0], chain[len(chain)-1]
	g.Locality, g.Street = locLabel(last.Listing), last.Street
	g.Current, g.CurrentURL = last.ID, last.URL
	g.FirstListed = first.FirstSeen
	if first.CreatedAt != "" && first.CreatedAt < first.FirstSeen {
		g.FirstListed = first.CreatedAt
	}
	if d, ok := immovlan.DaysListed(g.FirstListed, now); ok {
		g.TrueDaysOnMkt = d
	}
	if d, ok := immovlan.DaysListed(last.CreatedAt, now); ok {
		g.ShownDaysOnMkt = &d
	}
	g.FirstPrice, g.CurrentPrice = first.Price, last.Price
	if first.Price != nil && last.Price != nil && *first.Price > 0 {
		p := pct1((*last.Price - *first.Price) / *first.Price)
		g.PriceChangePct = &p
	}
	g.FlaggedAsNew = last.Flag == "new"
	return g
}
