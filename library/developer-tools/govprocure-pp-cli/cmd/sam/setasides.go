package sam

import (
	"fmt"
	"strings"

	"github.com/silentnw/govprocure-pp-cli/internal/api"
	"github.com/silentnw/govprocure-pp-cli/internal/config"
	"github.com/silentnw/govprocure-pp-cli/internal/db"
	"github.com/silentnw/govprocure-pp-cli/internal/output"
	"github.com/spf13/cobra"
)

func newSetAsidesCmd(opts *output.Options) *cobra.Command {
	var setAside string
	var limit int

	cmd := &cobra.Command{
		Use:   "set-asides",
		Short: "Filter SAM opportunities by set-aside code",
		Long: `Filter SAM.gov opportunities by set-aside type.

Available codes:
  sdvosb   — Service-Disabled Veteran-Owned Small Business
  wosb     — Women-Owned Small Business
  8a       — 8(a) Small Business
  hubzone  — HUBZone Small Business
  sba      — Small Business (general)`,
		Example: `  govprocure-pp-cli sam set-asides --type sdvosb
  govprocure-pp-cli sam set-asides --type wosb --limit 50
  govprocure-pp-cli sam set-asides --type 8a --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if setAside == "" {
				// Print all set-asides summary
				return listAllSetAsides(opts)
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if opts.DryRun {
				output.DryRunMsg(fmt.Sprintf("sam set-asides --type %q", setAside))
				return nil
			}

			code := strings.ToLower(setAside)
			saCode, ok := api.SetAsideCodes[code]
			if !ok {
				saCode = strings.ToUpper(setAside) // pass through if not in map
			}

			database, err := db.Open(cfg.DBPath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer database.Close()

			opps, err := database.SAMBySetAside(saCode, limit)
			if err != nil {
				return fmt.Errorf("query set-asides: %w", err)
			}

			if len(opps) == 0 {
				fmt.Printf("No %s opportunities in local DB. Run 'sam sync' first.\n", setAside)
				return nil
			}

			renderSAM(opps, opts)
			return nil
		},
	}

	cmd.Flags().StringVar(&setAside, "type", "", "set-aside type: sdvosb|wosb|8a|hubzone|sba")
	cmd.Flags().IntVar(&limit, "limit", 50, "max results")
	return cmd
}

func listAllSetAsides(opts *output.Options) error {
	if output.EffectiveFormat(opts) == output.FormatJSON {
		codes := make([]map[string]string, 0, len(api.SetAsideCodes))
		for name, code := range api.SetAsideCodes {
			codes = append(codes, map[string]string{"name": name, "code": code})
		}
		output.PrintJSON(codes)
		return nil
	}

	fmt.Println("Available set-aside codes:")
	fmt.Println()
	headers := []string{"NAME", "SAM CODE", "DESCRIPTION"}
	rows := [][]string{
		{"sdvosb", "SBP", "Service-Disabled Veteran-Owned Small Business"},
		{"wosb", "WOSB", "Women-Owned Small Business"},
		{"8a", "8AN", "8(a) Small Business Program"},
		{"hubzone", "HZC", "Historically Underutilized Business Zone"},
		{"sba", "SBA", "Small Business (general)"},
	}
	output.PrintTable(headers, rows)
	fmt.Println()
	fmt.Println("Usage: govprocure-pp-cli sam set-asides --type sdvosb")
	return nil
}
