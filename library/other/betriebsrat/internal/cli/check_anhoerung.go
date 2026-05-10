package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/googlarz/betriebsrat/internal/betrvg"
	"github.com/spf13/cobra"
)

type anhoerungCheck struct {
	Field    string `json:"field"`
	Present  bool   `json:"present"`
	Severity string `json:"severity"` // critical, high, medium
	Note     string `json:"note"`
}

type anhoerungResult struct {
	Valid           bool             `json:"valid"`
	ClockRunning    bool             `json:"clock_running"`
	KuendigungsArt  string           `json:"kuendigungs_art,omitempty"` // ordentlich | außerordentlich | unknown
	DeadlineDays    int              `json:"deadline_days,omitempty"`
	Checks          []anhoerungCheck `json:"checks"`
	MissingCritical []string         `json:"missing_critical,omitempty"`
	Recommendation  string           `json:"recommendation"`
	LegalBasis      string           `json:"legal_basis"`
}

// requiredAnhoerungFields defines what a complete § 102 Anhörung must contain.
// Each field has detection keywords and a note explaining the legal consequence if absent.
var requiredAnhoerungFields = []struct {
	Field    string
	Keywords []string
	Severity string
	Note     string
}{
	{
		Field:    "Name des Arbeitnehmers",
		Keywords: []string{"name", "herr", "frau", "mitarbeiter"},
		Severity: "critical",
		Note:     "Ohne vollständige Identifikation ist die Anhörung nicht zuzuordnen — Frist beginnt nicht.",
	},
	{
		Field:    "Geburtsdatum oder Alter",
		Keywords: []string{"geboren", "geb.", "jahrgang", "alter", "jährig"},
		Severity: "critical",
		Note:     "Sozialdatum: Ohne Alter kann BR Sozialauswahl nicht prüfen (§ 1 Abs. 3 KSchG).",
	},
	{
		Field:    "Familienstand / Unterhaltspflichten",
		Keywords: []string{"verheiratet", "ledig", "geschieden", "kind", "kinder", "unterhalt", "familie"},
		Severity: "critical",
		Note:     "Sozialdatum: Pflichtangabe für Sozialauswahlprüfung. Fehlen → Anhörung unvollständig, Frist läuft nicht.",
	},
	{
		Field:    "Betriebszugehörigkeit / Eintrittsdatum",
		Keywords: []string{"eintritt", "seit", "beschäftigt", "betriebszugehörigkeit", "jahre", "tätig"},
		Severity: "critical",
		Note:     "Sozialdatum: Ohne Betriebszugehörigkeit keine Sozialauswahl prüfbar. Fehlen = unvollständige Anhörung.",
	},
	{
		Field:    "Schwerbehinderung / GdB",
		Keywords: []string{"schwerbehinderung", "gdb", "behinderung", "schwerbehindert", "gleichgestellt", "keine behinderung", "nicht schwerbehindert"},
		Severity: "high",
		Note:     "Muss angegeben werden — auch wenn verneint. Schwerbehinderung erfordert Zustimmung Integrationsamt (§ 168 SGB IX).",
	},
	{
		Field:    "Kündigungsart (ordentlich/außerordentlich)",
		Keywords: []string{"ordentlich", "außerordentlich", "fristlos", "fristgemäß", "frist"},
		Severity: "critical",
		Note:     "Bestimmt die Frist: 7 Tage (ordentlich) oder 3 Tage (außerordentlich). Ohne Angabe unklar welche Frist gilt.",
	},
	{
		Field:    "Kündigungsgrund",
		Keywords: []string{"grund", "ursache", "weil", "da ", "aufgrund", "betriebsbedingt", "personenbedingt", "verhaltensbedingt", "leistung", "fehlzeiten", "fehlverhalten", "umstruktur"},
		Severity: "critical",
		Note:     "Kern der Anhörung: AG muss Grund vollständig mitteilen. Unvollständiger Grund = unvollständige Anhörung (BAG Urt. v. 22.04.2010).",
	},
	{
		Field:    "Kündigungsdatum / Kündigungsfrist",
		Keywords: []string{"zum", "kündigung zum", "kündigt zum", "frist", "ablauf", "datum", "termin"},
		Severity: "high",
		Note:     "BR muss wissen, wann die Kündigung wirken soll, um Weiterbeschäftigungsoptionen zu prüfen.",
	},
	{
		Field:    "Sozialauswahl (bei betriebsbedingter Kündigung)",
		Keywords: []string{"sozialauswahl", "auswahlgruppe", "vergleichbar", "vergleichspersonen", "auswahlkriterien", "punkte", "ranking"},
		Severity: "high",
		Note:     "Bei betriebsbedingter Kündigung Pflicht. Fehlen → BR kann Sozialauswahl nicht prüfen. Widerspruchsgrund nach § 102 Abs. 3 Nr. 2.",
	},
	{
		Field:    "Abmahnungen (bei verhaltensbedingter Kündigung)",
		Keywords: []string{"abmahnung", "abgemahnt", "verwarnung", "keine abmahnung"},
		Severity: "medium",
		Note:     "Bei verhaltensbedingter Kündigung erwartet: vorangegangene Abmahnungen oder Begründung, warum keine erteilt wurden.",
	},
}

func newCheckAnhoerungCmd(flags *rootFlags) *cobra.Command {
	var kuendigungsArt string

	cmd := &cobra.Command{
		Use:   "check-anhoerung [text]",
		Short: "Check a § 102 Anhörungsschreiben for completeness",
		Long: `Scans the text of an employer's Anhörungsschreiben (§ 102 BetrVG) and
reports which required elements are present and which are missing.

A § 102 Anhörung is only valid — and the response clock only starts — when
ALL required information has been provided. Missing social data (Sozialdaten)
is the most common defect and is the BR's strongest procedural defence.

Required elements:
  1. Name und Identifikation des Arbeitnehmers
  2. Geburtsdatum / Alter
  3. Familienstand / Unterhaltspflichten / Kinder
  4. Betriebszugehörigkeit / Eintrittsdatum
  5. Schwerbehinderung (auch wenn verneint)
  6. Kündigungsart (ordentlich oder außerordentlich)
  7. Kündigungsgrund (vollständig)
  8. Kündigungsdatum / geplante Kündigungsfrist
  9. Sozialauswahl (bei betriebsbedingter Kündigung)
  10. Abmahnungen (bei verhaltensbedingter Kündigung)

Paste the full letter text as the argument. Enclose in quotes or pipe via stdin.`,
		Example: strings.Trim(`
  betriebsrat check-anhoerung "Wir hören den Betriebsrat zur beabsichtigten ordentlichen Kündigung von Herrn Max Mustermann, geb. 15.03.1980, verheiratet, 2 Kinder, seit 2015 beschäftigt, an. Grund: betriebsbedingt, Stellenabbau." --agent
  betriebsrat check-anhoerung "$(cat anhoerung.txt)" --json
  betriebsrat check-anhoerung "Sehr geehrte Damen, wir kündigen Frau Schmidt außerordentlich." --type außerordentlich`, "\n"),
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

			text := strings.Join(args, " ")
			lower := strings.ToLower(text)

			r := anhoerungResult{
				LegalBasis: "§ 102 BetrVG; BAG Urt. v. 22.04.2010 – 2 AZR 991/08",
			}

			// Detect Kündigungsart from text if not overridden by flag
			art := kuendigungsArt
			if art == "" {
				if betrvg.ContainsFold(lower, "außerordentlich") || betrvg.ContainsFold(lower, "fristlos") {
					art = "außerordentlich"
				} else if betrvg.ContainsFold(lower, "ordentlich") || betrvg.ContainsFold(lower, "fristgemäß") {
					art = "ordentlich"
				} else {
					art = "unbekannt"
				}
			}
			r.KuendigungsArt = art
			switch art {
			case "außerordentlich":
				r.DeadlineDays = 3
			case "ordentlich":
				r.DeadlineDays = 7
			}

			var missingCritical []string

			for _, field := range requiredAnhoerungFields {
				// Skip Sozialauswahl check unless text mentions betriebsbedingt
				if field.Field == "Sozialauswahl (bei betriebsbedingter Kündigung)" {
					if !betrvg.ContainsFold(lower, "betriebsbedingt") && !betrvg.ContainsFold(lower, "stellenabbau") {
						continue
					}
				}
				// Skip Abmahnungen check unless text mentions verhaltensbedingt
				if field.Field == "Abmahnungen (bei verhaltensbedingter Kündigung)" {
					if !betrvg.ContainsFold(lower, "verhaltensbedingt") && !betrvg.ContainsFold(lower, "fehlverhalten") && !betrvg.ContainsFold(lower, "pflichtverletzung") {
						continue
					}
				}

				present := false
				for _, kw := range field.Keywords {
					if betrvg.ContainsFold(lower, kw) {
						present = true
						break
					}
				}

				check := anhoerungCheck{
					Field:    field.Field,
					Present:  present,
					Severity: field.Severity,
					Note:     field.Note,
				}
				if !present && field.Severity == "critical" {
					missingCritical = append(missingCritical, field.Field)
				}
				r.Checks = append(r.Checks, check)
			}

			r.MissingCritical = missingCritical
			r.Valid = len(missingCritical) == 0
			r.ClockRunning = r.Valid

			if r.Valid {
				days := r.DeadlineDays
				dayStr := fmt.Sprintf("%d Tagen", days)
				if days == 7 {
					dayStr = "1 Woche (7 Tage)"
				}
				r.Recommendation = fmt.Sprintf(
					"Anhörung erscheint vollständig. Frist läuft: BR muss innerhalb von %s antworten. "+
						"Prüfen Sie dennoch inhaltlich, ob der Kündigungsgrund plausibel und die Sozialauswahl korrekt ist.",
					dayStr)
			} else {
				deadlineStr := fmt.Sprintf("Die %d-Tage-Frist", r.DeadlineDays)
				if r.DeadlineDays == 0 {
					deadlineStr = "Die Frist"
				}
				r.Recommendation = fmt.Sprintf(
					"Anhörung ist UNVOLLSTÄNDIG (%d kritische Felder fehlen). "+
						"%s beginnt NICHT. BR sollte sofort schriftlich auf die fehlenden Informationen hinweisen. "+
						"Ohne vollständige Anhörung ist eine Kündigung unwirksam (§ 102 Abs. 1 Satz 3 BetrVG).",
					len(missingCritical), deadlineStr)
			}

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(r)
			}

			// Human-readable output
			status := "✅ VOLLSTÄNDIG"
			if !r.Valid {
				status = "❌ UNVOLLSTÄNDIG"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "§ 102 Anhörungsprüfung — %s\n\n", status)
			fmt.Fprintf(cmd.OutOrStdout(), "Kündigungsart: %s", r.KuendigungsArt)
			if r.DeadlineDays > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), " (Frist: %d Tage)", r.DeadlineDays)
			}
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintf(cmd.OutOrStdout(), "Frist läuft: %v\n\n", r.ClockRunning)

			fmt.Fprintln(cmd.OutOrStdout(), "Prüfergebnis:")
			for _, c := range r.Checks {
				icon := "✓"
				if !c.Present {
					icon = "✗"
					if c.Severity == "critical" {
						icon = "✗ KRITISCH"
					}
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s\n", icon, c.Field)
				if !c.Present {
					fmt.Fprintf(cmd.OutOrStdout(), "         → %s\n", c.Note)
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\nEmpfehlung: %s\n", r.Recommendation)
			fmt.Fprintf(cmd.OutOrStdout(), "Rechtsgrundlage: %s\n", r.LegalBasis)
			return nil
		},
	}

	cmd.Flags().StringVar(&kuendigungsArt, "type", "", "Kündigungsart überschreiben: ordentlich | außerordentlich")
	return cmd
}
