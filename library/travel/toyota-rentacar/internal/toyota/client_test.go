package toyota

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/toyota-rentacar/internal/cliutil"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func reply(r *http.Request, status int, body, location string) *http.Response {
	headers := http.Header{}
	if location != "" {
		headers.Set("Location", location)
	}
	return &http.Response{StatusCode: status, Header: headers, Body: io.NopCloser(strings.NewReader(body)), Request: r}
}
func fastClient(f roundTripFunc) *Client {
	c := newClient("https://test.example", f)
	c.limiter = cliutil.NewAdaptiveLimiter(100000)
	return c
}

func TestClientHonorsExplicitRequestRateCeiling(t *testing.T) {
	c := NewClient(0.1)
	for range 30 {
		c.limiter.OnSuccess()
	}
	c.limiter.ObserveHeaders(1000, time.Now().Add(time.Second))
	if err := c.limiter.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()
	if err := c.limiter.Wait(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("explicit slow ceiling bypassed after success/header observations: %v", err)
	}
	if NewClient(0).limiter != nil || NewClient(-1).limiter == nil {
		t.Fatal("disabled/auto rate modes were not preserved")
	}
}

func rentalDoc(classBody string) string {
	var b strings.Builder
	b.WriteString(`<form id="PageForm">`)
	for id, v := range map[string]string{
		"txtDepShop": "", "hdDepShop": "Tokyo Nihonbashi Shop", "txtRetShop": "", "hdRetShop": "",
		"txtHdnDepRCode": "63601", "txtHdnDepECode": "01V", "txtHdnRetRCode": "63601", "txtHdnRetECode": "01V",
		"hdDepDate": "2026_10_20_0900", "hdRetDate": "2026_10_21_0900",
		"txtHdnSameShopReturnFlg": "", "txtHdnCarSelectPageFlg": "",
		"hdnTrans": "0", "hdnDrive": "0", "hdnTire": "0", "hdnSeatSum": "0",
		"hdnRdoSeat01": "", "hdnRdoSeat02": "", "hdnRdoSeat03": "", "hdnRdoSeat04": "",
	} {
		fmt.Fprintf(&b, `<input id="%s" name="f$%s" value="%s">`, id, id, v)
	}
	b.WriteString(`<input id="cbRetShop" name="cb" type="checkbox" checked>`)
	b.WriteString(`<span id="lblOpenJson">{"open":{"20261020":["0800","2000"],"20261021":["0800","2000"]}}</span><span id="lblReturnOpenJson">{"open":{"20261021":["0800","2000"]}}</span>`)
	b.WriteString(shopCard("63601:01V", "Tokyo Nihonbashi", "東京日本橋", "*One-way:Available inside / outside the prefecture"))
	if classBody != "" {
		b.WriteString(`<span id="lblTypeName">Mini/Compact</span>` + classBody)
	}
	b.WriteString("</form>")
	return b.String()
}

func TestQuoteReplaysDatedFormAndRejectsContextDrift(t *testing.T) {
	period := Period{Pickup: time.Date(2026, 10, 20, 9, 0, 0, 0, jst), Dropoff: time.Date(2026, 10, 21, 9, 0, 0, 0, jst), Hours: 24}
	for _, tc := range []struct {
		name   string
		change func(string) string
		ok     bool
	}{
		{"valid", func(s string) string { return s }, true},
		{"changed date", func(s string) string { return strings.ReplaceAll(s, "2026_10_20_0900", "2026_10_22_0900") }, false},
		{"changed option", func(s string) string {
			return strings.ReplaceAll(s, `id="hdnDrive" name="f$hdnDrive" value="0"`, `id="hdnDrive" name="f$hdnDrive" value="1"`)
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := fastClient(func(r *http.Request) (*http.Response, error) {
				if r.Method == "POST" {
					_ = r.ParseForm()
					switch r.PostForm.Get("__EVENTTARGET") {
					case "ctl00$MasterBodyPlaceHolder$ReserveBasicInfoUc$lnkBtnSearch":
						for k, want := range map[string]string{"f$hdDepDate": "2026_10_20_0900", "f$hdRetDate": "2026_10_21_0900", "f$txtRetShop": "Tokyo Nihonbashi Shop", "f$txtHdnSameShopReturnFlg": "1", "cb": "on"} {
							if r.PostForm.Get(k) != want {
								t.Errorf("posted %s=%q want %q", k, r.PostForm.Get(k), want)
							}
						}
						return reply(r, 302, "", "/eng/reservation/recommended/"), nil
					case "ctl00$MasterBodyPlaceHolder$lnkChangeCarPc":
						return reply(r, 302, "", "/eng/reservation/index02.aspx"), nil
					}
					t.Fatalf("unexpected POST: %v", r.PostForm)
				}
				switch r.URL.Path {
				case BookingPath:
					return reply(r, 200, rentalDoc(""), ""), nil
				case "/eng/reservation/recommended/":
					return reply(r, 200, `<form id="PageForm"></form>`, ""), nil
				case "/eng/reservation/index02.aspx":
					return reply(r, 200, tc.change(rentalDoc(classCards)), ""), nil
				}
				t.Fatalf("unexpected path=%s", r.URL)
				return nil, nil
			})
			out, err := c.Quote(context.Background(), "63601:01V", "63601:01V", period, SearchOptions{Transmission: "AT", Seats: []string{}}, "compact", "", 8, false)
			if (err == nil) != tc.ok {
				t.Fatalf("err=%v", err)
			}
			if tc.ok && (len(out.Offers) != 2 || out.ConfirmedFullTotalJPY != nil || out.Meta.Requests != 5 || out.Meta.ResponseBytes == 0 || out.Period.Hours != 24) {
				t.Fatalf("quote=%+v", out)
			}
		})
	}
}

func TestQuoteFiltersClassBeforeLimitingOffers(t *testing.T) {
	period := Period{Pickup: time.Date(2026, 10, 20, 9, 0, 0, 0, jst), Dropoff: time.Date(2026, 10, 21, 9, 0, 0, 0, jst), Hours: 24}
	for _, tc := range []struct {
		name, class, wantClass string
		limit                  int
		truncated              bool
		missing                bool
		invalid                bool
	}{
		{name: "later class remains returned", class: "C0", limit: 1, wantClass: "C0"},
		{name: "unfiltered limit", limit: 1, wantClass: "C1", truncated: true},
		{name: "absent class", class: "C9", limit: 1, missing: true},
		{name: "filtered invalid zero limit", class: "C0", limit: 0, invalid: true},
		{name: "filtered invalid large limit", class: "C0", limit: 21, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := fastClient(func(r *http.Request) (*http.Response, error) {
				if r.Method == http.MethodPost {
					return reply(r, http.StatusFound, "", "/eng/reservation/index02.aspx"), nil
				}
				if r.URL.Path == BookingPath {
					return reply(r, http.StatusOK, rentalDoc(""), ""), nil
				}
				if r.URL.Path == "/eng/reservation/index02.aspx" {
					return reply(r, http.StatusOK, rentalDoc(classCards), ""), nil
				}
				t.Fatalf("unexpected source request: %s", r.URL.Path)
				return nil, nil
			})
			out, err := c.Quote(context.Background(), "63601:01V", "63601:01V", period, SearchOptions{Transmission: "AT", Seats: []string{}}, "compact", tc.class, tc.limit, false)
			if tc.invalid {
				var input *InputError
				if !errors.As(err, &input) || c.requests != 0 {
					t.Fatalf("limit validation: err=%v requests=%d", err, c.requests)
				}
				return
			}
			if tc.missing {
				var missing *NotFoundError
				if !errors.As(err, &missing) {
					t.Fatalf("missing class should be typed: %v", err)
				}
				return
			}
			if err != nil || len(out.Offers) != 1 || out.Offers[0].Class != tc.wantClass || out.Truncated != tc.truncated || out.SourceCount != 2 || out.ConfirmedFullTotalJPY != nil {
				t.Fatalf("filtered quote=%+v err=%v", out, err)
			}
			if tc.class == "C0" && out.Offers[0].Availability != "fully_booked" {
				t.Fatalf("class filtering changed source availability: %+v", out.Offers[0])
			}
		})
	}
}

func TestClientRejectsWrongSearchContextAndMissingID(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		wantError  bool
	}{
		{"correct", `<form id="PageForm"><h1>Around "Kyoto Station"</h1>` + shopCard("65201:002", "Kyoto Shinkansen", "京都駅", "") + "</form>", false},
		{"fallback", `<form id="PageForm"><h1>Around "Tokyo Station"</h1>` + shopCard("63601:01V", "Tokyo", "東京", "") + "</form>", true},
		{"empty", `<form id="PageForm"><h1>Around "Kyoto Station"</h1></form>`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := fastClient(func(r *http.Request) (*http.Response, error) {
				if r.URL.Query().Get("keyword") != "Kyoto Station" {
					t.Fatal("lost search keyword")
				}
				return reply(r, 200, tc.body, ""), nil
			})
			out, err := c.SearchShops(context.Background(), "Kyoto Station", 3)
			if (err != nil) != tc.wantError {
				t.Fatalf("err=%v", err)
			}
			if tc.name == "empty" && (out.Shops == nil || len(out.Shops) != 0 || out.Note == "") {
				t.Fatalf("empty=%+v", out)
			}
		})
	}
	c := fastClient(func(r *http.Request) (*http.Response, error) { return reply(r, 200, rentalDoc(""), ""), nil })
	if _, err := c.GetShop(context.Background(), "63601:ZZZ"); err == nil {
		t.Fatal("returned another shop for unknown ID")
	}
}

func TestClientBudgetsTimeoutsThrottleAndRedirectOrigin(t *testing.T) {
	for _, tc := range []struct {
		name           string
		status         int
		body, location string
		budget         bool
		cancel         bool
		want           string
	}{
		{"429", 429, "", "", false, false, "rate"},
		{"outside redirect", 302, "", "https://outside.example/", false, false, "source"},
		{"oversized", 200, strings.Repeat("x", maxBody+1), "", false, false, "source"},
		{"budget", 200, "<p>ok</p>", "", true, false, "source"},
		{"cancel", 200, "<p>ok</p>", "", false, true, "context"},
		{"source error", 200, "<p>Source network error</p>", "", false, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := fastClient(func(r *http.Request) (*http.Response, error) { return reply(r, tc.status, tc.body, tc.location), nil })
			if tc.budget {
				c.requests = maxRequests
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			_, _, _, err := c.request(ctx, OptionsPath, nil)
			switch tc.want {
			case "rate":
				var e *cliutil.RateLimitError
				if !errors.As(err, &e) {
					t.Fatalf("not typed throttle: %v", err)
				}
			case "source":
				var e *SourceError
				if !errors.As(err, &e) {
					t.Fatalf("not typed source error: %v", err)
				}
			case "context":
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("context not honored: %v", err)
				}
			}
		})
	}
	c := NewClient(-1)
	if c.http.Jar == nil || c.origin != Origin || c.Meta().MaxRequests != maxRequests {
		t.Fatal("anonymous in-memory session/budget not initialized")
	}
}

func TestOneWayZeroFeeAndFamilyAreSourceBound(t *testing.T) {
	for _, tc := range []struct {
		name               string
		fee, family, drift string
		ok                 bool
	}{{"zero", "0", "C1", "", true}, {"missing", "", "C1", "", false}, {"wrong family", "5,500", "W1", "", false},
		{"pickup annex", "5,500", "C1", "pickup", false}, {"return annex", "5,500", "C1", "return", false}} {
		t.Run(tc.name, func(t *testing.T) {
			calculated := false
			c := fastClient(func(r *http.Request) (*http.Response, error) {
				if r.Method == "POST" {
					_ = r.ParseForm()
					switch r.PostForm.Get("__EVENTTARGET") {
					case "ctl00$MasterBodyPlaceHolder$btnSearchShop":
						return reply(r, 302, "", "/eng/shop/"), nil
					case "ctl00$MasterBodyPlaceHolder$OnewayPriceInfoUc$lnkBtnSearch":
						if r.PostForm.Get("f$txtHdnRetECode") != "095" {
							t.Fatal("return identity not preserved")
						}
						return reply(r, 302, "", OneWayPath), nil
					case "ctl00$MasterBodyPlaceHolder$lnkCalculationBotton":
						if r.PostForm.Get("ctl00$MasterBodyPlaceHolder$ddlCarType") != "C1" {
							t.Fatal("fee family lost")
						}
						calculated = true
					}
				}
				if r.URL.Path == "/eng/shop/" {
					body := rentalDoc("")
					if r.URL.Query().Get("shopMode") == "1" {
						body = strings.ReplaceAll(body, "01V", "095")
						body = strings.ReplaceAll(body, "Tokyo Nihonbashi", "Hatchobori")
					}
					return reply(r, 200, body, ""), nil
				}
				body := fmt.Sprintf(`<form id="PageForm"><select id="ddlCarType" name="ctl00$MasterBodyPlaceHolder$ddlCarType"><option value="%s" selected>Standard</option></select>`, tc.family)
				if calculated {
					body += fmt.Sprintf(`<span id="lblDepartShop">From Tokyo NihonbashiStore</span><span id="lblReturnShop">To HatchoboriStore</span><span id="lblOnewayPrice">%s</span>`, tc.fee)
					if tc.drift == "pickup" {
						body = strings.ReplaceAll(body, "Tokyo NihonbashiStore", "Tokyo Nihonbashi AnnexStore")
					}
					if tc.drift == "return" {
						body = strings.ReplaceAll(body, "HatchoboriStore", "Hatchobori AnnexStore")
					}
				}
				return reply(r, 200, body+"</form>", ""), nil
			})
			out, err := c.OneWay(context.Background(), "63601:01V", "63601:095", "standard")
			if (err == nil) != tc.ok {
				t.Fatalf("err=%v", err)
			}
			if tc.ok && (out.FeeJPY == nil || *out.FeeJPY != 0 || out.Status != "calculated" || out.DatedAvailability != "unknown" || out.Meta.Requests != 8) {
				t.Fatalf("simulation=%+v", out)
			}
		})
	}
}

func TestPoliciesMethodsUseFirstPartyPathsAndMetadata(t *testing.T) {
	c := fastClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case OptionsPath:
			return reply(r, 200, string(fixture(t, "option.html")), ""), nil
		case InsurancePath:
			return reply(r, 200, string(fixture(t, "insurance.html")), ""), nil
		case EligibilityPath:
			return reply(r, 200, string(fixture(t, "drive.html")), ""), nil
		}
		t.Fatalf("unexpected path=%s", r.URL.Path)
		return nil, nil
	})
	out, err := c.Options(context.Background())
	if err != nil || len(out.Options) != 13 || out.Meta.Requests != 2 {
		t.Fatalf("options=%+v err=%v", out, err)
	}
	guidance, err := c.Eligibility(context.Background())
	if err != nil || guidance.Meta.Requests != 3 || guidance.IndividualEligibility != "not_assessed" {
		t.Fatalf("guidance=%+v err=%v", guidance, err)
	}
	// A form path cannot become an arbitrary URL even if a caller supplies it.
	if _, _, _, err := c.request(context.Background(), "//outside.example/", url.Values{}); err == nil {
		t.Fatal("arbitrary origin accepted")
	}
}
