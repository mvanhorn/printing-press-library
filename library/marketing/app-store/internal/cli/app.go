// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newAppCmd(flags))
	})
}

// appView is the app command's JSON: the iTunes Lookup fields (Apple's key
// names; description and screenshot URL lists only with --full) merged with
// what only the product page shows. ratingCount/averageRating repeat the
// exact Lookup values under the names the other commands use; totalRatings
// and histogram come from the page (cached separately, so they can differ
// slightly from the Lookup count). version, currentVersionReleaseDate and
// releaseNotes come from the page's newest version entry when the page's
// data is newer than an older Lookup copy naming another version
// (appstore.CurrentVersion); lookupDataAsOf says when Apple produced the
// Lookup copy, and notes explain a copy an hour or more old.
type appView struct {
	TrackID                            int64                   `json:"trackId"`
	TrackName                          string                  `json:"trackName"`
	Subtitle                           string                  `json:"subtitle"`
	BundleID                           string                  `json:"bundleId"`
	Storefront                         string                  `json:"storefront"`
	URL                                string                  `json:"url"`
	Kind                               string                  `json:"kind,omitempty"`
	ArtistID                           int64                   `json:"artistId"`
	ArtistName                         string                  `json:"artistName"`
	ArtistViewURL                      string                  `json:"artistViewUrl,omitempty"`
	SellerName                         string                  `json:"sellerName,omitempty"`
	SellerURL                          string                  `json:"sellerUrl,omitempty"`
	Price                              float64                 `json:"price"`
	FormattedPrice                     string                  `json:"formattedPrice"`
	Currency                           string                  `json:"currency"`
	HasInAppPurchases                  bool                    `json:"hasInAppPurchases"`
	AverageRating                      float64                 `json:"averageRating"`
	RatingCount                        int64                   `json:"ratingCount"`
	AverageUserRating                  float64                 `json:"averageUserRating"`
	UserRatingCount                    int64                   `json:"userRatingCount"`
	AverageUserRatingForCurrentVersion float64                 `json:"averageUserRatingForCurrentVersion"`
	UserRatingCountForCurrentVersion   int64                   `json:"userRatingCountForCurrentVersion"`
	TotalRatings                       int64                   `json:"totalRatings"`
	Histogram                          []int64                 `json:"histogram"`
	EditorsChoice                      bool                    `json:"editorsChoice"`
	ChartPosition                      *appstore.ChartPosition `json:"chartPosition"`
	FeaturedIn                         []appstore.FeaturedCard `json:"featuredIn"`
	Version                            string                  `json:"version"`
	CurrentVersionReleaseDate          string                  `json:"currentVersionReleaseDate,omitempty"`
	ReleaseDate                        string                  `json:"releaseDate,omitempty"`
	ReleaseNotes                       string                  `json:"releaseNotes,omitempty"`
	PrimaryGenreID                     int64                   `json:"primaryGenreId"`
	PrimaryGenreName                   string                  `json:"primaryGenreName"`
	GenreIDs                           []string                `json:"genreIds"`
	Genres                             []string                `json:"genres"`
	ContentAdvisoryRating              string                  `json:"contentAdvisoryRating,omitempty"`
	TrackContentRating                 string                  `json:"trackContentRating,omitempty"`
	Advisories                         []string                `json:"advisories"`
	MinimumOSVersion                   string                  `json:"minimumOsVersion,omitempty"`
	FileSizeBytes                      string                  `json:"fileSizeBytes,omitempty"`
	LanguageCodesISO2A                 []string                `json:"languageCodesISO2A"`
	SupportedDevices                   []string                `json:"supportedDevices"`
	AppPlatforms                       []string                `json:"appPlatforms"`
	Features                           []string                `json:"features"`
	IsGameCenterEnabled                bool                    `json:"isGameCenterEnabled"`
	IsVppDeviceBasedLicensingEnabled   bool                    `json:"isVppDeviceBasedLicensingEnabled"`
	ArtworkURL512                      string                  `json:"artworkUrl512,omitempty"`
	ArtworkURL100                      string                  `json:"artworkUrl100,omitempty"`
	ArtworkURL60                       string                  `json:"artworkUrl60,omitempty"`
	TrackViewURL                       string                  `json:"trackViewUrl"`
	ScreenshotCount                    int                     `json:"screenshotCount"`
	LookupDataAsOf                     string                  `json:"lookupDataAsOf,omitempty"`
	Notes                              []string                `json:"notes,omitempty"`
	PageError                          string                  `json:"pageError,omitempty"`

	// --full only (absent otherwise; [] / empty objects when the page has none).
	Description     *string                  `json:"description,omitempty"`
	Versions        *[]appstore.VersionEntry `json:"versions,omitempty"`
	Privacy         *appstore.Privacy        `json:"privacy,omitempty"`
	InAppPurchases  *[]appstore.IAPItem      `json:"inAppPurchases,omitempty"`
	Events          *[]appstore.AppEvent     `json:"events,omitempty"`
	Capabilities    *[]appstore.Capability   `json:"capabilities,omitempty"`
	AgeRating       *appstore.AgeRating      `json:"ageRating,omitempty"`
	Accessibility   *appstore.Accessibility  `json:"accessibility,omitempty"`
	Similar         *[]appstore.AppLockup    `json:"similar,omitempty"`
	MoreByDeveloper *[]appstore.AppLockup    `json:"moreByDeveloper,omitempty"`
	Screenshots     *[]appstore.MediaSet     `json:"screenshots,omitempty"`
}

func newAppCmd(flags *rootFlags) *cobra.Command {
	var full bool
	var platform string
	cmd := &cobra.Command{
		Use:   "app <app>",
		Short: "Get an app's live listing: Lookup data, subtitle, histogram, chart badge, Editor's Choice",
		Long: strings.Trim(`
Show one app's App Store listing in one storefront: the iTunes Lookup fields
(price, exact rating count, version, genres, size, languages, devices) merged
with what only the product page shows: subtitle, Editor's Choice, the
5-star-first rating histogram, the chart badge ("#1 in Music"), the in-app
purchase flag and Featured In stories. --full adds the description, version
history, privacy labels, in-app purchase prices, in-app events, capabilities,
age rating, accessibility, similar apps, more apps by the developer and
screenshots per device.

<app> is a numeric id (324684580), id324684580, an App Store URL (its
storefront is used unless --country is set) or a bundle id
(com.spotify.client). Costs two requests (Lookup + product page); repeats
within 15 minutes come from the local cache. Each fresh page is recorded as a
listing snapshot for watch-listing and changes.

Use this for one app's current listing. For several apps side by side use
'compare', for ratings across storefronts 'ratings', for the version list
alone 'versions'. An unknown id exits 3.`, "\n"),
		Example: strings.Trim(`
  app-store-pp-cli app 324684580 --json
  app-store-pp-cli app com.spotify.client --select trackName,subtitle,ratingCount,histogram
  app-store-pp-cli app 361309726 --full --country gb`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "live",
			"pp:typed-exit-codes": "0,7",
			"pp:happy-args":       "app=324684580",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "app")
			}
			ref, err := s2aOneAppArg(cmd, args)
			if err != nil {
				return err
			}
			plat := ""
			if strings.TrimSpace(platform) != "" {
				if plat, err = parsePlatform(platform); err != nil {
					return err
				}
			}
			cc, err := refCountry(cmd, flags, ref)
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

			lk, err := lookupAppRef(ctx, c, ref, cc)
			if err != nil {
				return err
			}
			page, pageErr := s2aProductPage(ctx, c, lk, cc, plat)
			// stopped: a throttle or pacing refusal kept the page out; the
			// Lookup fields still print, then exit 7.
			var stopped error
			if pageErr != nil {
				if isRateLimited(pageErr) {
					stopped = pageErr
				}
				s2aPrintf(cmd.ErrOrStderr(), "warning: product page unavailable (%v); showing Lookup fields only\n", pageErr)
			} else {
				s2aWarnStore(cmd, "the listing snapshot", s2aRecordProductPage(ctx, cmd, st, cc, page, lk))
			}
			view := s2aBuildAppView(cc, lk, page, full)
			if pageErr != nil {
				view.PageError = pageErr.Error()
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
				return s2aStopped(stopped)
			}
			if err := s2aPrintAppHuman(cmd.OutOrStdout(), view); err != nil {
				return err
			}
			return s2aStopped(stopped)
		},
	}
	cmd.Flags().BoolVar(&full, "full", false, "Add description, versions, privacy, IAP prices, events, capabilities, age rating, accessibility, similar apps and screenshots")
	cmd.Flags().StringVar(&platform, "platform", "", "Product page platform: iphone (default), ipad, mac, appletv, watch or vision")
	s2bDBFlag(cmd)
	return cmd
}

// s2aBuildAppView merges a Lookup row with its product page (page may be nil
// when the page could not be fetched).
func s2aBuildAppView(storefront string, lk appstore.LookupResult, page *appstore.ProductPage, full bool) appView {
	v := appView{
		TrackID:                            lk.TrackID,
		TrackName:                          lk.TrackName,
		BundleID:                           lk.BundleID,
		Storefront:                         storefront,
		URL:                                lk.CanonicalURL(),
		Kind:                               lk.Kind,
		ArtistID:                           lk.ArtistID,
		ArtistName:                         lk.ArtistName,
		ArtistViewURL:                      appstore.CanonicalAppURL(lk.ArtistViewURL),
		SellerName:                         lk.SellerName,
		SellerURL:                          lk.SellerURL,
		Price:                              lk.Price,
		FormattedPrice:                     lk.FormattedPrice,
		Currency:                           lk.Currency,
		AverageRating:                      lk.AverageUserRating,
		RatingCount:                        lk.UserRatingCount,
		AverageUserRating:                  lk.AverageUserRating,
		UserRatingCount:                    lk.UserRatingCount,
		AverageUserRatingForCurrentVersion: lk.AverageUserRatingForCurrentVersion,
		UserRatingCountForCurrentVersion:   lk.UserRatingCountForCurrentVersion,
		Histogram:                          make([]int64, 0, 5),
		FeaturedIn:                         make([]appstore.FeaturedCard, 0),
		Version:                            lk.Version,
		CurrentVersionReleaseDate:          lk.CurrentVersionReleaseDate,
		ReleaseDate:                        lk.ReleaseDate,
		ReleaseNotes:                       lk.ReleaseNotes,
		PrimaryGenreID:                     lk.PrimaryGenreID,
		PrimaryGenreName:                   lk.PrimaryGenreName,
		GenreIDs:                           s2aStrings(lk.GenreIDs),
		Genres:                             s2aStrings(lk.Genres),
		ContentAdvisoryRating:              lk.ContentAdvisoryRating,
		TrackContentRating:                 lk.TrackContentRating,
		Advisories:                         s2aStrings(lk.Advisories),
		MinimumOSVersion:                   lk.MinimumOSVersion,
		FileSizeBytes:                      lk.FileSizeBytes,
		LanguageCodesISO2A:                 s2aStrings(lk.LanguageCodesISO2A),
		SupportedDevices:                   s2aStrings(lk.SupportedDevices),
		AppPlatforms:                       make([]string, 0),
		Features:                           s2aStrings(lk.Features),
		IsGameCenterEnabled:                lk.IsGameCenterEnabled,
		IsVppDeviceBasedLicensingEnabled:   lk.IsVppDeviceBasedLicensingEnabled,
		ArtworkURL512:                      lk.ArtworkURL512,
		ArtworkURL100:                      lk.ArtworkURL100,
		ArtworkURL60:                       lk.ArtworkURL60,
		TrackViewURL:                       lk.TrackViewURL,
		ScreenshotCount:                    len(lk.ScreenshotURLs) + len(lk.IPadScreenshotURLs) + len(lk.AppleTVScreenshotURLs),
	}
	lookupMedia := s2aLookupMedia(lk)
	if page != nil {
		v.Subtitle = page.Subtitle
		v.HasInAppPurchases = page.HasInAppPurchases || len(page.InAppPurchases) > 0
		v.TotalRatings = page.TotalRatings
		v.Histogram = append(v.Histogram, page.Histogram...)
		v.EditorsChoice = page.EditorsChoice
		v.ChartPosition = page.ChartPosition
		v.FeaturedIn = append(v.FeaturedIn, page.FeaturedIn...)
		v.AppPlatforms = append(v.AppPlatforms, page.AppPlatforms...)
		if page.CanonicalURL != "" && v.URL == "" {
			v.URL = page.CanonicalURL
		}
		if n := page.ScreenshotCount(); n > 0 {
			v.ScreenshotCount = n
		}
	}
	cv := appstore.CurrentVersion(lk, page)
	v.Version, v.CurrentVersionReleaseDate, v.ReleaseNotes = cv.Version, cv.ReleasedAt, cv.Notes
	v.LookupDataAsOf = appstore.FormatTime(lk.DataAsOf)
	v.Notes = s2aLookupCopyNotes(lk, page, cv)
	if !full {
		return v
	}
	desc := lk.Description
	if page != nil && page.Description != "" {
		desc = page.Description
	}
	v.Description = &desc
	media := lookupMedia
	if page != nil && len(page.Media) > 0 {
		media = page.Media
	}
	v.Screenshots = &media
	if page == nil {
		return v
	}
	versions := append(make([]appstore.VersionEntry, 0, len(page.Versions)), page.Versions...)
	iaps := append(make([]appstore.IAPItem, 0, len(page.InAppPurchases)), page.InAppPurchases...)
	events := append(make([]appstore.AppEvent, 0, len(page.Events)), page.Events...)
	caps := append(make([]appstore.Capability, 0, len(page.Capabilities)), page.Capabilities...)
	similar := append(make([]appstore.AppLockup, 0, len(page.SimilarItems)), page.SimilarItems...)
	more := append(make([]appstore.AppLockup, 0, len(page.MoreByDeveloper)), page.MoreByDeveloper...)
	privacy := page.Privacy
	if privacy.Types == nil {
		privacy.Types = make([]appstore.PrivacyType, 0)
	}
	acc := page.Accessibility
	if acc.Features == nil {
		acc.Features = make([]string, 0)
	}
	v.Versions = &versions
	v.InAppPurchases = &iaps
	v.Events = &events
	v.Capabilities = &caps
	v.Similar = &similar
	v.MoreByDeveloper = &more
	v.Privacy = &privacy
	if page.Privacy.Unreadable {
		// A parse gap is not "no labels": say so instead of an empty list.
		v.Notes = append(v.Notes, "the product page has privacy labels this version cannot read (Apple may have changed the page); privacy.privacyTypes is empty for that reason, not because the app has none")
	}
	v.Accessibility = &acc
	v.AgeRating = page.AgeRating
	return v
}

// s2aStaleLookupCopy is the Lookup copy age from which the app view adds a
// note: Apple's Lookup CDN serves per-URL copies up to 24 h old.
const s2aStaleLookupCopy = time.Hour

// s2aLookupCopyNotes explains where the app view's fields come from when the
// Lookup copy is an hour or more older than its fetch, or when the newer
// product page replaced its version. Nil when there is nothing to say.
func s2aLookupCopyNotes(lk appstore.LookupResult, page *appstore.ProductPage, cv appstore.ListingVersion) []string {
	var notes []string
	if cv.FromPage {
		note := fmt.Sprintf("version %s comes from the product page", cv.Version)
		if cv.LookupVersion != "" {
			note += fmt.Sprintf(", whose data is newer than the Lookup copy (which still lists %s)", cv.LookupVersion)
		}
		notes = append(notes, note)
	}
	if !lk.DataAsOf.IsZero() && !lk.FetchedAt.IsZero() {
		if age := lk.FetchedAt.Sub(lk.DataAsOf); age >= s2aStaleLookupCopy {
			note := fmt.Sprintf("the Lookup fields (ratingCount, userRatingCount, averageRating and the rest) come from Apple's CDN copy of %s, %s older than this fetch",
				appstore.FormatTime(lk.DataAsOf), staleAge(age))
			if page != nil && page.DataAsOf.After(lk.DataAsOf) {
				note += "; totalRatings and histogram come from the newer product page"
			}
			notes = append(notes, note)
		}
	}
	return notes
}

// s2aLookupMedia groups the Lookup screenshot lists per device, the fallback
// when the page has no media shelves.
func s2aLookupMedia(lk appstore.LookupResult) []appstore.MediaSet {
	out := make([]appstore.MediaSet, 0, 3)
	for _, set := range []struct {
		device string
		urls   []string
	}{{"phone", lk.ScreenshotURLs}, {"pad", lk.IPadScreenshotURLs}, {"tv", lk.AppleTVScreenshotURLs}} {
		if len(set.urls) > 0 {
			out = append(out, appstore.MediaSet{Device: set.device, Screenshots: append([]string(nil), set.urls...)})
		}
	}
	return out
}

// s2aStrings returns a non-nil copy so empty lists marshal as [].
func s2aStrings(in []string) []string {
	return append(make([]string, 0, len(in)), in...)
}

func s2aPrintAppHuman(w io.Writer, v appView) error {
	line := func(label, value string) {
		if strings.TrimSpace(value) != "" {
			s2aPrintf(w, "%-16s %s\n", label+":", cliutil.ScrubTerminal(value))
		}
	}
	title := v.TrackName
	if v.Subtitle != "" {
		title += " — " + v.Subtitle
	}
	s2aPrintln(w, cliutil.ScrubTerminal(title))
	line("App id", fmt.Sprintf("%d (%s)", v.TrackID, v.BundleID))
	line("Developer", fmt.Sprintf("%s (%d)", v.ArtistName, v.ArtistID))
	line("Storefront", v.Storefront)
	price := v.FormattedPrice
	if v.HasInAppPurchases {
		price += " · in-app purchases"
	}
	line("Price", price)
	line("Rating", fmt.Sprintf("%s from %s ratings", s2aRating(v.AverageRating), s2aCount(v.RatingCount)))
	if len(v.Histogram) == 5 {
		parts := make([]string, 0, 5)
		for i, n := range v.Histogram {
			parts = append(parts, fmt.Sprintf("%d★ %s", 5-i, s2aCount(n)))
		}
		line("Histogram", strings.Join(parts, " · "))
	}
	line("Version", strings.TrimSpace(v.Version+" "+s2aDay(v.CurrentVersionReleaseDate)))
	line("Genre", v.PrimaryGenreName)
	if v.ChartPosition != nil {
		line("Chart", fmt.Sprintf("#%d in %s (%s)", v.ChartPosition.Position, v.ChartPosition.GenreName, v.ChartPosition.Chart))
	}
	if v.EditorsChoice {
		line("Editors' Choice", "yes")
	}
	if len(v.FeaturedIn) > 0 {
		titles := make([]string, 0, len(v.FeaturedIn))
		for _, f := range v.FeaturedIn {
			titles = append(titles, f.Title)
		}
		line("Featured in", strings.Join(titles, "; "))
	}
	line("URL", v.URL)
	if v.PageError != "" {
		line("Page", "unavailable: "+v.PageError)
	}
	for _, n := range v.Notes {
		line("Note", n)
	}
	if v.Description == nil {
		return nil
	}
	if v.Accessibility != nil {
		acc := "not declared by the developer"
		if v.Accessibility.Declared {
			acc = "declared: " + strings.Join(v.Accessibility.Features, ", ")
		}
		line("Accessibility", acc)
	}
	if v.AgeRating != nil {
		line("Age rating", v.AgeRating.Rating)
	}
	if v.Capabilities != nil && len(*v.Capabilities) > 0 {
		names := make([]string, 0, len(*v.Capabilities))
		for _, c := range *v.Capabilities {
			names = append(names, c.Title)
		}
		line("Supports", strings.Join(names, ", "))
	}
	if v.Privacy != nil {
		names := make([]string, 0, len(v.Privacy.Types))
		for _, t := range v.Privacy.Types {
			names = append(names, t.Title)
		}
		line("Privacy", strings.Join(names, "; "))
	}
	if v.InAppPurchases != nil && len(*v.InAppPurchases) > 0 {
		s2aPrintln(w, "\nIn-app purchases:")
		rows := make([][]string, 0, len(*v.InAppPurchases))
		for _, it := range *v.InAppPurchases {
			rows = append(rows, []string{fmt.Sprint(it.Position), it.Name, it.Price})
		}
		_ = s2aTable(w, []string{"#", "name", "price"}, rows)
	}
	if v.Events != nil && len(*v.Events) > 0 {
		s2aPrintln(w, "\nIn-app events:")
		for _, e := range *v.Events {
			s2aPrintf(w, "  %s (%s) %s to %s\n", cliutil.ScrubTerminal(e.Title), e.Kind, s2aDay(e.StartDate), s2aDay(e.EndDate))
		}
	}
	if v.Versions != nil && len(*v.Versions) > 0 {
		s2aPrintln(w, "\nRecent versions:")
		rows := make([][]string, 0, 5)
		for i, ve := range *v.Versions {
			if i == 5 {
				break
			}
			rows = append(rows, []string{ve.Version, s2aDay(ve.ReleasedAt), s2aOneLine(ve.Notes, 70)})
		}
		_ = s2aTable(w, []string{"version", "released", "notes"}, rows)
	}
	if v.Similar != nil && len(*v.Similar) > 0 {
		names := make([]string, 0, len(*v.Similar))
		for _, s := range *v.Similar {
			names = append(names, s.Title)
		}
		s2aPrintf(w, "\nSimilar: %s\n", cliutil.ScrubTerminal(strings.Join(names, "; ")))
	}
	if v.Screenshots != nil {
		counts := make([]string, 0, len(*v.Screenshots))
		for _, m := range *v.Screenshots {
			counts = append(counts, fmt.Sprintf("%s %d", m.Device, len(m.Screenshots)))
		}
		if len(counts) > 0 {
			s2aPrintf(w, "Screenshots: %s\n", strings.Join(counts, ", "))
		}
	}
	if *v.Description != "" {
		s2aPrintf(w, "\n%s\n", cliutil.EscapeTerminalStream(*v.Description))
	}
	return nil
}
