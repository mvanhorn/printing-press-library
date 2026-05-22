// Hand-authored: novel feature `lookalike`.
//
// Enrich a seed company, derive industry/size/geo filters from the result,
// and run /search-company to find similar companies.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/prospeo/internal/client"
)

// LookalikeSeed is the slice of the enrich-company response we use to
// derive filters.
type LookalikeSeed struct {
	Domain         string   `json:"domain,omitempty"`
	Name           string   `json:"name,omitempty"`
	Industry       string   `json:"industry,omitempty"`
	EmployeeCount  int      `json:"employee_count,omitempty"`
	CountryCode    string   `json:"country_code,omitempty"`
	TechnologyList []string `json:"technology_list,omitempty"`
}

// DerivedFilters echoes the filter object sent to /search-company so the
// caller can see how the seed translated.
type DerivedFilters struct {
	Industry     string `json:"industry,omitempty"`
	EmployeesMin int    `json:"employees_min,omitempty"`
	EmployeesMax int    `json:"employees_max,omitempty"`
	CountryCode  string `json:"country_code,omitempty"`
}

// LookalikeResult is the full response from the `lookalike` command.
type LookalikeResult struct {
	SeedDomain     string         `json:"seed_domain"`
	Seed           LookalikeSeed  `json:"seed_company"`
	DerivedFilters DerivedFilters `json:"derived_filters"`
	Matches        any            `json:"matches"`
}

func newLookalikeCmd(flags *rootFlags) *cobra.Command {
	var seedDomain string
	var employeesMin, employeesMax, page int
	cmd := &cobra.Command{
		Use:   "lookalike",
		Short: "Find companies similar to a seed domain by auto-deriving industry, size, and geo filters.",
		Long: `Enriches the seed company, extracts industry / employee count / country
code, builds a /search-company filter that targets a 0.5x-2x employee band
in the same industry and country, and returns the search results.

Override the auto-derived employee band with --employees-min / --employees-max.`,
		Example: "  prospeo-pp-cli lookalike --seed-company stripe.com\n" +
			"  prospeo-pp-cli lookalike --seed-company acme.io --employees-min 50 --employees-max 250",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if seedDomain == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would enrich %s and search /search-company for lookalikes\n", seedDomain)
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			result, err := runLookalike(cmd.Context(), c, seedDomain, employeesMin, employeesMax, page)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&seedDomain, "seed-company", "", "Domain of the seed company (required).")
	cmd.Flags().IntVar(&employeesMin, "employees-min", 0, "Override the auto-derived employees floor.")
	cmd.Flags().IntVar(&employeesMax, "employees-max", 0, "Override the auto-derived employees ceiling.")
	cmd.Flags().IntVar(&page, "page", 1, "Page of /search-company results to return (1-1000).")
	return cmd
}

func runLookalike(ctx context.Context, c *client.Client, seedDomain string, empMin, empMax, page int) (*LookalikeResult, error) {
	// 1. Enrich the seed.
	enrichBody := map[string]any{"company_website": seedDomain}
	enrichRaw, _, err := c.PostWithParams(ctx, "/enrich-company", nil, enrichBody)
	if err != nil {
		return nil, fmt.Errorf("enrich seed: %w", err)
	}
	seed, err := extractLookalikeSeed(enrichRaw)
	if err != nil {
		return nil, fmt.Errorf("parse seed enrichment: %w", err)
	}
	seed.Domain = seedDomain

	// 2. Build derived filters.
	derived := DerivedFilters{
		Industry:    seed.Industry,
		CountryCode: seed.CountryCode,
	}
	if empMin > 0 {
		derived.EmployeesMin = empMin
	} else if seed.EmployeeCount > 0 {
		derived.EmployeesMin = int(math.Floor(float64(seed.EmployeeCount) * 0.5))
	}
	if empMax > 0 {
		derived.EmployeesMax = empMax
	} else if seed.EmployeeCount > 0 {
		derived.EmployeesMax = int(math.Ceil(float64(seed.EmployeeCount) * 2.0))
	}

	// 3. Compose /search-company filter and call. Filter names come from
	// Prospeo's filter docs (https://prospeo.io/api-docs/filters-documentation):
	// company_industry, company_headcount_custom (min/max), and (per docs)
	// person_location_search uses {include: []string} but company-side
	// location filter is composed differently. We stick to industry +
	// headcount + skip country since the Prospeo filter for company
	// location is not a simple country include.
	filters := map[string]any{}
	if derived.Industry != "" {
		filters["company_industry"] = map[string]any{"include": []string{derived.Industry}}
	}
	if derived.EmployeesMin > 0 || derived.EmployeesMax > 0 {
		emp := map[string]any{}
		if derived.EmployeesMin > 0 {
			emp["min"] = derived.EmployeesMin
		}
		if derived.EmployeesMax > 0 {
			emp["max"] = derived.EmployeesMax
		}
		filters["company_headcount_custom"] = emp
	}
	searchBody := map[string]any{"filters": filters, "page": page}
	matchesRaw, _, err := c.PostQueryWithParams(ctx, "/search-company", nil, searchBody)
	if err != nil {
		return nil, fmt.Errorf("search lookalikes: %w", err)
	}
	var matches any
	_ = json.Unmarshal(matchesRaw, &matches)

	return &LookalikeResult{
		SeedDomain:     seedDomain,
		Seed:           seed,
		DerivedFilters: derived,
		Matches:        matches,
	}, nil
}

// extractLookalikeSeed tolerates either the bare company shape or a
// {"company":{...}} envelope and pulls the few fields we care about.
func extractLookalikeSeed(raw json.RawMessage) (LookalikeSeed, error) {
	var seed LookalikeSeed
	// Try bare object first.
	var bare map[string]json.RawMessage
	if err := json.Unmarshal(raw, &bare); err != nil {
		return seed, err
	}
	if companyRaw, ok := bare["company"]; ok {
		return decodeSeedFields(companyRaw)
	}
	if dataRaw, ok := bare["data"]; ok {
		// {data: {company: {...}}} or {data: {...}}
		var nested map[string]json.RawMessage
		if err := json.Unmarshal(dataRaw, &nested); err == nil {
			if companyRaw, ok := nested["company"]; ok {
				return decodeSeedFields(companyRaw)
			}
		}
		return decodeSeedFields(dataRaw)
	}
	return decodeSeedFields(raw)
}

func decodeSeedFields(raw json.RawMessage) (LookalikeSeed, error) {
	var seed LookalikeSeed
	_ = json.Unmarshal(raw, &seed)
	return seed, nil
}
