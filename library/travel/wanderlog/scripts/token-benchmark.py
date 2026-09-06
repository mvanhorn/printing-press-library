#!/usr/bin/env python3
"""Count an offline CLI workload with tiktoken's o200k_base encoding.

Install explicitly in your own environment (this script never installs packages):
  python3 -m venv /tmp/wanderlog-tokenizer
  /tmp/wanderlog-tokenizer/bin/pip install tiktoken

Generate the synthetic workload in the CLI module:
  WANDERLOG_TOKEN_BENCH_DIR=/tmp/wl-tokens go test ./internal/cli \
    -run 'TestPlanDayTokenWorkload|TestPlanFlowTokenWorkload|TestCreateBatchTokenWorkload' -count=1

Capture preserved baseline and candidate discovery stdout into that directory:
  BEFORE agent-context --agent > /tmp/wl-tokens/before-context.json
  BEFORE agent-context --for-edit --agent > /tmp/wl-tokens/before-edit-context.json
  AFTER agent-context --agent > /tmp/wl-tokens/after-context.json
  AFTER agent-context --for-edit --agent > /tmp/wl-tokens/after-edit-context.json

For focused discovery, capture AFTER agent-context --task review --agent into
flow-task-review.json (and similarly create/edit). Optional flow-before-skill.md
and flow-after-skill.md count skill input separately. Creation payload files and
commands are described by the generated create-workload.json manifest.

Then run this script --artifacts /tmp/wl-tokens. o200k_base is an explicit
encoding proxy, not a guarantee for every model. This is a static workload
comparison, not an evaluation of model reasoning or real-trip planning quality.
"""
import argparse
import json
from pathlib import Path

import tiktoken


def measure(encoding, path):
    text = path.read_text()
    return {"bytes": len(text.encode()), "tokens": len(encoding.encode(text, disallowed_special=()))}


def workflow(encoding, paths, commands):
    rows = [measure(encoding, p) for p in paths]
    stdout = sum(row["tokens"] for row in rows)
    command = sum(len(encoding.encode(text)) for text in commands)
    return {
        "calls": len(paths),
        "stdout_bytes": sum(row["bytes"] for row in rows),
        "stdout_tokens": stdout,
        "command_tokens": command,
        "command_plus_stdout_tokens": command + stdout,
        "files": [p.name for p in paths],
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--artifacts", type=Path, required=True)
    args = parser.parse_args()
    encoding = tiktoken.get_encoding("o200k_base")
    results = {
        "encoding": encoding.name,
        "tokenizer_version": tiktoken.__version__,
        "limitations": "Static synthetic workload; not a SOTA/model-quality evaluation. Counts stdout and command text, excluding tool-call framing, reasoning, skills and latency. Calls are CLI invocations, not API roundtrips. Baseline read projections use legacy pretty rendering; candidate uses --agent JSON compaction.",
    }
    for phase in ("before", "after"):
        for kind in ("context", "edit-context"):
            path = args.artifacts / f"{phase}-{kind}.json"
            if path.exists():
                results[f"{phase}_{kind}"] = measure(encoding, path)
    paths = sorted(args.artifacts.glob("before-day-*.json"))
    if paths:
        commands = []
        for path in paths:
            if path.name == "before-day-01-outline.json":
                commands.append("wanderlog-pp-cli plan outline --target-key naertjcoixqrgrfc --day 1 --agent")
            elif path.name == "before-day-02-legs.json":
                commands.append("wanderlog-pp-cli plan route legs --target-key naertjcoixqrgrfc --day 1 --modes walking,driving --travel-mode walking --with-planning --agent")
            else:
                block_id = int(path.stem.removeprefix("before-day-block-"))
                commands.append(f"wanderlog-pp-cli plan block get --target-key naertjcoixqrgrfc --block-id {block_id} --agent")
        results["before_day"] = workflow(encoding, paths, commands)
    for name in ("snapshot", "envelope", "unchanged", "one-change"):
        path = args.artifacts / f"after-day-{name}.json"
        if path.exists():
            command = "wanderlog-pp-cli plan day --target-key naertjcoixqrgrfc --day 1 --modes walking,driving --travel-mode walking --agent"
            if name in ("unchanged", "one-change"):
                command += " --since day-state.json"
            results[f"after_day_{name}"] = workflow(encoding, [path], [command])
    # The flow fixture has three full days with repeated global constraints and
    # identical place metadata. Count the complete returned envelopes.
    separate = sorted(args.artifacts.glob("flow-separate-day-*.json"))
    if separate:
        results["flow_separate_days"] = workflow(encoding, separate, [
            f"wanderlog-pp-cli plan day --target-key naertjcoixqrgrfc --day {i} --modes walking,driving --travel-mode walking --agent"
            for i in range(1, len(separate) + 1)
        ])
    for name, command in {
        "days": "plan days --days 1-3 --modes walking,driving --travel-mode walking",
        "overview": "plan overview --modes walking,driving --travel-mode walking",
    }.items():
        path = args.artifacts / f"flow-{name}.json"
        if path.exists():
            results[f"flow_{name}"] = workflow(encoding, [path], [
                f"wanderlog-pp-cli {command} --target-key naertjcoixqrgrfc --agent"
            ])
    for task in ("review", "create", "edit"):
        path = args.artifacts / f"flow-task-{task}.json"
        if path.exists():
            results[f"flow_task_{task}"] = workflow(encoding, [path], [
                f"wanderlog-pp-cli agent-context --task {task} --agent"
            ])
    for phase in ("before", "after"):
        path = args.artifacts / f"flow-{phase}-skill.md"
        if path.exists():
            results[f"flow_{phase}_skill"] = measure(encoding, path)
    for path in sorted(args.artifacts.glob("flow-create-*.json")):
        results[path.stem.replace("-", "_")] = measure(encoding, path)
    creation = args.artifacts / "create-workload.json"
    if creation.exists():
        manifest = json.loads(creation.read_text())
        results["creation_description"] = manifest["description"]
        for name, case in manifest["cases"].items():
            row = workflow(encoding, [args.artifacts / p for p in case["outputs"]], case["commands"])
            row["payload_tokens"] = sum(measure(encoding, args.artifacts / p)["tokens"] for p in case.get("payload_files", []))
            row["command_stdout_payload_tokens"] = row["command_plus_stdout_tokens"] + row["payload_tokens"]
            results[f"creation_{name}"] = row
    if "flow_task_review" in results and "flow_days" in results:
        # Explicitly name the extra overview and skill input. These totals do
        # not include model reasoning, search, edit payloads or tool framing.
        components = ["flow_task_review", "flow_overview", "flow_days"]
        results["flow_review_entry"] = {
            "components": components,
            "calls": sum(results[k]["calls"] for k in components),
            "command_plus_stdout_tokens": sum(results[k]["command_plus_stdout_tokens"] for k in components),
            "skill_tokens": results.get("flow_after_skill", {}).get("tokens"),
        }
    if not any(key.startswith("before_") or key.startswith("after_") for key in results):
        parser.error("no workload artifacts found")
    print(json.dumps(results, indent=2))


if __name__ == "__main__":
    main()
