# Novel-features brainstorm (audit trail)

## Customer model
1. Jon, migrating Pearl's pinned greeting off ElevenLabs: re-renders greeting lines weekly while iterating copy; no record of what was rendered, what it cost, or duplicates (Fish Audio has no server-side history).
2. Narration developer batch-rendering multi-line scripts: loops curl per line, no total cost visibility, no pre-flight balance check.
3. Voice-clone builder verifying clone quality before shipping: clone quality is a vibe check with no measurable signal.

## Candidates (pre-cut)
1. render log (keep) 2. tts render --skip-if-rendered (keep) 3. cost compare vs ElevenLabs invoice (soft kill, monthly cadence, folded into 6) 4. clone verify/audition (wrapper risk) 5. voice design candidate audition (speculative pain) 6. render spend --group-by (keep) 7. voice usage (dup of 6) 8. render diff (keep) 9. voice verify via TTS->ASR WER (keep) 10. tts batch --budget-guard (keep) 11. render replay (invented) 12. voice casting auto-assign (LLM dependency, cut).

## Survivors and kills
### Survivors
| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---|---|---|---|---|---|---|
| 1 | Local render history | `render log` | 10/10 | hand-code | Reads local SQLite render-log (populated by every tts render/batch) listing text, model, voice, bytes, cost. | No GET /tts/history endpoint exists; User Vision cost-tracking ask | Use this command to list individual past TTS renders with their text, model, voice, bytes, and cost. Do NOT use this command for aggregated spend totals; use 'render spend' instead. |
| 2 | Skip identical re-renders | `tts render --skip-if-rendered` | 8/10 | hand-code | Hashes text+voice+model+params and checks render-log before POST /v1/tts. | Render-log stores request hash; Build Priority #3 | none |
| 3 | Grouped spend report | `render spend --group-by voice\|model\|day` | 9/10 | hand-code | GROUP BY over local render-log with cost totals. | User Vision cost tracking vs ElevenLabs | Use this command to view aggregated Fish Audio spend grouped by voice, model, or day. Do NOT use this command to see individual render records; use 'render log' instead. |
| 4 | Compare two renders | `render diff <id1> <id2>` | 6/10 | hand-code | Reads two render-log rows and computes cost/model/byte deltas. | Batch-iteration A/B across takes | Use this command to compare two specific past renders by log ID. Do NOT use this command for a full list or totals; use 'render log' or 'render spend' instead. |
| 5 | Clone fidelity check | `voice verify <model_id>` | 8/10 | hand-code | Renders a reference phrase via POST /v1/tts with the model, transcribes via POST /v1/asr, computes WER vs known text. | Top Workflow #2; need to trust clone before replacing live ElevenLabs voice | Use this command to test a cloned voice's fidelity by rendering a reference phrase and checking its ASR transcription. Do NOT use this command to create a clone; use 'voice clone' instead. |
| 6 | Pre-batch budget guard | `tts batch --budget-guard` | 7/10 | hand-code | Sums estimated batch cost, checks GET /wallet/self/api-credit, refuses if insufficient. | Top Workflow #5; dual-ledger confusion | none |

### Killed candidates
| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| Cost compare vs ElevenLabs invoice | Monthly cadence; duplicates render spend | render spend |
| Clone verify/audition | Thin wrapper over tts render, no verification logic | voice verify |
| Voice design candidate audition | One-time per voice; raw voice design response already returns candidates | voice design (absorbed) |
| Voice usage | Duplicate of render spend --group-by voice | render spend |
| Render replay | No research evidence; invented ritual | render log |
| Voice casting auto-assign | Semantic matching = LLM dependency | tts batch --dialogue (absorbed) |
