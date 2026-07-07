package gplay

import (
	"context"
	"fmt"
	"net/url"
)

// AppDetails fetches the full detail listing for an appId by parsing the
// AF_initDataCallback ds:5 block of the details HTML page.
func (c *Client) AppDetails(ctx context.Context, appID string) (*App, error) {
	if appID == "" {
		return nil, fmt.Errorf("appId is required")
	}
	q := url.Values{}
	q.Set("id", appID)
	html, err := c.getHTML(ctx, "/store/apps/details", q)
	if err != nil {
		return nil, err
	}
	ds, err := extractAFData(html)
	if err != nil {
		return nil, fmt.Errorf("parsing app page for %s: %w", appID, err)
	}
	raw, ok := ds["ds:5"]
	if !ok {
		return nil, fmt.Errorf("app %s: details block (ds:5) not found (store layout may have changed)", appID)
	}
	root := decode(raw).path(1, 2)
	if root.v == nil {
		return nil, fmt.Errorf("app %s: unexpected details layout", appID)
	}

	a := &App{
		AppID: appID,
		URL:   baseURL + "/store/apps/details?id=" + appID,
	}
	a.Title = root.path(0, 0).cleanStr()
	if a.Title == "" {
		return nil, fmt.Errorf("app %s not found", appID)
	}
	a.Description = root.path(12, 0, 0, 1).cleanStr()
	if a.Description == "" {
		a.Description = root.path(72, 0, 1).cleanStr()
	}
	a.Summary = root.path(73, 0, 1).cleanStr()
	a.Installs = root.path(13, 0).str()
	a.MinInstalls = root.path(13, 1).int64()
	a.RealInstalls = root.path(13, 2).int64()
	a.Score = root.path(51, 0, 1).float()
	a.ScoreText = root.path(51, 0, 0).str()
	a.Ratings = root.path(51, 2, 1).int64()
	a.Reviews = root.path(51, 3, 1).int64()
	hist := root.path(51, 1)
	for i := 0; i < 5; i++ {
		// histogram entries are [_, count]
		a.Histogram[i] = hist.path(i+1, 1).int64()
	}
	a.Developer = root.path(68, 0).cleanStr()
	a.DeveloperID = devIDFromURL(root.path(68, 1, 4, 2).str())
	a.DeveloperMail = root.path(69, 1, 0).str()
	a.DeveloperWeb = root.path(69, 0, 5, 2).str()
	a.PrivacyPolicy = root.path(99, 0, 5, 2).str()
	a.Genre = root.path(79, 0, 0, 0).cleanStr()
	a.GenreID = root.path(79, 0, 0, 2).str()
	a.Icon = root.path(95, 0, 3, 2).str()
	a.HeaderImage = root.path(96, 0, 3, 2).str()
	a.Video = root.path(100, 0, 0, 3, 2).str()
	for _, sc := range root.path(78, 0).arr() {
		if u := sc.path(3, 2).str(); u != "" {
			a.Screenshots = append(a.Screenshots, u)
		}
	}
	a.ContentRating = root.path(9, 0).cleanStr()
	a.Released = root.path(10, 0).cleanStr()
	a.Updated = root.path(145, 0, 1, 0).int64()
	a.Version = root.path(140, 0, 0, 0).str()
	a.RecentChanges = root.path(144, 1, 1).cleanStr()
	a.AndroidVer = root.path(140, 1, 1, 0, 0, 1).str()
	a.ContainsAds = root.path(48).bool()
	a.OffersIAP = root.path(19, 0).v != nil

	// price micros at a deep path; currency alongside.
	priceMicros := root.path(57, 0, 0, 0, 0, 1, 0, 0).float()
	a.Price = priceMicros / 1e6
	a.Currency = root.path(57, 0, 0, 0, 0, 1, 0, 1).str()
	a.Free = a.Price == 0
	if a.OffersIAP {
		a.IAPRange = root.path(19, 0).cleanStr()
	}
	return a, nil
}
