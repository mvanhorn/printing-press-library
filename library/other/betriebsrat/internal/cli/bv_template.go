package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type bvTemplateResult struct {
	Topic      string   `json:"topic"`
	Title      string   `json:"title"`
	LegalBasis string   `json:"legal_basis"`
	Sections   []bvSection `json:"sections"`
	Note       string   `json:"note"`
}

type bvSection struct {
	Number  int    `json:"section"`
	Heading string `json:"heading"`
	Body    string `json:"body"`
}

func newBVTemplateCmd(flags *rootFlags) *cobra.Command {
	var employer string
	var dateStr string

	cmd := &cobra.Command{
		Use:   "bv-template [topic]",
		Short: "Generate a skeleton Betriebsvereinbarung for common topics",
		Long: `Generates a structured draft Betriebsvereinbarung (BV) with the required clauses,
BR-negotiated must-haves, and placeholder sections to fill in.

Topics:
  homeoffice          Mobile Arbeit / Homeoffice (§ 87 Abs. 1 Nr. 2, 14 BetrVG)
  software            Einführung IT-Systeme / Überwachungssoftware (§ 87 Abs. 1 Nr. 6 BetrVG)
  arbeitszeit         Arbeitszeit / Gleitzeit / Überstunden (§ 87 Abs. 1 Nr. 2, 3 BetrVG)
  datenschutz         Datenschutz und Beschäftigtendaten (DSGVO Art. 88, § 26 BDSG)
  videoüberwachung    Videoüberwachung im Betrieb (§ 87 Abs. 1 Nr. 6 BetrVG, DSGVO)
  leistungsbeurteilung Leistungsbeurteilung / Zielvereinbarung (§ 94 BetrVG)

Each template includes:
  - Legally required clauses (Pflichtinhalt)
  - BR-negotiated protective clauses (Schutzklauseln)
  - Placeholder sections ([...]) to fill with your specific terms
  - Common pitfalls and negotiation tips as inline comments`,
		Example: strings.Trim(`
  betriebsrat bv-template homeoffice --employer "Musterfirma GmbH"
  betriebsrat bv-template software --agent
  betriebsrat bv-template arbeitszeit --employer "Firma AG" --json`, "\n"),
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

			topic := strings.ToLower(strings.TrimSpace(args[0]))
			refDate := time.Now()
			if dateStr != "" {
				parsed, err := time.Parse("2006-01-02", dateStr)
				if err != nil {
					return fmt.Errorf("invalid --date format (YYYY-MM-DD): %w", err)
				}
				refDate = parsed
			}
			employerStr := employer
			if employerStr == "" {
				employerStr = "[Arbeitgeber / Unternehmen]"
			}
			dateFormatted := refDate.Format("02.01.2006")

			var r *bvTemplateResult
			switch topic {
			case "homeoffice", "mobile arbeit", "mobile-arbeit", "mobiles arbeiten", "telearbeit":
				r = bvTemplateHomeoffice(employerStr, dateFormatted)
			case "software", "it-system", "ki", "überwachung", "monitoring":
				r = bvTemplateSoftware(employerStr, dateFormatted)
			case "arbeitszeit", "gleitzeit", "überstunden", "schichtplan":
				r = bvTemplateArbeitszeit(employerStr, dateFormatted)
			case "datenschutz", "dsgvo", "beschäftigtendaten", "personaldaten":
				r = bvTemplateDatenschutz(employerStr, dateFormatted)
			case "videoüberwachung", "videoueberwachung", "kamera", "kameraüberwachung":
				r = bvTemplateVideoüberwachung(employerStr, dateFormatted)
			case "leistungsbeurteilung", "leistungsbewertung", "zielvereinbarung", "beurteilung":
				r = bvTemplateLeistungsbeurteilung(employerStr, dateFormatted)
			default:
				return fmt.Errorf("unbekanntes Thema %q — erlaubt: homeoffice, software, arbeitszeit, datenschutz, videoüberwachung, leistungsbeurteilung", topic)
			}

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(r)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "BETRIEBSVEREINBARUNG\n%s\n\n", strings.Repeat("=", 60))
			fmt.Fprintf(w, "zwischen\n  %s (Arbeitgeber)\nund\n  dem Betriebsrat\n\nwird folgende Betriebsvereinbarung geschlossen:\n\n", employerStr)
			for _, s := range r.Sections {
				fmt.Fprintf(w, "§ %d  %s\n%s\n\n", s.Number, s.Heading, s.Body)
			}
			fmt.Fprintf(w, "Rechtsgrundlage: %s\n", r.LegalBasis)
			if r.Note != "" {
				fmt.Fprintf(w, "\nHinweis: %s\n", r.Note)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&employer, "employer", "", "Name des Arbeitgebers / Unternehmens")
	cmd.Flags().StringVar(&dateStr, "date", "", "Datum der BV (YYYY-MM-DD, Standard: heute)")
	return cmd
}

func bvTemplateHomeoffice(employer, date string) *bvTemplateResult {
	return &bvTemplateResult{
		Topic:      "homeoffice",
		Title:      "Betriebsvereinbarung Mobile Arbeit / Homeoffice",
		LegalBasis: "§ 87 Abs. 1 Nr. 2 und Nr. 14 BetrVG; ArbStättV; DSGVO",
		Sections: []bvSection{
			{1, "Geltungsbereich", "Diese Betriebsvereinbarung gilt für alle Arbeitnehmerinnen und Arbeitnehmer der " + employer + ", soweit ihre Tätigkeit mobil ausgeführt werden kann."},
			{2, "Begriffsbestimmung", `Mobile Arbeit im Sinne dieser BV ist die Erbringung von Arbeitsleistung außerhalb der Betriebsstätte mit Hilfe von Informations- und Kommunikationstechnik, insbesondere im häuslichen Umfeld (Homeoffice).

[HINWEIS: Abgrenzen von Telearbeit i.S.v. § 2 Abs. 7 ArbStättV, wenn fester häuslicher Arbeitsplatz eingerichtet wird — dann gelten Arbeitsstättenregeln.]`},
			{3, "Anspruch und Voraussetzungen", `Arbeitnehmer haben Anspruch auf mobile Arbeit im Umfang von bis zu [X] Tagen pro Woche / Monat, sofern:
  a) die Tätigkeit mobil ausführbar ist (keine physische Anwesenheit erforderlich),
  b) ein geeigneter, datenschutzkonformer Arbeitsplatz im Homeoffice vorhanden ist,
  c) keine betrieblichen Gründe entgegenstehen.

[TIPP BR: Individualanspruch vereinbaren — nicht nur "nach Absprache".]`},
			{4, "Arbeitszeit im Homeoffice", `Im Homeoffice gelten dieselben Arbeitszeitregelungen wie im Betrieb (ArbZG, Tarifvertrag, geltende BV Arbeitszeit).

Kernarbeitszeiten (Erreichbarkeit): [Uhrzeit] bis [Uhrzeit] Uhr
Außerhalb der Kernarbeitszeiten besteht keine Pflicht zur Erreichbarkeit (Recht auf Nichterreichbarkeit).

[TIPP BR: Das Recht auf Nichterreichbarkeit ausdrücklich verankern — ohne es degeneriert Homeoffice zu 24/7-Verfügbarkeit.]`},
			{5, "Ausstattung und Kosten", `Der Arbeitgeber stellt folgende Arbeitsmittel zur Verfügung:
  - [Laptop, Headset, Monitor — konkret aufzählen]
  - Zugang zu sicheren VPN-Verbindungen und betrieblichen Systemen

Der Arbeitgeber übernimmt folgende Aufwendungen:
  - Internetkosten: [Pauschale X € pro Monat / anteilig / keine]
  - Strom: [Pauschale X € pro Monat / Nachweis / keine]
  - Büromaterial: [auf Nachweis / Pauschale]

[PFLICHT § 40 BetrVG: Kosten der BR-Tätigkeit trägt AG — dies gilt auch für Homeoffice-Equipment von BR-Mitgliedern.]`},
			{6, "Datenschutz und Informationssicherheit", `Im Homeoffice sind folgende Datenschutzmaßnahmen einzuhalten:
  - Bildschirm für Dritte nicht einsehbar
  - Ausdrucke mit personenbezogenen Daten gesichert aufbewahren und vernichten
  - Keine Nutzung privater Geräte für betriebliche Daten ohne ausdrückliche Genehmigung
  - VPN-Pflicht bei Zugriff auf interne Systeme

Eine Überwachung der Arbeitnehmer im Homeoffice (Kamera, Screencapture, Keylogger) ist untersagt.`},
			{7, "Arbeitsschutz", `Der Arbeitgeber ist verantwortlich für die Einhaltung der Arbeitssicherheitsvorschriften. Der Arbeitnehmer versichert, dass der häusliche Arbeitsplatz den Anforderungen der Bildschirmarbeitsverordnung entspricht.

Der Arbeitgeber hat das Recht, den häuslichen Arbeitsplatz nach Vorankündigung zu besichtigen (nur nach Zustimmung des Arbeitnehmers).`},
			{8, "Widerruf und Rückkehr", `Der Arbeitgeber kann mobile Arbeit mit einer Ankündigungsfrist von [4 Wochen / 2 Wochen] widerrufen, wenn:
  a) betriebliche Erfordernisse dies verlangen,
  b) Datenschutz- oder Sicherheitspflichten verletzt werden.

[TIPP BR: Widerrufsfrist möglichst lang vereinbaren; Widerruf aus betrieblichen Gründen eng definieren.]`},
			{9, "Schlussbestimmungen", fmt.Sprintf(`Diese Betriebsvereinbarung tritt am %s in Kraft. Sie kann mit einer Frist von [3 Monaten] zum Ende eines Kalenderquartals gekündigt werden.

Nachwirkung: Bis zum Abschluss einer neuen BV gelten die Regelungen dieser BV weiter (§ 77 Abs. 6 BetrVG, nur für erzwingbare Mitbestimmung).

Ort, Datum: ___________

%s                              Der Betriebsrat
(Arbeitgeber)                  (Vorsitzende/r)`, date, employer)},
		},
		Note: "Dieses Template ist ein Ausgangspunkt — passen Sie alle [Platzhalter] und [TIPP]-Abschnitte an Ihre spezifische Situation an. Lassen Sie die BV von einem Fachanwalt für Arbeitsrecht prüfen.",
	}
}

func bvTemplateSoftware(employer, date string) *bvTemplateResult {
	return &bvTemplateResult{
		Topic:      "software",
		Title:      "Betriebsvereinbarung Einführung und Nutzung von IT-Systemen",
		LegalBasis: "§ 87 Abs. 1 Nr. 6 BetrVG; DSGVO Art. 88; § 26 BDSG",
		Sections: []bvSection{
			{1, "Geltungsbereich und Zweck", "Diese BV regelt den Einsatz des Systems [Systemname / Hersteller] bei " + employer + ". Zweck: [Projektmanagement / Zeiterfassung / Kommunikation / HR-Tool — konkret benennen]. Eine Nutzung zu anderen als den vereinbarten Zwecken ist untersagt."},
			{2, "Technische Beschreibung", `Das System [Name] ermöglicht folgende Funktionen:
  - [Funktion 1]
  - [Funktion 2]

Technisch mögliche, aber ausdrücklich NICHT genutzte Funktionen:
  - [z.B. Standortverfolgung, Aktivitätsmonitoring, Keystroke-Logging]

[PFLICHT: Alle Überwachungsfunktionen explizit ausschließen — nicht nur "wir nutzen es nicht", sondern technisch sperren oder vertraglich ausschließen.]`},
			{3, "Zulässige Datenerhebung", `Folgende Daten werden erhoben:
  - [Datenkategorie 1]: Zweck [X], Speicherdauer [Y]
  - [Datenkategorie 2]: Zweck [X], Speicherdauer [Y]

Nicht erhoben werden:
  - Leistungs- oder Verhaltensdaten, die eine individuelle Beurteilung ermöglichen
  - Standortdaten außerhalb ausdrücklicher Zustimmung
  - Kommunikationsinhalte (nur Metadaten für [definierten Zweck])

[KRITISCH: DSGVO-konforme Zweckbindung ist Kerninhalt dieser BV. Ohne klare Negativliste nutzt AG Graubereiche.]`},
			{4, "Auswertung und Zugriffsrechte", `Auswertungen auf Individualebene (einzelne Mitarbeiterdaten) sind nur zulässig für:
  - [Enumerierte Ausnahmen, z.B. bei begründetem Verdacht auf Straftat nach Betriebsratsanhörung]

Regelauswertungen erfolgen nur auf Abteilungs- oder Teamebene (aggregiert, anonymisiert).

Zugriffsrechte: [Liste: wer darf was sehen — HR, direkte Führungskraft, IT-Admin]

Der Betriebsrat erhält auf Anfrage Einsicht in Systemprotokolle im Rahmen seiner Überwachungspflicht (§ 80 BetrVG).`},
			{5, "Löschfristen", `Daten werden wie folgt gelöscht:
  - Aktivitätsprotokolle: nach [30 / 90] Tagen
  - Projektdaten: [nach Projektabschluss + 6 Monate / nach 3 Jahren]
  - HR-relevante Daten: nach Ausscheiden + [3 / 10] Jahre (gesetzliche Aufbewahrung)

Automatische Löschung ist technisch sicherzustellen.`},
			{6, "Information und Schulung", `Alle Beschäftigten werden vor Einführung über:
  a) Zweck und Funktionsweise des Systems,
  b) erhobene Daten und ihre Verwendung,
  c) ihre Rechte (Auskunft, Berichtigung, Löschung nach DSGVO)
informiert. Der Betriebsrat ist an der Gestaltung der Information zu beteiligen.`},
			{7, "Änderungen am System", `Wesentliche Änderungen an Funktionsumfang oder Datenerhebung bedürfen einer Änderung dieser BV und der erneuten Zustimmung des Betriebsrats (§ 87 Abs. 1 Nr. 6 BetrVG).

[TIPP BR: "Wesentliche Änderung" weit definieren — auch Updates, die neue Überwachungsfunktionen aktivieren.]`},
			{8, "Schlussbestimmungen", fmt.Sprintf(`Diese BV tritt am %s in Kraft. Eine Kündigung ist mit [3-monatiger] Frist möglich. Bei Kündigung darf das System bis zum Abschluss einer neuen BV nicht genutzt werden.

Ort, Datum: ___________

%s                              Der Betriebsrat
(Arbeitgeber)                  (Vorsitzende/r)`, date, employer)},
		},
		Note: "Holen Sie vor Verhandlung eine Datenschutz-Folgenabschätzung (DSGVO Art. 35) beim Datenschutzbeauftragten ein. BR hat Recht, einen Sachverständigen hinzuzuziehen (§ 80 Abs. 3 BetrVG — auf Kosten des AG).",
	}
}

func bvTemplateArbeitszeit(employer, date string) *bvTemplateResult {
	return &bvTemplateResult{
		Topic:      "arbeitszeit",
		Title:      "Betriebsvereinbarung Arbeitszeit / Gleitzeitregelung",
		LegalBasis: "§ 87 Abs. 1 Nr. 2 und 3 BetrVG; ArbZG; ggf. Tarifvertrag",
		Sections: []bvSection{
			{1, "Geltungsbereich", "Diese BV gilt für alle Arbeitnehmer der " + employer + " mit Ausnahme von [leitende Angestellte i.S.d. § 5 Abs. 3 BetrVG / Außendienstmitarbeiter mit freier Zeiteinteilung]."},
			{2, "Regelarbeitszeit", `Die wöchentliche Regelarbeitszeit beträgt [X] Stunden, verteilt auf [5] Arbeitstage.
Tägliche Normalarbeitszeit: [X,X] Stunden.

[HINWEIS: ArbZG §§ 3, 4: max. 8h/Tag, Verlängerung auf 10h nur wenn Ausgleich innerhalb 6 Monaten; 11h Ruhezeit zwischen Schichten (§ 5 ArbZG).]`},
			{3, "Gleitzeit", `Im Gleitzeitrahmen können Arbeitnehmer Beginn und Ende der täglichen Arbeitszeit eigenverantwortlich bestimmen:

  Gleitzeitrahmen: [06:00] bis [20:00] Uhr
  Kernarbeitszeit (Anwesenheitspflicht): [09:00] bis [15:00] Uhr
  Mittagspause: mindestens [30 Minuten] ab [11:30] Uhr (ArbZG § 4)

[TIPP BR: Kernarbeitszeit möglichst kurz halten — schützt Beschäftigte mit Betreuungspflichten.]`},
			{4, "Arbeitszeitkonto / Gleitzeitsaldo", `Plusstunden: Übertrag bis maximal [+40] Stunden zum Monatsende zulässig.
Minusstunden: Maximal [-20] Stunden zulässig.

Abbau von Plusstunden: Freizeitausgleich hat Vorrang vor Auszahlung.
Verfallsregelung: Plusstunden über der Kappungsgrenze verfallen [nicht / nach [X] Monaten].

[TIPP BR: Verfall von angesammelten Stunden am Ende des Ausgleichszeitraums ausschließen oder Auszahlung sicherstellen.]`},
			{5, "Überstunden", `Überstunden sind Arbeitsstunden, die über die tägliche Sollarbeitszeit hinausgehen und vom Vorgesetzten ausdrücklich angeordnet oder nachträglich genehmigt wurden.

Freiwillige Mehrarbeit ohne Anordnung begründet keinen Entgeltanspruch [soweit einzelvertraglich vereinbart].

Vergütung: [Zuschlag X% / Freizeitausgleich 1:1 / Freizeitausgleich 1:1,25]

[BR-RECHT: Anordnung von Überstunden bedarf der Zustimmung des BR (§ 87 Abs. 1 Nr. 3 BetrVG).]`},
			{6, "Arbeitszeiterfassung", `Die Arbeitszeit wird erfasst durch: [Stechuhr / elektronisches System / Selbsteintrag]

Aufzeichnungspflicht: [AG / AN / beides]
Einsichtnahme: Jede/r AN kann jederzeit den eigenen Zeitnachweis einsehen.
Korrektur von Fehlbuchungen: [Verfahren beschreiben]

[TIPP BR: EuGH-Entscheidung C-55/18 — AG ist zur Arbeitszeiterfassung verpflichtet. Nutzen Sie dies als Verhandlungsgrundlage.]`},
			{7, "Schlussbestimmungen", fmt.Sprintf(`Diese BV tritt am %s in Kraft. Kündigung mit [3-monatiger] Frist zum Quartalsende.
Nachwirkung gemäß § 77 Abs. 6 BetrVG.

Ort, Datum: ___________

%s                              Der Betriebsrat
(Arbeitgeber)                  (Vorsitzende/r)`, date, employer)},
		},
		Note: "Prüfen Sie zunächst, ob ein Tarifvertrag die Arbeitszeit bereits regelt (Tarifvorbehalt § 77 Abs. 3 BetrVG). Eine BV darf nicht zuungunsten des TV abweichen.",
	}
}

func bvTemplateDatenschutz(employer, date string) *bvTemplateResult {
	return &bvTemplateResult{
		Topic:      "datenschutz",
		Title:      "Betriebsvereinbarung Beschäftigtendatenschutz",
		LegalBasis: "§ 26 BDSG; DSGVO Art. 88; § 87 Abs. 1 Nr. 6 BetrVG",
		Sections: []bvSection{
			{1, "Zweck und Geltungsbereich", "Diese BV konkretisiert den Datenschutz für die Verarbeitung personenbezogener Daten von Beschäftigten bei " + employer + " im Beschäftigungskontext (§ 26 BDSG)."},
			{2, "Grundsätze der Datenverarbeitung", `Personenbezogene Daten von Beschäftigten werden nur verarbeitet, wenn:
  a) die Verarbeitung für die Begründung, Durchführung oder Beendigung des Beschäftigungsverhältnisses erforderlich ist (§ 26 Abs. 1 BDSG), oder
  b) die betroffene Person eingewilligt hat (freiwillige Einwilligung; nicht erzwingbar), oder
  c) ein gesetzlicher Erlaubnistatbestand besteht.

Daten werden nur zum angegebenen Zweck verarbeitet (Zweckbindung). Eine Profilbildung ohne konkreten Anlass ist untersagt.`},
			{3, "Kategorien verarbeiteter Daten", `Folgende Datenkategorien werden verarbeitet:
  - Stammdaten: Name, Adresse, Geburtsdatum, Bankverbindung
  - Beschäftigungsdaten: Eintrittsdatum, Stellenbezeichnung, Abteilung, Entgelt
  - Leistungsdaten: [nur aggregiert / nicht erhoben]
  - Gesundheitsdaten: nur soweit für Entgeltfortzahlung oder Arbeitsschutz erforderlich
  - IT-Nutzungsdaten: [gemäß separater BV IT-Systeme]

[KRITISCH: Gesundheitsdaten sind besondere Kategorie (DSGVO Art. 9) — erhöhte Schutzanforderungen.]`},
			{4, "Rechte der Beschäftigten", `Beschäftigte haben folgende Rechte gegenüber dem Arbeitgeber:
  - Auskunft über verarbeitete Daten (DSGVO Art. 15)
  - Berichtigung unrichtiger Daten (DSGVO Art. 16)
  - Löschung nach Ablauf der Aufbewahrungsfristen (DSGVO Art. 17)
  - Widerspruch bei Verarbeitung auf Basis berechtigter Interessen (DSGVO Art. 21)

Ansprechpartner: [Datenschutzbeauftragter, Kontakt]`},
			{5, "Aufbewahrungsfristen", `  - Entgeltdaten: 6 Jahre (§ 257 HGB)
  - Personalunterlagen allgemein: 3 Jahre nach Ausscheiden
  - Bewerberdaten (Abgelehnte): 6 Monate nach Ablehnung
  - Krankheitsdaten: nach Ende Lohnfortzahlungszeitraum löschen
  - Video-/Kameraaufnahmen: max. [72 Stunden / 7 Tage]

Nach Ablauf der Fristen sind Daten zu löschen oder zu anonymisieren.`},
			{6, "Rolle des Betriebsrats", `Der Betriebsrat:
  a) ist vor Einführung neuer Datenverarbeitungssysteme zu unterrichten und hat Mitbestimmungsrecht (§ 87 Abs. 1 Nr. 6 BetrVG),
  b) kann Auskunft über Verarbeitungen verlangen (§ 80 BetrVG),
  c) kann Datenschutzverstöße beim betrieblichen Datenschutzbeauftragten und der Aufsichtsbehörde melden,
  d) unterliegt selbst der Geheimhaltungspflicht (§ 79 BetrVG) für personenbezogene Daten, die ihm im Rahmen seiner Tätigkeit bekannt werden.`},
			{7, "Schlussbestimmungen", fmt.Sprintf(`Diese BV tritt am %s in Kraft.

Ort, Datum: ___________

%s                              Der Betriebsrat
(Arbeitgeber)                  (Vorsitzende/r)`, date, employer)},
		},
		Note: "Holen Sie die Einschätzung des betrieblichen Datenschutzbeauftragten ein, bevor Sie diese BV unterzeichnen. Der DSB hat ein Recht auf Konsultation nach DSGVO Art. 38.",
	}
}

func bvTemplateVideoüberwachung(employer, date string) *bvTemplateResult {
	return &bvTemplateResult{
		Topic:      "videoüberwachung",
		Title:      "Betriebsvereinbarung Videoüberwachung",
		LegalBasis: "§ 87 Abs. 1 Nr. 6 BetrVG; § 4 BDSG; DSGVO Art. 6 Abs. 1 lit. f, Art. 13",
		Sections: []bvSection{
			{1, "Geltungsbereich", "Diese Betriebsvereinbarung gilt für alle Bereiche der " + employer + ", in denen Videokameras oder vergleichbare optische Überwachungseinrichtungen betrieben werden."},
			{2, "Zweck und Grundsatz", `Videoüberwachung ist nur zulässig für folgende klar definierte Zwecke:
  a) Schutz von Personen und Sachwerten in sicherheitsrelevanten Bereichen ([Kassenbereiche / Eingangsbereiche / ...])
  b) Zugangskontrolle zu besonders gesicherten Bereichen

Die Überwachung des Arbeitsverhaltens oder der Leistung von Arbeitnehmern ist AUSDRÜCKLICH AUSGESCHLOSSEN.

[VERHANDLUNGSTIPP: Auf einem abschließenden Zweckkatalog bestehen — vage Formulierungen wie "Betriebssicherheit allgemein" ermöglichen schleichende Ausweitung.]`},
			{3, "Technische Spezifikation", `Folgende Kameras sind zulässig:
  • Anzahl: [X] Kameras an folgenden fest definierten Standorten: [Liste der genauen Positionen]
  • Keine Schwenk-/Zoom-Funktion, die gezieltes Verfolgen einzelner Personen ermöglicht
  • Keine Ton-/Audioaufzeichnung
  • Auflösung: [maximal X Megapixel]

Der BR wird vorab über jede Änderung (Standort, Anzahl, Technik) unterrichtet und seine Zustimmung eingeholt.`},
			{4, "Datenspeicherung und -löschung", `Aufgezeichnetes Bildmaterial wird automatisch nach [72 Stunden / maximal 7 Tagen] gelöscht, sofern kein konkreter Anlassfall vorliegt.

Bei Anlassfall (konkreter Verdacht einer Straftat oder eines schwerwiegenden Regelverstoßes):
  • Sicherung nur der betroffenen Sequenz
  • Maximale Aufbewahrung: 30 Tage
  • Zugriff nur durch: [Sicherheitsbeauftragten + BR-Vorsitzenden + ggf. Strafverfolgungsbehörden]

[SCHUTZKLAUSEL: BR-Mitzeichnungspflicht bei jeder Datensicherung/Auswertung sicherstellt.]`},
			{5, "Zugriff und Auswertung", `Zugriff auf gespeichertes Bildmaterial haben ausschließlich:
  a) [Sicherheitsbeauftragter / IT-Leiter] — nur zur Systemwartung und im Anlassfall
  b) Geschäftsführung — nur bei konkretem Anlassfall und nach schriftlicher Unterrichtung des BR

Eine Nutzung zur Leistungs- oder Verhaltenskontrolle von Arbeitnehmern ist verboten.
Jeder Zugriff wird in einem Zugangsprotokoll dokumentiert, das dem BR zugänglich ist.`},
			{6, "Transparenz und Betroffenenrechte", `  a) Alle Bereiche mit Videoüberwachung werden deutlich sichtbar mit dem Piktogramm und dem Text "Videoüberwachung gemäß DSGVO / BV Videoüberwachung [Datum]" gekennzeichnet.
  b) Eine vollständige Datenschutzinformation nach DSGVO Art. 13 ist auszuhängen und im Intranet zu veröffentlichen.
  c) Beschäftigte können beim DSB oder BR Auskunft über Auswertungen verlangen, die ihre Person betreffen.`},
			{7, "Rolle des Betriebsrats", `  a) Der BR ist bei jeder Änderung der Überwachungsinfrastruktur vorab zu unterrichten und seine Zustimmung einzuholen (§ 87 Abs. 1 Nr. 6 BetrVG).
  b) Jede Auswertung, die einen Bezug zu namentlich bestimmbaren Arbeitnehmern hat, ist dem BR unverzüglich anzuzeigen.
  c) Bei Verdacht auf unzulässige Nutzung kann der BR die Einigungsstelle anrufen.`},
			{8, "Schlussbestimmungen", fmt.Sprintf(`Diese BV tritt am %s in Kraft. Sie kann mit einer Frist von 3 Monaten zum Monatsende gekündigt werden.

Bei Kündigung sind alle gespeicherten personenbezogenen Daten unverzüglich zu löschen.

Ort, Datum: ___________

%s                              Der Betriebsrat
(Arbeitgeber)                  (Vorsitzende/r)`, date, employer)},
		},
		Note: "Videoüberwachung von Arbeitnehmern ohne BV ist unzulässig (§ 87 Abs. 1 Nr. 6 BetrVG). Ohne Einigung muss die Einigungsstelle entscheiden. Achten Sie auf DSGVO-Datenschutz-Folgenabschätzung (Art. 35) bei umfangreicher Überwachung.",
	}
}

func bvTemplateLeistungsbeurteilung(employer, date string) *bvTemplateResult {
	return &bvTemplateResult{
		Topic:      "leistungsbeurteilung",
		Title:      "Betriebsvereinbarung Leistungsbeurteilung und Zielvereinbarung",
		LegalBasis: "§ 94 BetrVG (Beurteilungsgrundsätze); § 87 Abs. 1 Nr. 1 BetrVG; § 75 Abs. 2 BetrVG",
		Sections: []bvSection{
			{1, "Geltungsbereich", "Diese Betriebsvereinbarung gilt für alle Arbeitnehmerinnen und Arbeitnehmer der " + employer + ", die einer regelmäßigen Leistungsbeurteilung unterliegen. Ausgenommen sind leitende Angestellte i.S.v. § 5 Abs. 3 BetrVG."},
			{2, "Grundsätze", `Leistungsbeurteilungen werden nach folgenden verbindlichen Grundsätzen durchgeführt:
  a) Transparenz: Alle Kriterien, ihre Gewichtung und das Bewertungsverfahren sind den Arbeitnehmern vorab bekannt.
  b) Fairness: Gleiche Maßstäbe für vergleichbare Tätigkeiten und Funktionen.
  c) Nachvollziehbarkeit: Jede Bewertung ist mit konkreten Beispielen und Beobachtungen zu begründen.
  d) Schutz vor Diskriminierung: Beurteilungen dürfen keine geschützten Merkmale (Geschlecht, Herkunft, Alter, Behinderung etc.) berücksichtigen.

[SCHUTZKLAUSEL: BR hat Mitbestimmung bei den Beurteilungsgrundsätzen nach § 94 BetrVG — nutzen Sie das, um Diskriminierungspotenzial strukturell auszuschließen.]`},
			{3, "Beurteilungskriterien", `Folgende Kriterien werden bewertet (Gewichtung in Klammern):

Pflichtkriterien:
  • Ergebnisse / Zielerreichung: [X %]
  • Arbeitsqualität: [X %]
  • Arbeitsweise / Kompetenzen: [X %]

Optionale Kriterien (nur nach gemeinsamer Festlegung):
  • Führungsverhalten (nur für Führungskräfte): [X %]
  • [weitere Kriterien: ...]

Ausdrücklich NICHT zulässig als Beurteilungskriterium:
  • Krankheitszeiten / Anwesenheit (BAG-Rechtsprechung)
  • Betriebsratstätigkeit (§ 78 Satz 2 BetrVG)
  • Gewerkschaftszugehörigkeit`},
			{4, "Beurteilungsverfahren", `  a) Beurteilungsgespräch: Mindestens einmal jährlich führt die direkte Führungskraft ein Beurteilungsgespräch. Der Arbeitnehmer erhält das ausgefüllte Beurteilungsformular mindestens [5] Werktage vor dem Gespräch.
  b) Selbstbeurteilung: Jede/r Arbeitnehmer/in kann eine Selbsteinschätzung einreichen, die in die Akte aufgenommen wird.
  c) Widerspruchsrecht: Arbeitnehmer können innerhalb von [14] Tagen schriftlich widersprechen. Widersprüche werden von [Personalleitung + nächsthöherer Führungskraft] geprüft.
  d) BR-Beschwerde: Bleibt der Widerspruch erfolglos, steht der Weg zur Beschwerde beim Betriebsrat offen (§ 85 BetrVG).`},
			{5, "Zielvereinbarung", `  a) Ziele werden gemeinsam zwischen Arbeitnehmer und Führungskraft vereinbart — einseitige Zielvorgaben sind unzulässig.
  b) Ziele müssen SMART sein (spezifisch, messbar, erreichbar, relevant, terminiert).
  c) Ziele dürfen nicht nachträglich ohne Zustimmung des Arbeitnehmers verändert werden.
  d) Außerordentliche Ereignisse (Elternzeit, Langzeiterkrankung, Kurzarbeit, unvorhergesehene Betriebsänderungen), die die Zielerreichung behindern, führen zur anteiligen Anpassung, nicht zur Abwertung.

[SCHUTZKLAUSEL: Klauseln zu Bonuskürzung / variablem Entgelt in einer separaten Entgelt-BV regeln — hier nur Beurteilungsverfahren.]`},
			{6, "Datenschutz und Aufbewahrung", `  a) Beurteilungsunterlagen werden nur in der Personalakte gespeichert und unterliegen dem Datenschutz.
  b) Aufbewahrungsfrist: [3 Jahre] nach Beendigung des Arbeitsverhältnisses, danach Löschung.
  c) Einsichtsrecht: Arbeitnehmer haben jederzeit das Recht auf vollständige Einsicht in ihre Beurteilungsunterlagen (§ 83 BetrVG).
  d) Weitergabe an Dritte (intern oder extern) nur mit schriftlicher Zustimmung des Arbeitnehmers.`},
			{7, "Beteiligung des Betriebsrats", `  a) Änderungen der Beurteilungsgrundsätze, -kriterien oder des Verfahrens bedürfen der Zustimmung des Betriebsrats (§ 94 BetrVG).
  b) Der BR erhält jährlich eine anonymisierte Auswertung der Beurteilungsergebnisse (Verteilung der Noten je Abteilung, disaggregiert nach Geschlecht).
  c) Bei systematischen Auffälligkeiten (z.B. auffällig schlechtere Beurteilungen einer Beschäftigtengruppe) ist der BR unverzüglich zu unterrichten.`},
			{8, "Schlussbestimmungen", fmt.Sprintf(`Diese BV tritt am %s in Kraft.

Ort, Datum: ___________

%s                              Der Betriebsrat
(Arbeitgeber)                  (Vorsitzende/r)`, date, employer)},
		},
		Note: "§ 94 BetrVG gibt dem BR echtes Mitbestimmungsrecht bei Beurteilungsgrundsätzen — nutzen Sie es, um Fairness strukturell zu verankern. Besonderes Augenmerk: Beurteilungen dürfen nie Grundlage für Kündigungen sein, ohne die regulären Kündigungsschutzregeln zu beachten.",
	}
}
