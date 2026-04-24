package cli

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type resourceSpec struct {
	Name        string
	Path        string
	Aliases     []string
	Description string
	Archiveable bool
}

var syncResourceSpecs = []resourceSpec{
	{Name: "account", Path: "/v1/account/get-api-usage", Description: "account usage snapshots", Archiveable: true},
	{Name: "bluesky", Path: "/v1/bluesky/user/posts", Description: "Bluesky user posts"},
	{Name: "facebook", Path: "/v1/facebook/group/posts", Description: "Facebook group posts"},
	{Name: "google", Path: "/v1/google/search", Description: "Google search results"},
	{Name: "instagram", Path: "/v2/instagram/user/posts", Description: "Instagram user posts"},
	{Name: "linkedin", Path: "/v1/linkedin/company/posts", Description: "LinkedIn company posts"},
	{Name: "pinterest", Path: "/v1/pinterest/user/boards", Description: "Pinterest user boards"},
	{Name: "reddit", Path: "/v1/reddit/search", Description: "Reddit search results"},
	{Name: "tiktok", Path: "/v1/tiktok/videos/popular", Aliases: []string{"tiktok_videos"}, Description: "TikTok videos"},
	{Name: "truthsocial", Path: "/v1/truthsocial/user/posts", Description: "Truth Social user posts"},
	{Name: "youtube", Path: "/v1/youtube/channel-videos", Description: "YouTube channel videos"},
}

var platformRootSummaries = map[string]string{
	"account":     "Account usage, quota, and credit analytics commands",
	"bluesky":     "Bluesky profile and post commands",
	"facebook":    "Facebook pages, posts, comments, groups, and ad library commands",
	"google":      "Google search and advertiser lookup commands",
	"instagram":   "Instagram profile, post, reel, transcript, and user-feed commands",
	"linkedin":    "LinkedIn profile, company, post, and ad commands",
	"pinterest":   "Pinterest pin, search, and board commands",
	"reddit":      "Reddit search, subreddit, post, and ad commands",
	"threads":     "Threads profile, post, and search commands",
	"tiktok":      "TikTok profile, video, search, shop, and analytics commands",
	"truthsocial": "Truth Social profile and post commands",
	"twitch":      "Twitch clip and profile commands",
	"twitter":     "Twitter/X profile, tweet, community, and feed commands",
	"youtube":     "YouTube channel, video, transcript, playlist, and search commands",
}

var apiUtilityCommands = map[string]bool{
	"agent":      true,
	"analytics":  true,
	"api":        true,
	"auth":       true,
	"completion": true,
	"doctor":     true,
	"export":     true,
	"search":     true,
	"sync":       true,
	"tail":       true,
	"version":    true,
	"archive":    true,
}

func applyPlatformRootMetadata(root *cobra.Command) {
	for _, child := range root.Commands() {
		summary, ok := platformRootSummaries[child.Name()]
		if !ok {
			continue
		}
		child.Short = summary
		child.Long = summary
		child.Flags().VisitAll(func(flag *pflag.Flag) {
			_ = child.Flags().MarkHidden(flag.Name)
		})
	}
}

func apiInterfaces(root *cobra.Command) []*cobra.Command {
	var commands []*cobra.Command
	for _, child := range root.Commands() {
		if apiUtilityCommands[child.Name()] || len(child.Commands()) == 0 {
			continue
		}
		commands = append(commands, child)
	}
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Name() < commands[j].Name()
	})
	return commands
}

func resolveResourceSpec(name string) (resourceSpec, bool) {
	key := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), "-", "_"))
	for _, spec := range syncResourceSpecs {
		if key == spec.Name {
			return spec, true
		}
		for _, alias := range spec.Aliases {
			if key == alias {
				return spec, true
			}
		}
	}
	return resourceSpec{}, false
}

func knownResourceNames() []string {
	names := make([]string, 0, len(syncResourceSpecs))
	for _, spec := range syncResourceSpecs {
		names = append(names, spec.Name)
	}
	sort.Strings(names)
	return names
}

func archiveableResourceNames() []string {
	names := make([]string, 0, len(syncResourceSpecs))
	for _, spec := range syncResourceSpecs {
		if spec.Archiveable {
			names = append(names, spec.Name)
		}
	}
	sort.Strings(names)
	return names
}

func resourceAPIPath(name string) string {
	if spec, ok := resolveResourceSpec(name); ok {
		return spec.Path
	}
	resource := strings.TrimSpace(name)
	if resource == "" {
		return "/"
	}
	return "/" + resource
}
