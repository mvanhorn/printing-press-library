package betrvg

import (
	"strings"
	"unicode"
)

// StemDE strips common German morphological suffixes to improve keyword matching.
// "kündigt" → "kündig", "kündigung" → "kündig", "einführung" → "einführ".
func StemDE(word string) string {
	word = strings.ToLower(word)
	const minStem = 4
	for _, sfx := range []string{
		"ungsrecht", "ungsrechts",
		"ierungen", "ierung",
		"ungen", "igkeit", "heit", "keit", "schaft",
		"tionen", "tion",
		"lichen", "licher", "liches", "liche", "lich",
		"ischen", "ischer", "isches", "ische", "isch",
		"enden", "ender", "endes", "ende",
		"ierten", "ierte", "iert",
		"ung", "igt",
	} {
		if len(word)-len(sfx) >= minStem && strings.HasSuffix(word, sfx) {
			return word[:len(word)-len(sfx)]
		}
	}
	for _, sfx := range []string{"sten", "stem", "ster", "stes", "ste", "ten", "ter", "tes", "tem"} {
		if len(word)-len(sfx) >= minStem && strings.HasSuffix(word, sfx) {
			return word[:len(word)-len(sfx)]
		}
	}
	for _, sfx := range []string{"en", "er", "es", "em", "et", "te", "t"} {
		if len(word)-len(sfx) >= minStem && strings.HasSuffix(word, sfx) {
			return word[:len(word)-len(sfx)]
		}
	}
	return word
}

func normalizeTerm(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		return unicode.ToLower(r)
	}, strings.TrimSpace(s))
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

// ContainsFold reports whether sub is contained in s, case-insensitively.
func ContainsFold(s, sub string) bool {
	return containsFold(s, sub)
}

// Deadlines returns situation-specific BetrVG Fristen (deadline rules).
type DeadlineRule struct {
	Situation   string
	Paragraph   int
	Days        int
	Description string
	Note        string
}

func Deadlines() []DeadlineRule {
	return deadlineRules
}

var deadlineRules = []DeadlineRule{
	{
		Situation:   "ordentliche kündigung",
		Paragraph:   102,
		Days:        7,
		Description: "Stellungnahme bei ordentlicher Kündigung",
		Note:        "1 Woche ab Zugang der Anhörung. Ohne Anhörung ist die Kündigung unwirksam (§ 102 Abs. 1 Satz 3).",
	},
	{
		Situation:   "außerordentliche kündigung",
		Paragraph:   102,
		Days:        3,
		Description: "Stellungnahme bei außerordentlicher (fristloser) Kündigung",
		Note:        "3 Tage ab Zugang der Anhörung. Ohne Anhörung ist auch die außerordentliche Kündigung unwirksam.",
	},
	{
		Situation:   "massenentlassung",
		Paragraph:   17,
		Days:        30,
		Description: "Konsultationsverfahren vor Massenentlassung (KSchG § 17 i.V.m. § 111 BetrVG)",
		Note:        "Der AG muss den BR mindestens 30 Tage vor Einreichung der Massenentlassungsanzeige bei der Agentur für Arbeit konsultieren.",
	},
	{
		Situation:   "einstellung",
		Paragraph:   99,
		Days:        7,
		Description: "Zustimmung oder Verweigerung bei Einstellungen",
		Note:        "1 Woche ab Vorlage der vollständigen Unterlagen. Ohne Äußerung gilt Zustimmung als erteilt.",
	},
	{
		Situation:   "versetzung",
		Paragraph:   99,
		Days:        7,
		Description: "Zustimmung oder Verweigerung bei Versetzungen",
		Note:        "1 Woche ab Vorlage. Ohne Äußerung gilt Zustimmung als erteilt.",
	},
	{
		Situation:   "betriebsänderung",
		Paragraph:   111,
		Days:        0,
		Description: "Unterrichtung und Beratung bei Betriebsänderung",
		Note:        "Kein starres Fristende – Beratung muss ernsthaft geführt werden. Interessenausgleichsverhandlungen können mehrere Wochen dauern.",
	},
	{
		Situation:   "betriebsversammlung",
		Paragraph:   43,
		Days:        0,
		Description: "Betriebsversammlung einzuberufen pro Kalendervierteljahr",
		Note:        "Mindestens einmal pro Quartal (§ 43 Abs. 1). Kein gesetzlicher Mindestabstand zwischen den Terminen.",
	},
}

// Checklists returns pre-built action checklists per situation.
type ChecklistItem struct {
	Step      int
	Action    string
	Paragraph string
	Priority  string // high, medium, low
}

type Checklist struct {
	Situation string
	Keywords  []string
	Steps     []ChecklistItem
}

// AllChecklists returns all pre-built situation checklists.
func AllChecklists() []Checklist {
	return checklists
}

func GetChecklist(situation string) *Checklist {
	norm := normalizeTerm(situation)
	for _, cl := range checklists {
		for _, kw := range cl.Keywords {
			if containsFold(norm, kw) {
				return &cl
			}
		}
	}
	return nil
}

var checklists = []Checklist{
	{
		Situation: "Kündigung durch Arbeitgeber",
		Keywords:  []string{"kündigung", "kündigt", "kündigen", "entlassung", "entlässt", "fristlos"},
		Steps: []ChecklistItem{
			{1, "Anhörungsschreiben prüfen: Vollständigkeit der Sozialdaten, Kündigungsgrund", "§ 102 BetrVG", "high"},
			{2, "Frist einhalten: 1 Woche bei ordentlicher, 3 Tage bei fristloser Kündigung", "§ 102 Abs. 2", "high"},
			{3, "Sozialauswahl prüfen: Hat AG die Sozialkriterien (Alter, Betriebszugehörigkeit, Unterhaltspflichten, Schwerbehinderung) beachtet?", "§ 1 Abs. 3 KSchG", "high"},
			{4, "Ggf. Widerspruch einlegen (§ 102 Abs. 3): z.B. fehlerhafte Sozialauswahl, andere Beschäftigungsmöglichkeit", "§ 102 Abs. 3 BetrVG", "high"},
			{5, "Betroffenen Arbeitnehmer informieren und beraten", "§ 80 BetrVG", "medium"},
			{6, "Bei Schwerbehinderung: Zustimmung Integrationsamt erforderlich (SGB IX § 168)", "SGB IX § 168", "high"},
			{7, "Protokoll der Stellungnahme anfertigen", "§ 34 BetrVG", "medium"},
		},
	},
	{
		Situation: "Betriebsänderung / Restrukturierung",
		Keywords:  []string{"betriebsänderung", "restrukturierung", "umstrukturierung", "stilllegung", "verlagerung", "outsourcing"},
		Steps: []ChecklistItem{
			{1, "Unterrichtung durch AG verlangen: vollständige Informationen über geplante Maßnahme, Zeitplan, betroffene Arbeitnehmer", "§ 111 BetrVG", "high"},
			{2, "Beraten ob Betriebsänderung gem. § 111 BetrVG vorliegt (>20 AN Betrieb, wesentliche Änderung)", "§ 111 BetrVG", "high"},
			{3, "Interessenausgleich verhandeln: Ob und wie die Betriebsänderung durchgeführt wird", "§ 112 BetrVG", "high"},
			{4, "Sozialplan verhandeln: Ausgleich der wirtschaftlichen Nachteile für betroffene AN", "§ 112 BetrVG", "high"},
			{5, "Bei Scheitern: Einigungsstelle anrufen (für Sozialplan erzwingbar)", "§ 112 Abs. 4 BetrVG", "high"},
			{6, "Informationspflichten gegenüber Arbeitnehmern: Betriebsversammlung einberufen", "§ 43 BetrVG", "medium"},
			{7, "Bei Massenentlassung: Konsultationsverfahren nach § 17 KSchG", "§ 17 KSchG, § 111 BetrVG", "high"},
			{8, "Nachteilsausgleichsansprüche prüfen falls AG ohne Interessenausgleich handelt", "§ 113 BetrVG", "medium"},
		},
	},
	{
		Situation: "Einführung neuer technischer Systeme / Software",
		Keywords:  []string{"software", "system einführung", "it-system", "überwachung", "monitoring", "ki einführung", "künstliche intelligenz"},
		Steps: []ChecklistItem{
			{1, "Prüfen ob Leistungs- oder Verhaltensüberwachung möglich ist (auch indirekt durch Logs, KI)", "§ 87 Abs. 1 Nr. 6 BetrVG", "high"},
			{2, "Erzwingbare Mitbestimmung: Betriebsvereinbarung aushandeln bevor Einführung", "§ 87 BetrVG", "high"},
			{3, "Datenschutzfolgenabschätzung anfordern (DSGVO Art. 35)", "DSGVO Art. 35", "high"},
			{4, "Datenschutzbeauftragten einbeziehen", "DSGVO", "medium"},
			{5, "Sachverständigen für technische Beurteilung hinzuziehen (auf Kosten des AG)", "§ 80 Abs. 3 BetrVG", "medium"},
			{6, "BV aushandeln: Zweck, Umfang, Datenzugriff, Löschfristen, Schulung", "§ 88 BetrVG", "high"},
			{7, "Bei KI: Mitbestimmung bei Änderung der Arbeitsabläufe prüfen (§ 87 Abs. 1 Nr. 1, Nr. 6)", "§ 87 BetrVG", "medium"},
		},
	},
	{
		Situation: "Personelle Einzelmaßnahmen (Einstellung, Versetzung)",
		Keywords:  []string{"einstellung", "versetzung", "umgruppierung", "eingruppierung"},
		Steps: []ChecklistItem{
			{1, "Unterlagen prüfen: Stellt AG alle erforderlichen Infos zur Verfügung?", "§ 99 BetrVG", "high"},
			{2, "Frist: 1 Woche ab Vorlage der vollständigen Unterlagen zur Zustimmung/Verweigerung", "§ 99 Abs. 3 BetrVG", "high"},
			{3, "Prüfen ob Verweigerungsgrund (§ 99 Abs. 2) vorliegt: Verstoß gegen Gesetz/BV, Nachteil für andere AN", "§ 99 Abs. 2 BetrVG", "high"},
			{4, "Schriftliche Verweigerung mit Begründung innerhalb der Frist", "§ 99 Abs. 3 BetrVG", "high"},
			{5, "Bei Nichtäußerung: Zustimmung gilt als erteilt", "§ 99 Abs. 3 Satz 2", "high"},
		},
	},
	{
		Situation: "Homeoffice / Remote Work Regelung",
		Keywords:  []string{"homeoffice", "remote work", "mobiles arbeiten", "telearbeit"},
		Steps: []ChecklistItem{
			{1, "Mitbestimmungsrecht prüfen: Beginn/Ende der Arbeitszeit (§ 87 Abs. 1 Nr. 2)", "§ 87 Abs. 1 Nr. 2 BetrVG", "high"},
			{2, "Betriebsvereinbarung Homeoffice aushandeln: Zeiten, Erreichbarkeit, Ausstattung", "§ 87 BetrVG", "high"},
			{3, "Datenschutz im Homeoffice regeln: Zugriffsschutz, Datenverschlüsselung", "DSGVO, § 87 BetrVG", "high"},
			{4, "Arbeitssicherheit im Homeoffice: Bildschirmarbeitsplatzverordnung, ArbStättV", "§ 89 BetrVG", "medium"},
			{5, "Regelung zu Aufwendungsersatz (Internet, Strom, Büromaterial)", "§ 87, § 40 BetrVG", "medium"},
		},
	},
}
