<!-- slop-gate: off -->
# Polish pass (printing-press-polish, forked)

  Scorecard:   95 -> 95
  Verify:      100% -> 100% (104/104)
  Tools-audit: 0 -> 0 pending
  gosec (hand files): 8 -> 0 (narrow #nosec with reasons; 35 remaining are generated-file retro candidates)
  go vet 0 -> 0; verify-skill 0 -> 0; PII 0 -> 0; go test ./... pass

Fixes: #nosec annotations in postmark_servers.go / postmark_template_layout.go; recipient-domains single sync hint (no default home path echoed) with TestRecipientDomainsEmptyResultHints; recipient-domains help states the rest-of-account rule and 5% floor; preview-by-default command Shorts (MCP descriptions) now say they preview unless --send/--apply/--yes.
Skipped: generated dead helper; 35 generated-file gosec findings; live-check cannot expand "$SENDER" in the send-once example and example.com trips the host guard; verify mock bare-array artifact; structural MCP Quality 8/10 and Cache Freshness 5/10.
Ship recommendation: ship. Acceptance marker stale after 8 source edits; live dogfood re-run required before promote.
