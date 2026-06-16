# Acceptance smoke

Local checks performed on the published package:

- `go test ./...`
- `go build ./...`
- `python3 .github/scripts/verify-skill/verify_skill.py --dir library/devices/fellow-stagg-ekg`

All passed on the committed package tree before opening the PR.

