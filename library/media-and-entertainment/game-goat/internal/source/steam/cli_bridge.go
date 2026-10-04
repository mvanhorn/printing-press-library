// cli_bridge.go — thin bridge imported by internal/cli commands. Wraps the
// raw ResolveAppID + ReviewSummary (+ optional price) calls into one
// per-title enrichment lookup so commands never touch Steam's wire shapes.

package steam

import "context"

// SteamReview is the per-title Steam enrichment the CLI surfaces: the
// keyless appreviews rollup plus the current price when appdetails has one.
type SteamReview struct {
	AppID    int64          `json:"app_id"`
	Score    float64        `json:"score"`
	Desc     string         `json:"desc"`
	Positive int            `json:"positive"`
	Negative int            `json:"negative"`
	Total    int            `json:"total"`
	Price    *PriceOverview `json:"price,omitempty"`
	// DemoAppIDs lists the app's demos when the appdetails call succeeded.
	DemoAppIDs []int64 `json:"demo_app_ids,omitempty"`
	// HasDemo is known only when appdetails succeeded: nil means unknown,
	// not false.
	HasDemo *bool `json:"has_demo,omitempty"`
}

// bridgeClient is the shared process-wide client the bridge uses.
var bridgeClient = New(nil)

// SteamReviewForTitle resolves a title to a Steam app and returns its
// review summary plus current price when appdetails exposes one. Price
// failures degrade to Price=nil — the review block still ships. Resolution
// or review failures return the typed error so commands can degrade the
// whole steam block (sources_missing) instead of failing the command.
func SteamReviewForTitle(ctx context.Context, title string) (*SteamReview, error) {
	return enrichTitle(ctx, bridgeClient, title)
}

// SteamReviewForApp returns the review rollup, plus the current price when
// appdetails exposes one, for an appid that is already known — no title
// resolution. Price failures degrade to Price=nil so the review block ships.
func SteamReviewForApp(ctx context.Context, appid int64) (*SteamReview, error) {
	return enrichAppID(ctx, bridgeClient, appid)
}

// SteamReviewForTitleWithYear is the store-aware title path: it resolves
// through the plural store search with a release-year hint (so "(YYYY)"
// suffixes and remake collisions resolve), then fetches the review block.
func SteamReviewForTitleWithYear(ctx context.Context, title string, year int) (*SteamReview, error) {
	return enrichTitleWithYear(ctx, bridgeClient, title, year)
}

func enrichTitleWithYear(ctx context.Context, c *Client, title string, year int) (*SteamReview, error) {
	if c == nil {
		c = New(nil)
	}
	appid, err := c.ResolveAppIDWithHint(ctx, title, year)
	if err != nil {
		return nil, err
	}
	return enrichAppID(ctx, c, appid)
}

// enrichAppID fetches the review rollup for a known appid.
func enrichAppID(ctx context.Context, c *Client, appid int64) (*SteamReview, error) {
	if c == nil {
		c = New(nil)
	}
	summary, err := c.ReviewSummary(ctx, appid)
	if err != nil {
		return nil, err
	}
	review := &SteamReview{
		AppID:    appid,
		Score:    summary.Score,
		Desc:     summary.Desc,
		Positive: summary.Positive,
		Negative: summary.Negative,
		Total:    summary.Total,
	}
	// Optional appdetails enrichment: never fail the review block for it, but
	// report demo data only when the call actually succeeded — a failed
	// appdetails leaves has_demo unknown (nil), not false.
	if details, derr := c.AppDetails(ctx, appid); derr == nil && details != nil {
		review.Price = details.Price
		review.DemoAppIDs = details.DemoAppIDs
		hasDemo := len(details.DemoAppIDs) > 0
		review.HasDemo = &hasDemo
	}
	return review, nil
}

func enrichTitle(ctx context.Context, c *Client, title string) (*SteamReview, error) {
	if c == nil {
		c = New(nil)
	}
	appid, err := c.ResolveAppID(ctx, title)
	if err != nil {
		return nil, err
	}
	return enrichAppID(ctx, c, appid)
}
