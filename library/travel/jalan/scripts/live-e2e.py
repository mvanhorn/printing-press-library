#!/usr/bin/env python3
"""Read-only live acceptance and efficiency proof; never saves raw source pages.

Run from the generated module after building:
  python3 scripts/live-e2e.py --cli bin/jalan-pp-cli --output /tmp/jalan-live-proof.json

The selected dates are source investigation fixtures, not promises of inventory.
A changed source condition produces a failing proof, never a fabricated pass.
Only anonymous Jalan pages and private temporary cache observations are read.
"""
import argparse
import datetime as dt
import hashlib
import html.parser
import json
import os
import pathlib
import re
import subprocess
import sys
import tempfile
import time
import urllib.parse
import urllib.request

MAX_CLI_REQUESTS = 48
MAX_WITNESS_REQUESTS = 14
MAX_COMMANDS = 64
MAX_SOURCE_BYTES = 4 * 1024 * 1024
MAX_SNAPSHOT_FILE_BYTES = 16 * 1024 * 1024 + 1024
MAX_SNAPSHOT_BODY_BYTES = 16 * 1024 * 1024


class SourceText(html.parser.HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.parts = []
        self.skip = 0
        self.stack = []
        self.captures = []
        self.charge_quotes = []
        self.card_captures = []
        self.cards = {}

    def handle_starttag(self, tag, attrs):
        if tag in ("script", "style"):
            self.skip += 1
        if tag not in ("area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr"):
            self.stack.append(tag)
            attributes = dict(attrs)
            classes = attributes.get("class", "").split()
            if "p-planOverview__charge" in classes:
                self.captures.append([len(self.stack), []])
            identifier = attributes.get("id", "")
            card_id = re.fullmatch(r"yadNo([0-9]+)", identifier)
            if ("p-yadoCassette" in classes and "p-yadoCassette--pr" not in classes
                    and not identifier.startswith("sa_") and card_id):
                self.card_captures.append([len(self.stack), card_id.group(1), []])

    def handle_endtag(self, tag):
        if tag in ("script", "style") and self.skip:
            self.skip -= 1
        if tag in self.stack:
            depth = len(self.stack) - self.stack[::-1].index(tag)
            finished = [capture for capture in self.captures if capture[0] >= depth]
            for capture in finished:
                self.charge_quotes.append(compact_text(" ".join(capture[1])))
                self.captures.remove(capture)
            finished_cards = [capture for capture in self.card_captures if capture[0] >= depth]
            for capture in finished_cards:
                self.cards.setdefault(capture[1], []).append(compact_text(" ".join(capture[2])))
                self.card_captures.remove(capture)
            self.stack = self.stack[:depth-1]

    def handle_data(self, data):
        if not self.skip:
            self.parts.append(data)
            for capture in self.captures:
                capture[1].append(data)
            for capture in self.card_captures:
                capture[2].append(data)


def compact_text(value):
    return re.sub(r"\s+", "", str(value))


def require(condition, message):
    if not condition:
        raise AssertionError(message)


class Runner:
    def __init__(self, args, home):
        self.args = args
        self.home = home
        self.rows = []
        self.sources = {}
        self.witness_requests = 0
        self.cli_requests = 0
        self.calls = 0
        self.current = None
        self.saved = {}

    def invoke(self, arguments, expected=0, help_output=False):
        require(self.calls < MAX_COMMANDS, "command budget exhausted")
        require(self.cli_requests <= MAX_CLI_REQUESTS - 4, "upstream request budget exhausted")
        self.calls += 1
        argv = [str(self.args.cli), "--no-learn", "--home", self.home,
                "--timeout", "45s", "stay", *arguments]
        env = dict(os.environ, JALAN_NO_LEARN="true", NO_COLOR="1")
        started = time.monotonic()
        with tempfile.TemporaryFile() as stdout, tempfile.TemporaryFile() as stderr:
            process = subprocess.Popen(argv, stdout=stdout, stderr=stderr, env=env)
            peak = None
            deadline = started + 50
            try:
                while True:
                    pid, status, usage = os.wait4(process.pid, os.WNOHANG)
                    if pid:
                        process.returncode = os.waitstatus_to_exitcode(status)
                        peak = usage.ru_maxrss / 1024 if sys.platform == "darwin" else usage.ru_maxrss
                        break
                    if time.monotonic() >= deadline:
                        process.kill()
                        _, status, usage = os.wait4(process.pid, 0)
                        process.returncode = os.waitstatus_to_exitcode(status)
                        raise AssertionError("CLI exceeded bounded 50s subprocess deadline")
                    time.sleep(0.025)
            except BaseException:
                if process.returncode is None:
                    process.kill()
                    process.wait()
                raise
            stdout.seek(0)
            stderr.seek(0)
            output = stdout.read()
            diagnostic = stderr.read()
        elapsed = round((time.monotonic() - started) * 1000, 2)
        if help_output:
            payload = None
            request_count = 0
        elif output:
            payload = json.loads(output)
            expected_keys = {"meta", "results", "pagination", "fetch_failures"}
            if payload.get("dry_run") is True:
                expected_keys |= {"dry_run", "action"}
                require(isinstance(payload.get("action"), str) and payload["action"],
                        "dry-run action must be a nonempty string")
            require(set(payload) == expected_keys,
                    "stdout must contain one complete stay envelope with dry-run markers only in dry-run")
            require(isinstance(payload["results"], list), "results must be an array")
            request_count = payload["meta"].get("upstream_requests", 0)
            require(isinstance(request_count, int) and request_count >= 0,
                    "request metric must be nonnegative integer")
            if request_count:
                require(payload["meta"].get("observed_at"), "live response lost observed_at")
                require(payload["meta"].get("timezone") == "Asia/Tokyo", "wrong source timezone")
                require(payload["meta"].get("coverage"), "live response lost coverage")
            require(b"<html" not in output.lower(), "raw source page leaked to stdout")
        else:
            payload = None
            request_count = 0 if expected == 2 else 2
        self.cli_requests += request_count
        metrics = {"args": arguments, "exit_code": process.returncode,
                   "stdout_bytes": len(output), "elapsed_ms": elapsed,
                   "peak_rss_kib": round(peak, 2) if peak is not None else None,
                   "upstream_requests": request_count}
        if payload:
            metrics.update({"cache_status": payload["meta"].get("cache_status"),
                            "observed_at": payload["meta"].get("observed_at"),
                            "result_count": len(payload["results"]),
                            "source_elapsed_ms": payload["meta"].get("elapsed_ms")})
        self.current["invocations"].append(metrics)
        require(process.returncode == expected,
                f"expected exit {expected}, got {process.returncode}: {diagnostic.decode(errors='replace')[:700]}")
        if expected == 0:
            require(not diagnostic, f"unexpected stderr: {diagnostic.decode(errors='replace')[:500]}")
        else:
            require(not output if expected != 8 else bool(output), "failure stdout contract violated")
            detail = json.loads(diagnostic)
            require(detail.get("exit_code") == expected and detail.get("error", {}).get("hint"),
                    "error lost structured code or actionable hint")
            self.current["error_code"] = detail["error"]["code"]
        if help_output:
            require(b"Usage:" in output, "help missing usage")
        return payload, metrics

    def source(self, url):
        if url in self.sources:
            return self.sources[url]
        require(self.witness_requests < MAX_WITNESS_REQUESTS, "independent source request budget exhausted")
        parsed = urllib.parse.urlsplit(url)
        require(parsed.scheme == "https" and parsed.hostname == "www.jalan.net",
                "source witness must use public HTTPS Jalan origin")
        request = urllib.request.Request(url, headers={
            "User-Agent": "jalan-pp-cli/1.0 (anonymous read-only accommodation research)",
            "Accept": "text/html", "Accept-Language": "ja",
        })
        self.witness_requests += 1
        with urllib.request.urlopen(request, timeout=20) as response:
            require(urllib.parse.urlsplit(response.geturl()).hostname == "www.jalan.net",
                    "independent source redirected outside Jalan")
            raw = response.read(MAX_SOURCE_BYTES + 1)
            require(len(raw) <= MAX_SOURCE_BYTES, "independent source exceeds 4MiB")
            charset = response.headers.get_content_charset()
            if (charset and charset.lower() in ("utf-8", "utf8")) or re.search(br'charset=["\']?utf-8', raw[:5000], re.I):
                document = raw.decode("utf-8")
            else:
                document = raw.decode("cp932", errors="replace")
        parser = SourceText()
        parser.feed(document)
        witness = {"text": compact_text(" ".join(parser.parts)), "bytes": len(raw),
                   "charge_quotes": parser.charge_quotes, "cards": parser.cards}
        self.sources[url] = witness
        self.current.setdefault("independent_sources", []).append({"url": url, "bytes": len(raw)})
        return witness

    def evidence(self, item, url, mandatory, source_witness=None):
        source = self.source(url) if source_witness is None else source_witness
        search_page = (urllib.parse.urlsplit(url).path == "/uw/uwp1400/uww1400.do"
                       or bool(source["cards"]))
        if search_page:
            property_id = str(item.get("id", ""))
            witnesses = source["cards"].get(property_id, [])
            require(witnesses, f"search result property card absent for {property_id}")
        else:
            witnesses = [source["text"]]
        entries = [*item.get("evidence", []), *item.get("baths", {}).get("evidence", []),
                   *item.get("price", {}).get("evidence", [])]
        last_error = None
        for witness in witnesses:
            try:
                checked = []
                for name in mandatory:
                    value = item.get(name)
                    require(value and compact_text(value) in witness,
                            f"{name} not corroborated by source")
                    checked.append(name)
                matched = [e.get("field") for e in entries
                           if e.get("text") and compact_text(e["text"]) in witness]
                require(matched, "no source evidence corroborated independently")
                price = item.get("price", {})
                if price.get("amount") is not None:
                    require(str(price["amount"]) in witness or f'{price["amount"]:,}' in witness,
                            "base amount not present in independent source")
                    require(price.get("basis") not in (None, "", "unknown"),
                            "observed amount lost basis")
            except AssertionError as error:
                last_error = error
                continue
            self.current.setdefault("asserted_fields", []).extend(sorted(set(checked + matched)))
            if search_page:
                self.current.setdefault("asserted_search_card_ids", []).append(property_id)
            return witness
        raise last_error

    def snapshot_evidence(self, item, response):
        meta = response["meta"]
        require(meta.get("cache_status") == "live" and meta.get("upstream_requests", 0) > 0,
                "paging snapshot requires a fresh CLI source observation")
        url = meta.get("source_url")
        require(isinstance(url, str) and url and meta.get("source_urls") == [url],
                "paging snapshot requires one exact source URL")
        parsed = urllib.parse.urlsplit(url)
        require(parsed.scheme == "https" and parsed.hostname == "www.jalan.net",
                "paging snapshot source must be public HTTPS Jalan")
        observed_at = meta.get("observed_at")
        require(isinstance(observed_at, str) and observed_at,
                "paging snapshot lost original observation time")
        digest = hashlib.sha256(url.encode("utf-8")).hexdigest()
        cache_path = pathlib.Path(self.home) / "cache" / "observations" / f"v1-{digest}.json"
        require(cache_path.is_file(), "fresh paging snapshot cache entry missing")
        require(cache_path.stat().st_size <= MAX_SNAPSHOT_FILE_BYTES,
                "paging snapshot cache file exceeds 16MiB bound")
        with cache_path.open("rb") as cache_file:
            raw = cache_file.read(MAX_SNAPSHOT_FILE_BYTES + 1)
        require(len(raw) <= MAX_SNAPSHOT_FILE_BYTES,
                "paging snapshot cache file exceeds 16MiB bound")
        entry = json.loads(raw)
        require(isinstance(entry, dict) and entry.get("version") == 1,
                "paging snapshot cache version changed")
        require(entry.get("url") == url, "paging snapshot URL differs from cold response")
        require(entry.get("observed_at") == observed_at,
                "paging snapshot observation time differs from cold response")
        document = entry.get("body")
        require(isinstance(document, str), "paging snapshot has no decoded HTML body")
        body = document.encode("utf-8")
        require(len(body) <= MAX_SNAPSHOT_BODY_BYTES,
                "paging snapshot decoded body exceeds 16MiB bound")
        parser = SourceText()
        parser.feed(document)
        witness = {"text": compact_text(" ".join(parser.parts)),
                   "cards": parser.cards, "charge_quotes": parser.charge_quotes,
                   "bytes": len(body)}
        self.current["source_snapshot_witness"] = {
            "url": url, "observed_at": observed_at, "sha256": hashlib.sha256(body).hexdigest(),
            "bytes": len(body), "kind": "fresh_cli_http_observation_independent_parser",
        }
        return self.evidence(item, url, ("name_ja",), source_witness=witness)

    def case(self, name, callback):
        self.current = {"name": name, "status": "running", "invocations": []}
        self.rows.append(self.current)
        try:
            callback()
            self.current["status"] = "pass"
        except Exception as error:
            self.current["status"] = "fail"
            self.current["error"] = str(error)[:1200]
        self.current = None

    def query_url(self, payload, expected):
        urls = payload["meta"].get("source_urls", [])
        require(urls, "missing exact source URLs")
        for url in urls:
            values = urllib.parse.parse_qs(urllib.parse.urlsplit(url).query)
            for key, value in expected.items():
                require(values.get(key) == [str(value)], f"source parameter {key} was not preserved")
        self.current["asserted_source_parameters"] = expected

    def search(self, extras=()):
        return self.invoke(["search", "--destination", "Hakone", "--check-in", self.args.check_in,
                            "--adults", "2", "--limit", "5", *extras])

    def property_case(self, property_id, label):
        payload, _ = self.invoke(["property", property_id])
        item = payload["results"][0]
        require(item["id"] == property_id, "property identity changed")
        require(item.get("address") and item.get("access"), "property missing address or access")
        witness = self.evidence(item, payload["meta"]["source_url"], ("name_ja", "address", "access"))
        if property_id == "371898":
            require(item["baths"]["hot_spring"] is True and item["room_baths"]["hot_spring"] is False,
                    "property hot spring incorrectly inferred into room hot-spring water")
            require("客室露天は温泉ではございません" in witness, "negative room onsen source qualification lost")
            categories = ("総合", "部屋", "風呂", "料理（朝食）", "料理（夕食）", "接客・サービス", "清潔感")
            require(all(label in item["review_categories"] for label in categories), "Japanese review category set incomplete")
            require(all(compact_text(label) in witness for label in categories), "review labels not corroborated by source")
        self.current["lodging_fixture"] = label
        self.current["property_id"] = property_id

    def plan_case(self, plan_id, room_id, extras=()):
        payload, _ = self.invoke(["plan", "385995", "--plan-id", plan_id, "--room-id", room_id,
                                  "--check-in", self.args.check_in, "--adults", "2", *extras])
        item = payload["results"][0]
        require(item["plan_id"] == plan_id and item["room_id"] == room_id, "exact room/plan pair changed")
        self.evidence(item, payload["meta"]["source_url"], ("plan_name", "room_name"))
        charges = self.source(payload["meta"]["source_url"])["charge_quotes"]
        require(charges, "exact source charge block missing")
        quote = re.search(r"([0-9][0-9,]*)円", charges[0])
        require(quote and int(quote.group(1).replace(",", "")) == item["price"]["amount"],
                "base amount differs from the displayed exact charge block")
        require(item.get("cancellation") and item.get("fees"), "plan terms missing cancellation or fees")
        require(item["price"].get("final_payable") is None, "unresolved extras incorrectly computed as payable")
        require(item["price"].get("points") is not None, "points distinction missing")
        self.current["price_basis"] = item["price"].get("basis")
        self.current["availability"] = item.get("availability")
        return payload

    def run(self):
        for command in ("search", "locations", "property", "offers", "plan", "compare", "capabilities"):
            self.case(f"help:{command}", lambda command=command: self.invoke([command, "--help"], help_output=True))
            def dry(command=command):
                payload, _ = self.invoke([command, "--dry-run", "--agent"])
                require(payload["meta"].get("dry_run") is True and payload["meta"].get("upstream_requests") == 0,
                        "dry-run performed source work")
            self.case(f"dry-run:{command}", dry)
        self.case("catalogue:locations", lambda: self.invoke(["locations", "--query", "Hakone"]))
        self.case("catalogue:capabilities", lambda: self.invoke(["capabilities", "--agent"]))
        self.case("property:regional-ryokan", lambda: self.property_case("385995", "regional ryokan"))
        self.case("property:onsen", lambda: self.property_case("371898", "onsen property"))

        def urban():
            payload, _ = self.invoke(["search", "--area-code", "136200", "--check-in", self.args.check_in,
                                      "--lodging-type", "hotel", "--adults", "2", "--limit", "3"])
            require(payload["results"], "urban hotel search has no observed fixture")
            item = payload["results"][0]
            self.saved["urban_id"] = item["id"]
            self.query_url(payload, {"lrgCd": "136200", "yadHb": "1"})
            self.property_case(item["id"], "urban hotel discovered from source area 136200")
        self.case("property:urban-hotel", urban)

        def paging():
            first, fm = self.search(["--max-age", "5m", "--refresh"])
            second, sm = self.search(["--page", "2", "--max-age", "5m"])
            full, wm = self.search(["--limit", "10", "--max-age", "5m"])
            ids = lambda p: [v["id"] for v in p["results"]]
            self.current["paging_ids"] = {"first": ids(first), "second": ids(second), "full": ids(full)}
            self.current["paging_observation_times"] = [p["meta"]["observed_at"] for p in (first, second, full)]
            self.current["paging_scope"] = "same cached source observation; fresh source order is non-atomic"
            require(fm["upstream_requests"] >= 1 and sm["upstream_requests"] == 0 and wm["upstream_requests"] == 0,
                    "paging fixture did not reuse one exact source observation")
            require(first["meta"]["observed_at"] == second["meta"]["observed_at"] == full["meta"]["observed_at"],
                    "paging cache lost its original observation time")
            require(ids(first) + ids(second) == ids(full), "logical paging skipped or duplicated source results")
            require(len(full["results"]) == 10, "paging fixture requires at least ten current matches")
            require(first["pagination"].get("source_page_size") == 30, "source page size not explicit")
            self.snapshot_evidence(first["results"][0], first)
        self.case("search:logical-paging", paging)

        def filtered():
            payload, _ = self.search(["--meals", "breakfast_dinner", "--lodging-type", "ryokan", "--onsen",
                                      "--outdoor-bath", "--private-bath", "--room-outdoor-bath", "--non-smoking"])
            self.query_url(payload, {"mealType":"3", "yadRk":"1", "careOnsen":"1", "careOpenbath":"1",
                                     "careBathRent":"1", "carePribateBath":"1", "careNsmr":"1"})
            require(payload["meta"]["query"]["non_smoking"] is True, "filter lost in query metadata")
            if payload["results"]:
                self.evidence(payload["results"][0], payload["meta"]["source_urls"][0], ("name_ja",))
        self.case("search:filters", filtered)

        def offers():
            payload, _ = self.invoke(["offers", "385995", "--check-in", self.args.check_in,
                                      "--adults", "2", "--limit", "5", "--meals", "breakfast_dinner", "--non-smoking"])
            self.query_url(payload, {"mealType":"3", "careNsmr":"1", "roomCrack":"200000"})
            require(payload["results"], "offers fixture has no current plans")
            pairs = [(v["plan_id"], v["room_id"]) for v in payload["results"]]
            require(len(pairs) == len(set(pairs)), "exact duplicate room/plan pair leaked")
            require(all(item["meals"] == "breakfast_dinner" and item["smoking"] == "non_smoking"
                        for item in payload["results"]), "returned offer does not satisfy requested meal/smoking filters")
            self.evidence(payload["results"][0], payload["meta"]["source_url"], ("plan_name",))
        self.case("offers:exact-pairs-and-filters", offers)
        self.case("plan:base-quote-and-room-facts", lambda: self.plan_case("03912759", "0576806"))

        def family():
            payload = self.plan_case("03806855", "0546600", ["--rooms", "2", "--children-elementary", "1"])
            self.query_url(payload, {"roomCount":"2", "adultNum":"2", "child1Num":"1", "roomCrack":"210000,210000"})
            require(payload["meta"]["query"]["stay"]["children_per_room"] == [1,0,0,0,0], "child categories altered")
        self.case("plan:family-two-room-occupancy", family)

        def unavailable():
            payload, _ = self.invoke(["plan", "385995", "--plan-id", "03912759", "--room-id", "0576806",
                                      "--check-in", self.args.fallback_date, "--adults", "2"])
            item = payload["results"][0]
            witness = self.source(payload["meta"]["source_url"])["text"]
            require("日付未定" in witness, "chosen live fallback fixture changed; inspect source before updating date")
            require(item["availability"] == "unavailable" and item["price"]["amount"] is None,
                    "reference quote incorrectly became a dated available cash quote")
            require(payload["meta"]["status"] == "unavailable", "unavailable plan status lost")
        self.case("plan:undated-reference-fallback", unavailable)

        def no_matches():
            payload, _ = self.invoke(["offers", "385995", "--check-in", self.args.empty_date, "--adults", "2"])
            witness = self.source(payload["meta"]["source_url"])["text"]
            require(any(term in witness for term in ("ご希望の条件に該当するプランが", "予約受付を停止", "予約受付停止", "該当するプランが", "プランがありません")),
                    "empty source fixture changed; inspect source before updating date")
            require(payload["results"] == [] and payload["meta"]["status"] == "no_matches", "no-match became sold-out or empty unlabelled success")
        self.case("offers:no-matches-not-sold-out", no_matches)

        def compare_fields(arguments, mode):
            arguments = ["compare", "385995", *arguments, "--adults", "2", "--max-age", "5m"]
            cold, cm = self.invoke(arguments + ["--refresh"])
            warm, wm = self.invoke(arguments)
            fields = "check_in,results.property_id,results.plan_id,results.room_id,results.price"
            selected, sm = self.invoke(arguments + ["--agent", "--select", fields])
            require(cold["meta"].get("comparison_mode") == mode and len(cold["results"]) == 2,
                    "comparison lost alternatives")
            require("not exhaustive" in cold["meta"]["coverage"], "bounded comparison overclaims coverage")
            require(cm["upstream_requests"] >= 2 and wm["upstream_requests"] == sm["upstream_requests"] == 0,
                    "comparison cache did not preserve both source observations")
            require(cold["meta"]["observed_at"] == warm["meta"]["observed_at"] == selected["meta"]["observed_at"],
                    "comparison selection changed source observation time")
            require(selected["meta"]["query"] == cold["meta"]["query"], "comparison selection lost query")
            require(selected["pagination"] == warm["pagination"] and selected["fetch_failures"] == warm["fetch_failures"],
                    "comparison selection dropped outer coverage or failures")
            require(len(selected["results"]) == len(warm["results"]), "selection changed alternative count")
            for original, projected in zip(warm["results"], selected["results"]):
                for key in ("alternative_index", "check_in", "query", "status", "pagination", "source_url"):
                    require(projected.get(key) == original[key], f"comparison selection lost {key}")
                require([(x["url"], x["observed_at"]) for x in projected["observations"]] ==
                        [(x["url"], x["observed_at"]) for x in original["observations"]],
                        "selection lost per-alternative source freshness")
                require(original["results"], "live comparison fixture has no offers to select")
                require(len(original["results"]) == len(projected["results"]), "selection changed offer count")
                for offer, selected_offer in zip(original["results"], projected["results"]):
                    require(selected_offer == {key: offer[key] for key in ("property_id", "plan_id", "room_id", "price")},
                            "nested selection changed identities, price facts or offer order")
            require(sm["stdout_bytes"] < wm["stdout_bytes"], "comparison selection did not reduce output")
            self.current["efficiency"] = {"cold": cm, "warm": wm, "selected": sm,
                                          "selection_byte_reduction": wm["stdout_bytes"]-sm["stdout_bytes"],
                                          "cache_request_reduction": cm["upstream_requests"]-wm["upstream_requests"]}
        next_day = (dt.date.fromisoformat(self.args.check_in) + dt.timedelta(days=1)).isoformat()
        self.case("compare:equal-party-dates", lambda: compare_fields(
            ["--dates", self.args.check_in+","+next_day, "--limit", "2"], "dates"))
        self.case("compare:exact-plans", lambda: compare_fields(
            ["--check-in", self.args.check_in, "--plans", "03912759:0576806,03806855:0546600"], "plans"))

        def efficiency():
            args = ["property", "385995", "--max-age", "5m"]
            cold, cm = self.invoke(args + ["--refresh"])
            warm, wm = self.invoke(args)
            selected, sm = self.invoke(args + ["--agent", "--select", "id,name_ja,baths.in_room,price.amount"])
            require(cm["upstream_requests"] >= 1 and wm["upstream_requests"] == 0 and sm["upstream_requests"] == 0,
                    "exact warm cache did not eliminate upstream requests")
            require(cold["meta"]["observed_at"] == warm["meta"]["observed_at"] == selected["meta"]["observed_at"],
                    "cache hit rewrote observation time")
            require(selected["meta"]["query"] == cold["meta"]["query"], "selection dropped query context")
            require(set(selected["results"][0]) == {"id","name_ja","baths","price"}, "item projection lost or added fields")
            require(sm["stdout_bytes"] < wm["stdout_bytes"], "selection did not reduce output bytes")
            self.current["efficiency"] = {"cold": cm, "warm": wm, "selected": sm,
                                          "selection_byte_reduction": wm["stdout_bytes"]-sm["stdout_bytes"],
                                          "cache_request_reduction": cm["upstream_requests"]-wm["upstream_requests"]}
        self.case("efficiency:cold-warm-and-select", efficiency)
        for name,args in (
            ("unknown-destination", ["search","--destination","UnlistedFakeTown","--check-in",self.args.check_in]),
            ("invalid-calendar-date", ["search","--destination","Hakone","--check-in","2026-02-30"]),
            ("missing-plan-id", ["plan","385995","--check-in",self.args.check_in]),
            ("unknown-flag", ["property","385995","--not-a-real-flag"]),
            ("explicit-zero-party", ["search","--destination","Hakone","--check-in",self.args.check_in,"--adults","0"]),
            ("local-inventory-unsupported", ["property","385995","--data-source","local"]),
            ("nine-plus-adults-unsupported", ["search","--destination","Hakone","--check-in",self.args.check_in,"--adults","9"]),
            ("child-category-out-of-range", ["search","--destination","Hakone","--check-in",self.args.check_in,"--children-elementary","6"]),
        ):
            self.case("error:"+name, lambda args=args: self.invoke(args, expected=2))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cli", type=pathlib.Path, default=pathlib.Path("bin/jalan-pp-cli"))
    parser.add_argument("--output", type=pathlib.Path, required=True)
    parser.add_argument("--check-in", default="2026-11-10")
    parser.add_argument("--fallback-date", default="2026-12-31")
    parser.add_argument("--empty-date", default="2027-09-20")
    args = parser.parse_args()
    args.cli = args.cli.resolve()
    require(args.cli.is_file(), "build the CLI before running the live matrix")
    started = dt.datetime.now(dt.timezone.utc).isoformat()
    with tempfile.TemporaryDirectory(prefix="jalan-live-proof-") as home:
        runner = Runner(args, home)
        runner.run()
        passed = sum(row["status"] == "pass" for row in runner.rows)
        failed = len(runner.rows) - passed
        proof = {"schema_version":1,"started_at":started,"finished_at":dt.datetime.now(dt.timezone.utc).isoformat(),
                 "verdict":"PASS" if not failed else "FAIL","passed":passed,"failed":failed,
                 "read_only":True,"raw_pages_persisted":False,"fixture_dates":{"check_in":args.check_in,"fallback":args.fallback_date,"empty":args.empty_date},
                 "bounds":{"cli_upstream_requests_max":MAX_CLI_REQUESTS,"independent_source_requests_max":MAX_WITNESS_REQUESTS,"commands_max":MAX_COMMANDS,"subprocess_timeout_seconds":50,"witness_body_bytes_max":MAX_SOURCE_BYTES},
                 "observed":{"cli_upstream_requests":runner.cli_requests,"independent_source_requests":runner.witness_requests,"commands":runner.calls},
                 "fixtures":{"regional_ryokan":"385995","onsen":"371898","urban_hotel":runner.saved.get("urban_id")},
                 "rows":runner.rows}
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(json.dumps(proof, ensure_ascii=False, indent=2, allow_nan=False)+"\n")
        print(json.dumps({"verdict":proof["verdict"],"passed":passed,"failed":failed,"proof":str(args.output)}, separators=(",",":")))
        return 0 if not failed else 1


if __name__ == "__main__":
    sys.exit(main())
