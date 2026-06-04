package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/walkingpad/internal/profile"
)

func newProfileCmd(flags *rootFlags) *cobra.Command {
	profileCmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage your body profile (weight, used for calorie estimates)",
	}
	profileCmd.AddCommand(newProfileSetCmd(flags))
	profileCmd.AddCommand(newProfileShowCmd(flags))
	return profileCmd
}

func newProfileSetCmd(flags *rootFlags) *cobra.Command {
	var weight float64
	cmd := &cobra.Command{
		Use: "set",
		// Hidden from MCP to keep the surface read-only (it mutates local config).
		Annotations: map[string]string{"mcp:hidden": "true"},
		Short:       "Set your body weight (kg) for calorie estimates",
		Example:     "  walkingpad-pp-cli profile set --weight 80",
		RunE: func(cmd *cobra.Command, args []string) error {
			if weight <= 0 {
				return fmt.Errorf("--weight must be greater than 0 (kg)")
			}
			path, err := profilePath()
			if err != nil {
				return err
			}
			p := profile.Profile{WeightKg: weight}
			if err := profile.Save(path, p); err != nil {
				return err
			}
			return emit(cmd, flags, p, fmt.Sprintf("saved profile: %.1f kg", weight))
		},
	}
	cmd.Flags().Float64Var(&weight, "weight", 0, "Body weight in kilograms")
	return cmd
}

func newProfileShowCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "show",
		Short:       "Show your saved body profile",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := profilePath()
			if err != nil {
				return err
			}
			p, ok, err := profile.Load(path)
			if err != nil {
				return err
			}
			if !ok {
				return emit(cmd, flags, map[string]any{"weight_kg": 0},
					"no profile set; run: walkingpad-pp-cli profile set --weight <kg>")
			}
			return emit(cmd, flags, p, fmt.Sprintf("weight: %.1f kg", p.WeightKg))
		},
	}
}
