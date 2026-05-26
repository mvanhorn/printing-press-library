package categorize

import "strings"

var known = map[string]string{
	"github.com": "Coding", "gitlab.com": "Coding", "bitbucket.org": "Coding", "stackoverflow.com": "Coding", "serverfault.com": "Coding", "superuser.com": "Coding", "news.ycombinator.com": "Coding", "pypi.org": "Coding", "npmjs.com": "Coding", "go.dev": "Coding", "pkg.go.dev": "Coding", "rust-lang.org": "Coding", "developer.mozilla.org": "Coding", "w3.org": "Coding", "sqlite.org": "Coding", "golang.org": "Coding", "vercel.com": "Coding", "netlify.com": "Coding", "digitalocean.com": "Coding", "cloudflare.com": "Coding", "render.com": "Coding",
	"openai.com": "AI", "chatgpt.com": "AI", "claude.ai": "AI", "claude.com": "AI", "anthropic.com": "AI", "gemini.google.com": "AI", "perplexity.ai": "AI", "huggingface.co": "AI", "replicate.com": "AI", "midjourney.com": "AI", "stability.ai": "AI", "cohere.com": "AI", "mistral.ai": "AI",
	"google.com": "Search", "bing.com": "Search", "duckduckgo.com": "Search", "search.brave.com": "Search", "ecosia.org": "Search", "startpage.com": "Search", "baidu.com": "Search",
	"wikipedia.org": "Research", "scholar.google.com": "Research", "arxiv.org": "Research", "jstor.org": "Research", "nature.com": "Research", "science.org": "Research", "nih.gov": "Research", "nasa.gov": "Research", "khanacademy.org": "Research", "coursera.org": "Research", "edx.org": "Research", "udemy.com": "Research", "medium.com": "Research", "substack.com": "Research",
	"slack.com": "Comms", "gmail.com": "Comms", "mail.google.com": "Comms", "docs.google.com": "Comms", "drive.google.com": "Comms", "outlook.com": "Comms", "office.com": "Comms", "discord.com": "Comms", "zoom.us": "Comms", "teams.microsoft.com": "Comms", "meet.google.com": "Comms", "atlassian.net": "Comms", "notion.so": "Comms", "airtable.com": "Comms", "linear.app": "Comms", "trello.com": "Comms", "asana.com": "Comms",
	"reddit.com": "Social", "twitter.com": "Social", "x.com": "Social", "instagram.com": "Social", "facebook.com": "Social", "tiktok.com": "Social", "threads.net": "Social", "bsky.app": "Social", "mastodon.social": "Social", "linkedin.com": "Social", "pinterest.com": "Social", "tumblr.com": "Social", "quora.com": "Social", "imgur.com": "Social", "9gag.com": "Social", "fark.com": "Social", "digg.com": "Social", "feedly.com": "Social",
	"youtube.com": "Streaming", "netflix.com": "Streaming", "twitch.tv": "Streaming", "hulu.com": "Streaming", "disneyplus.com": "Streaming", "spotify.com": "Streaming", "soundcloud.com": "Streaming", "max.com": "Streaming", "primevideo.com": "Streaming", "peacocktv.com": "Streaming", "apple.com": "Streaming", "music.youtube.com": "Streaming",
	"nytimes.com": "News", "washingtonpost.com": "News", "cnn.com": "News", "foxnews.com": "News", "bbc.com": "News", "reuters.com": "News", "bloomberg.com": "News", "theguardian.com": "News", "wsj.com": "News", "npr.org": "News", "politico.com": "News", "axios.com": "News", "apnews.com": "News",
	"amazon.com": "Shopping", "etsy.com": "Shopping", "ebay.com": "Shopping", "walmart.com": "Shopping", "target.com": "Shopping", "bestbuy.com": "Shopping", "costco.com": "Shopping", "aliexpress.com": "Shopping", "newegg.com": "Shopping", "shopify.com": "Shopping", "wayfair.com": "Shopping", "homedepot.com": "Shopping",
}

func Classify(domain string) (bucket, productivity string) {
	d := strings.ToLower(strings.TrimSpace(domain))
	d = strings.TrimPrefix(d, "www.")
	for k, b := range known {
		if d == k || strings.HasSuffix(d, "."+k) {
			return b, productivityFor(b)
		}
	}
	switch {
	case strings.HasSuffix(d, ".dev"), strings.Contains(d, "docs."):
		return "Coding", "productive"
	case strings.HasSuffix(d, ".slack.com"), strings.HasSuffix(d, ".atlassian.net"):
		return "Comms", "productive"
	case strings.Contains(d, "mastodon"):
		return "Social", "distracting"
	}
	return "Other", "neutral"
}

func productivityFor(bucket string) string {
	switch bucket {
	case "Coding", "Research", "AI", "Comms":
		return "productive"
	case "Social", "Streaming":
		return "distracting"
	case "Search":
		return "neutral"
	default:
		return "neutral"
	}
}
