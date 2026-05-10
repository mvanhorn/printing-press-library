package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type widerspruchGround struct {
	Number    int    `json:"number"`
	Paragraph string `json:"paragraph"`
	Label     string `json:"label"`
	Applies   bool   `json:"applies"`
	Strength  string `json:"strength,omitempty"` // strong, medium, weak
	Reason    string `json:"reason"`
	Evidence  string `json:"evidence_needed,omitempty"`
}

type widerspruchResult struct {
	KuendigungsArt   string              `json:"kuendigungs_art"`
	Employee         string              `json:"employee,omitempty"`
	RecommendedGrounds []widerspruchGround `json:"recommended_grounds"`
	NotApplicable    []widerspruchGround `json:"not_applicable,omitempty"`
	BestGround       string              `json:"best_ground"`
	DraftText        string              `json:"draft_widerspruch_ground_text"`
	Deadline         string              `json:"deadline_reminder"`
	LegalBasis       string              `json:"legal_basis"`
	Note             string              `json:"note"`
}

func newWiderspruchCheckCmd(flags *rootFlags) *cobra.Command {
	var kuendigungsArt string
	var employee string
	var wrongSocialSelection bool
	var otherPositionExists bool
	var retrainingPossible bool
	var reducedHoursPossible bool
	var bvViolation bool
	var noWarning bool
	var seniorityIgnored bool

	cmd := &cobra.Command{
		Use:   "widerspruch-check",
		Short: "Advise on the strongest Widerspruch grounds for a § 102 Abs. 3 BetrVG Widerspruch",
		Long: `Analyses the facts of a Kündigung and recommends which Widerspruch grounds
under § 102 Abs. 3 BetrVG are available and strongest.

A Widerspruch (§ 102 Abs. 3) is not the same as Bedenken (§ 102 Abs. 2).
Only a Widerspruch triggers the right to Weiterbeschäftigung (§ 102 Abs. 5).

The five grounds (§ 102 Abs. 3 Nr. 1–5):
  Nr. 1 — Verstoß gegen Auswahlrichtlinien (§ 95 BetrVG)
  Nr. 2 — Fehlerhafte Sozialauswahl (§ 1 Abs. 3 KSchG)
  Nr. 3 — Weiterbeschäftigung auf einem anderen Arbeitsplatz möglich
  Nr. 4 — Weiterbeschäftigung nach Umschulung/Fortbildung möglich
  Nr. 5 — Weiterbeschäftigung zu geänderten Bedingungen möglich (mit Einverständnis AN)`,
		Example: strings.Trim(`
  betriebsrat widerspruch-check --type betriebsbedingt --wrong-social-selection --other-position
  betriebsrat widerspruch-check --type betriebsbedingt --seniority-ignored --employee "Max Mustermann" --agent
  betriebsrat widerspruch-check --type verhaltensbedingt --no-warning --bv-violation`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			r := analyseWiderspruchGrounds(
				kuendigungsArt, employee,
				wrongSocialSelection, otherPositionExists,
				retrainingPossible, reducedHoursPossible,
				bvViolation, noWarning, seniorityIgnored,
			)

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(r)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Widerspruch-Analyse — § 102 Abs. 3 BetrVG\n")
			fmt.Fprintf(w, "Kündigungsart: %s\n\n", r.KuendigungsArt)

			if len(r.RecommendedGrounds) == 0 {
				fmt.Fprintln(w, "Keine Widerspruchsgründe auf Basis der angegebenen Fakten identifiziert.")
				fmt.Fprintln(w, "Prüfen Sie: Gibt es andere Stellen? Wurde die Sozialauswahl korrekt durchgeführt?")
			} else {
				fmt.Fprintf(w, "Empfohlene Widerspruchsgründe (%d):\n\n", len(r.RecommendedGrounds))
				for _, g := range r.RecommendedGrounds {
					fmt.Fprintf(w, "  § 102 Abs. 3 Nr. %d — %s [%s]\n", g.Number, g.Label, g.Strength)
					fmt.Fprintf(w, "  Begründung: %s\n", g.Reason)
					if g.Evidence != "" {
						fmt.Fprintf(w, "  Beweise: %s\n", g.Evidence)
					}
					fmt.Fprintln(w)
				}
			}

			if r.BestGround != "" {
				fmt.Fprintf(w, "Stärkster Grund: %s\n\n", r.BestGround)
			}

			if r.DraftText != "" {
				fmt.Fprintf(w, "Formulierungsvorschlag Widerspruchstext:\n%s\n\n", r.DraftText)
			}

			fmt.Fprintf(w, "Frist: %s\n", r.Deadline)
			fmt.Fprintf(w, "Rechtsgrundlage: %s\n", r.LegalBasis)
			if r.Note != "" {
				fmt.Fprintf(w, "Hinweis: %s\n", r.Note)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&kuendigungsArt, "type", "betriebsbedingt", "Kündigungsart: betriebsbedingt | verhaltensbedingt | personenbedingt")
	cmd.Flags().StringVar(&employee, "employee", "", "Name des betroffenen Mitarbeiters (optional)")
	cmd.Flags().BoolVar(&wrongSocialSelection, "wrong-social-selection", false, "Sozialauswahl fehlerhaft (falsche Vergleichsgruppe oder Gewichtung)")
	cmd.Flags().BoolVar(&seniorityIgnored, "seniority-ignored", false, "Betriebszugehörigkeit / Alter / Unterhaltspflichten wurden nicht korrekt gewichtet")
	cmd.Flags().BoolVar(&otherPositionExists, "other-position", false, "Freier Arbeitsplatz im Betrieb oder Unternehmen vorhanden")
	cmd.Flags().BoolVar(&retrainingPossible, "retraining", false, "Weiterbeschäftigung nach Umschulung/Fortbildung realistisch möglich")
	cmd.Flags().BoolVar(&reducedHoursPossible, "reduced-hours", false, "Weiterbeschäftigung zu geänderten Bedingungen (z.B. Teilzeit) möglich und AN einverstanden")
	cmd.Flags().BoolVar(&bvViolation, "bv-violation", false, "Auswahlrichtlinien (§ 95 BetrVG) oder Betriebsvereinbarung wurde verletzt")
	cmd.Flags().BoolVar(&noWarning, "no-warning", false, "Bei verhaltensbedingter Kündigung: keine vorherige Abmahnung erteilt")

	return cmd
}

func analyseWiderspruchGrounds(
	kuendigungsArt, employee string,
	wrongSocialSelection, otherPositionExists bool,
	retrainingPossible, reducedHoursPossible bool,
	bvViolation, noWarning, seniorityIgnored bool,
) widerspruchResult {
	r := widerspruchResult{
		KuendigungsArt: kuendigungsArt,
		Employee:       employee,
		LegalBasis:     "§ 102 Abs. 3 Nr. 1–5 BetrVG; § 102 Abs. 5 BetrVG (Weiterbeschäftigung)",
		Deadline:       "Innerhalb der Anhörungsfrist: 7 Tage bei ordentlicher Kündigung, 3 Tage bei außerordentlicher Kündigung (§ 102 Abs. 2 BetrVG)",
	}

	employeeRef := "den Arbeitnehmer"
	if employee != "" {
		employeeRef = "Herrn/Frau " + employee
	}

	var grounds []widerspruchGround
	var notApplicable []widerspruchGround

	// Nr. 1 — BV/Auswahlrichtlinien violation
	g1 := widerspruchGround{
		Number:    1,
		Paragraph: "§ 102 Abs. 3 Nr. 1 BetrVG",
		Label:     "Verstoß gegen Auswahlrichtlinien (§ 95 BetrVG) oder Betriebsvereinbarung",
	}
	if bvViolation {
		g1.Applies = true
		g1.Strength = "strong"
		g1.Reason = "Bei der Auswahl wurden die vereinbarten Auswahlrichtlinien (§ 95 BetrVG) oder eine Betriebsvereinbarung nicht eingehalten."
		g1.Evidence = "Auswahlrichtlinien / BV besorgen, Dokumentation der Auswahlentscheidung anfordern, konkrete Abweichung benennen."
		grounds = append(grounds, g1)
	} else {
		g1.Applies = false
		g1.Reason = "Kein Hinweis auf Verletzung von Auswahlrichtlinien angegeben."
		notApplicable = append(notApplicable, g1)
	}

	// Nr. 2 — Sozialauswahl
	g2 := widerspruchGround{
		Number:    2,
		Paragraph: "§ 102 Abs. 3 Nr. 2 BetrVG i.V.m. § 1 Abs. 3 KSchG",
		Label:     "Fehlerhafte Sozialauswahl",
	}
	if wrongSocialSelection || seniorityIgnored {
		g2.Applies = true
		g2.Strength = "strong"
		reasons := []string{}
		evidence := []string{"Sozialdaten aller Vergleichspersonen anfordern (§ 102 Abs. 1 Satz 2 BetrVG — AG muss diese mitteilen)."}
		if wrongSocialSelection {
			reasons = append(reasons, "Die Vergleichsgruppe für die Sozialauswahl wurde falsch gebildet")
			evidence = append(evidence, "Konkret benennen: Welche vergleichbaren Mitarbeiter hätten vorrangig gekündigt werden müssen und warum.")
		}
		if seniorityIgnored {
			reasons = append(reasons, "Die gesetzlichen Sozialkriterien (Betriebszugehörigkeit, Alter, Unterhaltspflichten, Schwerbehinderung) wurden nicht korrekt gewichtet")
			evidence = append(evidence, "Punktetabelle aufstellen: betroffener AN vs. verbleibende vergleichbare AN.")
		}
		g2.Reason = strings.Join(reasons, "; ")
		g2.Evidence = strings.Join(evidence, " ")
		grounds = append(grounds, g2)
	} else {
		g2.Applies = false
		g2.Reason = "Kein Hinweis auf fehlerhafte Sozialauswahl angegeben."
		notApplicable = append(notApplicable, g2)
	}

	// Nr. 3 — Other position available
	g3 := widerspruchGround{
		Number:    3,
		Paragraph: "§ 102 Abs. 3 Nr. 3 BetrVG",
		Label:     "Weiterbeschäftigung auf einem anderen Arbeitsplatz möglich",
	}
	if otherPositionExists {
		g3.Applies = true
		g3.Strength = "strong"
		g3.Reason = "Im Betrieb oder Unternehmen ist eine Weiterbeschäftigung auf einem anderen (ggf. auch geringerwertigen) Arbeitsplatz möglich."
		g3.Evidence = "Freie Stellen (auch konzernweit, § 1 Abs. 2 Satz 2 KSchG) benennen. AG muss erklären, warum keine Weiterbeschäftigung möglich."
		grounds = append(grounds, g3)
	} else {
		g3.Applies = false
		g3.Reason = "Kein freier Arbeitsplatz im Betrieb/Unternehmen angegeben."
		notApplicable = append(notApplicable, g3)
	}

	// Nr. 4 — Retraining
	g4 := widerspruchGround{
		Number:    4,
		Paragraph: "§ 102 Abs. 3 Nr. 4 BetrVG",
		Label:     "Weiterbeschäftigung nach Umschulung oder Fortbildung möglich",
	}
	if retrainingPossible {
		g4.Applies = true
		g4.Strength = "medium"
		g4.Reason = "Eine Weiterbeschäftigung wäre nach zumutbarer Umschulung oder Fortbildung möglich."
		g4.Evidence = "Konkrete Weiterbildungsmaßnahme benennen (Dauer, Kosten, erreichbare Position). AG muss Zumutbarkeit widerlegen."
		grounds = append(grounds, g4)
	} else {
		g4.Applies = false
		g4.Reason = "Keine Umschulung/Fortbildungsoption angegeben."
		notApplicable = append(notApplicable, g4)
	}

	// Nr. 5 — Changed terms
	g5 := widerspruchGround{
		Number:    5,
		Paragraph: "§ 102 Abs. 3 Nr. 5 BetrVG",
		Label:     "Weiterbeschäftigung zu geänderten Bedingungen mit Einverständnis des Arbeitnehmers",
	}
	if reducedHoursPossible {
		g5.Applies = true
		g5.Strength = "medium"
		g5.Reason = "Eine Weiterbeschäftigung zu geänderten Vertragsbedingungen (z.B. Teilzeit, andere Tätigkeit, anderes Entgelt) ist möglich und der Arbeitnehmer ist einverstanden."
		g5.Evidence = "Schriftliche Erklärung des AN einholen, dass er mit geänderten Bedingungen einverstanden ist. Konkrete Bedingungen benennen."
		grounds = append(grounds, g5)
	} else {
		g5.Applies = false
		g5.Reason = "Keine Weiterbeschäftigung zu geänderten Bedingungen angegeben."
		notApplicable = append(notApplicable, g5)
	}

	// Additional context for verhaltensbedingt without warning
	if kuendigungsArt == "verhaltensbedingt" && noWarning {
		r.Note = "WICHTIG: Bei verhaltensbedingter Kündigung ohne vorherige Abmahnung ist die Kündigung oft bereits materiell-rechtlich unwirksam (BAG-Rspr.). " +
			"Prüfen Sie dennoch die § 102 Abs. 3-Widerspruchsgründe (Nr. 1–5 gelten für jede ordentliche Kündigung, auch verhaltensbedingte — z.B. Nr. 3 wenn ein anderer Arbeitsplatz vorhanden ist). " +
			"Der Arbeitnehmer sollte zudem Kündigungsschutzklage erheben (§ 4 KSchG, 3 Wochen Frist!). " +
			"Ein Widerspruch gibt ihm das Recht auf Weiterbeschäftigung während des Verfahrens (§ 102 Abs. 5)."
	}

	r.RecommendedGrounds = grounds
	r.NotApplicable = notApplicable

	// Build best ground summary and draft text
	if len(grounds) > 0 {
		// Rank: strong > medium > weak
		var best *widerspruchGround
		for i := range grounds {
			if best == nil || grounds[i].Strength == "strong" {
				best = &grounds[i]
			}
		}
		r.BestGround = fmt.Sprintf("§ 102 Abs. 3 Nr. %d — %s", best.Number, best.Label)

		// Build draft ground text for all applicable grounds
		var draftParts []string
		for _, g := range grounds {
			draftParts = append(draftParts, fmt.Sprintf("Nr. %d (%s): %s", g.Number, g.Paragraph, g.Reason))
		}
		r.DraftText = fmt.Sprintf(
			"Der Betriebsrat widerspricht der beabsichtigten Kündigung von %s gemäß § 102 Abs. 3 BetrVG aus folgendem/folgenden Grund/Gründen:\n\n%s\n\nDer Betriebsrat behält sich vor, weitere Gründe im Rahmen eines etwaigen Kündigungsschutzverfahrens vorzutragen.",
			employeeRef, strings.Join(draftParts, "\n\n"))
	} else {
		r.DraftText = ""
		r.BestGround = "Keine Widerspruchsgründe nach § 102 Abs. 3 BetrVG identifiziert. Ggf. nur Bedenken nach § 102 Abs. 2 möglich."
	}

	return r
}
