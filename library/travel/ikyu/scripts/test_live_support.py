#!/usr/bin/env python3
"""Deterministic independent-parser regressions; no network or product adapter."""
import datetime as dt
import json
from pathlib import Path
import unittest
from live_support import cancellation, independent_offer, nuxt_operations, selection, stay_dates, visible_text

ROOT=Path(__file__).resolve().parents[1]


def encode_nuxt(operations):
    # Emit devalue references from sanitized public response facts. The unused
    # review/state refs are deliberately invalid: an independent offer decoder
    # must select the two public operations instead of traversing them.
    table=[None,{"data":2},["ShallowReactive",3],{}]
    def add(value):
        if value is None: return -1
        index=len(table);table.append(None)
        if isinstance(value,dict): table[index]={k:add(v) for k,v in value.items()}
        elif isinstance(value,list): table[index]=[add(v) for v in value]
        else: table[index]=value
        return index
    for name,value in operations.items():
        key=json.dumps({"o":name,"v":{}},separators=(",",":"))
        table[3][key]=add(value)
    table[3]['{"o":"AccommodationReviewList","v":{}}']=999999
    table[1]["state"]=999999
    return '<script id="__NUXT_DATA__">'+json.dumps(table,ensure_ascii=False)+'</script>'


class IndependentParserTests(unittest.TestCase):
    def test_source_prices_meal_and_full_ordered_policy(self):
        fixture=json.loads((ROOT/"testdata/ikyu/offer-response.json").read_text())
        rp=fixture["data"]["accommodation"]["roomPlan"]
        document=encode_nuxt({"RoomPlanDetailAlt":{"accommodation":{"roomPlan":{"room":rp["room"],"plan":rp["plan"]}}},
                              "RoomPlanDetailAmount":{"accommodation":{"roomPlan":{"booking":rp["booking"]}}}})
        result=independent_offer(document)
        self.assertEqual(result["prices"]["source_amount"],30800)
        self.assertEqual(result["prices"]["instant_points_payable"],24640)
        self.assertEqual(result["prices"]["points_applied"],6160)
        self.assertEqual(result["meal"]["code"],"000")
        self.assertEqual([r["day"] for r in result["cancellation"]["rules"]],[None,0,1,3,7,10])
        self.assertEqual([r["rate"] for r in result["cancellation"]["rules"]],[100,100,70,50,30,20])
        self.assertEqual(result["nightly_dates"],["2026-10-18"])
        self.assertEqual(result["occupancy"]["peopleCount"],2)

    def test_missing_or_changed_data_fails(self):
        for text in ["<html>Forbidden</html>",'<script id="__NUXT_DATA__">[]</script>']:
            with self.assertRaises(AssertionError): nuxt_operations(text,{"RoomPlanDetailAmount"})
        with self.assertRaises(AssertionError):
            cancellation({"id":"x","rules":[{"__typename":"NewRuleKind","amount":{"__typename":"CancelPolicyRuleAmountRate","rate":100}}]})

    def test_visible_text_excludes_script_and_decodes_entities(self):
        text=visible_text('<script>secret 24,640</script><style>hidden</style><p>30,800 &amp; 6,160</p><svg>icon</svg>')
        self.assertEqual(text,'30,800 & 6,160')

    def test_ids_and_future_stay_bounds(self):
        self.assertEqual(selection('00002889:10193741:11055986')[0],'00002889')
        with self.assertRaises(ValueError): selection('2889:10193741:11055986')
        start,end=stay_dates(); self.assertEqual((dt.date.fromisoformat(end)-dt.date.fromisoformat(start)).days,1)
        with self.assertRaises(ValueError): stay_dates('2000-01-01','2000-01-02')


if __name__=='__main__': unittest.main()
