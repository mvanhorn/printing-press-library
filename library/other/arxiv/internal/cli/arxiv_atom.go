package cli

import (
	"encoding/json"
	"encoding/xml"
	"strings"
)

type arxivAtomFeed struct {
	XMLName      xml.Name         `xml:"feed" json:"-"`
	Title        string           `xml:"title" json:"title,omitempty"`
	Updated      string           `xml:"updated" json:"updated,omitempty"`
	TotalResults int              `xml:"http://a9.com/-/spec/opensearch/1.1/ totalResults" json:"total_results"`
	StartIndex   int              `xml:"http://a9.com/-/spec/opensearch/1.1/ startIndex" json:"start_index"`
	ItemsPerPage int              `xml:"http://a9.com/-/spec/opensearch/1.1/ itemsPerPage" json:"items_per_page"`
	Entries      []arxivAtomEntry `xml:"entry" json:"entries"`
}

type arxivAtomEntry struct {
	ID         string            `xml:"id" json:"id,omitempty"`
	Title      string            `xml:"title" json:"title,omitempty"`
	Updated    string            `xml:"updated" json:"updated,omitempty"`
	Published  string            `xml:"published" json:"published,omitempty"`
	Summary    string            `xml:"summary" json:"summary,omitempty"`
	Authors    []arxivAtomAuthor `xml:"author" json:"authors,omitempty"`
	Links      []arxivAtomLink   `xml:"link" json:"links,omitempty"`
	Comment    string            `xml:"http://arxiv.org/schemas/atom comment" json:"comment,omitempty"`
	JournalRef string            `xml:"http://arxiv.org/schemas/atom journal_ref" json:"journal_ref,omitempty"`
	DOI        string            `xml:"http://arxiv.org/schemas/atom doi" json:"doi,omitempty"`
	Categories []arxivCategory   `xml:"category" json:"categories,omitempty"`
}

type arxivAtomAuthor struct {
	Name string `xml:"name" json:"name,omitempty"`
}

type arxivAtomLink struct {
	Href  string `xml:"href,attr" json:"href,omitempty"`
	Rel   string `xml:"rel,attr" json:"rel,omitempty"`
	Type  string `xml:"type,attr" json:"type,omitempty"`
	Title string `xml:"title,attr" json:"title,omitempty"`
}

type arxivCategory struct {
	Term string `xml:"term,attr" json:"term,omitempty"`
}

func parseArxivAtomJSON(data json.RawMessage) (json.RawMessage, error) {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		trimmed := strings.TrimSpace(string(data))
		if strings.HasPrefix(trimmed, "<") {
			raw = trimmed
		} else {
			var wrapped struct {
				Results string `json:"results"`
			}
			if wrapErr := json.Unmarshal(data, &wrapped); wrapErr != nil || wrapped.Results == "" {
				return data, err
			}
			raw = wrapped.Results
		}
	}
	if !strings.Contains(raw, "<feed") || !strings.Contains(raw, "http://www.w3.org/2005/Atom") {
		return data, nil
	}
	var feed arxivAtomFeed
	if err := xml.Unmarshal([]byte(raw), &feed); err != nil {
		return data, err
	}
	if feed.Entries == nil {
		feed.Entries = []arxivAtomEntry{}
	}
	for i := range feed.Entries {
		feed.Entries[i].Title = strings.Join(strings.Fields(feed.Entries[i].Title), " ")
		feed.Entries[i].Summary = strings.TrimSpace(feed.Entries[i].Summary)
	}
	out, err := json.Marshal(feed)
	if err != nil {
		return data, err
	}
	return out, nil
}
