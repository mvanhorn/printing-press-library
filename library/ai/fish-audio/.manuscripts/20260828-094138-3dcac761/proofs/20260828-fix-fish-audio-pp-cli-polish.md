# Polish pass (Phase 5.5)
scorecard 95 -> 96; verify 100 -> 100; gosec (hand-authored) 10 -> 0; tools-audit 4 pending -> 0; live sample probe 5/6 -> 6/6; phase5 acceptance regenerated: pass, 141/141 full.
Fixes: mcp-descriptions.json for all 11 endpoint tools; spec info.title "FishAudio OpenAPI" -> "Fish Audio" (root cause of the fishaudio slug and the auth.go display leak); every <model_id> placeholder replaced with a verified public voice id; parent-group Shorts rewritten; WAV size overflow guard; narrow #nosec annotations with reasons.
Incident: an rsync --delete restore removed cmd/fish-audio-pp-cli/; main.go reconstructed from the generator template, builds and runs.
Skipped: 29 gosec findings in generator-emitted files (retro); mcp intents needs spec+regen; cache_freshness rubric undocumented.
ship_recommendation: ship; further_polish_recommended: no.
