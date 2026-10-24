// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newCompareCmd(flags))
	})
}

// compareRow is one app in one storefront, every field from the same Lookup
// response so the apps line up field by field.
type compareRow struct {
	Storefront            string  `json:"storefront"`
	App                   string  `json:"app"`
	Title                 string  `json:"title"`
	Developer             string  `json:"developer"`
	BundleID              string  `json:"bundleId"`
	Genre                 string  `json:"genre"`
	Price                 float64 `json:"price"`
	FormattedPrice        string  `json:"formattedPrice"`
	Currency              string  `json:"currency"`
	AverageRating         float64 `json:"averageRating"`
	RatingCount           int64   `json:"ratingCount"`
	CurrentVersionAverage float64 `json:"currentVersionAverage"`
	CurrentVersionCount   int64   `json:"currentVersionCount"`
	Version               string  `json:"version"`
	VersionReleasedAt     string  `json:"versionReleasedAt"`
	FirstReleasedAt       string  `json:"firstReleasedAt"`
	SizeBytes             int64   `json:"sizeBytes"`
	MinimumOSVersion      string  `json:"minimumOsVersion"`
	ContentRating         string  `json:"contentRating"`
	Languages             int     `json:"languages"`
}

// compareMissing is an app Apple does not sell in a storefront.
type compareMissing struct {
	Storefront string `json:"storefront"`
	App        string `json:"app"`
}

type compareMeta struct {
	Apps        []string         `json:"apps"`
	Storefronts []string         `json:"storefronts"`
	Unavailable []compareMissing `json:"unavailable"`
	Requests    int              `json:"requests"`
	Notes       []string         `json:"notes"`
}

func newCompareCmd(flags *rootFlags) *cobra.Command {
	var countries string
	cmd := &cobra.Command{
		Use:   "compare <app> <app> [app...]",
		Short: "Compare apps side by side live: ratings, price, version and size per storefront",
		Long: strings.Trim(`
Line up two or more apps field by field in each storefront: title, developer,
genre, price, exact rating count and average, current version's rating,
version and release dates, size, minimum OS, content rating and language
count. One batched iTunes Lookup per storefront covers up to 150 apps, so
comparing ten apps costs one request per storefront, not ten. Each app's
ratings are recorded as a ratings snapshot (source lookup) for traction.

JSON is {meta, items, fetch_failures}: one item per app per storefront where
it is sold, in storefront then argument order; apps not sold somewhere are
listed in meta.unavailable, never as zero rows. The human view is a transposed
table per storefront. Needs at least two different apps; none of them sold in
any requested storefront exits 3.

Use this to benchmark competitors at a glance; 'app' for one listing in
depth, 'ratings' for one app across many storefronts.`, "\n"),
		Example: strings.Trim(`
  app-store-pp-cli compare 324684580 1017492454 292738169 --json
  app-store-pp-cli compare 553834731 850417475 --countries us,gb --select items.app,items.ratingCount,items.averageRating`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "live",
			"pp:typed-exit-codes": "0,7",
			"pp:happy-args":       "app=324684580;app=1017492454",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "compare")
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("compare needs at least two apps, got %d: %s", len(args), s2aAppArgHelp))
			}
			refs, err := s2aAppArgs(cmd, args, 2)
			if err != nil {
				return err
			}
			list, err := s2aStorefronts(cmd, flags, countries, nil)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := newAppStoreClient(flags)
			if err != nil {
				return err
			}
			st, err := openAppStoreStore(cmd, flags)
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()
			attachCaptureLog(c, st)

			failures := make([]fetchFailure, 0)
			ids, resolveFailures, resolveErrs, err := s2aResolveAppIDs(ctx, c, refs, list)
			if err != nil {
				return err
			}
			failures = append(failures, resolveFailures...)
			if len(ids) == 0 {
				if len(resolveErrs) > 0 {
					return resolveErrs[0]
				}
				return notFoundErr(fmt.Errorf("none of the apps was found in storefront(s) %s (check the ids, or try other --countries)", strings.Join(list, ",")))
			}

			meta := compareMeta{Apps: ids, Storefronts: list, Unavailable: make([]compareMissing, 0), Notes: make([]string, 0)}
			if len(ids) < 2 {
				meta.Notes = append(meta.Notes, "only one of the apps could be resolved; nothing to line it up against")
			}
			rows := make([]compareRow, 0, len(ids)*len(list))
			errs := make([]error, 0)
			// stopped: a throttle or pacing refusal ended the fan-out early;
			// what was gathered still prints, then exit 7.
			var stopped error
			for i, cc := range list {
				resp, err := c.Lookup(ctx, appstore.LookupQuery{IDs: ids, Country: cc})
				if err != nil {
					err = classifyAppStoreErr(err)
					failures = append(failures, fetchFailure{Storefront: cc, Error: err.Error()})
					if isRateLimited(err) || ctx.Err() != nil {
						stopped = err
						failures = append(failures, s2aNotAttempted("", list[i+1:], cc)...)
						break
					}
					errs = append(errs, err)
					continue
				}
				byID := make(map[string]appstore.LookupResult, len(resp.Results))
				for _, r := range resp.Results {
					byID[r.AppID()] = r
				}
				for _, id := range ids {
					lk, ok := byID[id]
					if !ok {
						meta.Unavailable = append(meta.Unavailable, compareMissing{Storefront: cc, App: id})
						continue
					}
					size, _ := strconv.ParseInt(strings.TrimSpace(lk.FileSizeBytes), 10, 64)
					rows = append(rows, compareRow{
						Storefront:            cc,
						App:                   id,
						Title:                 lk.TrackName,
						Developer:             lk.ArtistName,
						BundleID:              lk.BundleID,
						Genre:                 lk.PrimaryGenreName,
						Price:                 lk.Price,
						FormattedPrice:        lk.FormattedPrice,
						Currency:              lk.Currency,
						AverageRating:         lk.AverageUserRating,
						RatingCount:           lk.UserRatingCount,
						CurrentVersionAverage: lk.AverageUserRatingForCurrentVersion,
						CurrentVersionCount:   lk.UserRatingCountForCurrentVersion,
						Version:               lk.Version,
						VersionReleasedAt:     lk.CurrentVersionReleaseDate,
						FirstReleasedAt:       lk.ReleaseDate,
						SizeBytes:             size,
						MinimumOSVersion:      lk.MinimumOSVersion,
						ContentRating:         lk.TrackContentRating,
						Languages:             len(lk.LanguageCodesISO2A),
					})
					s2aWarnStore(cmd, "the "+cc+" ratings snapshot of "+id, s2aRecordLookupRatings(ctx, st, cc, lk, s2aIDFetchedAt(resp, ids, id)))
					s2aWarnStore(cmd, "the "+cc+" listing snapshot of "+id, s2aRecordLookupSnapshot(ctx, cmd, st, cc, lk, s2aIDFetchedAt(resp, ids, id)))
				}
			}
			meta.Requests = c.Requests()
			if stopped != nil && len(rows) == 0 {
				return s2aStopped(stopped)
			}
			if len(errs) == len(list) {
				return errs[0]
			}
			if len(rows) == 0 && len(errs) == 0 {
				return notFoundErr(fmt.Errorf("none of %s is sold in storefront(s) %s (check the ids, or try other --countries)", strings.Join(ids, ","), strings.Join(list, ",")))
			}
			if len(meta.Unavailable) > 0 {
				parts := make([]string, 0, len(meta.Unavailable))
				for _, m := range meta.Unavailable {
					parts = append(parts, m.App+"@"+m.Storefront)
				}
				meta.Notes = append(meta.Notes, "not sold: "+strings.Join(parts, ", "))
			}
			bundleRefs := 0
			for _, r := range refs {
				if r.ID == "" {
					bundleRefs++
				}
			}
			warnFetchFailures(cmd.ErrOrStderr(), len(failures), len(list)+bundleRefs, "lookups")
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				for _, n := range meta.Notes {
					s2aPrintln(cmd.ErrOrStderr(), "note: "+n)
				}
				if err := printJSONFiltered(cmd.OutOrStdout(), s2aEnvelope{Meta: meta, Items: rows, FetchFailures: failures}, flags); err != nil {
					return err
				}
				return s2aStopped(stopped)
			}
			if err := s2aPrintCompareHuman(cmd, list, rows, meta, failures); err != nil {
				return err
			}
			return s2aStopped(stopped)
		},
	}
	cmd.Flags().StringVar(&countries, "countries", "", "Storefronts to compare in, comma-separated (us,gb,jp) or all; empty uses --country")
	s2bDBFlag(cmd)
	return cmd
}

// s2aPrintCompareHuman prints one transposed table per storefront: fields as
// rows, apps as columns.
func s2aPrintCompareHuman(cmd *cobra.Command, storefronts []string, rows []compareRow, meta compareMeta, failures []fetchFailure) error {
	w := cmd.OutOrStdout()
	for _, cc := range storefronts {
		apps := make([]compareRow, 0)
		for _, r := range rows {
			if r.Storefront == cc {
				apps = append(apps, r)
			}
		}
		if len(apps) == 0 {
			continue
		}
		s2aPrintf(w, "\nStorefront %s\n", cc)
		headers := []string{"field"}
		for _, a := range apps {
			headers = append(headers, s2aOneLine(a.Title, 28))
		}
		field := func(name string, value func(compareRow) string) []string {
			out := []string{name}
			for _, a := range apps {
				out = append(out, value(a))
			}
			return out
		}
		table := [][]string{
			field("app id", func(r compareRow) string { return r.App }),
			field("developer", func(r compareRow) string { return s2aOneLine(r.Developer, 28) }),
			field("genre", func(r compareRow) string { return r.Genre }),
			field("price", func(r compareRow) string { return r.FormattedPrice }),
			field("average", func(r compareRow) string { return s2aRating(r.AverageRating) }),
			field("ratings", func(r compareRow) string { return s2aCount(r.RatingCount) }),
			field("current ver avg", func(r compareRow) string { return s2aRating(r.CurrentVersionAverage) }),
			field("current ver ratings", func(r compareRow) string { return s2aCount(r.CurrentVersionCount) }),
			field("version", func(r compareRow) string { return r.Version }),
			field("released", func(r compareRow) string { return s2aDay(r.VersionReleasedAt) }),
			field("first release", func(r compareRow) string { return s2aDay(r.FirstReleasedAt) }),
			field("size MB", func(r compareRow) string { return strconv.FormatFloat(float64(r.SizeBytes)/1e6, 'f', 1, 64) }),
			field("min OS", func(r compareRow) string { return r.MinimumOSVersion }),
			field("content rating", func(r compareRow) string { return r.ContentRating }),
			field("languages", func(r compareRow) string { return strconv.Itoa(r.Languages) }),
		}
		if err := s2aTable(w, headers, table); err != nil {
			return err
		}
	}
	for _, n := range meta.Notes {
		s2aPrintln(w, cliutil.ScrubTerminal(n))
	}
	s2aFailuresHuman(w, failures)
	return nil
}
