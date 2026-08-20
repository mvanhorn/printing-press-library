Acceptance Report: sarvam
  Level: Full Dogfood
  Tests: 173/173 passed
  Failures: 0
  Fixes applied: 5
    - Added Examples to docai/stt-job/voices parent commands (dogfood requires Examples section)
    - Fixed docai schema/pron-check/stt-job Example args to fixture-free commands that work in a clean sandbox
    - Fixed text-to-speech stream --dry-run --json (binary-response guard now detects dry-run envelope)
    - Added pp:typed-exit-codes + pp:happy-args to local-store/stateful novel commands
    - Added pp:no-error-path-probe to feedback (accepts free text by design)
  Printing Press issues: 1
    - feedback parent command lacks an Example in the generator template
  Gate: PASS
