#!/usr/bin/env python3
"""Derive the adapted provider catalog from actual stdio MCP descriptors."""
import argparse
import datetime
import json
import pathlib
import subprocess
import sys

root = pathlib.Path(__file__).resolve().parents[1]
parser = argparse.ArgumentParser()
parser.add_argument("--mcp-binary", default=str(root / "build/japan-bus-online-pp-mcp"))
parser.add_argument("--research", help="Approved run research.json for verified feature descriptions")
args = parser.parse_args()
catalog_path = root / "evidence/mcp-runtime-tools.json"
subprocess.run([
    sys.executable, str(root / "scripts/mcp-probe.py"),
    "--binary", args.mcp_binary, "--output", str(catalog_path),
    "--probe-output", str(root / "evidence/mcp-runtime-probe.json"),
], check=True, cwd=root)
catalog = json.loads(catalog_path.read_text())
tools = catalog["tools"]
provider_names = {"routes_list", "bus_route", "bus_services", "bus_quote", "bus_conditions"}
assert provider_names <= {tool["name"] for tool in tools}
for tool in tools:
    if tool["name"] in provider_names:
        assert tool["annotations"]["readOnlyHint"]
        assert not tool["annotations"]["destructiveHint"]
manifest_path = root / "tools-manifest.json"
previous = json.loads(manifest_path.read_text())
provenance = root / "generator-endpoint-provenance.json"
if not provenance.exists():
    previous["catalog_role"] = "historical_generator_endpoint_input_not_runtime_tools"
    provenance.write_text(json.dumps(previous, indent=2) + "\n")
manifest_path.write_text(json.dumps({
    "api_name": "japan-bus-online", "base_url": "https://japanbusonline.com",
    "description": "Read-only bus routes, dated inventory and source-grounded fares.",
    "mcp_ready": "full", "http_transport": "standard-go-http",
    "auth": {"type": "none"}, "required_headers": [],
    "catalog_role": "actual_registered_runtime_tools",
    "catalog_source": "stdio tools/list from companion MCP binary",
    "generated_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
    "tools": tools,
}, indent=2) + "\n")
metadata_path = root / ".printing-press.json"
metadata = json.loads(metadata_path.read_text())
metadata.setdefault("mcp_generation_endpoint_count", metadata.get("mcp_tool_count"))
metadata["mcp_tool_count"] = len(tools)
metadata["mcp_public_tool_count"] = len(tools)
metadata["mcp_provider_tool_count"] = len(provider_names)
metadata["mcp_catalog_source"] = "tools-manifest.json actual runtime catalog"
research_path = pathlib.Path(args.research) if args.research else None
if research_path is None and (root / ".press-path").exists():
    research_path = pathlib.Path((root / ".press-path").read_text().strip()) / "research.json"
if research_path is None:
    research_path = root / ".manuscripts" / metadata["run_id"] / "research.json"
if research_path.exists():
    research = json.loads(research_path.read_text())
    for key in ["novel_features", "novel_features_built"]:
        verified = [{field: row[field] for field in ["name", "command", "description"]} for row in research.get(key, [])]
        for row in verified:
            assert row["command"].replace(" ", "_") in {tool["name"] for tool in tools}
        metadata[key] = verified
metadata_path.write_text(json.dumps(metadata, indent=2) + "\n")
for filename in ["README.md", "SKILL.md"]:
    path = root / filename
    text = path.read_text()
    for statement in [
        "These capabilities aren't available in any other tool for this API.",
        "These features aren't available in any other tool for this API.",
    ]:
        text = text.replace(statement + "\n\n", "")
    if filename == "SKILL.md":
        canonical = (root / "evidence/canonical-install-section.txt").read_text()
        start = text.index("## Prerequisites: Install the CLI")
        end = text.index("\n## ", start + 1)
        section = text[start:end]
        if not section.startswith(canonical):
            raise RuntimeError("Generated installation text changed; review the canonical section before refreshing")
        extra = section[len(canonical):].strip()
        if extra:
            # Press may append approved provider prose inside its protected
            # installer section. Keep that prose under the workflow instead.
            text = text[:start] + canonical + "\n" + text[end + 1:]
            workflow = text.index("## Workflow\n", start)
            if extra not in text[workflow:]:
                insertion = workflow + len("## Workflow\n")
                text = text[:insertion] + "\n" + extra + "\n" + text[insertion:]
    path.write_text(text)
print(json.dumps({"runtime_tools": len(tools), "provider_tools": len(provider_names), "manifest": "tools-manifest.json"}))
