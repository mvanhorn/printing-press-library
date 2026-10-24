// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/appstore"
	"github.com/mvanhorn/printing-press-library/library/marketing/app-store/internal/store"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newFeaturedCmd(flags))
	})
}

// featuredApp is one app featured by a Today story.
type featuredApp struct {
	AppID string `json:"appId"`
	Title string `json:"title"`
}

// featuredStory is one Today story in display order.
type featuredStory struct {
	Position int           `json:"position"`
	StoryID  string        `json:"storyId"`
	Title    string        `json:"title"`
	Heading  string        `json:"heading,omitempty"`
	Apps     []featuredApp `json:"apps"`
}

type featuredMeta struct {
	Storefront   string   `json:"storefront"`
	Platform     string   `json:"platform"`
	SourceURL    string   `json:"sourceUrl"`
	CapturedAt   string   `json:"capturedAt"`
	FromCache    bool     `json:"fromCache"`
	Stories      int      `json:"stories"`
	Returned     int      `json:"returned"`
	DistinctApps int      `json:"distinctApps"`
	Recorded     bool     `json:"recorded"`
	Notes        []string `json:"notes,omitempty"`
}

// featuredPlatform validates --platform: the web Today tab exists for the
// iPhone and iPad stores.
func featuredPlatform(raw string) (string, error) {
	p, err := parsePlatform(raw)
	if err != nil {
		return "", err
	}
	if p != "iphone" && p != "ipad" {
		return "", usageErr(fmt.Errorf("--platform: the Today tab exists for iphone and ipad, not %q", raw))
	}
	return p, nil
}

func newFeaturedCmd(flags *rootFlags) *cobra.Command {
	var platform string
	var limit int
	cmd := &cobra.Command{
		Use:   "featured",
		Short: "List today's App Store editorial stories and the apps they feature, and record them",
		Long: strings.Trim(`
Read the App Store Today tab for a storefront (--country) from the
robots-allowed apps.apple.com Today page: every editorial story in display
order with the apps it features. Each capture is stored in the local
featured_snapshots table, so the capture date is the featuring date for
changes, impact and traction. --limit trims the output only; every story is
stored. A page served from the local cache (15 minutes) is stored once, at
its original fetch time.`, "\n"),
		Example: strings.Trim(`
  app-store-pp-cli featured --json
  app-store-pp-cli featured --country gb --limit 5
  app-store-pp-cli featured --platform ipad --agent`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "live",
			"pp:typed-exit-codes": "0,7",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "featured")
			}
			if len(args) > 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("featured takes no positional arguments (got %q); pick the storefront with --country", args[0]))
			}
			p, err := featuredPlatform(platform)
			if err != nil {
				return err
			}
			if limit < 0 {
				return usageErr(fmt.Errorf("--limit must be 0 (all) or more (got %d)", limit))
			}
			cc, err := appStoreCountry(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, st, err := s2bOpenLive(cmd, flags)
			if err != nil {
				return err
			}
			defer func() { _ = st.Close() }()
			return runFeatured(ctx, cmd, flags, c, st, cc, p, limit)
		},
	}
	cmd.Flags().StringVar(&platform, "platform", "iphone", "Today tab to read: iphone or ipad")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum stories to return (0 = all); every story is still stored")
	s2bDBFlag(cmd)
	return cmd
}

func runFeatured(ctx context.Context, cmd *cobra.Command, flags *rootFlags, c *appstore.Client, st *store.Store, cc, platform string, limit int) error {
	pageURL := appstore.TodayPageURL(cc, platform)
	meta := featuredMeta{Storefront: cc, Platform: platform, SourceURL: pageURL, Notes: make([]string, 0)}
	model, err := c.PageOfKind(ctx, pageURL, s2bTodayKind)
	if err != nil {
		return classifyAppStoreErr(err)
	}
	page, err := appstore.ParseTodayPage(model)
	if err != nil {
		return apiErr(fmt.Errorf("parsing the Today page %s: %w", pageURL, err))
	}
	fetchedAt := s2bNowIfZero(model.FetchedAt)
	meta.CapturedAt, meta.FromCache = s2bTime(fetchedAt), model.FromCache
	meta.Stories = len(page.Stories)

	stories := make([]featuredStory, 0, len(page.Stories))
	snap := store.FeaturedSnapshot{CapturedAt: fetchedAt, Storefront: cc, Platform: platform, Stories: make([]store.FeaturedStory, 0, len(page.Stories))}
	distinct := map[string]bool{}
	for _, s := range page.Stories {
		fs := featuredStory{Position: s.Position, StoryID: s.StoryID, Title: s.Title, Heading: s.Heading, Apps: make([]featuredApp, 0, len(s.Apps))}
		for _, a := range s.Apps {
			fs.Apps = append(fs.Apps, featuredApp{AppID: a.AdamID, Title: a.Title})
			distinct[a.AdamID] = true
		}
		stories = append(stories, fs)
		snap.Stories = append(snap.Stories, store.FeaturedStory{Position: s.Position, StoryID: s.StoryID, Title: s.Title, AppIDs: s.AppIDs()})
	}
	meta.DistinctApps = len(distinct)
	if len(stories) == 0 {
		meta.Notes = append(meta.Notes, fmt.Sprintf("the Today page for %s/%s listed no stories (verified empty); nothing was recorded", cc, platform))
		meta.Returned = 0
		return printFeatured(cmd, flags, meta, stories)
	}
	// The store writes each observation once: a Today page served from the
	// local cache keeps its fetch time and is already stored.
	recorded, err := store.RecordFeaturedOnce(s2bWriteCtx(ctx), st, snap)
	if err != nil {
		s2bStoreFailed(cmd, &meta.Notes, "the Today snapshot", err)
	}
	meta.Recorded = recorded
	if !recorded && err == nil {
		meta.Notes = append(meta.Notes, "this Today capture (served from the local cache, fetched "+meta.CapturedAt+") is already stored")
	}
	if limit > 0 && len(stories) > limit {
		stories = stories[:limit]
	}
	meta.Returned = len(stories)
	return printFeatured(cmd, flags, meta, stories)
}

func printFeatured(cmd *cobra.Command, flags *rootFlags, meta featuredMeta, stories []featuredStory) error {
	w := cmd.OutOrStdout()
	if !wantsHumanTable(w, flags) {
		return s2bPrintEnvelope(w, flags, meta, stories)
	}
	if len(stories) == 0 {
		_, _ = fmt.Fprintln(w, "No stories.")
		s2bPrintNotes(cmd, meta.Notes)
		return nil
	}
	items := make([]map[string]any, 0, len(stories))
	for _, s := range stories {
		titles := make([]string, 0, len(s.Apps))
		for _, a := range s.Apps {
			titles = append(titles, a.Title)
		}
		items = append(items, map[string]any{
			"position": s.Position,
			"title":    s.Title,
			"heading":  s.Heading,
			"apps":     strings.Join(titles, ", "),
		})
	}
	if err := printAutoTable(w, items); err != nil {
		return err
	}
	s2bPrintNotes(cmd, meta.Notes)
	return nil
}
