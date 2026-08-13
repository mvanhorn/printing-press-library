// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored. `preset` saves and replays a named bundle of generation
// settings to local config so a "house style" can be reused across generate
// and batch without re-typing. Pure local config; never generates or downloads.
//
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// genSettings is the shared bundle of generation parameters used by generate,
// batch, redo, and preset. Zero values mean "unset / use default".
type genSettings struct {
	Tool         string `json:"tool,omitempty"`
	Style        string `json:"style,omitempty"`
	Negative     string `json:"negative_prompt,omitempty"`
	Aspect       string `json:"aspect_ratio,omitempty"`
	Quality      string `json:"quality,omitempty"`
	Quantity     int    `json:"quantity,omitempty"`
	Seed         int    `json:"seed,omitempty"`
	CheckpointID int    `json:"checkpoint_id,omitempty"`
	FolderID     int    `json:"folder_id,omitempty"`
}

// resolveAspect maps an aspect ratio label to the pixel dimensions Artistly
// accepts. An empty label defaults to 1:1; any other unrecognized label is a
// usage error so a typo (e.g. "5:4") fails loudly instead of silently squaring
// the image.
func resolveAspect(aspect string) (string, int, int, error) {
	switch strings.TrimSpace(aspect) {
	case "16:9":
		return "16:9", 1344, 768, nil
	case "9:16":
		return "9:16", 768, 1344, nil
	case "4:3":
		return "4:3", 1152, 896, nil
	case "3:4":
		return "3:4", 896, 1152, nil
	case "3:2":
		return "3:2", 1216, 832, nil
	case "2:3":
		return "2:3", 832, 1216, nil
	case "", "1:1":
		return "1:1", 1024, 1024, nil
	default:
		return "", 0, 0, usageErr(fmt.Errorf("unrecognized --aspect-ratio %q (valid: 1:1, 16:9, 9:16, 4:3, 3:4, 3:2, 2:3)", aspect))
	}
}

func presetPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "artistly-pp-cli", "presets.json"), nil
}

func loadPresets() (map[string]genSettings, error) {
	path, err := presetPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path) // #nosec G304 -- path is the fixed ~/.config/artistly-pp-cli/presets.json, not user-supplied
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]genSettings{}, nil
		}
		return nil, err
	}
	var m map[string]genSettings
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("presets file unreadable: %w", err)
	}
	if m == nil {
		m = map[string]genSettings{}
	}
	return m, nil
}

func savePresets(m map[string]genSettings) error {
	path, err := presetPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// loadPreset returns the named preset, or an error naming the available presets.
func loadPreset(name string) (genSettings, error) {
	m, err := loadPresets()
	if err != nil {
		return genSettings{}, err
	}
	s, ok := m[name]
	if !ok {
		names := make([]string, 0, len(m))
		for k := range m {
			names = append(names, k)
		}
		sort.Strings(names)
		avail := "none saved"
		if len(names) > 0 {
			avail = strings.Join(names, ", ")
		}
		return genSettings{}, usageErr(fmt.Errorf("preset %q not found (available: %s)", name, avail))
	}
	return s, nil
}

func newNovelPresetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "preset",
		Short: "Save and reuse a named bundle of generation settings (style, aspect ratio, negative prompt, quality).",
		Long: strings.Trim(`
Save and reuse generation SETTINGS (not prompts) as a named preset. Apply one
with --preset on 'generate' or 'batch'. This command is local config only; it
never generates images or downloads anything.`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newPresetSaveCmd(flags))
	cmd.AddCommand(newPresetUseCmd(flags))
	cmd.AddCommand(newPresetListCmd(flags))
	cmd.AddCommand(newPresetRemoveCmd(flags))
	return cmd
}

func newPresetSaveCmd(flags *rootFlags) *cobra.Command {
	var s genSettings
	cmd := &cobra.Command{
		Use:         "save <name>",
		Short:       "Save a named generation-settings preset",
		Example:     "  artistly-pp-cli preset save house-style --style watercolor --aspect-ratio 1:1 --quality highQuality",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("preset name is required"))
			}
			if _, _, _, err := resolveAspect(s.Aspect); err != nil {
				return err
			}
			m, err := loadPresets()
			if err != nil {
				return err
			}
			m[args[0]] = s
			if err := savePresets(m); err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"saved": args[0], "settings": s}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved preset %q.\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&s.Tool, "tool", "", "Generator feature slug (default image-designer-v6)")
	cmd.Flags().StringVar(&s.Style, "style", "", "Style label/slug")
	cmd.Flags().StringVar(&s.Negative, "negative-prompt", "", "Negative prompt")
	cmd.Flags().StringVar(&s.Aspect, "aspect-ratio", "", "Aspect ratio (1:1, 16:9, 9:16, 4:3, 3:4, 3:2, 2:3)")
	cmd.Flags().StringVar(&s.Quality, "quality", "", "Quality (fast or highQuality)")
	cmd.Flags().IntVar(&s.Quantity, "quantity", 0, "Number of images per prompt")
	cmd.Flags().IntVar(&s.CheckpointID, "checkpoint-id", 0, "Checkpoint (model) id")
	cmd.Flags().IntVar(&s.FolderID, "folder-id", 0, "Folder id to place results in")
	return cmd
}

func newPresetUseCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "use <name>",
		Short:       "Show a saved preset's settings",
		Example:     "  artistly-pp-cli preset use house-style --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			s, err := loadPreset(args[0])
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), s, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Preset %q: tool=%s style=%s aspect=%s quality=%s quantity=%d\n",
				args[0], s.Tool, s.Style, s.Aspect, s.Quality, s.Quantity)
			return nil
		},
	}
	return cmd
}

func newPresetListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List all saved generation presets with their settings",
		Example:     "  artistly-pp-cli preset list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			m, err := loadPresets()
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), m, flags)
			}
			if len(m) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No presets saved. Create one with: artistly-pp-cli preset save <name> ...")
				return nil
			}
			names := make([]string, 0, len(m))
			for k := range m {
				names = append(names, k)
			}
			sort.Strings(names)
			for _, n := range names {
				fmt.Fprintln(cmd.OutOrStdout(), n)
			}
			return nil
		},
	}
	return cmd
}

func newPresetRemoveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "remove <name>",
		Short:       "Delete a saved preset",
		Example:     "  artistly-pp-cli preset remove house-style",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			m, err := loadPresets()
			if err != nil {
				return err
			}
			if _, ok := m[args[0]]; !ok {
				return notFoundErr(fmt.Errorf("preset %q not found", args[0]))
			}
			delete(m, args[0])
			if err := savePresets(m); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed preset %q.\n", args[0])
			return nil
		},
	}
	return cmd
}
