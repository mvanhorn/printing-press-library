// PATCH (instacart-regional-host): `instacart config set-host` persists the
// regional Instacart site so users don't have to hand-edit config.json.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/auth"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/config"
)

func newConfigSetHostCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-host <hostname>",
		Short: "Set the Instacart site to use (e.g. www.instacart.ca); default www.instacart.com",
		Long: `Selects the regional Instacart site. The host drives the GraphQL endpoint,
the Origin/Referer headers, and which browser cookies ` + "`auth login`" + ` imports.
Supported sites: www.instacart.com and www.instacart.ca.

The INSTACART_HOST environment variable overrides this setting for a single
invocation. Pass "default" to go back to www.instacart.com. Run
` + "`instacart auth login`" + ` again after switching sites: sessions are per site.`,
		Example: "  instacart config set-host www.instacart.ca\n  instacart config set-host default",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			value := args[0]
			if value == "default" {
				value = ""
			}
			host, err := auth.NormalizeHost(value)
			if err != nil {
				return coded(ExitUsage, "%v", err)
			}
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if host == auth.DefaultHost {
				cfg.Host = ""
			} else {
				cfg.Host = host
			}
			if dryRun, _ := cmd.Flags().GetBool("dry-run"); dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] would set host to %s\n", host)
				return nil
			}
			if err := cfg.Save(); err != nil {
				return err
			}
			if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"saved": true, "host": host})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "host set to %s\n", host)
			return nil
		},
	}
}
