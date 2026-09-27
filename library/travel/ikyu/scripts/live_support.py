#!/usr/bin/env python3
"""Bounded anonymous test helpers; no account state, raw SSR persistence or actions."""
import datetime as dt
import html
from html.parser import HTMLParser
import json
from pathlib import Path
import re
import subprocess
import time
from urllib.parse import urlencode

JST = dt.timezone(dt.timedelta(hours=9))
MAX_BODY = 8 * 1024 * 1024
ID = re.compile(r"[0-9]{8}:[0-9]{8}:[0-9]{8}\Z")
PRICE_FIELDS = {
    "source_amount": "amount", "base_discount_amount": "baseDiscountAmount",
    "headline_before_points": "baseDiscountAmount", "instant_points_payable": "discountAmount",
    "earn_points_payable": "discountAmountEarn", "payable_without_coupon": "discountAmountWithoutCoupon",
    "points_earned": "point", "points_rate": "pointRate", "points_applied": "instantPoint",
    "points_applied_rate": "instantPointRate",
}

def timestamp():
    return dt.datetime.now(dt.timezone.utc).isoformat()

def stay_dates(check_in=None, check_out=None, nights=1):
    today = dt.datetime.now(JST).date()
    start = dt.date.fromisoformat(check_in) if check_in else today + dt.timedelta(days=21)
    end = dt.date.fromisoformat(check_out) if check_out else start + dt.timedelta(days=nights)
    if not today <= start <= today + dt.timedelta(days=365) or not 1 <= (end-start).days <= 30:
        raise ValueError("stay must be future Japan dates, check-in within 365 days and 1–30 nights")
    return start.isoformat(), end.isoformat()

def selection(value):
    if not ID.fullmatch(value):
        raise ValueError("selection requires property:room:plan with three eight-digit IDs")
    return value.split(":")

def stay_args(check_in, check_out, adults=2, rooms=1):
    return ["--check-in", check_in, "--check-out", check_out, "--adults", str(adults), "--rooms", str(rooms)]

def cli(binary, args, cache, expected=0):
    started = time.perf_counter()
    p = subprocess.run([str(binary), *args, "--cache-dir", str(cache), "--timeout", "45s"],
                       capture_output=True, timeout=50)
    if p.returncode != expected:
        diagnostic = p.stderr.decode(errors="replace")[:600]
        raise AssertionError(f"{' '.join(args[:4])}: exit {p.returncode}, expected {expected}: {diagnostic}")
    if expected:
        return {"exit_code": p.returncode, "elapsed_seconds": round(time.perf_counter()-started, 6)}
    if len(p.stdout) > 2 * MAX_BODY:
        raise AssertionError("unbounded command output")
    if p.stdout.count(b"\n") != 1:
        raise AssertionError("default JSON must occupy one line")
    value = json.loads(p.stdout)
    value.setdefault("_measurement", {}).update(output_bytes=len(p.stdout), elapsed_seconds=round(time.perf_counter()-started, 6))
    return value

def discover(binary, property_id, dates, cache):
    result = cli(binary, ["stay", "rooms", property_id, *stay_args(*dates), "--limit", "2", "--plan-limit", "3"], cache)
    for room in result["data"]:
        for plan in room["plans"]:
            if plan["source_point_variation"] in (None, 0):
                return f"{property_id}:{room['id']}:{plan['id']}"
    raise AssertionError(f"no default-variation room-plan observed for {property_id}; supply --hotel-offer/--ryokan-offer")

def canonical(selector, check_in, check_out, adults=2, rooms=1):
    prop, room, plan = selection(selector)
    start, end = dt.date.fromisoformat(check_in), dt.date.fromisoformat(check_out)
    q = urlencode({"cid": start.strftime("%Y%m%d"), "cod": end.strftime("%Y%m%d"), "lc": (end-start).days, "ppc": adults, "rc": rooms})
    return f"https://www.ikyu.com/{prop}/{plan}/{room}/?{q}"

def fetch_html(url):
    if not re.fullmatch(r"https://www\.ikyu\.com/[0-9]{8}/[0-9]{8}/[0-9]{8}/\?[a-z0-9=&]+", url):
        raise ValueError("only canonical public room-plan URLs may be fetched")
    started = time.perf_counter()
    p = subprocess.run(["curl", "--silent", "--show-error", "--fail", "--proto", "=https", "--max-time", "25",
                        "--max-filesize", str(MAX_BODY), "--write-out", "\n%{http_code}", url],
                       capture_output=True, timeout=30)
    if p.returncode:
        raise AssertionError(f"public HTML fetch failed: curl {p.returncode}: {p.stderr.decode(errors='replace')[:300]}")
    body, code = p.stdout.rsplit(b"\n", 1)
    if code != b"200" or len(body) > MAX_BODY:
        raise AssertionError("public HTML must be HTTP200 within 8MiB")
    return body.decode("utf-8"), {"url": url, "captured_at": timestamp(), "http_status": 200,
                                "response_bytes": len(body), "elapsed_seconds": round(time.perf_counter()-started, 6)}

def nuxt_operations(document, wanted):
    """Decode only selected public data operations, never state/review caches."""
    m = re.search(r'<script\b[^>]*\bid="__NUXT_DATA__"[^>]*>(.*?)</script>', document, re.S)
    if not m:
        raise AssertionError("public SSR Nuxt data absent")
    table = json.loads(m.group(1)); memo = {}; active = set(); count = 0
    def ref(index, depth=0):
        nonlocal count
        if not isinstance(index, int) or isinstance(index, bool) or depth > 200:
            raise AssertionError("invalid Nuxt reference/depth")
        if index < 0:
            if index in (-1, -2): return None
            raise AssertionError("non-finite Nuxt value")
        if index >= len(table) or index in active:
            raise AssertionError("invalid/cyclic Nuxt index")
        if index in memo: return memo[index]
        count += 1
        if count > 100000: raise AssertionError("Nuxt expansion bound exceeded")
        active.add(index); value = table[index]
        if isinstance(value, dict): result = {k: ref(v, depth+1) for k,v in value.items()}
        elif isinstance(value, list):
            if value and isinstance(value[0], str):
                if value[0] not in ("Reactive", "ShallowReactive", "Ref", "ShallowRef") or len(value) != 2:
                    raise AssertionError("unsupported Nuxt tag")
                result = ref(value[1], depth+1)
            else: result = [ref(v, depth+1) for v in value]
        else: result = value
        active.remove(index); memo[index] = result; return result
    if len(table) < 4 or not isinstance(table[1], dict) or "data" not in table[1]:
        raise AssertionError("Nuxt data root absent")
    index = table[1]["data"]
    for _ in range(5):
        raw = table[index]
        if isinstance(raw, dict): break
        if not isinstance(raw, list) or len(raw) != 2 or raw[0] not in ("Reactive", "ShallowReactive", "Ref", "ShallowRef"):
            raise AssertionError("Nuxt data wrapper changed")
        index = raw[1]
    else: raise AssertionError("Nuxt data wrapper bound exceeded")
    found = {}
    for key, index in raw.items():
        if key.endswith("-skip"): continue
        try: meta = json.loads(key[:key.rfind("}")+1])
        except (ValueError, TypeError): continue
        if meta.get("o") in wanted:
            name = meta["o"]
            if name in found: raise AssertionError("ambiguous SSR operation")
            found[name] = {"variables": meta.get("v", {}), "value": ref(index)}
    if set(found) != set(wanted):
        raise AssertionError(f"missing public SSR operations: {sorted(set(wanted)-set(found))}")
    return found

class VisibleText(HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=True); self.hidden=0; self.parts=[]
    def handle_starttag(self, tag, attrs):
        if tag in ("script", "style", "svg"): self.hidden += 1
    def handle_endtag(self, tag):
        if tag in ("script", "style", "svg") and self.hidden: self.hidden -= 1
    def handle_data(self, data):
        if not self.hidden: self.parts.append(data)

def visible_text(document):
    parser=VisibleText(); parser.feed(document)
    return " ".join(" ".join(parser.parts).split())

def source_text(value):
    return html.unescape(value.strip()) if isinstance(value, str) else value

def cancellation(policy):
    if not policy or not policy.get("rules"): raise AssertionError("complete public cancellation policy absent")
    rules=[]
    for row in policy["rules"]:
        kind = row.get("__typename"); amount=row.get("amount") or {}
        if kind not in ("CancelPolicyRuleDay", "CancelPolicyRuleNoShow"):
            raise AssertionError("unknown cancellation rule kind")
        if amount.get("__typename") not in ("CancelPolicyRuleAmountRate", "CancelPolicyRuleAmountFixed"):
            raise AssertionError("unknown cancellation amount kind")
        rules.append({"type": kind, "day": row.get("day"), "hour_minutes": row.get("hourMinutes"),
                      "rate": amount.get("rate"), "amount": amount.get("amount")})
    return {"id": policy.get("id"), "known": True, "rules": rules}

def independent_offer(document):
    ops = nuxt_operations(document, {"RoomPlanDetailAlt", "RoomPlanDetailAmount"})
    detail = ops["RoomPlanDetailAlt"]["value"]["accommodation"]["roomPlan"]
    amount = ops["RoomPlanDetailAmount"]["value"]["accommodation"]["roomPlan"]["booking"]["amount"]
    if not amount: raise AssertionError("public SSR offer has no amount; choose an available selection")
    plan = detail["plan"]; room = detail["room"]
    return {"room_name": source_text(room["name"]), "plan_name": source_text(plan["name"]),
            "meal": {"code": plan["meal"]["code"], "name": source_text(plan["meal"]["name"])},
            "cancellation": cancellation(plan.get("cancelPolicy")),
            "prices": {out: amount.get(src) for out,src in PRICE_FIELDS.items()},
            "nightly_dates": [row["date"] for row in amount.get("details") or []],
            "occupancy": {name: amount.get(name) for name in ("peopleCount","roomCount","lodgingCount","childACount","childBCount","childCCount","childDCount","childECount","childFCount")}}

def write_workflow(path, dates):
    start,end=dates
    Path(path).write_text(f'''# Anonymous live search → returned rooms → exact offer. Generated with future JST dates.
workflows:
  - name: Find a dated Tokyo stay and inspect its exact room-plan offer
    primary: true
    steps:
      - command: stay search --destination tokyo --check-in {start} --check-out {end} --adults 2 --limit 1 --timeout 25s
        mode: live
        expect_fields: [data, stay, freshness, dependency_freshness, pagination, stats]
        extract:
          property: $.data[0].id
      - command: stay rooms ${{property}} --check-in {start} --check-out {end} --adults 2 --limit 1 --plan-limit 1 --timeout 25s
        mode: live
        expect_fields: [data, stay, freshness, pagination, occupancy_echo_verified, date_echo_verified, stats]
        extract:
          room: $.data[0].id
          plan: $.data[0].plans[0].id
      - command: stay offer ${{property}} ${{room}} ${{plan}} --check-in {start} --check-out {end} --adults 2 --timeout 25s
        mode: live
        expect_fields: [data, freshness, stats]
''')

def save_report(path, report):
    text=json.dumps(report,ensure_ascii=False,indent=2)+"\n"
    if path: Path(path).write_text(text)
    print(json.dumps(report,ensure_ascii=False,separators=(",", ":")))
