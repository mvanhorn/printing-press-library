package cmd

import (
	"fmt"

	"github.com/silentnw/govprocure-pp-cli/internal/config"
	"github.com/spf13/cobra"
)

func newAuthCmd() *cobra.Command {
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage API credentials",
	}
	authCmd.AddCommand(newSetKeyCmd())
	return authCmd
}

func newSetKeyCmd() *cobra.Command {
	var samKey string

	cmd := &cobra.Command{
		Use:   "set-key",
		Short: "Store API key(s) in config",
		Long: `Store API keys in ~/.config/govprocure-pp-cli/config.toml.

Currently supported:
  --sam   SAM.gov API key (required for sam commands)

Get a SAM.gov API key at: https://sam.gov/content/entity-registration`,
		Example: `  govprocure-pp-cli auth set-key --sam YOUR_SAM_API_KEY`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if samKey == "" {
				return fmt.Errorf("provide at least one key (--sam KEY)")
			}
			if samKey != "" {
				if err := config.SetSAMKey(samKey); err != nil {
					return fmt.Errorf("save SAM key: %w", err)
				}
				path, _ := config.ConfigPath()
				fmt.Printf("SAM API key saved to %s\n", path)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&samKey, "sam", "", "SAM.gov API key")
	return cmd
}
