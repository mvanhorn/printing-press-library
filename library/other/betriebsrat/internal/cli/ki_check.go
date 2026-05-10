package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type kiCheckResult struct {
	SystemDescription     string       `json:"system_description"`
	CoDetTriggered        bool         `json:"co_determination_triggered"`
	Paragraph             string       `json:"paragraph"`
	RightType             string       `json:"right_type"`
	RiskLevel             string       `json:"risk_level"`
	Triggers              []string     `json:"triggers"`
	BVRequiredClauses     []kiClause   `json:"bv_required_clauses"`
	EmployerMustNot       []string     `json:"employer_must_not_without_bv"`
	RelevantCaseLaw       []kiCaseRef  `json:"relevant_case_law"`
	Recommendation        string       `json:"recommendation"`
	LegalBasis            string       `json:"legal_basis"`
	Note                  string       `json:"note"`
}

type kiClause struct {
	Clause      string `json:"clause"`
	Explanation string `json:"explanation"`
}

type kiCaseRef struct {
	Citation string `json:"citation"`
	Holding  string `json:"holding"`
}

func newKICheckCmd(flags *rootFlags) *cobra.Command {
	var system string
	var purpose string
	var dataCollected string
	var monitorsPerformance bool
	var monitorsLocation bool
	var monitorsComms bool
	var influencesHR bool
	var biometric bool
	var autoDecision bool

	cmd := &cobra.Command{
		Use:   "ki-check",
		Short: tr(flags.lang,
			"Prüft ob ein KI-/IT-System die Mitbestimmung nach § 87 Abs. 1 Nr. 6 BetrVG auslöst",
			"Check whether an AI/IT system triggers co-determination rights under § 87 Abs. 1 Nr. 6 BetrVG"),
		Long: tr(flags.lang,
			`Analysiert, ob ein beschriebenes KI- oder IT-System die erzwingbare Mitbestimmung
des Betriebsrats nach § 87 Abs. 1 Nr. 6 BetrVG auslöst.

§ 87 Abs. 1 Nr. 6 BetrVG gilt für technische Einrichtungen, die dazu BESTIMMT sind,
das Verhalten oder die Leistung der Arbeitnehmer zu überwachen. Die Fähigkeit zur
Überwachung genügt — tatsächliche Nutzung ist nicht erforderlich (BAG-Rspr.).

Typische Auslöser: KI-Scoring, Dashboards mit Mitarbeiterdaten, Zeiterfassung,
Leistungs-Tracking, algorithmisches Management, GenAI mit Nutzerdaten, Teams/Slack-Analytik.`,
			`Analyses whether a described AI or IT system triggers the enforceable co-determination
right of the works council (Betriebsrat) under § 87 Abs. 1 Nr. 6 BetrVG.

§ 87 Abs. 1 Nr. 6 BetrVG applies to technical equipment DESIGNED to monitor
employee behaviour or performance. The capability to monitor is sufficient —
actual use is not required (established BAG case law).

Typical triggers: AI scoring, employee dashboards, time tracking, performance tracking,
algorithmic management, GenAI processing employee data, Teams/Slack analytics.`),
		Example: strings.Trim(`
  betriebsrat ki-check --system "KI-Tool das Salesforce-Aktivitäten auswertet und Mitarbeiter bewertet" --monitors-performance
  betriebsrat ki-check --system "GitHub Copilot" --purpose "Code completion" --data "keystrokes,accepted suggestions" --agent
  betriebsrat ki-check --system "Workday People Analytics" --monitors-performance --influences-hr --auto-decision --lang en`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if system == "" {
				return fmt.Errorf(tr(flags.lang,
					"--system Beschreibung des Systems ist erforderlich",
					"--system description of the system is required"))
			}

			r := analyseKISystem(flags.lang, system, purpose, dataCollected,
				monitorsPerformance, monitorsLocation, monitorsComms,
				influencesHR, biometric, autoDecision)

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(r)
			}

			w := cmd.OutOrStdout()
			triggered := tr(flags.lang, "JA — erzwingbare Mitbestimmung", "YES — enforceable co-determination right")
			if !r.CoDetTriggered {
				triggered = tr(flags.lang,
					"WAHRSCHEINLICH NICHT — kein klarer § 87 Nr. 6 Tatbestand erkennbar",
					"PROBABLY NOT — no clear § 87 Nr. 6 trigger identified")
			}

			fmt.Fprintf(w, tr(flags.lang,
				"KI/IT-System Mitbestimmungsprüfung — § 87 Abs. 1 Nr. 6 BetrVG\n",
				"AI/IT System Co-Determination Check — § 87 Abs. 1 Nr. 6 BetrVG\n"))
			fmt.Fprintf(w, "%s: %s\n\n",
				tr(flags.lang, "System", "System"), r.SystemDescription)
			fmt.Fprintf(w, "%s: %s\n",
				tr(flags.lang, "Mitbestimmung ausgelöst", "Co-determination triggered"), triggered)
			fmt.Fprintf(w, "%s: %s\n\n",
				tr(flags.lang, "Risikostufe", "Risk level"), r.RiskLevel)

			if len(r.Triggers) > 0 {
				fmt.Fprintf(w, "%s:\n", tr(flags.lang, "Auslöser", "Triggers"))
				for _, t := range r.Triggers {
					fmt.Fprintf(w, "  • %s\n", t)
				}
				fmt.Fprintln(w)
			}

			if len(r.BVRequiredClauses) > 0 {
				fmt.Fprintf(w, "%s:\n",
					tr(flags.lang, "Betriebsvereinbarung muss regeln", "The BV (works agreement) must cover"))
				for _, c := range r.BVRequiredClauses {
					fmt.Fprintf(w, "  [%s]\n", c.Clause)
					fmt.Fprintf(w, "   → %s\n", c.Explanation)
				}
				fmt.Fprintln(w)
			}

			if len(r.EmployerMustNot) > 0 {
				fmt.Fprintf(w, "%s:\n",
					tr(flags.lang, "Ohne Betriebsvereinbarung darf der AG nicht", "Without a BV the employer must not"))
				for _, s := range r.EmployerMustNot {
					fmt.Fprintf(w, "  ✗ %s\n", s)
				}
				fmt.Fprintln(w)
			}

			fmt.Fprintf(w, "%s:\n  %s\n\n",
				tr(flags.lang, "Empfehlung", "Recommendation"), r.Recommendation)

			if len(r.RelevantCaseLaw) > 0 {
				fmt.Fprintf(w, "%s:\n", tr(flags.lang, "Relevante Rechtsprechung", "Relevant case law"))
				for _, c := range r.RelevantCaseLaw {
					fmt.Fprintf(w, "  %s\n  → %s\n", c.Citation, c.Holding)
				}
				fmt.Fprintln(w)
			}

			fmt.Fprintf(w, "%s: %s\n", tr(flags.lang, "Rechtsgrundlage", "Legal basis"), r.LegalBasis)
			fmt.Fprintf(w, "%s: %s\n", tr(flags.lang, "Hinweis", "Note"), r.Note)
			return nil
		},
	}

	cmd.Flags().StringVar(&system, "system", "", tr(flags.lang,
		"Name/Beschreibung des Systems (z.B. 'Workday Analytics', 'GitHub Copilot')",
		"Name/description of the system (e.g. 'Workday Analytics', 'GitHub Copilot')"))
	cmd.Flags().StringVar(&purpose, "purpose", "", tr(flags.lang,
		"Erklärter Einsatzzweck laut Arbeitgeber",
		"Declared purpose according to the employer"))
	cmd.Flags().StringVar(&dataCollected, "data", "", tr(flags.lang,
		"Verarbeitete Datenkategorien (kommagetrennt)",
		"Data categories processed (comma-separated)"))
	cmd.Flags().BoolVar(&monitorsPerformance, "monitors-performance", false,
		tr(flags.lang,
			"System kann individuelle Leistung messen, bewerten oder vergleichen",
			"System can measure, score, or compare individual employee performance"))
	cmd.Flags().BoolVar(&monitorsLocation, "monitors-location", false,
		tr(flags.lang,
			"System erfasst Standort oder Bewegungsprofile",
			"System tracks location or movement patterns"))
	cmd.Flags().BoolVar(&monitorsComms, "monitors-comms", false,
		tr(flags.lang,
			"System analysiert Kommunikation (E-Mail, Chat, Anrufe)",
			"System analyses communication (email, chat, calls)"))
	cmd.Flags().BoolVar(&influencesHR, "influences-hr", false,
		tr(flags.lang,
			"System fließt in Personalentscheidungen ein (Einstellung, Beförderung, Kündigung)",
			"System feeds into HR decisions (hiring, promotion, termination)"))
	cmd.Flags().BoolVar(&biometric, "biometric", false,
		tr(flags.lang,
			"System verarbeitet biometrische Daten (Gesicht, Stimme, Fingerabdruck)",
			"System processes biometric data (face, voice, fingerprint)"))
	cmd.Flags().BoolVar(&autoDecision, "auto-decision", false,
		tr(flags.lang,
			"System trifft oder empfiehlt automatisiert Entscheidungen über Arbeitnehmer",
			"System makes or recommends automated decisions about employees"))

	return cmd
}

func analyseKISystem(lang, system, purpose, dataCollected string,
	monitorsPerformance, monitorsLocation, monitorsComms,
	influencesHR, biometric, autoDecision bool) kiCheckResult {

	var triggers []string
	riskScore := 0

	if monitorsPerformance {
		riskScore += 3
		triggers = append(triggers, tr(lang,
			"Leistungsüberwachung: System kann individuelles Verhalten/Leistung messen oder bewerten → Kerntatbestand § 87 Nr. 6",
			"Performance monitoring: system can measure or score individual behaviour/performance → core trigger for § 87 Nr. 6"))
	}
	if monitorsLocation {
		riskScore += 2
		triggers = append(triggers, tr(lang,
			"Standorterfassung: Bewegungsprofile oder Anwesenheitsdaten → § 87 Nr. 6 i.V.m. DSGVO Art. 9",
			"Location tracking: movement profiles or presence data → § 87 Nr. 6 combined with GDPR Art. 9"))
	}
	if monitorsComms {
		riskScore += 3
		triggers = append(triggers, tr(lang,
			"Kommunikationsanalyse: Auswertung von E-Mail, Chat oder Anrufdaten → § 87 Nr. 6 + ggf. § 75 Abs. 2 BetrVG (Persönlichkeitsrechte)",
			"Communication analysis: parsing email, chat, or call data → § 87 Nr. 6 + potentially § 75 Abs. 2 BetrVG (personality rights)"))
	}
	if influencesHR {
		riskScore += 2
		triggers = append(triggers, tr(lang,
			"Personalentscheidungen: Systemoutput fließt in Einstellung, Beförderung oder Kündigung ein → § 87 Nr. 6; bei automatisierten Entscheidungen zusätzlich DSGVO Art. 22",
			"HR decisions: system output feeds into hiring, promotion, or termination → § 87 Nr. 6; for automated decisions also GDPR Art. 22"))
	}
	if biometric {
		riskScore += 3
		triggers = append(triggers, tr(lang,
			"Biometrische Daten: gesondert schutzbedürftig (DSGVO Art. 9 besondere Kategorie) → Zustimmung und BV zwingend erforderlich",
			"Biometric data: special category under GDPR Art. 9 → consent and BV strictly required"))
	}
	if autoDecision {
		riskScore += 2
		triggers = append(triggers, tr(lang,
			"Automatisierte Entscheidungen: DSGVO Art. 22 Abs. 1 gibt Betroffenen das Recht, nicht automatisierten Entscheidungen unterworfen zu werden → muss in BV geregelt werden",
			"Automated decisions: GDPR Art. 22 Abs. 1 gives employees the right not to be subject to fully automated decisions → must be addressed in the BV"))
	}
	if dataCollected != "" {
		riskScore++
		triggers = append(triggers, tr(lang,
			fmt.Sprintf("Datenverarbeitung: Das System verarbeitet folgende Kategorien: %s — jede Kategorie mit Personenbezug löst Informationspflicht nach § 80 Abs. 2 BetrVG aus", dataCollected),
			fmt.Sprintf("Data processing: the system processes the following categories: %s — each category with personal reference triggers disclosure obligation under § 80 Abs. 2 BetrVG", dataCollected)))
	}

	triggered := riskScore >= 2 || monitorsPerformance || biometric || monitorsComms

	riskLevel := tr(lang, "niedrig", "low")
	switch {
	case riskScore >= 7:
		riskLevel = tr(lang, "sehr hoch — sofortiger BR-Eingriff erforderlich", "very high — immediate BR intervention required")
	case riskScore >= 4:
		riskLevel = tr(lang, "hoch — BR muss unverzüglich handeln", "high — BR must act immediately")
	case riskScore >= 2:
		riskLevel = tr(lang, "mittel — BV erforderlich vor Produktiveinsatz", "medium — BV required before go-live")
	}

	clauses := []kiClause{
		{
			Clause: tr(lang, "Zweck und Anwendungsbereich", "Purpose and scope"),
			Explanation: tr(lang,
				"Genaue Beschreibung, welche Funktionen aktiviert sind und welche explizit deaktiviert werden müssen.",
				"Precise description of which functions are activated and which must be explicitly disabled."),
		},
		{
			Clause: tr(lang, "Verarbeitete Datenkategorien", "Data categories processed"),
			Explanation: tr(lang,
				"Vollständige Liste aller verarbeiteten personenbezogenen Daten; keine Restklausel 'weitere Daten'.",
				"Complete list of all personal data processed; no catch-all 'other data' clause."),
		},
		{
			Clause: tr(lang, "Zugriffsrechte", "Access rights"),
			Explanation: tr(lang,
				"Wer darf welche Daten sehen? Explizites Verbot des Zugriffs durch direkte Vorgesetzte ohne Begründung.",
				"Who may access which data? Explicit prohibition on line-manager access without justification."),
		},
		{
			Clause: tr(lang, "Löschfristen", "Retention and deletion"),
			Explanation: tr(lang,
				"Maximale Speicherdauer je Datenkategorie; automatisierte Löschroutine.",
				"Maximum retention period per data category; automated deletion routine."),
		},
		{
			Clause: tr(lang, "Verbot disziplinarischer Nutzung", "Prohibition on disciplinary use"),
			Explanation: tr(lang,
				"Systemdaten dürfen nicht als alleinige Grundlage für Abmahnungen, Kündigungen oder Gehaltskürzungen genutzt werden.",
				"System data must not be used as the sole basis for warnings (Abmahnungen), dismissals, or pay cuts."),
		},
		{
			Clause: tr(lang, "Transparenz gegenüber Beschäftigten", "Transparency for employees"),
			Explanation: tr(lang,
				"Arbeitnehmer müssen informiert werden, welche Daten erfasst werden, wer Zugriff hat und wie sie Auskunft erhalten.",
				"Employees must be informed about what data is collected, who has access, and how they can request disclosure."),
		},
	}
	if autoDecision {
		clauses = append(clauses, kiClause{
			Clause: tr(lang, "Menschliche Überprüfung automatisierter Entscheidungen", "Human review of automated decisions"),
			Explanation: tr(lang,
				"Jede automatisierte Entscheidung mit wesentlicher Auswirkung muss durch einen Menschen überprüfbar sein (DSGVO Art. 22).",
				"Every automated decision with significant impact must be subject to human review (GDPR Art. 22)."),
		})
	}

	mustNot := []string{
		tr(lang,
			"Das System in Betrieb nehmen, bevor eine Betriebsvereinbarung abgeschlossen wurde",
			"Deploy the system before a works agreement (Betriebsvereinbarung) is concluded"),
		tr(lang,
			"Daten nutzen, um Arbeitnehmer zu überwachen, ohne dies in der BV zu regeln",
			"Use data to monitor employees without addressing this in the BV"),
		tr(lang,
			"Systemdaten als alleinige Grundlage für Personalentscheidungen nutzen",
			"Use system data as the sole basis for personnel decisions"),
	}
	if biometric {
		mustNot = append(mustNot, tr(lang,
			"Biometrische Daten ohne ausdrückliche Einwilligung und BV verarbeiten (DSGVO Art. 9 Abs. 2)",
			"Process biometric data without explicit consent and BV (GDPR Art. 9 Abs. 2)"))
	}

	caseLaw := []kiCaseRef{
		{
			Citation: "BAG 27.05.1986 – 1 ABR 48/84",
			Holding: tr(lang,
				"§ 87 Nr. 6 gilt für jede technische Einrichtung, die zur Verhaltens- oder Leistungsüberwachung GEEIGNET ist — unabhängig vom erklärten Zweck",
				"§ 87 Nr. 6 applies to any technical device CAPABLE of monitoring behaviour or performance — regardless of declared purpose"),
		},
		{
			Citation: "BAG 26.07.1994 – 1 ABR 11/94",
			Holding: tr(lang,
				"Software fällt unter § 87 Nr. 6, wenn sie bei bestimmungsgemäßem Einsatz Arbeitnehmer überwachen kann",
				"Software falls under § 87 Nr. 6 if it can monitor employees when used as intended"),
		},
		{
			Citation: "BAG 25.09.2012 – 1 ABR 52/11",
			Holding: tr(lang,
				"IT-Systeme, die Nutzungsprofile oder Tätigkeitsnachweise erzeugen können, lösen § 87 Nr. 6 aus",
				"IT systems capable of generating usage profiles or activity records trigger § 87 Nr. 6"),
		},
		{
			Citation: "BAG 22.03.2022 – 1 ABR 34/20",
			Holding: tr(lang,
				"Algorithmisches Management (automatisierte Aufgabenzuweisung, Routing, Scoring) unterliegt erzwingbarer Mitbestimmung",
				"Algorithmic management (automated task assignment, routing, scoring) is subject to enforceable co-determination"),
		},
	}

	recommendation := tr(lang,
		"Fordern Sie sofort die vollständige technische Dokumentation an (§ 80 Abs. 2 BetrVG). "+
			"Verweigern Sie die Zustimmung zur Einführung bis eine Betriebsvereinbarung abgeschlossen ist. "+
			"Bei bereits laufendem Betrieb: Unterlassungsantrag beim Arbeitsgericht möglich (§ 87 Abs. 1 BetrVG i.V.m. § 23 Abs. 3 BetrVG).",
		"Demand the full technical documentation immediately (§ 80 Abs. 2 BetrVG). "+
			"Withhold consent to deployment until a works agreement (BV) is signed. "+
			"If the system is already running: an injunction application to the labour court is possible (§ 87 Abs. 1 BetrVG combined with § 23 Abs. 3 BetrVG).")

	if !triggered {
		recommendation = tr(lang,
			"Kein unmittelbarer § 87 Nr. 6 Tatbestand erkennbar. Dennoch empfohlen: Vollständige technische Dokumentation anfordern (§ 80 Abs. 2 BetrVG) und laufend prüfen, ob zukünftige Updates Überwachungsfunktionen hinzufügen.",
			"No immediate § 87 Nr. 6 trigger identified. However, recommended: request full technical documentation (§ 80 Abs. 2 BetrVG) and continually review whether future updates add monitoring capabilities.")
	}

	return kiCheckResult{
		SystemDescription: system,
		CoDetTriggered:    triggered,
		Paragraph:         "§ 87 Abs. 1 Nr. 6 BetrVG",
		RightType: tr(lang,
			"Erzwingbare Mitbestimmung — BR kann Einführung blockieren und Einigungsstelle anrufen",
			"Enforceable co-determination — BR can block deployment and invoke the conciliation committee (Einigungsstelle)"),
		RiskLevel:         riskLevel,
		Triggers:          triggers,
		BVRequiredClauses: clauses,
		EmployerMustNot:   mustNot,
		RelevantCaseLaw:   caseLaw,
		Recommendation:    recommendation,
		LegalBasis:        "§ 87 Abs. 1 Nr. 6 BetrVG; DSGVO Art. 5, 22; § 75 Abs. 2 BetrVG",
		Note: tr(lang,
			"Die Beweislast für fehlende Überwachungsfähigkeit liegt beim Arbeitgeber. Im Zweifel immer von § 87 Nr. 6 ausgehen.",
			"The burden of proving the absence of monitoring capability lies with the employer. When in doubt, always assume § 87 Nr. 6 applies."),
	}
}
