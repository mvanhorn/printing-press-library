# Catalog local lifecycle proof
Sandbox: disposable scratch SQLite copy of snapshots created by real catalog find --save HTTP reads. No remote mutations.
```text
<runstate>/wha/runs/20261101-190344-8e78c635/working/government-data-pp-cli/build/stage/bin/government-data-pp-cli overlap --snapshot smoke-b --db <workspace>/work/lifecycle.sqlite --json --no-learn
exit 0
```
```text
<runstate>/wha/runs/20261101-190344-8e78c635/working/government-data-pp-cli/build/stage/bin/government-data-pp-cli changes --from smoke-a --to smoke-b --db <workspace>/work/lifecycle.sqlite --json --no-learn
exit 0
```
```text
<runstate>/wha/runs/20261101-190344-8e78c635/working/government-data-pp-cli/build/stage/bin/government-data-pp-cli coverage --snapshot smoke-b --db <workspace>/work/lifecycle.sqlite --json --no-learn
exit 0
```
```text
<runstate>/wha/runs/20261101-190344-8e78c635/working/government-data-pp-cli/build/stage/bin/government-data-pp-cli intake audit --snapshot smoke-b --db <workspace>/work/lifecycle.sqlite --json --no-learn
exit 0
```
```text
<runstate>/wha/runs/20261101-190344-8e78c635/working/government-data-pp-cli/build/stage/bin/government-data-pp-cli intake lock --snapshot smoke-b --resource california:a47fde7d-7071-4f89-bd95-d8912ec8c385:83440b3e-1c58-420a-8390-7ffe762853a0 --output <workspace>/work/lifecycle-lock.json --db <workspace>/work/lifecycle.sqlite --json --no-learn
exit 0
```
Verification: all five real state-dependent commands exited0 and returned parseable JSON. Lock artifact contained selected resource metadata.
Cleanup: removed scratch SQLite copy and generated lock artifact successfully; original live-smoke evidence retained in work/.