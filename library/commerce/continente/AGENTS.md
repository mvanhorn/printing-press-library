# AGENTS.md

## Scope

- This repo is for a Printing Press CLI targeting `continente.pt`.
- Optimize for a useful, shippable read/query CLI first: product search, product detail lookup, categories, pricing, promotions, availability, and related catalog data.
- Treat mutations like checkout, cart changes, account edits, or payment flows as out of scope unless the user asks for them explicitly.

## Working Style

- Be sharp, short, and save tokens.
- If the task needs a heavy refactor, generator rewrite, or paradigm change, ask first.
- Prefer simple, easy-to-reason code over compatibility glue unless compatibility is explicitly required.
- Avoid touching large generated artifacts unless the task requires it.

## Skill Use

- If the task clearly matches the `printing-press` skill, use it and follow its workflow.
- Prefer repo-local instructions over global defaults when they conflict.

## Project Rules

- Start from real API behavior, not guesses from site HTML.
- Prefer official contracts, network captures, OpenAPI/Postman artifacts, or browser-observed JSON endpoints over brittle scraping.
- Assume `continente.pt` may use session, locale, pagination, and anti-bot constraints; verify them before baking behavior into commands.
- Default to Portugal-facing behavior and naming unless the live API proves otherwise.
- Keep the first version read-only and query-focused unless the user expands scope.

## Generated Code

- Preserve the Printing Press structure where possible.
- Prefer thin, targeted edits over broad rewrites of generated files.
- Put handwritten logic in the narrowest sensible place and avoid duplicating generator patterns across files.

## Verification

- Do not claim the CLI works unless it was tested against real `continente.pt` targets or captured responses.
- Distinguish clearly between `generated`, `builds`, and `behaviorally tested`.
- When adding or fixing commands, verify happy path, `--json`, empty results, and obvious error paths.

## Secrets And Captures

- Never commit cookies, session headers, auth tokens, API keys, or raw secret-bearing HAR data.
- Redact captures before storing them in the repo.
- If a feature requires authenticated traffic, stop and confirm scope before proceeding.

## Docs

- Keep examples concrete and runnable.
- Use the real product/domain vocabulary discovered from the API.
- Document any known constraints like locale coupling, rate limits, required headers, or unstable endpoints.
