# Novel features brainstorm (subagent output, 2026-10-28)
Personas: Giulia (civil litigator), Marco (compliance officer: 231/2001, 196/2003, 81/2008, appalti), Sara (legal-tech RAG developer), Antonio (municipal officer: L. 241/1990, TUEL, D.lgs. 36/2023).
Candidates: C1 diff, C2 verifica, C3 segui/novita, C4 fascicolo, C5 rinvii, C6 vigenza, C7 confronta, C8 digest, C9 grep, C10 codici, C11 in-arrivo, C12 timeline, C13 abrogati, C14 statistiche, C15 modifiche.
Survivors: verifica 10/10, diff 10/10, segui+novita 8/10, fascicolo 7/10 (all hand-code).
Killed: rinvii (4/10, AKN links unconfirmed; dup of verifica), vigenza (thin; in verifica --al), confronta (subset of diff --originale), digest (no personal join), grep (dup of framework search), codici (monthly; covered by scarica/esporta), in-arrivo (future versions unverifiable), timeline (N*versions calls on WAF host), abrogati (verifica filter), statistiche (facets already in cerca), modifiche (overlaps cronologia; brittle markers).
