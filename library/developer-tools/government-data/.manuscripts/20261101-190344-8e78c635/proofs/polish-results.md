# Polish retry results

Public library clone not found locally; internal candidate treated as canonical. Mid-pipeline mode: no publish validation, promotion or publication attempted.

Scorecard improved 95/100 to 98/100. Mock verify remains 49/49 (100%). Mechanical dogfood PASS and MCP surface parity PASS. Verify-SKILL has no findings. Primary California search→metadata→resources workflow and SEC company→filings→Assets facts workflow passed all six live steps. Tools audit has no pending findings (one accepted conditional-write heuristic false positive); PII audit cleared after sanitizing generated workflow response contacts.

Full live matrix retry: 165/171 executed checks passed, six failed, plus 152 skipped/unverified checks. All six failures are federal find/organizations/topics happy-path and JSON checks, typed quota exit7 with DEMO_KEY. Source-bound acceptance marker remains FAIL. This is a release hold, not a shippable verdict. No private key or personal testing contact is bundled.

Fixes: durable MCP organization-list description override; applied with mcp-sync; propagate successful SEC response-body close errors; explicitly discard response-close errors when an HTTP error already takes precedence; explicitly discard best-effort temporary-file cleanup errors; add actual workflow verification manifest; redact captured public contacts in generated diagnostic report. No runtime default changed.

Security: gosec v2.29.0 with x/tools v0.52.0 fully loaded the module. Five handwritten G104 findings fixed, zero remaining. Fifty-one generator-owned findings retained in generated-security-triage.md for upstream review; this is not a globally clean security scan.

Output plausibility reviewer assessed the one eligible catalog snapshots sample and returned PASS. Other scorecard samples lacked mandatory inputs; they were excluded, not counted as passing output review.

Further polish recommended: no. The remaining live gate depends on federal quota renewal or a personal DATA_GOV_API_KEY.
