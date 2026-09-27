package walkerplus

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestFreeAdmissionSeparatesOptionalPurchases(t *testing.T) {
	for _, raw := range []string{
		"入場無料。入場無料（各ブースでの購入は有料）",
		"入場無料。商品購入は有料",
		"入場無料（飲食は有料）",
	} {
		e := emptyEvent("ar0726e612484", "京都パンフェスティバルin上賀茂神社 2026", "source")
		applyAdmission(&e, raw, "source")
		if e.Admission.Status != "free" || e.Admission.Price == nil || *e.Admission.Price != 0 || value(e.Admission.Raw) != raw {
			t.Fatalf("optional purchase became entry charge: %+v", e.Admission)
		}
	}
	for _, raw := range []string{
		"入場無料。一部有料",
		"入場無料。商品購入は有料。有料エリアあり",
		"入場無料。商品購入は有料。会場入館料500円",
		"入場無料。商品購入は有料。入館500円",
		"入場無料。商品購入は有料。商品購入が条件",
		"入場無料（小学生限定）。商品購入は有料",
		"小学生以下無料", "駐車場無料",
		"有料。料金は公式サイト参照。500円割引",
	} {
		e := emptyEvent("ar0726e1", "event", "source")
		applyAdmission(&e, raw, "source")
		if e.Admission.Status == "free" {
			t.Fatalf("qualified/required fee admitted as free: %q", raw)
		}
	}
}

func TestFreeConstraintIncludesSourceBackedEntryWithPurchases(t *testing.T) {
	title := "京都パンフェスティバルin上賀茂神社 2026"
	schema, err := json.Marshal(map[string]any{
		"@type": "Event", "name": title, "startDate": "2026-10-10", "endDate": "2026-10-11",
		"location": map[string]any{"name": "上賀茂神社", "address": map[string]any{"addressRegion": "京都府", "addressLocality": "京都市北区"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	script := "<script type=\"application/ld+json\">" + string(schema) + "</script>"
	card := "<li class=\"m-mainlist__item\"><a class=\"m-mainlist-item__ttl\" href=\"/event/ar0726e612484/\">" + title + "</a><p class=\"m-mainlist-item-event__period\">2026年10月10日～11日</p><p class=\"m-mainlist-item-event__place\">上賀茂神社</p><a href=\"/event_list/ar0726/\">京都府</a></li>"
	detail := "<h1>" + title + "</h1>" + script + "<table><tr class=\"m-infotable__row\"><th>開催日</th><td>2026年10月10日～11日</td></tr></table><a href=\"/event/ar0726e612484/price.html\">料金</a>"
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/event_list/10/ar0726/":
			w.Write([]byte(card + script))
		case "/event/ar0726e612484/":
			w.Write([]byte(detail))
		case "/event/ar0726e612484/price.html":
			w.Write(fixture(t, "free-admission-price.html"))
		default:
			t.Errorf("unexpected route %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}, Options{})
	result, err := c.Shortlist(context.Background(), Query{Prefecture: "kyoto", From: "2026-10-10", To: "2026-10-11", Free: true, MaxPages: 1, MaxDetails: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Events) != 1 || result.Events[0].ID != "ar0726e612484" || result.Events[0].Admission.Status != "free" || result.Events[0].Admission.Price == nil || *result.Events[0].Admission.Price != 0 || !strings.Contains(value(result.Events[0].Admission.Raw), "各ブースでの購入は有料") {
		t.Fatalf("free entry filtered away or caveat lost: %+v", result)
	}
}
