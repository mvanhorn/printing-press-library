// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
//
// Novel agent feature: render the board's current layout as readable text.
// The Cloud API returns the current message as a 2D array of integer character
// codes (often JSON-encoded as a string), which is unreadable on its own.
// `message preview` calls the real `GET /` endpoint and decodes that grid into
// a bordered block of glyphs an agent or person can actually read. Read-only.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// currentMessageResponse models the GET / payload. Layout is captured as raw
// JSON because the API encodes it as a *string* containing a 2D array on some
// boards and as a bare 2D array on others; decodeLayout handles both.
type currentMessageResponse struct {
	CurrentMessage struct {
		Layout json.RawMessage `json:"layout"`
		ID     string          `json:"id"`
	} `json:"currentMessage"`
}

// decodeLayout normalizes the polymorphic layout field into a 2D int grid.
func decodeLayout(raw json.RawMessage) ([][]int, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("response had no currentMessage.layout")
	}
	var grid [][]int
	if err := json.Unmarshal(raw, &grid); err == nil {
		return validateGrid(grid)
	}
	// Fall back to the string-encoded form: "[[0,0,...],...]".
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("layout is neither an array nor a string: %w", err)
	}
	if err := json.Unmarshal([]byte(s), &grid); err != nil {
		return nil, fmt.Errorf("layout string did not contain a 2D array: %w", err)
	}
	return validateGrid(grid)
}

// validateGrid rejects a nil/empty layout. A JSON `null` unmarshals into a nil
// slice without error, which would otherwise render as a blank board and hide
// API drift or a malformed response.
func validateGrid(grid [][]int) ([][]int, error) {
	if len(grid) == 0 {
		return nil, fmt.Errorf("layout was empty or null; expected a 2D character-code grid")
	}
	return grid, nil
}

// renderGrid draws the code grid as a bordered block of glyphs. Blanks render
// as spaces, which the border makes visible; color chips render as a lowercase
// initial (see the `characters` command legend).
func renderGrid(grid [][]int) string {
	width := 0
	for _, row := range grid {
		if len(row) > width {
			width = len(row)
		}
	}
	var b strings.Builder
	top := "┌" + strings.Repeat("─", width) + "┐"
	b.WriteString(top + "\n")
	for _, row := range grid {
		b.WriteString("│")
		for _, code := range row {
			b.WriteString(glyphForCode(code))
		}
		// Pad ragged rows so the right border stays aligned.
		if len(row) < width {
			b.WriteString(strings.Repeat(" ", width-len(row)))
		}
		b.WriteString("│\n")
	}
	b.WriteString("└" + strings.Repeat("─", width) + "┘")
	return b.String()
}

// usesColorChips reports whether any cell is a color-chip code (63-71), so the
// preview only prints the color legend when it is relevant.
func usesColorChips(grid [][]int) bool {
	for _, row := range grid {
		for _, code := range row {
			if code >= 63 && code <= 71 {
				return true
			}
		}
	}
	return false
}

func newNovelMessagePreviewCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "preview",
		Short: "Render the board's current message as readable text",
		Long: "Read the current board layout (GET /) and render its 2D character-code grid as a bordered\n" +
			"block of glyphs. Color chips render as a lowercase initial (r/o/y/g/b/v/w/k) or filled\n" +
			"cell (█); run 'vestaboard-pp-cli characters' for the full code table. Use --json for\n" +
			"the raw rows plus the rendered text.",
		Example:     "  vestaboard-pp-cli message preview\n  vestaboard-pp-cli message preview --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get(cmd.Context(), "/", map[string]string{})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var resp currentMessageResponse
			if err := json.Unmarshal(data, &resp); err != nil {
				return fmt.Errorf("parsing current message: %w", err)
			}
			grid, err := decodeLayout(resp.CurrentMessage.Layout)
			if err != nil {
				return err
			}
			rendered := renderGrid(grid)

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintln(cmd.OutOrStdout(), rendered)
				if resp.CurrentMessage.ID != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "id: %s\n", resp.CurrentMessage.ID)
				}
				if usesColorChips(grid) {
					fmt.Fprintln(cmd.OutOrStdout(), "legend: r=Red o=Orange y=Yellow g=Green b=Blue v=Violet w=White k=Black █=Filled")
				}
				return nil
			}

			out, err := json.Marshal(map[string]any{
				"id":      resp.CurrentMessage.ID,
				"rows":    grid,
				"preview": rendered,
			})
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}
