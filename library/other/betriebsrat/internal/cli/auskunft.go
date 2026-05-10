package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type auskunftResult struct {
	Topic       string `json:"topic"`
	Reason      string `json:"reason,omitempty"`
	Letter      string `json:"letter"`
	Deadline    string `json:"response_deadline"`
	Enforcement string `json:"enforcement_if_ignored"`
	LegalBasis  string `json:"legal_basis"`
	Note        string `json:"note"`
}

type auskunftTopic struct {
	labelDE  string
	labelEN  string
	detailDE string
	detailEN string
}

var auskunftTopics = map[string]auskunftTopic{
	"sozialdaten": {
		labelDE:  "Sozialdaten aller Arbeitnehmer",
		labelEN:  "Social data of all employees",
		detailDE: "sämtliche relevanten Sozialdaten (Geburtsdatum, Eintrittsdatum, Unterhaltspflichten, Grad der Behinderung) aller in Betracht kommenden Arbeitnehmer, insbesondere der vergleichbaren Arbeitnehmergruppen",
		detailEN: "all relevant social data (date of birth, start date, dependents, disability grade) for all potentially affected employees, particularly those in comparable groups",
	},
	"stellenplan": {
		labelDE:  "Stellenplan und Organigramm",
		labelEN:  "Headcount plan and organisational chart",
		detailDE: "den aktuellen Stellenplan, das vollständige Organigramm sowie alle geplanten personellen Veränderungen einschließlich der Besetzung offener Stellen",
		detailEN: "the current headcount plan, full organisational chart, and all planned personnel changes including open positions",
	},
	"gehaelter": {
		labelDE:  "Vergütungsstruktur und Entgeltgruppen",
		labelEN:  "Salary structure and pay grades",
		detailDE: "die vollständige Vergütungsstruktur, die angewandten Eingruppierungsgrundsätze sowie die Entgeltgruppen und Gehaltsbandbreiten aller Beschäftigten",
		detailEN: "the complete salary structure, applied grading principles, and pay bands for all employees",
	},
	"planung": {
		labelDE:  "Geplante Unternehmensmaßnahmen und Betriebsänderungen",
		labelEN:  "Planned business measures and operational changes",
		detailDE: "alle geplanten Maßnahmen, die Bestand, Organisation, Zweck oder Anlagen des Betriebs wesentlich verändern könnten, sowie den aktuellen Stand der internen Planung und Entscheidungsfindung",
		detailEN: "all planned measures that could materially change the size, organisation, purpose, or facilities of the establishment, and the current state of internal planning and decision-making",
	},
	"auswahlrichtlinien": {
		labelDE:  "Auswahlrichtlinien und Auswahlkriterien (§ 95 BetrVG)",
		labelEN:  "Selection guidelines and criteria (§ 95 BetrVG)",
		detailDE: "die geltenden Auswahlrichtlinien gemäß § 95 BetrVG sowie die konkreten Auswahlkriterien und ihre Gewichtung bei der vorliegenden Personalentscheidung",
		detailEN: "the applicable selection guidelines under § 95 BetrVG and the specific selection criteria with their weighting for the pending personnel decision",
	},
	"ki": {
		labelDE:  "Technische Dokumentation zum eingesetzten KI-/IT-System",
		labelEN:  "Technical documentation for the deployed AI/IT system",
		detailDE: "die vollständige technische Dokumentation, Verarbeitungszwecke, verarbeitete Datenkategorien, eingesetzte Algorithmen, mögliche Auswirkungen auf Arbeitnehmer sowie alle Auftragsverarbeiter des eingesetzten Systems",
		detailEN: "complete technical documentation, processing purposes, data categories, algorithms used, potential impact on employees, and all data processors of the deployed system",
	},
	"wirtschaft": {
		labelDE:  "Wirtschaftliche Lage des Unternehmens (§ 106 BetrVG)",
		labelEN:  "Economic situation of the company (§ 106 BetrVG)",
		detailDE: "die aktuelle wirtschaftliche Lage des Unternehmens, einschließlich aktueller Jahresabschluss, Umsatz- und Ertragsentwicklung sowie alle Unterlagen, die dem Wirtschaftsausschuss gemäß § 106 BetrVG vorzulegen sind",
		detailEN: "the current economic situation of the company, including the latest financial statements, revenue and earnings trend, and all documents required to be disclosed to the economic committee under § 106 BetrVG",
	},
}

func newAuskunftCmd(flags *rootFlags) *cobra.Command {
	var topic string
	var customTopic string
	var reason string
	var employer string
	var deadlineDays int
	var dateStr string

	cmd := &cobra.Command{
		Use:   "auskunft",
		Short: "Draft a formal § 80 BetrVG information request to the employer (Auskunftsverlangen)",
		Long: tr(flags.lang,
			`Generiert ein formelles Auskunftsverlangen gemäß § 80 Abs. 2 BetrVG.

Der Betriebsrat hat das Recht, alle Informationen zu erhalten, die er zur Erfüllung
seiner Aufgaben benötigt. Der Arbeitgeber muss rechtzeitig und umfassend unterrichten.
Bei Verweigerung kann der BR die Herausgabe vor dem Arbeitsgericht erzwingen.

Vordefinierte Themen (--topic):
  sozialdaten         Sozialdaten für Sozialauswahl (§ 102, § 1 Abs. 3 KSchG)
  stellenplan         Organigramm und Stellenplan
  gehaelter           Vergütungsstruktur und Eingruppierung
  planung             Geplante Betriebsänderungen (§ 111 BetrVG)
  auswahlrichtlinien  Auswahlrichtlinien (§ 95 BetrVG)
  ki                  KI-/IT-System Dokumentation (§ 87 Nr. 6 BetrVG)
  wirtschaft          Wirtschaftliche Lage (§ 106 BetrVG)
  custom              Freier Text via --custom`,
			`Generates a formal information request letter under § 80 Abs. 2 BetrVG.

The works council (Betriebsrat) has the right to receive all information needed to
fulfil its statutory duties. The employer must inform the BR promptly and fully.
If the employer refuses, the BR can compel disclosure through the labour court.

Predefined topics (--topic):
  sozialdaten         Social data for social selection (§ 102, § 1 Abs. 3 KSchG)
  stellenplan         Org chart and headcount plan
  gehaelter           Salary structure and pay grading
  planung             Planned operational changes (§ 111 BetrVG)
  auswahlrichtlinien  Selection guidelines (§ 95 BetrVG)
  ki                  AI/IT system documentation (§ 87 Nr. 6 BetrVG)
  wirtschaft          Economic situation of the company (§ 106 BetrVG)
  custom              Free text via --custom`),
		Example: strings.Trim(`
  betriebsrat auskunft --topic sozialdaten --reason "Prüfung Sozialauswahl § 102" --employer "Firma GmbH"
  betriebsrat auskunft --topic ki --reason "Einführung KI-Bewertungssystem" --deadline-days 10 --agent
  betriebsrat auskunft --topic custom --custom "Überstundenaufstellungen der letzten 12 Monate" --lang en`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if topic == "" && customTopic == "" {
				return fmt.Errorf(tr(flags.lang,
					"--topic oder --custom ist erforderlich",
					"--topic or --custom is required"))
			}

			refDate := time.Now()
			if dateStr != "" {
				parsed, err := time.Parse("2006-01-02", dateStr)
				if err != nil {
					return fmt.Errorf("invalid --date (YYYY-MM-DD): %w", err)
				}
				refDate = parsed
			}
			if deadlineDays <= 0 {
				deadlineDays = 14
			}
			dueDate := refDate.AddDate(0, 0, deadlineDays)

			employerStr := employer
			if employerStr == "" {
				employerStr = "[Arbeitgeber / Unternehmen]"
			}

			var topicLabel, topicDetail string
			if topic == "custom" || topic == "" {
				topicLabel = customTopic
				topicDetail = customTopic
			} else {
				t, ok := auskunftTopics[topic]
				if !ok {
					keys := make([]string, 0, len(auskunftTopics))
					for k := range auskunftTopics {
						keys = append(keys, k)
					}
					return fmt.Errorf(tr(flags.lang,
						"unbekanntes Thema %q. Erlaubt: %s",
						"unknown topic %q. Allowed: %s"),
						topic, strings.Join(keys, ", "))
				}
				topicLabel = tr(flags.lang, t.labelDE, t.labelEN)
				topicDetail = tr(flags.lang, t.detailDE, t.detailEN)
			}

			reasonLine := ""
			if reason != "" {
				reasonLine = fmt.Sprintf("\nAnlass: %s\n", reason)
			}

			letter := buildAuskunftLetter(employerStr, topicLabel, topicDetail, reasonLine,
				refDate.Format("02.01.2006"), dueDate.Format("02.01.2006"))

			deadlineStr := fmt.Sprintf("%s (%d %s)", dueDate.Format("02.01.2006"), deadlineDays,
				tr(flags.lang, "Tage", "days"))

			r := auskunftResult{
				Topic:  topicLabel,
				Reason: reason,
				Letter: letter,
				Deadline: deadlineStr,
				Enforcement: tr(flags.lang,
					"Bei Verweigerung: Antrag auf Herausgabe beim Arbeitsgericht (§ 80 Abs. 2 Satz 3 BetrVG); ggf. Einleitung eines Beschlussverfahrens.",
					"If refused: application to the labour court (Arbeitsgericht) for disclosure (§ 80 Abs. 2 Satz 3 BetrVG); initiate Beschlussverfahren if necessary."),
				LegalBasis: "§ 80 Abs. 2 BetrVG",
				Note: tr(flags.lang,
					"Das Auskunftsverlangen muss sich auf eine konkrete BR-Aufgabe stützen (§ 80 Abs. 1 BetrVG). Formloses Schreiben genügt; Schriftform empfohlen für Beweissicherung. Frist ist nicht gesetzlich geregelt — 2 Wochen ist üblich.",
					"The information request must be tied to a specific BR task (§ 80 Abs. 1 BetrVG). No particular form required; written form recommended for evidence purposes. No statutory deadline — 2 weeks is standard practice."),
			}

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(r)
			}

			fmt.Fprint(cmd.OutOrStdout(), letter)
			fmt.Fprintf(cmd.OutOrStdout(), "\n\n--- %s ---\n", tr(flags.lang, "Hinweise", "Notes"))
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n",
				tr(flags.lang, "Antwortfrist", "Response deadline"), r.Deadline)
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n",
				tr(flags.lang, "Bei Verweigerung", "If refused"), r.Enforcement)
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n",
				tr(flags.lang, "Rechtsgrundlage", "Legal basis"), r.LegalBasis)
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n",
				tr(flags.lang, "Hinweis", "Note"), r.Note)
			return nil
		},
	}

	cmd.Flags().StringVar(&topic, "topic", "", "Thema: sozialdaten|stellenplan|gehaelter|planung|auswahlrichtlinien|ki|wirtschaft|custom")
	cmd.Flags().StringVar(&customTopic, "custom", "", "Freies Auskunftsthema (wenn --topic custom)")
	cmd.Flags().StringVar(&reason, "reason", "", "Anlass / Begründung des Auskunftsverlangens")
	cmd.Flags().StringVar(&employer, "employer", "", "Name des Arbeitgebers")
	cmd.Flags().IntVar(&deadlineDays, "deadline-days", 14, "Antwortfrist in Tagen (Standard: 14)")
	cmd.Flags().StringVar(&dateStr, "date", "", "Datum des Schreibens (YYYY-MM-DD, Standard: heute)")
	return cmd
}

func buildAuskunftLetter(employer, topicLabel, topicDetail, reasonLine, date, due string) string {
	return fmt.Sprintf(`An die Geschäftsleitung
%s

Ort, %s

Betr.: Auskunftsverlangen des Betriebsrats gemäß § 80 Abs. 2 BetrVG
       Thema: %s

Sehr geehrte Damen und Herren,

der Betriebsrat fordert hiermit gemäß § 80 Abs. 2 Satz 1 BetrVG die vollständige und rechtzeitige Unterrichtung und Vorlage der erforderlichen Unterlagen zu folgendem Thema:
%s
Benötigte Informationen und Unterlagen:

    %s

Der Betriebsrat benötigt diese Informationen, um seine gesetzlichen Aufgaben ordnungsgemäß wahrnehmen zu können (§ 80 Abs. 1 BetrVG).

Wir bitten um vollständige Übermittlung der angeforderten Unterlagen bis spätestens

    %s.

Sollten Sie der Aufforderung bis zum genannten Termin nicht nachkommen, sieht sich der Betriebsrat gezwungen, seinen Auskunftsanspruch gerichtlich durchzusetzen (§ 80 Abs. 2 Satz 3 BetrVG i.V.m. § 2a Abs. 1 Nr. 1 ArbGG).

Mit freundlichen Grüßen

_______________________________
[Vorsitzende/r des Betriebsrats]
`, employer, date, topicLabel, reasonLine, topicDetail, due)
}
