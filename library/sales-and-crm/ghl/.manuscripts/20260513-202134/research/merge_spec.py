#!/usr/bin/env python3
"""Merge GHL per-resource OpenAPI JSON files into one spec.

Pre-processing rules:
- Inline external $refs that point to ../common/common-schemas.json.
- Strip the required `Version` header parameter from every operation
  (single allowed enum value 2021-07-28 → injected by the printed client).
- Keep only the `bearer` security scheme (PIT path); the OAuth-flavored
  Agency-Access/Location-Access schemes are out of scope for v1.
- Rewrite per-operation `security` blocks so each one references just `bearer`.
- Merge components.schemas / parameters / responses / requestBodies.
- Emit a single OpenAPI 3.0 document.

Output: $RESEARCH_DIR/ghl-merged-openapi.json
"""
from __future__ import annotations

import copy
import json
import sys
from pathlib import Path

RESEARCH_DIR = Path(sys.argv[1])
APPS_DIR = RESEARCH_DIR / "apps"
COMMON_PATH = RESEARCH_DIR / "common" / "common-schemas.json"
OUTPUT = RESEARCH_DIR / "ghl-merged-openapi.json"

PRIORITY = [
    "contacts",
    "conversations",
    "calendars",
    "opportunities",
    "workflows",
    "custom-fields",
    "locations",
    "users",
]


def load(p: Path) -> dict:
    return json.loads(p.read_text())


def merge_components(dst: dict, src: dict) -> None:
    """Merge OpenAPI components from src into dst (in-place)."""
    src_components = src.get("components", {})
    dst_components = dst.setdefault("components", {})
    for key in ("schemas", "parameters", "requestBodies", "responses", "headers", "examples"):
        s = src_components.get(key)
        if not s:
            continue
        d = dst_components.setdefault(key, {})
        for name, val in s.items():
            if name in d and d[name] != val:
                # Schema name collision with different definitions — namespace
                # the new one with the resource prefix to avoid silent overwrites.
                # This shouldn't happen for GHL per-resource files but defend.
                ns = src.get("_resource", "x") + "_" + name
                d[ns] = val
                # Rewrite refs across src so they point at the namespaced copy.
                _rewrite_refs_in_place(src, f"#/components/{key}/{name}",
                                       f"#/components/{key}/{ns}")
            else:
                d[name] = val


def _rewrite_refs_in_place(node, old: str, new: str) -> None:
    if isinstance(node, dict):
        for k, v in list(node.items()):
            if k == "$ref" and isinstance(v, str) and v == old:
                node[k] = new
            else:
                _rewrite_refs_in_place(v, old, new)
    elif isinstance(node, list):
        for item in node:
            _rewrite_refs_in_place(item, old, new)


def rewrite_external_refs(node, external_base="../common/common-schemas.json"):
    """Replace ../common/common-schemas.json#/...  →  local #/... (common schemas
    will be merged into the root components.schemas)."""
    if isinstance(node, dict):
        for k, v in list(node.items()):
            if k == "$ref" and isinstance(v, str) and v.startswith(external_base):
                # ../common/common-schemas.json#/components/schemas/BadRequestDTO
                fragment = v.split("#", 1)[1] if "#" in v else ""
                node[k] = "#" + fragment
            else:
                rewrite_external_refs(v, external_base)
    elif isinstance(node, list):
        for item in node:
            rewrite_external_refs(item, external_base)


def strip_version_header(operation: dict) -> None:
    params = operation.get("parameters", [])
    operation["parameters"] = [
        p for p in params
        if not (
            p.get("name") == "Version"
            and p.get("in") == "header"
        )
    ]


def normalize_security(operation: dict) -> None:
    """Replace per-operation security blocks with a single bearer reference,
    preserving the scopes listed in the original block (any scheme).
    """
    sec = operation.get("security")
    if not sec:
        return
    # Collect all scopes from any of the schemes (Agency-Access, Location-Access, bearer).
    scopes: list[str] = []
    for entry in sec:
        for _name, scope_list in entry.items():
            for s in scope_list:
                if s not in scopes:
                    scopes.append(s)
    operation["security"] = [{"bearer": scopes}]


def normalize_operations(paths: dict) -> None:
    for _path, item in paths.items():
        for method in ("get", "post", "put", "patch", "delete", "head", "options"):
            op = item.get(method)
            if not op:
                continue
            strip_version_header(op)
            normalize_security(op)


def main() -> None:
    common = load(COMMON_PATH)
    common_schemas = common.get("components", {}).get("schemas", {})

    # Seed merged doc from contacts.json
    base_resource = "contacts"
    merged = load(APPS_DIR / f"{base_resource}.json")
    merged["_resource"] = base_resource
    merged["info"] = {
        "title": "GoHighLevel API v2",
        "description": (
            "GoHighLevel (HighLevel) Sub-Account API. Authenticates with a Private "
            "Integration Token (PIT) issued from a sub-account location. Every request "
            "carries the API version header automatically; you only need to pass the "
            "PIT and (when required) the location id."
        ),
        "version": "2021-07-28",
        "contact": {"name": "GoHighLevel", "url": "https://highlevel.stoplight.io/docs/integrations/"},
    }
    merged["servers"] = [{"url": "https://services.leadconnectorhq.com"}]

    # Replace per-resource security schemes with a clean bearer-only block
    merged["components"]["securitySchemes"] = {
        "bearer": {
            "type": "http",
            "scheme": "bearer",
            "bearerFormat": "PIT",
            "description": (
                "Private Integration Token (PIT) issued for a HighLevel sub-account "
                "location. Generated from Settings → Private Integrations in the sub-"
                "account UI. Sent as `Authorization: Bearer <pit>`."
            ),
        }
    }

    # Top-level security so every operation defaults to bearer when unspecified
    merged["security"] = [{"bearer": []}]

    # Inline common error schemas
    components_schemas = merged["components"].setdefault("schemas", {})
    for name, sch in common_schemas.items():
        components_schemas.setdefault(name, sch)

    # Rewrite external refs in the base resource then normalize operations
    rewrite_external_refs(merged)
    normalize_operations(merged["paths"])

    # Now layer in every other resource
    for resource in PRIORITY:
        if resource == base_resource:
            continue
        src = load(APPS_DIR / f"{resource}.json")
        src["_resource"] = resource
        rewrite_external_refs(src)
        # Add paths (no collisions expected — each resource uses a distinct prefix)
        for path, item in src.get("paths", {}).items():
            if path in merged["paths"]:
                # If this ever happens, namespace it to surface the conflict.
                merged["paths"][f"{path}_{resource}"] = item
            else:
                merged["paths"][path] = item
        merge_components(merged, src)
        normalize_operations(merged["paths"])

    merged.pop("_resource", None)

    # Drop leftover per-resource security scheme leftovers that may have been
    # merged in. Only `bearer` should remain.
    schemes = merged["components"]["securitySchemes"]
    for stray in ("Agency-Access", "Agency-Access-Only", "Location-Access", "Location-Access-Only"):
        schemes.pop(stray, None)

    OUTPUT.write_text(json.dumps(merged, indent=2))
    print(f"Wrote {OUTPUT}")
    print(f"  paths: {len(merged['paths'])}")
    print(f"  schemas: {len(merged['components']['schemas'])}")
    print(f"  parameters: {len(merged['components'].get('parameters', {}))}")


if __name__ == "__main__":
    main()
