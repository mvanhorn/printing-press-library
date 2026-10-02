package toyota

import (
	"strings"
	"testing"
	"time"
)

func TestShopIdentityAndCanonicalURL(t *testing.T) {
	for _, tc := range []struct {
		id string
		ok bool
	}{
		{"63601:01V", true}, {"65201:002", true}, {"63601:01v", false}, {"63601_01V", false},
		{"63601:01V&x=y", false}, {"63601:01V/../../", false}, {"6360:01V", false},
	} {
		t.Run(tc.id, func(t *testing.T) {
			r, e, err := ParseShopID(tc.id)
			if (err == nil) != tc.ok {
				t.Fatalf("err=%v", err)
			}
			u, err := ShopURL(tc.id, true)
			if tc.ok {
				if r+":"+e != tc.id || err != nil || !strings.Contains(u, "shopMode=1") || !strings.HasPrefix(u, Origin+BookingPath+"?") {
					t.Fatalf("identity/url=%s %s %v", r, e, err)
				}
			} else if err == nil {
				t.Fatal("invalid identity accepted for URL")
			}
		})
	}
}

func TestPeriodJSTAndCalendarBounds(t *testing.T) {
	now := time.Date(2026, 10, 2, 0, 0, 0, 0, jst)
	for _, tc := range []struct {
		name, pick, ret string
		ok              bool
		hours           float64
	}{
		{"JST", "2026-10-20T09:00", "2026-10-21T09:00", true, 24},
		{"UTC normalizes", "2026-10-20T00:00:00Z", "2026-10-21T00:30:00Z", true, 24.5},
		{"equal", "2026-10-20T09:00", "2026-10-20T09:00", false, 0},
		{"before", "2026-10-20T09:00", "2026-10-19T09:00", false, 0},
		{"past", "2026-10-01T09:00", "2026-10-02T09:00", false, 0},
		{"wrong increment", "2026-10-20T09:15", "2026-10-21T09:00", false, 0},
		{"seconds", "2026-10-20T09:00:01+09:00", "2026-10-21T09:00", false, 0},
		{"too far", "2027-01-03T09:00", "2027-01-04T09:00", false, 0},
		{"too long", "2026-10-20T09:00", "2026-11-20T09:30", false, 0},
		{"bad date", "2026-02-30T09:00", "2026-03-02T09:00", false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := ParsePeriod(tc.pick, tc.ret, now)
			if (err == nil) != tc.ok {
				t.Fatalf("err=%v", err)
			}
			if tc.ok && (p.Hours != tc.hours || p.Pickup.Format("-07:00") != "+09:00") {
				t.Fatalf("period=%+v", p)
			}
		})
	}
	for _, tc := range []struct {
		start  string
		months int
		want   string
	}{
		{"2026-01-31", 1, "2026-02-28"}, {"2028-01-31", 1, "2028-02-29"}, {"2026-11-30", 3, "2027-02-28"},
	} {
		t0, _ := time.ParseInLocation("2006-01-02", tc.start, jst)
		if got := addMonthsClamped(t0, tc.months).Format("2006-01-02"); got != tc.want {
			t.Fatalf("clamp=%s want=%s", got, tc.want)
		}
	}
}

func TestOptionsAndHandoffDoNotClaimInventoryOrFullTotal(t *testing.T) {
	p, _ := ParsePeriod("2026-10-20T09:00", "2026-10-21T09:00", time.Date(2026, 10, 2, 0, 0, 0, 0, jst))
	for _, tc := range []struct {
		name  string
		o     SearchOptions
		class string
		ok    bool
	}{
		{"normal", SearchOptions{Transmission: "AT", Seats: []string{"infant", "booster"}}, "C1", true},
		{"MT", SearchOptions{Transmission: "MT", FourWD: true, WinterTires: true, Seats: []string{}}, "SUV2", true},
		{"seat typo", SearchOptions{Transmission: "AT", Seats: []string{"baby"}}, "C1", false},
		{"too many", SearchOptions{Transmission: "AT", Seats: []string{"child", "child", "child", "child", "child"}}, "", false},
		{"transmission typo", SearchOptions{Transmission: "automatic"}, "C1", false},
		{"class injection", SearchOptions{Transmission: "AT"}, "C1&foo", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, err := BookingHandoff("63601:01V", "63601:095", p, tc.o, tc.class)
			if (err == nil) != tc.ok {
				t.Fatalf("err=%v", err)
			}
			if tc.ok && (h.InventoryChecked || h.ConfirmedFullTotalJPY != nil || len(h.RequiresReentry) != 3 || len(h.URLPrefills) != 1 || strings.Contains(h.BookingURL, "2026")) {
				t.Fatalf("dishonest handoff=%+v", h)
			}
		})
	}
}
