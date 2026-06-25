package foxnews

import (
	"fmt"
	"strings"
)

const DefaultFeedBase = "https://moxie.foxnews.com/google-publisher"

// Section identifies a Fox News Google Publisher RSS feed.
type Section struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Path        string `json:"path"`
}

// Sections is the supported feed list (default: latest).
var Sections = []Section{
	{ID: "latest", Label: "Latest Headlines", Description: "Latest headlines across all sections", Path: "latest.xml"},
	{ID: "world", Label: "World", Description: "World news headlines", Path: "world.xml"},
	{ID: "politics", Label: "Politics", Description: "Politics headlines", Path: "politics.xml"},
	{ID: "science", Label: "Science", Description: "Science headlines", Path: "science.xml"},
	{ID: "health", Label: "Health", Description: "Health headlines", Path: "health.xml"},
	{ID: "sports", Label: "Sports", Description: "Sports headlines", Path: "sports.xml"},
	{ID: "travel", Label: "Travel", Description: "Travel headlines", Path: "travel.xml"},
	{ID: "tech", Label: "Tech", Description: "Technology headlines", Path: "tech.xml"},
	{ID: "opinion", Label: "Opinion", Description: "Opinion headlines", Path: "opinion.xml"},
	{ID: "video", Label: "Video", Description: "Video headlines", Path: "videos.xml"},
}

// ResolveSection maps a user-facing section id (case-insensitive) to a Section.
// Aliases: "videos" -> video, "all" -> latest.
func ResolveSection(raw string) (Section, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" || raw == "all" {
		return Sections[0], nil
	}
	if raw == "videos" {
		raw = "video"
	}
	for _, s := range Sections {
		if s.ID == raw {
			return s, nil
		}
	}
	ids := make([]string, 0, len(Sections))
	for _, s := range Sections {
		ids = append(ids, s.ID)
	}
	return Section{}, fmt.Errorf("unknown section %q: use one of %s", raw, strings.Join(ids, ", "))
}

// FeedURL builds the RSS URL for a section and optional base override.
func FeedURL(section Section, base string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		base = DefaultFeedBase
	}
	return base + "/" + section.Path
}
