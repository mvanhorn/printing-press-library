// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Shared helpers for the cross-app "fleet" commands (cockpit, pipeline,
// traction, reviews recent, blockers). These fan the App Store Connect read
// endpoints across every app and fold the results into one view — the thing no
// single-app tool does.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/asc/internal/client"

	"github.com/spf13/cobra"
)

// ---- JSON:API envelope ----

type ascResource struct {
	Type       string          `json:"type"`
	ID         string          `json:"id"`
	Attributes json.RawMessage `json:"attributes"`
}

type ascListResp struct {
	Data  []ascResource `json:"data"`
	Links struct {
		Next string `json:"next"`
	} `json:"links"`
}

// ---- attribute shapes (only the fields the fleet views need) ----

type ascApp struct {
	ID       string
	Name     string `json:"name"`
	BundleID string `json:"bundleId"`
	SKU      string `json:"sku"`
}

type ascVersion struct {
	VersionString string `json:"versionString"`
	AppStoreState string `json:"appStoreState"`
	AppVersion    string `json:"appVersionState"`
	Platform      string `json:"platform"`
	CreatedDate   string `json:"createdDate"`
}

func (v ascVersion) state() string {
	if v.AppStoreState != "" {
		return v.AppStoreState
	}
	return v.AppVersion
}

type ascBuild struct {
	Version         string `json:"version"`
	ProcessingState string `json:"processingState"`
	UploadedDate    string `json:"uploadedDate"`
	Expired         bool   `json:"expired"`
}

type ascReview struct {
	Rating           int    `json:"rating"`
	Title            string `json:"title"`
	Body             string `json:"body"`
	ReviewerNickname string `json:"reviewerNickname"`
	CreatedDate      string `json:"createdDate"`
	Territory        string `json:"territory"`
}

// ---- fetch helpers ----

// ascGetAll fetches all pages of a JSON:API collection (bounded), following
// links.next. The client handles base URL, auth, rate limiting, and caching.
func ascGetAll(ctx context.Context, c *client.Client, path string, params map[string]string) ([]ascResource, error) {
	raw, err := c.Get(ctx, path, params)
	if err != nil {
		return nil, err
	}
	var out []ascResource
	for i := 0; i < 50; i++ {
		var page ascListResp
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, fmt.Errorf("parsing %s response: %w", path, err)
		}
		out = append(out, page.Data...)
		if page.Links.Next == "" || len(out) >= 5000 {
			break
		}
		nextPath := strings.TrimPrefix(page.Links.Next, c.RequestBaseURL())
		if raw, err = c.Get(ctx, nextPath, nil); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func fleetApps(ctx context.Context, c *client.Client) ([]ascApp, error) {
	res, err := ascGetAll(ctx, c, "/v1/apps", map[string]string{"limit": "200"})
	if err != nil {
		return nil, err
	}
	apps := make([]ascApp, 0, len(res))
	for _, r := range res {
		var a ascApp
		_ = json.Unmarshal(r.Attributes, &a)
		a.ID = r.ID
		apps = append(apps, a)
	}
	sort.Slice(apps, func(i, j int) bool { return apps[i].Name < apps[j].Name })
	return apps, nil
}

// latestVersion returns the most recently created appStoreVersion for an app.
func latestVersion(ctx context.Context, c *client.Client, appID string) (ascVersion, bool, error) {
	vs, err := appVersions(ctx, c, appID)
	if err != nil || len(vs) == 0 {
		return ascVersion{}, false, err
	}
	return vs[0], true, nil
}

// appVersions returns an app's versions, newest first.
func appVersions(ctx context.Context, c *client.Client, appID string) ([]ascVersion, error) {
	res, err := ascGetAll(ctx, c, "/v1/apps/"+appID+"/appStoreVersions", map[string]string{"limit": "20"})
	if err != nil {
		return nil, err
	}
	out := make([]ascVersion, 0, len(res))
	for _, r := range res {
		var v ascVersion
		_ = json.Unmarshal(r.Attributes, &v)
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedDate > out[j].CreatedDate })
	return out, nil
}

// latestBuild returns the most recently uploaded build for an app.
func latestBuild(ctx context.Context, c *client.Client, appID string) (ascBuild, bool, error) {
	res, err := ascGetAll(ctx, c, "/v1/apps/"+appID+"/builds", map[string]string{"limit": "20"})
	if err != nil || len(res) == 0 {
		return ascBuild{}, false, err
	}
	builds := make([]ascBuild, 0, len(res))
	for _, r := range res {
		var b ascBuild
		_ = json.Unmarshal(r.Attributes, &b)
		builds = append(builds, b)
	}
	sort.Slice(builds, func(i, j int) bool { return builds[i].UploadedDate > builds[j].UploadedDate })
	return builds[0], true, nil
}

// appReviews returns up to `limit` newest customer reviews for an app.
func appReviews(ctx context.Context, c *client.Client, appID string, limit int) ([]ascReview, error) {
	res, err := ascGetAll(ctx, c, "/v1/apps/"+appID+"/customerReviews",
		map[string]string{"limit": fmt.Sprintf("%d", limit), "sort": "-createdDate"})
	if err != nil {
		return nil, err
	}
	out := make([]ascReview, 0, len(res))
	for _, r := range res {
		var rev ascReview
		_ = json.Unmarshal(r.Attributes, &rev)
		out = append(out, rev)
	}
	return out, nil
}

// ---- shared logic ----

// inFlightStates are the appStoreState values that mean "in the review pipeline".
var inFlightStates = map[string]bool{
	"WAITING_FOR_REVIEW":        true,
	"IN_REVIEW":                 true,
	"PENDING_APPLE_RELEASE":     true,
	"PENDING_DEVELOPER_RELEASE": true,
	"PROCESSING_FOR_APP_STORE":  true,
	"PENDING_CONTRACT":          true,
}

// blockedReason returns a non-empty action string when the app can't ship, else "".
func blockedReason(state, buildState string) string {
	switch state {
	case "REJECTED", "DEVELOPER_REJECTED":
		return "version rejected — resubmit"
	case "METADATA_REJECTED":
		return "metadata rejected — fix metadata"
	case "INVALID_BINARY":
		return "invalid binary — upload a new build"
	case "DEVELOPER_REMOVED_FROM_SALE":
		return "removed from sale"
	}
	switch buildState {
	case "FAILED", "INVALID":
		return "latest build " + strings.ToLower(buildState)
	}
	return ""
}

// actionFlag is the cockpit's one-word attention signal.
func actionFlag(state, buildState string) string {
	if r := blockedReason(state, buildState); r != "" {
		return r
	}
	if inFlightStates[state] {
		return "in review"
	}
	if buildState == "PROCESSING" {
		return "build processing"
	}
	return "ok"
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func round1(f float64) float64 { return math.Round(f*10) / 10 }

func atoiOr(s string, def int) int {
	if s == "" {
		return def
	}
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && n >= 0 {
		return n // negatives (e.g. --limit -5) would panic a slice; fall back
	}
	return def
}

func shortDate(iso string) string {
	if len(iso) >= 10 {
		return iso[:10]
	}
	return iso
}

// reviewsWithin keeps reviews created within the last `days` days.
func reviewsWithin(revs []ascReview, days int) []ascReview {
	if days <= 0 {
		return revs
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	out := revs[:0:0]
	for _, r := range revs {
		if t, err := time.Parse(time.RFC3339, r.CreatedDate); err == nil && t.After(cutoff) {
			out = append(out, r)
		}
	}
	return out
}

func meanRating(revs []ascReview) float64 {
	if len(revs) == 0 {
		return 0
	}
	sum := 0
	for _, r := range revs {
		sum += r.Rating
	}
	return float64(sum) / float64(len(revs))
}

// ratingTrend returns mean(newer half) - mean(older half). revs are newest-first,
// so a negative value means recent reviews are worse than older ones.
func ratingTrend(revs []ascReview) float64 {
	if len(revs) < 4 {
		return 0
	}
	mid := len(revs) / 2
	return meanRating(revs[:mid]) - meanRating(revs[mid:])
}

func ageDays(created string) float64 {
	t, err := time.Parse(time.RFC3339, created)
	if err != nil {
		return 0
	}
	return time.Since(t).Hours() / 24
}

// renderFleet emits typed JSON for agents/pipes and a table for humans, matching
// the generated commands' output convention.
func renderFleet(cmd *cobra.Command, flags *rootFlags, jsonVal any, headers []string, rows [][]string, emptyMsg string) error {
	if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
		return flags.printJSON(cmd, jsonVal)
	}
	if len(rows) == 0 {
		if !flags.quiet && emptyMsg != "" {
			fmt.Fprintln(cmd.OutOrStdout(), emptyMsg)
		}
		return nil
	}
	return flags.printTable(cmd, headers, rows)
}
