package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var presetCmd = &cobra.Command{
	Use:   "preset",
	Short: "Save and load capture configuration presets",
	Long:  `Manage reusable capture configurations for common workflows.`,
}

var presetSaveCmd = &cobra.Command{
	Use:   "save <name>",
	Short: "Save a capture preset",
	Example: `  agent-capture preset save pr-evidence --duration 8 --fps 12 --width 640 --max-size 5mb
  agent-capture preset save quick-screenshot --format png --retina`,
	Args: cobra.ExactArgs(1),
	RunE: runPresetSave,
}

var presetListCmd = &cobra.Command{
	Use:   "list",
	Short: "List saved presets",
	RunE:  runPresetList,
}

var presetDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a saved preset",
	Args:  cobra.ExactArgs(1),
	RunE:  runPresetDelete,
}

var presetShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show details of a preset",
	Args:  cobra.ExactArgs(1),
	RunE:  runPresetShow,
}

// Preset flags
var (
	psDuration int
	psFPS      int
	psWidth    int
	psMaxSize  string
	psFormat   string
	psRetina   bool
	psCursor   bool
)

func init() {
	presetCmd.AddCommand(presetSaveCmd)
	presetCmd.AddCommand(presetListCmd)
	presetCmd.AddCommand(presetDeleteCmd)
	presetCmd.AddCommand(presetShowCmd)

	presetSaveCmd.Flags().IntVar(&psDuration, "duration", 0, "Recording duration")
	presetSaveCmd.Flags().IntVar(&psFPS, "fps", 0, "Frame rate")
	presetSaveCmd.Flags().IntVar(&psWidth, "width", 0, "Output width")
	presetSaveCmd.Flags().StringVar(&psMaxSize, "max-size", "", "Max file size")
	presetSaveCmd.Flags().StringVar(&psFormat, "format", "", "Output format")
	presetSaveCmd.Flags().BoolVar(&psRetina, "retina", false, "Retina mode")
	presetSaveCmd.Flags().BoolVar(&psCursor, "cursor", false, "Show cursor")
}

type preset struct {
	Name     string `json:"name"`
	Duration int    `json:"duration,omitempty"`
	FPS      int    `json:"fps,omitempty"`
	Width    int    `json:"width,omitempty"`
	MaxSize  string `json:"max_size,omitempty"`
	Format   string `json:"format,omitempty"`
	Retina   bool   `json:"retina,omitempty"`
	Cursor   bool   `json:"cursor,omitempty"`
}

func presetDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "agent-capture", "presets")
}

func presetPath(name string) string {
	return filepath.Join(presetDir(), name+".json")
}

func runPresetSave(cmd *cobra.Command, args []string) error {
	name := args[0]
	dir := presetDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating preset dir: %w", err)
	}

	p := preset{
		Name:     name,
		Duration: psDuration,
		FPS:      psFPS,
		Width:    psWidth,
		MaxSize:  psMaxSize,
		Format:   psFormat,
		Retina:   psRetina,
		Cursor:   psCursor,
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling preset: %w", err)
	}

	if err := os.WriteFile(presetPath(name), data, 0644); err != nil {
		return fmt.Errorf("writing preset: %w", err)
	}

	if jsonOutput {
		return printJSON(p)
	}
	infof("Preset saved: %s", name)
	return nil
}

func runPresetList(cmd *cobra.Command, args []string) error {
	entries, err := os.ReadDir(presetDir())
	if err != nil {
		if os.IsNotExist(err) {
			if jsonOutput {
				return printJSON([]preset{})
			}
			fmt.Println("No presets saved.")
			return nil
		}
		return err
	}

	var presets []preset
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(presetDir(), e.Name()))
		if err != nil {
			continue
		}
		var p preset
		if json.Unmarshal(data, &p) == nil {
			presets = append(presets, p)
		}
	}

	if jsonOutput {
		return printJSON(presets)
	}

	if len(presets) == 0 {
		fmt.Println("No presets saved.")
		return nil
	}

	headers := []string{"NAME", "DURATION", "FPS", "WIDTH", "MAX SIZE", "FORMAT"}
	var rows [][]string
	for _, p := range presets {
		rows = append(rows, []string{
			p.Name,
			intOrEmpty(p.Duration),
			intOrEmpty(p.FPS),
			intOrEmpty(p.Width),
			p.MaxSize,
			p.Format,
		})
	}
	printTable(headers, rows)
	return nil
}

func runPresetDelete(cmd *cobra.Command, args []string) error {
	name := args[0]
	path := presetPath(name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return errorf("preset %q not found", name)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("deleting preset: %w", err)
	}
	if jsonOutput {
		return printJSON(map[string]string{"deleted": name})
	}
	infof("Preset deleted: %s", name)
	return nil
}

func runPresetShow(cmd *cobra.Command, args []string) error {
	name := args[0]
	data, err := os.ReadFile(presetPath(name))
	if err != nil {
		if os.IsNotExist(err) {
			return errorf("preset %q not found", name)
		}
		return err
	}
	var p preset
	if err := json.Unmarshal(data, &p); err != nil {
		return fmt.Errorf("parsing preset: %w", err)
	}
	if jsonOutput {
		return printJSON(p)
	}
	fmt.Printf("Name:     %s\n", p.Name)
	fmt.Printf("Duration: %s\n", intOrEmpty(p.Duration))
	fmt.Printf("FPS:      %s\n", intOrEmpty(p.FPS))
	fmt.Printf("Width:    %s\n", intOrEmpty(p.Width))
	fmt.Printf("Max Size: %s\n", p.MaxSize)
	fmt.Printf("Format:   %s\n", p.Format)
	return nil
}

func intOrEmpty(n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%d", n)
}
