# Policy Intel Research Plan

## Goal

Build a read-only Printing Press CLI for federal rulemaking and policy-monitoring workflows using public government APIs.

## Sources

FederalRegister.gov provides public REST endpoints and states that API keys are not required. Its document search endpoint supports term, agency, document type, publication date, ordering, and pagination filters.

Regulations.gov v4 provides GET APIs for documents, comments, and dockets. The GSA documentation says an api.data.gov key is required and that `DEMO_KEY` may be used for sample calls. The first print uses `POLICY_INTEL_REGULATIONS_API_KEY` when configured and falls back to `DEMO_KEY` for small read-only smoke tests.

## Command Surface

- `federal-register search <term>` searches Federal Register documents.
- `rules <topic>` searches Federal Register rules and proposed rules using the `RULE` and `PRORULE` type filters.
- `docket <docket-id>` fetches Regulations.gov docket metadata.
- `comments <docket-id>` lists public Regulations.gov comments for a docket.
- `deadlines <topic>` lists Regulations.gov documents with comment end dates on or after a selected date.
- `sources` and `doctor` explain source readiness and authentication mode.

## Live Research Findings

- Federal Register search for `artificial intelligence` returned current 2026 documents.
- Federal Register type filters `conditions[type][]=RULE` and `conditions[type][]=PRORULE` returned rules/proposed rules.
- Federal Register agency slugs such as `federal-trade-commission` worked with `conditions[agencies][]`.
- Regulations.gov rejected `page[size]=2`; the public API requires page size 5 or greater.
- Regulations.gov docket detail worked for `EPA-HQ-OPPT-2018-0462`.
- Regulations.gov comments by docket worked with `filter[docketId]`.
- Regulations.gov comment deadline filters worked with `filter[commentEndDate][ge]=YYYY-MM-DD`.

## Non-Goals

- No legal advice.
- No compliance certification.
- No comment submission.
- No lobbying workflow automation.
- No prediction of rulemaking outcomes.
