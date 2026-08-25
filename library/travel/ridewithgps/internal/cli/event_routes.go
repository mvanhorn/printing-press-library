// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source live
//
// `event-routes` lists the routes attached to an organized ride (event) and can
// export them straight to bike-computer files. The v1 event response does not
// surface routes, so this reads the legacy event endpoint, which embeds them.
// Hand-authored transcendence command.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/ridewithgps/internal/cliutil"
	"github.com/spf13/cobra"
)

type eventRoute struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type eventRoutesView struct {
	EventID  string         `json:"event_id"`
	Routes   []eventRoute   `json:"routes"`
	Exported []exportedFile `json:"exported,omitempty"`
	Note     string         `json:"note,omitempty"`
}

func newNovelEventRoutesCmd(flags *rootFlags) *cobra.Command {
	var export string
	var out string
	var native bool

	cmd := &cobra.Command{
		Use:   "event-routes <event-id>",
		Short: "List the routes attached to an event (group ride, fondo, brevet), and optionally export them for your bike computer.",
		Long: `Resolve the routes attached to an organized ride and, with --export, write them
straight to files for your head unit.

The v1 event response does not include routes, so this reads the legacy event
endpoint. For your own routes use 'routes list' / 'export'.`,
		Example: strings.Trim(`
  ridewithgps-pp-cli event-routes 12345678
  ridewithgps-pp-cli event-routes 12345678 --export gpx --out ./fondo
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list (and optionally export) an event's routes")
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an <event-id> is required"))
			}
			eventID := strings.TrimSpace(args[0])

			format := strings.ToLower(strings.TrimSpace(export))
			if format != "" {
				if native {
					if !nativeFormats[format] {
						return usageErr(fmt.Errorf("--native supports gpx, tcx, fit, kml (got %q)", format))
					}
				} else if !synthFormats[format] {
					return usageErr(fmt.Errorf("--export %q requires --native (synthesized: gpx, tcx, csv)", format))
				}
			}
			if out == "" {
				out = "."
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			raw, err := c.Get(ctx, fmt.Sprintf("/events/%s.json", eventID), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			routes := extractEventRoutes(raw)
			view := eventRoutesView{EventID: eventID, Routes: routes}
			if len(routes) == 0 {
				view.Note = "no routes attached to this event (or the event is not visible to your account)"
			}

			if format != "" && len(routes) > 0 {
				if cliutil.IsVerifyEnv() {
					view.Note = "verify mode: routes listed, no files written"
				} else {
					if err := os.MkdirAll(out, 0o750); err != nil {
						return fmt.Errorf("creating output dir %q: %w", out, err)
					}
					targets := routes
					if cliutil.IsDogfoodEnv() && len(targets) > 1 {
						targets = targets[:1]
					}
					view.Exported = make([]exportedFile, 0, len(targets))
					for _, r := range targets {
						view.Exported = append(view.Exported, exportAsset(ctx, c, "routes", r.ID, format, native, out))
					}
				}
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Event %s — %d route(s)\n", eventID, len(routes))
			for _, r := range routes {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\t%s\n", r.ID, r.Name)
			}
			for _, ef := range view.Exported {
				if ef.Error != "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "  export %s FAILED: %s\n", ef.ID, ef.Error)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  exported %s -> %s\n", ef.ID, ef.File)
			}
			if view.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&export, "export", "", "Export each route in this format: gpx, tcx, csv (or fit, kml with --native)")
	cmd.Flags().StringVar(&out, "out", ".", "Output directory for --export")
	cmd.Flags().BoolVar(&native, "native", false, "Stream Ride with GPS's own file render (enables fit/kml)")
	return cmd
}

// extractEventRoutes pulls the routes array from a legacy event response,
// tolerant of top-level, {"event":{...}}, and {"eventDetails":{...}} envelopes.
func extractEventRoutes(raw json.RawMessage) []eventRoute {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil
	}
	candidates := []json.RawMessage{}
	if r, ok := top["routes"]; ok {
		candidates = append(candidates, r)
	}
	for _, wrap := range []string{"event", "eventDetails"} {
		if inner, ok := top[wrap]; ok {
			var m map[string]json.RawMessage
			if json.Unmarshal(inner, &m) == nil {
				if r, ok := m["routes"]; ok {
					candidates = append(candidates, r)
				}
			}
		}
	}
	for _, c := range candidates {
		var arr []struct {
			ID   json.Number `json:"id"`
			Name string      `json:"name"`
		}
		if err := json.Unmarshal(c, &arr); err == nil && len(arr) > 0 {
			out := make([]eventRoute, 0, len(arr))
			for _, a := range arr {
				out = append(out, eventRoute{ID: a.ID.String(), Name: a.Name})
			}
			return out
		}
	}
	return nil
}
