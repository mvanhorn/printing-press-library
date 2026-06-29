# Agent-driven form filling (the fill harness)

This is the human-in-the-loop form filler. The CLI is the **brain** (it knows the
right markets and the exact values to enter); a browser agent is the **hands**.
Together they reproduce what a human insurance advisor does: fill everything the
applicant already provided, and hand off CAPTCHAs / IDs / payment / submit to a
real person.

The CLI never drives a browser itself. This harness defines how an agent with a
browser tool (Playwright, Claude-in-Chrome, computer-use, etc.) uses the CLI's
`fill-plan` output to fill a provider's quote form safely.

## Prerequisites

- A saved profile: `biz-insurance-finder intake` (or write `profile.json`).
- A browser automation tool available to the agent.

## Protocol

For each provider you want to apply to:

1. **Get the plan** (machine-readable):
   ```bash
   biz-insurance-finder fill-plan <provider-id> --agent
   # or for every shortlisted provider:
   biz-insurance-finder fill-plan --all --agent
   ```
   The JSON has `quote_url`, an `auto_fill[]` list (data the user already
   provided), and a `human_gates[]` list (the steps you must NOT do).

2. **Open** `quote_url` in the browser.

3. **Fill the `auto_fill` fields.** For each entry, find the matching input on
   the live form and enter `value`. Match by meaning, not exact string — each
   carrier labels fields differently. Use `type` (`text|email|tel|date|select|
   checkbox`) as a hint, but adapt to the actual control you find. Page through
   multi-step wizards, filling each step's `auto_fill` values as they appear.
   - Decline optional marketing / SMS consents (uncheck them) — this is part of
     auto-fill, surfaced as the "Marketing / SMS consent" field.

4. **Stop at every `human_gate`.** Never act on these. When the form reaches one,
   pause and tell the user exactly what to do (use the gate's `note`):
   - `captcha` — a CAPTCHA / "I'm not a robot" appeared. The user solves it.
   - `account` — account creation / password. The user does it.
   - `gov_id` — EIN / SSN / government ID. Not stored; the user types it.
   - `payment` — card / bank info. The user does it (only at bind, never a quote).
   - `submit` — the final submit. See two-gate rule below.

5. **Two-gate submit.** When the form is filled, **stop**. Show the user the
   filled values for review (gate 1). The **user** clicks submit (gate 2). The
   agent must never click the final submit/quote button itself — submitting is a
   consequential, consent-bearing action (e.g. TCPA on lead capture). Surface the
   provider's `submit_note` so the user knows where the real submission happens
   (some wizards capture the lead at the contact step).

## Hard rules (never cross)

The agent must NEVER, under any circumstances:
- solve a CAPTCHA,
- create an account or type a password,
- enter an EIN, SSN, or any government ID,
- enter payment / card / bank details,
- click the final submit / get-quote button.

These are exactly the `human_gates` in the fill plan. Everything else — the data
the applicant already gave during intake — is safe to type automatically.

## Why this split is safe

The only things withheld from automation are (a) things automation *can't* do
(CAPTCHA), (b) sensitive identifiers the tool deliberately never stores
(EIN/SSN/payment), and (c) the one irreversible, consent-bearing action (submit).
Re-typing the legal name, address, revenue band, GL limits, and importer
disclosure that the user already provided carries none of those risks — so the
agent fills them, and the human owns only the gates.
