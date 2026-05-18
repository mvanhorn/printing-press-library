// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
// PATCH: Add local vibe recipe commands for prompt bundles.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type vibeRecipe struct {
	Name             string   `json:"name"`
	Tags             []string `json:"tags,omitempty"`
	PromptTemplate   string   `json:"prompt_template,omitempty"`
	PersonaID        string   `json:"persona_id,omitempty"`
	ModelVersion     string   `json:"mv,omitempty"`
	MakeInstrumental bool     `json:"make_instrumental,omitempty"`
	CreatedAt        string   `json:"created_at"`
	UpdatedAt        string   `json:"updated_at"`
}

func newVibesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "vibes", Short: "Save and replay local generation recipes"}
	cmd.AddCommand(newVibesListCmd(flags))
	cmd.AddCommand(newVibesSaveCmd(flags))
	cmd.AddCommand(newVibesGetCmd(flags))
	cmd.AddCommand(newVibesDeleteCmd(flags))
	cmd.AddCommand(newVibesUseCmd(flags))
	return cmd
}

func newVibesListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List saved vibe recipes",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openExistingStore(cmd.Context())
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			if s == nil {
				return printJSONFiltered(cmd.OutOrStdout(), []vibeRecipe{}, flags)
			}
			defer s.Close()
			rows, err := s.DB().QueryContext(cmd.Context(), `SELECT data FROM resources WHERE resource_type='vibe' ORDER BY id`)
			if err != nil {
				return fmt.Errorf("querying vibes: %w", err)
			}
			defer rows.Close()
			var recipes []vibeRecipe
			for rows.Next() {
				var raw string
				if err := rows.Scan(&raw); err != nil {
					return fmt.Errorf("scanning vibe: %w", err)
				}
				var r vibeRecipe
				if json.Unmarshal([]byte(raw), &r) == nil {
					recipes = append(recipes, r)
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), recipes, flags)
		},
	}
	return cmd
}

func newVibesSaveCmd(flags *rootFlags) *cobra.Command {
	var tags, promptTemplate, personaID, mv string
	var makeInstrumental bool
	cmd := &cobra.Command{
		Use:   "save <name>",
		Short: "Save a vibe recipe",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			now := time.Now().UTC().Format(time.RFC3339)
			r := vibeRecipe{Name: args[0], Tags: splitList(tags), PromptTemplate: promptTemplate, PersonaID: personaID, ModelVersion: mv, MakeInstrumental: makeInstrumental, CreatedAt: now, UpdatedAt: now}
			s, err := openDefaultStore(cmd.Context())
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer s.Close()
			if err := s.Upsert("vibe", args[0], mustJSON(r)); err != nil {
				return fmt.Errorf("saving vibe %q: %w", args[0], err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), r, flags)
		},
	}
	cmd.Flags().StringVar(&tags, "tags", "", "Comma-separated style tags")
	cmd.Flags().StringVar(&promptTemplate, "prompt-template", "", "Prompt template, optionally containing {topic}")
	cmd.Flags().StringVar(&personaID, "persona-id", "", "Voice persona UUID")
	cmd.Flags().StringVar(&mv, "mv", "", "Model version")
	cmd.Flags().BoolVar(&makeInstrumental, "make-instrumental", false, "Generate without vocals")
	return cmd
}

func newVibesGetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "get <name>",
		Short:       "Show a saved vibe recipe",
		Example:     "  suno-pp-cli vibes get synthwave-banger",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			s, err := openExistingStore(cmd.Context())
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			if s == nil {
				return notFoundErr(fmt.Errorf("vibe %q not found", args[0]))
			}
			defer s.Close()
			raw, err := s.Get("vibe", args[0])
			if err == sql.ErrNoRows {
				return notFoundErr(fmt.Errorf("vibe %q not found", args[0]))
			}
			if err != nil {
				return fmt.Errorf("reading vibe %q: %w", args[0], err)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
}

func newVibesDeleteCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a saved vibe recipe",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			s, err := openDefaultStore(cmd.Context())
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer s.Close()
			res, err := s.DB().ExecContext(cmd.Context(), `DELETE FROM resources WHERE resource_type='vibe' AND id=?`, args[0])
			if err != nil {
				return fmt.Errorf("deleting vibe %q: %w", args[0], err)
			}
			n, _ := res.RowsAffected()
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"deleted": n > 0, "name": args[0]}, flags)
		},
	}
}

func newVibesUseCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "use <name> [topic]",
		Short: "Generate from a saved vibe recipe",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			raw, err := func() (json.RawMessage, error) {
				s, err := openExistingStore(cmd.Context())
				if err != nil {
					return nil, err
				}
				if s == nil {
					return nil, sql.ErrNoRows
				}
				defer s.Close()
				return s.Get("vibe", args[0])
			}()
			if err != nil {
				return fmt.Errorf("reading vibe %q: %w", args[0], err)
			}
			var r vibeRecipe
			if err := json.Unmarshal(raw, &r); err != nil {
				return fmt.Errorf("parsing vibe %q: %w", args[0], err)
			}
			body := map[string]any{}
			if r.PromptTemplate != "" {
				topic := ""
				if len(args) > 1 {
					topic = strings.Join(args[1:], " ")
				}
				body["prompt"] = strings.ReplaceAll(r.PromptTemplate, "{topic}", topic)
			}
			if len(r.Tags) > 0 {
				body["tags"] = strings.Join(r.Tags, ", ")
			}
			if r.PersonaID != "" {
				body["persona_id"] = r.PersonaID
			}
			if r.ModelVersion != "" {
				body["mv"] = r.ModelVersion
			}
			if r.MakeInstrumental {
				body["make_instrumental"] = true
			}
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), body, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, _, err := c.Post("/api/generate/v2-web/", body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	return cmd
}
