// pp:data-source live
package cli

import (
	"context"
	"github.com/mvanhorn/printing-press-library/library/travel/jma/internal/jma"
	"github.com/spf13/cobra"
)

type jmaRunner func(func(context.Context, *jma.Client) (jma.Envelope, error)) func(*cobra.Command, []string) error

func newJMAForecastGetCmd(run jmaRunner) *cobra.Command {
	var area, period, station string
	var days int
	cmd := &cobra.Command{Use: "get", Short: "Retrieve resolved short/weekly forecasts with units and valid times", Example: "  jma-pp-cli forecast get --area 130010 --period all --days 3", Annotations: map[string]string{"pp:endpoint": "forecast.get", "mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--area=130010"}, RunE: run(func(ctx context.Context, c *jma.Client) (jma.Envelope, error) {
		return c.Forecast(ctx, area, period, station, days)
	})}
	cmd.Flags().StringVar(&area, "area", "", "JMA source area ID or exact Japanese/English name")
	cmd.Flags().StringVar(&period, "period", "all", "Forecast period: short, week or all")
	cmd.Flags().StringVar(&station, "station", "", "Optional forecast temperature station ID within resolved district")
	cmd.Flags().IntVar(&days, "days", 3, "Forecast days to return from source issue date, 1..7")
	return cmd
}
