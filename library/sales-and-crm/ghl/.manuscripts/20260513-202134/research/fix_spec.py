#!/usr/bin/env python3
"""Repair OpenAPI 3.0 quirks in the merged GHL spec so the Printing Press
parser accepts it:

1. Schema-level `examples` as an object (3.1-ish, malformed) → convert to a
   list (`[]interface{}` is what the parser wants).
2. Parameter-level `examples` as an object (valid OpenAPI 3.0
   `Map[string, Example]`) — leave alone; parser handles those.
3. Schema-level `example` (singular) where the value type is the schema's
   array element type — leave alone.

We discriminate Schema vs Parameter by checking the parent node. The Schema
parent has fields like `type`, `properties`, `enum`; the Parameter parent
has `in` and `name`.
"""
import json
import sys
from pathlib import Path

SPEC = Path(sys.argv[1])


def looks_like_schema(parent: dict) -> bool:
    schema_markers = {
        "type", "properties", "items", "enum",
        "allOf", "oneOf", "anyOf", "additionalProperties", "$ref",
        "format", "nullable", "required", "minimum", "maximum",
        "minLength", "maxLength", "pattern", "default",
    }
    parameter_markers = {"in", "name"}
    if parameter_markers.issubset(parent.keys()):
        return False
    return bool(schema_markers & parent.keys())


def fix_examples(node, parent=None):
    if isinstance(node, dict):
        ex = node.get("examples")
        if isinstance(ex, dict) and looks_like_schema(node):
            # Map of "label" -> value (possibly with "value" sub-key) — flatten.
            flat = []
            for v in ex.values():
                if isinstance(v, dict) and "value" in v:
                    flat.append(v["value"])
                else:
                    flat.append(v)
            node["examples"] = flat
        for k, v in list(node.items()):
            fix_examples(v, parent=node)
    elif isinstance(node, list):
        for item in node:
            fix_examples(item, parent=parent)


def strip_surrogate_emoji_in_descriptions(node):
    """Replace 'orphaned' high-surrogate emoji byte sequences inside strings
    that some YAML parsers reject. Replaces the leading '🚨 ' (U+1F6A8) prefix
    found on Deprecated description fields with `[Deprecated] `.

    Defensive: applied to every string-valued `description`. The JSON parser
    handles surrogates fine; this is for the YAML fallback path in the press's
    spec cleaner."""
    import re
    if isinstance(node, dict):
        for k, v in list(node.items()):
            if k == "description" and isinstance(v, str):
                # Drop any non-BMP code points (>U+FFFF) — these become the
                # surrogate pairs YAML can't handle. The user only loses an
                # emoji; the prose stays intact.
                node[k] = "".join(ch for ch in v if ord(ch) <= 0xFFFF)
            else:
                strip_surrogate_emoji_in_descriptions(v)
    elif isinstance(node, list):
        for item in node:
            strip_surrogate_emoji_in_descriptions(item)


def main():
    spec = json.loads(SPEC.read_text())
    fix_examples(spec)
    strip_surrogate_emoji_in_descriptions(spec)
    SPEC.write_text(json.dumps(spec, indent=2))
    print(f"Fixed {SPEC}")


if __name__ == "__main__":
    main()
