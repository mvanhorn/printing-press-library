package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

func newStatusWatchCmd(flags *rootFlags) *cobra.Command {
	var watch bool
	var until string

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Monitor AI agent status with periodic polling",
		Example: strings.Trim(`
  zylos-pp-cli status watch --watch
  zylos-pp-cli status watch --watch --until idle
  zylos-pp-cli status watch --watch --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !watch {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			sig := make(chan os.Signal, 1)
			signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)

			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()

			fmt.Fprintf(os.Stderr, "Watching status every 3s (Ctrl+C to stop)\n")

			for {
				data, err := c.Get("/api/status", nil)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: poll failed: %v\n", err)
				} else {
					var status map[string]any
					if json.Unmarshal(data, &status) == nil {
						if flags.asJSON {
							enc := json.NewEncoder(cmd.OutOrStdout())
							enc.SetIndent("", "  ")
							enc.Encode(status)
						} else {
							state, _ := status["status"].(string)
							ts := time.Now().Format("15:04:05")
							fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s\n", ts, state)
						}

						// Check --until condition
						if until != "" {
							if state, _ := status["status"].(string); strings.EqualFold(state, until) {
								fmt.Fprintf(os.Stderr, "Target state reached: %s\n", until)
								return nil
							}
						}
					}
				}

				select {
				case <-sig:
					fmt.Fprintln(os.Stderr, "\nShutting down gracefully...")
					return nil
				case <-ticker.C:
					continue
				}
			}
		},
	}

	cmd.Flags().BoolVar(&watch, "watch", false, "Enable periodic status polling every 3 seconds")
	cmd.Flags().StringVar(&until, "until", "", "Exit when state matches (idle, busy, offline, stopped)")

	return cmd
}
