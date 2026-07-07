Canonical shipcheck passed after implementing the seven approved novel features.

Command:

```bash
/Users/matthewherzog/go/bin/cli-printing-press shipcheck --dir /Users/matthewherzog/printing-press/.runstate/dev-a8743ce9/runs/20260614-204724-5aa6a87d/working/lda-gov-pp-cli --spec /Users/matthewherzog/printing-press/.runstate/dev-a8743ce9/runs/20260614-204724-5aa6a87d/research/lda-gov-openapi.yaml --research-dir /Users/matthewherzog/printing-press/.runstate/dev-a8743ce9/runs/20260614-204724-5aa6a87d
```

Result:

- verify: PASS
- validate-narrative: PASS
- dogfood: PASS
- workflow-verify: PASS
- verify-skill: PASS
- scorecard: PASS
- final verdict: PASS (6/6 legs)
- scorecard: 95/100, Grade A

Noted non-blocking scorecard gap:

- Sample output probe passed 6/7; `entities resolve Boeing` produced no query-token output without a synced local mirror. The command intentionally returns empty machine-readable output plus a sync hint when the local store is absent.
