# Tella novel-features brainstorm (audit trail)

## Customer model

- **Sasha (sales SDR):** records personalized walkthroughs, manually triages "who watched 75%" weekly. Frustration: no rollup of milestone hits.
- **Fiona (founder):** records demos, repeats the same edit pass per clip (filler removal, silence trim, blur secrets). Frustration: no bulk apply.
- **Sam (support engineer):** records bug repros, periodically hunts old recordings. Frustration: no cross-video transcript search.
- **Cam (creator):** runs async-update channel with webhook-driven pipeline. Frustration: webhook dev needs ngrok; no replay.

## Survivors (8)
1. Cross-video transcript search (`transcripts search`) — 9/10
2. Watch-milestone digest (`videos viewed`) — 8/10
3. Webhook tail + replay (`webhooks tail`, `webhooks replay`) — 8/10
4. Bulk standard edit pass (`clips edit-pass`) — 8/10
5. Transcript diff cut vs uncut (`clips transcript-diff`) — 7/10
6. Exports waitlist (`exports wait`) — 7/10
7. Caption-file export (`clips captions`) — 7/10
8. Workspace stats (`workspace stats`) — 6/10

## Killed candidates
- Silence atlas — subsumed by bulk edit-pass
- Effect-preset library — scope creep
- Stale detector — monthly archeology, not weekly
- Viewer-engagement compare — sibling overlap with milestone digest
- Reorder by topic — niche + LLM-adjacent
- Webhook signature verifier — once-per-integration, not weekly
- AI chapter markers — LLM dependency
- Sentiment trend — LLM dependency

## Personas served by survivor
- Sasha (sales SDR): `videos viewed`, `clips captions`
- Fiona (founder): `clips edit-pass`, `clips transcript-diff`, `exports wait`, `workspace stats`
- Sam (support engineer): `transcripts search`
- Cam (creator): `webhooks tail`, `clips captions`, `exports wait`
