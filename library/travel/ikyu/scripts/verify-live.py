#!/usr/bin/env python3
"""Assert real anonymous stays and independently cross-check public HTML/SSR.

No raw responses are saved. Dates default to today +21 days in Japan. Explicit
selectors can reproduce a known baseline, or discover a current default-variation
selection from each property. A sold-out exact selection fails this happy-flow
check; the CLI's sold-out/error behavior has separate deterministic Go tests.
"""
import argparse
import datetime as dt
import json
from pathlib import Path
import tempfile
import subprocess
from live_support import (PRICE_FIELDS, canonical, cli, discover, fetch_html, independent_offer,
                          save_report, selection, source_text, stay_args, stay_dates, timestamp,
                          visible_text, write_workflow)


def crosscheck(result, selector, dates):
    prop, room, plan = selection(selector)
    data=result["data"]; offer=data["offer"]; quote=offer["price"]
    assert data["property"]["id"]==prop and data["room"]["id"]==room and data["plan"]["id"]==plan
    assert offer["available"] is True and offer["date_verified"] is True and quote is not None
    assert offer["source_stay"]==offer["stay"]
    assert quote["currency"]=="JPY" and quote["unit"]=="booking_total"
    assert quote["checkout_confirmed_payable"] is None and quote["eligibility_known"] is False
    document,meta=fetch_html(canonical(selector,*dates))
    source=independent_offer(document)
    assert data["room"]["name"]==source["room_name"], "room name differs from fresh public SSR"
    assert data["plan"]["name"]==source["plan_name"], "plan name differs from fresh public SSR"
    assert data["plan"]["meal"]==source["meal"], "meal differs from fresh public SSR"
    assert data["plan"]["cancellation"]==source["cancellation"], "ordered cancellation fields differ from fresh public SSR"
    for field in PRICE_FIELDS:
        assert quote[field]==source["prices"][field], f"{field} differs from fresh public SSR"
        if field not in ("points_rate","points_applied_rate") and quote[field] is not None:
            assert isinstance(quote[field],int) and not isinstance(quote[field],bool), "money must remain source integer"
    start=dt.date.fromisoformat(dates[0]); end=dt.date.fromisoformat(dates[1])
    expected_dates=[(start+dt.timedelta(days=i)).isoformat() for i in range((end-start).days)]
    assert source["nightly_dates"]==expected_dates, "public SSR nightly stay echo changed"
    party=source["occupancy"]
    assert party["peopleCount"]==2 and party["roomCount"]==1 and party["lodgingCount"]==(end-start).days
    assert all(party[f"child{x}Count"]==0 for x in "ABCDEF")
    visible=visible_text(document)
    visible_headline=f'{quote["headline_before_points"]:,}'
    visible_earned=f'{quote["points_earned"]:,}'
    assert visible_headline in visible, "earn-mode headline absent from public visible HTML"
    assert visible_earned in visible, "earned points absent from public visible HTML"
    assert source_text(source["meal"]["name"]) in visible, "meal absent from public visible HTML"
    assert data["plan"]["source_point_variation"]==0, "exact offer default variant identity absent"
    bath=data["room"]["bath"]
    if bath["hot_spring"] is True:
        assert any(a["value"]=="16" for a in bath["proof"]), "room hot spring lacks explicit room evidence"
    return {**meta,"selector":selector,"room_name":source["room_name"],"plan_name":source["plan_name"],
            "meal":source["meal"],"cancellation":source["cancellation"],"prices":source["prices"],
            "nightly_dates":source["nightly_dates"],"occupancy":party,
            "visible_earn_mode":{"headline_before_points":True,"earned_points":True,"meal":True},
            "instant_price_evidence":"fresh public SSR JSON; default visible HTML uses earn points",
            "status":"pass"}


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary",type=Path,default=Path(__file__).resolve().parents[1]/"ikyu-pp-cli")
    parser.add_argument("--check-in"); parser.add_argument("--check-out")
    parser.add_argument("--hotel-offer",help="property:room:plan; default discovers from hotel 00000600")
    parser.add_argument("--ryokan-offer",help="property:room:plan; default discovers from ryokan 00002889")
    parser.add_argument("--output",type=Path)
    parser.add_argument("--write-workflow",type=Path,help="refresh workflow_verify.yaml with the same future stay")
    args=parser.parse_args(); dates=stay_dates(args.check_in,args.check_out)
    report={"status":"running","checked_at":timestamp(),"stay":{"check_in":dates[0],"check_out":dates[1],"adults_per_room":2,"rooms":1},"checks":[],"independent_source":[],"scope":"anonymous read-only; no raw HTML/SSR or account fields persisted"}
    try:
        with tempfile.TemporaryDirectory(prefix="ikyu-live-") as tmp:
            cache=Path(tmp)
            destinations=cli(args.binary,["stay","destinations","tokyo","--limit","5"],cache)
            assert destinations["data"] and any(d["path"]=="/tokyo/140000/" and d["name"]=="東京" for d in destinations["data"])
            report["checks"].append({"check":"source Tokyo destination","status":"pass"})
            search=cli(args.binary,["stay","search","--destination","tokyo",*stay_args(*dates),"--limit","3"],cache)
            assert search["data"] and search["stay"]["check_in"]==dates[0] and search["stay"]["adults_per_room"]==2
            assert all(p["id"].isdigit() and len(p["id"])==8 and p["latitude"] is not None and 35.0 < p["latitude"] < 36.5 for p in search["data"]), "search did not yield relevant Tokyo source identities"
            assert search["pagination"]["scanned"]<=3 and search["pagination"]["source_total"]>=len(search["data"])
            mismatch=cli(args.binary,["stay","search","tokyo",*stay_args(*dates),"--limit","3","--max-budget","1","--outdoor-bath"],cache)
            assert mismatch["data"]==[] and mismatch["pagination"]["complete"] is False and mismatch["filter_coverage"]["note"]
            report["checks"].append({"check":"dated Tokyo and combined-filter coverage","status":"pass","source_ids":[p["id"] for p in search["data"]],"pagination":search["pagination"]})
            selectors=[]
            for kind,value,prop in [("hotel",args.hotel_offer,"00000600"),("ryokan",args.ryokan_offer,"00002889")]:
                selector=value or discover(args.binary,prop,dates,cache); prop,room,plan=selection(selector); selectors.append(selector)
                property_data=cli(args.binary,["stay","property",prop],cache)
                assert property_data["data"]["id"]==prop and property_data["data"]["name"]
                rooms=cli(args.binary,["stay","rooms",prop,*stay_args(*dates),"--limit","1","--plan-limit","2"],cache)
                assert rooms["data"] and rooms["occupancy_echo_verified"] is True and rooms["date_echo_verified"] is False
                assert rooms["pagination"]["scanned"]<=1 and len(rooms["data"][0]["plans"])<=2
                if rooms["pagination"]["source_total"]>1:
                    next_rooms=cli(args.binary,["stay","rooms",prop,*stay_args(*dates),"--limit","1","--offset","1","--plan-limit","2"],cache)
                    assert next_rooms["pagination"]["offset"]==1 and next_rooms["pagination"]["source_total"]==rooms["pagination"]["source_total"]
                    if next_rooms["data"]: assert next_rooms["data"][0]["id"]!=rooms["data"][0]["id"]
                offer_args=["stay","offer",prop,room,plan,*stay_args(*dates)]
                exact=cli(args.binary,offer_args,cache)
                report["independent_source"].append({"kind":kind,**crosscheck(exact,selector,dates)})
                # Projection is read through on the same key and must preserve exact integer amounts.
                projected=cli(args.binary,[*offer_args,"--fields","data.plan.id,data.offer.price.instant_points_payable,stats"],cache)
                assert projected["data"]["plan"]=={"id":plan} and projected["stats"]["requests"]==0
                assert projected["data"]["offer"]["price"]=={"instant_points_payable":exact["data"]["offer"]["price"]["instant_points_payable"]}
                assert "room" not in projected["data"]
                report["checks"].append({"check":f"{kind} property/room paging and exact projected offer","status":"pass","selector":selector,"requests":exact["stats"]["requests"]})
            comparison=cli(args.binary,["stay","compare","--offers",",".join(selectors),*stay_args(*dates)],cache)
            assert comparison["requested"]==2 and len(comparison["data"])==2 and comparison["successful"]==2
            assert comparison["comparisons"] and all(not pair["assessment"]["equivalent"] and pair["assessment"]["source_amount_difference"] is None for pair in comparison["comparisons"])
            next_date=(dt.date.fromisoformat(dates[0])+dt.timedelta(days=1)).isoformat()
            date_result=cli(args.binary,["stay","dates",*selection(selectors[1]),"--check-ins",f"{dates[0]},{next_date}","--nights",str((dt.date.fromisoformat(dates[1])-dt.date.fromisoformat(dates[0])).days)],cache)
            assert [r["selector"] for r in date_result["data"]]==[dates[0],next_date] and date_result["requested"]==2
            for row in date_result["data"]:
                if row["data"] and row["data"]["offer"]["available"] is False: assert row["data"]["offer"]["price"] is None
            report["checks"].append({"check":"comparison alternatives and all requested dates retained","status":"pass","date_rows":len(date_result["data"]),"date_successful":date_result["successful"]})
            before=set(cache.glob("*.json"))
            cli(args.binary,["stay","search","tokyo","--check-in","2026-13-40","--check-out","2026-13-41"],cache,expected=2)
            cli(args.binary,["stay","search","tokyo",*stay_args(*dates),"--adults","0"],cache,expected=2)
            assert set(cache.glob("*.json"))==before
            report["checks"].append({"check":"invalid dates/adults rejected without cache additions","status":"pass","network_zero_evidence":"deterministic source/CLI Go tests assert no source call before validation"})
        if args.write_workflow: write_workflow(args.write_workflow,dates)
        report["status"]="pass"
    except (AssertionError,ValueError,KeyError,TypeError,RuntimeError,OSError,subprocess.SubprocessError) as error:
        report["status"]="fail"; report["error"]=str(error)[:600]
    save_report(args.output,report)
    return 0 if report["status"]=="pass" else 1

if __name__=="__main__":
    raise SystemExit(main())
