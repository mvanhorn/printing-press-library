package cli

import (
	"github.com/googlarz/betriebsrat/internal/betrvg"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"strings"
)

type meetingPrep struct {
	Topic          string   `json:"topic"`
	AgendaItems    []string `json:"agenda_items"`
	QuorumNote     string   `json:"quorum_note"`
	QuestionsForAG []string `json:"questions_for_employer"`
	LegalBasis     []string `json:"legal_basis"`
	Resources      []string `json:"resources,omitempty"`
}

func newPrepareMeetingCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prepare-meeting [topic]",
		Short: "Generate agenda, quorum rules, and employer questions for a BR meeting",
		Long: `Generates a meeting preparation guide for a Betriebsrat meeting on a specific topic.
Includes: suggested agenda items, quorum requirements, questions to ask the employer,
and relevant legal basis.`,
		Example: strings.Trim(`
  betriebsrat prepare-meeting "Einführung KI-System"
  betriebsrat prepare-meeting Kündigung --json
  betriebsrat prepare-meeting "Homeoffice" --agent`, "\n"),
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

			prep := meetingPrep{
				Topic:      topic,
				QuorumNote: "Beschlussfähig wenn mehr als die Hälfte der BR-Mitglieder anwesend (§ 33 BetrVG). Beschluss mit einfacher Mehrheit der Anwesenden.",
			}

			// Build agenda
			prep.AgendaItems = []string{
				"1. Feststellung der Beschlussfähigkeit",
				"2. Genehmigung des letzten Protokolls",
				fmt.Sprintf("3. Tagesordnungspunkt: %s", topic),
				"4. Unterrichtung durch Arbeitgeber verlangen (falls erforderlich)",
				"5. Diskussion und Meinungsbildung",
				"6. Beschlussfassung",
				"7. Sonstiges",
			}

			// Build questions for employer based on paragraphs
			prep.QuestionsForAG = buildEmployerQuestions(topic, paragraphs)

			// Legal basis
			for _, p := range paragraphs {
				prep.LegalBasis = append(prep.LegalBasis,
					fmt.Sprintf("§ %d BetrVG – %s (%s)", p.Number, p.Title, string(p.CoDetermType)))
				if p.TopicURL != "" {
					prep.Resources = append(prep.Resources, p.TopicURL)
				}
			}

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(prep)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Meeting-Vorbereitung: %s\n\n", prep.Topic)
			fmt.Fprintf(cmd.OutOrStdout(), "Beschlussfähigkeit: %s\n\n", prep.QuorumNote)

			fmt.Fprintln(cmd.OutOrStdout(), "Vorgeschlagene Tagesordnung:")
			for _, item := range prep.AgendaItems {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", item)
			}

			if len(prep.QuestionsForAG) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nFragen an den Arbeitgeber:")
				for i, q := range prep.QuestionsForAG {
					fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s\n", i+1, q)
				}
			}

			if len(prep.LegalBasis) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nRechtliche Grundlagen:")
				for _, l := range prep.LegalBasis {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", l)
				}
			}

			return nil
		},
	}
	return cmd
}

func buildEmployerQuestions(topic string, paragraphs []betrvg.Paragraph) []string {
	var questions []string
	low := strings.ToLower(topic)

	// Generic questions based on right type
	strongest := findStrongestRight(paragraphs)
	switch strongest {
	case betrvg.MitbestimmungErzwingbar:
		questions = append(questions,
			"Welche konkreten Maßnahmen sind geplant und ab wann?",
			"Welche Arbeitnehmer sind betroffen?",
			"Warum ist diese Maßnahme notwendig? Welche Alternativen wurden geprüft?",
			"Welche Daten werden erhoben, gespeichert oder verarbeitet?",
		)
	case betrvg.Zustimmung:
		questions = append(questions,
			"Wurden alle relevanten Unterlagen vollständig vorgelegt?",
			"Warum wurde dieser Arbeitnehmer/diese Position ausgewählt?",
			"Welche Auswirkungen hat die Maßnahme auf andere Kollegen?",
		)
	}

	// Topic-specific questions
	if containsAny(low, "ki", "künstliche intelligenz", "software", "system") {
		questions = append(questions,
			"Welche Daten werden durch das System verarbeitet? Werden Leistungs- oder Verhaltensdaten erhoben?",
			"Wer hat Zugriff auf die erhobenen Daten?",
			"Gibt es eine Datenschutz-Folgenabschätzung (DSGVO Art. 35)?",
			"Welche Schulungen werden für Arbeitnehmer angeboten?",
		)
	}
	if containsAny(low, "kündigung", "entlassung") {
		questions = append(questions,
			"Welche Sozialdaten wurden bei der Sozialauswahl berücksichtigt?",
			"Gibt es anderweitige Beschäftigungsmöglichkeiten im Betrieb?",
			"Liegt eine Schwerbehinderung vor? Wurde das Integrationsamt eingeschaltet?",
		)
	}
	if containsAny(low, "homeoffice", "remote", "mobil") {
		questions = append(questions,
			"Für welche Tätigkeiten/Positionen soll Homeoffice gelten?",
			"Wie werden Arbeitszeiten im Homeoffice erfasst und kontrolliert?",
			"Wer trägt die Kosten für Ausstattung (Möbel, Internet, Strom)?",
		)
	}

	if len(questions) == 0 {
		questions = []string{
			"Was genau ist geplant und welchen Zeitrahmen sieht der Arbeitgeber vor?",
			"Welche Arbeitnehmer und Bereiche sind betroffen?",
			"Welche Informationen kann der Arbeitgeber jetzt bereitstellen?",
		}
	}
	return questions
}
