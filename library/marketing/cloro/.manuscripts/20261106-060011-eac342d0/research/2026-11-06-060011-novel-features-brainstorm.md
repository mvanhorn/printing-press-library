# cloro novel features brainstorm

## Customer model
- GEO and SEO teams that must report whether AI engines cite their site for the prompts their buyers ask.
- Agents that need an answer with sources, or a quick check of one claim across engines.
- Builders who run large prompt sets and pay per request.

## Candidates
| # | Candidate | Command | Score | Verdict |
|---|-----------|---------|-------|---------|
| 1 | Is a domain cited by ChatGPT, AI Mode, Gemini and the Google AI Overview for a prompt | cited <domain> --prompt <p> | 9/10 | Keep. The main GEO question; today it needs four calls and four manual scans of source lists. Unit tests with a stub server, one live run. |
| 2 | Which sources entered or left an answer since the last run | answers diff --prompt <p> | 6/10 | Cut. Needs a local snapshot store. AI answers and their sources change between runs of the same prompt, so a diff of two single runs shows mostly noise. |
| 3 | Async batch from a prompt file with polling | batch run --file prompts.txt | 6/10 | Cut for v1. The generated `async batch` and `async get` already cover submit and poll; a wrapper adds code to maintain with small gain. |

## Survivors
1. `cited`: cross-engine citation check (hand-code).

Scope set by operator direction (fewer, tested commands). No subagent pass was run for this print; the candidates come from the operator's item plan.
