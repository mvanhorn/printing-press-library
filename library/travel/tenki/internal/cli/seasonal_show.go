package cli

// pp:data-source auto

import (
	"fmt"
	"github.com/spf13/cobra"
	"strings"
	"time"
)

func newSeasonalShowCmd(flags *rootFlags, factory tenkiFactory) *cobra.Command {
	var kind, place string
	var year int
	var read tenkiReadFlags
	cmd := &cobra.Command{Use: "show", Short: "Show a seasonal report, prediction and normal period without mixing their times", Example: strings.Trim(`
  tenki-pp-cli seasonal show --kind kouyou --place https://tenki.jp/kouyou/3/13/30139.html
  tenki-pp-cli seasonal show --kind sakura --place https://tenki.jp/sakura/3/16/54401.html --year 2026 --agent
`, "\n"), Annotations: tenkiAnnotations("--kind=kouyou;--place=" + tenkiOze)}
	cmd.Flags().StringVar(&kind, "kind", "", "Seasonal product: sakura or kouyou (required)")
	cmd.Flags().StringVar(&place, "place", "", "Canonical tenki.jp seasonal spot URL (required)")
	cmd.Flags().IntVar(&year, "year", 0, "Requested season year (default: current JST year)")
	read.attach(cmd)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if done, err := tenkiPrelude(cmd, args, flags); done {
			return err
		}
		if year == 0 && !cmd.Flags().Changed("year") {
			year = time.Now().In(time.FixedZone("JST", 9*60*60)).Year()
		}
		if err := validateTenkiSeason(kind, year); err != nil {
			return err
		}
		if err := validateTenkiPlace(place); err != nil {
			return err
		}
		if !strings.Contains(place, "/"+kind+"/") {
			return usageErr(fmt.Errorf("--place must match the requested --kind %s", kind))
		}
		client, err := read.source(flags, factory)
		if err != nil {
			return err
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		result, err := client.Seasonal(ctx, kind, place, year)
		if err != nil {
			return tenkiProductError(err)
		}
		tenkiWarnings(cmd, result.Warnings)
		return emitTenki(cmd, flags, client, result, result.Source)
	}
	return cmd
}
