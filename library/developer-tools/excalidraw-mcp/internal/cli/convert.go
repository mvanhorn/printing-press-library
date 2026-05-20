// Copyright 2026 bk20260126-code. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newConvertCmd(flags *rootFlags) *cobra.Command {
	var inputFile string
	var outputFile string
	var format string
	var background bool
	var snapshotName string

	cmd := &cobra.Command{
		Use:   "convert [--input <file.mmd>] [--output <file.png>]",
		Short: "Convert a Mermaid diagram to PNG or SVG in one step.",
		Long: `Convert a Mermaid .mmd file to a PNG or SVG image.

Calls three canvas server operations in sequence:
  1. elements from-mermaid (Mermaid to canvas elements)
  2. snapshots create     (optional: save state)
  3. export image         (render PNG or SVG)

The canvas server must be running at http://127.0.0.1:3000.
The Excalidraw browser tab must be open for PNG/SVG export to work.`,
		Example: strings.Trim(`
  excalidraw-mcp-pp-cli convert --input flow.mmd --output diagram.png
  excalidraw-mcp-pp-cli convert --input arch.mmd --output arch.svg --format svg
  excalidraw-mcp-pp-cli convert --input flow.mmd --output flow.png --snapshot v1`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputFile == "" {
				return fmt.Errorf("--input is required (path to .mmd file)")
			}
			if outputFile == "" {
				return fmt.Errorf("--output is required (path for PNG/SVG output)")
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would convert: %s to %s (format: %s)\n", inputFile, outputFile, format)
				return nil
			}

			cleanInput := filepath.Clean(inputFile)
			mmdBytes, err := os.ReadFile(cleanInput)
			if err != nil {
				return fmt.Errorf("reading %s: %w", cleanInput, err)
			}
			mmdContent := strings.TrimSpace(string(mmdBytes))

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Step 1: Mermaid to canvas
			fmt.Fprintln(cmd.ErrOrStderr(), "Step 1/3: Converting Mermaid to canvas elements...")
			_, _, convErr := c.Post("/api/elements/from-mermaid", map[string]any{
				"mermaidDiagram": mmdContent,
			})
			if convErr != nil {
				return fmt.Errorf("mermaid conversion failed: %w", convErr)
			}

			// Step 2: Optional snapshot
			if snapshotName != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "Step 2/3: Saving snapshot %q...\n", snapshotName)
				_, _, snapErr := c.Post("/api/snapshots", map[string]any{
					"name": snapshotName,
				})
				if snapErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: snapshot save failed: %v\n", snapErr)
				}
			} else {
				fmt.Fprintln(cmd.ErrOrStderr(), "Step 2/3: No snapshot (pass --snapshot <name> to save state).")
			}

			// Step 3: Export
			time.Sleep(500 * time.Millisecond)
			fmt.Fprintf(cmd.ErrOrStderr(), "Step 3/3: Exporting as %s...\n", format)
			exportData, _, exportErr := c.Post("/api/export/image", map[string]any{
				"format":     format,
				"background": background,
			})
			if exportErr != nil {
				return fmt.Errorf("export failed (is the Excalidraw browser tab open at http://127.0.0.1:3000 ?): %w", exportErr)
			}

			var exportResp struct {
				Success bool   `json:"success"`
				Data    string `json:"data"`
				Format  string `json:"format"`
			}
			if parseErr := json.Unmarshal(exportData, &exportResp); parseErr != nil || exportResp.Data == "" {
				return fmt.Errorf("unexpected export response: %s", string(exportData))
			}

			rawData := exportResp.Data
			if idx := strings.Index(rawData, ","); idx >= 0 {
				rawData = rawData[idx+1:]
			}
			imgBytes, err := base64.StdEncoding.DecodeString(rawData)
			if err != nil {
				return fmt.Errorf("decoding export: %w", err)
			}

			cleanOutput := filepath.Clean(outputFile)
			if err := os.WriteFile(cleanOutput, imgBytes, 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", cleanOutput, err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Converted: %s to %s (%d bytes)\n", inputFile, cleanOutput, len(imgBytes))
			return nil
		},
	}

	cmd.Flags().StringVarP(&inputFile, "input", "i", "", "Path to Mermaid .mmd file (required)")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file path for PNG or SVG (required)")
	cmd.Flags().StringVar(&format, "format", "png", "Output format: png or svg")
	cmd.Flags().BoolVar(&background, "background", true, "Include white background")
	cmd.Flags().StringVar(&snapshotName, "snapshot", "", "Save a canvas snapshot with this name after conversion")
	return cmd
}
