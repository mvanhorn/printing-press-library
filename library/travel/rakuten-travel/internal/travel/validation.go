package travel

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var hotelIDPattern = regexp.MustCompile(`^[0-9]{1,12}$`)
var prefecturePattern = regexp.MustCompile(`^[a-z]{2,20}$`)
var areaPattern = regexp.MustCompile(`^[a-z]{2,20}/[A-Za-z0-9_]{1,20}$`)
var jst = time.FixedZone("JST", 9*60*60)

func queryError(message string) error {
	return &SourceError{Kind: "unsupported_query", Message: message}
}
func ValidateHotelID(id string) error {
	if !hotelIDPattern.MatchString(id) {
		return queryError("hotel ID must be a numeric source identifier")
	}
	return nil
}
func ValidateAreaParent(parent string) error {
	if parent != "" && !prefecturePattern.MatchString(parent) {
		return queryError("parent must be one source prefecture path segment")
	}
	return nil
}
func validateWindow(page, offset, limit int) error {
	if page < 0 || page > 100 || offset < 0 || offset > 10000 || limit < 0 || limit > 100 {
		return queryError("page must be 1–100, offset 0–10000 and limit 1–100 (zero selects defaults)")
	}
	return nil
}
func (q HotelQuery) Validate() error {
	if (strings.TrimSpace(q.Query) == "") == (q.Area == "") {
		return queryError("provide exactly one of query or area")
	}
	if len([]rune(q.Query)) > 100 || strings.ContainsAny(q.Query, "\x00\r\n") {
		return queryError("keyword must be at most 100 characters without control characters")
	}
	if q.Area != "" && !areaPattern.MatchString(q.Area) {
		return queryError("area must be a source path ID such as tokyo/E")
	}
	return validateWindow(q.Page, q.Offset, q.Limit)
}
func (q OfferQuery) Validate() error { return q.validateAt(time.Now()) }
func (q OfferQuery) validateAt(now time.Time) error {
	if err := ValidateHotelID(q.HotelID); err != nil {
		return err
	}
	in, err := time.ParseInLocation("2006-01-02", q.Checkin, jst)
	if err != nil || in.Format("2006-01-02") != q.Checkin {
		return queryError("checkin must be an explicit YYYY-MM-DD date")
	}
	out, err := time.ParseInLocation("2006-01-02", q.Checkout, jst)
	if err != nil || out.Format("2006-01-02") != q.Checkout {
		return queryError("checkout must be an explicit YYYY-MM-DD date")
	}
	nights := int(out.Sub(in) / (24 * time.Hour))
	if nights < 1 || nights > 28 {
		return queryError("stay must be 1–28 nights (conservative CLI bound)")
	}
	today := time.Date(now.In(jst).Year(), now.In(jst).Month(), now.In(jst).Day(), 0, 0, 0, 0, jst)
	if in.Before(today) || in.After(today.AddDate(1, 0, 0)) {
		return queryError("checkin must be today or within the next year in JST")
	}
	if q.Rooms < 1 || q.Rooms > 10 || q.AdultsPerRoom < 1 || q.AdultsPerRoom > 10 {
		return queryError("rooms and adults per room must be 1–10 (conservative CLI bounds)")
	}
	for _, count := range q.Children.values() {
		if count < 0 || count > 10 {
			return queryError("each child count per room must be 0–10 (conservative CLI bound)")
		}
	}
	return validateWindow(q.Page, q.Offset, q.Limit)
}
func (c Children) values() []int {
	return []int{c.Upper, c.Lower, c.InfantMealBed, c.InfantMeal, c.InfantBed, c.InfantNone}
}
func normalizedHotel(q HotelQuery) HotelQuery {
	if q.Page == 0 {
		q.Page = 1
	}
	if q.Limit == 0 {
		q.Limit = 5
	}
	return q
}
func normalizedOffer(q OfferQuery) OfferQuery {
	if q.Page == 0 {
		q.Page = 1
	}
	if q.Limit == 0 {
		q.Limit = 5
	}
	return q
}
func offerValues(q OfferQuery) url.Values {
	in, _ := time.Parse("2006-01-02", q.Checkin)
	out, _ := time.Parse("2006-01-02", q.Checkout)
	v := url.Values{"f_nen1": {strconv.Itoa(in.Year())}, "f_tuki1": {strconv.Itoa(int(in.Month()))}, "f_hi1": {strconv.Itoa(in.Day())}, "f_nen2": {strconv.Itoa(out.Year())}, "f_tuki2": {strconv.Itoa(int(out.Month()))}, "f_hi2": {strconv.Itoa(out.Day())}, "f_heya_su": {strconv.Itoa(q.Rooms)}, "f_otona_su": {strconv.Itoa(q.AdultsPerRoom)}, "f_flg": {"PLAN"}, "f_page_no": {strconv.Itoa(q.Page)}}
	for i, key := range []string{"f_s1", "f_s2", "f_y1", "f_y2", "f_y3", "f_y4"} {
		v.Set(key, strconv.Itoa(q.Children.values()[i]))
	}
	return v
}
func offerURL(q OfferQuery) string {
	return "https://hotel.travel.rakuten.co.jp/hotelinfo/plan/" + q.HotelID + "?" + offerValues(q).Encode()
}
func window(total, offset, limit int, sourceNext bool, page int, unit string, sourceItems, scanned int, sourceTotal *int) (int, int, PageInfo) {
	start := min(offset, total)
	end := min(start+limit, total)
	p := PageInfo{SourcePage: page, SourceUnit: unit, Offset: offset, Limit: limit, SourceItemsSeen: sourceItems, RowsSeen: total, RowsScanned: scanned, Emitted: end - start, SourceTotal: sourceTotal, Coverage: "source_page"}
	if end < total {
		p.HasMore = true
		p.NextPage = &page
		n := end
		p.NextOffset = &n
		p.Coverage = "within_source_page"
	} else if sourceNext {
		p.HasMore = true
		n := page + 1
		p.NextPage = &n
		zero := 0
		p.NextOffset = &zero
		p.Coverage = "source_page_with_more"
	}
	return start, end, p
}
func sourceError(kind, url string, code int, message string, cause error) error {
	return &SourceError{Kind: kind, URL: url, StatusCode: code, Message: message, Cause: cause}
}
func parseFailure(url, message string) error {
	return sourceError("parse_error", url, 0, fmt.Sprintf("Rakuten HTML: %s", message), nil)
}
