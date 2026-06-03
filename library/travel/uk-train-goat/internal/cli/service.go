// uk-train-goat hand-authored: service status command.
package cli

import (
	"encoding/json"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/config"
	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/openldbws"

	"github.com/spf13/cobra"
)

func newServiceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service <serviceID>",
		Short: "Live status / platform / formation for a single service",
		Long:  "Look up real-time status for a specific train by service ID. The service ID is returned by `board` and `arrivals` results.",
		Example: strings.Trim(`
  uk-train-goat-pp-cli service L8rW0bMonHt3K4IengVPQw==
  uk-train-goat-pp-cli service L8rW0bMonHt3K4IengVPQw== --json --select platform,etd,subsequent_calling_points
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

			// pp:client-call — wraps openldbws.Service -> OpenLDBWS GetServiceDetails.
			svc, err := c.Service(strings.TrimSpace(args[0]))
			if err != nil {
				return apiErr(err)
			}
			data, _ := json.Marshal(serializeService(svc))
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	return cmd
}

func serializeService(s *openldbws.TrainServiceDetails) map[string]any {
	if s == nil {
		return nil
	}
	out := map[string]any{
		"location_name": s.LocationName,
		"crs":           s.CRS,
		"operator":      s.Operator,
		"operator_code": s.OperatorCode,
		"service_type":  s.ServiceType,
		"platform":      ptrOrEmpty(s.Platform),
		"sta":           ptrOrEmpty(s.STA),
		"eta":           ptrOrEmpty(s.ETA),
		"std":           ptrOrEmpty(s.STD),
		"etd":           ptrOrEmpty(s.ETD),
		"generated_at":  s.GeneratedAt,
	}
	out["previous_calling_points"] = serializeLocations(s.PreviousCallingPoints)
	out["subsequent_calling_points"] = serializeLocations(s.SubsequentCallingPoints)
	return out
}

func serializeLocations(locs []*openldbws.Location) []map[string]any {
	out := make([]map[string]any, 0, len(locs))
	for _, l := range locs {
		if l == nil {
			continue
		}
		out = append(out, map[string]any{
			"crs":  l.CRS,
			"name": l.Name,
		})
	}
	return out
}
