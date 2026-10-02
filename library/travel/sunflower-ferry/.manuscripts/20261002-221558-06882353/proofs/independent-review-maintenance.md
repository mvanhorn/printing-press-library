# Targeted maintenance-boundary verification

PASS — no concrete finding in the observed-maintenance handling change.

Reviewed managed publication source `internal/ferry/operations.go:25–53` and `TestSourceMaintenanceStopsBeforeAnonymousPost` at `internal/ferry/ferry_test.go:360–372`. The exact normalized visible phrase in the retained real HTTP-200 source-health response is detected before hidden-field extraction and before returning a form to the anonymous POST path. The error identifies official booking-system maintenance and returns no fare/sailing data.

The consequential regression invokes Quote with a valid future date/party and an HTTP-200 observed-message response. It rejects any request other than the initial canonical GET, requires exactly one request, checks the maintenance error and verifies zero sailings. This proves the failure boundary; the mocked response is deterministic regression evidence and does not replace the real observation in source-health-current.json.

Independent targeted test run passed, including the prior publication shared-cabin and exact-terminal overlay cases. Reproduction, source hashes and test output are in reviewer-maintenance-verification.json. No live requests, source edits, extra agents, browser/CDP sessions, booking/hold/account/payment actions or GitHub writes were performed. Healthy live samples were not rerun. Earlier generator limitations remain unchanged; this check does not claim a new successful live-provider acceptance while the provider is under maintenance.
