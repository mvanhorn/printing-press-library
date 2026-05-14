#!/usr/bin/env python3
"""Add x-mcp Cloudflare-pattern enrichment to the merged GHL spec.

>50 tools => recommended pattern per pre-generation MCP enrichment rules:
  - transport: [stdio, http]
  - orchestration: code
  - endpoint_tools: hidden
"""
import json
import sys
from pathlib import Path

p = Path(sys.argv[1])
doc = json.loads(p.read_text())

doc["x-mcp"] = {
    "transport": ["stdio", "http"],
    "orchestration": "code",
    "endpoint_tools": "hidden",
}

p.write_text(json.dumps(doc, indent=2))
print(f"Added x-mcp to {p}")
