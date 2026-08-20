// Package cli wires up the masterpark-pp-cli command tree.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/masterpark/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/masterpark/internal/config"
)

type globalOpts struct {
	json       bool
	configPath string
	timeout    time.Duration
}

// IsVerifyEnv reports whether the CLI is running under the Printing Press
// verifier, in which case mutating commands must no-op.
func IsVerifyEnv() bool {
	return os.Getenv("PRINTING_PRESS_VERIFY") == "1"
}

const version = "0.0.0-dev"

// Execute builds and runs the root command.
func Execute() error {
	g := &globalOpts{}
	root := &cobra.Command{
		Use:           "masterpark-pp-cli",
		Short:         "Unofficial CLI for the MasterPark (netParkV2) reservation API",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate("masterpark-pp-cli {{ .Version }}\n")
	root.PersistentFlags().BoolVar(&g.json, "json", false, "output JSON")
	root.PersistentFlags().StringVar(&g.configPath, "config", "", "config file path (default ~/.config/masterpark-pp-cli/config.json)")
	root.PersistentFlags().DurationVar(&g.timeout, "timeout", 30*time.Second, "HTTP timeout")

	root.AddCommand(
		newLocationsCmd(g),
		newQuoteCmd(g),
		newAuthCmd(g),
		newReservationsCmd(g),
		newReserveCmd(g),
		newAgentContextCmd(g),
	)
	return root.Execute()
}

func (g *globalOpts) newClient() *client.Client {
	base := os.Getenv("MASTERPARK_BASE_URL")
	if base == "" {
		if f, err := g.loadConfig(); err == nil && f != nil {
			base = f.BaseURL
		}
	}
	return client.New(base, g.timeout)
}

func (g *globalOpts) ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), g.timeout+5*time.Second)
}

func (g *globalOpts) loadConfig() (*config.File, error) {
	return config.Load(g.configPath)
}

func printJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printRawJSON(raw json.RawMessage) error {
	var pretty interface{}
	if err := json.Unmarshal(raw, &pretty); err != nil {
		fmt.Println(string(raw))
		return nil
	}
	return printJSON(pretty)
}
