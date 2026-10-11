# Promote path decision: Path A (swap)
regen-merge dry-run vs the prior library copy (press v4.9.0):
- NOVEL-COLLISION internal/cli/jobs.go, jobs_test.go: intentional restructure; the prior `jobs list` implementation now lives in jobs_list.go (+test) with `jobs` as the leaf and `jobs list` as a hidden alias. Verified by tests and live dogfood.
- Missing referents newImportCmd / newAPICmd: framework commands from the v4.9.0 generator that v4.33.4 no longer emits; not hand-authored.
- today/availability/guests/fetch registrations are re-created through a novel-command hook (hostfully_ops_register.go).
The fresh tree reimplements every prior novel feature (ported from the newer source repo, which is ahead of the library copy), so the swap does not lose hand work.
