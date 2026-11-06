<!-- slop-gate: off -->
# Phase 4.8 SKILL review + Phase 4.9 README/SKILL/AGENTS audit

Reviewer: one subagent running both contracts (read-only, --help + agent-context only).
Contract A: 4 errors, 8 warnings. Contract B: 1 error, 8 warnings.

Fixed in research.json (source of truth for README/SKILL/root help/which): A1 course push recipe, A2 inbox mirror-only
disclosure, A3 course pull HTML + no media, A4 recap transcript/attendee disclosure, A5 branch on verdict, A6/A7/A8 examples
without --dry-run plus --yes disclosure, A10 trialing/past_due count as paid, A11 spaces apply recipe, A12 trigger phrases,
B1 no `sql` CLI command, B8 quickstart doctor --dry-run comment.
Fixed in source via fix batch H: A9 --every help text, A11 YAML example in spaces apply Long, B9 onboard --csv renamed --from-csv.
Template-shape (retro, not patched in the printed CLI): B2 nested subcommands missing from command reference, B3 Filterable
example lacks --select, B4 "Three sinks" omits spacefast, B5 `auth-status` is not a command, B6 README says `echo $<TOKEN>`,
B7 placeholder literals in boilerplate examples.
