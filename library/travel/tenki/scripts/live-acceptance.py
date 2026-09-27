#!/usr/bin/env python3
"""Bounded serial acceptance against a final tenki-pp-cli binary.

Use --binary and a new/empty workspace --output-dir. Every case captures stdout,
stderr, command, timing, assertions and actual source snapshot. Cache and CLI
home are isolated under that directory. --self-test makes no network requests.
"""
import argparse
import datetime as dt
import json
import math
import os
from pathlib import Path
import signal
import subprocess
import sys
import time
from urllib.parse import urlparse

JST = dt.timezone(dt.timedelta(hours=9), "Asia/Tokyo")
TOKYO = "https://tenki.jp/forecast/3/16/4410/13101/"
SAPPORO = "https://tenki.jp/forecast/1/2/1400/1100/"
KYOTO = "https://tenki.jp/forecast/6/29/6110/26103/"
NAHA = "https://tenki.jp/forecast/10/50/9110/47201/"
KINKAKU = "https://tenki.jp/leisure/6/29/189/7327/"
FUJI = "https://tenki.jp/mountain/famous100/5/25/150.html"
MURODO = "https://tenki.jp/kouyou/4/19/30314.html"
UENO = "https://tenki.jp/sakura/3/16/54401.html"


def workspace_root():
    script = Path(__file__).resolve()
    for parent in script.parents:
        if parent.name == ".press":
            return parent.parent
    return script.parents[1]


def require(condition, message):
    if not condition:
        raise ValueError(message)


def timestamp(value):
    require(isinstance(value, str) and "T" in value, f"missing RFC3339 timestamp: {value!r}")
    parsed = dt.datetime.fromisoformat(value)
    require(parsed.utcoffset() == dt.timedelta(hours=9), f"timestamp lacks JST offset: {value}")
    return parsed


def source_checks(source, issue=False):
    require(isinstance(source, dict), "source metadata missing")
    url = urlparse(source.get("url", ""))
    require(url.scheme == "https" and url.netloc == "tenki.jp", "source URL is not canonical tenki.jp")
    require(source.get("timezone") == "Asia/Tokyo", "source timezone missing")
    fetched, expiry = timestamp(source.get("fetched_at")), timestamp(source.get("expires_at"))
    require(expiry >= fetched, "cache expiry precedes fetch")
    require(source.get("freshness") in ("fresh", "stale", "unknown"), "freshness status missing")
    if source.get("issue_at"):
        timestamp(source["issue_at"])
    if issue:
        require(bool(source.get("issue_at")), "weather/publication issue time missing")
        require(source.get("freshness") == "fresh" and not source.get("source_stale")
                and not source.get("cache_stale"), "current source is stale or freshness unknown")


def nullable_number(row, key):
    require(key in row, f"numeric field {key} omitted rather than null")
    value = row[key]
    require(value is None or (type(value) in (int, float) and math.isfinite(value)), f"invalid numeric {key}: {value!r}")


def daily_checks(result, today, place):
    require(result.get("status") == "ok", "daily source is unavailable")
    require(result["place"]["url"] == place, "daily place identity differs from selected URL")
    source_checks(result["source"], issue=True)
    periods = result.get("periods")
    require(isinstance(periods, list) and periods, "daily periods empty/null")
    dates = [dt.date.fromisoformat(p["date"]) for p in periods]
    require(dates == sorted(set(dates)), "daily dates duplicate or unordered")
    require(all((b-a).days == 1 for a,b in zip(dates, dates[1:])), "daily date horizon has gaps")
    issue_day = timestamp(result["source"]["issue_at"]).date()
    # Use actual coverage rather than assuming the issue's calendar day always
    # matches the first source date during a midnight product rollout.
    first_source = timestamp(result.get("coverage_start")).date()
    end_source = timestamp(result.get("coverage_end")).date()
    require((end_source-first_source).days == 14, "source daily coverage is not fourteen calendar dates")
    expected = [first_source + dt.timedelta(days=i) for i in range(14)
                if today <= first_source + dt.timedelta(days=i) < today + dt.timedelta(days=14)]
    require(dates == expected, f"daily source horizon mismatch: returned {dates}, expected intersection {expected}")
    units = result.get("units", {})
    require(units.get("temperature") == "°C" and units.get("wind_speed") == "m/s"
            and units.get("timezone") == "Asia/Tokyo" and "mm" in units.get("precipitation", ""), "daily units missing")
    require(any(p.get("weather") for p in periods), "daily weather content empty")
    require(any(p.get("precip_probability_pct") is not None for p in periods), "all daily probabilities missing")
    for row in periods:
        start, end = timestamp(row.get("start")), timestamp(row.get("end"))
        require(end-start == dt.timedelta(days=1) and start.hour == 0, "daily calendar bounds changed")
        for key in ("min_temperature_c", "max_temperature_c", "precip_probability_pct", "precip_amount_mm", "wind_speed_m_s"):
            nullable_number(row, key)
        require(row["wind_speed_m_s"] is None, "daily wind fabricated")
        if row["date"] == issue_day.isoformat():
            require(row.get("partial") and row.get("kind") == "mixed", "day zero lacks mixed/partial provenance")
            require(row.get("weather_probability_from") == result["source"]["issue_at"], "today weather/probability onset missing")
            require(row.get("temperature_kind") in ("forecast_or_estimated_actual", "estimated_actual"), "today min/max falsely all forecast")
    return {"dates": [p["date"] for p in periods], "place": result["place"], "source": result["source"],
            "coverage_start": result.get("coverage_start"), "coverage_end": result.get("coverage_end")}


def hourly_checks(result, tomorrow):
    require(result.get("status") == "ok", "hourly source unavailable")
    source_checks(result["source"], issue=True)
    rows = result.get("periods", [])
    require(len(rows) == 9, "09–17 hourly display must include nine instantaneous endpoints")
    require([timestamp(p["valid_at"]).hour for p in rows] == list(range(9,18)), "hourly endpoint alignment incorrect")
    for row in rows:
        start,end,instant = timestamp(row["start"]),timestamp(row["end"]),timestamp(row["valid_at"])
        require(row["date"] == tomorrow and row["kind"] == "forecast", "next-day hour is not a forecast for requested date")
        require(end-start == dt.timedelta(hours=1) and instant == end, "preceding-hour interval/instant conflated")
        for key in ("temperature_c", "precip_probability_pct", "precip_rate_mm_h", "wind_speed_m_s"):
            nullable_number(row,key)
    require("mm/h" in result.get("units",{}).get("precipitation",""), "hourly rain units missing")
    return {"endpoints": [p["valid_at"] for p in rows], "source": result["source"]}


def comparison_checks(result, expected_cells, hourly=False):
    comparison = result.get("comparison", {})
    cells = comparison.get("cells", [])
    require(len(cells) == expected_cells and comparison.get("returned_cells") == expected_cells, "comparison cells missing")
    require(not result.get("fetch_failures"), "comparison source fetch failures")
    for cell in cells:
        source_checks(cell["weather_source"], issue=True)
        require(cell.get("source_status") == "ok", "comparison weather unavailable")
        require(cell.get("status") in ("meets_criteria", "fails_criteria"), "available future weather was left indeterminate")
        failed = []
        for key, criterion in cell.get("criteria", {}).items():
            require(criterion.get("unit") and criterion.get("reduction"), "criterion units/reduction missing")
            expected = 8 if hourly and key in ("max-pop","max-rain") else 9 if hourly else 1
            evidence = criterion.get("evidence", [])
            coverage = criterion.get("coverage", {})
            require(len(evidence) == expected and coverage.get("expected") == expected
                    and coverage.get("available") == expected, f"{key} evidence coverage incorrect")
            values = [item.get("value") for item in evidence]
            require(all(type(v) in (int,float) and math.isfinite(v) for v in values), "criterion evidence lacks actual numeric values")
            require(all(item.get("kind") == "forecast" for item in evidence), "estimated/mixed value earned future recommendation")
            reduced = min(values) if criterion["operator"] == ">=" else max(values)
            require(criterion.get("value") == reduced, "criterion reduction differs from source evidence")
            does_fail = reduced < criterion["threshold"] if criterion["operator"] == ">=" else reduced > criterion["threshold"]
            require(criterion.get("verdict") == ("fail" if does_fail else "pass"), "criterion verdict disagrees with explicit threshold")
            failed.append(does_fail)
            if hourly:
                instants = [timestamp(item["end"] if key in ("max-pop","max-rain") else item["valid_at"]).hour for item in evidence]
                require(instants == list(range(10,18)) if key in ("max-pop","max-rain") else instants == list(range(9,18)), "rain interval/instant endpoints incorrectly selected")
        require(cell["status"] == ("fails_criteria" if any(failed) else "meets_criteria"), "cell status disagrees with actual criterion evidence")
    return {"cells": cells, "rain_unit": comparison.get("rain_unit"), "hours": comparison.get("requested_hours")}


def validate(kind, result, context):
    today,tomorrow,year = context["today"],context["tomorrow"],context["year"]
    if kind.startswith("daily:"):
        return daily_checks(result,today,kind.split(":",1)[1])
    if kind == "hourly":
        return hourly_checks(result,tomorrow)
    if kind in ("compare-daily","compare-hourly"):
        return comparison_checks(result,4 if kind == "compare-daily" else 1,kind == "compare-hourly")
    if kind == "leisure":
        place = result.get("place",{})
        require("金閣寺" in place.get("name","") and place.get("url") == KINKAKU, "Kinkaku identity missing")
        require(place.get("forecast_reference_url") == "https://tenki.jp/forecast/6/29/6110/26101/"
                and place.get("forecast_reference_name") == "京都市北区", "leisure linked municipality missing")
        source_checks(result["source"],issue=True)
    elif kind == "leisure-search":
        places = result.get("places",[])
        require(result.get("status") == "ok" and 1 <= len(places) <= 5, "bounded leisure search empty/unbounded")
        require(all("金閣寺" in p["name"] for p in places), "leisure search returned unrelated place")
        require(any(p["url"] == KINKAKU for p in places), "verified destination missing from directory search")
        require(result.get("search_scope") == "selected_leisure_directory" and result.get("scanned",0) > 0
                and result.get("pages") == 1, "leisure directory scope missing")
        source_checks(result["source"])
    elif kind == "mountain":
        mountain = result.get("mountain",{})
        place,model = mountain.get("place",{}),mountain.get("model_source",{})
        require(place.get("name") == "富士山" and place.get("scope") == "foothill" and place.get("elevation_m") == 3776, "Fuji identity/altitude missing")
        require(place.get("forecast_reference_name") == "富士宮市" and place.get("forecast_reference_url")
                == "https://tenki.jp/forecast/5/25/5030/22207/", "Fuji foothill reference wrong")
        require(mountain.get("model_kind") == "nearby_model_guidance" and mountain.get("summit_forecast_available") is False, "numerical guidance promoted to summit forecast")
        initial = mountain.get("model_initial_at")
        timestamp(initial)
        require(mountain.get("model_initial_raw") and not model.get("issue_at")
                and model.get("freshness_reference") == "model_initial_at" and model.get("freshness_at") == initial, "initialization laundered into publication issue")
        levels = mountain.get("levels",[])
        require(8 <= len(levels) <= 72 and min(p["elevation_m"] for p in levels) == 300
                and max(p["elevation_m"] for p in levels) == 4400, "actual altitude model levels missing")
        for row in levels:
            require(timestamp(row["valid_at"]).hour in (9,15), "model valid hour changed")
            nullable_number(row,"temperature_c");nullable_number(row,"wind_speed_m_s")
        source_checks(model)
    elif kind in ("kouyou","sakura-ended","sakura-next-year"):
        source_checks(result["source"],issue=kind == "kouyou")
        require(result.get("requested_year") == year+(kind == "sakura-next-year"), "requested seasonal year missing")
        if kind == "kouyou":
            spot = result.get("spot",{})
            require(result.get("status") == "ok" and result.get("update_state") == "active" and result.get("year") == year, "current foliage unavailable")
            require(spot.get("condition") and spot.get("report_date") and spot.get("normal_period"), "foliage report/normal period missing")
            require(not spot.get("report_at") and spot.get("report_raw"), "day-only foliage report fabricated precise instant")
            require(result["place"].get("forecast_reference_url") == "https://tenki.jp/forecast/4/19/5510/16323/", "foliage weather reference wrong")
        else:
            require(result.get("status") == ("season_ended" if kind == "sakura-ended" else "year_unavailable")
                    and result.get("update_state") == "ended", "ended/unavailable Sakura status wrong")
            require(result.get("year") == year and not result["source"].get("issue_at"), "retained Sakura facts refreshed by weather timestamp")
            require(result.get("spot",{}).get("normal_period"), "retained Sakura typical period missing")
    elif kind == "kouyou-search":
        spots = result.get("spots",[])
        require(result.get("status") == "ok" and 1 <= len(spots) <= 5 and result.get("pages") == 1, "foliage bounded search empty/unbounded")
        require(all("立山" in spot["place"]["name"] for spot in spots), "foliage search returned unrelated spots")
        require(any(spot["place"]["url"] == MURODO for spot in spots), "Murodo missing from verified venue search")
        source_checks(result["source"])
    elif kind == "unsupported":
        require(result.get("status") == "out_of_horizon" and result.get("periods") == [], "unsupported date produced invented forecast rows")
        require(result.get("requested_from") == context["unsupported"], "unsupported request date lost")
        require(timestamp(result["coverage_end"]).date() < dt.date.fromisoformat(context["unsupported"]), "source horizon falsely expanded")
        source_checks(result["source"],issue=True)
    elif kind in ("compare-season","compare-mountain"):
        cells = result.get("comparison",{}).get("cells",[])
        require(len(cells) == 1 and cells[0].get("status") == "insufficient_data", "unsupported destination/season earned recommendation")
        cell = cells[0]
        require(not result.get("fetch_failures"), "comparison hides source acquisition failure")
        source_checks(cell["weather_source"],issue=True)
        if kind == "compare-season":
            season = cell.get("seasonal",{})
            require(season.get("verdict") == "unknown" and season.get("reasons"), "current seasonal report certified tomorrow")
        else:
            require(cell["place"].get("kind") == "mountain" and any("level mismatch" in reason for reason in cell.get("reasons",[])), "mountain scope mismatch not explained")
    else:
        raise ValueError(f"unknown acceptance kind {kind}")
    return result


def cases(context):
    tomorrow,year = context["tomorrow"],str(context["year"])
    daily = [("daily_"+name,["forecast","daily","--place",place,"--from",context["today"].isoformat(),"--days","14","--limit","14"],"daily:"+place)
             for name,place in (("chiyoda",TOKYO),("sapporo",SAPPORO),("kyoto_sakyo",KYOTO),("naha",NAHA))]
    generous = ["--from",tomorrow,"--max-pop","100","--min-temp","-100","--max-temp","100"]
    return daily + [
        ("kinkaku_place",["places","show","--place",KINKAKU],"leisure"),
        ("kyoto_leisure_directory",["places","search","--query","金閣寺","--kind","leisure","--directory","https://tenki.jp/leisure/6/29/","--limit","5","--max-scan-pages","1"],"leisure-search"),
        ("fuji_model",["mountain","show","--place",FUJI,"--limit","72"],"mountain"),
        ("murodo_current_foliage",["seasonal","show","--kind","kouyou","--place",MURODO,"--year",year],"kouyou"),
        ("foliage_venue_search",["seasonal","list","--kind","kouyou","--query","立山","--year",year,"--limit","5","--max-scan-pages","1"],"kouyou-search"),
        ("ueno_ended_sakura",["seasonal","show","--kind","sakura","--place",UENO,"--year",year],"sakura-ended"),
        ("ueno_next_year_unavailable",["seasonal","show","--kind","sakura","--place",UENO,"--year",str(context["year"]+1)],"sakura-next-year"),
        ("daily_out_of_horizon",["forecast","daily","--place",TOKYO,"--from",context["unsupported"],"--days","1"],"unsupported"),
        ("hourly_tomorrow_window",["forecast","hourly","--place",TOKYO,"--date",tomorrow,"--hours","09:00-17:00","--limit","9"],"hourly"),
        ("compare_daily_future",["compare","--place",TOKYO,"--place",SAPPORO,"--days","2",*generous],"compare-daily"),
        ("compare_hourly_endpoints",["compare","--place",TOKYO,"--days","1","--hours","09:00-17:00","--max-rain","1000","--max-wind","1000",*generous],"compare-hourly"),
        ("compare_future_season_unknown",["compare","--place",MURODO,"--days","1","--season","kouyou","--year",year,*generous],"compare-season"),
        ("compare_mountain_insufficient",["compare","--place",FUJI,"--days","1",*generous],"compare-mountain"),
    ]


def self_test():
    require(timestamp("2026-12-31T23:00:00+09:00").hour == 23,"JST parse failed")
    for invalid in (None,"2026-01-01","2026-01-01T00:00:00+00:00"):
        try: timestamp(invalid)
        except (ValueError,TypeError): pass
        else: raise ValueError("invalid timestamp accepted")
    nullable_number({"temperature_c":None},"temperature_c")
    nullable_number({"temperature_c":-2.5},"temperature_c")
    for invalid in (True,float("nan"),"20"):
        try: nullable_number({"temperature_c":invalid},"temperature_c")
        except ValueError: pass
        else: raise ValueError("invalid numeric accepted")
    context={"today":dt.date(2026,12,31),"tomorrow":"2027-01-01","year":2026,"unsupported":"2027-01-30"}
    require(len(cases(context)) == 17,"finite matrix count changed")
    require(cases(context)[10][1][-1] == "2027","next-year unavailable request wrong")
    print("PASS: local timestamp, nullable numeric, finite-case and year-rollover helpers; no CLI/network")


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary");parser.add_argument("--output-dir")
    parser.add_argument("--self-test",action="store_true")
    args=parser.parse_args()
    if args.self_test: self_test();return 0
    if not args.binary or not args.output_dir: parser.error("--binary and --output-dir are required")
    try:
        binary=Path(args.binary).expanduser().resolve();output=Path(args.output_dir).expanduser().resolve()
        require(binary.is_file() and os.access(binary,os.X_OK),"binary must be an existing executable")
        require(output.is_relative_to(workspace_root()),"output directory must stay inside workspace")
        require(not output.exists() or (output.is_dir() and not any(output.iterdir())),"choose a new/empty output directory; earlier proofs are preserved")
        output.mkdir(parents=True,exist_ok=True)
        with (output/".acceptance-run").open("x") as claim: claim.write("scripts/live-acceptance.py\n")
        cache,home=output/"cache",output/"cli-home";cache.mkdir();home.mkdir()
    except (OSError,ValueError) as exc: parser.error(str(exc))
    began=dt.datetime.now(JST);today=began.date()
    context={"today":today,"tomorrow":(today+dt.timedelta(days=1)).isoformat(),"year":today.year,
             "unsupported":(today+dt.timedelta(days=30)).isoformat()}
    records=[]
    selected=cases(context)
    for index,(name,arguments,kind) in enumerate(selected):
        command=[str(binary),*arguments,"--agent","--data-source","auto","--cache-dir",str(cache),"--home",str(home)]
        stdout,stderr=output/(name+".stdout.json"),output/(name+".stderr.txt")
        record={"name":name,"command":command,"stdout_file":stdout.name,"stderr_file":stderr.name,"status":"FAIL","failures":[]}
        start=time.monotonic()
        try:
            with stdout.open("wb") as out,stderr.open("wb") as err:
                process=subprocess.Popen(command,stdout=out,stderr=err,cwd=workspace_root(),start_new_session=True)
                try: code=process.wait(timeout=120)
                except subprocess.TimeoutExpired:
                    os.killpg(process.pid,signal.SIGKILL);process.wait();raise
            record["exit_code"]=code
            require(code == 0,f"command exited {code}; inspect captured stderr")
            payload=json.loads(stdout.read_bytes(),parse_constant=lambda v: (_ for _ in ()).throw(ValueError("invalid JSON constant "+v)))
            require(isinstance(payload,dict) and isinstance(payload.get("meta"),dict) and "results" in payload,"JSON meta/results envelope missing")
            require(payload["meta"].get("timezone") == "Asia/Tokyo","envelope timezone missing")
            metrics=payload["meta"].get("metrics",{})
            require(all(type(metrics.get(key)) is int and metrics[key] >= 0 for key in ("http_requests","cache_hits","response_bytes")),"measured HTTP/cache metrics missing")
            record["metrics"]={"http_requests":metrics["http_requests"],"cache_hits":metrics["cache_hits"],"decoded_response_bytes":metrics["response_bytes"]}
            record["snapshot"]=validate(kind,payload["results"],context)
            record["assertions"]="Typed content, availability, source time, units and product-specific evidence assertions passed"
            record["status"]="PASS"
        except (OSError,ValueError,KeyError,TypeError,subprocess.TimeoutExpired) as exc:
            record["failures"].append(str(exc))
        record["wall_seconds"]=round(time.monotonic()-start,6)
        record["stdout_bytes"]=stdout.stat().st_size if stdout.exists() else 0
        records.append(record)
        if index+1 < len(selected): time.sleep(1)  # Separate from measured process time.
    failures=[row["name"]+": "+failure for row in records for failure in row["failures"]]
    summary={"status":"PASS" if not failures else "FAIL","started_at":began.isoformat(),"finished_at":dt.datetime.now(JST).isoformat(),
             "timezone":"Asia/Tokyo","date_anchor":today.isoformat(),"next_date":context["tomorrow"],"binary":str(binary),
             "cache_dir":str(cache),"cli_home":str(home),"cases":records,"failures":failures,
             "limitations":"This acceptance snapshot assumes currently active foliage and ended current-year Sakura; source product/year/markup rollout changes are reported as explicit failures, not substituted facts."}
    (output/"summary.json").write_text(json.dumps(summary,ensure_ascii=False,indent=2)+"\n",encoding="utf-8")
    lines=[f"Live acceptance: **{summary['status']}**. JST anchor {today}; next day {context['tomorrow']}.","",
           "| Case | Result | Exit | Wall (s) | HTTP | Stdout bytes |","|---|---|---:|---:|---:|---:|"]
    lines += [f"| {row['name']} | {row['status']} | {row.get('exit_code','unknown')} | {row['wall_seconds']:.3f} | {row.get('metrics',{}).get('http_requests','unknown')} | {row['stdout_bytes']} |" for row in records]
    if failures: lines += ["","Failed assertions:",""]+["- "+failure for failure in failures]
    lines += ["","Full stdout/stderr and actual source snapshots are retained alongside summary.json. No fixed weather values were assumed."]
    (output/"summary.md").write_text("\n".join(lines)+"\n",encoding="utf-8")
    print(f"{summary['status']}: {len(records)} finite serial cases; {output/'summary.md'}")
    return 0 if not failures else 1


if __name__ == "__main__":
    raise SystemExit(main())
