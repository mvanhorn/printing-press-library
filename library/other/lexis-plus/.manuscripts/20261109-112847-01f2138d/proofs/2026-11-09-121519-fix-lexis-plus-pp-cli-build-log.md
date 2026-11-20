Manifest transcendence rows: 6 planned, 6 built. Phase 3 will not pass until all 6 ship.

# Build log
- Hand-written: internal/cli/lexis_session.go (session, lookup, delivery, retrieval log), lexis_cmds.go (case get, statute get, cite check, batch), lexis_brief.go (brief cites/check/gaps, matter list/show/binder, statute whatsnew).
- Live verification (user's own session): citation lookup for case, statute, and unrecognized cite; Word download of a case and an annotated statute (annotations confirmed in the .docx).
- Fixes found by live testing: delivery job needs a non-empty title; request must send every retrieveoptions key and clientId "-None-" for annotations to render.
- Not verified live: brief check, statute whatsnew, batch, matter binder end-to-end (local logic only).
