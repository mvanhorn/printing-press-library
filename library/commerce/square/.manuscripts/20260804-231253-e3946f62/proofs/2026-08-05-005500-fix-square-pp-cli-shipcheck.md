# Square CLI shipcheck proof

Canonical shipcheck was run from the CLI working directory so the documented relative fixture `payment-request.json` resolved in its intended context.

- Verdict: PASS, 7/7 legs.
- Runtime verification: PASS, 250/250 checks, 0 critical failures.
- Narrative validation: PASS, 9 commands resolved and examples passed.
- Dogfood: PASS, 5/5 paths, 0 dead flags, 0 dead functions, 6/6 novel features.
- Skill validation: PASS.
- Scorecard: 93/100, Grade A.
- Custom output probe: 6/6.
- Three legacy Square v1 Orders paths were intentionally skipped because the generator could not derive stable resources. Documentation claims the current Square v2 surface, not the legacy v1 surface.

No live Square API verification was performed because no credential was available; the auth-aware skip marker records that limitation.

After polish changed the request-readiness wording and empty collection shapes, the complete canonical shipcheck was rerun. It again passed 7/7 legs, 250/250 runtime checks, 6/6 custom samples, and retained the 93/100 Grade A score.
