package thparse

type Trend struct {
	Slug         string   `json:"slug"`
	Title        string   `json:"title"`
	Description  string   `json:"description,omitempty"`
	ImageURL     string   `json:"image_url,omitempty"`
	Keywords     []string `json:"keywords,omitempty"`
	Author       string   `json:"author,omitempty"`
	Category     string   `json:"category,omitempty"`
	TrendID      string   `json:"trend_id,omitempty"`
	PubDate      string   `json:"pub_date,omitempty"`
	BodyText     string   `json:"body_text,omitempty"`
	RelatedSlugs []string `json:"related_slugs,omitempty"`
	FAQ          []FAQ    `json:"faq,omitempty"`
	SourceURL    string   `json:"source_url,omitempty"`
	Source       string   `json:"source,omitempty"`
}

type FAQ struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

type SitemapEntry struct {
	URL     string `json:"url"`
	LastMod string `json:"last_mod,omitempty"`
	Kind    string `json:"kind"`
}
