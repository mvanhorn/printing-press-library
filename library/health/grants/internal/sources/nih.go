package sources

import "fmt"

// NIH RePORTER v2 — awarded NIH projects. Keyless.
const nihSearchURL = "https://api.reporter.nih.gov/v2/projects/search"

type NIHProject struct {
	ProjectNum  string  `json:"project_num"`
	Title       string  `json:"project_title"`
	AwardAmount float64 `json:"award_amount"`
	FiscalYear  int     `json:"fiscal_year"`
	PI          string  `json:"contact_pi_name"`
	Org         struct {
		Name string `json:"org_name"`
	} `json:"organization"`
}

type nihResp struct {
	Meta struct {
		Total int `json:"total"`
	} `json:"meta"`
	Results []NIHProject `json:"results"`
}

// SearchNIH returns awarded projects for a keyword, largest awards first.
func SearchNIH(keyword string, fiscalYear, limit int) ([]NIHProject, int, error) {
	criteria := map[string]any{
		"advanced_text_search": map[string]any{
			"operator":     "and",
			"search_field": "projecttitle,terms,abstracttext",
			"search_text":  keyword,
		},
	}
	if fiscalYear > 0 {
		criteria["fiscal_years"] = []int{fiscalYear}
	}
	payload := map[string]any{
		"criteria":   criteria,
		"limit":      limit,
		"offset":     0,
		"sort_field": "award_amount",
		"sort_order": "desc",
	}
	var resp nihResp
	if err := postJSON(nihSearchURL, payload, &resp); err != nil {
		return nil, 0, fmt.Errorf("NIH RePORTER: %w", err)
	}
	return resp.Results, resp.Meta.Total, nil
}
