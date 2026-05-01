package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newAPICmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api [interface]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Access all 170 Steam Web API endpoints by interface name",
		Long: `Browse and call any Steam Web API endpoint using the raw interface names.

The friendly top-level commands (resolve, profile, games, etc.) cover the
most common operations. This command provides access to ALL 170 endpoints
across 54 interfaces for power users and agents that need full API coverage.

Run 'api' with no arguments to list all interfaces.
Run 'api <interface>' to see that interface's methods.`,
		Example: `  # List all available interfaces
  steam-web-pp-cli api

  # Show methods for ISteamUser
  steam-web-pp-cli api isteam-user

  # Call a specific method directly
  steam-web-pp-cli isteam-user get-player-summaries --steamids 76561197968052866`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()

			if len(args) > 0 {
				// Show methods for a specific interface
				target := strings.ToLower(args[0])
				for _, child := range root.Commands() {
					if child.Hidden && strings.ToLower(child.Name()) == target {
						methods := child.Commands()
						if len(methods) == 0 {
							return child.Help()
						}
						fmt.Fprintf(cmd.OutOrStdout(), "%s — %s\n\nMethods:\n", child.Name(), child.Short)
						for _, method := range methods {
							fmt.Fprintf(cmd.OutOrStdout(), "  %-50s %s\n", child.Name()+" "+method.Name(), method.Short)
						}
						fmt.Fprintf(cmd.OutOrStdout(), "\nUse 'steam-web-pp-cli %s <method> --help' for details.\n", child.Name())
						return nil
					}
				}
				return fmt.Errorf("interface %q not found. Run 'api' to list all interfaces", args[0])
			}

			// List all hidden interface commands
			var interfaces []string
			for _, child := range root.Commands() {
				if child.Hidden {
					interfaces = append(interfaces, fmt.Sprintf("  %-45s %s", child.Name(), child.Short))
				}
			}
			sort.Strings(interfaces)

			fmt.Fprintf(cmd.OutOrStdout(), "Available Steam Web API interfaces (%d):\n\n", len(interfaces))
			for _, line := range interfaces {
				fmt.Fprintln(cmd.OutOrStdout(), line)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nUse 'steam-web-pp-cli api <interface>' to see methods.\n")
			fmt.Fprintf(cmd.OutOrStdout(), "Use 'steam-web-pp-cli <interface> <method> --help' for method details.\n")
			return nil
		},
	}

	return cmd
}
