# Shipcheck Proof - cloud-run-admin-pp-cli

Run ID: 20260507T171842Z-6844a8b4

## Command

```bash
./printing-press shipcheck \
  --dir ~/printing-press/library/cloud-run-admin \
  --spec ~/printing-press/library/cloud-run-admin/spec.yaml \
  --research-dir ~/printing-press/.runstate/mogadishu-36cf6133/runs/20260507T171842Z-6844a8b4/pipeline \
  --json
```

JSON proof: `proofs/20260507T172601Z-shipcheck.json`

## Result

Overall: PASS

- dogfood: PASS
- verify: PASS
- workflow-verify: PASS
- verify-skill: PASS
- scorecard: PASS

## Fixes Before Final Pass

- Structural dogfood flagged the dead helper `extractResponseData`; it was removed.
- Structural dogfood flagged `which` as a hand-rolled replacement for an existing framework command; the novel feature was replaced with `services list`.
- Verify-skill flagged the SKILL recipe `search service --query`; the recipe was corrected to `search "api" --type services --json --select resource_type,title`.

## Scorecard

Final standalone scorecard proof: `proofs/20260507T172601Z-scorecard.json`

- Overall grade: A
- Total: 87
- Percentage: 87

Before the final SKILL recipe fix, the umbrella shipcheck failed at verify-skill while dogfood, verify, workflow-verify, and scorecard were already passing. After the fix, all five shipcheck legs passed.

## Recommendation

Ship.
