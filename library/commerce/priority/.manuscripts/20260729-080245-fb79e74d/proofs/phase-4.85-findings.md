# Phase 4.85 Agentic Output Review — priority-pp-cli
Status: WARN → fixed in-session.
- batch resume <id> returned success JSON for nonexistent journal IDs. FIXED: journal lookup now returns exit 3 not-found; also added tenant-mismatch guard and dangling-dependsOn stripping on resume.
All other checks passed (honest empty-store notes, clean JSON, per-form license verdicts).
