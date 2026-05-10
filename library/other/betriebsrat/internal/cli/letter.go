package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newLetterCmd(flags *rootFlags) *cobra.Command {
	var employee string
	var employer string
	var letterType string
	var ground string
	var dateStr string
	var measure string
	var affected int

	cmd := &cobra.Command{
		Use:   "letter [situation]",
		Short: "Draft a formal BR letter (Stellungnahme, Widerspruch, Zustimmungsverweigerung)",
		Long: `Generates a draft formal letter for common Betriebsrat responses.

Situations:
  kündigung        Stellungnahme nach § 102 BetrVG
  einstellung      Zustimmung oder Verweigerung nach § 99 BetrVG
  versetzung       Zustimmung oder Verweigerung nach § 99 BetrVG
  betriebsänderung Unterrichtungsverlangen oder Interessenausgleich nach §§ 111–112 BetrVG

Types for kündigung:        zustimmung | bedenken | widerspruch
Types for einstellung/versetzung:  zustimmung | verweigerung
Types for betriebsänderung: unterrichtung | interessenausgleich`,
		Example: strings.Trim(`
  betriebsrat letter kündigung --type widerspruch --employee "Max Mustermann" --ground "fehlerhafte Sozialauswahl"
  betriebsrat letter einstellung --type verweigerung --employee "Anna Schmidt" --ground "Nachteil bestehender Mitarbeiter § 99 Abs. 2 Nr. 3"
  betriebsrat letter betriebsänderung --type unterrichtung --measure "Schließung Filiale Hamburg" --affected 45
  betriebsrat letter betriebsänderung --type interessenausgleich --measure "Verlagerung Produktion" --agent`, "\n"),
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

			situation := strings.ToLower(strings.TrimSpace(args[0]))

			refDate := time.Now()
			if dateStr != "" {
				parsed, err := time.Parse("2006-01-02", dateStr)
				if err != nil {
					return fmt.Errorf("invalid --date format (expected YYYY-MM-DD): %w", err)
				}
				refDate = parsed
			}

			type letterResult struct {
				Situation   string `json:"situation"`
				Type        string `json:"letter_type"`
				Employee    string `json:"employee,omitempty"`
				Employer    string `json:"employer,omitempty"`
				Date        string `json:"date"`
				LegalBasis  string `json:"legal_basis"`
				SubjectLine string `json:"subject_line"`
				Body        string `json:"body"`
				Note        string `json:"note,omitempty"`
			}

			employerStr := employer
			if employerStr == "" {
				employerStr = "[Arbeitgeber/Unternehmen]"
			}
			employeeStr := employee
			if employeeStr == "" {
				employeeStr = "[Mitarbeiter/in]"
			}

			r := letterResult{
				Situation: situation,
				Type:      letterType,
				Employee:  employee,
				Employer:  employer,
				Date:      refDate.Format("02.01.2006"),
			}

			switch situation {
			case "kündigung", "kuendigung":
				r.LegalBasis = "§ 102 BetrVG"
				switch letterType {
				case "zustimmung", "":
					r.SubjectLine = fmt.Sprintf("Stellungnahme nach § 102 BetrVG zur Kündigung von %s", employeeStr)
					r.Body = letterKuendigungZustimmung(employeeStr, employerStr, refDate)
				case "bedenken":
					r.SubjectLine = fmt.Sprintf("Stellungnahme nach § 102 BetrVG – Bedenken zur Kündigung von %s", employeeStr)
					r.Body = letterKuendigungBedenken(employeeStr, employerStr, refDate, ground)
				case "widerspruch":
					r.SubjectLine = fmt.Sprintf("Widerspruch nach § 102 Abs. 3 BetrVG zur Kündigung von %s", employeeStr)
					r.Body = letterKuendigungWiderspruch(employeeStr, employerStr, refDate, ground)
					r.Note = "Widerspruch begründet Weiterbeschäftigungsanspruch nach § 102 Abs. 5 BetrVG bis zur rechtskräftigen Entscheidung."
				default:
					return fmt.Errorf("unbekannter Typ %q für kündigung — erlaubt: zustimmung, bedenken, widerspruch", letterType)
				}

			case "einstellung":
				r.LegalBasis = "§ 99 BetrVG"
				switch letterType {
				case "zustimmung", "":
					r.SubjectLine = fmt.Sprintf("Zustimmung nach § 99 BetrVG zur Einstellung von %s", employeeStr)
					r.Body = letterEinstellungZustimmung(employeeStr, employerStr, refDate)
				case "verweigerung":
					r.SubjectLine = fmt.Sprintf("Zustimmungsverweigerung nach § 99 Abs. 2 BetrVG zur Einstellung von %s", employeeStr)
					r.Body = letterEinstellungVerweigerung(employeeStr, employerStr, refDate, ground)
					r.Note = "Einstellung ist ohne Zustimmung unzulässig. BR kann Aufhebung beim Arbeitsgericht beantragen (§ 101 BetrVG)."
				default:
					return fmt.Errorf("unbekannter Typ %q für einstellung — erlaubt: zustimmung, verweigerung", letterType)
				}

			case "versetzung":
				r.LegalBasis = "§ 99 BetrVG"
				switch letterType {
				case "zustimmung", "":
					r.SubjectLine = fmt.Sprintf("Zustimmung nach § 99 BetrVG zur Versetzung von %s", employeeStr)
					r.Body = letterVersetzungZustimmung(employeeStr, employerStr, refDate)
				case "verweigerung":
					r.SubjectLine = fmt.Sprintf("Zustimmungsverweigerung nach § 99 Abs. 2 BetrVG zur Versetzung von %s", employeeStr)
					r.Body = letterVersetzungVerweigerung(employeeStr, employerStr, refDate, ground)
					r.Note = "Versetzung ist ohne Zustimmung unzulässig. BR kann Aufhebung beim Arbeitsgericht beantragen (§ 101 BetrVG)."
				default:
					return fmt.Errorf("unbekannter Typ %q für versetzung — erlaubt: zustimmung, verweigerung", letterType)
				}

			case "betriebsänderung", "betriebsaenderung", "restrukturierung":
				r.LegalBasis = "§§ 111–112 BetrVG"
				measureStr := measure
				if measureStr == "" {
					measureStr = "[Beschreibung der geplanten Betriebsänderung, z.B. Schließung des Standorts X, Verlagerung der Produktion, Massenentlassung von Y Mitarbeitern]"
				}
				affectedStr := ""
				if affected > 0 {
					affectedStr = fmt.Sprintf("%d betroffene Arbeitnehmer", affected)
				} else {
					affectedStr = "[Anzahl betroffener Arbeitnehmer]"
				}
				switch letterType {
				case "unterrichtung", "":
					r.SubjectLine = fmt.Sprintf("Verlangen auf vollständige Unterrichtung nach § 111 BetrVG — %s", measureStr)
					r.Body = letterBetriebsaenderungUnterrichtung(measureStr, affectedStr, employerStr, refDate)
					r.Note = "Die Unterrichtungspflicht nach § 111 BetrVG ist vollständig und rechtzeitig zu erfüllen. Fehlende Unterlagen setzen keine Verhandlungsfristen in Gang."
				case "interessenausgleich":
					r.SubjectLine = fmt.Sprintf("Aufforderung zur Verhandlung eines Interessenausgleichs nach § 112 BetrVG — %s", measureStr)
					r.Body = letterBetriebsaenderungInteressenausgleich(measureStr, affectedStr, employerStr, refDate)
					r.Note = "Der Interessenausgleich ist nicht erzwingbar. Ein Sozialplan ist jedoch erzwingbar (§ 112 Abs. 4 BetrVG). Bei Scheitern: Einigungsstelle anrufen."
				default:
					return fmt.Errorf("unbekannter Typ %q für betriebsänderung — erlaubt: unterrichtung, interessenausgleich", letterType)
				}

			case "einigungsstelle":
				r.LegalBasis = "§ 76 BetrVG"
				topicStr := ground
				if topicStr == "" {
					topicStr = "[Streitgegenstand, z.B. Homeoffice-Regelung, Einführung Software X, Sozialplan]"
				}
				r.SubjectLine = fmt.Sprintf("Antrag auf Bildung einer Einigungsstelle — %s", topicStr)
				r.Body = letterEinigungsstelle(topicStr, employerStr, refDate)
				r.Note = "Die Einigungsstelle ist zwingend zu bilden wenn eine Partei dies beantragt (§ 76 Abs. 2 BetrVG). AG kann Antrag nicht ablehnen. Bei Uneinigkeit über Vorsitzenden entscheidet das Arbeitsgericht (§ 76 Abs. 2 Satz 2 BetrVG)."

			case "br-mitglied", "betriebsratsmitglied":
				r.LegalBasis = "§ 103 BetrVG"
				switch letterType {
				case "ablehnung", "verweigerung", "":
					r.SubjectLine = fmt.Sprintf("Verweigerung der Zustimmung nach § 103 BetrVG zur außerordentlichen Kündigung von %s", employeeStr)
					r.Body = letterBRMitgliedAblehnung(employeeStr, employerStr, refDate, ground)
					r.Note = "§ 103 BetrVG: Außerordentliche Kündigung eines BR-Mitglieds ist ohne Zustimmung des BR UNWIRKSAM. BR hat 3 Tage Reaktionsfrist. AG kann Ersetzung beim Arbeitsgericht beantragen (§ 103 Abs. 2 BetrVG)."
				case "zustimmung":
					r.SubjectLine = fmt.Sprintf("Zustimmung nach § 103 BetrVG zur außerordentlichen Kündigung von %s", employeeStr)
					r.Body = letterBRMitgliedZustimmung(employeeStr, employerStr, refDate)
					r.Note = "Mit dieser Zustimmung ist die außerordentliche Kündigung zulässig. Das Arbeitsgericht prüft die Wirksamkeit auf Klage des Mitglieds."
				default:
					return fmt.Errorf("unbekannter Typ %q für br-mitglied — erlaubt: ablehnung, zustimmung", letterType)
				}

			default:
				return fmt.Errorf("unbekannte Situation %q — erlaubt: kündigung, einstellung, versetzung, betriebsänderung, einigungsstelle, br-mitglied", situation)
			}

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(r)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Betreff: %s\n\n", r.SubjectLine)
			fmt.Fprintln(cmd.OutOrStdout(), r.Body)
			if r.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\nRechtlicher Hinweis: %s\n", r.Note)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&employee, "employee", "", "Name des betroffenen Mitarbeiters/der Mitarbeiterin")
	cmd.Flags().StringVar(&employer, "employer", "", "Name des Arbeitgebers / Unternehmens")
	cmd.Flags().StringVar(&letterType, "type", "", "Brieftyp: zustimmung | bedenken | widerspruch | verweigerung | unterrichtung | interessenausgleich")
	cmd.Flags().StringVar(&ground, "ground", "", "Begründung (für Widerspruch und Verweigerung)")
	cmd.Flags().StringVar(&dateStr, "date", "", "Referenzdatum YYYY-MM-DD (Standard: heute)")
	cmd.Flags().StringVar(&measure, "measure", "", "Beschreibung der Betriebsänderungsmaßnahme (für betriebsänderung)")
	cmd.Flags().IntVar(&affected, "affected", 0, "Anzahl betroffener Arbeitnehmer (für betriebsänderung)")

	return cmd
}

func letterKuendigungZustimmung(employee, employer string, date time.Time) string {
	return fmt.Sprintf(`An
%s

%s

Stellungnahme des Betriebsrats zur beabsichtigten Kündigung

Sehr geehrte Damen und Herren,

der Betriebsrat nimmt Bezug auf die Anhörung nach § 102 Abs. 1 BetrVG vom [Datum der Anhörung] zur beabsichtigten Kündigung von Frau/Herrn %s.

Der Betriebsrat hat die mitgeteilten Kündigungsgründe geprüft und erhebt keine Einwände gegen die beabsichtigte Kündigung.

Diese Stellungnahme ergeht ohne Präjudiz für zukünftige Fälle und entbindet den Arbeitgeber nicht von der Verpflichtung, alle gesetzlichen Anforderungen einzuhalten.

Mit freundlichen Grüßen

Der Betriebsrat
___________________________
[Vorsitzende/r]

Ort, den %s`, employer, date.Format("02.01.2006"), employee, date.Format("02.01.2006"))
}

func letterKuendigungBedenken(employee, employer string, date time.Time, ground string) string {
	groundStr := ground
	if groundStr == "" {
		groundStr = "– [Begründung einfügen, z.B.: soziale Gesichtspunkte wurden nicht ausreichend berücksichtigt / fehlende Sozialauswahl / Weiterbeschäftigung auf anderer Position möglich]"
	}
	return fmt.Sprintf(`An
%s

%s

Stellungnahme des Betriebsrats – Bedenken zur beabsichtigten Kündigung

Sehr geehrte Damen und Herren,

der Betriebsrat nimmt Bezug auf die Anhörung nach § 102 Abs. 1 BetrVG vom [Datum der Anhörung] zur beabsichtigten Kündigung von Frau/Herrn %s.

Der Betriebsrat erhebt gemäß § 102 Abs. 2 Satz 1 BetrVG BEDENKEN gegen die beabsichtigte Kündigung.

Begründung:
%s

Der Betriebsrat weist darauf hin, dass Bedenken keine Sperrwirkung entfalten. Wir empfehlen dem Arbeitgeber, die genannten Gesichtspunkte sorgfältig zu prüfen und gegebenenfalls von der Kündigung abzusehen.

Mit freundlichen Grüßen

Der Betriebsrat
___________________________
[Vorsitzende/r]

Ort, den %s`, employer, date.Format("02.01.2006"), employee, groundStr, date.Format("02.01.2006"))
}

func letterKuendigungWiderspruch(employee, employer string, date time.Time, ground string) string {
	groundStr := ground
	if groundStr == "" {
		groundStr = `[Widerspruchsgrund nach § 102 Abs. 3 BetrVG — einen oder mehrere der folgenden Gründe angeben:]

  Nr. 1 — Verstoß gegen Auswahlrichtlinien (§ 95 BetrVG)
  Nr. 2 — Nichtberücksichtigung sozialer Gesichtspunkte (Sozialauswahl, § 1 Abs. 3 KSchG)
  Nr. 3 — Weiterbeschäftigung auf einem anderen Arbeitsplatz möglich
  Nr. 4 — Weiterbeschäftigung nach Umschulung/Fortbildung möglich
  Nr. 5 — Weiterbeschäftigung zu geänderten Vertragsbedingungen mit Einverständnis des AN`
	}
	return fmt.Sprintf(`An
%s

%s

Widerspruch des Betriebsrats nach § 102 Abs. 3 BetrVG

Sehr geehrte Damen und Herren,

der Betriebsrat widerspricht der beabsichtigten Kündigung von Frau/Herrn %s, über die wir mit Schreiben vom [Datum der Anhörung] nach § 102 Abs. 1 BetrVG angehört wurden.

Widerspruchsgrund gemäß § 102 Abs. 3 BetrVG:
%s

Rechtliche Konsequenz dieses Widerspruchs:
Erhebt Frau/Herr %s Kündigungsschutzklage beim Arbeitsgericht, ist der Arbeitgeber nach § 102 Abs. 5 BetrVG verpflichtet, Frau/Herrn %s über den Ablauf der Kündigungsfrist hinaus bis zum rechtskräftigen Abschluss des Verfahrens zu den bisherigen Arbeitsbedingungen weiterzubeschäftigen.

Der Betriebsrat behält sich vor, weitere Argumente im gerichtlichen Verfahren vorzutragen.

Mit freundlichen Grüßen

Der Betriebsrat
___________________________
[Vorsitzende/r]

Ort, den %s`, employer, date.Format("02.01.2006"), employee, groundStr, employee, employee, date.Format("02.01.2006"))
}

func letterEinstellungZustimmung(employee, employer string, date time.Time) string {
	return fmt.Sprintf(`An
%s

%s

Zustimmung nach § 99 Abs. 1 BetrVG zur Einstellung

Sehr geehrte Damen und Herren,

der Betriebsrat nimmt Bezug auf die Vorlage der Unterlagen zur beabsichtigten Einstellung von Frau/Herrn %s gemäß § 99 Abs. 1 BetrVG.

Nach Prüfung der vorgelegten Unterlagen erteilt der Betriebsrat hiermit seine Zustimmung zur Einstellung.

Mit freundlichen Grüßen

Der Betriebsrat
___________________________
[Vorsitzende/r]

Ort, den %s`, employer, date.Format("02.01.2006"), employee, date.Format("02.01.2006"))
}

func letterEinstellungVerweigerung(employee, employer string, date time.Time, ground string) string {
	groundStr := ground
	if groundStr == "" {
		groundStr = `[Verweigerungsgrund nach § 99 Abs. 2 BetrVG — einen oder mehrere der folgenden Gründe angeben:]

  Nr. 1 — Verstoß gegen Gesetz, Verordnung, Unfallverhütungsvorschrift, Tarifvertrag oder BV
  Nr. 2 — Benachteiligung wegen Gewerkschaftszugehörigkeit oder Betriebsratstätigkeit
  Nr. 3 — Einstellung führt zur Kündigung oder sonstigen Benachteiligung anderer AN
  Nr. 4 — Verstoß gegen ausgeschriebene Stellenausschreibungspflicht (§ 93 BetrVG)
  Nr. 5 — Fehlende soziale Ausgewogenheit im Betrieb`
	}
	return fmt.Sprintf(`An
%s

%s

Zustimmungsverweigerung nach § 99 Abs. 2 BetrVG zur Einstellung

Sehr geehrte Damen und Herren,

der Betriebsrat nimmt Bezug auf die Vorlage der Unterlagen zur beabsichtigten Einstellung von Frau/Herrn %s gemäß § 99 Abs. 1 BetrVG.

Der Betriebsrat verweigert hiermit seine Zustimmung zur Einstellung gemäß § 99 Abs. 2 BetrVG.

Begründung:
%s

Der Betriebsrat weist darauf hin, dass die Einstellung ohne seine Zustimmung unzulässig ist. Bei einer ohne Zustimmung durchgeführten Einstellung ist der Betriebsrat berechtigt, beim Arbeitsgericht die Aufhebung der Maßnahme zu beantragen (§ 101 BetrVG).

Mit freundlichen Grüßen

Der Betriebsrat
___________________________
[Vorsitzende/r]

Ort, den %s`, employer, date.Format("02.01.2006"), employee, groundStr, date.Format("02.01.2006"))
}

func letterVersetzungZustimmung(employee, employer string, date time.Time) string {
	return fmt.Sprintf(`An
%s

%s

Zustimmung nach § 99 Abs. 1 BetrVG zur Versetzung

Sehr geehrte Damen und Herren,

der Betriebsrat nimmt Bezug auf die Vorlage der Unterlagen zur beabsichtigten Versetzung von Frau/Herrn %s gemäß § 99 Abs. 1 BetrVG.

Nach Prüfung der vorgelegten Unterlagen erteilt der Betriebsrat hiermit seine Zustimmung zur Versetzung.

Mit freundlichen Grüßen

Der Betriebsrat
___________________________
[Vorsitzende/r]

Ort, den %s`, employer, date.Format("02.01.2006"), employee, date.Format("02.01.2006"))
}

func letterVersetzungVerweigerung(employee, employer string, date time.Time, ground string) string {
	groundStr := ground
	if groundStr == "" {
		groundStr = "[Verweigerungsgrund nach § 99 Abs. 2 BetrVG angeben — gleiche Gründe wie bei Einstellung]"
	}
	return fmt.Sprintf(`An
%s

%s

Zustimmungsverweigerung nach § 99 Abs. 2 BetrVG zur Versetzung

Sehr geehrte Damen und Herren,

der Betriebsrat nimmt Bezug auf die Vorlage der Unterlagen zur beabsichtigten Versetzung von Frau/Herrn %s gemäß § 99 Abs. 1 BetrVG.

Der Betriebsrat verweigert hiermit seine Zustimmung zur Versetzung gemäß § 99 Abs. 2 BetrVG.

Begründung:
%s

Der Betriebsrat weist darauf hin, dass die Versetzung ohne seine Zustimmung unzulässig ist. Bei einer ohne Zustimmung durchgeführten Versetzung ist der Betriebsrat berechtigt, beim Arbeitsgericht die Aufhebung der Maßnahme zu beantragen (§ 101 BetrVG).

Mit freundlichen Grüßen

Der Betriebsrat
___________________________
[Vorsitzende/r]

Ort, den %s`, employer, date.Format("02.01.2006"), employee, groundStr, date.Format("02.01.2006"))
}

func letterBetriebsaenderungUnterrichtung(measure, affected, employer string, date time.Time) string {
	return fmt.Sprintf(`An
%s

%s

Verlangen auf vollständige Unterrichtung nach § 111 Satz 1 BetrVG

Sehr geehrte Damen und Herren,

der Betriebsrat wurde unterrichtet, dass das Unternehmen folgende Betriebsänderung plant:

  %s
  (ca. %s)

Der Betriebsrat fordert gemäß § 111 Satz 1 BetrVG die rechtzeitige und vollständige Unterrichtung über die geplante Maßnahme. Diese Unterrichtung muss umfassen:

  1. Genaue Beschreibung der geplanten Betriebsänderung und ihrer Auswirkungen
  2. Zeitplan für die Durchführung der Maßnahme
  3. Anzahl und Gruppen der betroffenen Arbeitnehmer
  4. Geplante Sozialmaßnahmen (Abfindungen, Transfermaßnahmen, Weiterbildung)
  5. Wirtschaftliche Gründe für die Maßnahme (§ 111 BetrVG i.V.m. §§ 106, 108 BetrVG)
  6. Alle relevanten Unterlagen und Planungsdokumente

Der Betriebsrat weist darauf hin, dass die gesetzlichen Verhandlungsfristen (Interessenausgleich, Sozialplan) erst nach vollständiger Unterrichtung beginnen können. Eine unvollständige Unterrichtung setzt keine Fristen in Gang.

Der Betriebsrat erwartet die vollständige Unterrichtung bis spätestens [Datum einfügen] und behält sich vor, bei Nichterfüllung rechtliche Schritte einzuleiten.

Mit freundlichen Grüßen

Der Betriebsrat
___________________________
[Vorsitzende/r]

Ort, den %s`, employer, date.Format("02.01.2006"), measure, affected, date.Format("02.01.2006"))
}

func letterBetriebsaenderungInteressenausgleich(measure, affected, employer string, date time.Time) string {
	return fmt.Sprintf(`An
%s

%s

Aufforderung zur Aufnahme von Verhandlungen über einen Interessenausgleich und Sozialplan nach § 112 BetrVG

Sehr geehrte Damen und Herren,

der Betriebsrat nimmt Bezug auf die angekündigte Betriebsänderung:

  %s
  (ca. %s)

Diese Maßnahme stellt eine Betriebsänderung im Sinne des § 111 BetrVG dar, die wesentliche Nachteile für die betroffenen Arbeitnehmer zur Folge haben kann.

Der Betriebsrat fordert den Arbeitgeber gemäß § 112 Abs. 1 BetrVG auf, unverzüglich Verhandlungen über einen INTERESSENAUSGLEICH und SOZIALPLAN aufzunehmen.

Zum Interessenausgleich (§ 112 Abs. 1 BetrVG):
Der Betriebsrat ist bereit, ernsthaft über Ob, Wann und Wie der Betriebsänderung zu verhandeln. Der BR erwartet vom Arbeitgeber konkrete Alternativvorschläge und ist offen für kreative Lösungen im Interesse der Belegschaft.

Zum Sozialplan (§ 112 Abs. 1 Satz 2 BetrVG):
Der Betriebsrat wird darauf bestehen, dass den betroffenen Arbeitnehmern ein angemessener Ausgleich für entstehende wirtschaftliche Nachteile gewährt wird. Hierzu zählen insbesondere:
  - Abfindungsregelungen nach sozialen Gesichtspunkten
  - Transfergesellschaft / Qualifizierungsmaßnahmen
  - Verlängerung von Kündigungsfristen
  - Outplacement-Maßnahmen

Hinweis: Kommt kein Sozialplan zustande, ist der Betriebsrat berechtigt, die Einigungsstelle anzurufen (§ 112 Abs. 4 BetrVG). Der Sozialplan ist erzwingbar; der Interessenausgleich nicht.

Weicht der Arbeitgeber ohne zwingenden Grund vom vereinbarten Interessenausgleich ab, haben betroffene Arbeitnehmer Anspruch auf Nachteilsausgleich nach § 113 BetrVG.

Der Betriebsrat schlägt vor, die Verhandlungen spätestens bis [Datum einfügen] aufzunehmen, und bittet um Terminvorschlag innerhalb der nächsten 14 Tage.

Mit freundlichen Grüßen

Der Betriebsrat
___________________________
[Vorsitzende/r]

Ort, den %s`, employer, date.Format("02.01.2006"), measure, affected, date.Format("02.01.2006"))
}

func letterEinigungsstelle(topic, employer string, date time.Time) string {
	return fmt.Sprintf(`An
%s

%s

Antrag auf Bildung einer Einigungsstelle gemäß § 76 Abs. 2 BetrVG

Sehr geehrte Damen und Herren,

zwischen dem Betriebsrat und dem Arbeitgeber besteht in folgender Angelegenheit keine Einigung:

Streitgegenstand: %s

Da eine gütliche Einigung nicht erzielt werden konnte, beantragt der Betriebsrat hiermit die Bildung einer Einigungsstelle gemäß § 76 Abs. 2 BetrVG.

Der Betriebsrat schlägt als unparteiischen Vorsitzenden vor:
  [Name des vorgeschlagenen Vorsitzenden, z.B. Richter/in am Arbeitsgericht]

Als Beisitzer benennt der Betriebsrat:
  1. _____________________________
  2. _____________________________

Der Betriebsrat bittet um Bestätigung der Einigungsstellenbildung und Mitteilung der Beisitzer seitens des Arbeitgebers innerhalb von 5 Arbeitstagen.

Hinweis: Der Arbeitgeber ist zur Einigungsstellenbildung verpflichtet (§ 76 Abs. 2 Satz 1 BetrVG). Besteht keine Einigung über den Vorsitzenden, wird dieser auf Antrag vom Arbeitsgericht bestellt (§ 76 Abs. 2 Satz 2 BetrVG). Bei erzwingbarer Mitbestimmung ersetzt der Spruch der Einigungsstelle die Einigung der Betriebsparteien.

Mit freundlichen Grüßen

Der Betriebsrat
___________________________
[Vorsitzende/r]

Ort, den %s`, employer, date.Format("02.01.2006"), topic, date.Format("02.01.2006"))
}

func letterBRMitgliedAblehnung(employee, employer string, date time.Time, ground string) string {
	groundStr := ground
	if groundStr == "" {
		groundStr = `[Begründung der Ablehnung, z.B.:]
  • Die mitgeteilten Vorwürfe rechtfertigen nach Überzeugung des BR keine außerordentliche Kündigung
  • Ein wichtiger Grund i.S.d. § 626 BGB liegt nicht vor
  • Das Mandat schützt das Mitglied vor willkürlichen Kündigungsversuchen (§ 78 BetrVG)`
	}
	return fmt.Sprintf(`An
%s

%s

Verweigerung der Zustimmung nach § 103 Abs. 1 BetrVG
zur außerordentlichen Kündigung von %s (Betriebsratsmitglied)

Sehr geehrte Damen und Herren,

der Betriebsrat nimmt Bezug auf Ihr Schreiben vom [Datum] zur beabsichtigten außerordentlichen Kündigung von Frau/Herrn %s.

Der Betriebsrat verweigert hiermit die Zustimmung nach § 103 Abs. 1 BetrVG.

Begründung:
%s

Rechtliche Folge: Die außerordentliche Kündigung ist ohne Zustimmung des BR UNWIRKSAM (§ 103 Abs. 1 BetrVG). Der Arbeitgeber kann die Ersetzung beim Arbeitsgericht beantragen (§ 103 Abs. 2 BetrVG). Die Ersetzung setzt das Vorliegen eines wichtigen Grundes nach § 626 BGB voraus.

Mit freundlichen Grüßen

Der Betriebsrat
___________________________
[Vorsitzende/r]

Ort, den %s`, employer, date.Format("02.01.2006"), employee, employee, groundStr, date.Format("02.01.2006"))
}

func letterBRMitgliedZustimmung(employee, employer string, date time.Time) string {
	return fmt.Sprintf(`An
%s

%s

Zustimmung nach § 103 Abs. 1 BetrVG
zur außerordentlichen Kündigung von %s (Betriebsratsmitglied)

Sehr geehrte Damen und Herren,

nach eingehender Beratung erteilt der Betriebsrat die Zustimmung zur außerordentlichen Kündigung von Frau/Herrn %s gemäß § 103 Abs. 1 BetrVG.

Diese Zustimmung stellt keine Bewertung der materiell-rechtlichen Wirksamkeit der Kündigung dar. Frau/Herr %s behält das Recht, die Kündigung gerichtlich prüfen zu lassen.

Mit freundlichen Grüßen

Der Betriebsrat
___________________________
[Vorsitzende/r]

Ort, den %s`, employer, date.Format("02.01.2006"), employee, employee, employee, date.Format("02.01.2006"))
}
