package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/mt5/internal/config"
)

// `mt5-pp-cli config init` writes the example config file. `mt5-pp-cli config` (the
// existing command in foundation.go) reports resolved paths.

func newConfigInitCmd() *cobra.Command {
	var (
		path  string
		force bool
	)
	cmd := &cobra.Command{
		Use:   "config-init",
		Short: "Write an example ~/.config/mt5-pp-cli/config.toml",
		Long: `Create the config file at its default location with all guardrails set
to safe defaults, plus commented-out profile templates. Refuses to overwrite
unless --force is passed.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if path == "" {
				path = config.DefaultPath()
			}
			if force {
				_ = removeFile(path)
			}
			if err := config.WriteDefault(path); err != nil {
				return &ExitErr{Code: ExitConfig, Err: err}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote: %s\n", path)
			fmt.Fprintln(cmd.OutOrStdout(), "edit the file, then run: mt5-pp-cli doctor")
			return nil
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "Override config path")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite if the file already exists")
	return cmd
}

func removeFile(p string) error {
	return removeIgnore(p)
}
