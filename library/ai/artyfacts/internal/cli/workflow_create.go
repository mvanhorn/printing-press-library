// Copyright 2026 bernard-maltais. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// sectionInput is the YAML/JSON shape for a section in the workflow file.
type sectionInput struct {
	ID        string `yaml:"id"       json:"id"`
	Heading   string `yaml:"heading"  json:"heading"`
	Type      string `yaml:"type"     json:"type"`
	Body      string `yaml:"body"     json:"body"`
	Position  int    `yaml:"position" json:"position"`
	AgentName string `yaml:"agent_name" json:"agent_name"`
}

// workflowFile is the top-level shape of the --sections-file input.
type workflowFile struct {
	Title     string         `yaml:"title"     json:"title"`
	Type      string         `yaml:"type"      json:"type"`
	Summary   string         `yaml:"summary"   json:"summary"`
	Tags      []string       `yaml:"tags"      json:"tags"`
	Status    string         `yaml:"status"    json:"status"`
	ParentID  string         `yaml:"parent_id" json:"parent_id"`
	Sections  []sectionInput `yaml:"sections"  json:"sections"`
}

func newWorkflowCreateCmd(flags *rootFlags) *cobra.Command {
	var flagTitle string
	var flagType string
	var flagSummary string
	var flagSectionsFile string
	var flagAgentName string
	var flagParentID string
	var flagStatus string

	cmd := &cobra.Command{
		Use:   "create-with-sections",
		Short: "Create an artifact and add all sections in one step",
		Long: `Creates an artifact envelope, then writes all sections sequentially.
Sets is_streaming=true before writing sections and flips it to false
when all sections are complete.

Sections can be provided via --sections-file (YAML or JSON) or
via inline flags for a single section.

This combines what requires N+1 MCP tool calls into one CLI invocation.
Ideal for CI pipelines generating structured reports.

Sections file format (YAML):
  title: "My Report"
  type: report
  summary: "Q1 analysis"
  sections:
    - id: overview
      heading: "Overview"
      type: document/markdown
      body: |
        The quick summary here.
    - id: data
      heading: "Data"
      type: data/json
      body: '{"key":"value"}'`,
		Example: `  artyfacts-pp-cli workflow create-with-sections --sections-file report.yaml
  artyfacts-pp-cli workflow create-with-sections --title "Build Report" --type report --summary "CI run"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var wf workflowFile

			if flagSectionsFile != "" {
				data, err := os.ReadFile(flagSectionsFile)
				if err != nil {
					return fmt.Errorf("reading sections file: %w", err)
				}
				ext := strings.ToLower(flagSectionsFile)
				if strings.HasSuffix(ext, ".json") {
					if err := json.Unmarshal(data, &wf); err != nil {
						return fmt.Errorf("parsing JSON sections file: %w", err)
					}
				} else {
					if err := yaml.Unmarshal(data, &wf); err != nil {
						return fmt.Errorf("parsing YAML sections file: %w", err)
					}
				}
			}

			// CLI flags override file values
			if flagTitle != "" {
				wf.Title = flagTitle
			}
			if flagType != "" {
				wf.Type = flagType
			}
			if flagSummary != "" {
				wf.Summary = flagSummary
			}
			if flagParentID != "" {
				wf.ParentID = flagParentID
			}
			if flagStatus != "" {
				wf.Status = flagStatus
			}

			if wf.Title == "" {
				return fmt.Errorf("artifact title is required (--title or 'title:' in sections file)")
			}
			if wf.Type == "" {
				return fmt.Errorf("artifact type is required (--type or 'type:' in sections file)")
			}

			// 1. Create artifact envelope
			artifact := map[string]any{
				"id":      uuid.New().String(),
				"title":   wf.Title,
				"type":    wf.Type,
				"summary": wf.Summary,
			}
			if len(wf.Tags) > 0 {
				artifact["tags"] = wf.Tags
			}
			if wf.ParentID != "" {
				artifact["parent_id"] = wf.ParentID
			}
			if wf.Status != "" {
				artifact["status"] = wf.Status
			}
			createBody := map[string]any{
				"aah_version": "0.5",
				"artifact":    artifact,
			}

			artData, _, err := c.Post("/artifacts", createBody)
			if err != nil {
				return fmt.Errorf("creating artifact: %w", classifyAPIError(err, flags))
			}

			var art map[string]any
			if err := json.Unmarshal(artData, &art); err != nil {
				return fmt.Errorf("parsing artifact response: %w", err)
			}
			artID, _ := art["id"].(string)
			if artID == "" {
				return fmt.Errorf("API returned artifact without id: %s", string(artData))
			}

			fmt.Fprintf(os.Stderr, "Created artifact %s\n", artID)

			// 2. Write sections
			for i, sec := range wf.Sections {
				if sec.ID == "" {
					return fmt.Errorf("section at index %d is missing required 'id' field", i)
				}
				if sec.Heading == "" {
					return fmt.Errorf("section %q is missing required 'heading' field", sec.ID)
				}
				if sec.Type == "" {
					sec.Type = "document/markdown"
				}

				secBody := map[string]any{
					"id":           sec.ID,
					"heading":      sec.Heading,
					"type":         sec.Type,
					"body":         sec.Body,
					"is_streaming": i < len(wf.Sections)-1, // streaming until last section
				}
				if sec.Position > 0 {
					secBody["position"] = sec.Position
				}
				agentName := sec.AgentName
				if agentName == "" {
					agentName = flagAgentName
				}
				if agentName != "" {
					secBody["agent_name"] = agentName
				}

				_, _, secErr := c.Post(fmt.Sprintf("/artifacts/%s/sections", artID), secBody)
				if secErr != nil {
					fmt.Fprintf(os.Stderr, "warning: section %q failed: %v\n", sec.ID, secErr)
					continue
				}
				fmt.Fprintf(os.Stderr, "  Added section %s/%s\n", artID, sec.ID)
			}

			// 3. Ensure last section has is_streaming=false (belt-and-suspenders)
			if len(wf.Sections) > 0 {
				lastSec := wf.Sections[len(wf.Sections)-1]
				_, _, _ = c.Patch(
					fmt.Sprintf("/artifacts/%s/sections/%s", artID, lastSec.ID),
					map[string]any{"is_streaming": false},
				)
			}

			if flags.asJSON {
				return printOutputWithFlags(cmd.OutOrStdout(), artData, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Created artifact %s with %d section(s).\n", artID, len(wf.Sections))
			return nil
		},
	}

	cmd.Flags().StringVar(&flagTitle, "title", "", "Artifact title")
	cmd.Flags().StringVar(&flagType, "type", "", "Artifact type (doc, spec, research, report, etc.)")
	cmd.Flags().StringVar(&flagSummary, "summary", "", "Brief description")
	cmd.Flags().StringVar(&flagSectionsFile, "sections-file", "", "YAML or JSON file with artifact metadata and sections")
	cmd.Flags().StringVar(&flagAgentName, "agent-name", "", "Default agent name for all sections")
	cmd.Flags().StringVar(&flagParentID, "parent-id", "", "Parent folder ID")
	cmd.Flags().StringVar(&flagStatus, "status", "", "Initial status (draft, active, final)")

	return cmd
}
