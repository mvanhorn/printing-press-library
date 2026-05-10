package cli

import (
	"github.com/googlarz/betriebsrat/internal/betrvg"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"strings"
)

func newCoDeterminationTypeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "codetermination-type [topic]",
		Short: "Classify the Betriebsrat's co-determination right type for a topic",
		Long: `Classifies the type of Betriebsrat right for a topic or situation:

  Mitbestimmung (erzwingbar) — BR can force a Betriebsvereinbarung via Einigungsstelle
  Zustimmungsvorbehalt       — Employer needs BR consent
  Mitwirkung                 — BR has participation but cannot block
  Beratung                   — Employer must consult but retains decision
  Unterrichtung              — Employer must inform BR
  Keine                      — No specific BR right found`,
		Example: strings.Trim(`
  betriebsrat codetermination-type Versetzung
  betriebsrat codetermination-type "Überwachungssoftware" --json
  betriebsrat codetermination-type Kündigung --agent`, "\n"),
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

			topic := strings.Join(args, " ")
			paragraphs := betrvg.ByKeywords(tokenize(topic))

			type result struct {
				Topic             string `json:"topic"`
				CoDetermType      string `json:"codetermination_type"`
				CanBlock          bool   `json:"can_block_employer"`
				CanForceAgreement bool   `json:"can_force_betriebsvereinbarung"`
				Paragraphs        []int  `json:"applicable_paragraphs,omitempty"`
				Explanation       string `json:"explanation"`
			}

			strongest := findStrongestRight(paragraphs)
			var nums []int
			for _, p := range paragraphs {
				nums = append(nums, p.Number)
			}

			r := result{
				Topic:        topic,
				CoDetermType: string(strongest),
			}

			switch strongest {
			case betrvg.MitbestimmungErzwingbar:
				r.CanBlock = true
				r.CanForceAgreement = true
				r.Explanation = "Der Betriebsrat hat erzwingbare Mitbestimmung. Der Arbeitgeber darf ohne Zustimmung des BR oder Spruch der Einigungsstelle (§ 76 BetrVG) nicht handeln. Der BR kann eine Betriebsvereinbarung erzwingen."
			case betrvg.Zustimmung:
				r.CanBlock = true
				r.CanForceAgreement = false
				r.Explanation = "Der Betriebsrat hat Zustimmungsvorbehalt. Der Arbeitgeber braucht die Zustimmung des BR. Diese kann aus bestimmten Gründen verweigert werden."
			case betrvg.Mitwirkung:
				r.CanBlock = false
				r.CanForceAgreement = false
				r.Explanation = "Mitwirkungsrecht: Der BR wird einbezogen und kann Einwände erheben, aber nicht blockieren. Praktischer Druck durch schriftliche Einwände und Öffentlichkeit ist trotzdem möglich."
			case betrvg.Beratung:
				r.CanBlock = false
				r.CanForceAgreement = false
				r.Explanation = "Beratungsrecht: Der Arbeitgeber muss den BR konsultieren und eine ernsthafte Beratung durchführen, ist an deren Ergebnis aber nicht gebunden."
			case betrvg.Unterrichtung:
				r.CanBlock = false
				r.CanForceAgreement = false
				r.Explanation = "Unterrichtungsrecht: Der AG muss den BR informieren. Kein Blockaderecht, aber Informationsanspruch ist durchsetzbar."
			default:
				r.Explanation = "Kein spezifisches Betriebsratsrecht in der Datenbank gefunden. Konsultieren Sie betriebsrat.de oder einen Fachanwalt."
			}
			r.Paragraphs = nums

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(r)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Thema: %s\n\n", r.Topic)
			fmt.Fprintf(cmd.OutOrStdout(), "Mitbestimmungsart: %s\n", r.CoDetermType)
			fmt.Fprintf(cmd.OutOrStdout(), "Kann blockieren: %v\n", r.CanBlock)
			fmt.Fprintf(cmd.OutOrStdout(), "Kann BV erzwingen: %v\n\n", r.CanForceAgreement)
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", r.Explanation)
			if len(r.Paragraphs) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\nAnwendbare §§: ")
				for i, n := range r.Paragraphs {
					if i > 0 {
						fmt.Fprint(cmd.OutOrStdout(), ", ")
					}
					fmt.Fprintf(cmd.OutOrStdout(), "§ %d", n)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}
	return cmd
}
