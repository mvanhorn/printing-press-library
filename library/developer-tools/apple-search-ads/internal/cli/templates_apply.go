// Copyright 2026 Ryan Kelley and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// campaignTemplate is a saved campaign structure.
type campaignTemplate struct {
	Name     string                 `json:"name"`
	Campaign map[string]interface{} `json:"campaign"`
	AdGroups []templateAdGroup      `json:"ad_groups"`
	SavedAt  string                 `json:"saved_at,omitempty"`
}

// templateAdGroup is a saved ad group structure within a template.
type templateAdGroup struct {
	Name     string                   `json:"name"`
	Settings map[string]interface{}   `json:"settings"`
	Keywords []map[string]interface{} `json:"keywords"`
}

// templateApplyResult is the result of applying a template.
type templateApplyResult struct {
	TemplateName string              `json:"template_name"`
	OrgID        string              `json:"org_id"`
	Action       string              `json:"action"` // "applied" or "diff"
	Created      []string            `json:"created,omitempty"`
	Diff         []templateDiffEntry `json:"diff,omitempty"`
}

// templateDiffEntry describes a difference between the template and what would be created.
type templateDiffEntry struct {
	Field    string `json:"field"`
	Template string `json:"template_value"`
	Current  string `json:"current_value,omitempty"`
}

func templatesDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "apple-search-ads-pp-cli", "templates")
}

func templatePath(name string) (string, error) {
	base := templatesDir()
	// Resolve and validate the path to prevent traversal attacks.
	resolved := filepath.Clean(filepath.Join(base, name+".json"))
	if !strings.HasPrefix(resolved, base+string(os.PathSeparator)) && resolved != base {
		return "", fmt.Errorf("invalid template name %q: path traversal detected", name)
	}
	return resolved, nil
}

func newNovelTemplatesListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List saved campaign templates",
		Example:     "  apple-search-ads-pp-cli templates list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			dir := templatesDir()
			entries, err := os.ReadDir(dir)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Fprintln(cmd.ErrOrStderr(), "No templates directory found. Use 'templates save' to create one.")
					return printJSONFiltered(cmd.OutOrStdout(), []string{}, flags)
				}
				return fmt.Errorf("reading templates directory %s: %w", dir, err)
			}

			type templateMeta struct {
				Name string `json:"name"`
				Path string `json:"path"`
			}
			var templates []templateMeta
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
					continue
				}
				name := strings.TrimSuffix(entry.Name(), ".json")
				templates = append(templates, templateMeta{
					Name: name,
					Path: filepath.Join(dir, entry.Name()),
				})
			}

			if len(templates) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No templates saved yet. Use 'templates save --name <name> --campaign-id <id>' to create one.")
			}
			return printJSONFiltered(cmd.OutOrStdout(), templates, flags)
		},
	}
	return cmd
}

func newNovelTemplatesSaveCmd(flags *rootFlags) *cobra.Command {
	var flagName string
	var flagCampaignID string

	cmd := &cobra.Command{
		Use:   "save",
		Short: "Save a campaign's structure as a reusable template",
		Long: `Fetch a campaign's ad groups and keywords, then save the structure as a
JSON template file. Use 'templates apply' to deploy the template to other orgs.`,
		Example:     "  apple-search-ads-pp-cli templates save --name brand-baseline --campaign-id 12345",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagName == "" {
				return fmt.Errorf("--name is required")
			}
			if flagCampaignID == "" {
				return fmt.Errorf("--campaign-id is required")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Fetch campaign details.
			campaignData, err := c.Get(cmd.Context(), "/campaigns/"+flagCampaignID, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var campaignMap map[string]interface{}
			if err := json.Unmarshal(campaignData, &campaignMap); err != nil {
				campaignMap = map[string]interface{}{"id": flagCampaignID}
			}
			if data, ok := campaignMap["data"]; ok {
				if m, ok := data.(map[string]interface{}); ok {
					campaignMap = m
				}
			}

			// Fetch ad groups.
			adGroupsData, err := c.Get(cmd.Context(), "/campaigns/"+flagCampaignID+"/adgroups", map[string]string{"limit": "100"})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Parse ad groups.
			var adGroupItems []map[string]interface{}
			if err := extractJSONArray(adGroupsData, &adGroupItems); err != nil {
				adGroupItems = nil
			}

			var adGroups []templateAdGroup
			for _, ag := range adGroupItems {
				agID := templateStringField(ag, "id", "adGroupId")
				agName := templateStringField(ag, "name")

				// Fetch keywords for this ad group.
				var keywords []map[string]interface{}
				kwData, kwErr := c.Get(cmd.Context(), "/campaigns/"+flagCampaignID+"/adgroups/"+agID+"/targetingkeywords", map[string]string{"limit": "500"})
				if kwErr == nil {
					_ = extractJSONArray(kwData, &keywords) // best-effort extraction; empty slice is the safe fallback
				}

				// Strip IDs from keywords (they'll be new in the target org).
				for i := range keywords {
					delete(keywords[i], "id")
					delete(keywords[i], "adGroupId")
					delete(keywords[i], "campaignId")
				}

				// Strip IDs from ad group settings too.
				agCopy := make(map[string]interface{})
				for k, v := range ag {
					if k != "id" && k != "campaignId" {
						agCopy[k] = v
					}
				}

				adGroups = append(adGroups, templateAdGroup{
					Name:     agName,
					Settings: agCopy,
					Keywords: keywords,
				})
			}

			// Build template.
			// Strip campaign ID from the template payload.
			campaignCopy := make(map[string]interface{})
			for k, v := range campaignMap {
				if k != "id" && k != "orgId" {
					campaignCopy[k] = v
				}
			}

			tmpl := campaignTemplate{
				Name:     flagName,
				Campaign: campaignCopy,
				AdGroups: adGroups,
			}

			// Save to disk.
			if err := os.MkdirAll(templatesDir(), 0o750); err != nil {
				return fmt.Errorf("creating templates directory: %w", err)
			}
			data, err := json.MarshalIndent(tmpl, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling template: %w", err)
			}
			path, err := templatePath(flagName)
			if err != nil {
				return err
			}
			if err := os.WriteFile(path, data, 0o600); err != nil {
				return fmt.Errorf("writing template file %s: %w", path, err)
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"status":        "saved",
				"template_name": flagName,
				"path":          path,
				"ad_groups":     len(adGroups),
			}, flags)
		},
	}

	cmd.Flags().StringVar(&flagName, "name", "", "Template name (used as filename)")
	cmd.Flags().StringVar(&flagCampaignID, "campaign-id", "", "Campaign ID to save as template")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("campaign-id")
	return cmd
}

func newNovelTemplatesApplyCmd(flags *rootFlags) *cobra.Command {
	var flagOrgIDs string
	var flagCampaignName string
	var flagDiff bool

	cmd := &cobra.Command{
		Use:   "apply <template-name>",
		Short: "Apply a saved campaign template to one or more org IDs",
		Long: `Apply a saved campaign structure template to create a new campaign in the
target org(s). Use --diff to preview what would be created without making changes.
Use --dry-run to skip all API calls.`,
		Example: `  apple-search-ads-pp-cli templates apply brand-baseline --org-ids 111,222 --diff --dry-run
  apple-search-ads-pp-cli templates apply brand-baseline --campaign-name "Brand Q3" --org-ids 333`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			// --diff makes no API calls, so dry-run must not suppress it.
			if !flagDiff && dryRunOK(flags) {
				return nil
			}

			templateName := args[0]
			path, err := templatePath(templateName)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(path) // #nosec G304 -- path is validated by templatePath to stay within templatesDir()
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("template %q not found at %s — run 'templates list' to see available templates", templateName, path)
				}
				return fmt.Errorf("reading template %s: %w", path, err)
			}

			var tmpl campaignTemplate
			if err := json.Unmarshal(data, &tmpl); err != nil {
				return fmt.Errorf("parsing template %s: %w", path, err)
			}

			// Parse org IDs.
			orgIDs := []string{}
			if flagOrgIDs != "" {
				for _, id := range strings.Split(flagOrgIDs, ",") {
					id = strings.TrimSpace(id)
					if id != "" {
						orgIDs = append(orgIDs, id)
					}
				}
			}

			// Override campaign name if provided.
			if flagCampaignName != "" {
				tmpl.Campaign["name"] = flagCampaignName
			}

			var results []templateApplyResult

			if flagDiff {
				return printJSONFiltered(cmd.OutOrStdout(), buildTemplateDiff(tmpl, orgIDs, templateName), flags)
			}

			// Apply to each org.
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			for _, orgID := range orgIDs {
				campaignPayload := make(map[string]interface{})
				for k, v := range tmpl.Campaign {
					campaignPayload[k] = v
				}
				campaignPayload["orgId"] = orgID

				campaignResp, _, createErr := c.Post(cmd.Context(), "/campaigns", campaignPayload)
				if createErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not create campaign in org %s: %v\n", orgID, createErr)
					continue
				}

				newCampaignID := extractIDFromCreateResponse(campaignResp, "id", "campaignId")
				if newCampaignID == "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "error: campaign POST for org %s returned no campaign ID; skipping ad group creation\n", orgID)
					continue
				}

				var created []string
				created = append(created, fmt.Sprintf("campaign:%s", newCampaignID))

				// Create ad groups and keywords.
				for _, ag := range tmpl.AdGroups {
					agPayload := make(map[string]interface{})
					for k, v := range ag.Settings {
						agPayload[k] = v
					}
					agPayload["campaignId"] = newCampaignID
					agPayload["name"] = ag.Name

					agResp, _, agErr := c.Post(cmd.Context(), "/campaigns/"+newCampaignID+"/adgroups", agPayload)
					if agErr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not create ad group %q: %v\n", ag.Name, agErr)
						continue
					}

					newAGID := extractIDFromCreateResponse(agResp, "id", "adGroupId")
					created = append(created, fmt.Sprintf("ad_group:%s", newAGID))

					// Bulk-create keywords.
					if newAGID != "" && len(ag.Keywords) > 0 {
						var kwCreated int
						for _, kw := range ag.Keywords {
							kwPayload := make(map[string]interface{})
							for k, v := range kw {
								kwPayload[k] = v
							}
							kwPayload["adGroupId"] = newAGID
							_, _, kwErr := c.Post(cmd.Context(), "/campaigns/"+newCampaignID+"/adgroups/"+newAGID+"/targetingkeywords", kwPayload)
							if kwErr != nil {
								fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not create keyword: %v\n", kwErr)
							} else {
								kwCreated++
							}
						}
						created = append(created, fmt.Sprintf("keywords:%d", kwCreated))
					}
				}

				results = append(results, templateApplyResult{
					TemplateName: templateName,
					OrgID:        orgID,
					Action:       "applied",
					Created:      created,
				})
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}

	cmd.Flags().StringVar(&flagOrgIDs, "org-ids", "", "Comma-separated org IDs to apply the template to")
	cmd.Flags().StringVar(&flagCampaignName, "campaign-name", "", "Override the campaign name from the template")
	cmd.Flags().BoolVar(&flagDiff, "diff", false, "Show what would be created without making any API calls")
	_ = cmd.MarkFlagRequired("org-ids")
	return cmd
}

func buildTemplateDiff(tmpl campaignTemplate, orgIDs []string, templateName string) []templateApplyResult {
	var results []templateApplyResult
	for _, orgID := range orgIDs {
		var diffEntries []templateDiffEntry
		if name, ok := tmpl.Campaign["name"].(string); ok {
			diffEntries = append(diffEntries, templateDiffEntry{Field: "campaign.name", Template: name})
		}
		diffEntries = append(diffEntries, templateDiffEntry{
			Field:    "ad_groups.count",
			Template: fmt.Sprintf("%d", len(tmpl.AdGroups)),
		})
		for i, ag := range tmpl.AdGroups {
			diffEntries = append(diffEntries, templateDiffEntry{
				Field:    fmt.Sprintf("ad_groups[%d].name", i),
				Template: ag.Name,
			})
			diffEntries = append(diffEntries, templateDiffEntry{
				Field:    fmt.Sprintf("ad_groups[%d].keywords.count", i),
				Template: fmt.Sprintf("%d", len(ag.Keywords)),
			})
		}
		results = append(results, templateApplyResult{
			TemplateName: templateName,
			OrgID:        orgID,
			Action:       "diff",
			Diff:         diffEntries,
		})
	}
	return results
}

// extractJSONArray tries to extract a JSON array from a wrapped API response.
func extractJSONArray(data json.RawMessage, dest *[]map[string]interface{}) error {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err == nil {
		for _, key := range []string{"data", "items", "adGroups", "keywords"} {
			if raw, ok := top[key]; ok {
				if err := json.Unmarshal(raw, dest); err == nil {
					return nil
				}
			}
		}
	}
	return json.Unmarshal(data, dest)
}

// extractIDFromCreateResponse parses a campaign/ad-group POST response and returns
// the first non-empty ID field found, unwrapping a "data" envelope if present.
func extractIDFromCreateResponse(resp json.RawMessage, keys ...string) string {
	var m map[string]interface{}
	if err := json.Unmarshal(resp, &m); err != nil {
		return ""
	}
	if id := templateStringField(m, keys...); id != "" {
		return id
	}
	if data, ok := m["data"]; ok {
		if nested, ok := data.(map[string]interface{}); ok {
			return templateStringField(nested, keys...)
		}
	}
	return ""
}

func templateStringField(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
			if f, ok := v.(float64); ok {
				return fmt.Sprintf("%.0f", f)
			}
		}
	}
	return ""
}
