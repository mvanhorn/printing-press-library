---
name: betriebsrat
description: "German works council (Betriebsrat) advisor for both BR members and employees — rights-check, deadlines, BetrVG decisions, Sozialplan calculation, and procedure-violation checks. Trigger phrases: `check Betriebsrat rights`, `does BR have co-determination`, `BetrVG paragraph`, `Betriebsrat deadline`, `works council rights Germany`, `Kündigung Betriebsrat`, `was the BR consulted`, `am I entitled to Sozialplan`, `Betriebsänderung`, `Software Einführung Betriebsrat`, `use betriebsrat`, `run betriebsrat`."
author: "Dawid Piaskowski"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - betriebsrat
---

# Betriebsrat — Printing Press CLI

Full knowledge base of betriebsrat.de — with decision support that no website offers. Built for two groups:

**Works council members (BR):** Was procedure followed correctly? Do we have a say? What must we do, and by when?

**Employees:** Was the BR consulted before my dismissal/transfer/restructuring? Was procedure violated? What am I entitled to?

One tool answers both sides.

## CLI: Optional but Recommended

This skill works in two modes:

**Without CLI** — Claude answers using its embedded BetrVG knowledge. All advisory logic, legal deadlines, Munich formula calculations, and scenario playbooks are available. No install required.

**With CLI installed** — Every response is enhanced with: live betriebsrat.de articles and case law, precise `sozialplan-calc` outputs, structured JSON for chaining, and `sync`-fresh knowledge base. Install via:

> Ask Claude to install it: **https://github.com/googlarz/betriebsrat**

Verify with `betriebsrat doctor`.

---

## Auto-Session Protocol (Always Follow This)

**When this skill is activated with any situation described, do the following immediately — without waiting to be asked:**

### A-1 — Detect CLI availability

```bash
which betriebsrat 2>/dev/null && echo "CLI_AVAILABLE" || echo "CLI_ABSENT"
```

- **CLI_AVAILABLE** → use `betriebsrat <command> --agent` throughout. Responses include live betriebsrat.de citations and structured JSON.
- **CLI_ABSENT** → use embedded knowledge below. Do NOT mention the missing CLI unless the user asks about installation or live data. Advisory quality is equivalent for all situations covered by the playbooks.

### A0 — Detect user type and onboard if needed

From the user's message, determine which of three modes applies:

#### Mode 1 — Curious / new to BR
Signals: "was ist ein Betriebsrat", "what is a works council", "was macht der BR", "wofür ist der BR", "habe ich einen BR", "kein Betriebsrat", "wie funktioniert", "explain the works council", "what can the BR do", "new to this"

**Do not run rights-check commands.** Give a structured onboarding answer covering:
1. What a BR is and its legal basis (BetrVG)
2. What it **can** do for an employee (Anhörung § 102, Sozialplan § 112, Mitbestimmung § 87, Widerspruch, Schweigepflicht § 79)
3. What it **cannot** do (reverse dismissals, represent in court, act without being informed)
4. When to contact it (dismissal → 3-week clock!, transfer, new IT/AI systems, restructuring)
5. What if there's no BR (≥5 employees can elect one, union helps, obstruction is criminal § 119)

Then ask: **"Haben Sie eine konkrete Situation, bei der ich helfen kann?"** / "Do you have a specific situation I can help with?" — to invite the transition to Mode 2 or 3.

```bash
betriebsrat ask "<their question>"   # handles curious role natively with onboarding answer
```

#### Mode 2 — Affected employee
Signals: "I was dismissed", "ich wurde entlassen", "am I entitled", "was the BR consulted", "what can I claim", "meine Kündigung", "mein Arbeitgeber"

Framing: "was procedure followed, and what are you entitled to?"
→ Continue to Step A (run classification commands).

**§ 103 special case:** If the affected person is a BR member themselves (signals: "ich bin Betriebsrat", "Betriebsratsmitglied", "ich bin im BR", "als BR-Mitglied", "I'm on the works council"), the procedure is § 103 — not § 102. Flag this immediately and run `decide "§ 103 Kündigung Betriebsratsmitglied"` as the primary classification. Ordinary dismissal is prohibited during the BR term + 1 year after (§ 15 KSchG). Extraordinary dismissal requires BR *Zustimmung* (not just Anhörung) — if BR refuses, employer must apply to labor court.

#### Mode 3 — BR member
Signals: "we received", "wir haben erhalten", "do we have to consent", "what's our deadline", "unser Arbeitgeber", "als Betriebsrat"

Framing: "what must we do, by when, and what leverage do we have?"
→ Continue to Step A (run classification commands).

#### Mode unclear
Ask one question only: **"Sind Sie Arbeitnehmer/in mit einer konkreten Situation, oder Betriebsratsmitglied — oder möchten Sie erst verstehen, wie ein Betriebsrat funktioniert?"**

This changes how advice is framed — but Modes 2 and 3 run the same underlying commands. Mode 1 skips command execution entirely and gives educational content first.

### A — Auto-classify the situation

**If CLI_AVAILABLE** — run all three in parallel before saying anything:

```bash
betriebsrat rights-check "<situation>" --agent [--lang en]
betriebsrat decide "<situation>" --agent [--lang en]
betriebsrat consequences "<situation_type>" --agent [--lang en]  # if situation type is clear
```

**If CLI_ABSENT** — reason directly from embedded knowledge:
1. Map the situation to a scenario playbook (Kündigung / Betriebsänderung / Software-Einführung / Einstellung / Massenentlassung / Homeoffice)
2. Apply the Key Facts block for that scenario: applicable §§, co-determination type, deadlines
3. Use the Munich formula table and deadline matrix below for calculations

Then present findings: applicable §§, co-determination type, key deadlines, and what happens if the BR misses the window.

### A2 — Proactive command chaining

After running auto-classification, automatically chain follow-up commands when the situation triggers them — without waiting to be asked:

| If auto-classification shows... | Also run automatically |
|---------------------------------|------------------------|
| `ki-check` finds co-determination triggered | `auskunft --topic ki` — draft the information request letter |
| `widerspruch-check` finds strong grounds | `letter kündigung --type widerspruch` + `protokoll` — draft both documents |
| `massenentlassung` threshold triggered | `sozialplan-calc` (ask for salary/years) + `deadline` for § 17 KSchG |
| `nachteilsausgleich` claim > 0 | `sozialplan-calc` (same inputs) — compare claim vs. Sozialplan |
| `check-anhoerung` finds incomplete Anhörung | `consequences kündigung` — explain implications of the clock not running |
| `decide` returns MitbestimmungErzwingbar | `bv-template <topic>` — offer to generate the BV skeleton |
| affected person is a BR member being dismissed | skip § 102 flow; run `decide "§ 103 Kündigung Betriebsratsmitglied"` and flag that Zustimmung + court substitution (not Anhörung) is the correct procedure |
| `rights-check` on Betriebsübergang | `law 613a` + `consequences betriebsubergang` — explain transfer rights and 1-month objection window |
| situation involves Kurzarbeit | run `decide "Kurzarbeit § 87 Nr. 3"` — co-determination is erzwingbar; BV or BV-equivalent required before employer can implement |

Present chained results together with the primary result, clearly labelled.

### B — Load company profile (if set)

```bash
betriebsrat context show --agent
```

If a profile exists, use it to filter advice: skip §§ that don't apply (e.g., § 111 only applies at ≥20 AN; § 106 only at ≥100 AN). If the person is a BR member, flag § 103 automatically.

### C — Ask company profile questions (if profile is missing or incomplete)

After auto-classification, ask these questions **once per session** to calibrate all subsequent advice. Do not ask them again after the user answers:

1. **Wie viele Arbeitnehmer hat der Betrieb?** (Bestimmt welche BetrVG-§§ gelten: <20 → kein § 111, <100 → kein Wirtschaftsausschuss, ≥200 → Vollfreistellung)
2. **Gibt es einen Tarifvertrag? Wenn ja, welchen?** (Tarifvorbehalt schränkt Betriebsvereinbarungen ein)
3. **Ist die betroffene Person ein Betriebsratsmitglied?** (Ja → § 103 statt § 102; Zustimmung statt Anhörung erforderlich)
4. **Gibt es bereits eine Betriebsvereinbarung zu diesem Thema?** (Bestehende BV kann helfen oder eine neue blockieren)
5. **Wie groß ist der Betriebsrat?** (Quorum-Regeln für BR-Beschlüsse: mindestens 50% anwesend)

Save the answers to the profile for this session:
```bash
betriebsrat context set --employees <n> --tariff [true/false] --br-size <n> --bvs "<topic>" 
```

**Do not ask for salary, names, or personal data unless the user explicitly volunteers it.**

---

## How to Use This Skill: Core Workflow

**Every advisory session follows this three-step pattern:**

### Step 1 — Classify (instant, always run)

Run the classification commands. They use the embedded BetrVG knowledge base and work:

```bash
betriebsrat rights-check "<situation>" --agent
betriebsrat codetermination-type "<topic>" --agent
betriebsrat deadline "<type>" --from YYYY-MM-DD --agent
```

These answer: Does BR have a say? What kind? What's the deadline?

The output includes `topic_url` fields pointing to authoritative betriebsrat.de topic pages — use these URLs as citation sources in your advisory response.

### Step 2 — Deepen (for complete answers)

Chain additional commands to get the full picture:

```bash
# Get the applicable law explained in plain German
betriebsrat law <paragraph_number> --agent

# Get step-by-step action checklist
betriebsrat checklist "<situation>" --agent

# Get structured decision support with recommended action
betriebsrat decide "<situation>" --agent

# Get meeting preparation (agenda, quorum, questions for employer)
betriebsrat prepare-meeting "<topic>" --agent
```

### Step 3 — Compose (advisory response)

Build your advisory response from the command outputs.

**For BR members:**
```
Rechtslage: [co-determination type + applicable §§]
Ihr Recht: [what BR can do — block, demand BV, consult, or inform?]
Frist: [deadline if applicable]
Empfohlene Schritte: [ordered action list from checklist]
Weitere Informationen: [topic_url values from command output]
```

**For employees:**
```
Was das Gesetz sagt: [applicable §§ and what they protect]
Wurde das Verfahren eingehalten?: [was BR consulted correctly? was deadline met?]
Ihre Ansprüche: [what the employee is entitled to if procedure was violated]
Nächste Schritte: [concrete actions — object, consult lawyer, etc.]
```

---

## When to Use Each Command

| Situation | Primary Command | Follow-up |
|-----------|----------------|-----------|
| "I don't know which command to use" | `ask` | any command it suggests |
| "Share with non-technical colleague" | `serve` | — |
| **— Employee questions —** | | |
| "Was the BR consulted before my dismissal?" | `check-anhoerung` | `consequences kündigung` |
| "My dismissal — was it procedurally valid?" | `consequences kündigung` | `check-anhoerung` if you have the letter |
| "Am I entitled to a Sozialplan payment?" | `sozialplan-calc` | `law 112` for entitlement basis |
| "Employer restructured without Interessenausgleich — can I claim?" | `nachteilsausgleich` | `sozialplan-calc` for comparison |
| "Was the BR consulted before my transfer?" | `consequences versetzung` | `rights-check "Versetzung"` |
| "My hiring — did employer skip the BR?" | `consequences einstellung` | `rights-check "Einstellung"` |
| "Does the new AI tool at work trigger co-determination?" | `ki-check` | `consequences software` |
| "How many months' salary is my Sozialplan?" | `sozialplan-calc` | `law 112` |
| **— BR member questions —** | | |
| "Does BR have a say?" | `rights-check` | `decide` for full decision |
| "What kind of right do we have?" | `codetermination-type` | `law` for paragraph detail |
| "When must we respond?" | `deadline` | `checklist` for full process |
| "What do we do step by step?" | `checklist` | `law` for legal basis |
| "What does § X mean?" | `law <n>` | `rights-check` for situation match |
| "How do I prepare this meeting?" | `prepare-meeting` | `checklist` for pre-meeting steps |
| "Help me decide what to do" | `decide` | all follow-ups |
| "Draft a formal BR response" | `letter` | `consequences` for leverage |
| "What happens if we miss deadline?" | `consequences` | `deadline` for exact date |
| "What if employer acts without consent?" | `consequences` | `decide` for action plan |
| "How much Sozialplan is this employee entitled to?" | `sozialplan-calc` | `law 112` for legal basis |
| "Store/update company profile" | `context set` | `context show` to verify |
| "Is this Anhörungsschreiben valid? Does the clock run?" | `check-anhoerung` | `deadline` for exact due date |
| "Draft a Betriebsvereinbarung for homeoffice/software" | `bv-template` | `law 87` for legal basis |
| "Export deadline to calendar" | `deadline ... --ical` | pipe to `.ics` file |
| "Do these layoffs trigger § 17 KSchG?" | `massenentlassung` | `law 17` for legal detail |
| "What are the strongest Widerspruch grounds?" | `widerspruch-check` | `letter kündigung --type widerspruch` |
| "Generate BR meeting minutes template" | `protokoll` | — |
| "Calculate Sozialplan for all affected employees" | `sozialplan-calc --csv` | `sozialplan-calc` per individual |
| "Request documents/data from employer" | `auskunft` | `consequences` for enforcement |
| "Does this AI system trigger co-determination?" | `ki-check` | `bv-template software` for draft BV |
| "Employer bypassed Interessenausgleich — what can employees claim?" | `nachteilsausgleich` | `sozialplan-calc` for comparison |
| "Send a BR training request to the employer" | `schulungsantrag` | `law 37` for legal detail |
| "Can we conclude a BV on this topic, or does the Tarifvertrag block it?" | `tarifvertrag-check` | `bv-template` to draft if allowed |
| "Draft a BV for camera surveillance" | `bv-template videoüberwachung` | `ki-check` if digital system involved |
| "Draft a BV for performance appraisals" | `bv-template leistungsbeurteilung` | `law 94` for legal basis |

---

## Scenario Playbooks

### Kündigung (Dismissal) — § 102 BetrVG

The employer is dismissing an employee. BR must be heard before every dismissal.

```bash
# 1. Calculate your deadline (runs first — deadlines are the #1 risk)
betriebsrat deadline "ordentliche Kündigung" --from $(date +%Y-%m-%d) --agent
# For extraordinary dismissal:
betriebsrat deadline "außerordentliche Kündigung" --from $(date +%Y-%m-%d) --agent

# 2. Get the full action checklist
betriebsrat checklist "Kündigung" --agent

# 3. Get the legal basis explained
betriebsrat law 102 --agent

# 4. Check co-determination type
betriebsrat codetermination-type "Kündigung Anhörung" --agent

# 5. Draft the formal response (Stellungnahme or Widerspruch)
betriebsrat letter kündigung --type widerspruch --employee "Name" --ground "fehlerhafte Sozialauswahl" --agent
betriebsrat letter kündigung --type zustimmung --employee "Name" --agent

# 6. Understand consequences of missed deadline
betriebsrat consequences kündigung --agent
```

**Key facts for Kündigung:**
- Ordinary dismissal: BR has **1 week** to respond (§ 102 Abs. 2)
- Extraordinary dismissal: BR has **3 days** (§ 102 Abs. 2 S. 3)
- Silence = consent — BR MUST respond within the window or forfeits rights
- BR can: consent, object (Widerspruch), or express concern
- Widerspruch grounds (§ 102 Abs. 3): wrong social selection, BV violation, Weiterbeschäftigung elsewhere possible, missing retraining, seniority ignored
- Widerspruch triggers right to **Weiterbeschäftigung** during appeal (§ 102 Abs. 5)

---

### Betriebsänderung (Operational Change) — §§ 111–113 BetrVG

Employer is restructuring: closing sites, significant layoffs, outsourcing, mergers.

```bash
# 1. Verify co-determination rights and scope
betriebsrat rights-check "Betriebsänderung Schließung Standort" --agent
betriebsrat codetermination-type "Betriebsänderung" --agent

# 2. Get full step-by-step checklist
betriebsrat checklist "Betriebsänderung" --agent

# 3. Understand the legal instruments
betriebsrat law 111 --agent  # What counts as Betriebsänderung
betriebsrat law 112 --agent  # Interessenausgleich + Sozialplan

# 4. Prepare the first meeting
betriebsrat prepare-meeting "Betriebsänderung § 111" --agent

# 5. Get structured decision support
betriebsrat decide "Arbeitgeber plant Schließung eines Standorts" --agent

# 6. Send formal letters
betriebsrat letter betriebsänderung --type unterrichtung --measure "Schließung Filiale Hamburg" --affected 45 --employer "Firma GmbH" --agent
betriebsrat letter betriebsänderung --type interessenausgleich --measure "Verlagerung Produktion nach Polen" --affected 120 --agent
```

**Key facts for Betriebsänderung:**
- BR must be informed and consulted **before** the decision is implemented (not just before execution)
- Two instruments: **Interessenausgleich** (try to avoid/limit) + **Sozialplan** (compensate those affected)
- Sozialplan is **erzwingbar** — BR can force it via Einigungsstelle
- Interessenausgleich is NOT erzwingbar — employer can act without agreement but must pay **Nachteilsausgleich** (§ 113)
- Threshold: generally ≥20% of workforce or absolute numbers from § 111 BetrVG (varies by company size)
- Trigger early: BR information rights start immediately upon employer decision-making — not just when announced publicly

---

### Software-Einführung / KI-Systeme — § 87 Abs. 1 Nr. 6 BetrVG

Employer wants to introduce new software, monitoring tools, AI systems, or performance tracking.

```bash
# 1. Check co-determination right (usually erzwingbar under § 87 Nr. 6)
betriebsrat rights-check "Einführung Software Leistungsüberwachung KI" --agent

# 2. Classify the right type precisely
betriebsrat codetermination-type "Überwachungssoftware" --agent

# 3. Get the legal basis
betriebsrat law 87 --agent  # Social co-determination, § 87 Abs. 1 Nr. 6

# 4. Get decision framework
betriebsrat decide "Arbeitgeber will KI-System einführen das Mitarbeiter bewertet" --agent

# 5. Prepare for negotiation
betriebsrat prepare-meeting "Einführung KI-System § 87" --agent
```

**Key facts for Software-Einführung:**
- § 87 Abs. 1 Nr. 6: **erzwingbare Mitbestimmung** for technical equipment *capable* of monitoring employee behavior or performance
- The monitoring capability triggers the right — even if the employer says "we won't use it for monitoring"
- Applies to: surveillance cameras, keyloggers, time-tracking, AI tools with employee data, Teams/Slack analytics, GitHub telemetry, code-review AI
- BR can **block the introduction** without an agreed Betriebsvereinbarung (BV)
- BV must cover: purpose, data collected, access rights, retention/deletion schedule, prohibition on disciplinary use

---

### Einstellung / Versetzung (Hiring / Transfer) — §§ 99–101 BetrVG

Employer wants to hire someone or transfer an existing employee to a different role/location.

```bash
# 1. Check co-determination right
betriebsrat rights-check "Einstellung Neueinstellung Versetzung" --agent

# 2. Get the legal framework
betriebsrat law 99 --agent   # Consent requirement for hiring/transfer
betriebsrat law 100 --agent  # Provisional measures without consent

# 3. Understand refusal grounds
betriebsrat decide "Arbeitgeber will neuen Mitarbeiter einstellen ohne BR zu fragen" --agent

# 4. Get checklist for the process
betriebsrat checklist "Einstellung" --agent
```

**Key facts for Einstellung/Versetzung:**
- **Zustimmungsvorbehalt** — employer needs BR consent (§ 99 Abs. 1)
- BR has **1 week** to respond (silence = consent)
- Grounds to refuse consent (§ 99 Abs. 2): BV violation, legal violation, existing employee disadvantage, no internal job posting (§ 93), wrong social selection
- Employer can proceed **without consent** in urgent cases (§ 100) but must apply to labor court within **3 days**
- If labor court rejects: employer must reverse the measure

---

### Massenentlassung (Mass Dismissal) — § 17 KSchG + §§ 111–113 BetrVG

Employer plans large-scale layoffs. § 17 KSchG adds a notification procedure on top of the regular § 102 hearing.

```bash
# 1. Check if § 17 KSchG threshold is met
betriebsrat massenentlassung --employees 200 --planned 25 --agent

# 2. If triggered: check BR rights for the Betriebsänderung (runs in parallel)
betriebsrat rights-check "Massenentlassung Stellenabbau" --agent
betriebsrat law 17 --agent   # § 17 KSchG notification procedure
betriebsrat law 112 --agent  # Sozialplan (erzwingbar)

# 3. Get Betriebsänderung checklist and decision support
betriebsrat checklist "Betriebsänderung Massenentlassung" --agent
betriebsrat decide "Arbeitgeber kündigt 25 von 200 Mitarbeitern" --agent

# 4. Calculate Sozialplan for all affected employees (batch mode)
betriebsrat sozialplan-calc --csv affected_employees.csv --factor 0.75 --max-cap 80000 --agent

# 5. Advise on Widerspruch grounds for each individual dismissal
betriebsrat widerspruch-check --type betriebsbedingt --seniority-ignored --other-position --agent

# 6. Generate BR resolution minutes for the Widerspruch vote
betriebsrat protokoll --topic "Massenentlassung: § 102-Anhörung und Widerspruch" --br-size 7 --agent

# 7. Draft the Betriebsrat Stellungnahme letter
betriebsrat letter betriebsänderung --type unterrichtung --measure "Abbau von 25 Stellen" --affected 25 --agent
```

**Key facts for Massenentlassung:**
- § 17 KSchG is **in addition to** § 102 BetrVG — both must be satisfied
- Failing to file the Massenentlassungsanzeige makes ALL terminations void (BAG 2016)
- The BR Stellungnahme must be attached to the Anzeige to the Agentur für Arbeit
- 1-month Sperrfrist after Anzeige before dismissals can take effect (extendable to 2 months)
- Sozialplan is **erzwingbar** — if negotiations fail, the Einigungsstelle decides

---

### Homeoffice / Mobile Work — §§ 87 Abs. 1 Nr. 2, 14 BetrVG

Employer wants to introduce, change, or end homeoffice/remote work arrangements.

```bash
# 1. Check rights
betriebsrat rights-check "Homeoffice mobile Arbeit Telearbeit Einführung Abschaffung" --agent

# 2. Get the legal basis
betriebsrat law 87 --agent   # Working hours (Nr. 2) + mobile work (Nr. 14)

# 3. Get structured decision support
betriebsrat decide "Arbeitgeber will Homeoffice abschaffen" --agent

# 4. Prepare the meeting
betriebsrat prepare-meeting "Homeoffice-Regelung Betriebsvereinbarung" --agent
```

**Key facts for Homeoffice:**
- § 87 Abs. 1 Nr. 2 (working hours) and Nr. 14 (mobile work, added 2021): **erzwingbare Mitbestimmung**
- BV should cover: who qualifies, equipment (employer provides?), ergonomics, reachability hours, data protection, accident coverage, cost reimbursement
- Employer **cannot unilaterally end** homeoffice governed by a BV without renegotiating
- Individual agreements do not replace a BV — BV governs the framework for everyone

---

### Betriebsübergang (Business Transfer) — § 613a BGB + §§ 111–113 BetrVG

Employer is selling the business, outsourcing a department, or merging — employment contracts transfer automatically to the new employer.

```bash
# 1. Check BR information and consultation rights
betriebsrat rights-check "Betriebsübergang Unternehmensverkauf Outsourcing" --agent

# 2. Get the legal framework
betriebsrat law 613a --agent    # Transfer of employment contracts, void dismissal rule
betriebsrat law 111 --agent    # Betriebsänderung rights (almost always triggered)

# 3. Get structured decision support
betriebsrat decide "Arbeitgeber verkauft Betrieb an anderen Unternehmer" --agent

# 4. Get step-by-step checklist
betriebsrat checklist "Betriebsübergang" --agent

# 5. Understand consequences if employer skips the information duty
betriebsrat consequences betriebsänderung --agent

# 6. If layoffs are planned: check thresholds and draft Sozialplan
betriebsrat massenentlassung --employees 200 --planned 30 --agent
betriebsrat sozialplan-calc --salary 4500 --years 10 --age 45 --factor 0.75 --agent

# 7. Request the transfer documentation
betriebsrat auskunft --topic planung --reason "Betriebsübergang § 613a BGB — Unterrichtungspflicht" --agent
```

**Key facts for Betriebsübergang:**
- Contracts transfer **automatically** — employees do not need to consent (§ 613a Abs. 1)
- Every employee must be **informed in writing** before the transfer: date, reason, legal consequences, rights (§ 613a Abs. 5)
- Employees have **1 month** to object to the transfer (§ 613a Abs. 6) — they stay with the old employer (risk: redundancy)
- Dismissal **because of** the transfer is void (§ 613a Abs. 4); dismissal for other reasons is still valid
- Existing BVs (Betriebsvereinbarungen) continue as individual contractual terms for **1 year** unless superseded by new BVs
- BR information right: the transfer almost always constitutes a Betriebsänderung (§ 111) — Interessenausgleich and Sozialplan rights apply
- If employer skips Interessenausgleich: every affected employee has a **Nachteilsausgleich claim** (§ 113)

---

### Kurzarbeit (Short-time Work) — § 87 Abs. 1 Nr. 3 BetrVG

Employer wants to reduce working hours to avoid layoffs. BR consent is mandatory before any Kurzarbeit can be introduced.

```bash
# 1. Check co-determination right (erzwingbar)
betriebsrat rights-check "Kurzarbeit Arbeitszeitreduzierung" --agent
betriebsrat codetermination-type "Kurzarbeit" --agent

# 2. Get the legal framework
betriebsrat law 87 --agent   # § 87 Abs. 1 Nr. 3: temporary reduction/extension of working hours

# 3. Get structured decision support
betriebsrat decide "Arbeitgeber will Kurzarbeit einführen" --agent

# 4. Get the checklist (BV required before introduction)
betriebsrat checklist "Kurzarbeit" --agent

# 5. Draft the Betriebsvereinbarung
betriebsrat bv-template arbeitszeit --employer "Musterfirma GmbH" --agent

# 6. Prepare the meeting
betriebsrat prepare-meeting "Kurzarbeit § 87 Nr. 3" --agent
```

**Key facts for Kurzarbeit:**
- § 87 Abs. 1 Nr. 3: **erzwingbare Mitbestimmung** — employer cannot implement Kurzarbeit without BR agreement
- A BV (or at minimum an individual agreement per employee via § 87 Abs. 2) is required
- BV must cover: scope (which departments), duration, amount of reduction, notice period, Kurzarbeitergeld entitlement
- Employer must notify the Agentur für Arbeit to claim Kurzarbeitergeld (KUG) — BR agreement is a prerequisite
- BR can use this as leverage to negotiate: retraining measures, re-hiring commitments, Sozialplan protections
- Kurzarbeit does NOT substitute for a Massenentlassung procedure if the reduction is permanent — check § 17 KSchG if any terminations are also planned

---

### § 103 BetrVG — Dismissal of a BR Member

Employer wants to dismiss an employee who is a works council member. Ordinary dismissal is banned; extraordinary dismissal requires BR consent and potentially labor court approval.

```bash
# 1. Get the legal framework (fundamentally different from § 102)
betriebsrat law 103 --agent    # Extraordinary dismissal of BR member
betriebsrat law 15 --agent     # § 15 KSchG: dismissal protection during term + 1 year after

# 2. Classify the situation and get decision support
betriebsrat decide "Arbeitgeber will Betriebsratsmitglied außerordentlich kündigen" --agent

# 3. Get checklist for the BR response
betriebsrat checklist "§ 103 Kündigung Betriebsratsmitglied" --agent

# 4. Understand what happens if BR refuses
betriebsrat consequences kündigung --agent  # consequences if employer bypasses consent

# 5. Generate the BR resolution minutes (required — this is a BR resolution, not just a letter)
betriebsrat protokoll --topic "§ 103 Zustimmungsverweigerung Kündigung [Name]" --br-size 7 --agent

# 6. Draft the formal Zustimmungsverweigerung letter
betriebsrat letter kündigung --type verweigerung --employee "[Name]" --ground "§ 103 BetrVG — Zustimmung verweigert" --agent
```

**Key facts for § 103:**
- **Ordinary dismissal is completely prohibited** for BR members during their term and for 1 year after (§ 15 KSchG) — exceptions only at company closure
- Extraordinary (fristlose) dismissal: employer must apply to BR for **Zustimmung** (§ 103 Abs. 1)
- BR has **3 days** to respond (extraordinary dismissal — same as § 102 Abs. 2 S. 3); silence ≠ consent (unlike § 102)
- If BR **refuses** or does not respond: employer must apply to labor court to **substitute consent** (Ersetzungsverfahren, § 103 Abs. 2)
- Labor court substitutes consent only if the extraordinary dismissal is legally justified (wichtiger Grund, § 626 BGB)
- Until the court decides: the BR member continues working
- The BR member being dismissed is **excluded from voting** on the Zustimmung (§ 25 Abs. 1 S. 1 — conflict of interest)
- Applies equally to: BR members, Ersatzmitglieder during active service, Wahlbewerber during election, JAV members

---

### Überstunden / Mehrarbeit (Overtime) — § 87 Abs. 1 Nr. 3 BetrVG

Employer wants employees to work beyond regular hours. BR has an erzwingbares Mitbestimmungsrecht.

```bash
# 1. Check co-determination right
betriebsrat rights-check "Überstunden Mehrarbeit Arbeitszeitverlängerung" --agent
betriebsrat codetermination-type "Überstunden Anordnung" --agent

# 2. Get the legal framework
betriebsrat law 87 --agent   # § 87 Abs. 1 Nr. 3: temporary extension of working hours

# 3. Get structured decision support
betriebsrat decide "Arbeitgeber ordnet Überstunden an ohne BR-Zustimmung" --agent

# 4. Understand consequences if employer bypasses the BR
betriebsrat consequences software --agent  # substitute with: consequences for § 87 bypasses
```

**Key facts for Überstunden:**
- § 87 Abs. 1 Nr. 3: **erzwingbare Mitbestimmung** for temporary extension of working hours
- Applies to **anordnung** (employer ordering overtime), not volunteered overtime
- BR can refuse consent — without BR agreement, employer may not order overtime unilaterally
- Exception: true emergencies (natural disaster, sudden breakdown) where delay would cause disproportionate harm
- A standing BV on overtime (Rahmen-BV) is the cleanest solution: sets daily/weekly limits, approval process, compensation
- AR (Arbeitszeitkonto) — time credits also require a BV under § 87 Nr. 2
- ArbZG limits: generally max 10 hours/day, 48 hours/week averaged — BR should flag violations even if they agree to overtime

---

### BR-Wahl (Works Council Election) — §§ 14–21 BetrVG

No BR currently exists (or term has ended) and employees want to elect one.

```bash
# 1. Check if a BR can be elected
betriebsrat rights-check "BR-Wahl Betriebsrat gründen wählen" --agent
betriebsrat law 14 --agent    # Regular election procedure
betriebsrat law 17 --agent    # Election when no BR exists (initiated by 3 employees or union)

# 2. Get step-by-step election checklist
betriebsrat checklist "BR-Wahl" --agent

# 3. Understand employer obstruction consequences
betriebsrat consequences kündigung --agent  # § 119 BetrVG: obstruction is a criminal offence
```

**Key facts for BR election:**
- **Threshold:** ≥ 5 permanently employed workers in an establishment (§ 1 Abs. 1 BetrVG)
- **Who can initiate:** 3 eligible employees, a trade union with members in the company (§ 17 Abs. 2)
- **Wahlvorstand** (election committee): appointed by existing BR (§ 16) or by labor court / union if no BR exists (§ 17)
- **Voting rights (aktiv):** all employees aged ≥ 18, including fixed-term and part-time (§ 7)
- **Eligibility (passiv):** ≥ 18, employed for at least 6 months in the company (§ 8)
- **Term:** 4 years; regular elections every 4 years in March–May (§ 21)
- **BR size** (§ 9): 5–20 AN → 1; 21–50 → 3; 51–100 → 5; 101–200 → 7; 201–400 → 9; and so on
- **Employer obstruction is a criminal offence** (§ 119 BetrVG): up to 1 year imprisonment or fine — applies to interference, threatening voters, or preventing the election
- **Cost:** all election costs are borne by the employer (§ 20 Abs. 3)
- **Simplified procedure** (vereinfachtes Wahlverfahren): mandatory for establishments with 5–100 employees, optional for 101–200

---

### § 85 BetrVG — Beschwerdeverfahren (Employee Complaints)

An employee has a complaint — about their supervisor, working conditions, workload, unfair treatment, or a colleague — and wants the BR to act.

```bash
# 1. Get the legal framework
betriebsrat law 85 --agent   # Employee's right to file a complaint via BR
betriebsrat law 84 --agent   # Direct individual complaint right (no BR involvement needed)

# 2. Get structured decision support
betriebsrat decide "Arbeitnehmer beschwert sich beim Betriebsrat über Arbeitgeber" --agent

# 3. Get checklist for BR handling the complaint
betriebsrat checklist "Beschwerdeverfahren § 85" --agent
```

**Key facts for Beschwerdeverfahren:**
- Every employee has the right to file a complaint with the BR (§ 85 Abs. 1)
- BR must **examine the complaint** and, if it considers it justified, work toward a remedy with the employer
- Employer must respond within **1 week** to the BR's request for remedy
- Employee has the right to be **present** at the discussion between BR and employer
- If employer refuses: BR can bring the matter to the labor court (§ 85 Abs. 2 — rarely used but real)
- BR **cannot** be forced to act on every complaint — but systematic non-engagement is a failure of statutory duty
- Complaints involving bullying/Mobbing: BR can invoke § 75 (joint duty to protect employees' personality rights) as additional legal basis
- Distinguish from § 84 (direct individual complaint right without BR) — employees have both paths; they are independent

---

### Employee: "Was procedure followed? What am I entitled to?"

An employee is affected by a dismissal, transfer, or restructuring and wants to know if the BR was involved correctly and what they can claim.

```bash
# Dismissal: check if BR was properly consulted
betriebsrat check-anhoerung "<text of the Anhörungsschreiben>" --type ordentlich --agent
# → Shows: which required fields are present/missing, whether 7-day clock ran correctly

# If the Anhörung was incomplete: find out what that means for the dismissal
betriebsrat consequences kündigung --agent --lang en
# → Shows: dismissal may be void; employee can object in labour court

# Restructuring/layoff: check if Sozialplan applies
betriebsrat law 112 --agent --lang en
# → Shows: Sozialplan is erzwingbar; employees have an individual entitlement

# Calculate Sozialplan entitlement
betriebsrat sozialplan-calc --salary 4500 --years 8 --age 42 --factor 0.75 --lang en --agent

# If employer skipped Interessenausgleich: calculate Nachteilsausgleich claim
betriebsrat nachteilsausgleich --salary 4500 --years 8 --measure "Standortschließung" --no-ia-attempted --lang en --agent
# → This is ADDITIVE to any Sozialplan payment (with offset — sozialplan-calc shows the offset)

# Transfer without BR consent: check if the measure is void
betriebsrat consequences versetzung --agent --lang en
# → Shows: employer must reverse the transfer if labour court finds no consent was obtained
```

**Key employee facts:**
- A dismissal where the BR was not properly consulted (or Anhörung was incomplete) can be **void** — challenge in labour court within 3 weeks
- A Sozialplan is **legally enforceable** — employees have a direct claim even if the BV is silent on individual amounts; use `sozialplan-calc` to estimate
- Nachteilsausgleich (§ 113) is a **personal claim** independent of the Sozialplan — runs in parallel, not instead of it
- Transfer/hiring without BR consent: employer may have to **reverse the measure**; employee can rely on the invalidity

---

## Unique Capabilities

### Decision support
- **`rights-check`** — Answers 'Does the Betriebsrat have a say in this?' — maps situation to BetrVG paragraphs and co-determination type

  ```bash
  betriebsrat rights-check "employer wants to introduce home office policy" --agent
  ```

- **`decide`** — Step-by-step decision support: classify situation, find applicable §§, determine BR rights, recommend action

  ```bash
  betriebsrat decide "Arbeitgeber kündigt 15 Mitarbeiter" --agent
  ```

- **`checklist`** — Generates step-by-step action checklist for BR in a given situation

  ```bash
  betriebsrat checklist "Betriebsänderung" --agent
  ```

- **`codetermination-type`** — Classifies BR rights as: Mitbestimmung (erzwingbar) / Mitwirkung / Unterrichtung / keine

  ```bash
  betriebsrat codetermination-type "Versetzung" --agent
  ```

### Legal deadlines
- **`deadline`** — Calculates legal deadlines for BR response

  ```bash
  betriebsrat deadline "ordentliche Kündigung" --from 2026-05-10 --agent
  ```

### Meeting tools
- **`prepare-meeting`** — Generates agenda, quorum rules, questions for employer for a BR meeting on a specific topic

  ```bash
  betriebsrat prepare-meeting "Einführung KI-System" --agent
  ```

### Legal reference
- **`law`** — Plain-language explanation of any BetrVG paragraph

  ```bash
  betriebsrat law 87 --agent
  ```

### Document drafting
- **`letter`** — Draft a formal BR letter: Stellungnahme, Widerspruch, Zustimmung, Verweigerung, Unterrichtungsverlangen, Interessenausgleich

  _The most practical command for day-to-day BR work. Generates a ready-to-edit German letter with correct legal references and structure._

  ```bash
  betriebsrat letter kündigung --type widerspruch --employee "Max Mustermann" --ground "fehlerhafte Sozialauswahl" --agent
  betriebsrat letter einstellung --type verweigerung --employee "Anna Schmidt" --ground "Verstoß gegen § 93 BetrVG" --agent
  betriebsrat letter versetzung --type zustimmung --employee "Peter Müller" --agent
  betriebsrat letter betriebsänderung --type unterrichtung --measure "Schließung Standort X" --affected 60 --agent
  betriebsrat letter betriebsänderung --type interessenausgleich --measure "Verlagerung Produktion" --affected 120 --agent
  ```

  Types for `kündigung`: `zustimmung` | `bedenken` | `widerspruch`
  Types for `einstellung`/`versetzung`: `zustimmung` | `verweigerung`
  Types for `betriebsänderung`: `unterrichtung` | `interessenausgleich`
  Flags for `betriebsänderung`: `--measure "<Maßnahme>"` `--affected <Anzahl>`

### Consequences
- **`consequences`** — What happens if BR misses a deadline or employer acts without consent?

  _Critical for understanding leverage and urgency. Know the exact legal consequences before deciding how to respond._

  ```bash
  betriebsrat consequences kündigung --agent
  betriebsrat consequences einstellung --agent
  betriebsrat consequences betriebsänderung --agent
  betriebsrat consequences software --agent
  ```

  Situations: `kündigung` | `einstellung` | `versetzung` | `betriebsänderung` | `software` | `br-deadline`

### Sozialplan calculation
- **`sozialplan-calc`** — Calculates individual or batch Sozialplan entitlement using the Munich formula

  _Use when a Betriebsänderung is happening and you need to estimate what each affected employee is entitled to. Use `--csv` for batch mode across all affected employees._

  ```bash
  # Single employee
  betriebsrat sozialplan-calc --salary 4500 --years 8 --age 42 --factor 0.75 --agent
  betriebsrat sozialplan-calc --salary 6000 --years 15 --age 58 --disabled --children 2 --factor 1.0 --agent
  # Batch mode — CSV: name,salary,years,age,disabled,children[,factor[,max_cap]]
  betriebsrat sozialplan-calc --csv employees.csv --factor 0.75 --max-cap 80000 --agent
  ```

  Formula: `Betriebszugehörigkeit × Monatsgehalt × Faktor`  
  Adjustments: +25% disabled, +10%/child (max 3), +5% if age ≥55  
  Factors: 0.5 (floor) · 0.75 (standard) · 1.0 (typical) · 1.5 (strong BR position)

### Massenentlassung threshold check
- **`massenentlassung`** — Checks whether § 17 KSchG applies and generates the complete compliance procedure

  _Always run this when large-scale dismissals are planned. Missing the Massenentlassungsanzeige makes all terminations void._

  ```bash
  betriebsrat massenentlassung --employees 150 --planned 25 --agent
  betriebsrat massenentlassung --employees 500 --planned 35 --agent
  ```

  Thresholds: 21–59 AN → ≥6 | 60–499 AN → ≥10% or ≥26 | ≥500 AN → ≥30  
  Output: triggered/not, 7-step procedure with deadlines, consequences if skipped

### Widerspruch grounds advisor
- **`widerspruch-check`** — Advises which § 102 Abs. 3 Widerspruch grounds are available and strongest

  _A Widerspruch (§ 102 Abs. 3) — unlike Bedenken (§ 102 Abs. 2) — gives the employee the right to continued employment during appeal (§ 102 Abs. 5). Use this command to pick the right grounds._

  ```bash
  betriebsrat widerspruch-check --type betriebsbedingt --wrong-social-selection --other-position --employee "Max Mustermann" --agent
  betriebsrat widerspruch-check --type verhaltensbedingt --no-warning --agent
  betriebsrat widerspruch-check --type betriebsbedingt --seniority-ignored --retraining --agent
  ```

  Grounds (§ 102 Abs. 3 Nr. 1–5): BV violation · wrong social selection · other position exists · retraining possible · changed terms possible  
  Output: applicable grounds ranked by strength, draft Widerspruch text ready to use in a letter

### Information requests
- **`auskunft`** — Drafts a formal § 80 BetrVG information request letter to the employer

  _The BR's most-used leverage tool. Use it to demand social data for Sozialauswahl, org charts, salary structures, AI system documentation, or any other information needed for the BR's statutory tasks. The letter includes the enforcement threat (labour court application)._

  ```bash
  betriebsrat auskunft --topic sozialdaten --reason "Prüfung Sozialauswahl § 102" --employer "Firma GmbH"
  betriebsrat auskunft --topic ki --reason "Einführung KI-Bewertungssystem" --deadline-days 10 --agent
  betriebsrat auskunft --topic custom --custom "Überstundenaufstellungen letzter 12 Monate" --lang en
  ```

  Topics: `sozialdaten` · `stellenplan` · `gehaelter` · `planung` · `auswahlrichtlinien` · `ki` · `wirtschaft` · `custom`  
  Letter is always in German (legal document); metadata/notes switch with `--lang en`

### AI/IT co-determination check
- **`ki-check`** — Analyses whether an AI or IT system triggers § 87 Abs. 1 Nr. 6 co-determination

  _The most important tool for the current wave of AI deployments. § 87 Nr. 6 is triggered by the capability to monitor employees, not actual use. Use this to determine whether to block deployment and what the BV must cover._

  ```bash
  betriebsrat ki-check --system "Workday People Analytics" --monitors-performance --influences-hr --auto-decision --lang en
  betriebsrat ki-check --system "GitHub Copilot" --data "keystrokes,accepted suggestions" --agent
  betriebsrat ki-check --system "Slack Workforce Analytics" --monitors-comms --monitors-performance --agent
  ```

  Flags: `--monitors-performance` · `--monitors-location` · `--monitors-comms` · `--influences-hr` · `--biometric` · `--auto-decision`  
  Output: triggered/not, risk level, required BV clauses, what employer cannot do without BV, 4 key BAG rulings

### Nachteilsausgleich calculator
- **`nachteilsausgleich`** — Calculates the § 113 BetrVG disadvantage compensation claim

  _When the employer implements a Betriebsänderung without attempting an Interessenausgleich (or deviates from one already agreed), every affected employee has a personal claim. This is separate from — and additive to — the Sozialplan. Use it to quantify leverage during negotiations._

  ```bash
  betriebsrat nachteilsausgleich --salary 5000 --years 12 --measure "Standortschließung" --no-ia-attempted --factor 0.75 --lang en
  betriebsrat nachteilsausgleich --salary 6000 --years 15 --measure "Verlagerung" --ia-deviated --agent
  ```

  Key rule: any existing Sozialplan payment is offset against the Nachteilsausgleich claim (§ 113 Abs. 3 Hs. 2)  
  Statutory cap: 12 × monthly salary (§ 10 KSchG analogy)

### BR meeting minutes
- **`protokoll`** — Generates a formal BR Sitzungsprotokoll template with quorum calculation

  _BR resolutions are invalid without proper minutes signed by the chair and secretary (§ 34 BetrVG). This covers all required fields._

  ```bash
  betriebsrat protokoll --topic "Kündigung Max Mustermann § 102" --br-size 7 --date 2026-05-15 --agent
  betriebsrat protokoll --topic "Homeoffice-BV Abstimmung" --br-size 11 --employer "Musterfirma GmbH" --agent
  ```

  Output: complete template with attendance sheet, quorum check, TOP structure (with voting rows), and signature block

### Company profile
- **`context`** — Stores and displays company profile for context-aware advice

  ```bash
  betriebsrat context set --employees 150 --sector IT --tariff --tariff-name "TV-L" --br-size 7 --bvs "Homeoffice,Arbeitszeit"
  betriebsrat context show --agent
  betriebsrat context reset
  ```

  Thresholds applied automatically:
  - `employees ≥ 20` → § 111 Betriebsänderung rights active
  - `employees ≥ 100` → § 106 Wirtschaftsausschuss mandatory
  - `employees ≥ 200` → § 38 full-time BR member release required

---

## Command Reference

**ask** — Natural-language entry point — no command knowledge required
- `betriebsrat ask "<question in German or English>"`
- Detects role (employee/BR), language, situation; routes to right analysis; returns audience-appropriate answer
- Extracts salary/years from question for automatic Sozialplan estimate
- `--json` returns structured result with role, classification, paragraphs, actions, deadline, disclaimer

**serve** — Local web chat UI for employees and BR members without terminal access
- `betriebsrat serve [--port 8080]`
- Opens a browser-usable chat interface at `http://localhost:7890`
- POST `/ask` endpoint for integration; GET `/` serves the chat UI
- No external dependencies; works fully for embedded-knowledge questions

**articles** — Individual articles and guides from betriebsrat.de
- `betriebsrat articles` — Search for articles within a topic area

**cases** — Recent Betriebsrat case law (Rechtsprechung)
- `betriebsrat cases` — Fetch recent court decisions relevant to works councils

**glossary** — Legal terms and definitions (Lexikon) for works council members
- `betriebsrat glossary list` — Browse legal terms glossary
- `betriebsrat glossary search` — Search for a specific legal term

**topics** — Betriebsrat topic areas with articles, guides, and practical tips
- `betriebsrat topics get` — Fetch full topic overview page with articles and guides
- `betriebsrat topics list` — List all topic areas (35+ Betriebsrat topics A-Z)

**context** — Store and display company profile for calibrated, threshold-aware advice
- `betriebsrat context set --employees <n> [--sector <s>] [--tariff] [--br-size <n>] [--bvs "<topics>"]`
- `betriebsrat context show` — Display profile and applicable BetrVG thresholds
- `betriebsrat context reset` — Delete stored profile

**sozialplan-calc** — Calculate Sozialplan entitlement (Munich formula), single or batch
- `betriebsrat sozialplan-calc --salary <eur> --years <n> [--age <n>] [--factor <f>] [--disabled] [--children <n>] [--max-cap <eur>]`
- `betriebsrat sozialplan-calc --csv <file> [--factor <f>] [--max-cap <eur>]` — CSV: `name,salary,years,age,disabled,children[,factor[,max_cap]]`

**massenentlassung** — Check § 17 KSchG threshold and generate compliance procedure
- `betriebsrat massenentlassung --employees <n> --planned <n>` — both flags required
- Output: triggered/not, 7-step procedure (BR consultation → Stellungnahme → Interessenausgleich/Sozialplan → Anzeige → Sperrfrist → § 102 per person → Kündigung), legal consequences

**widerspruch-check** — Advise on § 102 Abs. 3 Widerspruch grounds and draft ground text
- `betriebsrat widerspruch-check [--type betriebsbedingt|verhaltensbedingt|personenbedingt] [--wrong-social-selection] [--seniority-ignored] [--other-position] [--retraining] [--reduced-hours] [--bv-violation] [--no-warning] [--employee "<name>"]`
- Output: applicable grounds ranked by strength, draft Widerspruch text, deadline reminder

**auskunft** — Draft a formal § 80 BetrVG information request letter
- `betriebsrat auskunft --topic <topic> [--custom "<text>"] [--reason "<text>"] [--employer "<name>"] [--deadline-days <n>] [--date YYYY-MM-DD] [--lang en|de]`
- Topics: `sozialdaten` · `stellenplan` · `gehaelter` · `planung` · `auswahlrichtlinien` · `ki` · `wirtschaft` · `custom`

**ki-check** — Check § 87 Nr. 6 co-determination for an AI/IT system
- `betriebsrat ki-check --system "<description>" [--purpose "<text>"] [--data "<categories>"] [--monitors-performance] [--monitors-location] [--monitors-comms] [--influences-hr] [--biometric] [--auto-decision] [--lang en|de]`
- Output: triggered/not, risk level (low/medium/high/very high), required BV clauses, employer prohibitions, 4 BAG rulings

**nachteilsausgleich** — Calculate § 113 BetrVG disadvantage compensation claim
- `betriebsrat nachteilsausgleich --salary <eur> --years <n> [--age <n>] [--measure "<text>"] --no-ia-attempted | --ia-deviated [--factor <f>] [--lang en|de]`
- Cap: 12 × monthly salary; Sozialplan offset applies; evidence checklist included

**protokoll** — Generate formal BR Sitzungsprotokoll template
- `betriebsrat protokoll [--topic "<text>"] [--date YYYY-MM-DD] [--br-size <n>] [--location "<text>"] [--employer "<text>"]`
- Output: complete template with quorum calculation, attendance sheet, TOP structure, voting rows, signature block

**check-anhoerung** — Check a § 102 Anhörungsschreiben for completeness
- `betriebsrat check-anhoerung "<letter text>" [--type ordentlich|außerordentlich]`
- Reports: which required fields are present/missing, whether 7-day clock is running, severity per gap

**bv-template** — Generate a skeleton Betriebsvereinbarung
- `betriebsrat bv-template <topic> [--employer "<name>"] [--date YYYY-MM-DD]`
- Topics: `homeoffice` | `software` | `arbeitszeit` | `datenschutz` | `videoüberwachung` | `leistungsbeurteilung`

**schulungsantrag** — Draft a § 37 Abs. 6 BetrVG training request letter
- `betriebsrat schulungsantrag --topic <topic> [--training-name "<name>"] [--provider "<name>"] [--employer "<name>"]`
- Topics: `betrvg` | `arbeitsrecht` | `betriebsrat-praxis` | `kuendigung` | `sozialplan` | `datenschutz` | `gesundheit` | `custom`
- Output: complete letter with legal justification, including cost and release-from-work claims
- `--lang en` supported; letter body stays in German (formal legal document)

**tarifvertrag-check** — Check § 77 Abs. 3 Tarifvorbehalt before drafting a BV
- `betriebsrat tarifvertrag-check --topic <topic> [--tv-type "<type>"] [--tv-covers] [--opening-clause]`
- Topics: `lohn` | `arbeitszeit` | `urlaub` | `zulagen` | `homeoffice` | `software` | `gesundheit` | `custom`
- Output: blocked/not-blocked verdict, what the BV can and cannot cover, legal basis
- Always run this before drafting a BV in a TV-regulated area

**deadline** (updated) — now supports `--ical` flag
- `betriebsrat deadline "ordentliche Kündigung" --from 2026-05-10 --ical > frist.ics`
- Outputs a standard iCalendar file with a 1-day-before reminder; importable into Apple Calendar, Outlook, Google Calendar

**sync** — Populate or refresh the local SQLite knowledge base
- `betriebsrat sync` — Sync all topic areas (run once; safe to re-run)

**search** — Full-text search across the synced knowledge base
- `betriebsrat search "<query>" --data-source local` — Find passages in synced data

### Finding the right command

```bash
betriebsrat which "<capability in your own words>"
```

---

## Auth Setup

No authentication required. Run `betriebsrat doctor` to verify setup.

---

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields
- **Embedded knowledge** — all decision-support commands work instantly (`rights-check`, `decide`, `deadline`, `checklist`, `law`, `codetermination-type`, `consequences`, `letter`, `sozialplan-calc`, `context`, `check-anhoerung`, `bv-template`, `massenentlassung`, `widerspruch-check`, `protokoll`, `auskunft`, `ki-check`, `nachteilsausgleich`, `schulungsantrag`, `tarifvertrag-check`)
- **Bilingual** — add `--lang en` to any command for English output. Legal document bodies always stay in German.

### Response envelope

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

---

## Embedded Reference (CLI_ABSENT Mode)

Use these tables when the CLI is not installed.

### Deadline Matrix

| Situation | § | Deadline | Silence means |
|-----------|---|----------|---------------|
| Ordentliche Kündigung | § 102 Abs. 2 | **1 week** from receipt | Consent |
| Außerordentliche Kündigung | § 102 Abs. 2 S. 3 | **3 days** from receipt | Consent |
| Einstellung / Versetzung | § 99 Abs. 3 | **1 week** from receipt | Consent |
| Provisional Einstellung (§ 100 employer app) | § 100 Abs. 2 | **3 days** for employer to apply to court | Reversal |
| § 17 KSchG Anzeige (Massenentlassung) | § 17 KSchG | File with Agentur für Arbeit **before** notice period begins; 1-month Sperrfrist (extendable to 2) | All terminations void |
| Betriebsänderung consultation | § 111 | Before implementation — no fixed deadline, but employer cannot act unilaterally | Nachteilsausgleich claim |
| BR election objection | § 19 | **2 weeks** from posting of electoral list | |

### Munich Formula (Sozialplan)

`Betriebszugehörigkeit (Jahre) × Monatsgehalt (brutto) × Faktor`

| Factor | Context |
|--------|---------|
| 0.5 | Weak BR position, first negotiation |
| 0.75 | Standard |
| 1.0 | Typical industry benchmark |
| 1.5 | Strong BR position, leverage available |

**Adjustments (cumulative):**
- +25% if severely disabled (GdB ≥ 50)
- +10% per child (max 3 children → +30%)
- +5% if age ≥ 55

**Statutory cap:** No legal cap; parties agree. Common: 12–18 × monthly salary or fixed EUR amount.

**Nachteilsausgleich cap (§ 113):** 12 × monthly salary (§ 10 KSchG analogy). Sozialplan payments are offset.

### § 17 KSchG Threshold Table

| Company size | Dismissals in 30 days that trigger notification |
|-------------|------------------------------------------------|
| 21 – 59 AN | ≥ 6 |
| 60 – 499 AN | ≥ 10% of workforce — capped at 26 (so the **lower** number triggers; at 100 AN: threshold is 10, not 26) |
| ≥ 500 AN | ≥ 30 |

### Company Size Thresholds

| Employees | Unlocked rights |
|-----------|----------------|
| ≥ 5 | BR can be elected |
| ≥ 20 | § 111 Betriebsänderung rights |
| ≥ 100 | § 106 Wirtschaftsausschuss mandatory |
| ≥ 200 | § 38 Abs. 1 full-time BR member release required |

### Co-determination Types

| Type | German | What it means |
|------|--------|---------------|
| Erzwingbare Mitbestimmung | § 87, § 112 | BR can block; dispute goes to Einigungsstelle; employer cannot act without agreement |
| Zustimmungsvorbehalt | § 99, § 103 | Employer needs BR consent; court can substitute consent |
| Mitwirkung / Beratung | § 111 (Interessenausgleich) | Employer must consult; BR cannot block; failure → Nachteilsausgleich |
| Unterrichtung | § 80, § 106 | Employer must inform; no blocking right |

### § 103 BetrVG — BR Member Dismissal Procedure

| Step | Who | Action | Deadline |
|------|-----|--------|----------|
| 1 | Employer | Apply to BR for Zustimmung to extraordinary dismissal | Before dismissal |
| 2 | BR | Vote on Zustimmung (affected member excluded from vote) | **3 days** (extraordinary) |
| 3a | BR consents | Employer may dismiss | — |
| 3b | BR refuses / silence | Employer applies to labor court (Ersetzungsverfahren § 103 Abs. 2) | No fixed deadline |
| 4 | Labor court | Substitutes consent if important reason (§ 626 BGB) exists | Months — member works during proceedings |

- Ordinary dismissal: **banned** during term + 1 year after (§ 15 KSchG)
- Covers: BR members, Ersatzmitglieder (while active), Wahlbewerber (during election), JAV members

### § 613a BGB — Betriebsübergang Key Facts

| Right / Rule | Detail |
|-------------|--------|
| Automatic transfer | Employment contracts move to the new employer by operation of law |
| Written information duty | Both old and new employer must inform each employee **before** the transfer: date, reason, legal consequences, rights |
| Objection window | Employee has **1 month** after written information to object — stays with old employer (redundancy risk) |
| Void dismissal | Dismissal **because of** the transfer is void (§ 613a Abs. 4); other grounds remain valid |
| BV continuity | Existing BVs continue as individual contractual terms for **1 year** unless superseded by new BVs |
| BR rights | Transfer almost always constitutes a Betriebsänderung (§ 111) → Interessenausgleich + erzwingbarer Sozialplan |
| Nachteilsausgleich | If employer skips Interessenausgleich: every affected employee has a personal claim (§ 113) |

### BR Election Quick Reference (§§ 9, 14–21 BetrVG)

| Item | Rule |
|------|------|
| Threshold | ≥ 5 permanently employed workers (§ 1) |
| Who initiates | 3 eligible employees or a union with members in the company (§ 17) |
| Voting rights | All employees ≥ 18 (including fixed-term, part-time) (§ 7) |
| Eligibility | ≥ 18 and ≥ 6 months in the company (§ 8) |
| Term | 4 years; regular elections March–May every 4 years (§ 21) |
| BR size | 5–20 → 1; 21–50 → 3; 51–100 → 5; 101–200 → 7; 201–400 → 9 (§ 9) |
| Simplified procedure | Mandatory for 5–100 AN; optional for 101–200 AN |
| Election costs | Borne entirely by employer (§ 20 Abs. 3) |
| Obstruction | § 119 BetrVG: criminal offence — up to 1 year imprisonment or fine |

### Key § 87 Abs. 1 Co-determination Triggers

| Nr. | Trigger | Type |
|-----|---------|------|
| 2 | Daily working hours, start/end times | Erzwingbar |
| 3 | Temporary reduction or extension of working hours (Kurzarbeit, Überstunden) | Erzwingbar |
| 6 | Technical equipment capable of monitoring employees (IT, AI, cameras) | Erzwingbar |
| 7 | Occupational health and safety rules; stress/workload management | Erzwingbar |
| 10 | Remuneration method (piecework, performance-related pay) | Erzwingbar |
| 14 | Mobile working / homeoffice arrangements | Erzwingbar |

---

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `betriebsrat --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

---

## MCP Server Installation

```bash
claude mcp add betriebsrat-pp-mcp -- betriebsrat-pp-mcp
```

Verify: `claude mcp list`

---

## Direct Use

1. Check if installed: `which betriebsrat`
   If not found, offer to install (see Prerequisites).
2. Match the user query to the best scenario playbook or command.
3. Execute with the `--agent` flag — chain multiple commands for a complete picture.
4. Compose the advisory response using the Classify → Deepen → Compose structure.
