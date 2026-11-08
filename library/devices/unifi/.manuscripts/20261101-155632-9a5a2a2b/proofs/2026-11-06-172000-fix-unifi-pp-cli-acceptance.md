# Acceptance Report: unifi

Level: Full Dogfood, run with UNIFI_READ_ONLY=1 so no matrix command could write.
Tests: 367/367 passed, 0 failed; 729 skipped or unverified.
Gate: PASS

## Skips that matter
- Most mutating commands ran as dry runs or were skipped by the runner, so they are unproven on real hardware.
- InnerSpace, Mobility and Carrier refuse this key (403): unverified.
- Protect get-by-id commands: the test console has no such devices. Protect users and hosts get-by-id verified with real ids.

## Separate approved write lifecycle (user approved a disposable voucher)
Commands, each exit 0 unless noted, against the test console over the cloud connector:
1. network list-hotspot-vouchers <site> --all --data-source live: baseline 0
2. network create-hotspot-voucher <site> --name pp-cli-test-delete-me --count 1 --time-limit-minutes 1 --authorized-guest-limit 1 --data-usage-limit-m-bytes 1 --dry-run
3. Same command without --dry-run
4. list: 1 test voucher
5. network get-hotspot-voucher <site> <id>
6. Single-id voucher delete
7. get after delete: exit 3, HTTP 404
8. list: 0, back to baseline
Console log records one create and one remove by the API key.

## Fixes applied: 3
- CLI fix: voucher delete names were crossed; swapped, overlay updated.
- CLI fix: zone-based-firewall-not-configured 400 is a sync warning; rule-predict names the legacy firewall cause.
- CLI fix: removed spec sample ids from pp:happy-args on 20 get commands so the runner resolves real ids.

## Printing Press issues for retro: 3
- Generator pins spec sample ids in pp:happy-args, which guarantees 404 on any real account.
- Cross-named singular and plural operations from the spec are not detected.
- Typed mutating commands send immediately while raw and MCP preview: inconsistent safety.
