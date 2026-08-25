// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.
// OpenAlex (CC0) second data source: live abstracts, authors, and citation
// counts for NEJM works that the Cloudflare-gated NEJM HTML transport cannot
// deliver. Uses only stdlib HTTP against api.openalex.org.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

const (
	// openAlexNEJMISSN identifies NEJM in OpenAlex's primary_location filter.
	openAlexNEJMISSN = "0028-4793"
	openAlexWorksURL = "https://api.openalex.org/works"
	// openAlexMailtoEnv names the env var whose email enrolls requests in
	// OpenAlex's polite pool (a dedicated, faster server pool). Unset,
	// requests use the anonymous pool — no identity is ever baked in.
	openAlexMailtoEnv  = "NEJM_OPENALEX_MAILTO"
	openAlexMaxPerPage = 50
	openAlexDefPerPage = 20
)

// openAlexSetMailto adds the polite-pool mailto param only when the user has
// opted in via env; a hardcoded address would ship personal data in every
// request URL.
func openAlexSetMailto(params url.Values) {
	if m := strings.TrimSpace(os.Getenv(openAlexMailtoEnv)); m != "" {
		params.Set("mailto", m)
	}
}

// openAlexResponse mirrors the subset of the OpenAlex /works envelope this
// command consumes.
type openAlexResponse struct {
	Results []openAlexWork `json:"results"`
	Meta    struct {
		Count int `json:"count"`
	} `json:"meta"`
}

type openAlexWork struct {
	ID                    string           `json:"id"`
	DOI                   string           `json:"doi"`
	Title                 string           `json:"title"`
	PublicationYear       int              `json:"publication_year"`
	PublicationDate       string           `json:"publication_date"`
	CitedByCount          int              `json:"cited_by_count"`
	AbstractInvertedIndex map[string][]int `json:"abstract_inverted_index"`
	Authorships           []struct {
		Author struct {
			DisplayName string `json:"display_name"`
		} `json:"author"`
	} `json:"authorships"`
	PrimaryLocation struct {
		LandingPageURL string `json:"landing_page_url"`
	} `json:"primary_location"`
	OpenAccess struct {
		IsOA bool `json:"is_oa"`
	} `json:"open_access"`
}

// openAlexArticle is the flattened output row.
type openAlexArticle struct {
	Title     string `json:"title"`
	Authors   string `json:"authors"`
	Year      int    `json:"year"`
	Date      string `json:"date"`
	DOI       string `json:"doi"`
	Abstract  string `json:"abstract"`
	Citations int    `json:"citations"`
	URL       string `json:"url"`
	IsOA      bool   `json:"is_oa"`
}

// openAlexDecodeAbstract reconstructs abstract text from OpenAlex's
// inverted index (word -> positions), the only form the API returns.
func openAlexDecodeAbstract(inv map[string][]int) string {
	if len(inv) == 0 {
		return ""
	}
	type wordPos struct {
		word string
		pos  int
	}
	var all []wordPos
	for word, positions := range inv {
		for _, p := range positions {
			all = append(all, wordPos{word, p})
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].pos < all[j].pos })
	var sb strings.Builder
	for i, w := range all {
		if i > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(w.word)
	}
	return sb.String()
}

// openAlexAuthors renders up to four author names, then "et al.".
func openAlexAuthors(w openAlexWork) string {
	var names []string
	for _, a := range w.Authorships {
		if a.Author.DisplayName != "" {
			names = append(names, a.Author.DisplayName)
		}
	}
	if len(names) > 4 {
		return strings.Join(names[:4], ", ") + " et al."
	}
	return strings.Join(names, ", ")
}

func newOpenAlexCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "openalex",
		Short:       "Query OpenAlex (CC0) as a second, copyright-clean data source for NEJM works.",
		Long:        "Live search of the OpenAlex scholarly index filtered to NEJM (ISSN 0028-4793). Returns abstracts, author lists, citation counts, and open-access flags that the NEJM website transport cannot deliver. Data is CC0-licensed and requires no local sync.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newOpenAlexSearchCmd(flags))
	return cmd
}

func newOpenAlexSearchCmd(flags *rootFlags) *cobra.Command {
	var query string
	var fromYear, toYear, perPage int
	var sortBy string

	cmd := &cobra.Command{
		Use:         "search",
		Short:       "Search NEJM works in OpenAlex: abstracts, authors, citation counts, OA flags.",
		Long:        "Queries api.openalex.org/works live, filtered to NEJM by ISSN, with optional full-text search and publication-year range. Sort by citation count (cited) or recency (date).",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  nejm-pp-cli openalex search --query \"vitamin D\" --per-page 10 --sort cited\n  nejm-pp-cli openalex search --from-year 2015 --to-year 2026 --sort date --json",
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if perPage < 1 || perPage > openAlexMaxPerPage {
				return usageErr(fmt.Errorf("invalid --per-page %d: must be 1-%d", perPage, openAlexMaxPerPage))
			}
			if sortBy != "cited" && sortBy != "date" {
				return usageErr(fmt.Errorf("invalid --sort %q: must be cited or date", sortBy))
			}
			if toYear > 0 && fromYear <= 0 {
				return usageErr(fmt.Errorf("--to-year requires --from-year"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			filters := []string{"primary_location.source.issn:" + openAlexNEJMISSN}
			switch {
			case fromYear > 0 && toYear > 0:
				filters = append(filters, fmt.Sprintf("publication_year:%d-%d", fromYear, toYear))
			case fromYear > 0:
				filters = append(filters, fmt.Sprintf("publication_year:%d", fromYear))
			}

			params := url.Values{}
			params.Set("filter", strings.Join(filters, ","))
			params.Set("per-page", fmt.Sprintf("%d", perPage))
			if query != "" {
				params.Set("search", query)
			}
			if sortBy == "date" {
				params.Set("sort", "publication_date:desc")
			} else {
				params.Set("sort", "cited_by_count:desc")
			}
			openAlexSetMailto(params)

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, openAlexWorksURL+"?"+params.Encode(), nil)
			if err != nil {
				return err
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return classifyAPIError(fmt.Errorf("querying OpenAlex: %w", err), flags)
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return classifyAPIError(fmt.Errorf("reading OpenAlex response: %w", err), flags)
			}
			if resp.StatusCode != http.StatusOK {
				return classifyAPIError(fmt.Errorf("OpenAlex request failed: HTTP %d: %s", resp.StatusCode, truncate(strings.TrimSpace(string(body)), 200)), flags)
			}

			var oaResp openAlexResponse
			if err := json.Unmarshal(body, &oaResp); err != nil {
				return apiErr(fmt.Errorf("parsing OpenAlex response: %w", err))
			}

			articles := make([]openAlexArticle, 0, len(oaResp.Results))
			for _, wk := range oaResp.Results {
				doi := strings.TrimPrefix(wk.DOI, "https://doi.org/")
				u := wk.PrimaryLocation.LandingPageURL
				if u == "" && doi != "" {
					u = "https://doi.org/" + doi
				}
				articles = append(articles, openAlexArticle{
					Title:     wk.Title,
					Authors:   openAlexAuthors(wk),
					Year:      wk.PublicationYear,
					Date:      wk.PublicationDate,
					DOI:       doi,
					Abstract:  openAlexDecodeAbstract(wk.AbstractInvertedIndex),
					Citations: wk.CitedByCount,
					URL:       u,
					IsOA:      wk.OpenAccess.IsOA,
				})
			}
			if len(articles) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "hint: no OpenAlex results. Broaden --query or widen the --from-year/--to-year range.")
			}
			return printJSONFiltered(cmd.OutOrStdout(), articles, flags)
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Full-text search string (title, abstract, fulltext)")
	cmd.Flags().IntVar(&fromYear, "from-year", 0, "Earliest publication year (0 = no lower bound)")
	cmd.Flags().IntVar(&toYear, "to-year", 0, "Latest publication year (requires --from-year)")
	cmd.Flags().IntVar(&perPage, "per-page", openAlexDefPerPage, "Results per page (1-50)")
	cmd.Flags().StringVar(&sortBy, "sort", "cited", "Sort order: cited (citation count) or date (newest first)")
	return cmd
}
