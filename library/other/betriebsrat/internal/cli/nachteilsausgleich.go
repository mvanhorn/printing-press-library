package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/spf13/cobra"
)

type nachteilsausgleichResult struct {
	Input            nachteilsausgleichInput `json:"input"`
	EstimatedClaim   float64                 `json:"estimated_claim_eur"`
	MaxStatutoryCap  float64                 `json:"max_statutory_cap_eur"`
	FormulaUsed      string                  `json:"formula_used"`
	ConditionsMet    []string                `json:"conditions_for_claim"`
	ConditionsMiss   []string                `json:"conditions_missing,omitempty"`
	EvidenceNeeded   []string                `json:"evidence_needed"`
	SozialplanNote   string                  `json:"sozialplan_offset_note"`
	LegalBasis       string                  `json:"legal_basis"`
	Note             string                  `json:"note"`
}

type nachteilsausgleichInput struct {
	MonthlySalary     float64 `json:"monthly_salary_eur"`
	YearsService      float64 `json:"years_service"`
	Age               int     `json:"age,omitempty"`
	Measure           string  `json:"betriebsaenderung_measure"`
	IAAttempted       bool    `json:"interessenausgleich_attempted"`
	IADeviated        bool    `json:"ia_deviated_from"`
	Factor            float64 `json:"factor"`
}

func newNachteilsausgleichCmd(flags *rootFlags) *cobra.Command {
	var salary float64
	var years float64
	var age int
	var measure string
	var iaAttempted bool
	var iaDeviated bool
	var factor float64

	cmd := &cobra.Command{
		Use:   "nachteilsausgleich",
		Short: tr(flags.lang,
			"Berechnet den Nachteilsausgleichsanspruch nach § 113 BetrVG (Betriebsänderung ohne Interessenausgleich)",
			"Calculate the § 113 BetrVG disadvantage compensation claim (Betriebsänderung without Interessenausgleich)"),
		Long: tr(flags.lang,
			`Schätzt den individuellen Nachteilsausgleichsanspruch nach § 113 Abs. 3 BetrVG.

§ 113 BetrVG greift, wenn der Arbeitgeber eine Betriebsänderung (§ 111 BetrVG) durchführt,
ohne mit dem Betriebsrat einen Interessenausgleich versucht zu haben, oder wenn er
von einem abgeschlossenen Interessenausgleich abweicht.

Der Nachteilsausgleich ist ein INDIVIDUELLER Anspruch jedes betroffenen Arbeitnehmers
gegen den Arbeitgeber — unabhängig davon, ob ein Sozialplan besteht. Ein Sozialplan
kann den Nachteilsausgleich aber teilweise oder vollständig abdecken (§ 113 Abs. 3 Hs. 2).

Anspruchsvoraussetzungen (§ 113 Abs. 3 BetrVG):
  1. Betriebsänderung im Sinne des § 111 BetrVG
  2. Kein Versuch eines Interessenausgleichs ODER Abweichung vom Interessenausgleich
  3. Wirtschaftlicher Nachteil für den Arbeitnehmer

Berechnung: analog § 10 KSchG (Abfindungsformel); Deckelung: 12 Monatsgehälter.`,
			`Estimates the individual disadvantage compensation claim under § 113 Abs. 3 BetrVG.

§ 113 BetrVG applies when the employer implements a Betriebsänderung (§ 111 BetrVG)
without attempting to reach an Interessenausgleich with the Betriebsrat, or when
the employer deviates from an agreed Interessenausgleich.

Nachteilsausgleich is an INDIVIDUAL claim by each affected employee against the employer
— independent of whether a Sozialplan exists. A Sozialplan can however partially or fully
offset the Nachteilsausgleich (§ 113 Abs. 3 Hs. 2).

Prerequisites for a claim (§ 113 Abs. 3 BetrVG):
  1. A Betriebsänderung under § 111 BetrVG
  2. No attempt at Interessenausgleich OR deviation from agreed Interessenausgleich
  3. Economic disadvantage suffered by the employee

Calculation: by analogy to § 10 KSchG (severance formula); statutory cap: 12 monthly salaries.`),
		Example: strings.Trim(`
  betriebsrat nachteilsausgleich --salary 4500 --years 8 --age 42 --measure "Standortschließung" --no-ia-attempted
  betriebsrat nachteilsausgleich --salary 6000 --years 15 --age 55 --measure "Verlagerung ins Ausland" --ia-deviated --factor 0.8 --agent
  betriebsrat nachteilsausgleich --salary 3800 --years 5 --age 38 --measure "Massenentlassung § 17 KSchG" --no-ia-attempted --lang en`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if salary <= 0 {
				return fmt.Errorf(tr(flags.lang,
					"--salary muss größer als 0 sein",
					"--salary must be greater than 0"))
			}
			if years < 0 {
				return fmt.Errorf(tr(flags.lang,
					"--years darf nicht negativ sein",
					"--years must not be negative"))
			}
			if !iaAttempted && !iaDeviated {
				return fmt.Errorf(tr(flags.lang,
					"Geben Sie an, ob kein Interessenausgleich versucht wurde (--no-ia-attempted) oder vom Interessenausgleich abgewichen wurde (--ia-deviated)",
					"Specify whether no Interessenausgleich was attempted (--no-ia-attempted) or the employer deviated from the agreed Interessenausgleich (--ia-deviated)"))
			}
			if factor <= 0 {
				factor = 0.5
			}
			if measure == "" {
				measure = tr(flags.lang, "[Betriebsänderung]", "[Betriebsänderung]")
			}

			r := calcNachteilsausgleich(flags.lang, salary, years, age, measure, iaAttempted, iaDeviated, factor)

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(r)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Nachteilsausgleich § 113 BetrVG\n")
			fmt.Fprintf(w, "%s\n\n", strings.Repeat("═", 50))
			fmt.Fprintf(w, "%s: %s\n",
				tr(flags.lang, "Maßnahme", "Measure"), r.Input.Measure)
			fmt.Fprintf(w, "%s: %.2f €\n",
				tr(flags.lang, "Monatsgehalt", "Monthly salary"), r.Input.MonthlySalary)
			fmt.Fprintf(w, "%s: %.1f %s\n",
				tr(flags.lang, "Betriebszugehörigkeit", "Years of service"),
				r.Input.YearsService,
				tr(flags.lang, "Jahre", "years"))
			fmt.Fprintf(w, "%s: %s\n\n",
				tr(flags.lang, "Berechnung", "Calculation"), r.FormulaUsed)

			fmt.Fprintf(w, "  ► %s: %.2f €\n",
				tr(flags.lang, "Geschätzter Anspruch", "Estimated claim"), r.EstimatedClaim)
			fmt.Fprintf(w, "  ► %s: %.2f €\n\n",
				tr(flags.lang, "Gesetzliche Höchstgrenze (12 × Gehalt)", "Statutory cap (12 × salary)"), r.MaxStatutoryCap)

			fmt.Fprintf(w, "%s:\n", tr(flags.lang, "Anspruchsvoraussetzungen (erfüllt)", "Prerequisites (met)"))
			for _, c := range r.ConditionsMet {
				fmt.Fprintf(w, "  ✓ %s\n", c)
			}
			if len(r.ConditionsMiss) > 0 {
				fmt.Fprintf(w, "%s:\n", tr(flags.lang, "Noch zu prüfen / fehlend", "Still to verify / missing"))
				for _, c := range r.ConditionsMiss {
					fmt.Fprintf(w, "  ✗ %s\n", c)
				}
			}
			fmt.Fprintln(w)
			fmt.Fprintf(w, "%s:\n", tr(flags.lang, "Beweismittel", "Evidence needed"))
			for _, e := range r.EvidenceNeeded {
				fmt.Fprintf(w, "  • %s\n", e)
			}
			fmt.Fprintf(w, "\n%s: %s\n",
				tr(flags.lang, "Sozialplan-Anrechnung", "Sozialplan offset"), r.SozialplanNote)
			fmt.Fprintf(w, "%s: %s\n",
				tr(flags.lang, "Rechtsgrundlage", "Legal basis"), r.LegalBasis)
			fmt.Fprintf(w, "%s: %s\n",
				tr(flags.lang, "Hinweis", "Note"), r.Note)
			fmt.Fprintf(w, "\n⚠️  %s\n", tr(flags.lang,
				"SCHÄTZUNG. Der tatsächliche Anspruch hängt von den konkreten Umständen ab. Konsultieren Sie einen Fachanwalt für Arbeitsrecht.",
				"ESTIMATE ONLY. The actual claim depends on the specific circumstances. Consult a labour law specialist."))
			return nil
		},
	}

	cmd.Flags().Float64Var(&salary, "salary", 0,
		tr(flags.lang, "Bruttomonatsgehalt in Euro", "Gross monthly salary in EUR"))
	cmd.Flags().Float64Var(&years, "years", 0,
		tr(flags.lang, "Betriebszugehörigkeit in Jahren", "Years of service"))
	cmd.Flags().IntVar(&age, "age", 0,
		tr(flags.lang, "Alter des Arbeitnehmers", "Employee age"))
	cmd.Flags().StringVar(&measure, "measure", "",
		tr(flags.lang, "Art der Betriebsänderung (z.B. 'Standortschließung', 'Verlagerung')",
			"Type of Betriebsänderung (e.g. 'site closure', 'relocation')"))
	cmd.Flags().BoolVar(&iaAttempted, "no-ia-attempted", false,
		tr(flags.lang,
			"Kein Interessenausgleich versucht (§ 113 Abs. 3 Alt. 1)",
			"No Interessenausgleich was attempted (§ 113 Abs. 3 Alt. 1)"))
	cmd.Flags().BoolVar(&iaDeviated, "ia-deviated", false,
		tr(flags.lang,
			"Arbeitgeber ist vom abgeschlossenen Interessenausgleich abgewichen (§ 113 Abs. 3 Alt. 2)",
			"Employer deviated from the agreed Interessenausgleich (§ 113 Abs. 3 Alt. 2)"))
	cmd.Flags().Float64Var(&factor, "factor", 0.5,
		tr(flags.lang,
			"Berechnungsfaktor (Standard: 0.5; üblicher Bereich: 0.5–1.5)",
			"Calculation factor (default: 0.5; typical range: 0.5–1.5)"))
	_ = cmd.MarkFlagRequired("salary")
	_ = cmd.MarkFlagRequired("years")
	return cmd
}

func calcNachteilsausgleich(lang string, salary, years float64, age int,
	measure string, iaAttempted, iaDeviated bool, factor float64) nachteilsausgleichResult {

	base := math.Round(years*salary*factor*100) / 100
	maxCap := math.Round(12*salary*100) / 100
	estimated := base
	if estimated > maxCap {
		estimated = maxCap
	}

	formulaStr := fmt.Sprintf(tr(lang,
		"%.1f Jahre × %.2f € × Faktor %.2f = %.2f € (Deckelung: 12 × %.2f € = %.2f €)",
		"%.1f years × %.2f EUR × factor %.2f = %.2f EUR (cap: 12 × %.2f EUR = %.2f EUR)"),
		years, salary, factor, base, salary, maxCap)

	var condsMet []string
	var condsMiss []string

	condsMet = append(condsMet, tr(lang,
		"Betriebsänderung i.S.v. § 111 BetrVG liegt vor: "+measure,
		"Betriebsänderung under § 111 BetrVG is present: "+measure))

	if iaAttempted {
		condsMet = append(condsMet, tr(lang,
			"Kein Interessenausgleich versucht (§ 113 Abs. 3 Alt. 1 BetrVG)",
			"No Interessenausgleich attempted (§ 113 Abs. 3 Alt. 1 BetrVG)"))
	}
	if iaDeviated {
		condsMet = append(condsMet, tr(lang,
			"Abweichung vom abgeschlossenen Interessenausgleich (§ 113 Abs. 3 Alt. 2 BetrVG)",
			"Deviation from the agreed Interessenausgleich (§ 113 Abs. 3 Alt. 2 BetrVG)"))
	}

	condsMiss = append(condsMiss, tr(lang,
		"Wirtschaftlicher Nachteil des Arbeitnehmers muss konkret nachgewiesen werden (Arbeitsplatzverlust, Einkommensverlust, verlängerte Arbeitslosigkeit)",
		"Economic disadvantage of the employee must be specifically evidenced (job loss, income loss, prolonged unemployment)"))

	evidence := []string{
		tr(lang,
			"Dokumentation, dass kein Interessenausgleich versucht wurde: Protokolle aller Gespräche (oder Fehlen von Gesprächen)",
			"Documentation that no Interessenausgleich was attempted: minutes of all meetings (or absence thereof)"),
		tr(lang,
			"Nachweis der Betriebsänderung: Unternehmensankündigung, BR-Unterrichtung nach § 111 BetrVG",
			"Evidence of Betriebsänderung: company announcement, BR notification under § 111 BetrVG"),
		tr(lang,
			"Wirtschaftlicher Nachteil: Kündigung oder Aufhebungsvertrag, Gehaltsvergleich vor/nach, Dauer der Arbeitslosigkeit",
			"Economic disadvantage: termination or settlement agreement, salary comparison before/after, duration of unemployment"),
		tr(lang,
			"Bei Abweichung vom IA: Kopie des abgeschlossenen Interessenausgleichs + Dokumentation der Abweichung",
			"For IA deviation: copy of the agreed Interessenausgleich + documentation of the deviation"),
	}

	sozialplanNote := tr(lang,
		"Ein bestehender Sozialplan wird auf den Nachteilsausgleich angerechnet (§ 113 Abs. 3 Hs. 2 BetrVG). "+
			"Die Abfindung aus dem Sozialplan muss vom errechneten Nachteilsausgleich abgezogen werden. "+
			"Nur ein verbleibender Überschuss ist zusätzlich einklagbar.",
		"An existing Sozialplan is offset against the Nachteilsausgleich claim (§ 113 Abs. 3 Hs. 2 BetrVG). "+
			"The severance paid under the Sozialplan must be deducted from the calculated Nachteilsausgleich. "+
			"Only any remaining surplus is additionally claimable.")

	note := tr(lang,
		"Klagefrist: 3 Monate ab Beendigung des Arbeitsverhältnisses empfohlen (Verjährung nach § 195 BGB: 3 Jahre). "+
			"Der Anspruch entsteht mit der tatsächlichen Durchführung der Betriebsänderung — nicht mit der Ankündigung. "+
			"Klagen beim zuständigen Arbeitsgericht im Urteilsverfahren.",
		"Limitation: 3 months from end of employment recommended; general limitation period 3 years (§ 195 BGB). "+
			"The claim arises when the Betriebsänderung is actually implemented — not when announced. "+
			"File claim at the competent labour court (Arbeitsgericht) in ordinary proceedings (Urteilsverfahren).")

	return nachteilsausgleichResult{
		Input: nachteilsausgleichInput{
			MonthlySalary: salary,
			YearsService:  years,
			Age:           age,
			Measure:       measure,
			IAAttempted:   iaAttempted,
			IADeviated:    iaDeviated,
			Factor:        factor,
		},
		EstimatedClaim:  estimated,
		MaxStatutoryCap: maxCap,
		FormulaUsed:     formulaStr,
		ConditionsMet:   condsMet,
		ConditionsMiss:  condsMiss,
		EvidenceNeeded:  evidence,
		SozialplanNote:  sozialplanNote,
		LegalBasis:      "§ 113 Abs. 3 BetrVG i.V.m. § 10 KSchG; § 111 BetrVG",
		Note:            note,
	}
}
