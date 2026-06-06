// Copyright 2026 David Bryson and contributors. Licensed under Apache-2.0. See LICENSE.
//
// `export` — serialize a saved trip to GPX, GeoJSON, or markdown (hand-authored).
// Reads entirely from the local store; never hits the network.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/atlas-obscura/internal/cliutil"
)

func newNovelExportCmd(flags *rootFlags) *cobra.Command {
	var format string
	var out string

	cmd := &cobra.Command{
		Use:   "export <trip>",
		Short: "Serialize a saved trip to GPX, GeoJSON, or a markdown itinerary, fully offline.",
		Long: "Export a saved trip (see 'trip') to a routable GPX track, importable GeoJSON, or a\n" +
			"human-readable markdown itinerary. Reads only from the local store — no network calls.",
		Example: "  atlas-obscura-pp-cli export california-oddities --format gpx --out trip.gpx\n" +
			"  atlas-obscura-pp-cli export default --format md",
		// An unknown trip is an empty result (exit 0), not an error, so skip the
		// dogfood invalid-arg error-path probe.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would export a trip")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a trip name is required"))
			}
			trip := args[0]
			format = strings.ToLower(strings.TrimSpace(format))
			if format == "" {
				format = "md"
			}
			switch format {
			case "gpx", "geojson", "md", "markdown":
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("unsupported --format %q (use gpx, geojson, or md)", format))
			}

			s, err := aoDB(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			if err := ensureAOTables(s); err != nil {
				return err
			}
			places, err := readTripPlaces(s, trip)
			if err != nil {
				return err
			}
			// An empty trip is an empty result, not an error (exit 0). When the
			// caller asked for a structured document (gpx/geojson/md), emit a
			// valid empty document in *that* format so the output type matches the
			// request; only the agent/JSON envelope path reports the empty-trip
			// note as structured data. renderGPX/GeoJSON/Markdown all degrade
			// cleanly to an empty-but-valid document for an empty place list.
			if len(places) == 0 && flags.asJSON {
				return aoEmit(cmd, flags, map[string]any{
					"source": aoSourceNote,
					"trip":   trip,
					"count":  0,
					"note":   fmt.Sprintf("trip %q is empty; add places with 'trip add <id-or-slug> --trip %s'", trip, trip),
				})
			}

			var content string
			switch format {
			case "gpx":
				content = renderGPX(trip, places)
			case "geojson":
				content = renderGeoJSON(trip, places)
			default:
				content = renderMarkdown(cmd, trip, places)
			}

			if out != "" {
				if cliutil.IsVerifyEnv() {
					fmt.Fprintf(cmd.OutOrStdout(), "would write %d-place %s export to %s\n", len(places), format, out)
					return nil
				}
				if err := os.WriteFile(out, []byte(content), 0o600); err != nil {
					return fmt.Errorf("writing %s: %w", out, err)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "wrote %d places to %s (%s)\n", len(places), out, format)
				return nil
			}
			fmt.Fprint(cmd.OutOrStdout(), content)
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "md", "Export format: gpx, geojson, or md")
	cmd.Flags().StringVar(&out, "out", "", "Write to this file instead of stdout")
	return cmd
}

func renderGPX(trip string, places []AOPlace) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<gpx version="1.1" creator="atlas-obscura-pp-cli" xmlns="http://www.topografix.com/GPX/1/1">` + "\n")
	fmt.Fprintf(&b, "  <metadata><name>%s</name><desc>%s</desc></metadata>\n", xmlEscape(trip), xmlEscape(aoSourceNote))
	for _, p := range places {
		fmt.Fprintf(&b, "  <wpt lat=\"%f\" lon=\"%f\">\n    <name>%s</name>\n    <desc>%s</desc>\n    <link href=\"%s\"/>\n  </wpt>\n",
			p.Lat, p.Lng, xmlEscape(p.Title), xmlEscape(p.Location), xmlEscape(p.URL))
	}
	b.WriteString("</gpx>\n")
	return b.String()
}

func renderGeoJSON(trip string, places []AOPlace) string {
	type feature struct {
		Type       string         `json:"type"`
		Geometry   map[string]any `json:"geometry"`
		Properties map[string]any `json:"properties"`
	}
	feats := make([]feature, 0, len(places))
	for _, p := range places {
		feats = append(feats, feature{
			Type:     "Feature",
			Geometry: map[string]any{"type": "Point", "coordinates": []float64{p.Lng, p.Lat}},
			Properties: map[string]any{
				"id": p.ID, "title": p.Title, "location": p.Location, "url": p.URL,
			},
		})
	}
	fc := map[string]any{
		"type":     "FeatureCollection",
		"trip":     trip,
		"source":   aoSourceNote,
		"features": feats,
	}
	out, _ := json.MarshalIndent(fc, "", "  ")
	return string(out) + "\n"
}

func renderMarkdown(cmd *cobra.Command, trip string, places []AOPlace) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Trip: %s\n\n", trip)
	fmt.Fprintf(&b, "%d places · %s\n\n", len(places), aoSourceNote)
	for i, p := range places {
		fmt.Fprintf(&b, "## %d. %s\n\n", i+1, p.Title)
		if p.Location != "" {
			fmt.Fprintf(&b, "- Location: %s\n", p.Location)
		}
		if p.Lat != 0 || p.Lng != 0 {
			fmt.Fprintf(&b, "- Coordinates: %f, %f\n", p.Lat, p.Lng)
		}
		if p.URL != "" {
			fmt.Fprintf(&b, "- Link: %s\n", p.URL)
		}
		if p.Description != "" {
			fmt.Fprintf(&b, "\n%s\n", p.Description)
		}
		if p.KnowBeforeYouGo != "" {
			fmt.Fprintf(&b, "\n**Know Before You Go:** %s\n", p.KnowBeforeYouGo)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;")
	return r.Replace(s)
}
