// Hand-authored: novel feature `outreach map`.
//
// User-authored mapping between Prospeo response fields and outreach.people /
// outreach.companies columns. The mapping is the single source of truth for
// how `person funnel` (and future writers) populate the outreach schema.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/prospeo/internal/supa"
)

// ErrMappingNotFound is returned by LoadOutreachMapping when the mapping
// file does not exist.
var ErrMappingNotFound = errors.New("outreach mapping not found")

// OutreachMapping is the resolved mapping document.
type OutreachMapping struct {
	People    EntityMapping `yaml:"people"`
	Companies EntityMapping `yaml:"companies"`
}

// EntityMapping describes one entity's Prospeo->outreach mapping.
type EntityMapping struct {
	Table        string         `yaml:"table"`
	UpsertKey    string         `yaml:"upsert_key"`
	IdentityKeys []string       `yaml:"identity_keys"`
	Fields       []FieldMapping `yaml:"fields"`
}

// FieldMapping is one Prospeo->column mapping row.
type FieldMapping struct {
	Prospeo string `yaml:"prospeo"`
	Column  string `yaml:"column"`
}

// Internal Prospeo schemas. Dotted paths denote nested fields.
var prospeoPersonFields = []string{
	"person_id", "first_name", "last_name", "full_name", "linkedin_url",
	"current_job_title", "headline",
	"email.email", "email.verification_status", "email.verification_method", "email.mx_provider",
	"mobile.number", "mobile.status",
	"location.country_code", "location.state", "location.city", "location.timezone",
}

var prospeoCompanyFields = []string{
	"company_id", "company_website", "linkedin_company_url", "company_name",
	"industry", "description", "description_seo", "country_code",
	"employee_count", "employee_range", "technology_list", "funding", "job_postings",
}

func defaultMappingPath() string {
	if p := os.Getenv("PROSPEO_MAPPING_PATH"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".config", "prospeo-pp-cli", "outreach-mapping.md")
	}
	return filepath.Join(home, ".config", "prospeo-pp-cli", "outreach-mapping.md")
}

func newOutreachCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "outreach",
		Short: "Manage the Prospeo->outreach column mapping used by writers (person funnel, etc).",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newOutreachMapCmd(flags))
	cmd.AddCommand(newOutreachReviewCmd(flags))
	cmd.AddCommand(newOutreachResetCmd(flags))
	return cmd
}

func newOutreachMapCmd(flags *rootFlags) *cobra.Command {
	var nonInteractive, force bool
	cmd := &cobra.Command{
		Use:   "map",
		Short: "Interactive wizard that probes the outreach schema and writes a Prospeo->outreach mapping file.",
		Long: `Probes the live outreach schema via PostgREST OpenAPI, matches each Prospeo
field to a column, and writes the mapping to ~/.config/prospeo-pp-cli/outreach-mapping.md
(override with PROSPEO_MAPPING_PATH).`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would probe outreach schema and write mapping to %s\n", defaultMappingPath())
				return nil
			}
			path := defaultMappingPath()
			if _, err := os.Stat(path); err == nil && !force && !flags.yes {
				if nonInteractive || flags.noInput {
					return fmt.Errorf("mapping already exists at %s (use --force to overwrite)", path)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Mapping exists at %s. Re-create? [y/N]: ", path)
				var ans string
				_, _ = fmt.Fscanln(cmd.InOrStdin(), &ans)
				if !strings.EqualFold(strings.TrimSpace(ans), "y") {
					fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
					return nil
				}
			}
			sc, err := requireSupa()
			if err != nil {
				return err
			}
			schema, err := probeOutreachSchema(cmd.Context(), sc)
			if err != nil {
				return fmt.Errorf("probing outreach schema: %w (see README setup step for exposing outreach schema via PGRST_DB_SCHEMAS)", err)
			}
			people := schema["people"]
			companies := schema["companies"]
			if len(people) == 0 || len(companies) == 0 {
				return fmt.Errorf("outreach schema reachable but people/companies not found (people=%d cols, companies=%d cols)", len(people), len(companies))
			}

			interactive := !(nonInteractive || flags.noInput)
			peopleMap := buildEntityMapping(cmd, "people", prospeoPersonFields, people, "linkedin_url", interactive)
			companiesMap := buildEntityMapping(cmd, "companies", prospeoCompanyFields, companies, "domain", interactive)

			doc := &OutreachMapping{People: peopleMap, Companies: companiesMap}
			if err := writeMappingFile(path, doc); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Wrote mapping to %s\n", path)
			return nil
		},
	}
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "Pick the best-match column for every field without prompting.")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing mapping without prompting.")
	return cmd
}

func newOutreachReviewCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "review",
		Short:       "Print the current Prospeo->outreach mapping and warn on columns missing from the live schema.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := LoadOutreachMapping()
			if err != nil {
				return err
			}
			out := map[string]any{
				"path":      defaultMappingPath(),
				"people":    m.People,
				"companies": m.Companies,
			}
			// Best-effort schema check; if Supabase not reachable, skip warnings.
			if supa.IsConfigured() {
				if cfg, lerr := supa.LoadConfig(); lerr == nil {
					sc := supa.New(cfg)
					if schema, serr := probeOutreachSchema(cmd.Context(), sc); serr == nil {
						out["warnings"] = collectMappingWarnings(m, schema)
					}
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

func newOutreachResetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "reset",
		Short:       "Delete the Prospeo->outreach mapping file.",
		Example:     "  # Remove the current outreach column mapping\n  prospeo-pp-cli outreach reset --yes",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := defaultMappingPath()
			if _, err := os.Stat(path); err != nil {
				if os.IsNotExist(err) {
					fmt.Fprintln(cmd.OutOrStdout(), "no mapping file to delete")
					return nil
				}
				return err
			}
			if !flags.yes && !flags.noInput {
				fmt.Fprintf(cmd.OutOrStdout(), "Delete %s? [y/N]: ", path)
				var ans string
				_, _ = fmt.Fscanln(cmd.InOrStdin(), &ans)
				if !strings.EqualFold(strings.TrimSpace(ans), "y") {
					fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
					return nil
				}
			}
			if err := os.Remove(path); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted %s\n", path)
			return nil
		},
	}
	return cmd
}

func collectMappingWarnings(m *OutreachMapping, schema map[string][]string) []string {
	var warns []string
	check := func(label string, em EntityMapping) {
		cols := schema[em.Table]
		set := map[string]struct{}{}
		for _, c := range cols {
			set[c] = struct{}{}
		}
		for _, f := range em.Fields {
			if _, ok := set[f.Column]; !ok {
				warns = append(warns, fmt.Sprintf("%s.%s not present in live schema", label, f.Column))
			}
		}
	}
	check("people", m.People)
	check("companies", m.Companies)
	return warns
}

// probeOutreachSchema returns the column list per table by reading the
// PostgREST OpenAPI document for the outreach schema.
func probeOutreachSchema(ctx context.Context, sc *supa.Client) (map[string][]string, error) {
	// We use the supa client's underlying config to dial the OpenAPI root.
	// supa.Client doesn't expose a generic GET to a non-table path, so we
	// build our own request using the same auth headers.
	cfg, err := supa.LoadConfig()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", cfg.URL+"/rest/v1/", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", cfg.ServiceKey)
	req.Header.Set("Authorization", "Bearer "+cfg.ServiceKey)
	req.Header.Set("Accept", "application/openapi+json")
	req.Header.Set("Accept-Profile", cfg.Schema)
	httpc := &http.Client{Timeout: cfg.Timeout}
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("openapi probe -> %d: %s", resp.StatusCode, string(body))
	}
	var doc struct {
		Definitions map[string]struct {
			Properties map[string]any `json:"properties"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("decode openapi: %w", err)
	}
	out := map[string][]string{}
	for table, def := range doc.Definitions {
		cols := make([]string, 0, len(def.Properties))
		for c := range def.Properties {
			cols = append(cols, c)
		}
		sort.Strings(cols)
		out[table] = cols
	}
	// Silence the unused url import if Go vet complains.
	_ = url.Values{}
	return out, nil
}

func buildEntityMapping(cmd *cobra.Command, label string, prospeoFields, cols []string, defaultIdentity string, interactive bool) EntityMapping {
	colSet := map[string]struct{}{}
	for _, c := range cols {
		colSet[c] = struct{}{}
	}
	em := EntityMapping{Table: label}
	for _, pf := range prospeoFields {
		candidates := matchCandidates(pf, cols)
		var picked string
		switch {
		case len(candidates) == 1:
			picked = candidates[0]
		case len(candidates) == 0:
			if !interactive {
				continue
			}
			picked = promptColumn(cmd, label, pf, cols)
		default:
			if !interactive {
				picked = candidates[0]
			} else {
				picked = promptColumn(cmd, label, pf, candidates)
			}
		}
		if picked == "" {
			continue
		}
		em.Fields = append(em.Fields, FieldMapping{Prospeo: pf, Column: picked})
	}
	// Upsert key
	em.UpsertKey = "external_id"
	if _, ok := colSet["external_id"]; !ok {
		// fall back to first column that looks like an id
		em.UpsertKey = ""
		for _, c := range cols {
			if c == "id" {
				em.UpsertKey = c
				break
			}
		}
	}
	if _, ok := colSet[defaultIdentity]; ok {
		em.IdentityKeys = []string{defaultIdentity}
	}
	return em
}

// matchCandidates returns the columns that plausibly correspond to the
// dotted Prospeo field name.
func matchCandidates(prospeoField string, cols []string) []string {
	base := prospeoField
	if i := strings.LastIndex(prospeoField, "."); i >= 0 {
		base = prospeoField[i+1:]
	}
	candidates := []string{}
	// Exact match
	for _, c := range cols {
		if c == base {
			candidates = append(candidates, c)
		}
	}
	if len(candidates) > 0 {
		return candidates
	}
	// snake_case variants (already snake-case so try replacements common
	// to nested forms: email.verification_status -> email_status)
	flat := strings.ReplaceAll(prospeoField, ".", "_")
	for _, c := range cols {
		if c == flat {
			candidates = append(candidates, c)
		}
	}
	if len(candidates) > 0 {
		return candidates
	}
	// Special-case shortenings
	shorten := map[string]string{
		"email.verification_status": "email_status",
		"email.verification_method": "email_verification_method",
		"email.mx_provider":         "email_mx_provider",
		"email.email":               "email",
		"mobile.number":             "mobile",
		"mobile.status":             "mobile_status",
		"location.country_code":     "country_code",
		"location.state":            "state",
		"location.city":             "city",
		"location.timezone":         "timezone",
		"person_id":                 "external_id",
		"company_id":                "external_id",
		"company_website":           "domain",
		"company_name":              "name",
	}
	if want, ok := shorten[prospeoField]; ok {
		for _, c := range cols {
			if c == want {
				return []string{c}
			}
		}
	}
	// Substring match (last resort)
	for _, c := range cols {
		if strings.Contains(c, base) || strings.Contains(base, c) {
			candidates = append(candidates, c)
		}
	}
	return candidates
}

func promptColumn(cmd *cobra.Command, label, prospeoField string, options []string) string {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "\n[%s] Prospeo field %q\n", label, prospeoField)
	fmt.Fprintln(w, "  0) skip")
	for i, opt := range options {
		fmt.Fprintf(w, "  %d) %s\n", i+1, opt)
	}
	fmt.Fprint(w, "Choose: ")
	var ans string
	_, _ = fmt.Fscanln(cmd.InOrStdin(), &ans)
	ans = strings.TrimSpace(ans)
	if ans == "" || ans == "0" {
		return ""
	}
	for i, opt := range options {
		if ans == fmt.Sprintf("%d", i+1) {
			return opt
		}
		if ans == opt {
			return opt
		}
	}
	return ""
}

func writeMappingFile(path string, doc *OutreachMapping) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	yamlBytes, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	content := fmt.Sprintf(`# Prospeo -> outreach mapping

Generated by prospeo-pp-cli on %s. Hand-edit safely - re-running
`+"`prospeo-pp-cli outreach map --review`"+` validates the current mapping.

`+"```yaml\n%s```\n\n# Notes\n\n(human notes section, ignored by parser)\n",
		time.Now().UTC().Format("2006-01-02"), string(yamlBytes))
	return os.WriteFile(path, []byte(content), 0o644)
}

var yamlFenceRe = regexp.MustCompile("(?s)```ya?ml\\s*\\n(.*?)```")

// LoadOutreachMapping reads the mapping file, parses the YAML inside, and
// returns the resolved struct. Returns ErrMappingNotFound if the file does
// not exist.
func LoadOutreachMapping() (*OutreachMapping, error) {
	path := defaultMappingPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrMappingNotFound
		}
		return nil, err
	}
	m := yamlFenceRe.FindSubmatch(raw)
	if len(m) < 2 {
		return nil, fmt.Errorf("no yaml fenced block found in %s", path)
	}
	var doc OutreachMapping
	if err := yaml.Unmarshal(m[1], &doc); err != nil {
		return nil, fmt.Errorf("parse mapping yaml: %w", err)
	}
	return &doc, nil
}

// ApplyPersonMapping converts a parsed /enrich-person response into a row
// keyed by outreach column names per the mapping. Returns the row and the
// upsert key column name.
func ApplyPersonMapping(m *OutreachMapping, prospeoResp map[string]any) (map[string]any, string, error) {
	if m == nil {
		return nil, "", fmt.Errorf("nil mapping")
	}
	src, _ := prospeoResp["person"].(map[string]any)
	if src == nil {
		// Some Prospeo endpoints return the person at the top level.
		src = prospeoResp
	}
	row := applyFields(src, m.People.Fields)
	return row, m.People.UpsertKey, nil
}

// ApplyCompanyMapping is the company counterpart of ApplyPersonMapping.
func ApplyCompanyMapping(m *OutreachMapping, prospeoResp map[string]any) (map[string]any, string, error) {
	if m == nil {
		return nil, "", fmt.Errorf("nil mapping")
	}
	src, _ := prospeoResp["company"].(map[string]any)
	if src == nil {
		src = prospeoResp
	}
	row := applyFields(src, m.Companies.Fields)
	return row, m.Companies.UpsertKey, nil
}

func applyFields(src map[string]any, fields []FieldMapping) map[string]any {
	row := map[string]any{}
	for _, f := range fields {
		v, ok := lookupDotted(src, f.Prospeo)
		if !ok || v == nil {
			continue
		}
		row[f.Column] = v
	}
	return row
}

func lookupDotted(src map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	var cur any = src
	for _, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		v, exists := m[p]
		if !exists {
			return nil, false
		}
		cur = v
	}
	return cur, true
}
