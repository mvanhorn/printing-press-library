// Package betrvg provides a structured knowledge base of the German
// Betriebsverfassungsgesetz (BetrVG) for works council (Betriebsrat) advisory.
package betrvg

import "strings"

// CoDeterminationType classifies the strength of the BR's legal right.
type CoDeterminationType string

const (
	// MitbestimmungErzwingbar means the BR can force a Betriebsvereinbarung via Einigungsstelle.
	MitbestimmungErzwingbar CoDeterminationType = "Mitbestimmung (erzwingbar)"
	// Mitwirkung means the BR has participation rights but cannot block.
	Mitwirkung CoDeterminationType = "Mitwirkung"
	// Unterrichtung means the employer must inform the BR.
	Unterrichtung CoDeterminationType = "Unterrichtung"
	// Beratung means the employer must consult the BR but retains decision.
	Beratung CoDeterminationType = "Beratung"
	// Zustimmung means the employer needs BR consent.
	Zustimmung CoDeterminationType = "Zustimmungsvorbehalt"
	// Keine means no specific BR right.
	Keine CoDeterminationType = "Kein spezielles Betriebsratsrecht"
)

// Paragraph describes a BetrVG paragraph.
type Paragraph struct {
	Number       int
	Title        string
	Summary      string
	Keywords     []string
	CoDetermType CoDeterminationType
	DeadlineDays int    // 0 means no specific deadline
	TopicSlug    string // betriebsrat.de topic slug
	TopicURL     string
}

// All returns the full BetrVG paragraph knowledge base.
func All() []Paragraph {
	return paragraphs
}

// ByNumber returns a paragraph by § number, or nil.
func ByNumber(n int) *Paragraph {
	for i := range paragraphs {
		if paragraphs[i].Number == n {
			return &paragraphs[i]
		}
	}
	return nil
}

// ByKeywords returns paragraphs whose keywords overlap with the given terms,
// ordered by relevance score (most matches first).
func ByKeywords(terms []string) []Paragraph {
	scored := ByKeywordsScored(terms)
	result := make([]Paragraph, len(scored))
	for i, s := range scored {
		result[i] = s.Paragraph
	}
	return result
}

// ScoredParagraph pairs a paragraph with a relevance score.
type ScoredParagraph struct {
	Paragraph
	Score int
}

// ByKeywordsScored returns paragraphs ranked by how many input terms match their keywords.
// Exact keyword match scores 3; stem match scores 2; substring match scores 1.
func ByKeywordsScored(terms []string) []ScoredParagraph {
	scores := map[int]int{}
	for _, term := range terms {
		term = normalizeTerm(term)
		if len(term) < 3 {
			continue
		}
		termStem := StemDE(term)
		for _, p := range paragraphs {
			for _, kw := range p.Keywords {
				kwNorm := normalizeTerm(kw)
				if kwNorm == term {
					scores[p.Number] += 3
				} else if strings.Contains(kwNorm, term) || strings.Contains(term, kwNorm) {
					scores[p.Number] += 1
				} else {
					kwStem := StemDE(kwNorm)
					if len(termStem) >= 4 && len(kwStem) >= 4 {
						if termStem == kwStem {
							scores[p.Number] += 2
						} else if strings.Contains(kwStem, termStem) || strings.Contains(termStem, kwStem) {
							scores[p.Number] += 1
						}
					}
				}
			}
		}
	}

	type pair struct{ num, score int }
	var pairs []pair
	for num, score := range scores {
		if score > 0 {
			pairs = append(pairs, pair{num, score})
		}
	}
	// Sort by score descending, then paragraph number ascending for stability
	for i := 0; i < len(pairs); i++ {
		for j := i + 1; j < len(pairs); j++ {
			if pairs[j].score > pairs[i].score || (pairs[j].score == pairs[i].score && pairs[j].num < pairs[i].num) {
				pairs[i], pairs[j] = pairs[j], pairs[i]
			}
		}
	}

	seen := map[int]bool{}
	var result []ScoredParagraph
	for _, pr := range pairs {
		if seen[pr.num] {
			continue
		}
		for i := range paragraphs {
			if paragraphs[i].Number == pr.num {
				result = append(result, ScoredParagraph{paragraphs[i], pr.score})
				seen[pr.num] = true
				break
			}
		}
	}
	return result
}

var paragraphs = []Paragraph{
	{
		Number:       1,
		Title:        "Errichtung von Betriebsräten",
		Summary:      "In Betrieben mit in der Regel mindestens fünf ständigen wahlberechtigten Arbeitnehmern, von denen drei wählbar sind, werden Betriebsräte gewählt.",
		Keywords:     []string{"gründen", "errichten", "5 beschäftigte", "wahlberechtigt", "betriebsrat gründen", "wählbar"},
		CoDetermType: Keine,
		TopicSlug:    "betriebsrat-gruenden",
		TopicURL:     "https://www.betriebsrat.de/br/themen/betriebsrat-gruenden/uebersicht",
	},
	{
		Number:       9,
		Title:        "Größe des Betriebsrats",
		Summary:      "Die Größe des Betriebsrats richtet sich nach der Anzahl der wahlberechtigten Arbeitnehmer. Ab 5 AN: 1 Mitglied, ab 21: 3 Mitglieder, ab 51: 5 Mitglieder usw.",
		Keywords:     []string{"größe", "mitglieder", "anzahl", "betriebsratsmitglieder"},
		CoDetermType: Keine,
		TopicSlug:    "betriebsratswahl",
		TopicURL:     "https://www.betriebsrat.de/br/themen/betriebsratswahl/uebersicht",
	},
	{
		Number:       37,
		Title:        "Ehrenamtliche Tätigkeit, Arbeitsbefreiung",
		Summary:      "Betriebsratsmitglieder führen ihr Amt unentgeltlich als Ehrenamt. Sie sind für Betriebsratstätigkeit von der Arbeit freizustellen, ohne Minderung des Arbeitsentgelts.",
		Keywords:     []string{"freistellung", "arbeitsbefreiung", "ehrenamt", "entgelt", "arbeitszeit br"},
		CoDetermType: Keine,
		TopicSlug:    "betriebsverfassungsrecht",
		TopicURL:     "https://www.betriebsrat.de/br/themen/betriebsverfassungsrecht/uebersicht",
	},
	{
		Number:       38,
		Title:        "Freigestellte Betriebsratsmitglieder",
		Summary:      "Ab 200 Arbeitnehmern sind Betriebsratsmitglieder zur Durchführung ihrer Aufgaben von der beruflichen Tätigkeit vollständig freizustellen.",
		Keywords:     []string{"freistellung", "vollständig freigestellt", "ab 200"},
		CoDetermType: Keine,
		TopicSlug:    "betriebsverfassungsrecht",
		TopicURL:     "https://www.betriebsrat.de/br/themen/betriebsverfassungsrecht/uebersicht",
	},
	{
		Number:       40,
		Title:        "Kosten und Sachaufwand des Betriebsrats",
		Summary:      "Die durch die Tätigkeit des Betriebsrats entstehenden Kosten trägt der Arbeitgeber. Der AG stellt dem BR Räume, Sachmittel, Büropersonal und Informations-/Kommunikationsmittel zur Verfügung.",
		Keywords:     []string{"kosten", "sachaufwand", "büro", "computer", "sachmittel", "betriebsrat kosten", "laptop", "handy", "arbeitsmittel"},
		CoDetermType: Keine,
		TopicSlug:    "betriebsverfassungsrecht",
		TopicURL:     "https://www.betriebsrat.de/br/themen/betriebsverfassungsrecht/uebersicht",
	},
	{
		Number:       43,
		Title:        "Betriebsversammlungen",
		Summary:      "Der Betriebsrat hat einmal in jedem Kalendervierteljahr eine Betriebsversammlung einzuberufen. Themen: Tätigkeitsbericht des BR, Berichte AG.",
		Keywords:     []string{"betriebsversammlung", "versammlung", "einberufen", "vierteljahr"},
		CoDetermType: Keine,
		TopicSlug:    "betriebsversammlung",
		TopicURL:     "https://www.betriebsrat.de/br/themen/betriebsversammlung/uebersicht",
	},
	{
		Number:       74,
		Title:        "Grundsätze für die Zusammenarbeit",
		Summary:      "Arbeitgeber und Betriebsrat arbeiten vertrauensvoll zum Wohl der Arbeitnehmer und des Betriebs zusammen. Monatliche Besprechungen (§ 74 Abs. 1). Friedenspflicht.",
		Keywords:     []string{"zusammenarbeit", "vertrauensvoll", "besprechung", "monatlich", "friedenspflicht"},
		CoDetermType: Keine,
		TopicSlug:    "betriebsverfassungsrecht",
		TopicURL:     "https://www.betriebsrat.de/br/themen/betriebsverfassungsrecht/uebersicht",
	},
	{
		Number:       76,
		Title:        "Einigungsstelle",
		Summary:      "Die Einigungsstelle wird bei Regelungsstreitigkeiten zwischen AG und BR gebildet. Sie besteht aus Beisitzern beider Seiten und einem unparteiischen Vorsitzenden (Richter). Ihr Spruch ersetzt die fehlende Einigung bei erzwingbarer Mitbestimmung.",
		Keywords:     []string{"einigungsstelle", "einigen", "schlichter", "vermittlung", "spruch", "zwangsschlichtung", "schiedsstelle"},
		CoDetermType: Keine,
		TopicSlug:    "betriebsverfassungsrecht",
		TopicURL:     "https://www.betriebsrat.de/br/themen/betriebsverfassungsrecht/uebersicht",
	},
	{
		Number:       77,
		Title:        "Durchführung gemeinsamer Beschlüsse, Betriebsvereinbarungen",
		Summary:      "Betriebsvereinbarungen (BV) sind schriftlich niederzulegen und vom AG und BR zu unterzeichnen. Sie gelten unmittelbar und zwingend für alle AN des Betriebs. Günstigere einzelvertragliche Regelungen bleiben wirksam.",
		Keywords:     []string{"betriebsvereinbarung", "bv", "durchführung", "unterschreiben", "schriftlich", "unmittelbar zwingend"},
		CoDetermType: Keine,
		TopicSlug:    "betriebsvereinbarungen",
		TopicURL:     "https://www.betriebsrat.de/br/themen/betriebsvereinbarungen/uebersicht",
	},
	{
		Number:       78,
		Title:        "Schutzbestimmungen (Benachteiligungsverbot)",
		Summary:      "Mitglieder des Betriebsrats dürfen wegen ihrer Tätigkeit nicht benachteiligt oder begünstigt werden. Der AG darf BR-Mitglieder nicht versetzen oder anderweitig behindern. Zuwiderhandlungen sind strafbar.",
		Keywords:     []string{"benachteiligung", "schutz br", "behinderung betriebsratstätigkeit", "benachteiligungsverbot", "kündigungsschutz br"},
		CoDetermType: Keine,
		TopicSlug:    "betriebsverfassungsrecht",
		TopicURL:     "https://www.betriebsrat.de/br/themen/betriebsverfassungsrecht/uebersicht",
	},
	{
		Number:       79,
		Title:        "Geheimhaltungspflicht",
		Summary:      "Betriebsratsmitglieder sind zur Geheimhaltung verpflichtet über Betriebs- und Geschäftsgeheimnisse sowie über persönliche Angelegenheiten der Arbeitnehmer. Die Pflicht gilt auch nach Beendigung des Amts.",
		Keywords:     []string{"schweigepflicht", "geheimhaltung", "vertraulich", "betriebsgeheimnis", "datenschutz br"},
		CoDetermType: Keine,
		TopicSlug:    "betriebsverfassungsrecht",
		TopicURL:     "https://www.betriebsrat.de/br/themen/betriebsverfassungsrecht/uebersicht",
	},
	{
		Number:       80,
		Title:        "Allgemeine Aufgaben des Betriebsrats",
		Summary:      "Der BR hat u.a. die Aufgabe, darüber zu wachen, dass Gesetze, Verordnungen, Tarifverträge und Betriebsvereinbarungen eingehalten werden. Recht auf Unterrichtung durch den AG.",
		Keywords:     []string{"überwachung", "unterrichtung", "aufgaben", "gesetze überwachen", "allgemeine aufgaben"},
		CoDetermType: Unterrichtung,
		TopicSlug:    "betriebsverfassungsrecht",
		TopicURL:     "https://www.betriebsrat.de/br/themen/betriebsverfassungsrecht/uebersicht",
	},
	{
		Number:       87,
		Title:        "Mitbestimmungsrechte (Soziale Angelegenheiten)",
		Summary:      "Der BR hat in sozialen Angelegenheiten erzwingbare Mitbestimmungsrechte. Dazu gehören: Arbeitszeit, Überstunden, Urlaubsregelung, Überwachungssoftware/-technik (Nr. 6), Lohngestaltung, Betriebsordnung, mobiles Arbeiten (Nr. 14).",
		Keywords:     []string{"überstunden", "arbeitszeit", "urlaub", "urlaubsregelung", "überwachung", "monitoring", "kamera", "zeiterfassung", "homeoffice", "remote work", "mobiles arbeiten", "lohn", "vergütung", "betriebsordnung", "software überwachung", "leistungsüberwachung", "verhaltensüberwachung", "ki", "künstliche intelligenz", "schichtplan", "dienstplan", "führt ein", "einführung", "einführt", "überwacht", "trackt", "protokolliert", "analytics", "tracking", "telemetry", "surveillance", "co-determination", "performance tracking", "it system", "ai system", "artificial intelligence", "workforce analytics", "people analytics"},
		CoDetermType: MitbestimmungErzwingbar,
		TopicSlug:    "betriebsverfassungsrecht",
		TopicURL:     "https://www.betriebsrat.de/br/themen/betriebsverfassungsrecht/uebersicht",
	},
	{
		Number:       88,
		Title:        "Freiwillige Betriebsvereinbarungen",
		Summary:      "Durch Betriebsvereinbarung können weitere Regelungen getroffen werden, z.B. zu betrieblichem Vorschlagswesen, Humanisierung der Arbeit. Freiwillig = kein Spruch der Einigungsstelle.",
		Keywords:     []string{"freiwillige betriebsvereinbarung", "bv freiwillig", "vorschlagswesen"},
		CoDetermType: Mitwirkung,
		TopicSlug:    "betriebsvereinbarungen",
		TopicURL:     "https://www.betriebsrat.de/br/themen/betriebsvereinbarungen/uebersicht",
	},
	{
		Number:       91,
		Title:        "Menschengerechte Gestaltung der Arbeit",
		Summary:      "Der AG hat die Arbeit so zu gestalten, dass Arbeitnehmer nicht übermäßig belastet werden. Der BR kann Maßnahmen zur menschengerechten Gestaltung der Arbeit beantragen.",
		Keywords:     []string{"ergonomie", "belastung", "menschengerecht", "arbeitsbedingungen", "psychische belastung", "burnout", "stress"},
		CoDetermType: Mitwirkung,
		TopicSlug:    "arbeitsschutz-arbeitssicherheit",
		TopicURL:     "https://www.betriebsrat.de/br/themen/arbeitsschutz-arbeitssicherheit/uebersicht",
	},
	{
		Number:       92,
		Title:        "Personalplanung",
		Summary:      "Der AG hat den BR über die Personalplanung zu unterrichten und zu beraten. Der BR kann Vorschläge zur Einführung und Durchführung der Personalplanung machen.",
		Keywords:     []string{"personalplanung", "stellenplan", "personalentwicklung", "einstellungsplanung"},
		CoDetermType: Beratung,
		TopicSlug:    "personalentwicklung",
		TopicURL:     "https://www.betriebsrat.de/br/themen/personalentwicklung/uebersicht",
	},
	{
		Number:       93,
		Title:        "Ausschreibung von Arbeitsplätzen",
		Summary:      "Der BR kann verlangen, dass freie Arbeitsplätze im Betrieb oder Unternehmen vor Einstellung ausgeschrieben werden. Verstöß kann zur Zustimmungsverweigerung bei der Einstellung führen.",
		Keywords:     []string{"ausschreibung", "stellenausschreibung", "interne ausschreibung", "freie stellen"},
		CoDetermType: Mitwirkung,
		TopicSlug:    "personelles",
		TopicURL:     "https://www.betriebsrat.de/br/themen/personelles/uebersicht",
	},
	{
		Number:       94,
		Title:        "Personalfragebogen, Beurteilungsgrundsätze",
		Summary:      "Personalfragebogen und allgemeine Beurteilungsgrundsätze bedürfen der Zustimmung des Betriebsrats. Der BR kann unzulässige Fragen ablehnen.",
		Keywords:     []string{"personalfragebogen", "beurteilung", "assessment", "leistungsbeurteilung", "fragebogen"},
		CoDetermType: Zustimmung,
		TopicSlug:    "personelles",
		TopicURL:     "https://www.betriebsrat.de/br/themen/personelles/uebersicht",
	},
	{
		Number:       95,
		Title:        "Auswahlrichtlinien",
		Summary:      "Mitbestimmung bei Aufstellung von Richtlinien über die Auswahl von Bewerbern bei Einstellungen, Versetzungen, Umgruppierungen und Kündigungen.",
		Keywords:     []string{"auswahlrichtlinien", "auswahl", "bewerber", "einstellung richtlinien"},
		CoDetermType: MitbestimmungErzwingbar,
		TopicSlug:    "personelles",
		TopicURL:     "https://www.betriebsrat.de/br/themen/personelles/uebersicht",
	},
	{
		Number:       96,
		Title:        "Förderung der Berufsbildung",
		Summary:      "AG und BR haben die Berufsbildung der Arbeitnehmer zu fördern. Der BR kann Vorschläge für Maßnahmen der Berufsbildung machen und über die Planung informiert werden.",
		Keywords:     []string{"berufsbildung", "weiterbildung", "fortbildung", "schulung", "training", "qualifizierung"},
		CoDetermType: Beratung,
		TopicSlug:    "berufsbildung",
		TopicURL:     "https://www.betriebsrat.de/br/themen/berufsbildung/uebersicht",
	},
	{
		Number:       98,
		Title:        "Durchführung betrieblicher Bildungsmaßnahmen",
		Summary:      "Bei der Durchführung betrieblicher Berufsbildungsmaßnahmen hat der BR Mitbestimmungsrechte (Zustimmungsvorbehalt). BR kann aus bestimmten Gründen die Zustimmung verweigern.",
		Keywords:     []string{"bildungsmaßnahmen", "berufsbildung durchführung", "seminare", "schulungsmaßnahmen"},
		CoDetermType: Zustimmung,
		DeadlineDays: 7,
		TopicSlug:    "berufsbildung",
		TopicURL:     "https://www.betriebsrat.de/br/themen/berufsbildung/uebersicht",
	},
	{
		Number:       99,
		Title:        "Mitbestimmung bei personellen Einzelmaßnahmen",
		Summary:      "Der AG bedarf der Zustimmung des BR bei Einstellungen, Eingruppierungen, Umgruppierungen und Versetzungen. Der BR kann aus bestimmten Gründen die Zustimmung verweigern.",
		Keywords:     []string{"einstellung", "einstellt", "eingestellt", "stellt ein", "eingruppierung", "umgruppierung", "versetzung", "versetzt", "personelle massnahme", "zustimmung einstellung", "neueinstellung", "neu einstellen"},
		CoDetermType: Zustimmung,
		DeadlineDays: 7,
		TopicSlug:    "personelles",
		TopicURL:     "https://www.betriebsrat.de/br/themen/personelles/uebersicht",
	},
	{
		Number:       100,
		Title:        "Vorläufige personelle Maßnahmen",
		Summary:      "Der AG kann aus dringenden Gründen vorläufige Maßnahmen treffen. Der BR kann widersprechen; Entscheidung durch Arbeitsgericht.",
		Keywords:     []string{"vorläufige massnahme", "dringlich", "arbeitsgericht personell"},
		CoDetermType: Zustimmung,
		TopicSlug:    "personelles",
		TopicURL:     "https://www.betriebsrat.de/br/themen/personelles/uebersicht",
	},
	{
		Number:       102,
		Title:        "Mitbestimmung bei Kündigungen",
		Summary:      "Vor jeder Kündigung ist der BR zu hören. Bei ordentlicher Kündigung hat der BR 1 Woche Zeit zur Stellungnahme. Bei außerordentlicher Kündigung 3 Tage. Ohne Anhörung ist die Kündigung unwirksam.",
		Keywords:     []string{"kündigung", "kündigt", "kündigen", "entlassung", "entlässt", "entlassen", "fristlose kündigung", "ordentliche kündigung", "außerordentliche kündigung", "anhörung kündigung", "unwirksam", "widerspruch kündigung", "betriebsbedingt", "personenbedingt", "verhaltensbedingt", "sozialauswahl", "abmahnung"},
		CoDetermType: MitbestimmungErzwingbar,
		DeadlineDays: 7,
		TopicSlug:    "kuendigung",
		TopicURL:     "https://www.betriebsrat.de/br/themen/kuendigung/uebersicht",
	},
	{
		Number:       103,
		Title:        "Außerordentliche Kündigung und Versetzung von Betriebsratsmitgliedern",
		Summary:      "Die außerordentliche Kündigung eines BR-Mitglieds bedarf der Zustimmung des Betriebsrats. Verweigert der BR die Zustimmung, kann der AG Ersetzung durch Arbeitsgericht beantragen. Gleiches gilt für Versetzungen.",
		Keywords:     []string{"kündigung betriebsratsmitglied", "br kündigung", "sonderkündigungsschutz", "betriebsrat mitglied entlassen"},
		CoDetermType: Zustimmung,
		DeadlineDays: 3,
		TopicSlug:    "betriebsverfassungsrecht",
		TopicURL:     "https://www.betriebsrat.de/br/themen/betriebsverfassungsrecht/uebersicht",
	},
	{
		Number:       104,
		Title:        "Entfernung betriebsstörender Arbeitnehmer",
		Summary:      "Hat ein Arbeitnehmer durch gesetzwidriges Verhalten oder grobe Verletzung der Betriebsfreiheit die betriebliche Ordnung gestört, kann der BR verlangen, dass der AG die erforderlichen Maßnahmen trifft.",
		Keywords:     []string{"betriebsstörer", "mobbing", "betriebsordnung verletzung"},
		CoDetermType: Mitwirkung,
		TopicSlug:    "betriebsverfassungsrecht",
		TopicURL:     "https://www.betriebsrat.de/br/themen/betriebsverfassungsrecht/uebersicht",
	},
	{
		Number:       106,
		Title:        "Wirtschaftsausschuss",
		Summary:      "In Unternehmen mit mehr als 100 ständig beschäftigten Arbeitnehmern ist ein Wirtschaftsausschuss zu bilden. Er hat das Recht auf Unterrichtung über wirtschaftliche Angelegenheiten des Unternehmens.",
		Keywords:     []string{"wirtschaftsausschuss", "wirtschaftliche angelegenheiten", "unterrichtung wirtschaft", "bilanz", "umsatz"},
		CoDetermType: Unterrichtung,
		TopicSlug:    "betriebsverfassungsrecht",
		TopicURL:     "https://www.betriebsrat.de/br/themen/betriebsverfassungsrecht/uebersicht",
	},
	{
		Number:       111,
		Title:        "Betriebsänderungen",
		Summary:      "In Unternehmen mit mehr als 20 wahlberechtigten Arbeitnehmern hat der AG den BR über geplante Betriebsänderungen zu unterrichten und zu beraten. Betriebsänderungen sind: Stilllegung, Verlagerung, Zusammenschluss, wesentliche Änderungen, Einführung neuer Arbeitsmethoden.",
		Keywords:     []string{"betriebsänderung", "stilllegung", "schließt", "schließung", "verlagerung", "verlagert", "umstrukturierung", "outsourcing", "restrukturierung", "interessenausgleich", "stellenabbau", "fusioniert", "fusion"},
		CoDetermType: Beratung,
		TopicSlug:    "betriebsverfassungsrecht",
		TopicURL:     "https://www.betriebsrat.de/br/themen/betriebsverfassungsrecht/uebersicht",
	},
	{
		Number:       112,
		Title:        "Interessenausgleich und Sozialplan",
		Summary:      "Kommt wegen einer Betriebsänderung ein Interessenausgleich nicht zustande, sind Arbeitnehmer mit Nachteilen durch einen Sozialplan zu entschädigen. Der BR kann einen Sozialplan erzwingen.",
		Keywords:     []string{"sozialplan", "interessenausgleich", "entschädigung", "massenentlassung", "abfindung"},
		CoDetermType: MitbestimmungErzwingbar,
		TopicSlug:    "betriebsverfassungsrecht",
		TopicURL:     "https://www.betriebsrat.de/br/themen/betriebsverfassungsrecht/uebersicht",
	},
	{
		Number:       113,
		Title:        "Nachteilsausgleich",
		Summary:      "Weicht der AG ohne zwingenden Grund vom Interessenausgleich ab, haben betroffene Arbeitnehmer Anspruch auf Abfindung.",
		Keywords:     []string{"nachteilsausgleich", "abfindung interessenausgleich"},
		CoDetermType: Mitwirkung,
		TopicSlug:    "betriebsverfassungsrecht",
		TopicURL:     "https://www.betriebsrat.de/br/themen/betriebsverfassungsrecht/uebersicht",
	},
}
