package cli

import (
	"github.com/googlarz/betriebsrat/internal/betrvg"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"strings"
)

func newChecklistCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "checklist [situation]",
		Short: "Generate a step-by-step BR action checklist for a situation",
		Long: `Returns a prioritized, step-by-step action checklist for the Betriebsrat
in a given workplace situation. Covers legal obligations, deadlines,
and recommended actions with BetrVG references.

Supported situations: Kündigung, Betriebsänderung, Software-Einführung,
Einstellung, Versetzung, Homeoffice, and more.`,
		Example: strings.Trim(`
  betriebsrat checklist "Kündigung"
  betriebsrat checklist "Betriebsänderung" --json
  betriebsrat checklist "Homeoffice Regelung" --agent --select situation,steps`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			situation := strings.Join(args, " ")
			cl := betrvg.GetChecklist(situation)

			if cl == nil {
				msg := map[string]string{
					"situation": situation,
					"note":      "Keine spezifische Checkliste für diese Situation. Versuchen Sie: Kündigung, Betriebsänderung, Software, Einstellung, Homeoffice",
				}
				if flags.asJSON || flags.agent {
					enc := json.NewEncoder(cmd.OutOrStdout())
					enc.SetIndent("", "  ")
					return enc.Encode(msg)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Keine Checkliste für: %s\n\nVerfügbare Situationen:\n", situation)
				for _, c := range betrvg.AllChecklists() {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", c.Situation)
				}
				return nil
			}

			type output struct {
				Situation string                 `json:"situation"`
				Steps     []betrvg.ChecklistItem `json:"steps"`
			}
			out := output{Situation: cl.Situation, Steps: cl.Steps}

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Checkliste: %s\n\n", cl.Situation)
			for _, step := range cl.Steps {
				priority := ""
				if step.Priority == "high" {
					priority = " ⚠️ "
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%d.%s %s\n   Rechtsgrundlage: %s\n\n", step.Step, priority, step.Action, step.Paragraph)
			}
			return nil
		},
	}
	return cmd
}
