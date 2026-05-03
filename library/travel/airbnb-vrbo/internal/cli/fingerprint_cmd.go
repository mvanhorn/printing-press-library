package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-vrbo/internal/fingerprint"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-vrbo/internal/source/airbnb"
	"github.com/mvanhorn/printing-press-library/library/travel/airbnb-vrbo/internal/source/vrbo"
	"github.com/spf13/cobra"
)

func newFingerprintCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "fingerprint <listing-url>",
		Short:       "Compute a stable listing fingerprint",
		Example:     "  airbnb-vrbo-pp-cli fingerprint https://www.vrbo.com/h12345678",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			target := stripURLArg(args[0])
			ref, err := parseListingURL(target)
			if err != nil {
				return usageErr(err)
			}
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"hash": "", "components": map[string]string{}, "method": "dry_run"}, flags)
			}
			if ref.Platform == "airbnb" {
				l, err := airbnb.Get(cmd.Context(), ref.ID, airbnb.GetParams{})
				if err != nil {
					return classifyAPIError(err)
				}
				return printJSONFiltered(cmd.OutOrStdout(), fingerprint.FromAirbnb(l), flags)
			}
			l, err := vrbo.Get(cmd.Context(), ref.ID, vrbo.GetParams{})
			if err != nil {
				return classifyAPIError(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), fingerprint.FromVRBO(l), flags)
		},
	}
	return cmd
}
