package toyota

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

var categories = map[string]string{"compact": "1", "standard": "2", "minivan": "4", "suv": "5"}

func optionsWire(o SearchOptions) map[string]string {
	tr := "0"
	if o.Transmission == "MT" {
		tr = "1"
	}
	drive := "0"
	if o.FourWD {
		drive = "1"
	}
	tire := "0"
	if o.WinterTires {
		tire = "1"
	}
	values := map[string]string{"hdnTrans": tr, "hdnDrive": drive, "hdnTire": tire, "hdnSeatSum": strconv.Itoa(len(o.Seats))}
	seatValues := map[string]string{"child": "0", "infant": "1", "booster": "2"}
	for i := 0; i < 4; i++ {
		v := ""
		if i < len(o.Seats) {
			v = seatValues[o.Seats[i]]
		}
		values[fmt.Sprintf("hdnRdoSeat%02d", i+1)] = v
	}
	return values
}

func applyOptions(f url.Values, doc *html.Node, o SearchOptions) error {
	for id, v := range optionsWire(o) {
		if err := setID(f, doc, id, v); err != nil {
			return err
		}
	}
	for _, id := range []string{"rbTransA", "rbDriveN", "rbTireN"} {
		n := byID(doc, id)
		name := attr(n, "name")
		if name != "" {
			f.Del(name)
		}
	}
	radio := []string{"rbTransA", "rbDriveN", "rbTireN"}
	if o.Transmission == "MT" {
		radio[0] = "rbTransM"
	}
	if o.FourWD {
		radio[1] = "rbDrive4"
	}
	if o.WinterTires {
		radio[2] = "rbTireY"
	}
	for _, id := range radio {
		n := byID(doc, id)
		if name := attr(n, "name"); name != "" {
			f.Set(name, attr(n, "value"))
		}
	}
	for i, seat := range o.Seats {
		f.Set(fmt.Sprintf("seat%02d", i+1), map[string]string{"child": "0", "infant": "1", "booster": "2"}[seat])
	}
	return nil
}

func (c *Client) Quote(ctx context.Context, pickupID, dropoffID string, period Period, o SearchOptions, category, class string, limit int, details bool) (QuoteResult, error) {
	carType, ok := categories[category]
	if !ok {
		return QuoteResult{}, &InputError{"--category must be compact,standard,minivan or suv"}
	}
	if limit < 1 || limit > 20 {
		return QuoteResult{}, &InputError{"--limit must be between 1 and 20"}
	}
	if err := o.Validate(); err != nil {
		return QuoteResult{}, err
	}
	pickup, doc, final, err := c.shop(ctx, pickupID, BookingPath, false)
	if err != nil {
		return QuoteResult{}, err
	}
	// Shop-selection pages publish the operating calendars. Class pages may
	// omit them, so validate each exact shop's own calendar before searching.
	pickupWindow, err := checkOperatingWindow(doc, "lblOpenJson", "--pickup", period.Pickup)
	if err != nil {
		return QuoteResult{}, err
	}
	dropoff := pickup
	dropoffWindow, err := checkOperatingWindow(doc, "lblOpenJson", "--dropoff", period.Dropoff)
	if err != nil && dropoffID == pickupID {
		return QuoteResult{}, err
	}
	if dropoffID != pickupID {
		var returnDoc *html.Node
		dropoff, returnDoc, _, err = c.shop(ctx, dropoffID, BookingPath, false)
		if err != nil {
			return QuoteResult{}, err
		}
		dropoffWindow, err = checkOperatingWindow(returnDoc, "lblOpenJson", "--dropoff", period.Dropoff)
		if err != nil {
			return QuoteResult{}, err
		}
		// Restore the actual pickup after inspecting the return location.
		_, doc, final, err = c.shop(ctx, pickupID, BookingPath, false)
		if err != nil {
			return QuoteResult{}, err
		}
	}
	if pickupID != dropoffID && dropoff.OneWayReturns == "not_possible" {
		return QuoteResult{}, &InputError{"--dropoff-shop " + dropoffID + " does not allow one-way returns according to Toyota; choose another return shop"}
	}
	f := postFields(doc, "ctl00$MasterBodyPlaceHolder$ReserveBasicInfoUc$lnkBtnSearch")
	if err := fillShopPair(f, doc, pickup, dropoff); err != nil {
		return QuoteResult{}, err
	}
	same := "0"
	if pickupID == dropoffID {
		same = "1"
	}
	for id, v := range map[string]string{"hdDepDate": dateWire(period.Pickup), "hdRetDate": dateWire(period.Dropoff), "txtHdnSameShopReturnFlg": same, "txtHdnCarSelectPageFlg": "1"} {
		if err := setID(f, doc, id, v); err != nil {
			return QuoteResult{}, err
		}
	}
	if name := attr(byID(doc, "cbRetShop"), "name"); name != "" {
		if same == "1" {
			f.Set(name, "on")
		} else {
			f.Del(name)
		}
	}
	if err := applyOptions(f, doc, o); err != nil {
		return QuoteResult{}, err
	}
	data, doc, final, err := c.request(ctx, pathOf(final), f)
	if err != nil {
		return QuoteResult{}, err
	}
	if strings.Contains(final, "/recommended/") {
		f = postFields(doc, "ctl00$MasterBodyPlaceHolder$lnkChangeCarPc")
		data, doc, final, err = c.request(ctx, pathOf(final), f)
		if err != nil {
			return QuoteResult{}, err
		}
	}
	if !strings.Contains(final, "/index02.aspx") {
		return QuoteResult{}, &SourceError{"Toyota did not reach its dated class results; no inventory conclusion. Open " + pickup.URL}
	}
	if category != "compact" {
		data, doc, final, err = c.request(ctx, "/eng/reservation/index02.aspx?carType="+carType, nil)
		if err != nil {
			return QuoteResult{}, err
		}
	}
	if err := checkContext(doc, period, pickup, dropoff); err != nil {
		return QuoteResult{}, err
	}
	categoryToken := map[string]string{"compact": "compact", "standard": "standard", "minivan": "wagon", "suv": "suv"}[category]
	if !strings.Contains(strings.ToLower(textID(doc, "lblTypeName")), categoryToken) {
		return QuoteResult{}, &SourceError{"Toyota changed the requested category; no unrelated class offers returned"}
	}
	for id, want := range optionsWire(o) {
		if valueID(doc, id) != want {
			return QuoteResult{}, &SourceError{"Toyota changed requested option " + id + "; no quotes returned. Check options at " + pickup.URL}
		}
	}
	modelLimit := 3
	if details {
		modelLimit = 12
	}
	offers, err := ParseOffers(data, modelLimit)
	if err != nil {
		return QuoteResult{}, err
	}
	count := len(offers)
	if class != "" {
		matched := make([]ClassOffer, 0)
		for _, offer := range offers {
			if offer.Class == class {
				matched = append(matched, offer)
			}
		}
		if len(matched) == 0 {
			return QuoteResult{}, &NotFoundError{Message: fmt.Sprintf("class %s was not returned for category %s; use the matching --category", class, category)}
		}
		offers = matched
	}
	truncated := len(offers) > limit
	if len(offers) > limit {
		offers = offers[:limit]
	}
	return QuoteResult{Meta: c.Meta(), PickupShop: pickup, DropoffShop: dropoff, Period: period, Options: o, OperatingWindows: map[string]OperatingWindow{"pickup": pickupWindow, "dropoff": dropoffWindow}, Category: category, Offers: offers, SourceCount: count, Truncated: truncated,
		PriceAssumptions: []string{"Toyota's class-page Rental Price is a live tax-inclusive estimate, not a confirmed final booking total.",
			"This anonymous class-page search does not select or verify a driver/license profile; reconfirm the applicable rate in Toyota's booking flow.",
			"Search options above were echoed by Toyota. Specific child seats,ETC cards and other equipment require stock confirmation.",
			"Basic insurance is included in the basic rental rate; optional waiver/NOC,ETC/JAF,model selection and one-way inclusion are not inferred from this amount.",
			"Do not blindly add policy fees to the class estimate; some inclusions and seasonal tire rates require Toyota's final breakdown.",
			"Representative models are not guaranteed. Fuel,tolls,extensions and out-of-scope charges are not a quoted all-in cost."},
		BookingURL: pickup.URL, SourceURL: Origin + "/eng/reservation/index02.aspx?carType=" + carType}, nil
}

type OneWayResult struct {
	Meta              Metadata `json:"meta"`
	PickupShop        Shop     `json:"pickup_shop"`
	DropoffShop       Shop     `json:"dropoff_shop"`
	Family            string   `json:"vehicle_family"`
	Status            string   `json:"status"`
	FeeJPY            *int     `json:"source_surcharge_jpy"`
	TaxIncluded       bool     `json:"tax_included"`
	DatedAvailability string   `json:"dated_availability"`
	SourceURL         string   `json:"source_url"`
	Notes             []string `json:"notes"`
}

func (c *Client) OneWay(ctx context.Context, pickupID, dropoffID, family string) (OneWayResult, error) {
	class := "C1"
	if family == "wagon" {
		class = "W1"
	} else if family != "standard" {
		return OneWayResult{}, &InputError{"--family must be standard or wagon; buses are excluded from Toyota one-way rentals"}
	}
	if _, _, err := ParseShopID(pickupID); err != nil {
		return OneWayResult{}, err
	}
	if _, _, err := ParseShopID(dropoffID); err != nil {
		return OneWayResult{}, err
	}
	_, doc, _, err := c.request(ctx, OneWayPath, nil)
	if err != nil {
		return OneWayResult{}, err
	}
	_, _, _, err = c.request(ctx, OneWayPath, postFields(doc, "ctl00$MasterBodyPlaceHolder$btnSearchShop"))
	if err != nil {
		return OneWayResult{}, err
	}
	pickup, _, _, err := c.shop(ctx, pickupID, "/eng/shop/", false)
	if err != nil {
		return OneWayResult{}, err
	}
	dropoff, doc, final, err := c.shop(ctx, dropoffID, "/eng/shop/", true)
	if err != nil {
		// The return-mode list omits shops that prohibit one-way returns.
		// Resolve the normal shop card before treating that omission as unknown.
		if _, ok := err.(*NotFoundError); !ok {
			return OneWayResult{}, err
		}
		fallback, fallbackErr := c.GetShop(ctx, dropoffID)
		if fallbackErr != nil || fallback.OneWayReturns != "not_possible" {
			return OneWayResult{}, err
		}
		dropoff = fallback
	}
	result := OneWayResult{PickupShop: pickup, DropoffShop: dropoff, Family: family, Status: "unknown", TaxIncluded: true, DatedAvailability: "unknown", SourceURL: Origin + OneWayPath,
		Notes: []string{"Toyota's fee simulation is date-independent and does not guarantee dated class/route inventory.",
			"standard covers Standard/Special/Premium excluding LXC/LXP; wagon covers Wagon/SUV/Premium LXC/LXP.",
			"Apply for one-way use in advance. Source restrictions exclude buses,Hokkaido–other main islands,and Okinawa–other prefectures; shop/model exceptions apply."}}
	if pickupID != dropoffID && dropoff.OneWayReturns == "not_possible" {
		result.Status = "not_available"
		result.Notes = append(result.Notes, dropoff.OneWayNote)
		result.Meta = c.Meta()
		return result, nil
	}
	f := postFields(doc, "ctl00$MasterBodyPlaceHolder$OnewayPriceInfoUc$lnkBtnSearch")
	if err := fillShopPair(f, doc, pickup, dropoff); err != nil {
		return OneWayResult{}, err
	}
	_, doc, final, err = c.request(ctx, pathOf(final), f)
	if err != nil {
		return OneWayResult{}, err
	}
	if !strings.Contains(final, "/simulation.aspx") {
		return OneWayResult{}, &SourceError{"Toyota one-way shop selection did not reach the simulator"}
	}
	f = postFields(doc, "ctl00$MasterBodyPlaceHolder$lnkCalculationBotton")
	if err := setID(f, doc, "ddlCarType", class); err != nil {
		return OneWayResult{}, err
	}
	_, doc, _, err = c.request(ctx, OneWayPath, f)
	if err != nil {
		return OneWayResult{}, err
	}
	for id, expected := range map[string]struct{ prefix, name string }{
		"lblDepartShop": {"From ", pickup.Name}, "lblReturnShop": {"To ", dropoff.Name},
	} {
		label := textID(doc, id)
		if !strings.HasPrefix(label, expected.prefix) || !strings.HasSuffix(label, "Store") {
			return OneWayResult{}, &SourceError{"Toyota one-way calculator changed the shop pair; fee withheld"}
		}
		name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(label, expected.prefix), "Store"))
		want := strings.TrimSpace(strings.TrimSuffix(expected.name, " Shop"))
		if name != want {
			return OneWayResult{}, &SourceError{"Toyota one-way calculator changed the shop pair; fee withheld"}
		}
	}
	selectValues := successfulForm(doc)
	if selectValues.Get("ctl00$MasterBodyPlaceHolder$ddlCarType") != class {
		return OneWayResult{}, &SourceError{"Toyota one-way calculator changed the vehicle family; fee withheld"}
	}
	fee := integer(textID(doc, "lblOnewayPrice"))
	if fee == nil {
		return OneWayResult{}, &SourceError{"Toyota returned no numeric one-way fee; route may be unsupported. Check " + Origin + OneWayPath}
	}
	result.FeeJPY = fee
	result.Status = "calculated"
	result.Meta = c.Meta()
	return result, nil
}
