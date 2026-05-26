package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/config"
	"github.com/mvanhorn/printing-press-library/library/payments/defillama/internal/format"
)

// configKey normalises both hyphenated and underscored forms.
// Canonical CLI keys: pro-key, stale-threshold, stale-historical.
func configKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "_", "-")
	switch s {
	case "stale-overview":
		// historical alias for the same setting
		return "stale-threshold"
	}
	return s
}

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage CLI configuration",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "show",
			Short: "Show current config (pro key is masked)",
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := config.Load()
				if err != nil {
					return err
				}
				pro := c.ResolvedProKey()
				if pro != "" {
					end := 4
					if end > len(pro) {
						end = len(pro)
					}
					pro = pro[:end] + "…(masked)"
				} else {
					pro = "(unset)"
				}
				rec := format.NewRecorder([]string{"KEY", "VALUE"})
				rec.Append([]format.Cell{format.Str("config-dir"), format.Str(config.Dir())})
				rec.Append([]format.Cell{format.Str("db-path"), format.Str(config.DBPath())})
				rec.Append([]format.Cell{format.Str("pro-key"), format.Str(pro)})
				rec.Append([]format.Cell{format.Str("stale-threshold"), format.Str(c.StaleOverview)})
				rec.Append([]format.Cell{format.Str("stale-historical"), format.Str(c.StaleHistorical)})
				return format.RenderRec(os.Stdout, mode(), rec, format.Options{NoHeader: G.NoHeader})
			},
		},
		&cobra.Command{
			Use:   "set <key> <value>",
			Short: "Set a config value (keys: pro-key, stale-threshold, stale-historical)",
			Args:  cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				c, err := config.Load()
				if err != nil {
					return err
				}
				switch configKey(args[0]) {
				case "pro-key":
					c.ProKey = args[1]
				case "stale-threshold":
					c.StaleOverview = args[1]
				case "stale-historical":
					c.StaleHistorical = args[1]
				default:
					return fmt.Errorf("unknown config key %q (try pro-key, stale-threshold, stale-historical)", args[0])
				}
				if err := config.Save(c); err != nil {
					return err
				}
				fmt.Fprintln(os.Stderr, "config updated")
				return nil
			},
		},
	)
	return cmd
}
