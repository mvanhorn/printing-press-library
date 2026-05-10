# betriebsrat

**Know your rights at work — for employees and works council members.**

> **[🇩🇪 Deutsche Version weiter unten](#deutsch)**

When your employer hands you a dismissal notice, announces a restructuring, or rolls out new monitoring software — you're at a disadvantage. They have legal counsel. You have Google.

betriebsrat closes that gap. Describe your situation in plain German or English and get a grounded answer that cites the specific law, the relevant deadline, and what your employer was actually required to do.

Built on content from [betriebsrat.de](https://www.betriebsrat.de) — the recognised German authority on works council practice — and the complete text of the *Betriebsverfassungsgesetz*. Every answer links back to the relevant topic on betriebsrat.de so you can verify it yourself. Not AI guesswork: structured law reasoning grounded in authoritative sources.

---

## What can I ask?

Describe your situation in your own words. Here is what the tool knows to look for:

**As an employee:**

> "Can I get severance? I've worked here 7 years and earn €3,500 a month."

*Severance isn't automatic — it depends on whether a Sozialplan was agreed or bypassed, and whether your company meets the size threshold. If it applies, your numbers run through the Munich formula.*

> "I received a dismissal notice. Was the works council properly consulted?"

*If the hearing notice was missing, incomplete, or gave the BR too little time, the dismissal is invalid — regardless of the reason given. This is one of the most common procedural failures employers make.*

> "I'm pregnant and my employer wants to end my contract."

*The protection applies even if your employer doesn't know about the pregnancy yet. If you find out after receiving the notice, you have 2 weeks to inform them — and the protection kicks in retroactively.*

> "My employer introduced monitoring software. Did the works council have to agree?"

*The trigger isn't whether it's marketed as "monitoring" — it's whether it can track performance or behaviour, even indirectly through logs. If the BR wasn't involved before rollout, the deployment may be unlawful.*

> "I was transferred to a different location without my agreement. Is that allowed?"

*The BR must approve transfers. The question is how significantly your conditions actually change — that determines whether the BR had grounds to refuse, and what you can do if the employer skipped the step entirely.*

---

**As a works council member:**

> "We received a dismissal hearing notice. What must we do and by when?"

*The deadline isn't the only thing to check. If the employer gave incomplete social data or no reason, your options expand — an objection gives the employee the right to continued employment while any dispute is pending.*

> "The employer wants to introduce an AI writing assistant. Do we have co-determination rights?"

*The question isn't whether it's called "AI" — it's whether it can influence or evaluate how employees work, even indirectly. That triggers § 87 Nr. 6, and you can block rollout until a Betriebsvereinbarung is agreed.*

> "We're facing a mass layoff of 20 people. What's our role?"

*The 30-day consultation window starts before the employer files with the employment agency — not after. Missing that sequence gives grounds to challenge the entire process.*

> "Can we force a Sozialplan if the employer refuses to negotiate?"

*Yes — the Sozialplan is one of the few areas where the BR can go to the Einigungsstelle and get a binding decision without employer agreement.*

---

## How it answers

When you describe a situation, the tool works through a fixed sequence before it says anything:

**1. Who are you?**
Employee or works council member? This changes the framing entirely. For employees: was procedure followed, and what are you entitled to? For BR members: what must you do, by when, and what leverage do you have?

**2. What law applies?**
Your situation is mapped to BetrVG paragraphs first, then to KSchG, AGG, Mutterschutzgesetz, or SGB IX where relevant. It looks for the strongest right first — *erzwingbare Mitbestimmung* (BR can block) > *Zustimmungsvorbehalt* (employer needs consent) > *Beratung/Mitwirkung* (employer must consult) > *Unterrichtung* (employer must inform).

**3. Was procedure followed?**
For dismissals: was the BR consulted, was the hearing notice complete, did the 1-week window run correctly? For restructurings: was Interessenausgleich attempted before implementation? For hiring and transfers: did the employer obtain BR consent under § 99?

**4. What follows from that?**
If procedure was violated: what are the legal consequences? A dismissal without proper Anhörung is invalid. A Betriebsänderung without Interessenausgleich creates a personal Nachteilsausgleich claim. A transfer without BR consent may have to be reversed.

**5. The deadline — always first.**
§ 102: 1 week (ordinary) / 3 days (extraordinary dismissal). § 99: 1 week (hiring/transfer). § 17 KSchG: employer must file before notice period begins. The 3-week window to challenge a dismissal in court. Whichever deadline is relevant, it surfaces before anything else.

**6. Numbers when the situation calls for it.**
If a Sozialplan might apply, it calculates your entitlement estimate from the Munich formula (years × salary × factor, with adjustments for disability, children, age). If Nachteilsausgleich applies, it estimates the claim and notes the Sozialplan offset.

**7. A source you can verify.**
Every answer links to the relevant topic on betriebsrat.de so you can read the authoritative source yourself — not just trust the tool.

---

## How to get it

Tell Claude to install it:

> Install this: https://github.com/googlarz/betriebsrat

Claude will handle the rest. Then just describe your situation and ask away.

**No install needed to start.** The Claude skill works standalone — Claude answers from its embedded BetrVG knowledge. Installing the CLI adds live betriebsrat.de content and structured outputs.

---

## What it knows

- Every paragraph of the *Betriebsverfassungsgesetz* (BetrVG) — the German works constitution law
- Co-determination rights: when the works council can block, must consent, or is only informed
- Legal deadlines — including the critical **3-week window** to challenge a dismissal in court
- Sozialplan and severance calculations (Munich formula)
- AGG (discrimination), Mutterschutz (maternity), Elternzeit (parental leave), SGB IX (disability)
- Procedure violations — what happens if the employer didn't follow the rules

---

## License

Apache 2.0 — see [LICENSE](LICENSE).

*This tool provides legal orientation, not legal advice. For your specific situation, consult a labour law specialist or your trade union.*

---

<details>
<summary>For developers — CLI reference</summary>

### Install (CLI)

```bash
go install github.com/googlarz/betriebsrat/cmd/betriebsrat@latest
betriebsrat doctor
```

### Key commands

```bash
betriebsrat ask "Ich wurde entlassen. Was nun?"
betriebsrat rights-check "KI-Überwachungssystem einführen" --agent
betriebsrat decide "Massenentlassung 20 Personen" --json
betriebsrat sozialplan-calc --salary 3500 --years 7 --age 48
betriebsrat nachteilsausgleich --salary 3500 --years 7 --no-ia-attempted
betriebsrat check-anhoerung "<text des Anhörungsschreibens>"
betriebsrat deadline kuendigung --from 2026-05-10
betriebsrat law 102
betriebsrat bv-template homeoffice
betriebsrat ki-check "Teams-Analysefunktion zur Produktivitätsmessung"
betriebsrat massenentlassung --employees 120 --affected 25
betriebsrat serve                      # web UI at http://localhost:7890
betriebsrat serve --host 0.0.0.0       # share on local network
```

### MCP server

```bash
# Claude Code
claude mcp add betriebsrat betriebsrat-pp-mcp

# Claude Desktop
{
  "mcpServers": {
    "betriebsrat": { "command": "betriebsrat-pp-mcp" }
  }
}
```

### Output flags

`--json` · `--agent` · `--lang en` · `--lang de`

</details>

---

---

<a name="deutsch"></a>

# betriebsrat — Deutsche Dokumentation

**Arbeitsrechte kennen — für Arbeitnehmer und Betriebsratsmitglieder.**

> **[🇬🇧 English version above](#betriebsrat)**

Wenn der Arbeitgeber eine Kündigung überreicht, eine Umstrukturierung ankündigt oder neue Überwachungssoftware einführt — sind Sie im Nachteil. Er hat rechtliche Beratung. Sie haben Google.

betriebsrat schließt diese Lücke. Beschreiben Sie Ihre Situation auf Deutsch oder Englisch und erhalten Sie eine fundierte Antwort, die den konkreten Paragrafen, die relevante Frist und das nennt, wozu Ihr Arbeitgeber verpflichtet war.

Basiert auf Inhalten von [betriebsrat.de](https://www.betriebsrat.de) — der anerkannten deutschen Referenz für Betriebsratsrecht — und dem vollständigen Text des Betriebsverfassungsgesetzes. Jede Antwort verweist auf das entsprechende Thema auf betriebsrat.de, damit Sie es selbst nachprüfen können. Kein freies KI-Raten: strukturierte Rechtsauswertung auf Basis verlässlicher Quellen.

---

## Was kann ich fragen?

Beschreiben Sie Ihre Situation in eigenen Worten. Das Tool weiß, worauf es dabei ankommt:

**Als Arbeitnehmer:**

> „Habe ich Anspruch auf eine Abfindung? Ich arbeite hier seit 7 Jahren und verdiene 3.500 € monatlich."

*Eine Abfindung ist nicht automatisch — sie hängt davon ab, ob ein Sozialplan vereinbart oder übergangen wurde und ob die Betriebsgröße den Schwellenwert erreicht. Wenn sie greift, laufen Ihre Zahlen durch die Münchner Formel.*

> „Ich habe eine Kündigung erhalten. Wurde der Betriebsrat ordnungsgemäß angehört?"

*Fehlt das Anhörungsschreiben, war es unvollständig oder die Frist zu kurz, ist die Kündigung unwirksam — unabhängig vom genannten Grund. Das ist einer der häufigsten Verfahrensfehler.*

> „Ich bin schwanger und mein Arbeitgeber will meinen Vertrag beenden."

*Der Schutz gilt auch, wenn der Arbeitgeber von der Schwangerschaft noch nichts weiß. Erfahren Sie es erst nach der Kündigung, haben Sie 2 Wochen Zeit zur Mitteilung — der Schutz greift rückwirkend.*

> „Mein Arbeitgeber hat eine Überwachungssoftware eingeführt. Musste der Betriebsrat zustimmen?"

*Entscheidend ist nicht, ob es als „Überwachung" beworben wird — sondern ob es Leistung oder Verhalten erfassen kann, auch mittelbar über Logs. Wurde der BR nicht einbezogen, kann der Einsatz rechtswidrig sein.*

> „Ich wurde ohne mein Einverständnis versetzt. Ist das erlaubt?"

*Der BR muss Versetzungen zustimmen. Die Frage ist, wie stark sich die Arbeitsbedingungen tatsächlich ändern — das bestimmt, ob der BR Verweigerungsgründe hatte und was Sie tun können, wenn der Schritt übersprungen wurde.*

---

**Als Betriebsratsmitglied:**

> „Wir haben ein Anhörungsschreiben für eine Kündigung erhalten. Was müssen wir tun und bis wann?"

*Die Frist ist nicht das Einzige. Wenn der Arbeitgeber Sozialdaten oder den Grund unvollständig angegeben hat, erweitern sich Ihre Optionen — ein Widerspruch gibt dem Arbeitnehmer das Recht auf Weiterbeschäftigung während eines laufenden Verfahrens.*

> „Der Arbeitgeber will einen KI-Schreibassistenten einführen. Haben wir ein Mitbestimmungsrecht?"

*Nicht der Begriff „KI" ist entscheidend — sondern ob das Tool Arbeitsweise oder Leistung beeinflussen oder auswerten kann, auch indirekt. Das löst § 87 Nr. 6 aus, und Sie können die Einführung bis zur Einigung auf eine Betriebsvereinbarung blockieren.*

> „Uns droht eine Massenentlassung von 20 Personen. Was ist unsere Rolle?"

*Die 30-tägige Konsultationsfrist beginnt, bevor der Arbeitgeber die Anzeige bei der Agentur für Arbeit einreicht — nicht danach. Wird diese Reihenfolge missachtet, kann das gesamte Verfahren angefochten werden.*

> „Können wir einen Sozialplan erzwingen, wenn der Arbeitgeber nicht verhandeln will?"

*Ja — der Sozialplan ist einer der wenigen Bereiche, in denen der BR die Einigungsstelle anrufen und eine verbindliche Entscheidung auch ohne Einverständnis des Arbeitgebers erwirken kann.*

---

## So arbeitet das Tool

Wenn Sie eine Situation beschreiben, geht das Tool eine feste Abfolge durch, bevor es antwortet:

**1. Wer sind Sie?**
Arbeitnehmer oder Betriebsratsmitglied? Das bestimmt den Blickwinkel. Für Arbeitnehmer: Wurde das Verfahren eingehalten, und was steht Ihnen zu? Für BR-Mitglieder: Was müssen Sie tun, bis wann, und welche Hebel haben Sie?

**2. Welches Gesetz gilt?**
Ihre Situation wird zunächst auf BetrVG-Paragrafen abgebildet, dann auf KSchG, AGG, Mutterschutzgesetz oder SGB IX, wo einschlägig. Gesucht wird nach dem stärksten Recht: *erzwingbare Mitbestimmung* (BR kann blockieren) > *Zustimmungsvorbehalt* (Arbeitgeber braucht Zustimmung) > *Beratung/Mitwirkung* (Arbeitgeber muss konsultieren) > *Unterrichtung* (Arbeitgeber muss informieren).

**3. Wurde das Verfahren eingehalten?**
Bei Kündigungen: Wurde der BR angehört, war das Anhörungsschreiben vollständig, lief die 1-Wochenfrist korrekt? Bei Betriebsänderungen: Wurde ein Interessenausgleich versucht, bevor die Maßnahme umgesetzt wurde? Bei Einstellungen und Versetzungen: Hat der Arbeitgeber die Zustimmung des BR nach § 99 eingeholt?

**4. Was folgt daraus?**
Wenn Verfahrensfehler vorliegen: Welche rechtlichen Konsequenzen ergeben sich? Eine Kündigung ohne ordnungsgemäße Anhörung ist unwirksam. Eine Betriebsänderung ohne Interessenausgleich begründet einen persönlichen Nachteilsausgleichsanspruch. Eine Versetzung ohne BR-Zustimmung kann rückgängig gemacht werden müssen.

**5. Die Frist — immer zuerst.**
§ 102: 1 Woche (ordentliche) / 3 Tage (außerordentliche Kündigung). § 99: 1 Woche (Einstellung/Versetzung). § 17 KSchG: Anzeige vor Beginn der Kündigungsfrist. Die 3-Wochen-Frist zur Kündigungsschutzklage. Welche Frist auch immer relevant ist — sie erscheint vor allem anderen.

**6. Zahlen, wenn die Situation es erfordert.**
Wenn ein Sozialplan in Betracht kommt, wird Ihr Abfindungsanspruch nach der Münchner Formel geschätzt (Jahre × Gehalt × Faktor, mit Zuschlägen für Schwerbehinderung, Kinder, Alter). Bei Nachteilsausgleich wird der Anspruch berechnet und der Sozialplan-Offset ausgewiesen.

**7. Eine Quelle, die Sie prüfen können.**
Jede Antwort verlinkt auf das einschlägige Thema auf betriebsrat.de — damit Sie die maßgebliche Quelle selbst nachlesen können, anstatt nur dem Tool zu vertrauen.

---

## Wie bekomme ich das Tool?

Sagen Sie Claude, es soll installieren:

> Install this: https://github.com/googlarz/betriebsrat

Claude übernimmt den Rest. Dann beschreiben Sie einfach Ihre Situation.

**Keine Installation nötig für den Einstieg.** Das Claude-Skill funktioniert eigenständig — Claude antwortet aus seinem eingebetteten BetrVG-Wissen. Die CLI-Installation ergänzt live Inhalte von betriebsrat.de und strukturierte Ausgaben.

---

## Was das Tool weiß

- Alle Paragrafen des Betriebsverfassungsgesetzes (BetrVG)
- Mitbestimmungsrechte: wann der Betriebsrat blockieren kann, zustimmen muss oder nur informiert wird
- Gesetzliche Fristen — einschließlich der kritischen **3-Wochen-Frist** für die Kündigungsschutzklage
- Sozialplan- und Abfindungsberechnungen (Münchner Formel)
- AGG (Diskriminierungsschutz), Mutterschutz, Elternzeit (BEEG), SGB IX (Schwerbehinderung)
- Verfahrensfehler — was passiert, wenn der Arbeitgeber die Regeln nicht eingehalten hat

---

## Lizenz

Apache 2.0 — siehe [LICENSE](LICENSE).

*Dieses Tool bietet rechtliche Orientierung, keine Rechtsberatung. Für Ihren konkreten Fall wenden Sie sich an einen Fachanwalt für Arbeitsrecht oder Ihre Gewerkschaft.*

---

<details>
<summary>Für Entwickler — CLI-Referenz</summary>

### Installation (CLI)

```bash
go install github.com/googlarz/betriebsrat/cmd/betriebsrat@latest
betriebsrat doctor
```

### Wichtige Befehle

```bash
betriebsrat ask "Ich wurde entlassen. Was nun?"
betriebsrat rights-check "KI-Überwachungssystem einführen" --agent
betriebsrat decide "Massenentlassung 20 Personen" --json
betriebsrat sozialplan-calc --salary 3500 --years 7 --age 48
betriebsrat nachteilsausgleich --salary 3500 --years 7 --no-ia-attempted
betriebsrat check-anhoerung "<text des Anhörungsschreibens>"
betriebsrat deadline kuendigung --from 2026-05-10
betriebsrat law 102
betriebsrat bv-template homeoffice
betriebsrat ki-check "Teams-Analysefunktion zur Produktivitätsmessung"
betriebsrat massenentlassung --employees 120 --affected 25
betriebsrat serve                      # Web-UI unter http://localhost:7890
betriebsrat serve --host 0.0.0.0       # Im lokalen Netzwerk teilen
```

### Ausgabeformate

`--json` · `--agent` · `--lang en` · `--lang de`

</details>
