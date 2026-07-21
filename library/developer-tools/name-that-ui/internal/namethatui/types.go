package namethatui

type API struct {
	Framework string `json:"framework"`
	Symbol    string `json:"symbol"`
	Note      string `json:"note"`
}

type Part struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	API         string `json:"api"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
}

type Component struct {
	ID          string   `json:"id"`
	Slug        string   `json:"slug"`
	Platform    string   `json:"platform"`
	Name        string   `json:"name"`
	Tagline     string   `json:"tagline"`
	AKA         []string `json:"aka"`
	Fuzzy       []string `json:"fuzzy"`
	API         []API    `json:"api"`
	Prompt      string   `json:"prompt"`
	DebugPrompt string   `json:"debugPrompt"`
	Description string   `json:"description"`
	Parts       []Part   `json:"parts"`
	Related     []string `json:"related"`
	Demo        string   `json:"demo"`
	SourceURL   string   `json:"source_url"`
}

type Signal struct {
	ID          string `json:"id"`
	Facet       string `json:"facet"`
	Role        string `json:"role"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Section struct {
	Heading     string `json:"heading"`
	Text        string `json:"text"`
	SourceURL   string `json:"source_url"`
	ContentHash string `json:"content_hash"`
}

type Style struct {
	ID        string    `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	SourceURL string    `json:"source_url"`
	Signals   []Signal  `json:"signals"`
	Sections  []Section `json:"sections"`
}

type Report struct {
	Components, Styles int
}
