package cli

import (
	"github.com/googlarz/betriebsrat/internal/betrvg"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"strings"
)

type decisionResult struct {
	Situation      string           `json:"situation"`
	Classification string           `json:"classification"`
	BRRights       []decisionRight  `json:"br_rights"`
	ActionPlan     []decisionAction `json:"action_plan"`
	Deadlines      []deadlineResult `json:"deadlines,omitempty"`
	Resources      []string         `json:"resources,omitempty"`
}

type decisionRight struct {
	Right     string `json:"right"`
	Paragraph int    `json:"paragraph"`
	Strength  string `json:"strength"`
}

type decisionAction struct {
	Priority string `json:"priority"`
	Action   string `json:"action"`
	Law      string `json:"law,omitempty"`
}

func newDecideCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "decide [situation]",
		Short: "Multi-step decision support: rights, action plan, and deadlines for a situation",
		Long: `Provides comprehensive decision support for a Betriebsrat situation:
1. Classifies the situation type
2. Identifies applicable BetrVG rights
3. Recommends a prioritized action plan
4. Surfaces relevant deadlines

Use for complex situations where you need a complete picture quickly.`,
		Example: strings.Trim(`
  betriebsrat decide "Arbeitgeber kündigt 15 Mitarbeiter"
  betriebsrat decide "Einführung KI-Schreibassistent für alle Mitarbeiter" --json
  betriebsrat decide "Betrieb soll nach München verlagert werden" --agent`, "\n"),
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
			words := tokenize(situation)
			paragraphs := betrvg.ByKeywords(words)

			result := decisionResult{
				Situation: situation,
			}

			// Classify the situation
			result.Classification = classifySituation(flags.lang, situation)

			// Build rights list
			seen := map[betrvg.CoDeterminationType]bool{}
			for _, p := range paragraphs {
				if !seen[p.CoDetermType] {
					result.BRRights = append(result.BRRights, decisionRight{
						Right:     p.Title,
						Paragraph: p.Number,
						Strength:  string(p.CoDetermType),
					})
					seen[p.CoDetermType] = true
				}
			}

			// Build action plan
			strongest := findStrongestRight(paragraphs)
			result.ActionPlan = buildActionPlan(flags.lang, strongest, situation, paragraphs)

			// Check for deadlines
			for _, rule := range betrvg.Deadlines() {
				for _, w := range words {
					if betrvg.ContainsFold(rule.Situation, w) || betrvg.ContainsFold(w, rule.Situation) {
						result.Deadlines = append(result.Deadlines, deadlineResult{
							Situation:   rule.Situation,
							Paragraph:   rule.Paragraph,
							Days:        rule.Days,
							Description: rule.Description,
							Note:        rule.Note,
						})
						break
					}
				}
			}

			// Resource links
			seenURLs := map[string]bool{}
			for _, p := range paragraphs {
				if p.TopicURL != "" && !seenURLs[p.TopicURL] {
					result.Resources = append(result.Resources, p.TopicURL)
					seenURLs[p.TopicURL] = true
				}
			}

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", tr(flags.lang, "Situation", "Situation"), result.Situation)
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n\n", tr(flags.lang, "Einordnung", "Classification"), result.Classification)

			if len(result.BRRights) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), tr(flags.lang, "Betriebsratsrechte:", "Works council rights:"))
				for _, r := range result.BRRights {
					fmt.Fprintf(cmd.OutOrStdout(), "  § %d %s — %s\n", r.Paragraph, r.Right, r.Strength)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}

			if len(result.ActionPlan) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), tr(flags.lang, "Empfohlene Maßnahmen:", "Recommended actions:"))
				for i, a := range result.ActionPlan {
					law := ""
					if a.Law != "" {
						law = fmt.Sprintf(" [%s]", a.Law)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  %d. [%s]%s %s\n", i+1, a.Priority, law, a.Action)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}

			if len(result.Deadlines) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), tr(flags.lang, "Fristen:", "Deadlines:"))
				for _, d := range result.Deadlines {
					if d.Days > 0 {
						fmt.Fprintf(cmd.OutOrStdout(), "  § %d: %d %s — %s\n", d.Paragraph, d.Days, tr(flags.lang, "Tage", "days"), d.Description)
					}
				}
			}

			return nil
		},
	}
	return cmd
}

func classifySituation(lang, situation string) string {
	low := strings.ToLower(situation)
	switch {
	// Technical systems checked first — "co-determination" must not fall into dismissal via "termination"
	case containsAny(low, "software", "analytics", "überwachung", "monitoring", "ki-system", "ki system",
		"künstliche intelligenz", "system einführung", "artificial intelligence", "surveillance", "tracking"):
		return tr(lang, "Technische Einrichtung – Mitbestimmung nach § 87 Abs. 1 Nr. 6", "Technical facility – Co-determination under § 87 Abs. 1 Nr. 6")
	case containsAny(low, "massenentlassung", "sozialplan", "mass dismissal", "mass redundancy", "mass layoff", "layoff"):
		return tr(lang, "Massenentlassung / Sozialplan (§ 112 BetrVG, § 17 KSchG)", "Mass dismissal / Sozialplan (§ 112 BetrVG, § 17 KSchG)")
	case containsAny(low, "kündigung", "entlassung", "fristlos", "dismissal", "dismiss", "dismissing", "fired", "firing") ||
		containsWholeWord(low, "termination"):
		return tr(lang, "Personelle Angelegenheit – Kündigung", "Personnel matter – Dismissal")
	case containsAny(low, "betriebsänderung", "verlagerung", "stilllegung", "umstrukturierung", "outsourcing", "restructuring", "relocation", "closure"):
		return tr(lang, "Betriebsänderung (§ 111 ff. BetrVG)", "Operational change (§ 111 ff. BetrVG)")
	case containsAny(low, "homeoffice", "remote", "mobiles arbeiten", "telearbeit", "working from home", "mobile work"):
		return tr(lang, "Arbeitszeitregelung / Mobiles Arbeiten – § 87 Abs. 1 Nr. 1, 2", "Working time / Mobile work – § 87 Abs. 1 Nr. 1, 2")
	case containsAny(low, "einstellung", "versetzung", "umgruppierung", "hiring", "transfer", "regrading"):
		return tr(lang, "Personelle Einzelmaßnahme (§ 99 BetrVG)", "Individual personnel measure (§ 99 BetrVG)")
	default:
		return tr(lang, "Allgemeine Betriebsratsangelegenheit", "General works council matter")
	}
}

func containsAny(s string, terms ...string) bool {
	for _, t := range terms {
		if strings.Contains(s, t) {
			return true
		}
	}
	return false
}

// containsWholeWord checks that word appears surrounded by non-letter characters.
func containsWholeWord(s, word string) bool {
	idx := strings.Index(s, word)
	if idx < 0 {
		return false
	}
	before := idx > 0 && isWordRune(rune(s[idx-1]))
	after := idx+len(word) < len(s) && isWordRune(rune(s[idx+len(word)]))
	return !before && !after
}

func isWordRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func buildActionPlan(lang string, strongest betrvg.CoDeterminationType, situation string, paragraphs []betrvg.Paragraph) []decisionAction {
	var plan []decisionAction

	plan = append(plan, decisionAction{
		Priority: tr(lang, "sofort", "immediate"),
		Action:   tr(lang, "Vollständige Unterrichtung durch Arbeitgeber verlangen (schriftlich, mit Fristsetzung)", "Demand full written disclosure from the employer (in writing, with a deadline)"),
		Law:      "§ 80 Abs. 2 BetrVG",
	})

	switch strongest {
	case betrvg.MitbestimmungErzwingbar:
		plan = append(plan,
			decisionAction{tr(lang, "sofort", "immediate"), tr(lang, "Maßnahme stoppen: Arbeitgeber darf ohne BR-Zustimmung oder Einigungsstelle nicht handeln", "Stop the measure: employer may not act without BR consent or conciliation board ruling"), "§ 87 / § 99 BetrVG"},
			decisionAction{tr(lang, "kurzfristig", "short-term"), tr(lang, "Betriebsvereinbarung verhandeln oder Einigungsstelle anrufen", "Negotiate a Betriebsvereinbarung or invoke the conciliation board"), "§ 76 BetrVG"},
			decisionAction{tr(lang, "kurzfristig", "short-term"), tr(lang, "Stellungnahme schriftlich und fristgerecht abgeben", "Submit written statement within the statutory deadline"), ""},
			decisionAction{tr(lang, "mittelfristig", "medium-term"), tr(lang, "Sachverständigen hinzuziehen falls technische Fragen offen sind", "Engage an expert if technical questions remain open"), "§ 80 Abs. 3 BetrVG"},
		)
	case betrvg.Zustimmung:
		plan = append(plan,
			decisionAction{tr(lang, "sofort", "immediate"), tr(lang, "Frist prüfen: Zustimmung oder schriftliche Verweigerung binnen 1 Woche", "Check deadline: consent or written refusal within 1 week"), "§ 99 Abs. 3"},
			decisionAction{tr(lang, "kurzfristig", "short-term"), tr(lang, "Verweigerungsgründe prüfen und ggf. schriftlich begründen", "Review grounds for refusal and document in writing if applicable"), "§ 99 Abs. 2"},
		)
	case betrvg.Mitwirkung:
		plan = append(plan,
			decisionAction{tr(lang, "kurzfristig", "short-term"), tr(lang, "Schriftliche Stellungnahme mit konkreten Einwänden einreichen", "Submit written statement with specific objections"), ""},
			decisionAction{tr(lang, "mittelfristig", "medium-term"), tr(lang, "Alternativen und Gegenvorschläge schriftlich unterbreiten", "Submit alternatives and counter-proposals in writing"), ""},
		)
	case betrvg.Beratung:
		plan = append(plan,
			decisionAction{tr(lang, "kurzfristig", "short-term"), tr(lang, "Ernsthafte Beratung einfordern — nicht nur formale Unterrichtung akzeptieren", "Demand genuine consultation — do not accept mere formal notification"), "§ 74 BetrVG"},
		)
	}

	if cl := betrvg.GetChecklist(situation); cl != nil {
		plan = append(plan, decisionAction{
			Priority: tr(lang, "hinweis", "note"),
			Action:   fmt.Sprintf(tr(lang, "Vollständige Checkliste verfügbar: `betriebsrat checklist %q`", "Full checklist available: `betriebsrat checklist %q`"), cl.Situation),
		})
	}

	return plan
}
