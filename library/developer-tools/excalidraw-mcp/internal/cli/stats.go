// Copyright 2026 bk20260126-code. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type canvasStats struct {
	ElementCount int            `json:"elementCount"`
	Types        map[string]int `json:"types"`
	Colors       struct {
		Strokes     []string `json:"strokes"`
		Backgrounds []string `json:"backgrounds"`
	} `json:"colors"`
	BoundingBox struct {
		MinX   float64 `json:"minX"`
		MinY   float64 `json:"minY"`
		MaxX   float64 `json:"maxX"`
		MaxY   float64 `json:"maxY"`
		Width  float64 `json:"width"`
		Height float64 `json:"height"`
	} `json:"boundingBox"`
	Groups int `json:"groups"`
	Locked int `json:"locked"`
}

func newStatsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show element type distribution, color palette, and bounding box for the canvas.",
		Example: strings.Trim(`
  excalidraw-mcp-pp-cli stats
  excalidraw-mcp-pp-cli stats --json --agent
  excalidraw-mcp-pp-cli stats --json --select elementCount,types`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			data, err := c.Get("/api/elements", nil)
			if err != nil {
				return fmt.Errorf("fetching elements: %w", err)
			}

			var resp struct {
				Elements []map[string]any `json:"elements"`
			}
			if parseErr := json.Unmarshal(data, &resp); parseErr != nil {
				// Try as raw array
				var arr []map[string]any
				if arrErr := json.Unmarshal(data, &arr); arrErr == nil {
					resp.Elements = arr
				}
			}

			stats := buildCanvasStats(resp.Elements)

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				raw := json.RawMessage(jsonMarshalAny(stats))
				if flags.selectFields != "" {
					raw = filterFields(raw, flags.selectFields)
				}
				return printOutput(cmd.OutOrStdout(), raw, true)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Canvas Stats (%d elements)\n", stats.ElementCount)
			fmt.Fprintln(cmd.OutOrStdout(), "")
			fmt.Fprintln(cmd.OutOrStdout(), "Types:")
			for t, count := range stats.Types {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-15s %d\n", t, count)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "")
			fmt.Fprintf(cmd.OutOrStdout(), "Bounding Box: %.0fx%.0f (x: %.0f to %.0f, y: %.0f to %.0f)\n",
				stats.BoundingBox.Width, stats.BoundingBox.Height,
				stats.BoundingBox.MinX, stats.BoundingBox.MaxX,
				stats.BoundingBox.MinY, stats.BoundingBox.MaxY)
			if len(stats.Colors.Strokes) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Stroke colors: %s\n", strings.Join(stats.Colors.Strokes, ", "))
			}
			if stats.Groups > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Groups: %d | Locked: %d\n", stats.Groups, stats.Locked)
			}
			return nil
		},
	}
	return cmd
}

func buildCanvasStats(elements []map[string]any) canvasStats {
	var stats canvasStats
	stats.ElementCount = len(elements)
	stats.Types = make(map[string]int)

	strokeSet := make(map[string]bool)
	bgSet := make(map[string]bool)
	groupSet := make(map[string]bool)

	var minX, minY, maxX, maxY float64
	first := true

	for _, el := range elements {
		// Type count
		if t, ok := el["type"].(string); ok {
			stats.Types[t]++
		}

		// Colors
		if sc, ok := el["strokeColor"].(string); ok && sc != "" && sc != "transparent" {
			strokeSet[sc] = true
		}
		if bc, ok := el["backgroundColor"].(string); ok && bc != "" && bc != "transparent" {
			bgSet[bc] = true
		}

		// Groups
		if gids, ok := el["groupIds"].([]any); ok {
			for _, g := range gids {
				if gs, ok := g.(string); ok && gs != "" {
					groupSet[gs] = true
				}
			}
		}

		// Locked
		if locked, ok := el["locked"].(bool); ok && locked {
			stats.Locked++
		}

		// Bounding box
		x, _ := el["x"].(float64)
		y, _ := el["y"].(float64)
		w, _ := el["width"].(float64)
		h, _ := el["height"].(float64)
		if first {
			minX, minY, maxX, maxY = x, y, x+w, y+h
			first = false
		} else {
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x+w > maxX {
				maxX = x + w
			}
			if y+h > maxY {
				maxY = y + h
			}
		}
	}

	stats.BoundingBox.MinX = minX
	stats.BoundingBox.MinY = minY
	stats.BoundingBox.MaxX = maxX
	stats.BoundingBox.MaxY = maxY
	stats.BoundingBox.Width = maxX - minX
	stats.BoundingBox.Height = maxY - minY
	stats.Groups = len(groupSet)

	for c := range strokeSet {
		stats.Colors.Strokes = append(stats.Colors.Strokes, c)
	}
	for c := range bgSet {
		stats.Colors.Backgrounds = append(stats.Colors.Backgrounds, c)
	}

	return stats
}
