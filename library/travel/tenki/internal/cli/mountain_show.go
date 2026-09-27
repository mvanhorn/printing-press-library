package cli

// pp:data-source auto

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/tenki/internal/tenki"
	"github.com/spf13/cobra"
	"math"
	"strings"
)

func newMountainShowCmd(flags *rootFlags, factory tenkiFactory) *cobra.Command {
	var place string
	var level float64
	var limit int
	var read tenkiReadFlags
	cmd := &cobra.Command{Use: "show", Short: "Show foothill location and nearby numerical guidance at source altitudes", Long: "Mountain main weather refers to the named foothill municipality. Altitude values are nearby numerical model guidance with a separate initialization time. --level selects an exact source altitude in metres, without interpolation or a summit forecast.", Example: strings.Trim(`
  tenki-pp-cli mountain show --place https://tenki.jp/mountain/famous100/5/25/150.html --limit 8
  tenki-pp-cli mountain show --place https://tenki.jp/mountain/famous100/5/25/150.html --level 3000 --agent
`, "\n"), Annotations: tenkiAnnotations("--place=" + tenkiFuji + ";--limit=4")}
	cmd.Flags().StringVar(&place, "place", "", "Canonical tenki.jp mountain URL (required)")
	cmd.Flags().Float64Var(&level, "level", 0, "Exact source altitude in metres (optional)")
	cmd.Flags().IntVar(&limit, "limit", 24, "Maximum model values (1–72)")
	read.attach(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if done, err := tenkiPrelude(cmd, args, flags); done {
			return err
		}
		if err := validateTenkiPlace(place); err != nil {
			return err
		}
		if !strings.Contains(place, "/mountain/") {
			return usageErr(fmt.Errorf("--place must be a canonical tenki.jp mountain URL"))
		}
		if err := tenkiBound("limit", limit, 72); err != nil {
			return err
		}
		if cmd.Flags().Changed("level") && (level < 0 || math.IsNaN(level) || math.IsInf(level, 0)) {
			return usageErr(fmt.Errorf("--level must be a nonnegative source altitude in metres"))
		}
		client, err := read.source(flags, factory)
		if err != nil {
			return err
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		result, err := client.Mountain(ctx, place)
		if err != nil {
			return tenkiProductError(err)
		}
		levels := []tenki.ModelLevel{}
		for _, item := range result.Levels {
			if !cmd.Flags().Changed("level") || item.ElevationM == level {
				levels = append(levels, item)
			}
		}
		truncated := len(levels) > limit
		if truncated {
			levels = levels[:limit]
		}
		result.Levels = levels
		if len(levels) == 0 && result.Status == "ok" {
			result.Status = "level_unavailable"
		}
		tenkiWarnings(cmd, result.Warnings)
		return emitTenki(cmd, flags, client, map[string]any{"mountain": result, "requested_level_m": func() any {
			if cmd.Flags().Changed("level") {
				return level
			}
			return nil
		}(), "truncated": truncated}, result.Source, result.ModelSource)
	}
	return cmd
}
