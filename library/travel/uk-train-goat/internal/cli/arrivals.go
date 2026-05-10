// uk-train-goat hand-authored: live arrivals board command.
package cli

import (
	"encoding/json"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/config"
	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/openldbws"

	"github.com/spf13/cobra"
)

func newArrivalsCmd(flags *rootFlags) *cobra.Command {
	var (
		fromCRS   string
		numRows   int
		offsetStr string
		windowStr string
	)
	cmd := &cobra.Command{
		Use:   "arrivals <crs>",
		Short: "Live arrivals at a UK rail station",
		Long:  "Show the next arrivals at a UK rail station using OpenLDBWS. Mirror of `board` for arriving services.",
		Example: strings.Trim(`
  uk-train-goat-pp-cli arrivals KGX --num 5
  uk-train-goat-pp-cli arrivals KGX --from EDB
  uk-train-goat-pp-cli arrivals KGX --json --select services.sta,services.eta,services.platform
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			c, err := openldbws.New(cfg.LdbwsApiToken)
			if err != nil {
				return authErr(err)
			}
			crs := strings.ToUpper(strings.TrimSpace(args[0]))
			if numRows == 0 {
				numRows = 10
			}
			offsetMin, err := parseDurationMinutes(offsetStr)
			if err != nil {
				return usageErr(err)
			}
			windowMin, err := parseDurationMinutes(windowStr)
			if err != nil {
				return usageErr(err)
			}

			// pp:client-call — wraps openldbws.Arrivals -> OpenLDBWS GetArrivalBoard.
			board, err := c.Arrivals(crs, fromCRS, numRows, offsetMin, windowMin)
			if err != nil {
				return apiErr(err)
			}
			payload := map[string]any{
				"origin":   crs,
				"name":     board.LocationName,
				"services": serializeArrivals(board),
				"messages": board.NRCCMessages,
			}
			data, _ := json.Marshal(payload)
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&fromCRS, "from", "", "Filter to services arriving from this origin CRS")
	cmd.Flags().IntVar(&numRows, "num", 0, "Maximum number of services to return (default 10, max 150)")
	cmd.Flags().StringVar(&offsetStr, "in", "", "Show arrivals starting after this duration (e.g. 30, 30m, 1h)")
	cmd.Flags().StringVar(&windowStr, "within", "", "Limit to arrivals within this duration (e.g. 30, 30m, 1h)")
	return cmd
}

func serializeArrivals(board *openldbws.StationBoard) []map[string]any {
	if board == nil {
		return nil
	}
	out := make([]map[string]any, 0, len(board.TrainServices))
	for _, s := range board.TrainServices {
		row := map[string]any{
			"sta":          ptrOrEmpty(s.STA),
			"eta":          ptrOrEmpty(s.ETA),
			"platform":     ptrOrEmpty(s.Platform),
			"operator":     s.Operator,
			"service_id":   s.ServiceID,
			"service_type": s.ServiceType,
		}
		if s.Origin != nil {
			row["origin"] = map[string]any{"crs": s.Origin.CRS, "name": s.Origin.Name}
		}
		if s.DelayReason != nil {
			row["delay_reason"] = *s.DelayReason
		}
		out = append(out, row)
	}
	return out
}
