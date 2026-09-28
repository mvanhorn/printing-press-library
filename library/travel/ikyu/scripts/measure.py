#!/usr/bin/env python3
"""Measure cold/warm anonymous command costs using real request counters and OS RSS."""
import argparse
import datetime as dt
import json
from pathlib import Path
import platform
import re
import subprocess
import tempfile
import time
from live_support import discover, save_report, selection, stay_args, stay_dates, timestamp


def measured(binary, args, cache):
    system=platform.system()
    timer=["/usr/bin/time", "-l" if system=="Darwin" else "-v"] if system in ("Darwin","Linux") else []
    started=time.perf_counter()
    p=subprocess.run([*timer,str(binary),*args,"--cache-dir",str(cache),"--timeout","45s"],capture_output=True,timeout=50)
    elapsed=time.perf_counter()-started
    if p.returncode:
        raise AssertionError(f"{' '.join(args[:4])}: exit {p.returncode}: {p.stderr.decode(errors='replace')[:600]}")
    if p.stdout.count(b"\n")!=1: raise AssertionError("JSON output is not one compact line")
    result=json.loads(p.stdout); stats=result["stats"]
    diagnostic=p.stderr.decode(errors="replace")
    if system=="Darwin":
        match=re.search(r"([0-9]+)\s+maximum resident set size",diagnostic); unit=1
    elif system=="Linux":
        match=re.search(r"Maximum resident set size \(kbytes\):\s*([0-9]+)",diagnostic); unit=1024
    else: match=None; unit=1
    if system in ("Darwin","Linux") and not match: raise AssertionError("OS peak RSS was not reported")
    rss=int(match.group(1))*unit if match else None
    return {"output_bytes":len(p.stdout),"requests":stats["requests"],"response_bytes_decoded":stats["response_bytes"],
            "cache_hits":stats["cache_hits"],"stale_hits":stats["stale_hits"],"wall_seconds":round(elapsed,6),
            "peak_rss_bytes":rss,"peak_rss_source":"/usr/bin/time -l" if system=="Darwin" else "/usr/bin/time -v" if system=="Linux" else None},result


def source_content(value):
    """Freshness changes from live to cache; compare the retained source facts."""
    if isinstance(value,dict): return {k:source_content(v) for k,v in value.items() if k != "freshness"}
    if isinstance(value,list): return [source_content(v) for v in value]
    return value


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument("--binary",type=Path,default=Path(__file__).resolve().parents[1]/"ikyu-pp-cli")
    p.add_argument("--check-in");p.add_argument("--check-out")
    p.add_argument("--hotel-offer");p.add_argument("--ryokan-offer")
    p.add_argument("--output",type=Path)
    args=p.parse_args(); dates=stay_dates(args.check_in,args.check_out); common=stay_args(*dates)
    report={"status":"running","measured_at":timestamp(),"stay":{"check_in":dates[0],"check_out":dates[1]},
            "method":"separate empty public cache per command; identical warm command immediately afterward; request counters include redirects and retries; decoded response bytes; OS peak RSS", "commands":[]}
    try:
        with tempfile.TemporaryDirectory(prefix="ikyu-measure-") as tmp:
            base=Path(tmp);setup=base/"setup"
            hotel=args.hotel_offer or discover(args.binary,"00000600",dates,setup)
            ryokan=args.ryokan_offer or discover(args.binary,"00002889",dates,setup)
            report["selectors"]={"hotel":hotel,"ryokan":ryokan}
            next_date=(dt.date.fromisoformat(dates[0])+dt.timedelta(days=1)).isoformat()
            nights=(dt.date.fromisoformat(dates[1])-dt.date.fromisoformat(dates[0])).days
            commands=[
                ("destinations",["stay","destinations","tokyo","--limit","3"]),
                ("search",["stay","search","--destination","tokyo",*common,"--limit","3"]),
                ("property",["stay","property",selection(ryokan)[0]]),
                ("rooms",["stay","rooms",selection(ryokan)[0],*common,"--limit","2","--plan-limit","3"]),
                ("offer",["stay","offer",*selection(ryokan),*common]),
                ("compare",["stay","compare","--offers",f"{hotel},{ryokan}",*common]),
                ("dates",["stay","dates",*selection(ryokan),"--check-ins",f"{dates[0]},{next_date}","--nights",str(nights)]),
            ]
            for name,command in commands:
                cache=base/name
                cold,cold_data=measured(args.binary,command,cache)
                warm,warm_data=measured(args.binary,command,cache)
                assert cold["requests"]>0 and cold["requests"]<=20, "cold requests absent or budget exceeded"
                assert warm["requests"]==0 and warm["response_bytes_decoded"]==0, "warm <=5min command unexpectedly requested source data"
                assert warm["cache_hits"]>0 and warm["stale_hits"]==0, "warm response not fresh cache"
                assert cold["wall_seconds"]+warm["wall_seconds"]<300, "warm measurement exceeded availability TTL"
                assert source_content(cold_data["data"])==source_content(warm_data["data"]), "cold/warm source content changed"
                if name in ("search","rooms"): assert len(cold_data["data"])<=3
                report["commands"].append({"command":name,"arguments":command,"status":"pass","cold":cold,"warm":warm})
            # Field projection is measured both cold and warm on a separate empty cache.
            field_args=["stay","offer",*selection(ryokan),*common,"--fields","data.property.id,data.plan.id,data.offer.price.instant_points_payable,stats"]
            cache=base/"offer_projection"
            cold,data=measured(args.binary,field_args,cache);warm,warm_data=measured(args.binary,field_args,cache)
            assert cold["requests"]==1 and warm["requests"]==0
            assert data["data"]==warm_data["data"] and set(data["data"])=={"property","plan","offer"}
            assert set(data["data"]["offer"]["price"])=={"instant_points_payable"}
            full=next(row for row in report["commands"] if row["command"]=="offer")
            assert cold["output_bytes"]<full["cold"]["output_bytes"], "field selection did not shrink offer output"
            report["commands"].append({"command":"offer_projection","arguments":field_args,"status":"pass","cold":cold,"warm":warm,
                                       "output_bytes_saved":full["cold"]["output_bytes"]-cold["output_bytes"]})
        report["status"]="pass"
    except (AssertionError,ValueError,KeyError,TypeError,OSError,subprocess.SubprocessError) as error:
        report["status"]="fail";report["error"]=str(error)[:600]
    save_report(args.output,report)
    return 0 if report["status"]=="pass" else 1

if __name__=="__main__":
    raise SystemExit(main())
