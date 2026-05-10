package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type massenentlassungThreshold struct {
	MinEmployees int    `json:"min_employees"`
	MaxEmployees int    `json:"max_employees,omitempty"`
	MinDismissal string `json:"min_dismissal"`
	Triggered    bool   `json:"triggered"`
}

type massenentlassungResult struct {
	Employees       int                        `json:"employees"`
	PlannedDismiss  int                        `json:"planned_dismissals"`
	Threshold       massenentlassungThreshold  `json:"applicable_threshold"`
	Triggered       bool                       `json:"massenentlassung_triggered"`
	BRRights        []string                   `json:"br_rights"`
	Steps           []massenentlassungStep     `json:"procedure_steps"`
	Deadlines       []string                   `json:"key_deadlines"`
	Consequences    []string                   `json:"consequences_if_skipped"`
	LegalBasis      string                     `json:"legal_basis"`
	Note            string                     `json:"note"`
}

type massenentlassungStep struct {
	Step      int    `json:"step"`
	Who       string `json:"who"`
	Action    string `json:"action"`
	Deadline  string `json:"deadline,omitempty"`
	LegalRef  string `json:"legal_ref"`
	Critical  bool   `json:"critical"`
}

func newMassenentlassungCmd(flags *rootFlags) *cobra.Command {
	var employees int
	var planned int

	cmd := &cobra.Command{
		Use:   "massenentlassung",
		Short: "Check if Massenentlassung thresholds are met and generate the full § 17 KSchG procedure",
		Long: `Determines whether planned dismissals trigger the Massenentlassung procedure
under § 17 KSchG and generates the complete step-by-step compliance checklist.

§ 17 KSchG thresholds (Betrieb mit i.d.R. mehr als 20 AN):
  21–59 AN:   ≥ 6 Entlassungen within 30 days
  60–499 AN:  ≥ 10% of workforce OR ≥ 26 Entlassungen (whichever is lower)
  ≥ 500 AN:   ≥ 30 Entlassungen within 30 days

CRITICAL: Kündigungen without proper Massenentlassungsanzeige are VOID
(§ 17 Abs. 1 KSchG; BAG Urt. v. 22.09.2016 – 2 AZR 276/16).`,
		Example: strings.Trim(`
  betriebsrat massenentlassung --employees 150 --planned 18
  betriebsrat massenentlassung --employees 60 --planned 7 --agent
  betriebsrat massenentlassung --employees 500 --planned 35 --json`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if employees <= 0 {
				return fmt.Errorf("--employees ist erforderlich und muss > 0 sein")
			}
			if planned <= 0 {
				return fmt.Errorf("--planned ist erforderlich und muss > 0 sein")
			}
			if dryRunOK(flags) {
				return nil
			}

			r := buildMassenentlassungResult(employees, planned)

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(r)
			}

			w := cmd.OutOrStdout()
			triggered := "NEIN"
			if r.Triggered {
				triggered = "JA — § 17 KSchG-Verfahren ist einzuhalten"
			}
			fmt.Fprintf(w, "Massenentlassung ausgelöst: %s\n\n", triggered)
			fmt.Fprintf(w, "Betriebsgröße: %d AN | Geplante Entlassungen: %d | Schwelle: %s\n\n",
				r.Employees, r.PlannedDismiss, r.Threshold.MinDismissal)

			if r.Triggered {
				fmt.Fprintln(w, "Verfahrensschritte (in dieser Reihenfolge):")
				for _, s := range r.Steps {
					crit := ""
					if s.Critical {
						crit = " [KRITISCH]"
					}
					fmt.Fprintf(w, "  %d. [%s]%s %s\n", s.Step, s.Who, crit, s.Action)
					if s.Deadline != "" {
						fmt.Fprintf(w, "     Frist: %s  |  %s\n", s.Deadline, s.LegalRef)
					} else {
						fmt.Fprintf(w, "     %s\n", s.LegalRef)
					}
				}
				fmt.Fprintln(w, "\nFolgen bei Verstoß:")
				for _, c := range r.Consequences {
					fmt.Fprintf(w, "  • %s\n", c)
				}
			}
			fmt.Fprintf(w, "\nRechtsgrundlage: %s\n", r.LegalBasis)
			if r.Note != "" {
				fmt.Fprintf(w, "Hinweis: %s\n", r.Note)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&employees, "employees", 0, "Anzahl Arbeitnehmer im Betrieb (i.d.R. Beschäftigte)")
	cmd.Flags().IntVar(&planned, "planned", 0, "Anzahl geplanter Entlassungen innerhalb von 30 Tagen")
	_ = cmd.MarkFlagRequired("employees")
	_ = cmd.MarkFlagRequired("planned")
	return cmd
}

func buildMassenentlassungResult(employees, planned int) massenentlassungResult {
	r := massenentlassungResult{
		Employees:      employees,
		PlannedDismiss: planned,
		LegalBasis:     "§ 17 KSchG; §§ 111–113 BetrVG; BAG Urt. v. 22.09.2016 – 2 AZR 276/16",
	}

	// Determine threshold and whether triggered
	var threshold massenentlassungThreshold
	var triggered bool

	switch {
	case employees < 21:
		threshold = massenentlassungThreshold{
			MinEmployees: 1,
			MaxEmployees: 20,
			MinDismissal: "§ 17 KSchG gilt nicht (Betrieb < 21 AN)",
			Triggered:    false,
		}
		r.Note = "§ 17 KSchG findet keine Anwendung, da der Betrieb weniger als 21 Arbeitnehmer hat. Prüfen Sie Kündigungsschutz nach allgemeinem KSchG."
	case employees <= 59:
		threshold = massenentlassungThreshold{
			MinEmployees: 21,
			MaxEmployees: 59,
			MinDismissal: "≥ 6 Entlassungen innerhalb von 30 Tagen",
			Triggered:    planned >= 6,
		}
		triggered = planned >= 6
	case employees <= 499:
		minByPercent := int(float64(employees) * 0.10)
		if minByPercent < 26 {
			minByPercent = 26
		}
		minRequired := minByPercent
		threshold = massenentlassungThreshold{
			MinEmployees: 60,
			MaxEmployees: 499,
			MinDismissal: fmt.Sprintf("≥ 10%% der AN oder ≥ 26 (= %d von %d AN)", minRequired, employees),
			Triggered:    planned >= minRequired,
		}
		triggered = planned >= minRequired
	default:
		threshold = massenentlassungThreshold{
			MinEmployees: 500,
			MinDismissal: "≥ 30 Entlassungen innerhalb von 30 Tagen",
			Triggered:    planned >= 30,
		}
		triggered = planned >= 30
	}

	r.Threshold = threshold
	r.Triggered = triggered

	if !triggered {
		r.Note = fmt.Sprintf("Bei %d AN und %d geplanten Entlassungen wird die Schwelle nicht erreicht. "+
			"§ 17 KSchG-Verfahren ist nicht erforderlich — individuelle Kündigungsschutzregeln gelten weiterhin.",
			employees, planned)
		r.BRRights = []string{
			"§ 102 BetrVG: Anhörung vor jeder Einzelkündigung (7 Tage ordentlich / 3 Tage außerordentlich)",
			"§ 99 BetrVG gilt bei Einstellungen/Versetzungen im Zuge der Umstrukturierung",
		}
		return r
	}

	r.BRRights = []string{
		"§ 17 Abs. 2 KSchG: Recht auf schriftliche Unterrichtung und Beratung vor Anzeige",
		"§ 111 BetrVG: Unterrichtung und Beratung über die Betriebsänderung",
		"§ 112 BetrVG: Verhandlung Interessenausgleich und Sozialplan (Sozialplan erzwingbar)",
		"§ 102 BetrVG: Anhörung vor jeder Einzelkündigung (zusätzlich zum § 17-Verfahren)",
	}

	r.Steps = []massenentlassungStep{
		{
			Step:     1,
			Who:      "Arbeitgeber",
			Action:   "BR schriftlich über geplante Massenentlassung unterrichten: Gründe, Zahl und Gruppen der zu Entlassenden, Zeitraum, Auswahlkriterien, Berechnung der Abfindungen",
			Deadline: "Vor Einreichung der Anzeige bei der Agentur für Arbeit",
			LegalRef: "§ 17 Abs. 2 Satz 1 KSchG",
			Critical: true,
		},
		{
			Step:     2,
			Who:      "Betriebsrat",
			Action:   "Stellungnahme zur geplanten Massenentlassung abgeben (kann auch Zustimmung, Ablehnung oder Gegenvorschläge umfassen). Ohne Stellungnahme muss AG 2 Wochen warten.",
			Deadline: "Keine gesetzliche Frist; AG muss mindestens 2 Wochen nach Unterrichtung warten",
			LegalRef: "§ 17 Abs. 3 Satz 2 KSchG",
			Critical: false,
		},
		{
			Step:     3,
			Who:      "Arbeitgeber + Betriebsrat",
			Action:   "Parallele Verhandlungen: Interessenausgleich (§ 112 Abs. 1 BetrVG) und Sozialplan (§ 112 Abs. 4 BetrVG — erzwingbar). Interessenausgleich regelt Ob/Wie, Sozialplan regelt Entschädigungen.",
			Deadline: "Vor Umsetzung der Maßnahme; Einigungsstelle auf Antrag",
			LegalRef: "§§ 111, 112 BetrVG",
			Critical: true,
		},
		{
			Step:     4,
			Who:      "Arbeitgeber",
			Action:   "Massenentlassungsanzeige (Formular BA 17) bei der zuständigen Agentur für Arbeit einreichen. Abschrift der BR-Stellungnahme beifügen (oder Erklärung warum keine vorliegt).",
			Deadline: "Mindestens 30 Tage vor den ersten Kündigungen",
			LegalRef: "§ 17 Abs. 1 und Abs. 3 KSchG",
			Critical: true,
		},
		{
			Step:     5,
			Who:      "Agentur für Arbeit",
			Action:   "Sperrfrist: Entlassungen dürfen frühestens 1 Monat nach Eingang der Anzeige wirksam werden (verlängerbar auf 2 Monate in begründeten Ausnahmefällen).",
			Deadline: "1 Monat Wartezeit ab Anzeigeneingang (§ 18 Abs. 1 KSchG)",
			LegalRef: "§ 18 KSchG",
			Critical: true,
		},
		{
			Step:     6,
			Who:      "Arbeitgeber",
			Action:   "Individuelle Anhörung des BR zu jeder Einzelkündigung nach § 102 BetrVG (zusätzlich und unabhängig vom § 17-Verfahren). Ohne Anhörung ist die jeweilige Kündigung unwirksam.",
			Deadline: "7 Tage je ordentlicher Kündigung (3 Tage fristlos)",
			LegalRef: "§ 102 BetrVG",
			Critical: true,
		},
		{
			Step:     7,
			Who:      "Arbeitgeber",
			Action:   "Kündigungen aussprechen erst nach: Ablauf der Sperrfrist + vollständiger § 102-Anhörung + ggf. nach Sozialplaneinigung oder Einigungsstellenspruch.",
			Deadline: "Nach allen vorherigen Schritten",
			LegalRef: "§§ 17–18 KSchG, § 102 BetrVG",
			Critical: true,
		},
	}

	r.Deadlines = []string{
		"BR-Konsultation: mindestens 2 Wochen vor der Anzeige (§ 17 Abs. 3 KSchG)",
		"Massenentlassungsanzeige: mindestens 30 Tage vor den Entlassungen",
		"Sperrfrist: 1 Monat ab Anzeigeneingang (verlängerbar auf 2 Monate)",
		"§ 102-Anhörung je Kündigung: 7 Tage ordentlich / 3 Tage außerordentlich",
	}

	r.Consequences = []string{
		"Fehlende Massenentlassungsanzeige → Kündigungen sind UNWIRKSAM (§ 17 Abs. 1 KSchG, BAG 2016)",
		"Keine BR-Unterrichtung/Beratung → Anzeige ist unwirksam → Kündigungen unwirksam",
		"Nichteinhaltung Sperrfrist → Kündigungen wirken nicht (§ 18 KSchG)",
		"Kein Sozialplan → BR kann Einigungsstelle anrufen; Nachteilsausgleich nach § 113 BetrVG",
		"Fehlende § 102-Anhörung je Kündigung → jeweilige Kündigung unwirksam (§ 102 Abs. 1 Satz 3 BetrVG)",
	}

	return r
}
