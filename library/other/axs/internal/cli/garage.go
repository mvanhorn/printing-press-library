// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for the SRAM AXS CLI. Not generated.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type garageComponent struct {
	Serial string `json:"serial"`
	Name   string `json:"name"`
	Model  string `json:"model"`
}

type garageBike struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Model      string            `json:"model"`
	Components []garageComponent `json:"components"`
}

type garageView struct {
	Bikes []garageBike `json:"bikes"`
}

func newNovelGarageCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "garage",
		Short:       "A tree of every bike with its components and serials in one shot.",
		Long:        "Joins your bikes and components locally into a single tree — each bike with its components and serials — that no single API call returns. The one-command overview of a rider's whole AXS setup.",
		Example:     "  axs-pp-cli garage",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			bikes, err := fetchList(ctx, c, "/bikes/", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			components, err := fetchList(ctx, c, "/components/", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			toComp := func(comp map[string]any) garageComponent {
				return garageComponent{
					Serial: gstr(comp, "serial", "serial_number"),
					Name:   gstr(comp, "name", "part_description", "mobile_display_name_key", "description", "model_code"),
					Model:  gstr(comp, "model", "model_id"),
				}
			}
			view := garageView{Bikes: []garageBike{}}
			byBike := map[string][]garageComponent{}
			for _, comp := range components {
				bikeRef := gstr(comp, "bike", "bike_id", "bikeId")
				byBike[bikeRef] = append(byBike[bikeRef], toComp(comp))
			}
			seen := map[string]bool{}
			for _, b := range bikes {
				id := gstr(b, "id")
				seen[id] = true
				gb := garageBike{
					ID:         id,
					Name:       gstr(b, "nickname", "name"),
					Model:      gstr(b, "model"),
					Components: byBike[id],
				}
				if gb.Components == nil {
					gb.Components = []garageComponent{}
				}
				view.Bikes = append(view.Bikes, gb)
			}
			// Components not attached to any known bike.
			var orphans []garageComponent
			for ref, comps := range byBike {
				if ref == "" || !seen[ref] {
					orphans = append(orphans, comps...)
				}
			}
			if len(orphans) > 0 {
				view.Bikes = append(view.Bikes, garageBike{
					Name:       "Unassigned",
					Components: orphans,
				})
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			if len(view.Bikes) == 0 {
				fmt.Fprintln(out, "no bikes found")
				return nil
			}
			for _, b := range view.Bikes {
				label := b.Name
				if b.Model != "" {
					label = fmt.Sprintf("%s (%s)", b.Name, b.Model)
				}
				fmt.Fprintf(out, "%s\n", label)
				for _, comp := range b.Components {
					fmt.Fprintf(out, "  • %s  serial=%s\n", comp.Name, comp.Serial)
				}
			}
			return nil
		},
	}
	return cmd
}
