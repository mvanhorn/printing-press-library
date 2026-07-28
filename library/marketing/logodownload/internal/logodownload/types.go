package logodownload

// LogoResult is the JSON contract consumed by agents and PrintingPressDev.
type LogoResult struct {
	Title        string `json:"title"`
	URL          string `json:"url"`
	ImageURL     string `json:"image_url,omitempty"`
	DownloadPath string `json:"download_path,omitempty"`
}

type wpSearchResult struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

type wpPostResult struct {
	Embedded struct {
		FeaturedMedia []struct {
			SourceURL    string `json:"source_url"`
			MediaDetails struct {
				Sizes map[string]struct {
					SourceURL string `json:"source_url"`
				} `json:"sizes"`
			} `json:"media_details"`
		} `json:"wp:featuredmedia"`
	} `json:"_embedded"`
}

type SelectionMode string

const (
	SelectFirst SelectionMode = "first"
	SelectAll   SelectionMode = "all"
	SelectIndex SelectionMode = "index"
)

type Selection struct {
	Mode  SelectionMode
	Index int
}
