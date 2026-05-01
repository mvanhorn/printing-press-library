// Hand-written: legal command. Companies House (UK) + SEC EDGAR Form D
// issuer fields (US).

package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/company-goat/internal/source/ch"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/company-goat/internal/source/sec"
	"github.com/spf13/cobra"
)

type legalResult struct {
	Domain string   `json:"domain"`
	Region string   `json:"region"`
	UK     *ukLegal `json:"uk,omitempty"`
	US     *usLegal `json:"us,omitempty"`
	Note   string   `json:"note,omitempty"`
}

type ukLegal struct {
	CompanyNumber string       `json:"company_number"`
	CompanyName   string       `json:"company_name"`
	Status        string       `json:"status"`
	Type          string       `json:"type"`
	Created       string       `json:"date_of_creation"`
	Address       string       `json:"address"`
	Officers      []ch.Officer `json:"officers,omitempty"`
}

type usLegal struct {
	IssuerName string `json:"issuer_name"`
	State      string `json:"state_of_inc"`
	EntityType string `json:"entity_type"`
	YearOfInc  string `json:"year_of_inc,omitempty"`
	Source     string `json:"source"` // "SEC EDGAR Form D"
}

func newLegalCmd(flags *rootFlags) *cobra.Command {
	var t targetFlags
	var region string

	cmd := &cobra.Command{
		Use:   "legal [co]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Legal entity lookup. UK via Companies House (optional COMPANIES_HOUSE_API_KEY); US via SEC EDGAR Form D issuer fields.",
		Long: `legal returns the structured legal-entity record for a company.

Region routing:
  --region uk   query Companies House (requires COMPANIES_HOUSE_API_KEY)
  --region us   pull issuer fields from the most recent SEC Form D filing
  --region auto (default)  try UK if a Companies House key is configured;
                otherwise US.

UK lookup includes officers; US lookup is just issuer name, state of incorporation, and entity type as filed on Form D.`,
		Example: strings.Trim(`
  company-goat-pp-cli legal monzo --region uk
  company-goat-pp-cli legal anthropic --region us --json
  company-goat-pp-cli legal stripe
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(cmd, flags) {
				return nil
			}
			if t.Domain == "" && len(args) == 0 {
				return cmd.Help()
			}
			domain, err := runResolveOrExit(cmd, flags, args, t)
			if err != nil {
				return err
			}
			result := legalResult{Domain: domain}
			region = strings.ToLower(strings.TrimSpace(region))
			if region == "" {
				region = "auto"
			}

			// US path.
			if region == "us" || region == "auto" {
				stem := strings.SplitN(domain, ".", 2)[0]
				secCli := sec.NewClient(getContactEmail(flags))
				ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
				defer cancel()
				filings, err := secCli.SearchAndFetchAll(ctx, stem, 1)
				if err == nil && len(filings) > 0 {
					f := filings[0]
					result.Region = "us"
					result.US = &usLegal{
						IssuerName: f.EntityName,
						State:      f.State,
						EntityType: f.EntityType,
						YearOfInc:  f.YearOfInc,
						Source:     "SEC EDGAR Form D (most recent filing)",
					}
				} else if err != nil && region == "us" {
					return classifyAPIError(fmt.Errorf("sec edgar: %w", err))
				} else if err != nil && region == "auto" {
					result.Note = "US SEC lookup failed: " + err.Error()
				}
			}

			// UK path. Only run if explicitly requested or auto with no US hit and key available.
			runUK := region == "uk"
			if region == "auto" && result.US == nil {
				runUK = true
			}
			if runUK {
				chCli := ch.NewClient()
				if !chCli.HasKey() {
					if region == "uk" {
						return errors.New(`legal --region uk requires COMPANIES_HOUSE_API_KEY.
Register at https://developer.companieshouse.gov.uk (free), create a REST application, then:
  export COMPANIES_HOUSE_API_KEY=<your-key>`)
					}
					if result.US == nil {
						result.Note = "no US Form D issuer found; UK lookup skipped (set COMPANIES_HOUSE_API_KEY for UK)"
					}
				} else {
					ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
					defer cancel()
					name := strings.Join(args, " ")
					if name == "" {
						name = strings.SplitN(domain, ".", 2)[0]
					}
					hits, err := chCli.Search(ctx, name, 5)
					if err != nil {
						result.Note = "Companies House search failed: " + err.Error()
					} else if len(hits) == 0 {
						if result.US == nil {
							result.Note = fmt.Sprintf("no Companies House match for %q", name)
						}
					} else {
						top := hits[0]
						profile, _ := chCli.GetProfile(ctx, top.CompanyNumber)
						officers, _ := chCli.ListOfficers(ctx, top.CompanyNumber, 10)
						uk := &ukLegal{
							CompanyNumber: top.CompanyNumber,
							CompanyName:   top.Title,
							Status:        top.CompanyStatus,
							Type:          top.CompanyType,
							Created:       top.DateOfCreation,
							Address:       top.AddressSnippet,
							Officers:      officers,
						}
						if profile != nil {
							uk.CompanyName = profile.CompanyName
						}
						result.UK = uk
						if result.Region == "" {
							result.Region = "uk"
						} else {
							result.Region = "us+uk"
						}
					}
				}
			}

			if result.US == nil && result.UK == nil {
				if result.Note == "" {
					result.Note = fmt.Sprintf("no legal entity record found for %s in %s", domain, region)
				}
			}
			renderLegal(cmd, flags, result)
			return nil
		},
	}
	cmd.Flags().StringVar(&t.Domain, "domain", "", "Skip name resolution and use this domain (e.g. stripe.com)")
	cmd.Flags().IntVar(&t.Pick, "pick", 0, "Pick candidate N (1-indexed) from a previous ambiguous resolve")
	cmd.Flags().StringVar(&region, "region", "auto", "Region: uk, us, or auto")
	return cmd
}

func renderLegal(cmd *cobra.Command, flags *rootFlags, r legalResult) {
	w := cmd.OutOrStdout()
	asJSON := flags.asJSON || !isTerminal(w)
	if asJSON {
		_ = flags.printJSON(cmd, r)
		return
	}
	fmt.Fprintf(w, "Domain: %s  Region: %s\n", r.Domain, r.Region)
	if r.US != nil {
		fmt.Fprintf(w, "\nUS legal (via %s):\n", r.US.Source)
		fmt.Fprintf(w, "  Issuer:        %s\n", r.US.IssuerName)
		fmt.Fprintf(w, "  Entity type:   %s\n", r.US.EntityType)
		fmt.Fprintf(w, "  State of inc:  %s\n", r.US.State)
		if r.US.YearOfInc != "" {
			fmt.Fprintf(w, "  Year of inc:   %s\n", r.US.YearOfInc)
		}
	}
	if r.UK != nil {
		fmt.Fprintf(w, "\nUK legal (via Companies House):\n")
		fmt.Fprintf(w, "  Number:    %s\n", r.UK.CompanyNumber)
		fmt.Fprintf(w, "  Name:      %s\n", r.UK.CompanyName)
		fmt.Fprintf(w, "  Status:    %s\n", r.UK.Status)
		fmt.Fprintf(w, "  Type:      %s\n", r.UK.Type)
		fmt.Fprintf(w, "  Created:   %s\n", r.UK.Created)
		fmt.Fprintf(w, "  Address:   %s\n", r.UK.Address)
		if len(r.UK.Officers) > 0 {
			fmt.Fprintf(w, "  Officers:\n")
			for _, o := range r.UK.Officers {
				fmt.Fprintf(w, "    - %-30s  %s\n", o.Name, o.OfficerRole)
			}
		}
	}
	if r.Note != "" {
		fmt.Fprintf(w, "\nNote: %s\n", r.Note)
	}
}
