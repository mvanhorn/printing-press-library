package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/recipe-goat/internal/recipes"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

func newTrustCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "trust",
		Short:   "View and override per-site trust scores used by the cross-site ranker",
		Example: "  recipe-goat-pp-cli trust list",
	}
	cmd.AddCommand(newTrustListCmd(flags))
	cmd.AddCommand(newTrustSetCmd(flags))
	cmd.AddCommand(newTrustListCmd(flags))
	cmd.AddCommand(newTrustSetCmd(flags))
	return cmd
}

func newTrustListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List the current per-site trust scores (built-ins plus any local overrides)",
		Example:     "  recipe-goat-pp-cli trust list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.asJSON {
				payload := map[string]any{
					"sites": recipes.Sites,
				}
				return flags.printJSON(cmd, payload)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "SITES")
			headers := []string{"SITE", "TIER", "TRUST"}
			rows := make([][]string, 0, len(recipes.Sites))
			for _, s := range recipes.Sites {
				rows = append(rows, []string{s.Hostname, strconv.Itoa(s.Tier), fmt.Sprintf("%.2f", s.Trust)})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}
}

// trustOverrides persisted at ~/.config/recipe-goat-pp-cli/trust.toml.
// Shape: [sites] "host" = 0.8
//
// Author overrides were removed when author_trust was dropped from the
// scoring formula. Existing trust.toml files with an [authors] section
// still parse cleanly — the field is just unused going forward.
type trustOverrides struct {
	Sites map[string]float64 `toml:"sites"`
}

func trustPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "recipe-goat-pp-cli", "trust.toml")
}

func loadTrust() (trustOverrides, error) {
	out := trustOverrides{Sites: map[string]float64{}}
	data, err := os.ReadFile(trustPath())
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return out, err
	}
	if err := toml.Unmarshal(data, &out); err != nil {
		return out, err
	}
	if out.Sites == nil {
		out.Sites = map[string]float64{}
	}
	return out, nil
}

func saveTrust(t trustOverrides) error {
	p := trustPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := toml.Marshal(t)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

func newTrustSetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "set <site> <delta>",
		Short:   "Save a local per-site trust adjustment that the ranker applies on top of the built-in scores",
		Example: "  recipe-goat-pp-cli trust set seriouseats.com +2",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			delta, err := strconv.ParseFloat(args[1], 64)
			if err != nil {
				return usageErr(fmt.Errorf("delta must be a number, got %q", args[1]))
			}
			// Only site overrides are supported — author_trust was removed
			// from the scoring formula. Site hostnames contain a dot.
			if !strings.Contains(target, ".") || strings.Contains(target, " ") {
				return usageErr(fmt.Errorf("target %q is not a site hostname; author_trust was removed from the ranker", target))
			}
			overrides, err := loadTrust()
			if err != nil {
				return err
			}
			overrides.Sites[strings.ToLower(target)] = delta
			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "would save trust override for %s → %.2f\n", target, delta)
				return nil
			}
			if err := saveTrust(overrides); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "saved: %s → %.2f (ranking integration wip — override stored but not yet applied)\n", target, delta)
			return nil
		},
	}
}
