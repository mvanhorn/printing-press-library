// Hand-authored: novel feature `find`.
//
// Search the local cache (Supabase `outreach` schema) for previously
// enriched people and companies without spending a Prospeo credit. ILIKE
// matches across name, title, email, linkedin URL, domain.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/prospeo/internal/supa"
)

// PersonHit is the summary projection for a cached person.
type PersonHit struct {
	ID          string `json:"id"`
	FullName    string `json:"full_name,omitempty"`
	JobTitle    string `json:"current_job_title,omitempty"`
	Email       string `json:"email,omitempty"`
	LinkedInURL string `json:"linkedin_url,omitempty"`
	CompanyID   string `json:"current_company_id,omitempty"`
	CountryCode string `json:"country_code,omitempty"`
}

// CompanyHit is the summary projection for a cached company.
type CompanyHit struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Domain      string `json:"domain,omitempty"`
	Industry    string `json:"industry,omitempty"`
	CountryCode string `json:"country_code,omitempty"`
}

// FindResult is the top-level shape returned by `find`.
type FindResult struct {
	Query     string       `json:"query"`
	People    []PersonHit  `json:"people"`
	Companies []CompanyHit `json:"companies"`
}

func newFindCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "find [query]",
		Short: "Search previously enriched people and companies in the local cache (free, no credits).",
		Long: `Search the local Supabase cache (outreach schema) for previously enriched
people and companies. Matches by ILIKE across name, job title, email,
LinkedIn URL, company name, and domain.

Requires SUPABASE_URL and SUPABASE_SERVICE_KEY in the environment.`,
		Example: "  prospeo-pp-cli find \"acme\"\n  prospeo-pp-cli find --limit 50 jane.doe@acme.com",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.TrimSpace(args[0])
			if query == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would query supabase outreach.people + outreach.companies for ILIKE %q (limit %d)\n", query, limit)
				return nil
			}
			if !supa.IsConfigured() {
				return configErr(fmt.Errorf("SUPABASE_URL and SUPABASE_SERVICE_KEY must be set to use 'find'"))
			}
			cfg, err := supa.LoadConfig()
			if err != nil {
				return configErr(fmt.Errorf("supabase config: %w", err))
			}
			sc := supa.New(cfg)
			result, err := runFind(cmd.Context(), sc, query, limit)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum number of results per category (people, companies).")
	return cmd
}

// runFind executes the two PostgREST queries and assembles the result.
func runFind(ctx context.Context, sc *supa.Client, query string, limit int) (*FindResult, error) {
	if limit <= 0 {
		limit = 25
	}
	wild := "*" + query + "*"
	out := &FindResult{Query: query, People: []PersonHit{}, Companies: []CompanyHit{}}

	// People
	{
		params := url.Values{}
		params.Set("select", "id,full_name,current_job_title,email,linkedin_url,current_company_id,country_code")
		params.Set("or", "(full_name.ilike."+wild+",current_job_title.ilike."+wild+",email.ilike."+wild+",linkedin_url.ilike."+wild+")")
		params.Set("limit", strconv.Itoa(limit))
		raw, err := sc.Select(ctx, "people", params)
		if err != nil {
			return nil, fmt.Errorf("supabase people: %w", err)
		}
		if err := json.Unmarshal(raw, &out.People); err != nil {
			return nil, fmt.Errorf("decode people: %w", err)
		}
	}

	// Companies
	{
		params := url.Values{}
		params.Set("select", "id,name,domain,industry,country_code")
		params.Set("or", "(name.ilike."+wild+",domain.ilike."+wild+")")
		params.Set("limit", strconv.Itoa(limit))
		raw, err := sc.Select(ctx, "companies", params)
		if err != nil {
			return nil, fmt.Errorf("supabase companies: %w", err)
		}
		if err := json.Unmarshal(raw, &out.Companies); err != nil {
			return nil, fmt.Errorf("decode companies: %w", err)
		}
	}

	return out, nil
}
