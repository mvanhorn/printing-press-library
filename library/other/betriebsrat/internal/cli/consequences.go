package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type consequenceResult struct {
	Situation   string         `json:"situation"`
	Paragraph   string         `json:"paragraph"`
	Consequences []consequence `json:"consequences"`
	BROptions   []string       `json:"br_options,omitempty"`
	Note        string         `json:"note,omitempty"`
}

type consequence struct {
	Actor       string `json:"actor"`
	What        string `json:"what"`
	LegalBasis  string `json:"legal_basis"`
	Severity    string `json:"severity"` // critical, high, medium
}

func newConsequencesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "consequences [situation]",
		Short: "What happens if BR misses a deadline or employer acts without consent?",
		Long: `Explains the legal consequences of procedural failures in co-determination situations.

Situations:
  kündigung         — Employer dismisses without BR hearing, or BR misses the deadline
  einstellung       — Employer hires without BR consent
  versetzung        — Employer transfers without BR consent
  betriebsänderung  — Employer restructures without Interessenausgleich/Sozialplan
  software          — Employer introduces monitoring software without BR agreement
  br-deadline       — What happens when BR does not respond within the statutory window`,
		Example: strings.Trim(`
  betriebsrat consequences kündigung --agent
  betriebsrat consequences einstellung --agent
  betriebsrat consequences betriebsänderung --agent`, "\n"),
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

			situation := strings.ToLower(strings.TrimSpace(strings.Join(args, " ")))
			r := resolveConsequences(flags.lang, situation)
			if r == nil {
				return fmt.Errorf(tr(flags.lang,
					"unbekannte Situation %q — versuchen Sie: kündigung, einstellung, versetzung, betriebsänderung, software, br-deadline",
					"unknown situation %q — try: kündigung, einstellung, versetzung, betriebsänderung, software, br-deadline"),
					situation)
			}

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(r)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", tr(flags.lang, "Situation", "Situation"), r.Situation)
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n\n", tr(flags.lang, "Rechtsgrundlage", "Legal basis"), r.Paragraph)
			fmt.Fprintln(cmd.OutOrStdout(), tr(flags.lang, "Rechtliche Konsequenzen:", "Legal consequences:"))
			for _, c := range r.Consequences {
				fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s: %s [%s]\n", c.Severity, c.Actor, c.What, c.LegalBasis)
			}
			if len(r.BROptions) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\n"+tr(flags.lang, "Handlungsoptionen des Betriebsrats:", "Works council options:"))
				for _, o := range r.BROptions {
					fmt.Fprintf(cmd.OutOrStdout(), "  • %s\n", o)
				}
			}
			if r.Note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s: %s\n", tr(flags.lang, "Hinweis", "Note"), r.Note)
			}
			return nil
		},
	}
	return cmd
}

func resolveConsequences(lang, situation string) *consequenceResult {
	switch {
	case containsAny(situation, "kündigung", "kuendigung", "entlassung", "dismissal", "termination"):
		return &consequenceResult{
			Situation: tr(lang, "Kündigung ohne BR-Anhörung / BR verpasst Frist", "Dismissal without BR hearing / BR misses deadline"),
			Paragraph: "§ 102 BetrVG",
			Consequences: []consequence{
				{tr(lang, "Kündigung (AG-Seite)", "Dismissal (employer side)"), tr(lang, "Kündigung ohne Anhörung des BR ist UNWIRKSAM (nichtig kraft Gesetzes)", "Dismissal without BR hearing is VOID (null by operation of law)"), "§ 102 Abs. 1 Satz 3 BetrVG", "critical"},
				{tr(lang, "BR verpasst Frist (7 Tage / 3 Tage)", "BR misses deadline (7 days / 3 days)"), tr(lang, "Schweigen gilt als Zustimmung — BR verliert alle Widerspruchsrechte für diese Kündigung", "Silence counts as consent — BR loses all objection rights for this dismissal"), "§ 102 Abs. 2 Satz 2 BetrVG", "critical"},
				{tr(lang, "BR-Widerspruch erfolgt", "BR objects"), tr(lang, "Arbeitnehmer kann Weiterbeschäftigung bis Urteil verlangen; AG muss faktisch weiterbeschäftigen", "Employee can demand continued employment until judgment; employer must continue employment in practice"), "§ 102 Abs. 5 BetrVG", "high"},
				{tr(lang, "Arbeitnehmer", "Employee"), tr(lang, "Kann Kündigungsschutzklage erheben und auf Unwirksamkeit der Kündigung klagen", "Can file an unfair dismissal claim and challenge the validity of the dismissal"), "§ 4 KSchG", "high"},
			},
			BROptions: []string{
				tr(lang, "Sofort schriftlich widersprechen mit Verweis auf § 102 Abs. 3 BetrVG (innerhalb der Frist)", "Immediately object in writing citing § 102 Abs. 3 BetrVG (within the deadline)"),
				tr(lang, "Arbeitnehmer über Widerspruch und Weiterbeschäftigungsanspruch (§ 102 Abs. 5) informieren", "Inform the employee of the objection and the continued-employment right (§ 102 Abs. 5)"),
				tr(lang, "Fehlende Anhörung dokumentieren und als Einwand in etwaigem Gerichtsverfahren vorbringen", "Document the missing hearing and raise it as a defence in any court proceedings"),
			},
			Note: tr(lang, "Die Frist beginnt mit Zugang der vollständigen Anhörung. Eine unvollständige Anhörung setzt die Frist NICHT in Gang.", "The deadline starts when the BR receives the COMPLETE hearing notice. An incomplete notice does NOT start the clock."),
		}

	case containsAny(situation, "einstellung", "hiring", "recruitment"):
		return &consequenceResult{
			Situation: tr(lang, "Einstellung ohne BR-Zustimmung", "Hiring without BR consent"),
			Paragraph: "§§ 99–101 BetrVG",
			Consequences: []consequence{
				{tr(lang, "Arbeitgeber", "Employer"), tr(lang, "Einstellung ohne Zustimmung ist unzulässig — AG handelt gesetzwidrig", "Hiring without consent is unlawful — employer acts illegally"), "§ 99 Abs. 1 BetrVG", "critical"},
				{"BR", tr(lang, "Kann beim Arbeitsgericht Aufhebung der Maßnahme beantragen", "Can apply to the labour court to rescind the measure"), "§ 101 Satz 1 BetrVG", "high"},
				{tr(lang, "Arbeitsgericht", "Labour court"), tr(lang, "Kann AG verpflichten, die Einstellung aufzuheben (Zwangsgeld bis 250 € je Tag)", "Can order the employer to reverse the hire (penalty up to €250 per day)"), "§ 101 Satz 2 BetrVG", "high"},
				{tr(lang, "BR verpasst 1-Woche-Frist", "BR misses 1-week deadline"), tr(lang, "Zustimmung gilt als erteilt — Einstellung ist dann zulässig", "Consent is deemed granted — hire is then permissible"), "§ 99 Abs. 3 Satz 2 BetrVG", "critical"},
				{tr(lang, "Vorläufige Maßnahme (§ 100)", "Provisional measure (§ 100)"), tr(lang, "AG kann vorläufig einstellen bei Dringlichkeit, muss aber innerhalb 3 Tage Klage erheben", "Employer may provisionally hire in urgent cases but must apply to court within 3 days"), "§ 100 Abs. 2 BetrVG", "medium"},
			},
			BROptions: []string{
				tr(lang, "Zustimmung schriftlich innerhalb 1 Woche verweigern mit Begründung nach § 99 Abs. 2", "Refuse consent in writing within 1 week with reasons under § 99 Abs. 2"),
				tr(lang, "Klage auf Aufhebung beim Arbeitsgericht (§ 101 BetrVG)", "Apply to labour court for rescission (§ 101 BetrVG)"),
				tr(lang, "Bei vorläufiger Maßnahme: prüfen ob Arbeitsgericht binnen 3 Tagen angerufen wurde", "For provisional measures: check whether court was invoked within 3 days"),
			},
			Note: tr(lang, "Die Frist beginnt mit Vorlage ALLER erforderlichen Unterlagen. Fehlende Unterlagen = Frist läuft nicht.", "The deadline starts only when ALL required documents are submitted. Missing documents = clock does not run."),
		}

	case containsAny(situation, "versetzung", "transfer", "relocation"):
		return &consequenceResult{
			Situation: tr(lang, "Versetzung ohne BR-Zustimmung", "Transfer without BR consent"),
			Paragraph: "§§ 99–101 BetrVG",
			Consequences: []consequence{
				{tr(lang, "Arbeitgeber", "Employer"), tr(lang, "Versetzung ohne Zustimmung ist unzulässig", "Transfer without consent is unlawful"), "§ 99 Abs. 1 BetrVG", "critical"},
				{"BR", tr(lang, "Kann beim Arbeitsgericht Aufhebung beantragen", "Can apply to labour court to rescind the transfer"), "§ 101 BetrVG", "high"},
				{tr(lang, "BR verpasst Frist", "BR misses deadline"), tr(lang, "Zustimmung gilt als erteilt — Versetzung dann zulässig", "Consent deemed granted — transfer then permissible"), "§ 99 Abs. 3 Satz 2 BetrVG", "critical"},
			},
			BROptions: []string{
				tr(lang, "Schriftliche Zustimmungsverweigerung innerhalb 1 Woche mit Begründung", "Written refusal of consent within 1 week with reasons"),
				tr(lang, "Antrag auf Aufhebung beim Arbeitsgericht nach § 101 BetrVG", "Application to labour court for rescission under § 101 BetrVG"),
			},
		}

	case containsAny(situation, "betriebsänderung", "restrukturierung", "umstrukturierung", "restructuring", "reorganisation"):
		return &consequenceResult{
			Situation: tr(lang, "Betriebsänderung ohne Interessenausgleich / Sozialplan", "Operational change without Interessenausgleich / Sozialplan"),
			Paragraph: "§§ 111–113 BetrVG",
			Consequences: []consequence{
				{tr(lang, "Kein Interessenausgleich versucht", "No Interessenausgleich attempted"), tr(lang, "Arbeitnehmer haben Anspruch auf Nachteilsausgleich (Abfindung)", "Employees are entitled to Nachteilsausgleich (severance) per § 113"), "§ 113 Abs. 3 BetrVG", "critical"},
				{tr(lang, "Sozialplan nicht vereinbart", "No Sozialplan agreed"), tr(lang, "BR kann Einigungsstelle anrufen und Sozialplan erzwingen", "BR can invoke conciliation board and enforce a Sozialplan"), "§ 112 Abs. 4 BetrVG", "critical"},
				{tr(lang, "Massenentlassung ohne KSchG § 17 Verfahren", "Mass dismissal without § 17 KSchG procedure"), tr(lang, "Kündigungen sind unwirksam (BAG-Rechtsprechung)", "Dismissals are void (BAG case law)"), "§ 17 KSchG", "critical"},
				{tr(lang, "AG weicht vom vereinbarten Interessenausgleich ab", "Employer deviates from agreed Interessenausgleich"), tr(lang, "Betroffene AN haben Abfindungsanspruch nach § 113 Abs. 1", "Affected employees have a severance claim under § 113 Abs. 1"), "§ 113 Abs. 1 BetrVG", "high"},
			},
			BROptions: []string{
				tr(lang, "Vollständige Unterrichtung verlangen (§ 111 Satz 1) — schriftlich mit Frist", "Demand full written disclosure (§ 111 Satz 1) with a deadline"),
				tr(lang, "Einigungsstelle für Sozialplan beantragen (jederzeit möglich, da erzwingbar)", "Invoke the conciliation board for a Sozialplan (always available — it is enforceable)"),
				tr(lang, "Nachteilsausgleichsansprüche der betroffenen AN prüfen und geltend machen", "Assess and assert Nachteilsausgleich claims for affected employees"),
				tr(lang, "Massenentlassungsanzeige beim Arbeitsamt: Konsultationsverfahren einhalten", "Mass redundancy notification to the employment agency: follow the consultation procedure"),
			},
			Note: tr(lang, "Der Sozialplan ist erzwingbar — die Einigungsstelle entscheidet verbindlich. Der Interessenausgleich ist NICHT erzwingbar.", "The Sozialplan is enforceable — the conciliation board rules bindingly. The Interessenausgleich is NOT enforceable."),
		}

	case containsAny(situation, "software", "system", "ki ", "monitoring", "überwachung", "surveillance", "ai "):
		return &consequenceResult{
			Situation: tr(lang, "Software-/KI-Einführung ohne BR-Zustimmung (§ 87 Abs. 1 Nr. 6)", "Software / AI deployment without BR consent (§ 87 Abs. 1 Nr. 6)"),
			Paragraph: "§ 87 Abs. 1 Nr. 6 BetrVG",
			Consequences: []consequence{
				{tr(lang, "Einführung ohne BV", "Deployment without Betriebsvereinbarung"), tr(lang, "Einführung ist unzulässig — BR kann Unterlassung verlangen", "Deployment is unlawful — BR can demand cessation"), "§ 87 Abs. 1 BetrVG", "critical"},
				{"BR", tr(lang, "Kann Einigungsstelle anrufen und BV erzwingen", "Can invoke conciliation board and enforce a Betriebsvereinbarung"), "§ 76 BetrVG", "high"},
				{tr(lang, "Erhobene Mitarbeiterdaten", "Employee data collected"), tr(lang, "Ohne BV erhobene Daten sind rechtswidrig — Datenschutzverstoß möglich", "Data collected without a BV is unlawful — GDPR violation possible"), "DSGVO Art. 5 Abs. 1", "high"},
				{tr(lang, "Arbeitsgericht", "Labour court"), tr(lang, "Kann AG verpflichten, System abzuschalten bis BV vorliegt", "Can order the employer to shut down the system until a BV is in place"), "§ 23 Abs. 3 BetrVG", "high"},
			},
			BROptions: []string{
				tr(lang, "Sofortige schriftliche Einwendung und Forderung nach Einstellung des Systems", "Immediate written objection and demand to shut down the system"),
				tr(lang, "Einigungsstelle nach § 76 BetrVG anrufen", "Invoke the conciliation board under § 76 BetrVG"),
				tr(lang, "Einstweilige Verfügung beim Arbeitsgericht (dringend bei laufender Datenerhebung)", "Apply for an injunction at the labour court (urgent if data collection is ongoing)"),
				tr(lang, "Datenschutzbehörde informieren bei DSGVO-Verstoß", "Notify the data protection authority if GDPR violation is suspected"),
			},
			Note: tr(lang, "BR hat 'erzwingbare Mitbestimmung' — ohne Einigung muss die Einigungsstelle entscheiden. AG kann nicht einseitig handeln.", "The BR has 'enforceable co-determination' — without agreement the conciliation board must decide. The employer cannot act unilaterally."),
		}

	case containsAny(situation, "br-deadline", "frist verpasst", "schweigen", "keine antwort", "missed deadline", "no response"):
		return &consequenceResult{
			Situation: tr(lang, "BR gibt keine Antwort innerhalb der gesetzlichen Frist", "BR does not respond within the statutory deadline"),
			Paragraph: "§§ 99, 102 BetrVG",
			Consequences: []consequence{
				{tr(lang, "§ 102 Kündigung", "§ 102 Dismissal"), tr(lang, "Schweigen = Zustimmung zur Kündigung. Widerspruchsrecht verloren.", "Silence = consent to dismissal. Right to object is lost."), "§ 102 Abs. 2 Satz 2", "critical"},
				{tr(lang, "§ 99 Einstellung/Versetzung", "§ 99 Hiring/Transfer"), tr(lang, "Schweigen = Zustimmung. Maßnahme ist zulässig.", "Silence = consent. Measure is permissible."), "§ 99 Abs. 3 Satz 2", "critical"},
				{tr(lang, "Nachträgliche Einwände", "Subsequent objections"), tr(lang, "Nicht mehr möglich — Fristversäumnis ist nicht heilbar", "No longer possible — missing the deadline cannot be remedied"), "", "critical"},
			},
			BROptions: []string{
				tr(lang, "Frist notieren und im BR-Kalender als Pflichttermin eintragen", "Record the deadline and add it as a mandatory entry in the BR calendar"),
				tr(lang, "Auch bei Zustimmung: schriftliche Stellungnahme zur Dokumentation abgeben", "Even when consenting: submit a written statement for the record"),
				tr(lang, "Bei Unsicherheit: Frist durch Teilstellungnahme unterbrechen (um Optionen zu wahren)", "When in doubt: interrupt the deadline with a partial statement (to preserve options)"),
			},
			Note: tr(lang, "Fristen beginnen mit Zugang der vollständigen Information/Anhörung beim BR-Vorsitzenden. Unvollständige Anhörung setzt die Frist NICHT in Gang — dies ist die wichtigste Verteidigungslinie.", "Deadlines start when the COMPLETE information/hearing reaches the BR chair. An incomplete hearing does NOT start the clock — this is the most important line of defence."),
		}

	default:
		return nil
	}
}
