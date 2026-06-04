# eRank CLI Shipcheck

Command:

`ERANK_CONFIG=/Users/smacdonald/.config/erank-pp-cli/config.toml /Users/smacdonald/go/bin/cli-printing-press shipcheck --dir /Users/smacdonald/printing-press/.runstate/grumpykinco-7e542738/runs/20260522-130049/working/erank-pp-cli --spec /Users/smacdonald/printing-press/.runstate/grumpykinco-7e542738/runs/20260522-130049/research/erank-browser-sniff-spec.yaml --research-dir /Users/smacdonald/printing-press/.runstate/grumpykinco-7e542738/runs/20260522-130049`

## Result

- Verdict: PASS, 6/6 legs passed
- verify: PASS, 39/39 checks, browser session proof valid
- validate-narrative: PASS
- dogfood: PASS, novel features 7/7 survived
- workflow-verify: PASS
- verify-skill: PASS
- scorecard: PASS, grade B, total 77/100
- full live dogfood: PASS, 129 passed, 72 skipped; phase5-acceptance.json status pass

## Non-blocking Scorecard Notes

- Scorecard live sample output probe passed 3/7 within the scorecard 10s per-sample timeout. Four multi-endpoint insight commands timed out under that probe, but full live dogfood and shipcheck passed.
- Auth protocol score remains low because eRank uses browser-session cookies and XSRF rather than a public API key/OAuth contract.
