package commands

import (
	"fmt"
	"os"

	goobs "github.com/andreykaipov/goobs"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/obs/internal/config"
	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "obs-pp-cli",
	Short: "OBS Studio control CLI — scenes, layouts, guests, streaming, recording",
	Long: `obs-pp-cli controls OBS Studio via its built-in WebSocket server.

Configure once with: obs-pp-cli configure
Then use subcommands to control OBS from your terminal or agent.`,
}

func init() {
	RootCmd.AddCommand(configureCmd)
	RootCmd.AddCommand(sceneCmd)
	RootCmd.AddCommand(streamCmd)
	RootCmd.AddCommand(recordCmd)
	RootCmd.AddCommand(layoutCmd)
	RootCmd.AddCommand(guestCmd)
	RootCmd.AddCommand(preflightCmd)
	RootCmd.AddCommand(statsCmd)
}

// connect loads config and returns an authenticated OBS client.
func connect() (*goobs.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	opts := []goobs.Option{}
	if cfg.Password != "" {
		opts = append(opts, goobs.WithPassword(cfg.Password))
	}

	client, err := goobs.New(cfg.Addr(), opts...)
	if err != nil {
		return nil, fmt.Errorf("could not connect to OBS at %s: %w\nIs OBS running with WebSocket server enabled?", cfg.Addr(), err)
	}
	return client, nil
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

func fatal(msg string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s: %v\n", msg, err)
	} else {
		fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
	}
	os.Exit(1)
}
