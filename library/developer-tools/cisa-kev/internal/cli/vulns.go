package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/cisa-kev/internal/types"
	"github.com/spf13/cobra"
)

const kevFeedPath = "/sites/default/files/feeds/known_exploited_vulnerabilities.json"

type kevCatalogView struct {
	CatalogVersion  string                   `json:"catalogVersion"`
	DateReleased    string                   `json:"dateReleased"`
	Count           int                      `json:"count"`
	Vulnerabilities []types.KevVulnerability `json:"vulnerabilities"`
}

type vulnFilter struct {
	query          string
	vendor         string
	product        string
	cwe            string
	ransomwareUse  string
	addedSince     string
	dueBefore      string
	overdue        bool
	ransomwareOnly bool
	limit          int
}

func newVulnsCmd(flags *rootFlags) *cobra.Command {
	var filter vulnFilter
	cmd := &cobra.Command{
		Use:     "vulns",
		Aliases: []string{"vulnerabilities", "kev"},
		Short:   "Search and triage CISA KEV vulnerabilities",
		Long: `Search and triage the CISA Known Exploited Vulnerabilities catalog.

The generated sites command returns the raw feed. The vulns commands flatten the
feed into analyst-friendly rows for CVE lookup, text search, due-date triage,
vendor/product filtering, CWE filtering, and ransomware-use filtering.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List KEV vulnerabilities with optional filters",
		Example: `  cisa-kev-pp-cli vulns list --limit 10
  cisa-kev-pp-cli vulns list --vendor Ivanti --product Sentry --json
  cisa-kev-pp-cli vulns list --cwe CWE-78 --ransomware-only --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			catalog, err := fetchKevCatalog(cmd.Context(), flags)
			if err != nil {
				return err
			}
			results, err := applyVulnFilter(catalog.Vulnerabilities, filter)
			if err != nil {
				return err
			}
			return printVulnResults(cmd, flags, catalog, results)
		},
	}
	addVulnFilterFlags(listCmd, &filter)

	searchCmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search KEV vulnerabilities",
		Args:  cobra.ExactArgs(1),
		Example: `  cisa-kev-pp-cli vulns search ivanti
  cisa-kev-pp-cli vulns search "PeopleSoft" --json
  cisa-kev-pp-cli vulns search "remote code execution" --limit 5 --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			searchFilter := filter
			searchFilter.query = args[0]
			catalog, err := fetchKevCatalog(cmd.Context(), flags)
			if err != nil {
				return err
			}
			results, err := applyVulnFilter(catalog.Vulnerabilities, searchFilter)
			if err != nil {
				return err
			}
			return printVulnResults(cmd, flags, catalog, results)
		},
	}
	addVulnFilterFlags(searchCmd, &filter)

	getCmd := &cobra.Command{
		Use:   "get <cve-id>",
		Short: "Show one KEV vulnerability by CVE ID",
		Args:  cobra.ExactArgs(1),
		Example: `  cisa-kev-pp-cli vulns get CVE-2026-10520
  cisa-kev-pp-cli vulns get CVE-2026-10520 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			catalog, err := fetchKevCatalog(cmd.Context(), flags)
			if err != nil {
				return err
			}
			want := strings.ToUpper(strings.TrimSpace(args[0]))
			for _, vuln := range catalog.Vulnerabilities {
				if strings.EqualFold(vuln.CveID, want) {
					if flags.asJSON || flags.agent {
						return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
							"meta":   catalogMeta(catalog),
							"result": normalizeVuln(vuln),
						}, flags)
					}
					return printVulnTable(cmd, []types.KevVulnerability{vuln})
				}
			}
			return notFoundErr(fmt.Errorf("%s not found in CISA KEV catalog", want))
		},
	}

	dueCmd := &cobra.Command{
		Use:   "due",
		Short: "List vulnerabilities due soon or overdue",
		Example: `  cisa-kev-pp-cli vulns due --due-before 2026-06-30
  cisa-kev-pp-cli vulns due --overdue --ransomware-only
  cisa-kev-pp-cli vulns due --due-before 2026-06-30 --ransomware-only --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			catalog, err := fetchKevCatalog(cmd.Context(), flags)
			if err != nil {
				return err
			}
			dueFilter := filter
			results, err := applyVulnFilter(catalog.Vulnerabilities, dueFilter)
			if err != nil {
				return err
			}
			sort.SliceStable(results, func(i, j int) bool {
				return results[i].DueDate < results[j].DueDate
			})
			return printVulnResults(cmd, flags, catalog, results)
		},
	}
	addVulnFilterFlags(dueCmd, &filter)
	dueCmd.Flags().BoolVar(&filter.overdue, "overdue", false, "Only include vulnerabilities whose due date is before today")

	cmd.AddCommand(listCmd, searchCmd, getCmd, dueCmd)
	return cmd
}

func addVulnFilterFlags(cmd *cobra.Command, filter *vulnFilter) {
	cmd.Flags().StringVar(&filter.vendor, "vendor", "", "Filter by vendor/project substring")
	cmd.Flags().StringVar(&filter.product, "product", "", "Filter by product substring")
	cmd.Flags().StringVar(&filter.cwe, "cwe", "", "Filter by CWE identifier, for example CWE-79")
	cmd.Flags().StringVar(&filter.ransomwareUse, "ransomware", "", "Filter by ransomware campaign use: Known or Unknown")
	cmd.Flags().BoolVar(&filter.ransomwareOnly, "ransomware-only", false, "Only include vulnerabilities with known ransomware campaign use")
	cmd.Flags().StringVar(&filter.addedSince, "added-since", "", "Only include vulnerabilities added on or after YYYY-MM-DD")
	cmd.Flags().StringVar(&filter.dueBefore, "due-before", "", "Only include vulnerabilities due on or before YYYY-MM-DD")
	cmd.Flags().IntVar(&filter.limit, "limit", 50, "Maximum vulnerabilities to return; 0 means all")
}

func fetchKevCatalog(ctx context.Context, flags *rootFlags) (*kevCatalogView, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	body, err := c.GetNoCache(ctx, kevFeedPath, nil)
	if err != nil {
		return nil, err
	}
	var raw types.KevCatalog
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode CISA KEV catalog: %w", err)
	}
	var vulns []types.KevVulnerability
	if len(raw.Vulnerabilities) > 0 {
		if err := json.Unmarshal(raw.Vulnerabilities, &vulns); err != nil {
			return nil, fmt.Errorf("decode CISA KEV vulnerabilities: %w", err)
		}
	}
	return &kevCatalogView{
		CatalogVersion:  raw.CatalogVersion,
		DateReleased:    raw.DateReleased,
		Count:           raw.Count,
		Vulnerabilities: vulns,
	}, nil
}

func applyVulnFilter(vulns []types.KevVulnerability, filter vulnFilter) ([]types.KevVulnerability, error) {
	addedSince, err := parseOptionalDate(filter.addedSince, "--added-since")
	if err != nil {
		return nil, err
	}
	dueBefore, err := parseOptionalDate(filter.dueBefore, "--due-before")
	if err != nil {
		return nil, err
	}
	today := time.Now().Format("2006-01-02")
	var out []types.KevVulnerability
	for _, vuln := range vulns {
		if filter.query != "" && !vulnMatchesQuery(vuln, filter.query) {
			continue
		}
		if !containsFold(vuln.VendorProject, filter.vendor) {
			continue
		}
		if !containsFold(vuln.Product, filter.product) {
			continue
		}
		if filter.cwe != "" && !vulnHasCWE(vuln, filter.cwe) {
			continue
		}
		if filter.ransomwareOnly && !strings.EqualFold(vuln.KnownRansomwareCampaignUse, "Known") {
			continue
		}
		if filter.ransomwareUse != "" && !strings.EqualFold(vuln.KnownRansomwareCampaignUse, filter.ransomwareUse) {
			continue
		}
		if !addedSince.IsZero() && vuln.DateAdded < addedSince.Format("2006-01-02") {
			continue
		}
		if !dueBefore.IsZero() && vuln.DueDate > dueBefore.Format("2006-01-02") {
			continue
		}
		if filter.overdue && vuln.DueDate >= today {
			continue
		}
		out = append(out, vuln)
		if filter.limit > 0 && len(out) >= filter.limit {
			break
		}
	}
	return out, nil
}

func parseOptionalDate(value, flagName string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, usageErr(fmt.Errorf("%s must use YYYY-MM-DD: %w", flagName, err))
	}
	return parsed, nil
}

func vulnMatchesQuery(vuln types.KevVulnerability, query string) bool {
	haystack := strings.Join([]string{
		vuln.CveID,
		vuln.VendorProject,
		vuln.Product,
		vuln.VulnerabilityName,
		vuln.ShortDescription,
		vuln.RequiredAction,
		vuln.KnownRansomwareCampaignUse,
		vuln.Notes,
		string(vuln.Cwes),
	}, "\n")
	return containsFold(haystack, query)
}

func vulnHasCWE(vuln types.KevVulnerability, want string) bool {
	want = strings.ToUpper(strings.TrimSpace(want))
	var cwes []string
	if err := json.Unmarshal(vuln.Cwes, &cwes); err != nil {
		return false
	}
	for _, cwe := range cwes {
		if strings.EqualFold(cwe, want) {
			return true
		}
	}
	return false
}

func containsFold(value, needle string) bool {
	needle = strings.TrimSpace(needle)
	if needle == "" {
		return true
	}
	return strings.Contains(strings.ToLower(value), strings.ToLower(needle))
}

func printVulnResults(cmd *cobra.Command, flags *rootFlags, catalog *kevCatalogView, results []types.KevVulnerability) error {
	if flags.asJSON || flags.agent {
		normalized := make([]map[string]any, 0, len(results))
		for _, vuln := range results {
			normalized = append(normalized, normalizeVuln(vuln))
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"meta":    catalogMeta(catalog),
			"count":   len(normalized),
			"results": normalized,
		}, flags)
	}
	return printVulnTable(cmd, results)
}

func catalogMeta(catalog *kevCatalogView) map[string]any {
	return map[string]any{
		"catalogVersion": catalog.CatalogVersion,
		"dateReleased":   catalog.DateReleased,
		"total":          catalog.Count,
	}
}

func normalizeVuln(vuln types.KevVulnerability) map[string]any {
	var cwes []string
	_ = json.Unmarshal(vuln.Cwes, &cwes)
	return map[string]any{
		"cveID":                      vuln.CveID,
		"vendorProject":              strings.TrimSpace(vuln.VendorProject),
		"product":                    strings.TrimSpace(vuln.Product),
		"vulnerabilityName":          vuln.VulnerabilityName,
		"dateAdded":                  vuln.DateAdded,
		"dueDate":                    vuln.DueDate,
		"knownRansomwareCampaignUse": vuln.KnownRansomwareCampaignUse,
		"cwes":                       cwes,
		"shortDescription":           vuln.ShortDescription,
		"requiredAction":             vuln.RequiredAction,
		"notes":                      vuln.Notes,
	}
}

func printVulnTable(cmd *cobra.Command, vulns []types.KevVulnerability) error {
	if len(vulns) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No vulnerabilities matched.")
		return nil
	}
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "CVE\tDUE\tRANSOMWARE\tVENDOR\tPRODUCT\tNAME")
	for _, vuln := range vulns {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			vuln.CveID,
			vuln.DueDate,
			vuln.KnownRansomwareCampaignUse,
			strings.TrimSpace(vuln.VendorProject),
			strings.TrimSpace(vuln.Product),
			vuln.VulnerabilityName,
		)
	}
	return w.Flush()
}
