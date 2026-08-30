#!/usr/bin/env python3
"""Fetch Kie.ai's docs.kie.ai endpoint pages and rebuild kie-final-openapi.yaml
and ../docs/MODELS.md from their embedded per-page OpenAPI fragments.

Usage:
    python3 build_spec.py

Requires: pyyaml, requests (or urllib fallback below -- no extra deps needed).
"""
import copy
import json
import re
import sys
import urllib.request
import yaml

BLOCK_RE = re.compile(r"## OpenAPI Specification\s*```yaml\s*\n(.*?)\n```", re.DOTALL)

# Dedicated (non-Market) endpoint pages -- one distinct real path each.
DEDICATED_PAGES = """
common-api/get-account-credits
common-api/download-url
4o-image-api/generate-4-o-image
4o-image-api/get-4-o-image-details
4o-image-api/get-4-o-image-download-url
flux-kontext-api/generate-or-edit-image
flux-kontext-api/get-image-details
runway-api/generate-ai-video
runway-api/get-ai-video-details
runway-api/extend-ai-video
runway-api/generate-aleph-video
runway-api/get-aleph-video-details
veo3-api/generate-veo-3-video
veo3-api/get-veo-3-video-details
veo3-api/get-veo-3-1080-p-video
veo3-api/get-veo-3-4k-video
veo3-api/extend-video
file-upload-api/upload-file-base-64
file-upload-api/upload-file-stream
file-upload-api/upload-file-url
suno-api/generate-music
suno-api/extend-music
suno-api/upload-and-cover-audio
suno-api/upload-and-extend-audio
suno-api/add-instrumental
suno-api/add-vocals
suno-api/get-music-details
suno-api/get-timestamped-lyrics
suno-api/boost-music-style
suno-api/cover-suno
suno-api/get-cover-suno-details
suno-api/replace-section
suno-api/generate-persona
suno-api/generate-mashup
suno-api/generate-lyrics
suno-api/get-lyrics-details
suno-api/convert-to-wav
suno-api/get-wav-details
suno-api/separate-vocals
suno-api/get-vocal-separation-details
suno-api/generate-midi
suno-api/get-midi-details
suno-api/create-music-video
suno-api/get-music-video-details
suno-api/generate-sounds
suno-api/suno-voice-validate
suno-api/suno-voice-validate-info
suno-api/suno-voice-generate
suno-api/suno-voice-record-info
suno-api/suno-voice-regenerate
suno-api/suno-voice-check-voice
""".split()

DOCS_BASE = "https://docs.kie.ai"


def fetch(url):
    req = urllib.request.Request(url, headers={"User-Agent": "kie-cli-spec-builder/1.0"})
    with urllib.request.urlopen(req, timeout=20) as resp:
        return resp.read().decode("utf-8", errors="replace")


def discover_market_urls():
    llms = fetch(f"{DOCS_BASE}/llms.txt")
    urls = re.findall(r"\(https://docs\.kie\.ai/market/[a-zA-Z0-9/_-]*\.md\)", llms)
    urls = sorted(set(u[1:-1] for u in urls if "/cn/" not in u))
    return urls


def load_block(text):
    m = BLOCK_RE.search(text)
    if not m:
        return None
    try:
        return yaml.safe_load(m.group(1))
    except Exception:
        return None


def clean_desc(text):
    if not isinstance(text, str):
        return text
    lines = text.split("\n")
    out = []
    for line in lines:
        stripped = line.strip()
        if re.match(r"^:::\s*\w*(\[\])?\s*$", stripped):
            continue
        m = re.match(r"^:::\s*\w+\[\]\s*(.*)$", stripped)
        if m and m.group(1):
            out.append(m.group(1))
            continue
        if stripped.startswith("> "):
            line = stripped[2:]
        out.append(line)
    result = "\n".join(out).strip()
    return re.sub(r"\n{3,}", "\n\n", result)


def clean_spec_tree(obj):
    if isinstance(obj, dict):
        for k in list(obj.keys()):
            if k.startswith("x-apidog") or k.startswith("x-run-in-apidog"):
                del obj[k]
        if "security" in obj and "responses" in obj:
            del obj["security"]
        if "description" in obj and isinstance(obj.get("description"), str):
            obj["description"] = clean_desc(obj["description"])
        if "summary" in obj and isinstance(obj.get("summary"), str):
            obj["summary"] = clean_desc(obj["summary"])
        for v in obj.values():
            clean_spec_tree(v)
    elif isinstance(obj, list):
        for item in obj:
            clean_spec_tree(item)


TAG_MAP = {
    "market": "Market",
    "common api": "Common",
    "file upload": "FileUpload",
    "claude": "Chat",
    "codex": "Chat",
    "4o image": "ImageGen4o",
    "flux kontext": "FluxKontext",
    "lyrics generation": "SunoLyrics",
    "music generation": "SunoMusic",
    "music video generation": "SunoMusicVideo",
    "sounds generation": "SunoSounds",
    "vocal removal": "SunoVocal",
    "wav conversion": "SunoWav",
    "suno api/voice": "SunoVoice",
    "veo3": "Veo3",
    "gemini omni": "GeminiOmni",
    "runway api/aleph": "RunwayAleph",
    "runway api": "Runway",
}


def normalize_tag(tag):
    t = tag.lower()
    if t == "market":
        return "Market"
    if "gpt" in t and "chat" in t:
        return "Chat"
    if "gemini" in t and "chat" in t:
        return "Chat"
    if "grok" in t and "chat" in t:
        return "Chat"
    for key, val in TAG_MAP.items():
        if key in t:
            return val
    return tag


def main():
    print("Fetching dedicated endpoint pages...", file=sys.stderr)
    merged_paths = {}
    merged_schemas = {}

    for slug in DEDICATED_PAGES:
        text = fetch(f"{DOCS_BASE}/{slug}.md")
        doc = load_block(text)
        if not doc or "paths" not in doc:
            print(f"  WARN: no OpenAPI block for {slug}", file=sys.stderr)
            continue
        for p, methods in doc["paths"].items():
            merged_paths.setdefault(p, {}).update(methods)
        for name, schema in (doc.get("components", {}) or {}).get("schemas", {}).items():
            merged_schemas[name] = schema

    print("Discovering Market model pages via llms.txt...", file=sys.stderr)
    market_slugs = discover_market_urls()
    print(f"  found {len(market_slugs)} market pages", file=sys.stderr)

    catalog = []
    nano_doc = None
    common_detail_doc = None
    for url in market_slugs:
        text = fetch(url)
        doc = load_block(text)
        if not doc or "paths" not in doc:
            continue
        paths = doc["paths"]
        if url.endswith("google/nano-banana.md"):
            nano_doc = doc
        if url.endswith("common/get-task-detail.md"):
            common_detail_doc = doc
        if "/api/v1/jobs/createTask" in paths:
            ct = paths["/api/v1/jobs/createTask"]["post"]
            props = (
                ct.get("requestBody", {})
                .get("content", {})
                .get("application/json", {})
                .get("schema", {})
                .get("properties", {})
            )
            model_enum = (props.get("model", {}) or {}).get("enum", [])
            model_val = model_enum[0] if model_enum else (props.get("model", {}) or {}).get("default", "")
            if model_val:
                catalog.append(
                    {"model": model_val, "summary": ct.get("summary", ""), "category": ct.get("tags", ["?"])[0]}
                )
            continue  # shared path, handled generically below
        if "/api/v1/jobs/recordInfo" in paths:
            continue  # handled generically below
        for p, methods in paths.items():
            merged_paths.setdefault(p, {}).update(methods)
        for name, schema in (doc.get("components", {}) or {}).get("schemas", {}).items():
            merged_schemas[name] = schema

    if nano_doc is None or common_detail_doc is None:
        print("ERROR: could not find nano-banana or common/get-task-detail page for the unified Market spec", file=sys.stderr)
        sys.exit(1)

    create_op = copy.deepcopy(nano_doc["paths"]["/api/v1/jobs/createTask"]["post"])
    create_op["summary"] = "Create a Market model generation task"
    create_op["operationId"] = "market-create-task"
    create_op["tags"] = ["Market"]
    create_op["description"] = (
        "Unified entry point for every model in the Kie.ai Market catalog (image, video, and audio "
        "generation). Pass the target model's identifier plus its model-specific input payload. "
        "See docs/MODELS.md in this repo, or https://docs.kie.ai/market/quickstart, for the full "
        "and growing catalog of supported `model` values and their `input` shapes."
    )
    props = create_op["requestBody"]["content"]["application/json"]["schema"]["properties"]
    props["model"] = {
        "type": "string",
        "description": (
            "Market model identifier, e.g. 'google/nano-banana', 'kling/v2-1-pro', "
            "'bytedance/v1-pro-text-to-video'. See docs/MODELS.md for the full catalog."
        ),
        "examples": ["google/nano-banana", "kling/v2-1-pro", "bytedance/v1-pro-text-to-video"],
    }
    props["input"] = {
        "type": "object",
        "description": "Model-specific input payload. Shape varies per model -- see docs/MODELS.md.",
        "additionalProperties": True,
    }
    merged_paths["/api/v1/jobs/createTask"] = {"post": create_op}

    query_op = copy.deepcopy(common_detail_doc["paths"]["/api/v1/jobs/recordInfo"]["get"])
    query_op["operationId"] = "market-query-task"
    query_op["tags"] = ["Market"]
    merged_paths["/api/v1/jobs/recordInfo"] = {"get": query_op}

    for name, schema in (nano_doc.get("components", {}) or {}).get("schemas", {}).items():
        merged_schemas.setdefault(name, schema)
    for name, schema in (common_detail_doc.get("components", {}) or {}).get("schemas", {}).items():
        merged_schemas.setdefault(name, schema)

    for p, methods in merged_paths.items():
        for m, op in methods.items():
            old = (op.get("tags") or ["?"])[0]
            op["tags"] = [normalize_tag(old)]

    spec = {
        "openapi": "3.0.1",
        "info": {
            "title": "Kie.ai API",
            "description": (
                "Unified API for Kie.ai's AI generation platform: image, video, music, and chat models, "
                "plus account and file utilities. Async generation endpoints return a taskId; poll the "
                "matching record-info/get-details endpoint (or use a callBackUrl) to retrieve results."
            ),
            "version": "1.0.0",
        },
        "servers": [{"url": "https://api.kie.ai", "description": "Production"}],
        "security": [{"BearerAuth": []}],
        "paths": merged_paths,
        "components": {
            "securitySchemes": {
                "BearerAuth": {
                    "type": "http",
                    "scheme": "bearer",
                    "bearerFormat": "API Key",
                    "description": (
                        "All API requests require a Bearer Token. Add header "
                        "`Authorization: Bearer YOUR_API_KEY`. Get a key at https://kie.ai/api-key."
                    ),
                }
            },
            "schemas": merged_schemas,
        },
    }
    clean_spec_tree(spec)

    with open("kie-final-openapi.yaml", "w", encoding="utf-8") as f:
        yaml.dump(spec, f, sort_keys=False, allow_unicode=True, width=100)
    print(f"Wrote kie-final-openapi.yaml ({len(merged_paths)} paths)", file=sys.stderr)

    # Model catalog doc
    cats = {}
    for m in catalog:
        raw = m["category"]
        parts = [p for p in raw.split("/") if p not in ("docs", "en", "Market")]
        label = " > ".join(re.sub(r"\s+", " ", p).strip() for p in parts)
        cats.setdefault(label, []).append(m)

    lines = ["# Kie.ai Market Model Catalog", ""]
    lines.append(f"All {len(catalog)} models below share the same two endpoints:")
    lines.append("")
    lines.append("- Create a task: `kie kie-ai-jobs market-create-task --model <id> --input '{...}'`")
    lines.append("- Query a task:  `kie kie-ai-jobs market-query-task --task-id <id>`")
    lines.append("")
    lines.append(
        "This catalog is a point-in-time snapshot from docs.kie.ai, regenerated by "
        "`research/build_spec.py` (see README.md for the weekly-refresh mechanism)."
    )
    lines.append("")
    for label in sorted(cats):
        lines.append(f"## {label}")
        lines.append("")
        lines.append("| Model | `--model` value |")
        lines.append("|---|---|")
        for m in sorted(cats[label], key=lambda x: x["model"]):
            lines.append(f"| {m['summary']} | `{m['model']}` |")
        lines.append("")

    with open("../docs/MODELS.md", "w", encoding="utf-8") as f:
        f.write("\n".join(lines))
    print(f"Wrote ../docs/MODELS.md ({len(catalog)} models)", file=sys.stderr)


if __name__ == "__main__":
    main()
