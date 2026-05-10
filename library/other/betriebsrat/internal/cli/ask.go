package cli

import (
	"github.com/googlarz/betriebsrat/internal/betrvg"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type askResult struct {
	Question       string     `json:"question"`
	Lang           string     `json:"lang"`
	UserRole       string     `json:"user_role"` // "employee" or "br_member"
	Classification string     `json:"classification"`
	RightType      string     `json:"right_type,omitempty"`
	Paragraphs     []askPara  `json:"paragraphs,omitempty"`
	Answer         string     `json:"answer"`
	Actions        []string   `json:"recommended_actions,omitempty"`
	Deadline       string     `json:"deadline,omitempty"`
	SozialplanHint string     `json:"sozialplan_hint,omitempty"`
	Disclaimer     string     `json:"disclaimer"`
	TopicURL       string     `json:"topic_url,omitempty"`
}

type askPara struct {
	Paragraph int    `json:"paragraph"`
	Title     string `json:"title"`
	RightType string `json:"right_type"`
}

func newAskCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ask [question]",
		Short: "Ask a question in plain language — no command knowledge required",
		Long: `Ask any workplace situation question in plain German or English.
The CLI detects whether you are an employee or a Betriebsrat member and
routes to the right legal analysis automatically.

Examples (employee):
  "Ich wurde entlassen. Hat der Betriebsrat mich angehört?"
  "Kann ich Sozialplan beanspruchen? Ich arbeite 8 Jahre und verdiene 4500 Euro."
  "My employer restructured without consulting the works council. What are my rights?"

Examples (BR member):
  "Arbeitgeber will KI-System einführen. Haben wir ein Mitbestimmungsrecht?"
  "Wir haben eine Anhörung für eine Kündigung erhalten. Was müssen wir tun?"
  "Does the employer need our consent for a mass layoff?"`,
		Example: strings.Trim(`
  betriebsrat ask "Ich wurde fristlos entlassen. Was nun?"
  betriebsrat ask "Employer introducing Teams analytics. Do we have co-determination?" --lang en
  betriebsrat ask "Wie viel Sozialplan bekomme ich bei 8 Jahren und 4500 Euro?" --json`, "\n"),
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

			question := strings.Join(args, " ")
			lang := flags.lang
			if lang == "de" {
				// Auto-detect English from the question itself
				if looksEnglish(question) {
					lang = "en"
				}
			}

			result := buildAskResult(lang, question)

			if flags.asJSON || flags.agent {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s\n\n", tr(lang, "Frage", "Question")+": "+result.Question)

			roleLabel := tr(lang, "Betriebsratsmitglied", "Works council member")
			switch result.UserRole {
			case "employee":
				roleLabel = tr(lang, "Arbeitnehmer", "Employee")
			case "curious":
				roleLabel = tr(lang, "Neu/Orientierung", "New / exploring")
			}
			fmt.Fprintf(w, "%s: %s\n", tr(lang, "Erkannte Rolle", "Detected role"), roleLabel)
			fmt.Fprintf(w, "%s: %s\n\n", tr(lang, "Einordnung", "Classification"), result.Classification)

			if result.RightType != "" {
				fmt.Fprintf(w, "%s: %s\n\n", tr(lang, "Mitbestimmungsart", "Co-determination type"), result.RightType)
			}

			if len(result.Paragraphs) > 0 {
				fmt.Fprintf(w, "%s:\n", tr(lang, "Anwendbare Paragrafen", "Applicable paragraphs"))
				for _, p := range result.Paragraphs {
					fmt.Fprintf(w, "  § %d %s — %s\n", p.Paragraph, p.Title, p.RightType)
				}
				fmt.Fprintln(w)
			}

			// Show disclaimer before the answer for high-stakes situations
			// (employee with a deadline or procedure check involved).
			highStakes := result.UserRole == "employee" && (result.Deadline != "" || result.SozialplanHint != "")
			if highStakes {
				fmt.Fprintf(w, "⚠️  %s\n\n", result.Disclaimer)
			}

			fmt.Fprintf(w, "%s\n\n", result.Answer)

			if len(result.Actions) > 0 {
				fmt.Fprintf(w, "%s:\n", tr(lang, "Empfohlene Schritte", "Recommended steps"))
				for i, a := range result.Actions {
					fmt.Fprintf(w, "  %d. %s\n", i+1, a)
				}
				fmt.Fprintln(w)
			}

			if result.Deadline != "" {
				fmt.Fprintf(w, "⏰ %s: %s\n", tr(lang, "Frist", "Deadline"), result.Deadline)
			}
			if result.SozialplanHint != "" {
				fmt.Fprintf(w, "\n💶 %s\n", result.SozialplanHint)
			}
			if result.TopicURL != "" {
				fmt.Fprintf(w, "\n%s: %s\n", tr(lang, "Mehr Infos", "More info"), result.TopicURL)
			}
			if !highStakes {
				fmt.Fprintf(w, "\n⚠️  %s\n", result.Disclaimer)
			}
			return nil
		},
	}
	return cmd
}

// buildAskResult is the pure logic behind the ask command — callable from serve.go.
func buildAskResult(lang, question string) askResult {
	role := detectRole(question)

	// Curious users get onboarding — skip rights lookup entirely.
	if role == "curious" {
		return askResult{
			Question:       question,
			Lang:           lang,
			UserRole:       "curious",
			Classification: tr(lang, "Grundlagenfrage – Betriebsrat-Einführung", "Introductory question – Works council basics"),
			Answer:         buildOnboardingAnswer(lang, question),
			Actions:        buildOnboardingActions(lang, question),
			Disclaimer: tr(lang,
				"Dies ist eine rechtliche Orientierungshilfe, kein Rechtsgutachten. Für Ihren konkreten Fall konsultieren Sie einen Fachanwalt für Arbeitsrecht.",
				"This is legal orientation only, not legal advice. Consult a labour law specialist for your specific situation."),
		}
	}

	words := tokenize(question)
	paras := betrvg.ByKeywords(words)
	strongest := betrvg.Keine
	if len(paras) > 0 {
		strongest = findStrongestRight(paras)
	}
	classification := classifySituation(lang, question)

	result := askResult{
		Question:       question,
		Lang:           lang,
		UserRole:       role,
		Classification: classification,
		Disclaimer: tr(lang,
			"Dies ist eine rechtliche Orientierungshilfe, kein Rechtsgutachten. Für Ihren konkreten Fall konsultieren Sie einen Fachanwalt für Arbeitsrecht.",
			"This is legal orientation only, not legal advice. Consult a labour law specialist for your specific situation."),
	}

	if len(paras) > 0 {
		result.RightType = string(strongest)
		for _, p := range paras {
			result.Paragraphs = append(result.Paragraphs, askPara{p.Number, p.Title, string(p.CoDetermType)})
		}
		result.TopicURL = paras[0].LegalSourceURL()
	} else {
		// No paragraphs matched by keyword, but classification may still be known.
		// Use a classification-based fallback answer.
		result.Answer = classificationFallback(lang, classification)
		result.Actions = []string{
			tr(lang,
				"Genaue Analyse: betriebsrat rights-check \""+question+"\"",
				"Precise analysis: betriebsrat rights-check \""+question+"\""),
			tr(lang,
				"Vollständige Entscheidungsunterstützung: betriebsrat decide \""+question+"\"",
				"Full decision support: betriebsrat decide \""+question+"\""),
		}
	}

	// Build audience-appropriate answer (only when paragraphs were found)
	if len(paras) > 0 {
		if role == "employee" {
			result.Answer = buildEmployeeAnswer(lang, question, strongest, paras, classification)
			result.Actions = buildEmployeeActions(lang, question, strongest, paras)
		} else {
			result.Answer = buildRightsSummary(lang, strongest, paras)
			plan := buildActionPlan(lang, strongest, question, paras)
			for _, a := range plan {
				law := ""
				if a.Law != "" {
					law = " [" + a.Law + "]"
				}
				result.Actions = append(result.Actions, "["+a.Priority+"]"+law+" "+a.Action)
			}
		}
	}

	// Deadline detection
	for _, rule := range betrvg.Deadlines() {
		for _, w := range words {
			if betrvg.ContainsFold(rule.Situation, w) || betrvg.ContainsFold(w, rule.Situation) {
				if rule.Days > 0 {
					result.Deadline = fmt.Sprintf("§ %d — %d %s: %s",
						rule.Paragraph, rule.Days,
						tr(lang, "Tage", "days"), rule.Description)
				}
				break
			}
		}
		if result.Deadline != "" {
			break
		}
	}

	// Sozialplan hint: extract salary + years if present
	if salary, years, ok := extractSozialplanNumbers(question); ok {
		estimate := years * salary * 0.75
		if role == "employee" {
			result.SozialplanHint = fmt.Sprintf(
				tr(lang,
					"Geschätzte Sozialplanabfindung (Münchner Formel, Faktor 0,75): %.0f € — genauer mit: betriebsrat sozialplan-calc --salary %.0f --years %.0f --factor 0.75",
					"Estimated Sozialplan entitlement (Munich formula, factor 0.75): €%.0f — refine with: betriebsrat sozialplan-calc --salary %.0f --years %.0f --factor 0.75"),
				estimate, salary, years)
		}
	}

	return result
}

func buildOnboardingAnswer(lang, question string) string {
	low := strings.ToLower(question)

	// "Do I have a BR / no BR" variant
	if containsAny(low, "habe ich einen betriebsrat", "do i have a works council",
		"gibt es einen betriebsrat", "is there a works council", "kein betriebsrat", "no works council") {
		return tr(lang,
			"Ein Betriebsrat muss nicht automatisch existieren — er muss von den Arbeitnehmern gewählt werden.\n\n"+
				"Ab 5 wahlberechtigten Arbeitnehmern in einem Betrieb kann ein Betriebsrat gewählt werden (§ 1 BetrVG). Ob es in Ihrem Betrieb einen gibt, erfahren Sie bei der Personalabteilung, am schwarzen Brett oder über Ihre Gewerkschaft.\n\n"+
				"Gibt es keinen BR? Die Gewerkschaft kann die Gründung unterstützen. Arbeitgeber dürfen die Wahl nicht behindern (Behinderung ist strafbar, § 119 BetrVG).",
			"A works council does not exist automatically — it must be elected by the employees.\n\n"+
				"A works council can be elected wherever 5 or more eligible employees work (§ 1 BetrVG). To find out if your company has one, check with HR, the notice board, or your trade union.\n\n"+
				"No works council? The union can support the founding process. Employers may not obstruct an election — doing so is a criminal offence (§ 119 BetrVG).")
	}

	// "What can / can't the BR do" variant
	if containsAny(low, "was kann der betriebsrat", "what can the works council", "was darf", "what may",
		"was macht", "what does") {
		return tr(lang,
			"Was der Betriebsrat für Sie tun kann:\n"+
				"• Muss bei jeder Kündigung angehört werden — ohne Anhörung ist die Kündigung unwirksam (§ 102 BetrVG)\n"+
				"• Kann Widerspruch gegen fehlerhafte Sozialauswahl einlegen → gibt Ihnen Recht auf Weiterbeschäftigung während eines Kündigungsschutzprozesses\n"+
				"• Erzwingt einen Sozialplan bei Massenentlassungen oder Betriebsänderungen (§ 112 BetrVG)\n"+
				"• Muss neuen Überwachungs- und KI-Systemen zustimmen (§ 87 Abs. 1 Nr. 6 BetrVG)\n"+
				"• Hat Auskunfts- und Einsichtsrechte gegenüber dem Arbeitgeber (§ 80 BetrVG)\n\n"+
				"Was der Betriebsrat nicht kann:\n"+
				"• Eine Kündigung direkt rückgängig machen — das ist Sache des Arbeitsgerichts\n"+
				"• Sie im Gerichtsverfahren vertreten — das übernehmen Gewerkschaft oder Anwalt\n"+
				"• Von sich aus tätig werden, wenn er nichts von Ihrer Situation weiß — informieren Sie ihn aktiv",
			"What the works council can do for you:\n"+
				"• Must be consulted before every dismissal — without consultation the dismissal is void (§ 102 BetrVG)\n"+
				"• Can object to incorrect social selection → gives you the right to continued employment during a court case\n"+
				"• Can enforce a Sozialplan in mass layoffs or restructurings (§ 112 BetrVG)\n"+
				"• Must approve new monitoring or AI systems (§ 87 Abs. 1 Nr. 6 BetrVG)\n"+
				"• Has information and inspection rights against the employer (§ 80 BetrVG)\n\n"+
				"What the works council cannot do:\n"+
				"• Directly reverse a dismissal — that is for the labour court\n"+
				"• Represent you in court proceedings — that is done by a union or lawyer\n"+
				"• Act on your behalf without knowing your situation — you must inform it proactively")
	}

	// Default: full intro
	return tr(lang,
		"Der Betriebsrat (BR) ist die gewählte Interessenvertretung der Arbeitnehmer im Betrieb — geregelt im Betriebsverfassungsgesetz (BetrVG).\n\n"+
			"Was der BR für Sie tun kann:\n"+
			"• Muss bei jeder Kündigung angehört werden (§ 102) — ohne Anhörung ist die Kündigung unwirksam\n"+
			"• Kann Sozialplan bei Stellenabbau erzwingen (§ 112)\n"+
			"• Muss neuen IT-/KI-Systemen und Überwachung zustimmen (§ 87 Nr. 6)\n"+
			"• Kann Widerspruch gegen falsche Sozialauswahl einlegen — gibt Ihnen Weiterbeschäftigungsrecht\n"+
			"• Handelt vertraulich: Betriebsratsmitglieder unterliegen der Schweigepflicht (§ 79 BetrVG)\n\n"+
			"Was der BR nicht kann:\n"+
			"• Eine Kündigung direkt rückgängig machen (das ist Aufgabe des Arbeitsgerichts)\n"+
			"• Sie im Prozess vertreten (das übernehmen Gewerkschaft oder Fachanwalt)\n"+
			"• Ohne Ihre Information tätig werden — er muss von Ihrer Situation wissen\n\n"+
			"Wann Sie den BR kontaktieren sollten:\n"+
			"• Sofort bei Erhalt einer Kündigung (3-Wochen-Klagefrist läuft ab Zugang!)\n"+
			"• Bei Versetzung, Umgruppierung oder Änderungskündigung\n"+
			"• Bei Einführung neuer Software, KI oder Überwachungssysteme\n"+
			"• Bei Restrukturierung, Betriebsänderung oder drohendem Stellenabbau\n\n"+
			"Kein BR im Betrieb? Ab 5 Arbeitnehmern kann einer gewählt werden (§ 1 BetrVG). Die Gewerkschaft hilft dabei.",
		"The Betriebsrat (BR) is the elected employee representation body in a German company — governed by the Works Constitution Act (BetrVG).\n\n"+
			"What the BR can do for you:\n"+
			"• Must be consulted before every dismissal (§ 102) — without consultation the dismissal is void\n"+
			"• Can enforce a Sozialplan (severance scheme) in layoffs (§ 112)\n"+
			"• Must approve new IT/AI systems and monitoring tools (§ 87 Nr. 6)\n"+
			"• Can object to incorrect social selection in dismissals — entitles you to continued employment\n"+
			"• Operates confidentially: BR members are bound by a duty of secrecy (§ 79 BetrVG)\n\n"+
			"What the BR cannot do:\n"+
			"• Directly reverse a dismissal (that is the labour court's job)\n"+
			"• Represent you in legal proceedings (that is done by a union or labour law specialist)\n"+
			"• Act without knowing your situation — you need to inform it\n\n"+
			"When to contact the BR:\n"+
			"• Immediately on receiving a dismissal (3-week claim deadline starts from receipt!)\n"+
			"• On a transfer, regrading, or amendment to your contract\n"+
			"• When new software, AI, or monitoring systems are being introduced\n"+
			"• When restructuring or layoffs are on the horizon\n\n"+
			"No works council at your company? One can be founded with as few as 5 employees (§ 1 BetrVG). A union can help.")
}

func buildOnboardingActions(lang, question string) []string {
	low := strings.ToLower(question)
	if containsAny(low, "kein betriebsrat", "no works council", "betriebsrat gründen", "elect a works council", "found a works council") {
		return []string{
			tr(lang, "Gewerkschaft kontaktieren — sie organisieren Betriebsratsgründungen kostenlos", "Contact a trade union — they organise works council elections for free"),
			tr(lang, "Wahlvorstand bestellen: 3 Arbeitnehmer können die Wahl einleiten (§ 17 BetrVG)", "Appoint an electoral board: 3 employees can initiate the election (§ 17 BetrVG)"),
			tr(lang, "Arbeitgeber darf die Wahl nicht behindern — Behinderung ist strafbar (§ 119 BetrVG)", "The employer may not obstruct the election — obstruction is a criminal offence (§ 119 BetrVG)"),
		}
	}
	// Default next steps — use cases only, no command syntax
	return []string{
		tr(lang, "Prüfen ob es in Ihrem Betrieb einen BR gibt: Fragen Sie die Personalabteilung oder schauen Sie am schwarzen Brett", "Check if your company has a works council: ask HR or check the notice board"),
		tr(lang, "Konkrete Situation? Beschreiben Sie, was gerade passiert — z.B. \"Ich habe eine Kündigung erhalten\"", "Have a specific situation? Describe what's happening — e.g. \"I received a dismissal\""),
		tr(lang, "Fragen Sie nach Ihren Rechten — z.B. \"Was passiert, wenn der Betrieb umstrukturiert wird?\"", "Ask about your rights — e.g. \"What happens if the company restructures?\""),
		tr(lang, "Fragen Sie nach einem konkreten Gesetzesparagrafen — z.B. \"Was sagt § 102 BetrVG?\"", "Ask about a specific law — e.g. \"What does § 102 BetrVG say?\""),
	}
}

func buildEmployeeAnswer(lang, question string, strongest betrvg.CoDeterminationType, paras []betrvg.Paragraph, classification string) string {
	low := strings.ToLower(question)
	var sb strings.Builder

	// Non-BetrVG special laws — AGG, Mutterschutz, Elternzeit, SGB IX
	if containsAny(low, "diskriminierung", "benachteiligung", "discrimination", "belästigung", "agg", "gleichbehandlung") {
		sb.WriteString(tr(lang,
			"Das Allgemeine Gleichbehandlungsgesetz (AGG) schützt Sie vor Benachteiligung aufgrund von Rasse, Geschlecht, Religion, Behinderung, Alter oder sexueller Identität. Wenden Sie sich an den Betriebsrat (§ 84 BetrVG) und reichen Sie ggf. eine Beschwerde nach § 13 AGG ein. Klagefrist: 3 Monate nach Kenntnis der Benachteiligung.",
			"The General Equal Treatment Act (AGG) protects you from discrimination based on race, gender, religion, disability, age, or sexual identity. Contact the works council (§ 84 BetrVG) and consider filing a complaint under § 13 AGG. Claim deadline: 3 months after becoming aware of the discrimination."))
		return sb.String()
	}
	if containsAny(low, "mutterschutz", "schwanger", "mutterschaft", "maternity", "pregnant", "pregnancy") {
		sb.WriteString(tr(lang,
			"Schwangere und Mütter genießen besonderen Kündigungsschutz nach dem Mutterschutzgesetz (MuSchG). Eine Kündigung während der Schwangerschaft und bis 4 Monate nach der Entbindung ist grundsätzlich unwirksam (§ 17 MuSchG) — auch ohne Betriebsratsbeteiligung. Benachrichtigen Sie Ihren Arbeitgeber schriftlich über die Schwangerschaft und informieren Sie den Betriebsrat.",
			"Pregnant employees and new mothers have special dismissal protection under the Maternity Protection Act (MuSchG). Dismissal during pregnancy and up to 4 months after birth is generally void (§ 17 MuSchG) — even without works council involvement. Notify your employer of the pregnancy in writing and inform the works council."))
		return sb.String()
	}
	if containsAny(low, "elternzeit", "parental leave", "elterngeld", "beeg") {
		sb.WriteString(tr(lang,
			"Während der Elternzeit (BEEG) besteht ein besonderer Kündigungsschutz. Eine Kündigung ist ohne behördliche Zulassung (Landesbehörde) in der Regel unzulässig (§ 18 BEEG). Elternzeit muss spätestens 7 Wochen vor Beginn schriftlich angemeldet werden (§ 16 BEEG). Der Betriebsrat hat ein Überwachungsrecht (§ 80 BetrVG).",
			"During parental leave (BEEG) there is special dismissal protection. Dismissal without official authorization (state authority) is generally prohibited (§ 18 BEEG). Parental leave must be registered in writing at least 7 weeks before it begins (§ 16 BEEG). The works council has a monitoring right (§ 80 BetrVG)."))
		return sb.String()
	}
	if containsAny(low, "schwerbehinderung", "behinderung", "sgb ix", "inklusionsamt", "integrationsamt", "disability", "severely disabled") {
		sb.WriteString(tr(lang,
			"Schwerbehinderte Arbeitnehmer (GdB ≥ 50 oder Gleichstellung) genießen besonderen Kündigungsschutz. Eine Kündigung bedarf der vorherigen Zustimmung des Inklusionsamtes (§ 168 SGB IX). Ohne diese Zustimmung ist die Kündigung unwirksam. Der Betriebsrat muss ebenfalls nach § 102 BetrVG angehört werden. Zusätzlich gilt: Der Arbeitgeber muss die Schwerbehindertenvertretung (SBV) einbeziehen.",
			"Severely disabled employees (degree of disability ≥ 50 or equivalent status) have special dismissal protection. Dismissal requires prior approval from the Inklusionsamt (§ 168 SGB IX). Without approval the dismissal is void. The works council must also be heard (§ 102 BetrVG). Additionally: the employer must involve the Schwerbehindertenvertretung (SBV)."))
		return sb.String()
	}

	// Was procedure followed?
	if containsAny(low, "entlassen", "kündigung", "gekündigt", "dismissed", "fired", "termination") {
		sb.WriteString(tr(lang,
			"Bei jeder Kündigung muss der Betriebsrat vorher angehört werden (§ 102 BetrVG). Ohne ordnungsgemäße Anhörung ist die Kündigung unwirksam.\n\n",
			"Before any dismissal the works council must be heard (§ 102 BetrVG). Without proper consultation the dismissal is void.\n\n"))
		sb.WriteString(tr(lang,
			"Prüfen Sie: Hat der Arbeitgeber dem BR ein schriftliches Anhörungsschreiben mit Ihren vollständigen Sozialdaten und dem Kündigungsgrund übergeben? Hat der BR innerhalb der Frist (1 Woche ordentlich / 3 Tage fristlos) reagiert?",
			"Check: Did the employer give the BR a written Anhörungsschreiben with your full personal data and the dismissal reason? Did the BR respond within the deadline (1 week ordinary / 3 days extraordinary)?"))
	} else if containsAny(low, "versetzt", "versetzung", "transferred", "transfer") {
		sb.WriteString(tr(lang,
			"Bei Versetzungen benötigt der Arbeitgeber die Zustimmung des Betriebsrats (§ 99 BetrVG). Ohne Zustimmung oder ohne Antrag auf Ersetzung beim Arbeitsgericht ist die Versetzung unwirksam.",
			"For transfers the employer needs the BR's consent (§ 99 BetrVG). Without consent or a court application to replace it, the transfer is invalid."))
	} else if containsAny(low, "sozialplan", "abfindung", "betriebsänderung", "redundancy", "layoff", "restructur") {
		sb.WriteString(tr(lang,
			"Bei einer Betriebsänderung (§ 111 BetrVG) haben betroffene Arbeitnehmer Anspruch auf einen Sozialplan (§ 112 BetrVG). Der Sozialplan ist erzwingbar — der Betriebsrat kann ihn über die Einigungsstelle durchsetzen.",
			"In a Betriebsänderung (§ 111 BetrVG) affected employees are entitled to a Sozialplan (§ 112 BetrVG). The Sozialplan is legally enforceable — the BR can force it through the conciliation board."))
	} else if strongest != betrvg.Keine && len(paras) > 0 {
		sb.WriteString(tr(lang,
			fmt.Sprintf("Für diese Situation gilt: %s. Das bedeutet, dass der Arbeitgeber ohne Beteiligung des Betriebsrats nicht einfach handeln kann.", classification),
			fmt.Sprintf("This situation falls under: %s. This means the employer cannot act unilaterally without involving the works council.", classification)))
	} else {
		sb.WriteString(tr(lang,
			"Für Ihre konkrete Situation empfehle ich: Prüfen Sie, ob der Betriebsrat einbezogen wurde, und konsultieren Sie gesetze-im-internet.de oder einen Fachanwalt für Arbeitsrecht.",
			"For your situation I recommend: check whether the works council was involved, and consult gesetze-im-internet.de or a labour law specialist."))
	}

	return sb.String()
}

func buildEmployeeActions(lang, question string, strongest betrvg.CoDeterminationType, paras []betrvg.Paragraph) []string {
	low := strings.ToLower(question)
	var actions []string

	if containsAny(low, "diskriminierung", "benachteiligung", "discrimination", "belästigung", "agg", "gleichbehandlung") {
		return []string{
			tr(lang, "Beschwerde nach § 13 AGG beim Arbeitgeber oder Betriebsrat innerhalb von 3 Monaten einreichen", "File a complaint under § 13 AGG with the employer or works council within 3 months"),
			tr(lang, "Antidiskriminierungsstelle des Bundes (ADS) kontaktieren: www.antidiskriminierungsstelle.de", "Contact the Federal Anti-Discrimination Agency (ADS): www.antidiskriminierungsstelle.de"),
			tr(lang, "Schadensersatz- und Entschädigungsklage nach § 15 AGG beim Arbeitsgericht innerhalb von 3 Monaten", "Bring a claim for damages under § 15 AGG at the labour court within 3 months"),
		}
	}
	if containsAny(low, "mutterschutz", "schwanger", "mutterschaft", "maternity", "pregnant", "pregnancy") {
		return []string{
			tr(lang, "Schwangerschaft dem Arbeitgeber schriftlich mitteilen (§ 15 MuSchG) — löst Sonderkündigungsschutz aus", "Notify the employer of the pregnancy in writing (§ 15 MuSchG) — triggers special dismissal protection"),
			tr(lang, "Aufsichtsbehörde (Gewerbeaufsicht oder Landesamt für Arbeitsschutz) informieren", "Notify the supervisory authority (Gewerbeaufsicht or Landesamt für Arbeitsschutz)"),
			tr(lang, "Betriebsrat informieren und um Unterstützung bitten (§ 80 BetrVG)", "Inform the works council and request support (§ 80 BetrVG)"),
		}
	}
	if containsAny(low, "elternzeit", "parental leave", "elterngeld", "beeg") {
		return []string{
			tr(lang, "Elternzeit schriftlich beim Arbeitgeber anmelden — mind. 7 Wochen vor Beginn (§ 16 BEEG)", "Register parental leave in writing with the employer — at least 7 weeks before start (§ 16 BEEG)"),
			tr(lang, "Elterngeld bei der Elterngeldstelle beantragen (online: elterngeld-digital.de)", "Apply for Elterngeld at the Elterngeldstelle (online: elterngeld-digital.de)"),
			tr(lang, "Bei Kündigung während Elternzeit: Zustimmung der Landesbehörde prüfen (§ 18 BEEG)", "If dismissed during parental leave: check whether state authority approval was obtained (§ 18 BEEG)"),
		}
	}
	if containsAny(low, "schwerbehinderung", "behinderung", "sgb ix", "inklusionsamt", "integrationsamt", "disability", "severely disabled") {
		return []string{
			tr(lang, "GdB (Grad der Behinderung) beim Versorgungsamt feststellen lassen falls noch nicht geschehen", "Have degree of disability (GdB) certified at the Versorgungsamt if not yet done"),
			tr(lang, "Bei Kündigung: Zustimmung des Inklusionsamtes nach § 168 SGB IX prüfen — ohne Zustimmung ist Kündigung unwirksam", "If dismissed: check Inklusionsamt approval under § 168 SGB IX — without approval dismissal is void"),
			tr(lang, "Schwerbehindertenvertretung (SBV) im Betrieb kontaktieren", "Contact the Schwerbehindertenvertretung (SBV) in the company"),
			tr(lang, "Gleichstellungsantrag bei der Agentur für Arbeit stellen (bei GdB 30–49)", "Apply for equivalent status at the Agentur für Arbeit (if GdB 30–49)"),
		}
	}

	if containsAny(low, "entlassen", "kündigung", "gekündigt", "dismissed", "fired", "termination") {
		actions = append(actions,
			tr(lang, "Kündigungsschutzklage beim Arbeitsgericht innerhalb von 3 Wochen nach Zugang der Kündigung einreichen (§ 4 KSchG)", "File Kündigungsschutzklage at the labour court within 3 weeks of receiving the dismissal (§ 4 KSchG)"),
			tr(lang, "Anhörungsschreiben anfordern: Hat der BR ein vollständiges Anhörungsschreiben erhalten?", "Request the Anhörungsschreiben: did the BR receive a complete hearing letter?"),
			tr(lang, "Prüfen lassen: betriebsrat check-anhoerung \"<text des Anhörungsschreibens>\"", "Check with: betriebsrat check-anhoerung \"<text of the hearing letter>\""),
			tr(lang, "Bei fehlerhafter Sozialauswahl: BR kann Widerspruch einlegen → Recht auf Weiterbeschäftigung während des Klageverfahrens (§ 102 Abs. 5)", "If social selection was wrong: BR can object → right to continued employment during the appeal (§ 102 Abs. 5)"),
		)
	} else if containsAny(low, "sozialplan", "abfindung", "betriebsänderung", "redundancy", "layoff") {
		actions = append(actions,
			tr(lang, "Sozialplanabfindung berechnen: betriebsrat sozialplan-calc --salary <monatslohn> --years <betriebsjahre>", "Calculate Sozialplan entitlement: betriebsrat sozialplan-calc --salary <monthly_salary> --years <years_service>"),
			tr(lang, "Bei fehlendem Interessenausgleich: Nachteilsausgleich nach § 113 BetrVG prüfen — betriebsrat nachteilsausgleich --salary <lohn> --years <jahre> --no-ia-attempted", "If Interessenausgleich was skipped: check Nachteilsausgleich under § 113 BetrVG — betriebsrat nachteilsausgleich --salary <salary> --years <years> --no-ia-attempted"),
			tr(lang, "Fachanwalt für Arbeitsrecht konsultieren", "Consult a labour law specialist"),
		)
	} else {
		actions = append(actions,
			tr(lang, "Betriebsrat kontaktieren und fragen, ob er ordnungsgemäß einbezogen wurde", "Contact the works council and ask if it was properly involved"),
			tr(lang, "Betriebsrat um Akteneinsicht und Auskunft bitten (§ 80 Abs. 2 BetrVG)", "Ask the BR for information and document access (§ 80 Abs. 2 BetrVG)"),
			tr(lang, "Fachanwalt für Arbeitsrecht konsultieren", "Consult a labour law specialist"),
		)
	}
	return actions
}

// classificationFallback returns a useful answer based on classification when keyword matching found nothing.
func classificationFallback(lang, classification string) string {
	low := strings.ToLower(classification)
	switch {
	case strings.Contains(low, "87") || strings.Contains(low, "technical") || strings.Contains(low, "technische"):
		return tr(lang,
			"Technische Systeme, die das Verhalten oder die Leistung von Mitarbeitern überwachen können, unterliegen der erzwingbaren Mitbestimmung des Betriebsrats (§ 87 Abs. 1 Nr. 6 BetrVG). Der Arbeitgeber braucht eine Betriebsvereinbarung — er darf das System nicht einführen, bevor der BR zugestimmt hat oder die Einigungsstelle entschieden hat.",
			"Technical systems capable of monitoring employee behaviour or performance trigger the works council's enforceable co-determination right (§ 87 Abs. 1 Nr. 6 BetrVG). The employer needs a Betriebsvereinbarung — it may not deploy the system until the BR consents or the conciliation board rules.")
	case strings.Contains(low, "dismissal") || strings.Contains(low, "kündigung"):
		return tr(lang,
			"Vor jeder Kündigung muss der Betriebsrat angehört werden (§ 102 BetrVG). Ohne vollständige Anhörung ist die Kündigung unwirksam.",
			"Before any dismissal the works council must be heard (§ 102 BetrVG). Without proper hearing the dismissal is void.")
	case strings.Contains(low, "betriebsänderung") || strings.Contains(low, "operational change"):
		return tr(lang,
			"Betriebsänderungen (§ 111 BetrVG) lösen Informations-, Beratungs- und Verhandlungspflichten aus. Der Sozialplan ist erzwingbar.",
			"Operational changes (§ 111 BetrVG) trigger information, consultation and negotiation obligations. The Sozialplan is legally enforceable.")
	default:
		return tr(lang,
			"Für diese Situation empfehle ich eine detaillierte Analyse mit: betriebsrat decide \"<Ihre Situation>\"",
			"For this situation I recommend a detailed analysis with: betriebsrat decide \"<your situation>\"")
	}
}

// detectRole returns "curious", "employee", or "br_member" based on question phrasing.
// "curious" = exploratory/definitional question from someone new to BR concepts.
func detectRole(question string) string {
	low := strings.ToLower(question)

	// Exploratory signals — user wants to understand BR, not navigate a specific situation
	exploratorySignals := []string{
		"was ist ein betriebsrat", "was ist der betriebsrat", "what is a works council",
		"what is the works council", "was macht ein betriebsrat", "was macht der betriebsrat",
		"what does a works council do", "what does the works council do",
		"wie funktioniert", "how does a works council", "how does the works council",
		"wofür ist der betriebsrat", "what is the br for", "what's the br for",
		"erkläre den betriebsrat", "erkläre mir", "explain the works council", "explain what a",
		"habe ich einen betriebsrat", "do i have a works council", "gibt es einen betriebsrat",
		"is there a works council", "kein betriebsrat", "no works council",
		"betriebsrat gründen", "found a works council", "elect a works council",
		"was kann der betriebsrat", "what can the works council", "what can a works council",
		"was darf der betriebsrat", "what may the works council",
		"unterschied zwischen betriebsrat", "difference between works council",
		"wozu ist ein betriebsrat", "wozu dient", "what's the point of",
		"neu hier", "new to this", "first time asking",
	}
	for _, sig := range exploratorySignals {
		if strings.Contains(low, sig) {
			return "curious"
		}
	}

	// Strong employee signals
	employeeSignals := []string{
		"ich wurde", "ich bin", "ich habe", "mein arbeitgeber", "meine kündigung",
		"meine abfindung", "bin ich berechtigt", "habe ich anspruch", "was kann ich",
		"wurde der br", "hat der br", "i was", "i am", "i have", "my employer",
		"my dismissal", "my redundancy", "am i entitled", "what can i", "was i",
		"was the br", "did the br", "wurde ich", "bekomme ich", "steht mir",
	}
	for _, sig := range employeeSignals {
		if strings.Contains(low, sig) {
			return "employee"
		}
	}

	// Strong BR signals
	brSignals := []string{
		"wir haben", "haben wir", "unser arbeitgeber", "wir als br", "als betriebsrat",
		"wir erhalten", "wir müssen", "we received", "do we have", "our employer",
		"as the br", "as works council", "müssen wir", "dürfen wir",
	}
	for _, sig := range brSignals {
		if strings.Contains(low, sig) {
			return "br_member"
		}
	}

	return "br_member" // default
}

// looksEnglish returns true when the question contains common English function words.
func looksEnglish(s string) bool {
	low := strings.ToLower(s)
	englishWords := []string{" the ", " my ", " our ", " your ", " is ", " are ", " was ",
		" have ", " has ", " do ", " does ", " will ", " can ", " employer ", " employee ",
		" dismissed ", " fired ", " layoff ", " redundancy "}
	count := 0
	for _, w := range englishWords {
		if strings.Contains(" "+low+" ", w) {
			count++
		}
	}
	return count >= 2
}

var reNumber = regexp.MustCompile(`(\d[\d.,]*)`)

// extractSozialplanNumbers tries to find salary and years in the question.
// Returns (salary, years, ok).
func extractSozialplanNumbers(question string) (salary, years float64, ok bool) {
	low := strings.ToLower(question)
	nums := reNumber.FindAllString(low, -1)
	if len(nums) < 2 {
		return 0, 0, false
	}

	// Heuristic: find number near "euro/€" → salary; near "jahr" → years
	reSalary := regexp.MustCompile(`(\d[\d.,]*)\s*(?:euro|eur|€)`)
	reYears := regexp.MustCompile(`(\d+(?:[.,]\d+)?)\s*(?:jahre?|years?)`)

	salaryMatches := reSalary.FindStringSubmatch(low)
	yearsMatches := reYears.FindStringSubmatch(low)

	if salaryMatches == nil || yearsMatches == nil {
		return 0, 0, false
	}

	salStr := strings.ReplaceAll(salaryMatches[1], ",", ".")
	yearStr := strings.ReplaceAll(yearsMatches[1], ",", ".")

	s, err1 := strconv.ParseFloat(salStr, 64)
	y, err2 := strconv.ParseFloat(yearStr, 64)
	if err1 != nil || err2 != nil || s <= 0 || y <= 0 {
		return 0, 0, false
	}
	return s, y, true
}
