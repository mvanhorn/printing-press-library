package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/other/weather-goat/internal/config"

	"github.com/spf13/cobra"
)

func newConfigCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Manage weather CLI configuration (location, commute times)",
	}

	cmd.AddCommand(newSetLocationCmd(flags))
	cmd.AddCommand(newSetCommuteCmd(flags))
	cmd.AddCommand(newShowConfigCmd(flags))

	return cmd
}

func newSetLocationCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "set-location <city name>",
		Short: "Save your default location (resolves via geocoding)",
		Example: `  weather-goat-pp-cli config set-location "San Francisco"
  weather-goat-pp-cli config set-location "Tokyo, Japan"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			lat, lon, displayName, err := geocodeLookup(name)
			if err != nil {
				return fmt.Errorf("could not resolve location %q: %w", name, err)
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}

			if err := cfg.SaveLocation(lat, lon, displayName); err != nil {
				return configErr(fmt.Errorf("saving location: %w", err))
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Location saved: %s (%.4f, %.4f)\n", displayName, lat, lon)
			fmt.Fprintf(cmd.OutOrStdout(), "Config: %s\n", cfg.Path)
			return nil
		},
	}
}

func newSetCommuteCmd(flags *rootFlags) *cobra.Command {
	var depart string
	var ret string

	cmd := &cobra.Command{
		Use:   "set-commute",
		Short: "Save your commute departure and return times",
		Example: `  weather-goat-pp-cli config set-commute --depart 08:00 --return 18:00
  weather-goat-pp-cli config set-commute --depart 07:30 --return 17:00`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}

			if err := cfg.SaveCommute(depart, ret); err != nil {
				return configErr(fmt.Errorf("saving commute times: %w", err))
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Commute times saved: depart %s, return %s\n", depart, ret)
			fmt.Fprintf(cmd.OutOrStdout(), "Config: %s\n", cfg.Path)
			return nil
		},
	}

	cmd.Flags().StringVar(&depart, "depart", "08:00", "Departure time (HH:MM)")
	cmd.Flags().StringVar(&ret, "return", "18:00", "Return time (HH:MM)")

	return cmd
}

func newShowConfigCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "show",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:   "Display current configuration",
		Example: "  weather-goat-pp-cli config show",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Config: %s\n", cfg.Path)
			fmt.Fprintln(w)

			if cfg.LocationName != "" {
				fmt.Fprintf(w, "Location: %s (%.4f, %.4f)\n", cfg.LocationName, cfg.Latitude, cfg.Longitude)
			} else {
				fmt.Fprintln(w, "Location: not set")
			}

			if cfg.CommuteDepartTime != "" {
				fmt.Fprintf(w, "Commute: depart %s, return %s\n", cfg.CommuteDepartTime, cfg.CommuteReturnTime)
			} else {
				fmt.Fprintln(w, "Commute: not set (defaults to 08:00 / 18:00)")
			}

			return nil
		},
	}
}
