# Activity Japan CLI maintenance

This is a local, read-only Printing Press CLI for known Activity Japan plans. Build with the Go version in `go.mod`; use `SKILL.md` for the agent workflow and `README.md` for local use.

Keep source plan, operator, option and session IDs distinct. Preserve Japanese names, source URLs, raw wording and Asia/Tokyo observation times. A dated status is an observation, never a reservation. Website locale and `support_language` do not prove an instructor's spoken language. Selected-date source price integers take precedence over any discount reconstruction; child and infant options cannot price adults. Source zero party bounds are unknown, and `basic_min_passenger_count` has unverified semantics.

Hand-authored behavior lives in `internal/activityjapan/` and `internal/cli/activity_japan_*.go`; register commands through the novel hook, not generated root edits. Keep response bodies, date scans, pages, retries, timeouts, rate limits and sitemap cache bounded. A denied or malformed source is an error, not an empty result. Dry runs stop before network and cache writes.

After changes, run affected `go test` packages and a live read-only source case plus a failure case. For final acceptance, run the Printing Press receipt gates. Preserve the run's scope and staging process. Delivery is local; do not publish or change shared tool configuration without an explicit request.
