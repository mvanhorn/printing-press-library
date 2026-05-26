// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newBriefCmd(flags *rootFlags) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "brief <event-id>",
		Short: "Agent-ready briefing for an earthquake: magnitude, place, PAGER, DYFI, tsunami, MMI, product inventory",
		Long: `Render a one-event newsroom-style briefing for a USGS earthquake.

Composes the event's GeoJSON properties (magnitude, place, time, PAGER alert,
DYFI felt count, ShakeMap MMI, tsunami status) with its product inventory
(ShakeMap, PAGER, DYFI, focal mechanisms, moment tensors) into a structured
block suitable for Slack, agent context, or copy.

Looks up from the local store first; falls back to live FDSN.`,
		Example: strings.Trim(`
  # Markdown briefing for a specific event
  usgs-earthquakes-pp-cli brief us7000abcd --format markdown

  # JSON for agent context
  usgs-earthquakes-pp-cli brief us7000abcd --format json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			eventID := args[0]

			var feat map[string]any
			db, err := openLocalStore(ctx)
			if err == nil {
				defer db.Close()
				feat, _ = localEventByID(ctx, db, eventID)
			}
			if feat == nil {
				c, cerr := flags.newClient()
				if cerr != nil {
					return cerr
				}
				data, gerr := c.Get("/query", map[string]string{
					"eventid": eventID,
					"format":  "geojson",
				})
				if gerr != nil {
					return classifyAPIError(gerr, flags)
				}
				if err := json.Unmarshal(data, &feat); err != nil {
					return fmt.Errorf("parse event: %w", err)
				}
			}

			b := buildBrief(eventID, feat)

			switch format {
			case "json":
				return printJSONFiltered(cmd.OutOrStdout(), b, flags)
			case "text":
				fmt.Fprintln(cmd.OutOrStdout(), b.RenderText())
			case "markdown", "md":
				fmt.Fprintln(cmd.OutOrStdout(), b.RenderMarkdown())
			default:
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), b, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), b.RenderMarkdown())
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "markdown", "Output format: markdown, text, json")
	return cmd
}

type briefing struct {
	ID         string          `json:"id"`
	Title      string          `json:"title"`
	Mag        float64         `json:"mag"`
	MagType    string          `json:"mag_type"`
	Place      string          `json:"place"`
	Time       string          `json:"time"`
	TimeAgo    string          `json:"time_ago"`
	DepthKm    float64         `json:"depth_km"`
	Alert      string          `json:"alert"`
	MMI        float64         `json:"mmi"`
	CDI        float64         `json:"cdi"`
	Felt       int             `json:"felt"`
	Tsunami    int             `json:"tsunami"`
	Sig        int             `json:"significance"`
	Status     string          `json:"status"`
	URL        string          `json:"url"`
	Products   []string        `json:"products"`
	HasProduct map[string]bool `json:"has_product"`
}

func buildBrief(id string, feat map[string]any) *briefing {
	props, _ := feat["properties"].(map[string]any)
	if props == nil {
		props = map[string]any{}
	}
	geom, _ := feat["geometry"].(map[string]any)
	var depth float64
	if geom != nil {
		if coords, ok := geom["coordinates"].([]any); ok && len(coords) >= 3 {
			depth, _ = coords[2].(float64)
		}
	}
	mag, _ := props["mag"].(float64)
	magType, _ := props["magType"].(string)
	place, _ := props["place"].(string)
	tMs, _ := props["time"].(float64)
	title, _ := props["title"].(string)
	alert, _ := props["alert"].(string)
	mmi, _ := props["mmi"].(float64)
	cdi, _ := props["cdi"].(float64)
	feltF, _ := props["felt"].(float64)
	tsunamiF, _ := props["tsunami"].(float64)
	sigF, _ := props["sig"].(float64)
	status, _ := props["status"].(string)
	url, _ := props["url"].(string)

	eventTime := time.Unix(int64(tMs)/1000, 0).UTC()
	timeAgo := humanizeAgo(time.Since(eventTime))

	// Product inventory: extract from properties.products map keys.
	prodMap := map[string]bool{}
	if products, ok := props["products"].(map[string]any); ok {
		for k := range products {
			prodMap[k] = true
		}
	}
	// Fallback to types CSV (",dyfi,...").
	if len(prodMap) == 0 {
		if typesCSV, ok := props["types"].(string); ok {
			for _, t := range strings.Split(typesCSV, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					prodMap[t] = true
				}
			}
		}
	}
	var products []string
	for k := range prodMap {
		products = append(products, k)
	}
	sort.Strings(products)

	if title == "" {
		title = fmt.Sprintf("M %.1f - %s", mag, place)
	}
	return &briefing{
		ID:         id,
		Title:      title,
		Mag:        mag,
		MagType:    magType,
		Place:      place,
		Time:       eventTime.Format(time.RFC3339),
		TimeAgo:    timeAgo,
		DepthKm:    depth,
		Alert:      alert,
		MMI:        mmi,
		CDI:        cdi,
		Felt:       int(feltF),
		Tsunami:    int(tsunamiF),
		Sig:        int(sigF),
		Status:     status,
		URL:        url,
		Products:   products,
		HasProduct: prodMap,
	}
}

func humanizeAgo(d time.Duration) string {
	if d < 0 {
		return "in the future"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

func (b *briefing) RenderMarkdown() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s\n\n", b.Title)
	fmt.Fprintf(&sb, "**Event ID:** `%s`  \n", b.ID)
	fmt.Fprintf(&sb, "**Time:** %s (%s)  \n", b.Time, b.TimeAgo)
	fmt.Fprintf(&sb, "**Magnitude:** M%.1f %s  \n", b.Mag, b.MagType)
	fmt.Fprintf(&sb, "**Depth:** %.1f km  \n", b.DepthKm)
	if b.Alert != "" {
		fmt.Fprintf(&sb, "**PAGER alert:** %s  \n", strings.ToUpper(b.Alert))
	}
	if b.MMI > 0 {
		fmt.Fprintf(&sb, "**ShakeMap max MMI:** %.1f  \n", b.MMI)
	}
	if b.CDI > 0 {
		fmt.Fprintf(&sb, "**DYFI CDI:** %.1f (%d felt reports)  \n", b.CDI, b.Felt)
	} else if b.Felt > 0 {
		fmt.Fprintf(&sb, "**Felt reports:** %d  \n", b.Felt)
	}
	if b.Tsunami != 0 {
		fmt.Fprintf(&sb, "**Tsunami flag:** triggered  \n")
	}
	fmt.Fprintf(&sb, "**Significance:** %d / 1000  \n", b.Sig)
	fmt.Fprintf(&sb, "**Status:** %s  \n", b.Status)
	if b.URL != "" {
		fmt.Fprintf(&sb, "**Event page:** %s  \n", b.URL)
	}
	if len(b.Products) > 0 {
		fmt.Fprintf(&sb, "\n**Products available:** %s\n", strings.Join(b.Products, ", "))
	}
	return sb.String()
}

func (b *briefing) RenderText() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s\n", b.Title)
	fmt.Fprintf(&sb, "Event ID: %s\n", b.ID)
	fmt.Fprintf(&sb, "Time: %s (%s)\n", b.Time, b.TimeAgo)
	fmt.Fprintf(&sb, "Magnitude: M%.1f %s\n", b.Mag, b.MagType)
	fmt.Fprintf(&sb, "Depth: %.1f km\n", b.DepthKm)
	if b.Alert != "" {
		fmt.Fprintf(&sb, "PAGER alert: %s\n", strings.ToUpper(b.Alert))
	}
	if b.MMI > 0 {
		fmt.Fprintf(&sb, "ShakeMap max MMI: %.1f\n", b.MMI)
	}
	if b.Felt > 0 {
		fmt.Fprintf(&sb, "Felt reports: %d (CDI %.1f)\n", b.Felt, b.CDI)
	}
	if b.Tsunami != 0 {
		fmt.Fprintf(&sb, "Tsunami: triggered\n")
	}
	fmt.Fprintf(&sb, "Significance: %d / 1000\n", b.Sig)
	if b.URL != "" {
		fmt.Fprintf(&sb, "Event page: %s\n", b.URL)
	}
	if len(b.Products) > 0 {
		fmt.Fprintf(&sb, "Products: %s\n", strings.Join(b.Products, ", "))
	}
	return sb.String()
}
