# Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
"""Replay the independent redirect and source-query tests without host paths."""
import json
import pathlib
import subprocess
import tempfile


def main():
    proof_dir = pathlib.Path(__file__).resolve().parent
    cli_dir = pathlib.Path(__file__).resolve().parents[4]
    template = json.loads((proof_dir / "redirect-overlay.json").read_text())

    def expand(value):
        for token, directory in [("<cli-dir>", cli_dir), ("<proof-dir>", proof_dir)]:
            if value.startswith(token + "/"):
                return str(directory / value[len(token) + 1:])
        raise ValueError("Overlay path lacks a supported portable root token")

    overlay = {"Replace": {expand(k): expand(v) for k, v in template["Replace"].items()}}
    with tempfile.TemporaryDirectory(prefix="airport-limousine-review-") as directory:
        path = pathlib.Path(directory) / "overlay.json"
        path.write_text(json.dumps(overlay))
        path.chmod(0o600)
        result = subprocess.run(
            ["go", "test", "-overlay=" + str(path), "./internal/limousine", "-run",
             "^TestReview(RedirectBudgetCountsWireRequests|PageSourceURLQueryPreserved)$", "-count=1", "-v"],
            cwd=cli_dir,
            timeout=120,
            check=False,
        )
    return result.returncode


if __name__ == "__main__":
    raise SystemExit(main())
