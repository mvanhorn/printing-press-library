package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// bmr [--sex F|M --age N --weight-kg X --height-cm Y --activity LEVEL]
// Mifflin-St Jeor formula (more accurate than Harris-Benedict for modern
// populations). Default unit is kJ to match the rest of the CLI; pass
// --unit kcal for kcal output.
func newKTBMRCmd(flags *rootFlags) *cobra.Command {
	var sex string
	var age int
	var weightKg float64
	var heightCm float64
	var activity string
	var unit string

	cmd := &cobra.Command{
		Use:   "bmr",
		Short: "Compute BMR (Mifflin-St Jeor) and daily energy needs",
		Long: `Computes basal metabolic rate via the Mifflin-St Jeor formula plus
your activity multiplier. Returns BMR, TDEE (total daily energy
expenditure), and the value the Kalorické Tabulky web UI calls
"AMR energy" (effectively TDEE).

Inputs default to none — provide what you have. If you've already set
your profile on kaloricketabulky.cz, fetch the values directly with
'summary --json' (basal, amr, amrEnergy) instead.`,
		Example: `  kaloricke-tabulky-pp-cli bmr --sex M --age 38 --weight-kg 80 --height-cm 178 --activity moderate
  kaloricke-tabulky-pp-cli bmr --sex F --age 34 --weight-kg 65 --height-cm 168 --activity light --unit kcal`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if weightKg <= 0 || heightCm <= 0 || age <= 0 {
				return fmt.Errorf("provide --age, --weight-kg, and --height-cm (sex defaults to M if unset)")
			}
			s := strings.ToUpper(sex)
			if s == "" {
				s = "M"
			}
			var bmrKcal float64
			switch s {
			case "M", "MALE":
				bmrKcal = 10*weightKg + 6.25*heightCm - 5*float64(age) + 5
			case "F", "FEMALE":
				bmrKcal = 10*weightKg + 6.25*heightCm - 5*float64(age) - 161
			default:
				return fmt.Errorf("--sex must be M or F")
			}
			var factor float64
			switch strings.ToLower(activity) {
			case "sedentary":
				factor = 1.2
			case "light":
				factor = 1.375
			case "moderate", "":
				factor = 1.55
			case "active":
				factor = 1.725
			case "very-active":
				factor = 1.9
			default:
				return fmt.Errorf("--activity must be sedentary|light|moderate|active|very-active")
			}
			tdeeKcal := bmrKcal * factor

			unitOut := strings.ToLower(unit)
			if unitOut == "" {
				unitOut = "kj"
			}
			scale := 4.184
			if unitOut == "kcal" {
				scale = 1
			}
			return ktEmit(cmd.OutOrStdout(), flags, map[string]any{
				"sex":             s,
				"age":             age,
				"weight_kg":       weightKg,
				"height_cm":       heightCm,
				"activity_level":  activity,
				"activity_factor": factor,
				"bmr_kcal":        bmrKcal,
				"tdee_kcal":       tdeeKcal,
				"unit":            unitOut,
				"bmr":             bmrKcal * scale,
				"tdee":            tdeeKcal * scale,
				"formula":         "Mifflin-St Jeor",
			})
		},
	}
	cmd.Flags().StringVar(&sex, "sex", "", "Biological sex: M or F (default M)")
	cmd.Flags().IntVar(&age, "age", 0, "Age in years")
	cmd.Flags().Float64Var(&weightKg, "weight-kg", 0, "Weight in kg")
	cmd.Flags().Float64Var(&heightCm, "height-cm", 0, "Height in cm")
	cmd.Flags().StringVar(&activity, "activity", "moderate", "Activity level: sedentary|light|moderate|active|very-active")
	cmd.Flags().StringVar(&unit, "unit", "kj", "Energy unit: kj (default) or kcal")
	return cmd
}
