# Phase 17 security review — round 2
Review path: direct reviewer subagent. Re-read custom catalog, SEC and local snapshot implementations plus regenerated tier auth and typed MCP wiring.

## Convergence
PASS for hand-authored security scope. All three round-1 findings are closed; no additional custom security defect found.

## Closure evidence
- All generated California command files now call WithTier("california"); internal/client/client.go authForRequest california case returns an empty requestAuth, and doInternal sends only authInfo.Value. Typed MCP bindings pass the california tier into makeAPIHandler, which also invokes WithTier. This closes the initial-destination federal key leak, not merely redirects.
- Custom govClient still calls config.Load and cfg.AuthHeader; the regenerated Config retains DataGovApiKey and stored AuthHeaderVal fallback. Custom govdata sends that key only on federal requests, and download requests never carry it.
- govdata.Download rejects parsed.User != nil before limiter/transport and never prints a rejected credential-bearing URL.
- intake lock now writes and closes a same-directory temporary file before exclusive hard-link publication; temp cleanup is deferred. Existing files cannot be overwritten and failed writes do not create a final destination.
- SEC URL origins and redirects remain restricted to www.sec.gov/data.sec.gov/efts.sec.gov over HTTPS. Configurable User-Agent is not emitted in results or source.
- SQL remains parameterized. Snapshot writes are transactional. Parent --save guard and root sync writer policy honor NO_LOCAL_DATA. Existing local reads are intentionally permitted.
- MCP output flags remain write-classified; output cannot be supplied through read-only mirrored tools.

## Generated template-shape candidate (reported separately)
P2: internal/client/client.go:241-251 federal tier reads only os.Getenv(DATA_GOV_API_KEY), whereas default authHeader uses cfg.AuthHeader and CredentialRouteBlocked. A key stored with auth set-token is ignored by raw `federal ...` and typed MCP commands, although custom catalog commands correctly use it. This is generated tier-routing template behavior, not a hand-authored custom regression; route upstream and fix through a durable authored seam/spec solution if available. It does not reopen the California leak. Parent notified.

## Cross-review numeric observations
UseNumber ingress requires callers to handle json.Number. Parent notified of California count float64 assertion and SEC ticker formatting; correctness reviewers own those fixes.
