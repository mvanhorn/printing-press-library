// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/wavespeed/internal/store"
	"github.com/spf13/cobra"
)

type brandFlags struct {
	fromFile     string
	styleAnchors []string
	negative     string
	palette      []string
	voice        string
	models       []string
	platforms    []string
}

func newBrandCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "brand",
		Short: "Manage brand profiles for D2C content production",
		Long:  "Create, inspect, and apply brand profiles. Profiles store style anchors, palette, negative prompts, voice, preferred models and platforms, and default params. They live in the library DB and auto-merge into pack/compose/variants/restyle and (via brand apply) run.",
	}
	cmd.AddCommand(
		newBrandInitCmd(flags),
		newBrandShowCmd(flags),
		newBrandListCmd(flags),
		newBrandApplyCmd(flags),
		newBrandEditCmd(flags),
	)
	return cmd
}

func newBrandInitCmd(flags *rootFlags) *cobra.Command {
	var bf brandFlags
	cmd := &cobra.Command{
		Use:   "init <name>",
		Short: "Create a brand profile (non-interactive by default)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return usageErr(fmt.Errorf("brand name is required"))
			}

			s, err := openLibrary()
			if err != nil {
				return configErr(err)
			}
			defer s.Close()

			if _, err := s.GetBrandProfile(name); err == nil {
				return usageErr(fmt.Errorf("brand %q already exists; use 'brand edit %s' to change it", name, name))
			} else if !errors.Is(err, sql.ErrNoRows) {
				return apiErr(err)
			}

			body, err := brandBodyFromFlags(cmd, &bf, flags)
			if err != nil {
				return err
			}
			data, _ := json.Marshal(body)
			prof, err := s.UpsertBrandProfile(newBrandID(), name, data)
			if err != nil {
				return apiErr(err)
			}

			env := newEnvelope("brand init")
			env.Results = []any{brandResult(prof, body)}
			env.RecommendedAction = fmt.Sprintf("brand apply %s", name)
			env.SuggestedNext = suggestNext(
				fmt.Sprintf("wavespeed-pp-cli brand apply %s", name),
				"wavespeed-pp-cli plan brief-to-shotlist --prompt \"...\"",
			)
			return emitEnvelope(cmd.OutOrStdout(), env)
		},
	}
	addBrandFieldFlags(cmd, &bf)
	return cmd
}

func newBrandShowCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show a brand profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prof, body, err := loadBrandProfile(strings.TrimSpace(args[0]))
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return notFoundErr(fmt.Errorf("brand %q not found; run 'brand list'", args[0]))
				}
				return apiErr(err)
			}
			env := newEnvelope("brand show")
			env.Results = []any{brandResult(prof, body)}
			return emitEnvelope(cmd.OutOrStdout(), env)
		},
	}
}

func newBrandListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List brand profiles",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openLibrary()
			if err != nil {
				return configErr(err)
			}
			defer s.Close()
			profs, err := s.ListBrandProfiles()
			if err != nil {
				return apiErr(err)
			}
			project, _ := loadWavespeedProjectConfig()
			env := newEnvelope("brand list")
			for _, p := range profs {
				var body brandProfileBody
				if len(p.Data) > 0 {
					_ = json.Unmarshal(p.Data, &body)
				}
				r := brandResult(p, body)
				r["active"] = p.Name == project.ActiveBrand
				env.Results = append(env.Results, r)
			}
			return emitEnvelope(cmd.OutOrStdout(), env)
		},
	}
}

func newBrandApplyCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "apply <name>",
		Short: "Set the active brand (writes activeBrand to wavespeed.json)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			if _, _, err := loadBrandProfile(name); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return notFoundErr(fmt.Errorf("brand %q not found; create it with 'brand init %s'", name, name))
				}
				return apiErr(err)
			}
			project, _ := loadWavespeedProjectConfig()
			project.ActiveBrand = name
			path, err := saveWavespeedProjectConfig(project)
			if err != nil {
				return apiErr(err)
			}
			env := newEnvelope("brand apply")
			env.Results = []any{map[string]any{"active_brand": name, "config_path": path}}
			env.SuggestedNext = suggestNext(
				"wavespeed-pp-cli pack --concept \"...\" --platforms instagram,tiktok",
			)
			return emitEnvelope(cmd.OutOrStdout(), env)
		},
	}
}

func newBrandEditCmd(flags *rootFlags) *cobra.Command {
	var bf brandFlags
	cmd := &cobra.Command{
		Use:   "edit <name>",
		Short: "Patch fields on an existing brand profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			s, err := openLibrary()
			if err != nil {
				return configErr(err)
			}
			defer s.Close()
			prof, err := s.GetBrandProfile(name)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return notFoundErr(fmt.Errorf("brand %q not found", name))
				}
				return apiErr(err)
			}
			var body brandProfileBody
			if len(prof.Data) > 0 {
				_ = json.Unmarshal(prof.Data, &body)
			}
			changed := patchBrandBody(cmd, &bf, &body)
			if !changed {
				return usageErr(fmt.Errorf("no fields to edit; pass at least one of --style-anchors, --negative, --palette, --voice, --models, --platforms"))
			}
			data, _ := json.Marshal(body)
			updated, err := s.UpsertBrandProfile(prof.ID, name, data)
			if err != nil {
				return apiErr(err)
			}
			env := newEnvelope("brand edit")
			env.Results = []any{brandResult(updated, body)}
			return emitEnvelope(cmd.OutOrStdout(), env)
		},
	}
	addBrandFieldFlags(cmd, &bf)
	return cmd
}

func addBrandFieldFlags(cmd *cobra.Command, bf *brandFlags) {
	cmd.Flags().StringVar(&bf.fromFile, "from-file", "", "Load profile fields from a JSON file")
	cmd.Flags().StringSliceVar(&bf.styleAnchors, "style-anchors", nil, "Style anchor phrases appended to prompts")
	cmd.Flags().StringVar(&bf.negative, "negative", "", "Negative prompt applied to generations")
	cmd.Flags().StringSliceVar(&bf.palette, "palette", nil, "Brand color palette")
	cmd.Flags().StringVar(&bf.voice, "voice", "", "Brand voice descriptor")
	cmd.Flags().StringSliceVar(&bf.models, "models", nil, "Preferred model IDs (first is the default)")
	cmd.Flags().StringSliceVar(&bf.platforms, "platforms", nil, "Default target platforms")
}

// brandBodyFromFlags builds a profile body from --from-file, field flags, or
// (only when interactive) prompts. The interactive gate is deliberately narrow:
// a TTY stdin AND no --from-file AND no field flags AND not --agent AND not
// --no-input. Agents and pipelines always take the non-interactive path.
func brandBodyFromFlags(cmd *cobra.Command, bf *brandFlags, flags *rootFlags) (brandProfileBody, error) {
	var body brandProfileBody
	if bf.fromFile != "" {
		raw, err := os.ReadFile(bf.fromFile)
		if err != nil {
			return body, usageErr(fmt.Errorf("reading --from-file: %w", err))
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			return body, usageErr(fmt.Errorf("parsing --from-file: %w", err))
		}
	}
	fieldFlagsSet := patchBrandBody(cmd, bf, &body)

	if bf.fromFile == "" && !fieldFlagsSet && stdinIsTTY() && !flags.agent && !flags.noInput {
		return promptBrandBody(cmd)
	}
	return body, nil
}

// patchBrandBody applies any explicitly-set field flags onto body, returning
// whether anything changed.
func patchBrandBody(cmd *cobra.Command, bf *brandFlags, body *brandProfileBody) bool {
	changed := false
	if cmd.Flags().Changed("style-anchors") {
		body.StyleAnchors = bf.styleAnchors
		changed = true
	}
	if cmd.Flags().Changed("negative") {
		body.Negative = bf.negative
		changed = true
	}
	if cmd.Flags().Changed("palette") {
		body.Palette = bf.palette
		changed = true
	}
	if cmd.Flags().Changed("voice") {
		body.Voice = bf.voice
		changed = true
	}
	if cmd.Flags().Changed("models") {
		body.Models = bf.models
		changed = true
	}
	if cmd.Flags().Changed("platforms") {
		body.Platforms = bf.platforms
		changed = true
	}
	return changed
}

func promptBrandBody(cmd *cobra.Command) (brandProfileBody, error) {
	var body brandProfileBody
	reader := bufio.NewReader(cmd.InOrStdin())
	ask := func(label string) string {
		fmt.Fprintf(cmd.ErrOrStderr(), "%s: ", label)
		line, _ := reader.ReadString('\n')
		return strings.TrimSpace(line)
	}
	body.Voice = ask("Brand voice")
	if v := ask("Style anchors (comma-separated)"); v != "" {
		body.StyleAnchors = splitCSV(v)
	}
	if v := ask("Palette (comma-separated)"); v != "" {
		body.Palette = splitCSV(v)
	}
	body.Negative = ask("Negative prompt")
	if v := ask("Models (comma-separated)"); v != "" {
		body.Models = splitCSV(v)
	}
	if v := ask("Platforms (comma-separated)"); v != "" {
		body.Platforms = splitCSV(v)
	}
	return body, nil
}

func brandResult(prof store.BrandProfile, body brandProfileBody) map[string]any {
	return map[string]any{
		"id":         prof.ID,
		"name":       prof.Name,
		"created_at": prof.CreatedAt,
		"updated_at": prof.UpdatedAt,
		"profile":    body,
	}
}

// saveWavespeedProjectConfig writes the project config back to disk. When no
// wavespeed.json was found, it creates one in the current directory. Returns
// the path written.
func saveWavespeedProjectConfig(project wavespeedProjectConfig) (string, error) {
	path := project.Path
	if path == "" {
		path = "wavespeed.json"
	}
	out := struct {
		DefaultModel string                           `json:"defaultModel,omitempty"`
		OutputDir    string                           `json:"outputDir,omitempty"`
		Aliases      map[string]wavespeedProjectAlias `json:"aliases,omitempty"`
		ActiveBrand  string                           `json:"activeBrand,omitempty"`
		Record       string                           `json:"record,omitempty"`
	}{
		DefaultModel: project.DefaultModel,
		OutputDir:    project.OutputDir,
		Aliases:      project.Aliases,
		ActiveBrand:  project.ActiveBrand,
		Record:       project.Record,
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling project config: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return "", fmt.Errorf("writing %s: %w", path, err)
	}
	return path, nil
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func stdinIsTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
