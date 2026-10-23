package cli

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/cloud/cloud-run-admin/internal/config"
	"github.com/spf13/cobra"
)

func addAccountCommands(auth *cobra.Command, flags *rootFlags) {
	auth.AddCommand(&cobra.Command{Use: "accounts", Short: "List saved account names (never credentials)", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		names, err := config.ListAccounts()
		if err != nil {
			return err
		}
		def, err := config.DefaultAccount()
		if err != nil {
			return err
		}
		_, active, err := config.ResolveAccount(flags.configPath, flags.accountName)
		if err != nil {
			return err
		}
		return flags.printJSON(cmd, map[string]any{"accounts": names, "default_account": def, "active_account": active})
	}})
	auth.AddCommand(&cobra.Command{Use: "set-default <name>", Short: "Choose the default saved Google account", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.SetDefaultAccount(args[0]); err != nil {
			return err
		}
		return flags.printJSON(cmd, map[string]any{"default_account": args[0]})
	}})
	auth.AddCommand(&cobra.Command{Use: "remove <name>", Short: "Remove saved credentials; preserve local data", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !flags.yes {
			return fmt.Errorf("confirmation required: pass --yes")
		}
		if err := config.RemoveAccount(args[0]); err != nil {
			return err
		}
		return flags.printJSON(cmd, map[string]any{"removed": args[0]})
	}})
	var source string
	add := &cobra.Command{Use: "add <name>", Short: "Import a saved OAuth config (0600); or use --account <name> auth login", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if source == "" {
			return fmt.Errorf("--from-config is required; alternatively use --account <name> auth login")
		}
		p, err := config.AccountPath(args[0])
		if err != nil {
			return err
		}
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			return fmt.Errorf("account already exists or cannot be accessed")
		}
		// Parse independently of ambient token and account selectors.
		data, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		if err := config.ImportAccount(args[0], data); err != nil {
			return err
		}
		return flags.printJSON(cmd, map[string]any{"added": args[0]})
	}}
	add.Flags().StringVar(&source, "from-config", "", "OAuth config TOML to copy into the named account")
	auth.AddCommand(add)
}
