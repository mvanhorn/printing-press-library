package cli

import (
	"github.com/googlarz/betriebsrat/internal/betrvg"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"strings"
)

type rightsCheckResult struct {
	Situation      string                 `json:"situation"`
	Summary        string                 `json:"summary"`
	HasRight       bool                   `json:"has_right"`
	Paragraphs     []rightsCheckParagraph `json:"paragraphs,omitempty"`
	Recommendation string                 `json:"recommendation"`
	TopicURL       string                 `json:"topic_url,omitempty"`
}

type rightsCheckParagraph struct {
	Paragraph int    `json:"paragraph"`
	Title     string `json:"title"`
	Summary   string `json:"summary"`
	RightType string `json:"right_type"`
	TopicURL  string `json:"topic_url,omitempty"`
}

func newRightsCheckCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rights-check [situation]",
		Short: "Check if the Betriebsrat has co-determination rights in a situation",
		Long: `Looks up applicable BetrVG paragraphs for a given workplace situation
and classifies the Betriebsrat's rights as: Mitbestimmung (erzwingbar),
Mitwirkung, Unterrichtung, Beratung, Zustimmungsvorbehalt, or keine.

This is the single most common question for works council members:
"Haben wir ein Mitbestimmungsrecht?" / "Does the employer need our consent?"`,
		Example: strings.Trim(`
  betriebsrat rights-check "Arbeitgeber will Überwachungssoftware einführen"
  betriebsrat rights-check "Kündigung eines Mitarbeiters" --json
  betriebsrat rights-check "Homeoffice Regelung" --agent`, "\n"),
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

			situation := strings.Join(args, " ")
			// Extract keywords from the situation description
			words := tokenize(situation)
			paragraphs := betrvg.ByKeywords(words)

			result := rightsCheckResult{
				Situation: situation,
			}

			if len(paragraphs) == 0 {
				result.Summary = tr(flags.lang,
					"Keine spezifischen BetrVG-Paragrafen für diese Situation gefunden.",
					"No specific BetrVG paragraphs found for this situation.")
				result.HasRight = false
				result.Recommendation = tr(flags.lang,
					"Prüfen Sie die Situation mit einem Fachanwalt für Arbeitsrecht oder konsultieren Sie betriebsrat.de für allgemeine Themen.",
					"Review the situation with a labour law specialist or consult betriebsrat.de for general topics.")
			} else {
				// Find the strongest right
				strongest := findStrongestRight(paragraphs)
				result.HasRight = strongest != betrvg.Keine
				result.Summary = buildRightsSummary(flags.lang, strongest, paragraphs)
				result.Recommendation = buildRecommendation(flags.lang, strongest, paragraphs)
				if len(paragraphs) > 0 {
					result.TopicURL = paragraphs[0].TopicURL
				}

				for _, p := range paragraphs {
					result.Paragraphs = append(result.Paragraphs, rightsCheckParagraph{
						Paragraph: p.Number,
						Title:     p.Title,
						Summary:   p.Summary,
						RightType: string(p.CoDetermType),
						TopicURL:  p.TopicURL,
					})
				}
			}

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n\n", tr(flags.lang, "Situation", "Situation"), result.Situation)
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n\n", tr(flags.lang, "Ergebnis", "Result"), result.Summary)
			if len(result.Paragraphs) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "%s:\n", tr(flags.lang, "Anwendbare Paragrafen", "Applicable paragraphs"))
				for _, p := range result.Paragraphs {
					fmt.Fprintf(cmd.OutOrStdout(), "  § %d %s — %s\n", p.Paragraph, p.Title, p.RightType)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", tr(flags.lang, "Empfehlung", "Recommendation"), result.Recommendation)
			if result.TopicURL != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", tr(flags.lang, "Mehr Infos", "More info"), result.TopicURL)
			}
			return nil
		},
	}
	return cmd
}

// stopwords are common German workplace nouns that appear in user input
// but are too generic to indicate a specific BetrVG right.
var stopwords = map[string]bool{
	"mitarbeiter": true, "arbeitnehmer": true, "arbeitgeber": true,
	"angestellter": true, "angestellte": true, "beschäftigter": true,
	"chef": true, "vorgesetzter": true, "firma": true, "unternehmen": true,
	"betrieb": true, "abteilung": true, "eine": true, "einen": true,
	"einem": true, "durch": true, "nach": true, "über": true, "beim": true,
	"beim betrieb": true, "neue": true, "neuen": true, "neuer": true,
}

func tokenize(s string) []string {
	words := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return r == ' ' || r == ',' || r == '.' || r == '?' || r == '!'
	})
	var result []string
	for _, w := range words {
		if len(w) > 3 && !stopwords[w] {
			result = append(result, w)
		}
	}
	// Also add bigrams (pairs of consecutive words) for better matching
	for i := 0; i < len(words)-1; i++ {
		bigram := words[i] + " " + words[i+1]
		if !stopwords[bigram] {
			result = append(result, bigram)
		}
	}
	return result
}

func findStrongestRight(paragraphs []betrvg.Paragraph) betrvg.CoDeterminationType {
	order := []betrvg.CoDeterminationType{
		betrvg.MitbestimmungErzwingbar,
		betrvg.Zustimmung,
		betrvg.Mitwirkung,
		betrvg.Beratung,
		betrvg.Unterrichtung,
		betrvg.Keine,
	}
	best := betrvg.Keine
	for _, p := range paragraphs {
		for i, o := range order {
			if p.CoDetermType == o {
				if i < indexOf(order, best) {
					best = o
				}
				break
			}
		}
	}
	return best
}

func indexOf(slice []betrvg.CoDeterminationType, val betrvg.CoDeterminationType) int {
	for i, v := range slice {
		if v == val {
			return i
		}
	}
	return len(slice)
}

func buildRightsSummary(lang string, strongest betrvg.CoDeterminationType, paragraphs []betrvg.Paragraph) string {
	n := len(paragraphs)
	switch strongest {
	case betrvg.MitbestimmungErzwingbar:
		return tr(lang,
			fmt.Sprintf("✅ Der Betriebsrat hat erzwingbare Mitbestimmungsrechte (%d Paragraf(en) einschlägig). Der Arbeitgeber darf ohne Zustimmung des BR oder Spruch der Einigungsstelle nicht handeln.", n),
			fmt.Sprintf("✅ The works council has enforceable co-determination rights (%d paragraph(s) applicable). The employer may not act without BR consent or a ruling by the conciliation board.", n))
	case betrvg.Zustimmung:
		return tr(lang,
			fmt.Sprintf("✅ Der Betriebsrat hat Zustimmungsvorbehalt (%d Paragraf(en) einschlägig). Der Arbeitgeber braucht die Zustimmung des BR.", n),
			fmt.Sprintf("✅ The works council has consent veto (%d paragraph(s) applicable). The employer needs the BR's approval.", n))
	case betrvg.Mitwirkung:
		return tr(lang,
			fmt.Sprintf("⚠️  Der Betriebsrat hat Mitwirkungsrechte (%d Paragraf(en) einschlägig). Der AG muss den BR einbeziehen, behält aber die Entscheidung.", n),
			fmt.Sprintf("⚠️  The works council has participation rights (%d paragraph(s) applicable). The employer must involve the BR but retains the final decision.", n))
	case betrvg.Beratung:
		return tr(lang,
			fmt.Sprintf("ℹ️  Der Betriebsrat hat Beratungsrechte (%d Paragraf(en) einschlägig). Der AG muss beraten, ist aber nicht gebunden.", n),
			fmt.Sprintf("ℹ️  The works council has consultation rights (%d paragraph(s) applicable). The employer must consult but is not bound.", n))
	case betrvg.Unterrichtung:
		return tr(lang,
			fmt.Sprintf("ℹ️  Der Betriebsrat hat Unterrichtungsrecht (%d Paragraf(en) einschlägig). Der AG muss informieren.", n),
			fmt.Sprintf("ℹ️  The works council has an information right (%d paragraph(s) applicable). The employer must inform the BR.", n))
	default:
		return tr(lang,
			"❓ Kein spezifisches Betriebsratsrecht in der Datenbank gefunden.",
			"❓ No specific works council right found in the database.")
	}
}

func buildRecommendation(lang string, strongest betrvg.CoDeterminationType, _ []betrvg.Paragraph) string {
	switch strongest {
	case betrvg.MitbestimmungErzwingbar:
		return tr(lang,
			"Verhandeln Sie eine Betriebsvereinbarung. Bei Nichteinigung können Sie die Einigungsstelle anrufen. Der AG darf ohne Einigung nicht handeln.",
			"Negotiate a Betriebsvereinbarung. If no agreement is reached you can invoke the conciliation board (Einigungsstelle). The employer may not act unilaterally.")
	case betrvg.Zustimmung:
		return tr(lang,
			"Prüfen Sie die Zustimmungsvoraussetzungen. Lehnen Sie die Zustimmung bei Vorliegen von Verweigerungsgründen schriftlich und begründet innerhalb der Frist ab.",
			"Check the consent conditions. If grounds for refusal exist, reject consent in writing with reasons within the statutory deadline.")
	case betrvg.Mitwirkung:
		return tr(lang,
			"Nehmen Sie Ihr Mitwirkungsrecht wahr: Äußern Sie sich schriftlich, machen Sie Gegenvorschläge. Auch wenn Sie nicht blockieren können, schafft das Verhandlungsdruck.",
			"Exercise your participation right: submit written comments and counter-proposals. Even without veto power this creates negotiating leverage.")
	case betrvg.Beratung:
		return tr(lang,
			"Verlangen Sie ernsthafte Beratung. Dokumentieren Sie den Beratungsprozess. Bei unzureichender Information: schriftliche Nachfrage stellen.",
			"Demand genuine consultation. Document the consultation process. If information is insufficient: send a written follow-up request.")
	case betrvg.Unterrichtung:
		return tr(lang,
			"Verlangen Sie vollständige Unterrichtung schriftlich. Dokumentieren Sie, wenn der AG dieser Pflicht nicht nachkommt.",
			"Demand full written disclosure. Document any failure by the employer to meet this obligation.")
	default:
		return tr(lang,
			"Konsultieren Sie betriebsrat.de oder einen Fachanwalt für Arbeitsrecht für diese spezifische Situation.",
			"Consult betriebsrat.de or a labour law specialist for this specific situation.")
	}
}
