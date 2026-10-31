# Polish (Phase 5.5) — ship
scorecard 95 -> 97, verify 98% -> 100%, dogfood WARN -> PASS, gosec (hand-written) 10 -> 0, tools-audit 0 pending.
Fixes: hidden --data-inizio-aggiornamento/--data-fine-aggiornamento aliases on aggiornati; diff JSON adds citazione/etichetta; G104 cleanup errors handled; narrow #nosec for user-named input read (G304) and 0644 exports (G302); removed dead generated helpers; gofmt.
Skipped (environmental/structural): mock-verify 2/3 on atto/cita/cronologia/testo/esporta/scarica; 44 gosec findings in generated files; pinned gosec v2.26.1 crashes on go1.27.3; amendment markers kept in body text by design; cache freshness 5/10.
Retro candidates: which.go re-synced unformatted by dogfood; retainCLIQueryParams emitted but unused.
ship_recommendation: ship
