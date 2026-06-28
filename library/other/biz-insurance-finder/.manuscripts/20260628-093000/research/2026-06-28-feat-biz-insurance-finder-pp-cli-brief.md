# Biz Insurance Finder CLI — Research Brief

## Problem

A US small business buying commercial General Liability (GL) for the first time
faces a fragmented market: a dozen online carriers, brokers, and marketplaces,
each with a different quote form, appetite, and submission model. The same
applicant facts get re-keyed differently into each form, and the wrong carrier
choice wastes days on declines.

## Grounding: a live multi-carrier quote run

This CLI is grounded in a real, hands-on quote run for an importer / private-label
small business (a consumer-products brand that imports and private-labels its own
goods). Quote requests were driven through five providers. Key findings, which the
CLI encodes:

- **The importer / private-label / manufacturer is a "deemed manufacturer"** for
  US product liability. Mainstream instant-quote carriers (e.g. The Hartford,
  biBerk) **declined** that class online; one routed the lead to a phone-transfer
  marketplace.
- **Specialty markets are the fit**: a surplus-lines consumer-products program
  accepted the application; a hard-to-place surplus-lines specialty accepted a
  quote request; a digital agency and a 350+ carrier lead-match marketplace can
  route the class to specialty carriers.
- **A mainstream broker marketplace** returned an instant GL quote, but on a
  retail class — which surfaced the #1 landmine below.

## Lessons baked into the matching logic + warnings

1. **Foreign-products exclusion is the #1 importer landmine.** A standard GL policy
   can quietly exclude imported goods. Confirm in writing there is no
   foreign-products / imported-goods exclusion before binding.
2. **GL Coverage B (Personal & Advertising Injury) excludes patent and most
   trademark infringement** — real brand-IP protection needs a separate IP/media
   policy.
3. **Mainstream instant-quote carriers decline the imported private-label class** —
   route those businesses to specialty markets.
4. **Multi-step broker wizards capture the lead at the CONTACT step** (entering a
   phone number and clicking Next is the submit + a marketing consent). Guidance
   must flag where the real submit happens.
5. **Default the policy effective date to the next business working day.**
6. **Decline optional marketing/SMS consents**; the unavoidable TCPA disclosure on
   lead capture is the cost of an online quote.

## Product thesis

A guided, agent-native CLI that interviews the business once, saves a reusable
profile, and for each matched provider emits a quote-start URL, a paste-ready
answer sheet, and a manual-actions checklist — guiding the user through their own
browser and never filling, submitting, or paying. The matching engine is a pure,
data-driven function over an editable provider registry, with explicit
special-casing for the importer / deemed-manufacturer class.

## Build priorities

1. Editable provider registry (embedded + on-disk override) with per-class appetite.
2. Pure, tested `Match(profile, providers)` with importer special-casing.
3. Answer-sheet generator mapping profile -> paste-ready field values.
4. Manual-actions checklist (CAPTCHA / account / EIN-SSN / payment / two-gate submit).
5. Underwriting warnings encoding the lessons above.
6. Interactive intake; agent-native JSON/CSV/plain output; doctor.
