# Novel Features Brainstorm — Sarvam AI CLI

## Customer model
- **Ravi Krishnan** — voice-agent/IVR developer, builds Hindi/Tamil IVR flows and notification bots for a regional bank. Weekly ritual: adds IVR prompts, auditions 2-3 new voices against a fixed sample sentence, maintains a pronunciation dictionary for product names. Frustration: 45 speakers with no speaker→language map; no one-command "hear all voices on this sample"; cannot verify whether a pron-dict edit changed pronunciation without manual listen-compare.
- **Meera Iyer** — call-center QA/ops lead, daily transcription of Hinglish/Tamil customer-service recordings. Weekly ritual: daily batch of 50-200 call recordings, checks overnight job completion, samples transcripts for QA. Frustration: per-file failures buried in dashboard, no scriptable digest, no non-zero exit code for cron, re-uploading failed files is manual.
- **Priya Nair** — localization/content engineer, ships Hindi/Tamil/Marathi UI strings and subtitled promos. Weekly ritual: weekly string freeze, biweekly subtitled promo videos from Hindi master voiceover. Frustration: every subtitle run means a throwaway script converting word-timestamp JSON to .srt; no offline reuse of past transcriptions.
- **Arjun Deshmukh** — fintech ops developer, automates KYC/insurance-policy extraction. Weekly ritual: weekly batch of KYC documents with the same schema. Frustration: schemas live in gists/Slack with no versioning; no local home for schemas; no one-command "run this schema on this folder."

## Candidates (pre-cut)
15 candidates generated across sources (a) persona-driven, (b) service-specific content patterns, (c) cross-entity local queries. Inline kills: doctor (thin single-endpoint probe), extract-then-translate (pipe redundancy), call digest (verifiability fail — transcript file schema undefined in OpenAPI). Pre-cut keeps: voices preview, chat resume, stt-job retry, stt-job report, pron-check, subs, docai schema, docai batch, models check, languages, prompts, pron-dict coverage.

## Survivors and kills
### Survivors (all hand-code, scored >= 5/10)
1. voices preview (8/10) — voice auditioning
2. chat resume (8/10) — conversation resume from local history
3. stt-job retry (8/10) — re-run failed batch files
4. pron-check (7/10) — pronunciation round-trip verify
5. subs (7/10) — .srt/.vtt subtitle export from timestamped transcriptions
6. docai schema (7/10) — extraction schema library
7. docai batch (6/10) — folder extraction run
8. stt-job report (7/10) — per-file batch digest with typed exit codes

### Killed candidates
| Feature | Kill reason | Sibling |
|---|---|---|
| languages (capability matrix) | fails weekly-use; weakest transcendence | voices preview, models |
| models check (drift) | fails weekly-use; deprecation is rare | models |
| doctor | thin single-endpoint probe; auth set-token covers it | auth set-token |
| pron-dict coverage | milestone-driven not weekly; weak token matching | pron-check |
| prompts (template library) | fails weekly-use; thin snippet store | chat resume |
| docai run --translate | pipe redundancy | docai batch |
| calls digest | verifiability fail (schema undefined) | stt-job report |

