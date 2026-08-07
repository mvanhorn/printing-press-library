# Proposal: add `job-boards/` as a new category to the printing-press-library

## Status

Draft for upstream maintainer review. Filed alongside the `greenhouse-pp-cli`
entry submission so the category lands with a seed entry.

## TL;DR

The library currently has no category that fits Greenhouse, Lever, Workday,
SmartRecruiters, BambooHR, Ashby, or other ATS / job-board CLIs. They get
dumped into `library/other/`, which buries them next to regional pizza shops
and Italian election feeds. A `job-boards/` category gives ATS-shaped CLIs a
home and signals to users "this is where hiring tools live."

## What goes in `job-boards/`

A "job-board" CLI is one that surfaces a public or semi-public feed of job
postings — either as the primary use case (Greenhouse Job Board API,
Lever postings API, Remotive public JSON, WeWorkRemotely RSS) or as the
read-side of a recruiting platform (LinkedIn job search, Indeed search,
Ashby job board). The shared shape: an ATS or board on one side, structured
job listings on the other.

### Distinguishing criteria

A CLI belongs in `job-boards/` if **all three** apply:

1. Its primary API surface lists or describes open positions at one or more
   employers (or for one recruiting platform).
2. The user-facing shape is query-then-apply: the user wants to find roles,
   read details, and either submit via the platform or follow an external
   apply URL.
3. There is at least one board_token / org / company / workspace identifier
   that scopes the query.

### What does NOT go in `job-boards/`

- HRIS platforms (BambooHR person records, Gusto payroll, Rippling) — those
  are employee lifecycle, not job listings. Keep in `sales-and-crm/` or
  `other/` until the HRIS category splits out.
- LinkedIn / Indeed if they only ever return your own network — those are
  social/messaging surfaces.
- Recruiting CRMs (Greenhouse Recruiting, Lever Nurture) with full ATS
  write surface — that's the same vendor as the job board but the
  operations are different. Two entries can coexist (job-boards/
  greenhouse vs sales-and-crm/ greenhouse-recruiting).

## Naming convention

- Category slug: `job-boards/` (kebab-case, plural, matches existing
  pattern: `sales-and-crm/`, `media-and-entertainment/`,
  `developer-tools/`).
- Entry slug: vendor name in kebab-case, scoped to the public-job-board
  surface: `greenhouse/`, `lever/`, `workday/`, `ashby/`, `smartrecruiters/`,
  `remotive/`, `weworkremotely/`.
- No prefix on the slug — the category is in the path, not the name.

## Seed entry: `library/job-boards/greenhouse/`

This PR includes the first entry under the proposed category:
`greenhouse-pp-cli` (slug: `greenhouse`).

- Public Job Board API, 4 operations, no auth
- ~600 lines of original Go (the spec + .printing-press-patches/ override)
- Verified end-to-end against Stripe (524 jobs) and Anthropic (411 jobs)
- Manuscripted under `.manuscripts/greenhouse-job-board-2026-07-19/`
- All publish-validate gates pass except 3 upstream-engine bugs:
  `transcendence`, `phase5`, `verify-skill`. Same gates fail on every
  entry in the library today (verified against `library/productivity/notion`
  baseline).

## Suggested follow-up entries

If `job-boards/` lands, these would be the natural next additions:

| Entry | Source | Auth | Why it's worth it |
|---|---|---|---|
| `lever` | postings API | API key | Second-biggest ATS after Greenhouse |
| `remotive` | public JSON | none | Free, no-auth, remote-only roles. Trivial CLI. |
| `weworkremotely` | RSS | none | Free, no-auth. RSS → JSON wrapper, simple. |
| `ashby` | public job board API | none | Many AI/ML startups use Ashby |
| `arbeitnow` | public JSON | none | EU/global, no auth |
| `adzuna` | public JSON | API key | Free tier, broad coverage |

These 6 would round the category out for ~80% of common use cases.

## Migration from `library/other/`

`greenhouse-pp-cli` will land at `library/other/greenhouse/` if this proposal
is rejected, since the library's existing categories don't fit. PR for the
new category can be a follow-up. Existing entries in `other/` that turn out
to be job-board CLIs would migrate in a separate PR.

## Open questions for the maintainer

1. Is there an existing process for proposing new categories? The
   `CONTRIBUTING.md` doesn't describe one. This proposal can serve as a
   template.
2. Does `library/job-boards/` need a maintainer commitment, or is
   "lands when an entry is ready" enough?
3. Should the category-split decision be a single PR (job-boards/ + greenhouse
   in one go) or two (category first, then seed entry)?
