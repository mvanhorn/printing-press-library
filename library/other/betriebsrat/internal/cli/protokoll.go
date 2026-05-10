package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type protokollResult struct {
	Topic     string           `json:"topic"`
	Date      string           `json:"date"`
	Template  string           `json:"template"`
	Sections  []protokollField `json:"required_fields"`
	QuorumNote string          `json:"quorum_note"`
	LegalBasis string          `json:"legal_basis"`
	Note      string           `json:"note"`
}

type protokollField struct {
	Field    string `json:"field"`
	Required bool   `json:"required"`
	Note     string `json:"note,omitempty"`
}

func newProtokollCmd(flags *rootFlags) *cobra.Command {
	var topic string
	var dateStr string
	var brSize int
	var location string
	var employer string

	cmd := &cobra.Command{
		Use:   "protokoll",
		Short: "Generate a BR meeting minutes template (Sitzungsprotokoll)",
		Long: `Generates a formal Betriebsrat meeting minutes template.

BR decisions are only legally valid when:
  1. The meeting was properly convened (§ 29 BetrVG)
  2. A quorum was present: mehr als die Hälfte der BR-Mitglieder (§ 33 Abs. 2 BetrVG)
  3. A resolution was passed by majority of those present (§ 33 Abs. 1 BetrVG)
  4. Minutes were recorded and signed (§ 34 BetrVG)

Missing or defective minutes can invalidate BR resolutions — including
Widersprüche, Zustimmungsverweigerungen, and BV approvals.`,
		Example: strings.Trim(`
  betriebsrat protokoll --topic "Kündigung Max Mustermann § 102" --br-size 7 --date 2026-05-15
  betriebsrat protokoll --topic "Einführung Homeoffice-BV" --br-size 11 --location "Konferenzraum 2" --agent
  betriebsrat protokoll --topic "Widerspruch Versetzung" --br-size 5 --employer "Musterfirma GmbH" --json`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			refDate := time.Now()
			if dateStr != "" {
				parsed, err := time.Parse("2006-01-02", dateStr)
				if err != nil {
					return fmt.Errorf("invalid --date format (YYYY-MM-DD): %w", err)
				}
				refDate = parsed
			}
			dateFormatted := refDate.Format("02.01.2006")

			topicStr := topic
			if topicStr == "" {
				topicStr = "[Tagesordnungspunkt / Thema der Sitzung]"
			}
			locationStr := location
			if locationStr == "" {
				locationStr = "[Ort der Sitzung]"
			}
			employerStr := employer
			if employerStr == "" {
				employerStr = "[Arbeitgeber / Unternehmen]"
			}

			quorumMin := 0
			quorumNote := "Quorum: mehr als die Hälfte der BR-Mitglieder muss anwesend sein (§ 33 Abs. 2 BetrVG)."
			if brSize > 0 {
				quorumMin = brSize/2 + 1
				quorumNote = fmt.Sprintf("Quorum: mindestens %d von %d Mitgliedern müssen anwesend sein (§ 33 Abs. 2 BetrVG).", quorumMin, brSize)
			}

			template := buildProtokollTemplate(topicStr, employerStr, locationStr, dateFormatted, brSize, quorumMin)

			r := protokollResult{
				Topic: topicStr,
				Date:  dateFormatted,
				Template: template,
				Sections: []protokollField{
					{"Datum, Uhrzeit (Beginn und Ende), Ort", true, "§ 34 BetrVG"},
					{"Anwesende BR-Mitglieder (namentlich)", true, "Für Quorumnachweis erforderlich"},
					{"Entschuldigte und unentschuldigte Abwesenheit", true, "Für Vollständigkeit des Protokolls"},
					{"Quorum-Feststellung (mehr als 50% anwesend?)", true, "§ 33 Abs. 2 BetrVG — ohne Quorum kein wirksamer Beschluss"},
					{"Tagesordnung (genehmigt oder geändert)", true, "§ 29 Abs. 2 BetrVG"},
					{"Zu jedem TOP: Sachdarstellung, Beratung, Antrag", true, "§ 34 BetrVG"},
					{"Zu jedem Beschluss: Abstimmungsergebnis (Ja/Nein/Enthaltung)", true, "§ 33 BetrVG — Mehrheit der Anwesenden"},
					{"Unterschrift Vorsitzende/r und Schriftführer/in", true, "§ 34 Abs. 1 BetrVG"},
					{"Verlesung/Genehmigung des letzten Protokolls", false, "Gute Praxis"},
					{"Nächster Sitzungstermin", false, "Organisatorisch"},
				},
				QuorumNote: quorumNote,
				LegalBasis: "§§ 29, 33, 34 BetrVG",
				Note: "Das Protokoll ist kein öffentliches Dokument — es unterliegt der Geheimhaltungspflicht (§ 79 BetrVG). " +
					"Aufbewahrung: mindestens 3 Jahre nach Ende der Amtszeit des BR.",
			}

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(r)
			}

			fmt.Fprint(cmd.OutOrStdout(), template)
			fmt.Fprintf(cmd.OutOrStdout(), "\n\n--- Hinweise ---\n")
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", quorumNote)
			fmt.Fprintf(cmd.OutOrStdout(), "Rechtsgrundlage: %s\n", r.LegalBasis)
			fmt.Fprintf(cmd.OutOrStdout(), "Datenschutz: %s\n", r.Note)
			return nil
		},
	}

	cmd.Flags().StringVar(&topic, "topic", "", "Hauptthema der Sitzung (z.B. 'Kündigung § 102', 'Homeoffice-BV')")
	cmd.Flags().StringVar(&dateStr, "date", "", "Datum der Sitzung (YYYY-MM-DD, Standard: heute)")
	cmd.Flags().IntVar(&brSize, "br-size", 0, "Anzahl BR-Mitglieder gesamt (für Quorum-Berechnung)")
	cmd.Flags().StringVar(&location, "location", "", "Ort der Sitzung")
	cmd.Flags().StringVar(&employer, "employer", "", "Name des Arbeitgebers / Unternehmens")
	return cmd
}

func buildProtokollTemplate(topic, employer, location, date string, brSize, quorumMin int) string {
	quorumLine := "Anwesend (___): ___ von ___ BR-Mitgliedern — Quorum: [✓ / ✗]"
	if brSize > 0 {
		quorumLine = fmt.Sprintf("Anwesend (___): ___ von %d BR-Mitgliedern — Quorum erfüllt ab %d [✓ / ✗]", brSize, quorumMin)
	}

	return fmt.Sprintf(`PROTOKOLL DER BETRIEBSRATSSITZUNG
%s
Arbeitgeber: %s
%s

═══════════════════════════════════════════════════════════════

SITZUNGSPROTOKOLL

Datum:     %s
Uhrzeit:   ___ Uhr bis ___ Uhr
Ort:       %s

ANWESENHEIT
%s

Anwesende BR-Mitglieder:
  1. _____________________________ (Vorsitzende/r)
  2. _____________________________
  3. _____________________________
  [weitere Mitglieder aufführen]

Entschuldigt abwesend:
  • _____________________________ (Grund: ___________)

Unentschuldigt abwesend:
  • _____________________________

Quorum festgestellt: [JA / NEIN]
→ Falls NEIN: Sitzung kann nicht stattfinden / keine wirksamen Beschlüsse möglich.

TAGESORDNUNG
Die Tagesordnung wurde mit Ladungsschreiben vom [Datum] bekannt gegeben.
Änderungen: [keine / _______________]

Genehmigte Tagesordnung:
  TOP 1:  Genehmigung des letzten Protokolls
  TOP 2:  %s
  TOP 3:  [weiterer TOP]
  TOP X:  Verschiedenes

───────────────────────────────────────────────────────────────

TOP 1 — Genehmigung des Protokolls der letzten Sitzung vom [Datum]

Das Protokoll der Sitzung vom [letztes Datum] wird [ohne Änderungen / mit folgenden Änderungen: ___] genehmigt.

Abstimmung: Ja: ___ | Nein: ___ | Enthaltung: ___
Ergebnis: [angenommen / abgelehnt]

───────────────────────────────────────────────────────────────

TOP 2 — %s

Sachdarstellung:
[Zusammenfassung des Sachverhalts / Anlass der Beratung]

Beratung:
[Wesentliche Diskussionspunkte — keine Wortwörtlichkeit, aber Kernargumente festhalten]

Antrag:
Der Betriebsrat beschließt:
„[Vollständiger Beschlusstext — möglichst präzise, z.B. 'Der Betriebsrat widerspricht der Kündigung von Herrn/Frau ___ gemäß § 102 Abs. 3 Nr. 2 BetrVG.']"

Abstimmung: Ja: ___ | Nein: ___ | Enthaltung: ___
Ergebnis: [angenommen / abgelehnt] (Mehrheit der Anwesenden erforderlich, § 33 Abs. 1 BetrVG)

───────────────────────────────────────────────────────────────

TOP X — Verschiedenes

[Mitteilungen, Informationen, nächste Sitzung]

Nächste Sitzung: [Datum, Uhrzeit, Ort]

───────────────────────────────────────────────────────────────

Sitzungsende: ___ Uhr

UNTERSCHRIFTEN

_______________________________     _______________________________
[Vorsitzende/r des Betriebsrats]    [Schriftführer/in]
Datum: %s                           Datum: %s
`, strings.Repeat("═", 63), employer, date, date, location, quorumLine, topic, topic, date, date)
}
