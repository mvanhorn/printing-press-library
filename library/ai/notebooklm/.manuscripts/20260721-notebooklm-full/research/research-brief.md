# NotebookLM CLI Research Brief

## Source

This printed CLI reverse-engineers Gemini Notebook (NotebookLM) web-app traffic at `https://notebooklm.google.com`. It uses undocumented `batchexecute` RPC calls observed from the product UI and community references such as notebooklm-py.

Raw browser captures are not archived in the public package because HAR/CDP captures can contain Google session cookies, account identifiers, or notebook content.

## Endpoint Shape

Observed surfaces include:

- `wXbhsf` — list recently viewed notebooks
- Notebook CRUD, source management, grounded chat streaming, Studio artifact generation, and share metadata via batchexecute RPC ids
- Session bootstrap tokens (`FdrFJe`, `SNlM0e`, `cfb2h`) scraped from the home page HTML

## Auth Model

NotebookLM has no public API key. This CLI supports:

- Chrome cookie import via `auth login --chrome`
- `NOTEBOOKLM_COOKIE` environment variable override for headless agents

No real cookie values or session tokens are included in this package.

## Novel Commands

- `auth login` — Chrome session import
- `search` — offline SQLite notebook search after `sync`
- `chat ask` — grounded Q&A with citations
- `studio generate-quiz` — Studio quiz artifact generation
- `doctor` — config, cookie, session, and cache health gate
