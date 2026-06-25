// uk-train-goat hand-authored: service delay-reason briefing. Transcendence feature.
//
// Composes GetServiceDetails with the calling-points view and reports the
// service status in plain prose. The upstream service-detail payload has no
// delay-reason field; the cause and operator alerts live on the board
// (board/arrivals), so this view reports status, not the reason text.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/config"
	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/openldbws"

	"github.com/spf13/cobra"
)

func newWhyCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "why <serviceID>",
		Short:       "Explain what's happening with a single train service (transcendence)",
		Long:        "Look up a service by ID and surface its status (on time, running late, or cancelled), current platform, expected vs scheduled times, and the calling-points context, in one terminal-friendly view. The delay-reason text and operator alerts (including strike notices) come from the live board: see the board and arrivals commands.",
		Example:     "  uk-train-goat-pp-cli why L8rW0bMonHt3K4IengVPQw== --json",
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

			// pp:client-call — wraps openldbws.Service -> GetServiceDetails.
			details, err := c.Service(strings.TrimSpace(args[0]))
			if err != nil {
				return apiErr(err)
			}

			explanation := summarizeDelay(details)
			payload := map[string]any{
				"service":     serializeService(details),
				"explanation": explanation,
			}
			out, _ := json.Marshal(payload)
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

// summarizeDelay converts the structured service details into a one-line
// explanation in plain prose. The CLI's value-add over a thin wrapper is
// composing this surface; agents can quote the explanation directly.
func summarizeDelay(s *openldbws.TrainServiceDetails) string {
	if s == nil {
		return "no service detail available"
	}
	std := ptrOrEmpty(s.STD)
	etd := ptrOrEmpty(s.ETD)
	platform := ptrOrEmpty(s.Platform)
	switch {
	case etd == "On time":
		if platform == "" {
			return fmt.Sprintf("On time. Scheduled %s. Operator: %s.", std, s.Operator)
		}
		return fmt.Sprintf("On time. Scheduled %s, platform %s. Operator: %s.", std, platform, s.Operator)
	case strings.EqualFold(etd, "Cancelled"):
		return fmt.Sprintf("Cancelled. Scheduled %s. Operator: %s.", std, s.Operator)
	case strings.EqualFold(etd, "Delayed"):
		return fmt.Sprintf("Delayed (no live time). Scheduled %s. Operator: %s.", std, s.Operator)
	case etd != "" && etd != std:
		return fmt.Sprintf("Running late. Scheduled %s, expected %s. Platform: %s. Operator: %s.",
			std, etd, platformOrTBD(platform), s.Operator)
	default:
		return fmt.Sprintf("Service info available. Scheduled %s, platform %s. Operator: %s.",
			std, platformOrTBD(platform), s.Operator)
	}
}

func platformOrTBD(p string) string {
	if p == "" {
		return "TBD"
	}
	return p
}
