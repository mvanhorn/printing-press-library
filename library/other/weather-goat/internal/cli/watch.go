package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/weather-goat/internal/config"

	"github.com/spf13/cobra"
)

func newWatchCmd(flags *rootFlags) *cobra.Command {
	var flagLat float64
	var flagLon float64
	var flagInterval int

	cmd := &cobra.Command{
		Use:   "watch [location]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Continuously poll NWS alerts and print new ones as they arrive",
		Long:  "Poll the NWS alerts endpoint at a regular interval and display new alerts as they appear. Useful during severe weather events. Exit with Ctrl+C.",
		Example: `  weather-goat-pp-cli watch
  weather-goat-pp-cli watch "Oklahoma City" --interval 30
  weather-goat-pp-cli watch --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				fmt.Fprintln(cmd.ErrOrStderr(), "GET /alerts/active (NWS, polling)")
				return nil
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}

			lat, lon, locName, err := resolveLocation(cfg, flagLat, flagLon,
				cmd.Flags().Changed("latitude"), cmd.Flags().Changed("longitude"), args)
			if err != nil {
				return err
			}

			interval := time.Duration(flagInterval) * time.Second
			fmt.Fprintf(cmd.ErrOrStderr(), "Watching alerts for %s (every %ds). Press Ctrl+C to stop.\n", locName, flagInterval)

			// Track seen alert IDs to only print new ones
			seen := map[string]bool{}

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt)
			defer signal.Stop(sigCh)

			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			// Run immediately, then on each tick
			poll := func() {
				alerts, err := nwsAlerts(lat, lon)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "error: %v\n", err)
					return
				}

				for _, a := range alerts {
					id, _ := a["id"].(string)
					if id == "" {
						// Use event + effective as fallback ID
						event, _ := a["event"].(string)
						effective, _ := a["effective"].(string)
						id = event + effective
					}
					if seen[id] {
						continue
					}
					seen[id] = true

					if flags.asJSON {
						b, _ := json.Marshal(a)
						fmt.Fprintln(cmd.OutOrStdout(), string(b))
					} else {
						event, _ := a["event"].(string)
						severity, _ := a["severity"].(string)
						headline, _ := a["headline"].(string)
						expires, _ := a["expires"].(string)

						ts := time.Now().Format("15:04:05")
						fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s", ts, bold(event))
						if severity != "" {
							fmt.Fprintf(cmd.OutOrStdout(), " [%s]", severity)
						}
						fmt.Fprintln(cmd.OutOrStdout())
						if headline != "" {
							fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", headline)
						}
						if expires != "" {
							fmt.Fprintf(cmd.OutOrStdout(), "  Expires: %s\n", formatTimeShort(expires))
						}
					}
				}

				if len(alerts) == 0 && len(seen) == 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "No active alerts. Watching...\n")
				}
			}

			poll()
			for {
				select {
				case <-ticker.C:
					poll()
				case <-sigCh:
					fmt.Fprintln(cmd.ErrOrStderr(), "\nStopped watching.")
					return nil
				}
			}
		},
	}

	cmd.Flags().Float64Var(&flagLat, "latitude", 0, "Latitude")
	cmd.Flags().Float64Var(&flagLon, "longitude", 0, "Longitude")
	cmd.Flags().IntVar(&flagInterval, "interval", 60, "Polling interval in seconds")

	return cmd
}
