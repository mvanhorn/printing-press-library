# policy-intel-pp-cli Agent Notes

This CLI is a generated-then-curated Printing Press entry for federal rulemaking and policy-monitoring workflows. Keep the command surface read-only and source-backed.

## Source Boundaries

- Federal Register search/rules commands use FederalRegister.gov public APIs and do not require credentials.
- Regulations.gov docket, comment, and deadline commands use the public v4 API with `POLICY_INTEL_REGULATIONS_API_KEY`; the public `DEMO_KEY` is used only when the env var is absent.
- Do not add comment submission, legal advice, compliance conclusions, or lobbying workflows.

## Patch Recording

Record code-level hand edits under `.printing-press-patches/`. Do not edit generated catalog artifacts such as `registry.json` or `cli-skills/pp-policy-intel/SKILL.md`.
