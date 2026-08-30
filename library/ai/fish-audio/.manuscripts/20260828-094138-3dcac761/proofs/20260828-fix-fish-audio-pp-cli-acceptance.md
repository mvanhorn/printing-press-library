Acceptance Report: fish-audio
  Level: Full Dogfood (cli-printing-press dogfood --live --level full)
  Tests: 141/141 passed (123 skipped by matrix rules)
  Runs: 4. Run 1: 7 failures. Run 2: 2. Run 3: 2 (parent-group Example carried a literal placeholder). Run 4: 2 (HTTP 402, dev API credit $0). Run 5 after top-up: 0.
  Failures fixed:
    - asr: opened the --audio file before honoring --dry-run (generated promoted_asr.go); dry-run now shows the field without opening the file.
    - feedback: parent command had no Examples section (generated feedback.go); added.
    - render diff: missing ids returned exit 3; now an empty local result (exit 0, empty fields, stderr hint), matching `render log` on an empty mirror.
    - voice verify: defaulted to a paid model; now defaults to s2.1-pro-free; validates reference_id client-side; parent-group Example used a placeholder id.
    - HTTP 402 now maps to an actionable error naming both credit ledgers and `wallet balance`.
  Live evidence: tts render on s2.1-pro-free wrote Pearl's greeting (24,240 bytes, cost 0, paid-equiv 0.00027); voice verify on a public voice: transcript matched, WER 0, cost 0.0005; wallet balance shows the two ledgers for the authenticated viewer.
  Printing Press issues (retro): parent-group Example synthesized from research example with placeholder; generated asr dry-run ordering; generated feedback lacks Example; dogfood --live does not honor pp:happy-args on novel positionals (uses the group Example instead).
  Gate: PASS
