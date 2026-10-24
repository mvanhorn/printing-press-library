// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/cliutil"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newDeveloperCmd(flags))
	})
}

// developerLookupMax is the most apps Apple's developer lookup returns.
const developerLookupMax = 200

type developerInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	Storefront string `json:"storefront"`
}

// developerApp is one app from the developer Lookup (exact counts).
type developerApp struct {
	Position          int     `json:"position"`
	AppID             string  `json:"appId"`
	Title             string  `json:"title"`
	BundleID          string  `json:"bundleId"`
	Kind              string  `json:"kind"`
	Genre             string  `json:"genre"`
	Price             float64 `json:"price"`
	FormattedPrice    string  `json:"formattedPrice"`
	AverageRating     float64 `json:"averageRating"`
	RatingCount       int64   `json:"ratingCount"`
	Version           string  `json:"version"`
	VersionReleasedAt string  `json:"versionReleasedAt"`
	FirstReleasedAt   string  `json:"firstReleasedAt"`
}

// developerShelf is one developer-page shelf ("iPad & iPhone", "Mac",
// "Apple TV", "Apple Watch", "Latest Release").
type developerShelf struct {
	Title    string   `json:"title"`
	Platform string   `json:"platform"`
	Count    int      `json:"count"`
	AppIDs   []string `json:"appIds"`
}

// developerView is the developer command's JSON. appCount is every app the
// Lookup returned (before --limit); pageOnlyAppIds are apps on the page's
// shelves the Lookup did not return (Apple TV, Watch or other-platform apps).
type developerView struct {
	Developer      developerInfo    `json:"developer"`
	Platform       string           `json:"platform,omitempty"`
	AppCount       int              `json:"appCount"`
	Apps           []developerApp   `json:"apps"`
	Shelves        []developerShelf `json:"shelves"`
	PageOnlyAppIDs []string         `json:"pageOnlyAppIds"`
	Note           string           `json:"note,omitempty"`
	PageError      string           `json:"pageError,omitempty"`
}

var (
	developerURLIDRe      = regexp.MustCompile(`/developer/(?:[^/?#]+/)?id(\d{1,12})`)
	developerURLCountryRe = regexp.MustCompile(`apps\.apple\.com/([a-zA-Z]{2})/developer/`)
)

func newDeveloperCmd(flags *rootFlags) *cobra.Command {
	var platform string
	var limit int
	cmd := &cobra.Command{
		Use:   "developer <developer-id-or-app>",
		Short: "Fetch a developer's apps live (Lookup, up to 200) and per-platform shelves",
		Long: strings.Trim(`
List a developer's apps: the iTunes developer Lookup (up to 200 apps with exact
rating counts, price, version and dates; limit=200 is sent, unlike libraries
that get 50) plus the developer page's per-platform shelves (iPad & iPhone,
Mac, Apple TV, Apple Watch) with the apps each shows. Apps that appear only on
the page are listed in pageOnlyAppIds.

The argument is a developer id (324684583), a developer page URL, or any app
reference (324684580, com.spotify.client, an app URL), which resolves to its
developer. A numeric id is tried as a developer first, then as an app.
--platform ipad or mac asks the Lookup for that platform's apps and keeps only
that platform's shelves. Nothing is written to the local store.

Use this for a publisher's portfolio; 'similar' for apps Apple relates to one
app. An unknown id exits 3.`, "\n"),
		Example: strings.Trim(`
  app-store-pp-cli developer 324684583 --json
  app-store-pp-cli developer com.spotify.client --limit 10 --select developer,apps.title,apps.ratingCount
  app-store-pp-cli developer 284417353 --platform mac`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "live",
			"pp:typed-exit-codes": "0,7",
			"pp:happy-args":       "developer=324684583",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "developer")
			}
			if len(args) != 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("developer takes one developer id, developer URL or app reference, got %d arguments", len(args)))
			}
			if limit < 0 {
				return usageErr(fmt.Errorf("--limit must be 0 (all) or more; got %d", limit))
			}
			plat, entity := "", ""
			if strings.TrimSpace(platform) != "" {
				p, err := parsePlatform(platform)
				if err != nil {
					return err
				}
				switch p {
				case "iphone":
				case "ipad":
					entity = "iPadSoftware"
				case "mac":
					entity = "macSoftware"
				default:
					return usageErr(fmt.Errorf("--platform: developer listings support iphone, ipad and mac, not %q", platform))
				}
				plat = p
			}
			arg := strings.TrimSpace(args[0])
			devID, devCountry := "", ""
			var ref appstore.AppRef
			if m := developerURLIDRe.FindStringSubmatch(arg); m != nil && strings.Contains(strings.ToLower(arg), "apple.com") {
				devID = m[1]
				if cm := developerURLCountryRe.FindStringSubmatch(arg); cm != nil {
					if cc, err := appstore.NormalizeCountry(cm[1]); err == nil {
						devCountry = cc
					}
				}
			} else {
				r, err := parseAppRef(arg)
				if err != nil {
					return usageErr(fmt.Errorf("unrecognized developer %q: pass a developer id (324684583), a developer URL or an app reference", arg))
				}
				ref = r
			}
			cc := ""
			if devID != "" {
				cc = devCountry
				if cc == "" || flagChanged(cmd, "country") {
					c2, err := appStoreCountry(flags)
					if err != nil {
						return err
					}
					cc = c2
				}
			} else {
				c2, err := refCountry(cmd, flags, ref)
				if err != nil {
					return err
				}
				cc = c2
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

			devResp, pageURL, err := s2aResolveDeveloper(ctx, c, devID, ref, cc, entity)
			if err != nil {
				return err
			}
			view := developerView{
				Platform:       plat,
				Apps:           make([]developerApp, 0),
				Shelves:        make([]developerShelf, 0),
				PageOnlyAppIDs: make([]string, 0),
			}
			view.Developer = developerInfo{Storefront: cc}
			if devResp.Artist != nil {
				view.Developer.ID = strconv.FormatInt(devResp.Artist.ArtistID, 10)
				view.Developer.Name = devResp.Artist.ArtistName
				view.Developer.URL = appstore.CanonicalAppURL(devResp.Artist.ArtistLinkURL)
			}
			if view.Developer.ID == "" || view.Developer.ID == "0" {
				view.Developer.ID = s2aDeveloperIDOf(devResp, devID)
			}
			if view.Developer.Name == "" && len(devResp.Results) > 0 {
				view.Developer.Name = devResp.Results[0].ArtistName
			}
			if view.Developer.URL == "" {
				view.Developer.URL = appstore.CanonicalAppURL(pageURL)
			}
			lookupIDs := map[string]bool{}
			for i, r := range devResp.Results {
				lookupIDs[r.AppID()] = true
				if limit > 0 && len(view.Apps) >= limit {
					continue
				}
				view.Apps = append(view.Apps, developerApp{
					Position:          i + 1,
					AppID:             r.AppID(),
					Title:             r.TrackName,
					BundleID:          r.BundleID,
					Kind:              r.Kind,
					Genre:             r.PrimaryGenreName,
					Price:             r.Price,
					FormattedPrice:    r.FormattedPrice,
					AverageRating:     r.AverageUserRating,
					RatingCount:       r.UserRatingCount,
					Version:           r.Version,
					VersionReleasedAt: r.CurrentVersionReleaseDate,
					FirstReleasedAt:   r.ReleaseDate,
				})
			}
			view.AppCount = len(devResp.Results)

			// stopped: a throttle or pacing refusal kept the page out; the Lookup
			// apps still print, then exit 7.
			var stopped error
			page, perr := c.DeveloperPage(ctx, cc, view.Developer.ID, pageURL)
			if perr != nil {
				perr = classifyAppStoreErr(perr)
				if isRateLimited(perr) {
					stopped = perr
				}
				view.PageError = perr.Error()
				s2aPrintf(cmd.ErrOrStderr(), "warning: developer page unavailable (%v); showing Lookup apps only\n", perr)
			} else {
				if view.Developer.Name == "" {
					view.Developer.Name = page.Name
				}
				if view.Developer.URL == "" {
					view.Developer.URL = page.URL
				}
				pageOnly := map[string]bool{}
				for _, s := range page.Shelves {
					if plat != "" && !s2aShelfMatches(s.Platform, plat) {
						continue
					}
					ids := make([]string, 0, len(s.Apps))
					for _, a := range s.Apps {
						ids = append(ids, a.AdamID)
						if !lookupIDs[a.AdamID] && !pageOnly[a.AdamID] {
							pageOnly[a.AdamID] = true
							view.PageOnlyAppIDs = append(view.PageOnlyAppIDs, a.AdamID)
						}
					}
					view.Shelves = append(view.Shelves, developerShelf{Title: s.Title, Platform: s.Platform, Count: len(s.Apps), AppIDs: ids})
				}
			}
			switch {
			case view.AppCount == 0 && plat != "":
				view.Note = fmt.Sprintf("Apple's developer Lookup lists no %s apps for developer %s in storefront %s", plat, view.Developer.ID, cc)
			case view.AppCount == 0:
				view.Note = fmt.Sprintf("Apple's developer Lookup lists no apps for developer %s in storefront %s", view.Developer.ID, cc)
			case view.AppCount >= developerLookupMax:
				view.Note = fmt.Sprintf("Apple's developer Lookup returns at most %d apps; this developer may have more", developerLookupMax)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if view.Note != "" {
					s2aPrintln(cmd.ErrOrStderr(), "note: "+view.Note)
				}
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
				return s2aStopped(stopped)
			}
			w := cmd.OutOrStdout()
			s2aPrintf(w, "%s (developer %s, storefront %s): %d apps\n", cliutil.ScrubTerminal(view.Developer.Name), view.Developer.ID, cc, view.AppCount)
			if len(view.Apps) > 0 {
				table := make([][]string, 0, len(view.Apps))
				for _, a := range view.Apps {
					table = append(table, []string{a.AppID, s2aOneLine(a.Title, 40), a.Genre, a.FormattedPrice, s2aRating(a.AverageRating), s2aCount(a.RatingCount), a.Version})
				}
				if err := s2aTable(w, []string{"app id", "title", "genre", "price", "rating", "ratings", "version"}, table); err != nil {
					return err
				}
			}
			for _, s := range view.Shelves {
				s2aPrintf(w, "shelf %q (%s): %d apps\n", s.Title, s.Platform, s.Count)
			}
			if len(view.PageOnlyAppIDs) > 0 {
				s2aPrintf(w, "on the page only: %s\n", strings.Join(view.PageOnlyAppIDs, ", "))
			}
			if view.Note != "" {
				s2aPrintln(w, view.Note)
			}
			return s2aStopped(stopped)
		},
	}
	cmd.Flags().StringVar(&platform, "platform", "", "Limit to one platform's apps and shelves: iphone, ipad or mac (default: all shelves, iPhone/universal apps)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum apps to list (0 = all the Lookup returned, up to 200)")
	s2bDBFlag(cmd)
	return cmd
}

// s2aResolveDeveloper finds the developer Lookup for an argument. devID set
// means a developer URL; otherwise ref is an app reference or a numeric id
// that may be either: it is tried as a developer first (one request), then as
// an app whose developer is looked up. Returns the developer Lookup and the
// developer page URL (Apple's artist link when known).
func s2aResolveDeveloper(ctx context.Context, c *appstore.Client, devID string, ref appstore.AppRef, cc, entity string) (*appstore.LookupResponse, string, error) {
	devLookup := func(id string) (*appstore.LookupResponse, error) {
		resp, err := c.Lookup(ctx, appstore.LookupQuery{ArtistID: id, Country: cc, Entity: entity})
		if err != nil {
			return nil, classifyAppStoreErr(err)
		}
		return resp, nil
	}
	pageURLOf := func(resp *appstore.LookupResponse, fallback string) string {
		if resp != nil && resp.Artist != nil && resp.Artist.ArtistLinkURL != "" {
			return resp.Artist.ArtistLinkURL
		}
		return fallback
	}
	if devID != "" {
		resp, err := devLookup(devID)
		if err != nil {
			return nil, "", err
		}
		if resp.Artist == nil && len(resp.Results) == 0 {
			return nil, "", notFoundErr(fmt.Errorf("no App Store developer %s in storefront %s (check the id, or try another --country)", devID, cc))
		}
		return resp, pageURLOf(resp, ""), nil
	}
	var appRow *appstore.LookupResult
	if ref.ID != "" && !strings.Contains(strings.ToLower(ref.Raw), "/app/") {
		// A bare number: a developer id answers with its artist row.
		resp, err := devLookup(ref.ID)
		if err != nil {
			return nil, "", err
		}
		if resp.Artist != nil {
			return resp, pageURLOf(resp, ""), nil
		}
		for i := range resp.Results {
			if resp.Results[i].AppID() == ref.ID {
				appRow = &resp.Results[i]
				break
			}
		}
	}
	if appRow == nil {
		lk, err := lookupAppRef(ctx, c, ref, cc)
		if err != nil {
			if ExitCode(err) == 3 {
				return nil, "", notFoundErr(fmt.Errorf("no App Store developer or app %s in storefront %s (check the id, or try another --country)", s2aRefLabel(ref), cc))
			}
			return nil, "", err
		}
		appRow = &lk
	}
	if appRow.ArtistID == 0 {
		return nil, "", notFoundErr(fmt.Errorf("app %s has no developer id in storefront %s", appRow.AppID(), cc))
	}
	artistID := strconv.FormatInt(appRow.ArtistID, 10)
	resp, err := devLookup(artistID)
	if err != nil {
		return nil, "", err
	}
	if resp.Artist == nil && len(resp.Results) == 0 {
		return nil, "", notFoundErr(fmt.Errorf("no App Store developer %s in storefront %s", artistID, cc))
	}
	return resp, pageURLOf(resp, appRow.ArtistViewURL), nil
}

// s2aDeveloperIDOf returns the developer id from a developer Lookup.
func s2aDeveloperIDOf(resp *appstore.LookupResponse, fallback string) string {
	if resp != nil && resp.Artist != nil && resp.Artist.ArtistID != 0 {
		return strconv.FormatInt(resp.Artist.ArtistID, 10)
	}
	if resp != nil {
		for _, r := range resp.Results {
			if r.ArtistID != 0 {
				return strconv.FormatInt(r.ArtistID, 10)
			}
		}
	}
	return fallback
}

// s2aShelfMatches reports whether a developer-page shelf belongs to the
// --platform filter. The page groups iPhone and iPad apps on one shelf.
func s2aShelfMatches(shelfPlatform, want string) bool {
	if want == "ipad" {
		want = "iphone"
	}
	return shelfPlatform == want
}
