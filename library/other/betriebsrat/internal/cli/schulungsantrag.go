package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type schulungsantragResult struct {
	Topic        string `json:"topic"`
	TrainingName string `json:"training_name"`
	LegalBasis   string `json:"legal_basis"`
	Letter       string `json:"letter"`
	Deadline     string `json:"response_deadline"`
	Enforcement  string `json:"enforcement_if_denied"`
	Note         string `json:"note"`
}

func newSchulungsantragCmd(flags *rootFlags) *cobra.Command {
	var topic string
	var trainingName string
	var provider string
	var dateStr string
	var employer string

	cmd := &cobra.Command{
		Use:   "schulungsantrag",
		Short: tr(flags.lang, "Erstellt einen formellen Schulungsantrag nach § 37 Abs. 6 BetrVG", "Draft a formal BR training request under § 37 Abs. 6 BetrVG"),
		Long: tr(flags.lang,
			`Generiert ein formelles Schulungsantrag-Schreiben nach § 37 Abs. 6 BetrVG.

Betriebsratsmitglieder haben Anspruch auf Teilnahme an Schulungsveranstaltungen,
die für ihre BR-Arbeit erforderliche Kenntnisse vermitteln. Der Arbeitgeber muss
die Kosten tragen und den BR-Mitgliedern die nötige Freistellung gewähren.

Voraussetzungen (§ 37 Abs. 6 BetrVG):
  1. Erforderlichkeit: Schulung vermittelt Kenntnisse, die für BR-Arbeit benötigt werden
  2. Freistellung: AN muss von der Arbeit freigestellt werden (§ 37 Abs. 2)
  3. Kostenübernahme: Schulungs-, Reise- und Übernachtungskosten trägt der AG
  4. Beschlussfassung: BR muss Teilnahme durch Beschluss festlegen

Gängige Themen:
  betrvg      Betriebsverfassungsgesetz (Grundlagenseminar)
  arbeitsrecht  Individualarbeitsrecht für BR-Mitglieder
  betriebsrat-praxis  BR-Sitzungsleitung, Protokollführung, Beschlussfassung
  kuendigung  Kündigungsschutz und BR-Rechte bei Kündigung
  sozialplan  Sozialplan- und Interessenausgleich-Verhandlung
  datenschutz Datenschutz im Betrieb (DSGVO, BDSG)
  gesundheit  Arbeitsschutz und betriebliches Gesundheitsmanagement
  custom      Eigener Schulungsname (via --training-name)`,
			`Generates a formal training request letter under § 37 Abs. 6 BetrVG.

Works council members are entitled to attend training courses that impart knowledge
required for their works council duties. The employer must bear the costs and grant
the necessary time off to the BR members.

Prerequisites (§ 37 Abs. 6 BetrVG):
  1. Necessity: training imparts knowledge required for BR work
  2. Release from work: employee must be released from work obligations (§ 37 Abs. 2)
  3. Cost coverage: training, travel and accommodation costs borne by the employer
  4. BR resolution: BR must resolve to send the member by formal vote

Common topics:
  betrvg        Works Constitution Act (basic seminar)
  arbeitsrecht  Individual labour law for BR members
  betriebsrat-praxis  BR meeting management, minutes, resolutions
  kuendigung    Dismissal protection and BR rights in dismissals
  sozialplan    Sozialplan and Interessenausgleich negotiation
  datenschutz   Data protection (GDPR, BDSG)
  gesundheit    Occupational health and safety / company health management
  custom        Custom training name (via --training-name)`),
		Example: strings.Trim(`
  betriebsrat schulungsantrag --topic betrvg --employer "Musterfirma GmbH"
  betriebsrat schulungsantrag --topic kuendigung --provider "ver.di Bildung" --employer "AG GmbH" --agent
  betriebsrat schulungsantrag --topic custom --training-name "KI im Betrieb" --employer "TechCo" --lang en`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if topic == "" && trainingName == "" {
				return fmt.Errorf(tr(flags.lang,
					"--topic oder --training-name ist erforderlich",
					"--topic or --training-name is required"))
			}

			refDate := time.Now()
			if dateStr != "" {
				parsed, err := time.Parse("2006-01-02", dateStr)
				if err != nil {
					return fmt.Errorf("invalid --date (YYYY-MM-DD): %w", err)
				}
				refDate = parsed
			}

			employerStr := employer
			if employerStr == "" {
				employerStr = "[Arbeitgeber / Unternehmen]"
			}
			providerStr := provider
			if providerStr == "" {
				providerStr = "[Schulungsanbieter / Bildungswerk]"
			}

			resolvedName, legalJustification := resolveSchulungsTopic(flags.lang, topic, trainingName)

			letter := buildSchulungsantragLetter(flags.lang, employerStr, resolvedName, providerStr,
				legalJustification, refDate.Format("02.01.2006"))

			r := schulungsantragResult{
				Topic:        topic,
				TrainingName: resolvedName,
				LegalBasis:   "§ 37 Abs. 6 BetrVG i.V.m. § 37 Abs. 2 BetrVG",
				Letter:       letter,
				Deadline: tr(flags.lang,
					"Keine gesetzliche Frist; Praxis: Antwort binnen 2 Wochen erwarten",
					"No statutory deadline; in practice: expect response within 2 weeks"),
				Enforcement: tr(flags.lang,
					"Bei Ablehnung: BR kann Einigungsstelle anrufen oder Arbeitsgericht (Beschlussverfahren). Kosten- und Freistellungsanspruch ist notfalls gerichtlich durchsetzbar.",
					"If denied: BR can invoke the conciliation board or the labour court (Beschlussverfahren). The right to cost coverage and release from work is enforceable through the courts if necessary."),
				Note: tr(flags.lang,
					"Der BR muss die Teilnahme durch Beschluss festlegen (§ 33 BetrVG). Der AG kann die Freistellung nur aus dringenden betrieblichen Gründen verschieben, nicht dauerhaft verweigern (§ 37 Abs. 6 Satz 3).",
					"The BR must formally resolve the member's attendance (§ 33 BetrVG). The employer may only postpone release for urgent operational reasons, not permanently deny it (§ 37 Abs. 6 Satz 3)."),
			}

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(r)
			}

			fmt.Fprint(cmd.OutOrStdout(), letter)
			fmt.Fprintf(cmd.OutOrStdout(), "\n\n--- %s ---\n", tr(flags.lang, "Hinweise", "Notes"))
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", tr(flags.lang, "Rechtsgrundlage", "Legal basis"), r.LegalBasis)
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", tr(flags.lang, "Bei Ablehnung", "If denied"), r.Enforcement)
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", tr(flags.lang, "Hinweis", "Note"), r.Note)
			return nil
		},
	}

	cmd.Flags().StringVar(&topic, "topic", "", tr(flags.lang,
		"Schulungsthema: betrvg|arbeitsrecht|betriebsrat-praxis|kuendigung|sozialplan|datenschutz|gesundheit|custom",
		"Training topic: betrvg|arbeitsrecht|betriebsrat-praxis|kuendigung|sozialplan|datenschutz|gesundheit|custom"))
	cmd.Flags().StringVar(&trainingName, "training-name", "", tr(flags.lang, "Freier Schulungsname (wenn --topic custom)", "Custom training name (when --topic custom)"))
	cmd.Flags().StringVar(&provider, "provider", "", tr(flags.lang, "Schulungsanbieter / Bildungswerk", "Training provider"))
	cmd.Flags().StringVar(&employer, "employer", "", tr(flags.lang, "Name des Arbeitgebers", "Employer name"))
	cmd.Flags().StringVar(&dateStr, "date", "", "Datum des Schreibens (YYYY-MM-DD, Standard: heute)")
	return cmd
}

func resolveSchulungsTopic(lang, topic, customName string) (name, justification string) {
	type topicEntry struct{ de, en, justDE, justEN string }
	topics := map[string]topicEntry{
		"betrvg": {
			"BetrVG Grundlagenseminar",
			"BetrVG Basic Seminar",
			"Kenntnisse des Betriebsverfassungsgesetzes sind für jedes BR-Mitglied unverzichtbare Grundlage der BR-Tätigkeit (BAG, Beschluss v. 19.07.1995 – 7 ABR 49/94).",
			"Knowledge of the BetrVG is an indispensable foundation for every BR member (BAG, decision of 19.07.1995 – 7 ABR 49/94).",
		},
		"arbeitsrecht": {
			"Individualarbeitsrecht für Betriebsratsmitglieder",
			"Individual Labour Law for Works Council Members",
			"Kenntnisse des Individualarbeitsrechts sind für die Beratung und Unterstützung der Arbeitnehmer sowie für die Prüfung von Anhörungsschreiben (§ 102 BetrVG) erforderlich.",
			"Knowledge of individual labour law is required for advising employees and reviewing dismissal notices (§ 102 BetrVG).",
		},
		"betriebsrat-praxis": {
			"BR-Praxis: Sitzungsleitung, Protokollführung und Beschlussfassung",
			"BR Practice: Meeting Management, Minutes and Resolutions",
			"Korrekte Sitzungsführung und rechtssichere Beschlussfassung sind Grundvoraussetzung für wirksame BR-Arbeit (§§ 29–34 BetrVG).",
			"Correct meeting management and legally valid resolutions are a fundamental prerequisite for effective BR work (§§ 29–34 BetrVG).",
		},
		"kuendigung": {
			"Kündigungsschutz und BR-Beteiligungsrechte",
			"Dismissal Protection and BR Rights in Dismissals",
			"Kenntnisse zu Kündigungsschutz und BR-Anhörungsrechten (§ 102 BetrVG) sind für die Wahrnehmung des Anhörungsrechts bei jeder Kündigung zwingend erforderlich.",
			"Knowledge of dismissal protection and the BR hearing right (§ 102 BetrVG) is essential for exercising the hearing right in every dismissal case.",
		},
		"sozialplan": {
			"Sozialplan- und Interessenausgleich-Verhandlung",
			"Sozialplan and Interessenausgleich Negotiation",
			"Kenntnisse zu §§ 111–113 BetrVG und Verhandlungsführung sind für die Wahrnehmung von Mitbestimmungsrechten bei Betriebsänderungen erforderlich.",
			"Knowledge of §§ 111–113 BetrVG and negotiation skills are required to exercise co-determination rights in operational changes.",
		},
		"datenschutz": {
			"Datenschutz im Betrieb (DSGVO / BDSG)",
			"Data Protection in the Workplace (GDPR / BDSG)",
			"Der BR hat Überwachungs- und Kontrollaufgaben beim betrieblichen Datenschutz (§ 80 Abs. 1 Nr. 1 BetrVG). Aktuelle DSGVO-Kenntnisse sind hierfür unerlässlich.",
			"The BR has monitoring and control functions in workplace data protection (§ 80 Abs. 1 Nr. 1 BetrVG). Up-to-date GDPR knowledge is essential.",
		},
		"gesundheit": {
			"Arbeitsschutz und betriebliches Gesundheitsmanagement",
			"Occupational Health, Safety and Workplace Health Management",
			"Der BR hat Mitbestimmungsrechte beim Arbeitsschutz (§ 87 Abs. 1 Nr. 7 BetrVG) und Überwachungspflichten nach § 89 BetrVG. Fachkenntnisse sind für deren Wahrnehmung erforderlich.",
			"The BR has co-determination rights in occupational health (§ 87 Abs. 1 Nr. 7 BetrVG) and monitoring duties under § 89 BetrVG. Specialist knowledge is required.",
		},
	}

	if entry, ok := topics[topic]; ok {
		return tr(lang, entry.de, entry.en), tr(lang, entry.justDE, entry.justEN)
	}
	if customName != "" {
		return customName, tr(lang,
			"Die Schulung vermittelt Kenntnisse, die für die ordnungsgemäße Durchführung der Betriebsratstätigkeit erforderlich sind (§ 37 Abs. 6 BetrVG).",
			"The training imparts knowledge required for the proper exercise of works council duties (§ 37 Abs. 6 BetrVG).")
	}
	return topic, tr(lang,
		"Die Schulung vermittelt erforderliche Kenntnisse für die BR-Tätigkeit (§ 37 Abs. 6 BetrVG).",
		"The training imparts knowledge required for BR work (§ 37 Abs. 6 BetrVG).")
}

func buildSchulungsantragLetter(lang, employer, trainingName, provider, justification, date string) string {
	return fmt.Sprintf(`An die Geschäftsleitung
%s

Ort, %s

Betr.: Antrag auf Freistellung zur Schulungsteilnahme nach § 37 Abs. 6 BetrVG
       Schulung: %s

Sehr geehrte Damen und Herren,

der Betriebsrat hat in seiner Sitzung vom [Datum des BR-Beschlusses] beschlossen,
folgendes Mitglied zur nachstehend genannten Schulungsveranstaltung zu entsenden:

  Betriebsratsmitglied: [Name des BR-Mitglieds]
  Schulung:             %s
  Anbieter:             %s
  Termin:               [Datum der Schulung, z.B. XX.XX.XXXX – XX.XX.XXXX]
  Ort:                  [Ort der Schulung]

Erforderlichkeit der Schulung:

%s

Wir beantragen daher gemäß § 37 Abs. 6 BetrVG:
  1. Freistellung des genannten Mitglieds für die Dauer der Schulungsveranstaltung (§ 37 Abs. 2 BetrVG)
  2. Übernahme der anfallenden Kosten (Schulungsgebühr, Reise- und Übernachtungskosten) durch den Arbeitgeber (§ 37 Abs. 6 Satz 3 BetrVG)

Bitte teilen Sie uns Ihre Entscheidung bis zum [Datum, z.B. 2 Wochen vor Schulungsbeginn] schriftlich mit.

Mit freundlichen Grüßen

_______________________________
[Vorsitzende/r des Betriebsrats]
`, employer, date, trainingName, trainingName, provider, justification)
}
