package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type tarifvertragCheckResult struct {
	Topic             string   `json:"topic"`
	TarifvertragType  string   `json:"tarifvertrag_type,omitempty"`
	TVPreempts        bool     `json:"tv_preempts_bv"`
	Explanation       string   `json:"explanation"`
	WhatBVCanStillDo  []string `json:"bv_still_possible,omitempty"`
	WhatBVCannotDo    []string `json:"bv_blocked,omitempty"`
	LegalBasis        string   `json:"legal_basis"`
	Note              string   `json:"note"`
}

func newTarifvertragCheckCmd(flags *rootFlags) *cobra.Command {
	var topic string
	var tvType string
	var tvCovers bool
	var hasOpeningClause bool

	cmd := &cobra.Command{
		Use:   "tarifvertrag-check",
		Short: tr(flags.lang, "Prüft den § 77 Abs. 3 Tarifvorbehalt: Kann der BR eine BV zu diesem Thema abschließen?", "Check § 77 Abs. 3 Tarifvorbehalt: can the BR conclude a BV on this topic?"),
		Long: tr(flags.lang,
			`Prüft, ob eine geplante Betriebsvereinbarung durch § 77 Abs. 3 BetrVG
(Tarifvorbehalt) gesperrt ist.

§ 77 Abs. 3 BetrVG: Arbeitsentgelte und sonstige Arbeitsbedingungen, die durch
Tarifvertrag geregelt sind oder üblicherweise geregelt werden, können nicht
Gegenstand einer Betriebsvereinbarung sein — es sei denn, der Tarifvertrag
enthält eine ausdrückliche Öffnungsklausel.

Wichtige Ausnahmen vom Tarifvorbehalt:
  • § 87 Abs. 1 BetrVG: Mitbestimmungsrechte bei sozialen Angelegenheiten (Arbeitszeit,
    Überwachung, Arbeitsentgeltgestaltung) — der TV sperrt hier NUR vollständig geregelte Bereiche
  • Verbesserungsprinzip: BV kann TV verbessern, wenn TV dies nicht ausschließt
  • Günstigkeitsprinzip (§ 4 Abs. 3 TVG) bei Individual-Arbeitsverträgen

Anwendungsfälle:
  lohn          Lohn / Gehalt / Entgelt → fast immer TV-gesperrt
  arbeitszeit   Arbeitszeit, Überstunden → oft TV-gesperrt, § 87 Öffnung möglich
  urlaub        Urlaubsanspruch und -entgelt → meist TV-gesperrt
  zulagen       Zulagen und Prämien → häufig TV-gesperrt
  homeoffice    Homeoffice / Mobile Arbeit → oft NICHT TV-gesperrt (neues Feld)
  software      IT-/KI-Systeme (§ 87 Nr. 6) → NICHT TV-gesperrt (Mitbestimmung)
  gesundheit    Arbeitsschutz / Gesundheit → meist NICHT TV-gesperrt
  custom        Freie Eingabe (--topic)`,
			`Checks whether a planned Betriebsvereinbarung is blocked by § 77 Abs. 3 BetrVG
(Tarifvorbehalt / collective agreement reservation).

§ 77 Abs. 3 BetrVG: Wages and other working conditions regulated or typically
regulated by a collective agreement (Tarifvertrag) may not be the subject of a
Betriebsvereinbarung — unless the Tarifvertrag contains an explicit opening clause.

Important exceptions to the Tarifvorbehalt:
  • § 87 Abs. 1 BetrVG: Co-determination rights on social matters (working time,
    monitoring, pay structure) — TV blocks only fully regulated areas
  • Improvement principle: BV may improve on TV unless TV expressly excludes this
  • Favourability principle (§ 4 Abs. 3 TVG) for individual employment contracts

Use cases:
  lohn          Pay / salary / wages → almost always TV-blocked
  arbeitszeit   Working time, overtime → often TV-blocked, § 87 opening possible
  urlaub        Holiday entitlement and pay → usually TV-blocked
  zulagen       Bonuses and allowances → often TV-blocked
  homeoffice    Homeoffice / mobile work → often NOT TV-blocked (emerging area)
  software      IT/AI systems (§ 87 Nr. 6) → NOT TV-blocked (co-determination right)
  gesundheit    Occupational health / safety → usually NOT TV-blocked
  custom        Free input (--topic)`),
		Example: strings.Trim(`
  betriebsrat tarifvertrag-check --topic lohn --tv-type "Branchentarifvertrag" --tv-covers
  betriebsrat tarifvertrag-check --topic homeoffice --no-tv-covers
  betriebsrat tarifvertrag-check --topic software --agent
  betriebsrat tarifvertrag-check --topic arbeitszeit --tv-covers --opening-clause --lang en`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if topic == "" {
				return fmt.Errorf(tr(flags.lang, "--topic ist erforderlich", "--topic is required"))
			}

			r := analyseTarifvorbehalt(flags.lang, topic, tvType, tvCovers, hasOpeningClause)

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(r)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Tarifvorbehalt-Check § 77 Abs. 3 BetrVG\n%s\n\n", strings.Repeat("═", 50))
			fmt.Fprintf(w, "%s: %s\n", tr(flags.lang, "Thema", "Topic"), r.Topic)
			if r.TarifvertragType != "" {
				fmt.Fprintf(w, "%s: %s\n", tr(flags.lang, "Tarifvertrag", "Collective agreement"), r.TarifvertragType)
			}
			blocked := tr(flags.lang, "✅ NEIN – BV ist zulässig", "✅ NO – BV is permissible")
			if r.TVPreempts {
				blocked = tr(flags.lang, "🚫 JA – TV sperrt dieses Thema", "🚫 YES – TV blocks this topic")
			}
			fmt.Fprintf(w, "%s: %s\n\n", tr(flags.lang, "TV-Sperrung", "TV blocks BV"), blocked)
			fmt.Fprintf(w, "%s\n\n", r.Explanation)

			if len(r.WhatBVCanStillDo) > 0 {
				fmt.Fprintf(w, "%s:\n", tr(flags.lang, "BV weiterhin möglich für", "BV still possible for"))
				for _, s := range r.WhatBVCanStillDo {
					fmt.Fprintf(w, "  ✓ %s\n", s)
				}
				fmt.Fprintln(w)
			}
			if len(r.WhatBVCannotDo) > 0 {
				fmt.Fprintf(w, "%s:\n", tr(flags.lang, "BV gesperrt für", "BV blocked for"))
				for _, s := range r.WhatBVCannotDo {
					fmt.Fprintf(w, "  ✗ %s\n", s)
				}
				fmt.Fprintln(w)
			}
			fmt.Fprintf(w, "%s: %s\n", tr(flags.lang, "Rechtsgrundlage", "Legal basis"), r.LegalBasis)
			fmt.Fprintf(w, "%s: %s\n", tr(flags.lang, "Hinweis", "Note"), r.Note)
			return nil
		},
	}

	cmd.Flags().StringVar(&topic, "topic", "", tr(flags.lang,
		"Thema: lohn|arbeitszeit|urlaub|zulagen|homeoffice|software|gesundheit|custom",
		"Topic: lohn|arbeitszeit|urlaub|zulagen|homeoffice|software|gesundheit|custom"))
	cmd.Flags().StringVar(&tvType, "tv-type", "", tr(flags.lang,
		"Art des Tarifvertrags (z.B. 'Branchentarifvertrag', 'Haustarifvertrag')",
		"Type of collective agreement (e.g. 'sector TV', 'company TV')"))
	cmd.Flags().BoolVar(&tvCovers, "tv-covers", false, tr(flags.lang,
		"Tarifvertrag regelt dieses Thema bereits",
		"Collective agreement already covers this topic"))
	cmd.Flags().BoolVar(&hasOpeningClause, "opening-clause", false, tr(flags.lang,
		"Tarifvertrag enthält eine Öffnungsklausel für BV zu diesem Thema",
		"Collective agreement contains an opening clause for BV on this topic"))
	_ = cmd.MarkFlagRequired("topic")
	return cmd
}

func analyseTarifvorbehalt(lang, topic, tvType string, tvCovers, hasOpeningClause bool) tarifvertragCheckResult {
	type topicProfile struct {
		inherentlyBlocked bool
		blockedDE, blockedEN string
		canDoDE, canDoEN []string
		cannotDE, cannotEN []string
		noteDE, noteEN string
	}

	profiles := map[string]topicProfile{
		"lohn": {
			inherentlyBlocked: true,
			blockedDE: "Arbeitsentgelt ist das Kerngebiet des § 77 Abs. 3 BetrVG. Sobald ein TV das Entgelt regelt, ist die BV gesperrt — ohne Öffnungsklausel.",
			blockedEN: "Pay is the core subject of § 77 Abs. 3 BetrVG. Once a TV regulates pay, the BV is blocked — without an opening clause.",
			canDoDE: []string{
				"Entgeltrahmenregelungen (wenn TV Öffnungsklausel enthält)",
				"Verfahren und Prozesse der Entgeltabrechnung (nicht die Höhe)",
				"Freiwillige Leistungen, die der TV ausdrücklich offenlässt",
			},
			canDoEN: []string{
				"Pay framework provisions (if TV contains opening clause)",
				"Payroll processes and procedures (not the amounts)",
				"Voluntary benefits the TV expressly leaves open",
			},
			cannotDE: []string{
				"Entgelthöhe, Lohngruppen, Gehaltserhöhungen",
				"Jahressonderzahlungen (Weihnachts-/Urlaubsgeld), wenn TV diese regelt",
				"Zulagenstruktur, soweit TV-geregelt",
			},
			cannotEN: []string{
				"Pay levels, pay grades, salary increases",
				"Annual special payments (Christmas/holiday pay) if regulated by TV",
				"Bonus structure, to the extent regulated by TV",
			},
			noteDE: "Ausnahme: § 87 Abs. 1 Nr. 10 BetrVG gibt BR Mitbestimmung bei der Lohngestaltung (Verteilungsprinzipien, Lohngruppen) — aber nicht bei der Entgelthöhe.",
			noteEN: "Exception: § 87 Abs. 1 Nr. 10 BetrVG gives BR co-determination over pay structure (distribution principles, pay grades) — but not over pay levels.",
		},
		"arbeitszeit": {
			inherentlyBlocked: true,
			blockedDE: "Arbeitszeit ist häufig TV-geregelt (Regelarbeitszeit, Überstundenzuschläge). Jedoch öffnet § 87 Abs. 1 Nr. 2 BetrVG einen eigenständigen Mitbestimmungsanspruch für Beginn/Ende der täglichen Arbeitszeit.",
			blockedEN: "Working time is frequently regulated by TV (standard hours, overtime premiums). However § 87 Abs. 1 Nr. 2 BetrVG creates an independent co-determination right over start/end of daily working time.",
			canDoDE: []string{
				"Beginn und Ende der täglichen Arbeitszeit (§ 87 Nr. 2 BetrVG — unabhängig vom TV)",
				"Gleitzeit-Rahmenregelungen, Gleitzeitkorridor",
				"Arbeitszeitkonten und Ausgleichszeiträume (wenn TV Öffnung lässt)",
				"Pausenregelungen im Detail",
			},
			canDoEN: []string{
				"Start and end of daily working time (§ 87 Nr. 2 — independent of TV)",
				"Flexitime framework, flexitime corridor",
				"Working-time accounts and settlement periods (if TV allows)",
				"Break arrangements in detail",
			},
			cannotDE: []string{
				"Gesamtdauer der Wochenarbeitszeit (TV-geregelt)",
				"Überstundenzuschlaghöhe (TV-geregelt)",
				"Urlaubsdauer (TV-geregelt)",
			},
			cannotEN: []string{
				"Total weekly hours (regulated by TV)",
				"Overtime premium rates (regulated by TV)",
				"Holiday duration (regulated by TV)",
			},
			noteDE: "§ 87 Abs. 1 BetrVG hat Vorrang: Die Mitbestimmungsrechte nach § 87 Nr. 2 und 3 bestehen auch bei TV-Regelung, sofern der TV keine abschließende Regelung trifft.",
			noteEN: "§ 87 Abs. 1 BetrVG takes precedence: co-determination rights under § 87 Nr. 2 and 3 exist even when a TV applies, unless the TV provides an exhaustive rule.",
		},
		"urlaub": {
			inherentlyBlocked: true,
			blockedDE: "Urlaubsanspruch und Urlaubsentgelt werden typischerweise durch TV geregelt und fallen unter § 77 Abs. 3 BetrVG.",
			blockedEN: "Holiday entitlement and holiday pay are typically regulated by TV and fall under § 77 Abs. 3 BetrVG.",
			canDoDE: []string{
				"Urlaubsplanung und Urlaubsliste (Verfahren, nicht Anspruchshöhe)",
				"Zusatzurlaub bei besonderen persönlichen Ereignissen (wenn TV Öffnung lässt)",
				"Betriebs-Urlaubszeiten / kollektive Betriebsschließung",
			},
			canDoEN: []string{
				"Holiday scheduling and holiday roster (procedure, not entitlement levels)",
				"Additional leave for special personal events (if TV allows)",
				"Collective plant closure periods",
			},
			cannotDE: []string{"Urlaubsdauer (Anzahl der Tage) über TV-Mindest hinaus ohne Öffnungsklausel", "Urlaubsentgeltberechnung"},
			cannotEN: []string{"Holiday duration beyond TV minimum without opening clause", "Holiday pay calculation"},
			noteDE:   "Betriebsvereinbarungen zum Verfahren der Urlaubsgewährung (§ 87 Abs. 1 Nr. 5 BetrVG) sind IMMER zulässig — das ist Mitbestimmung, kein TV-Thema.",
			noteEN:   "Works agreements on the procedure for granting leave (§ 87 Abs. 1 Nr. 5 BetrVG) are ALWAYS permissible — that is a co-determination matter, not a TV topic.",
		},
		"zulagen": {
			inherentlyBlocked: true,
			blockedDE: "Zulagen und Prämien, die der TV regelt, sind gesperrt. Freiwillige Leistungen des AG, die der TV nicht erfasst, können durch BV geregelt werden.",
			blockedEN: "Bonuses and allowances regulated by the TV are blocked. Voluntary employer benefits not covered by the TV can be regulated by BV.",
			canDoDE: []string{
				"Verteilung und Verfahren bei freiwilligen Leistungen (wenn TV keine abschließende Regelung trifft)",
				"Qualitative Kriterien für Leistungszulagen (§ 87 Nr. 10 BetrVG)",
			},
			canDoEN: []string{
				"Distribution and procedure for voluntary benefits (if TV has no exhaustive rule)",
				"Qualitative criteria for performance bonuses (§ 87 Nr. 10 BetrVG)",
			},
			cannotDE: []string{"Zulagenhöhe oder -art, die der TV abschließend regelt", "Jahresprämien, wenn TV diese festlegt"},
			cannotEN: []string{"Bonus level or type exhaustively regulated by TV", "Annual bonuses fixed by TV"},
			noteDE:   "Prüfen Sie immer, ob der TV die Zulage abschließend regelt oder ob Spielraum für eine ergänzende BV besteht.",
			noteEN:   "Always check whether the TV exhaustively regulates the bonus or whether there is room for a complementary BV.",
		},
		"homeoffice": {
			inherentlyBlocked: false,
			blockedDE: "Homeoffice und Mobile Arbeit sind in den meisten Tarifverträgen NICHT abschließend geregelt. § 77 Abs. 3 greift typischerweise NICHT — eine BV ist zulässig.",
			blockedEN: "Homeoffice and mobile work are NOT exhaustively regulated in most collective agreements. § 77 Abs. 3 typically does NOT apply — a BV is permissible.",
			canDoDE: []string{
				"Anspruch auf und Umfang von Mobile Arbeit",
				"Erreichbarkeitszeiten und Abwesenheitszeiten",
				"Ausstattungspflichten des Arbeitgebers",
				"Datenschutz und IT-Sicherheit im Homeoffice",
				"Unfallversicherung und Arbeitsstättenregeln",
			},
			canDoEN: []string{
				"Entitlement to and scope of mobile work",
				"Availability hours and offline periods",
				"Employer's equipment obligations",
				"Data protection and IT security in homeoffice",
				"Accident insurance and workplace rules",
			},
			cannotDE: nil,
			cannotEN: nil,
			noteDE:   "Prüfen Sie Ihren konkreten TV auf Klauseln zu 'mobiler Arbeit' oder 'Telearbeit'. Neuere TVs (post-2020) enthalten häufig Regelungen zu Homeoffice-Anspruch und Kostenerstattung.",
			noteEN:   "Check your specific TV for clauses on 'mobile work' or 'telework'. Newer TVs (post-2020) often include provisions on homeoffice entitlement and cost reimbursement.",
		},
		"software": {
			inherentlyBlocked: false,
			blockedDE: "IT-/KI-Systeme und technische Überwachungseinrichtungen sind grundsätzlich KEIN TV-Thema. Das Mitbestimmungsrecht nach § 87 Abs. 1 Nr. 6 BetrVG besteht unabhängig vom TV.",
			blockedEN: "IT/AI systems and technical monitoring devices are fundamentally NOT a TV topic. The co-determination right under § 87 Abs. 1 Nr. 6 BetrVG exists independently of any TV.",
			canDoDE: []string{
				"Einführung und Betrieb technischer Überwachungseinrichtungen",
				"Datenschutz-Regelungen für IT-Systeme",
				"KI-gestützte HR-Systeme und deren Einsatzgrenzen",
				"Löschfristen und Zugriffsprotokolle",
			},
			canDoEN: []string{
				"Introduction and operation of technical monitoring devices",
				"Data protection rules for IT systems",
				"AI-powered HR systems and their limits",
				"Deletion deadlines and access logs",
			},
			cannotDE: nil,
			cannotEN: nil,
			noteDE:   "§ 87 Abs. 1 Nr. 6 ist ein eigenständiges Mitbestimmungsrecht — der Tarifvorbehalt des § 77 Abs. 3 gilt für Mitbestimmungsrechte nach § 87 NICHT (§ 87 Abs. 1 Halbs. 1).",
			noteEN:   "§ 87 Abs. 1 Nr. 6 is an independent co-determination right — the Tarifvorbehalt of § 77 Abs. 3 does NOT apply to § 87 co-determination rights (§ 87 Abs. 1 Halbs. 1).",
		},
		"gesundheit": {
			inherentlyBlocked: false,
			blockedDE: "Arbeitsschutz und Gesundheitsmanagement sind typischerweise nicht TV-gesperrt. Das BR-Mitbestimmungsrecht nach § 87 Abs. 1 Nr. 7 BetrVG besteht unabhängig.",
			blockedEN: "Occupational health and safety is typically not TV-blocked. The BR co-determination right under § 87 Abs. 1 Nr. 7 BetrVG exists independently.",
			canDoDE: []string{
				"Gefährdungsbeurteilungen und Präventionsmaßnahmen",
				"Betriebliches Gesundheitsmanagement und Angebote",
				"Suchtprävention und psychische Gesundheit",
				"Bildschirmarbeitsplatz-Regelungen",
			},
			canDoEN: []string{
				"Risk assessments and preventive measures",
				"Workplace health management programmes",
				"Addiction prevention and mental health",
				"Display screen equipment rules",
			},
			cannotDE: nil,
			cannotEN: nil,
			noteDE:   "Lohnfortzahlung im Krankheitsfall ist hingegen TV-geregelt (EntgFG) — hier keine BV-Regelung möglich, soweit TV abschließend ist.",
			noteEN:   "Continued pay during illness is regulated by statute and TV (EntgFG) — no BV on that point where TV is exhaustive.",
		},
	}

	prof, known := profiles[topic]
	if !known {
		prof = topicProfile{
			inherentlyBlocked: tvCovers && !hasOpeningClause,
			blockedDE: "Für dieses Thema ist eine Einzelfallprüfung erforderlich. Entscheidend ist, ob ein geltender Tarifvertrag das Thema abschließend regelt.",
			blockedEN: "A case-by-case analysis is required for this topic. The decisive question is whether a currently applicable collective agreement exhaustively regulates the matter.",
		}
	}

	// Determine final blocked status
	finallyBlocked := prof.inherentlyBlocked
	if tvCovers && !hasOpeningClause {
		finallyBlocked = true
	}
	if hasOpeningClause {
		finallyBlocked = false
	}
	if !tvCovers {
		finallyBlocked = false
	}

	canDo := tr(lang, strings.Join(prof.canDoDE, "\n"), strings.Join(prof.canDoEN, "\n"))
	canNot := tr(lang, strings.Join(prof.cannotDE, "\n"), strings.Join(prof.cannotEN, "\n"))

	return tarifvertragCheckResult{
		Topic:            topic,
		TarifvertragType: tvType,
		TVPreempts:       finallyBlocked,
		Explanation:      tr(lang, prof.blockedDE, prof.blockedEN),
		WhatBVCanStillDo: splitLines(canDo),
		WhatBVCannotDo:   splitLines(canNot),
		LegalBasis:       "§ 77 Abs. 3 BetrVG; § 87 Abs. 1 BetrVG; § 4 Abs. 3 TVG",
		Note: tr(lang, func() string {
			if known {
				return prof.noteDE
			}
			return "Holen Sie sich rechtliche Beratung (Fachanwalt für Arbeitsrecht oder Gewerkschaft), bevor Sie eine BV in einem TV-geregelten Bereich abschließen."
		}(), func() string {
			if known {
				return prof.noteEN
			}
			return "Seek legal advice (labour law specialist or trade union) before concluding a BV in an area regulated by collective agreement."
		}()),
	}
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}
