# Acceptance Report: cloro

Level: Full Dogfood (`cli-printing-press dogfood --live --level full --timeout 120s`), operator-approved credit budget.
Tests: 121/121 passed (130 skipped by the runner: help-only framework commands, mutating commands without --allow-destructive, positionals with no portable fixture).

Live paid calls (happy path and JSON fidelity each): chatgpt, aimode, gemini, google, news, cited (aimode and gemini). Free live calls: countries, states, credits, async queue.
Dry-run only: async create (a real task spends credits). Blocked fixtures (no portable value, the token expires minutes after the scrape that issued it): goto, product. Skipped: async batch (body comes from stdin), async get (needs a task id).

Outside the matrix, the create-task and get-task endpoints were run once by hand (as `tasks create` and `tasks get`, before the rename below): a GOOGLE task went from QUEUED to COMPLETED and charged 3 credits.

Failures: none in the final run (third run).
Fixes applied after the first run (4 failures):
- goto and product had no help Example, because their examples carried a placeholder; they now use the example values from the cloro spec, and their live probes are marked blocked-fixture.
- async create: the synthesized `--payload` was not JSON; the happy path now sends a real payload as a dry run.

Fix after the second run: the async commands were first printed under a `tasks` resource. The name made the profiler classify the API as project management and add `stale`, `orphans` and `load` commands that do not apply to cloro. The resource is now `async` (`async create`, `async batch`, `async get`, `async queue`), and the three commands are gone. The second run (130/130) was on the tree before this rename; the third run is on the shipped tree.

Printing Press issues for retro: (1) the happy-args synthesizer took a description phrase ("ChatGPTMonitorRequest") as the value of an object-typed body field; (2) a resource named `tasks` makes an unrelated API a project-management archetype.

Gate: PASS
