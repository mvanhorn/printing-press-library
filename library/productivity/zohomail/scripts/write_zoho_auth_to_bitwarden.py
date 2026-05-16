#!/usr/bin/env python3
from __future__ import annotations

import base64
import json
import os
import re
import subprocess
import sys
from pathlib import Path

ITEM_NAME = "Zoho Mail OAuth"
URI = "https://api-console.zoho.com"
FIELDS = ["ZOHO_CLIENT_ID", "ZOHO_CLIENT_SECRET", "ZOHO_REFRESH_TOKEN"]


def latest_value(key: str) -> str:
    if os.environ.get(key):
        return os.environ[key]
    history = Path.home() / ".zsh_history"
    if history.exists():
        text = history.read_text(errors="ignore")
        values = re.findall(rf"(?:export\s+)?{re.escape(key)}=['\"]?([^'\"\s;]+)", text)
        if values:
            return values[-1]
    return ""


def run(args: list[str], session: str | None = None, input_text: str | None = None, check: bool = True) -> subprocess.CompletedProcess[str]:
    cmd = ["bw", *args]
    if session:
        cmd.extend(["--session", session])
    return subprocess.run(cmd, input=input_text, text=True, capture_output=True, check=check)


def unlock() -> str:
    status = subprocess.run(["bw", "status"], text=True, capture_output=True, check=True)
    parsed = json.loads(status.stdout)
    if parsed.get("status") == "unlocked" and os.environ.get("BW_SESSION"):
        return os.environ["BW_SESSION"]
    unlock_proc = subprocess.run(["bw", "unlock", "--raw"], text=True, stdout=subprocess.PIPE)
    token = unlock_proc.stdout.strip()
    if unlock_proc.returncode != 0 or not token:
        raise SystemExit("Bitwarden unlock failed")
    return token


def encoded(item: dict) -> str:
    raw = json.dumps(item, separators=(",", ":")).encode()
    return base64.b64encode(raw).decode()


def base_item(values: dict[str, str]) -> dict:
    return {
        "type": 1,
        "name": ITEM_NAME,
        "notes": "Zoho Mail OAuth credentials for pp-zohomail-cli. Do not paste into chat.",
        "favorite": False,
        "fields": [{"name": k, "value": v, "type": 0} for k, v in values.items()],
        "login": {
            "username": values["ZOHO_CLIENT_ID"],
            "password": values["ZOHO_CLIENT_SECRET"],
            "uris": [{"uri": URI, "match": None}],
        },
    }


def main() -> int:
    values = {k: latest_value(k) for k in FIELDS}
    missing = [k for k, v in values.items() if not v]
    if missing:
        print("Missing values: " + ", ".join(missing), file=sys.stderr)
        return 1

    session = unlock()
    found = run(["list", "items", "--search", ITEM_NAME], session=session)
    items = json.loads(found.stdout or "[]")
    existing = next((item for item in items if item.get("name") == ITEM_NAME), None)

    if existing:
        item = run(["get", "item", existing["id"]], session=session)
        payload = json.loads(item.stdout)
        payload["name"] = ITEM_NAME
        payload["notes"] = "Zoho Mail OAuth credentials for pp-zohomail-cli. Do not paste into chat."
        payload["fields"] = [{"name": k, "value": v, "type": 0} for k, v in values.items()]
        payload.setdefault("login", {})
        payload["login"]["username"] = values["ZOHO_CLIENT_ID"]
        payload["login"]["password"] = values["ZOHO_CLIENT_SECRET"]
        payload["login"]["uris"] = [{"uri": URI, "match": None}]
        run(["edit", "item", existing["id"]], session=session, input_text=encoded(payload))
        print(f"updated\t{ITEM_NAME}")
    else:
        run(["create", "item"], session=session, input_text=encoded(base_item(values)))
        print(f"created\t{ITEM_NAME}")
    print("fields\tZOHO_CLIENT_ID ZOHO_CLIENT_SECRET ZOHO_REFRESH_TOKEN")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
