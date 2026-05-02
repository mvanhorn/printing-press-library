package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

func newCreatorCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "creator",
		Short: "Cross-platform creator commands (presence, comparison, growth tracking)",
	}
	cmd.AddCommand(newCreatorFindCmd(flags))
	cmd.AddCommand(newCreatorCompareCmd(flags))
	cmd.AddCommand(newCreatorTrackCmd(flags))
	return cmd
}

// platformProbe describes one profile-endpoint probe used by `creator find`.
type platformProbe struct {
	platform   string
	path       string
	param      string // query-param name (e.g. "handle", "username")
	transform  func(string) string
	followerKy []string // JSON keys to look up the follower count, in priority order
	nameKy     []string // JSON keys for display name
	urlKy      []string // JSON keys for profile url
}

// findProbes is the list of platforms that expose a profile-by-handle
// endpoint. Order matches the spec; everything that takes a URL or numeric
// ID is omitted (e.g. Snapchat needs a handle but its endpoint takes one too).
func findProbes() []platformProbe {
	return []platformProbe{
		{platform: "tiktok", path: "/v1/tiktok/profile", param: "handle",
			followerKy: []string{"user.stats.followerCount", "stats.followerCount"},
			nameKy:     []string{"user.nickname", "nickname"}},
		{platform: "instagram", path: "/v1/instagram/profile", param: "handle",
			followerKy: []string{"data.user.edge_followed_by.count", "follower_count"},
			nameKy:     []string{"data.user.full_name", "full_name"}},
		{platform: "youtube", path: "/v1/youtube/channel", param: "handle",
			transform:  func(h string) string { return ensurePrefix(h, "@") },
			followerKy: []string{"subscriberCount", "subscriberCountText"},
			nameKy:     []string{"name", "title"}},
		{platform: "twitter", path: "/v1/twitter/profile", param: "handle",
			followerKy: []string{"followers_count"},
			nameKy:     []string{"name"}},
		{platform: "threads", path: "/v1/threads/profile", param: "handle",
			followerKy: []string{"follower_count"},
			nameKy:     []string{"full_name"}},
		{platform: "bluesky", path: "/v1/bluesky/profile", param: "handle",
			transform:  func(h string) string { return ensureSuffix(h, ".bsky.social") },
			followerKy: []string{"followersCount"},
			nameKy:     []string{"displayName"}},
		{platform: "snapchat", path: "/v1/snapchat/profile", param: "handle",
			followerKy: []string{"subscriberCount"},
			nameKy:     []string{"displayName"}},
		{platform: "twitch", path: "/v1/twitch/profile", param: "handle",
			followerKy: []string{"followers"},
			nameKy:     []string{"displayName"}},
		{platform: "truthsocial", path: "/v1/truthsocial/profile", param: "handle",
			followerKy: []string{"followers_count"},
			nameKy:     []string{"display_name"}},
		// LinkedIn requires a profile URL, so it's a different shape - omitted from
		// the cross-platform probe to keep the contract simple.
	}
}

func ensurePrefix(s, prefix string) string {
	if strings.HasPrefix(s, prefix) {
		return s
	}
	return prefix + s
}

func ensureSuffix(s, suffix string) string {
	if strings.Contains(s, suffix) || strings.Contains(s, ".") {
		return s
	}
	return s + suffix
}

// creatorFindResult is one row of the cross-platform presence matrix.
type creatorFindResult struct {
	Platform      string `json:"platform"`
	Handle        string `json:"handle"`
	Found         bool   `json:"found"`
	FollowerCount int64  `json:"follower_count,omitempty"`
	DisplayName   string `json:"display_name,omitempty"`
	Error         string `json:"error,omitempty"`
}

func newCreatorFindCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "find <handle>",
		Short: "Probe every platform's profile endpoint for a handle and return a presence matrix",
		Long: `Fans out concurrent profile lookups across TikTok, Instagram, YouTube, Twitter,
Threads, Bluesky, Snapchat, Twitch, and Truth Social. Returns one row per
platform with found / not-found and the follower count when available.

Each found platform burns one credit. Unfound platforms still consume credits
on platforms whose endpoint accepts unknown handles and returns a 200.`,
		Example:     "  scrape-creators-pp-cli creator find mrbeast --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			handle := strings.TrimPrefix(args[0], "@")
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			probes := findProbes()
			results := make([]creatorFindResult, len(probes))
			var wg sync.WaitGroup
			for i, p := range probes {
				wg.Add(1)
				go func(i int, p platformProbe) {
					defer wg.Done()
					h := handle
					if p.transform != nil {
						h = p.transform(h)
					}
					raw, err := c.Get(p.path, map[string]string{p.param: h})
					row := creatorFindResult{Platform: p.platform, Handle: h}
					if err != nil {
						row.Error = trimError(err.Error())
						results[i] = row
						return
					}
					row.Found = true
					var anyJSON map[string]any
					if json.Unmarshal(raw, &anyJSON) == nil {
						row.FollowerCount = lookupInt(anyJSON, p.followerKy)
						row.DisplayName = lookupString(anyJSON, p.nameKy)
					}
					results[i] = row
				}(i, p)
				// throttle a touch: stagger probes 50ms apart
				time.Sleep(50 * time.Millisecond)
			}
			wg.Wait()

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "PLATFORM\tFOUND\tFOLLOWERS\tDISPLAY NAME\tNOTE")
			for _, r := range results {
				note := r.Error
				if r.Found {
					fmt.Fprintf(tw, "%s\t✔\t%d\t%s\t\n", r.Platform, r.FollowerCount, r.DisplayName)
				} else {
					fmt.Fprintf(tw, "%s\t-\t-\t-\t%s\n", r.Platform, note)
				}
			}
			return tw.Flush()
		},
	}
	return cmd
}

// lookupInt walks dotted JSON paths and returns the first integer it finds.
func lookupInt(m map[string]any, paths []string) int64 {
	for _, p := range paths {
		if v := walkPath(m, p); v != nil {
			switch x := v.(type) {
			case float64:
				return int64(x)
			case int64:
				return x
			case int:
				return int64(x)
			case string:
				// parse common "1.2M" / "1,234" forms? Keep simple.
				return 0
			}
		}
	}
	return 0
}

func lookupString(m map[string]any, paths []string) string {
	for _, p := range paths {
		if v := walkPath(m, p); v != nil {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

func walkPath(m map[string]any, dotted string) any {
	parts := strings.Split(dotted, ".")
	var cur any = m
	for _, k := range parts {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = mm[k]
	}
	return cur
}

func trimError(s string) string {
	if len(s) > 80 {
		return s[:80] + "…"
	}
	return s
}
