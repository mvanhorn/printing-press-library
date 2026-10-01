package tab

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/tokyo-art-beat/internal/cliutil"
	"math"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Name struct {
	EN *string `json:"en"`
	JA *string `json:"ja"`
}
type Ref struct {
	ID   string `json:"id"`
	Name Name   `json:"name"`
}
type Geo struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}
type Hours struct {
	Opens              *string  `json:"opens"`
	Closes             *string  `json:"closes"`
	ClosedDays         []string `json:"closed_days"`
	HiddenClosedDays   *bool    `json:"hidden_closed_days"`
	SpecialCases       Name     `json:"special_cases"`
	Additional         Name     `json:"additional"`
	HiddenSpecialCases *bool    `json:"hidden_special_cases"`
	LastAdmissionText  Name     `json:"last_admission_text"`
	LastAdmission      *string  `json:"last_admission"`
}
type Admission struct {
	Status                     string `json:"status"`
	Text                       Name   `json:"text"`
	Coupon                     *bool  `json:"member_discount_listed"`
	DiscountRequiresMembership bool   `json:"discount_requires_membership"`
}
type Venue struct {
	ID            string     `json:"id"`
	Name          Name       `json:"name"`
	URL           *string    `json:"url"`
	URLJA         *string    `json:"url_ja"`
	Area          *Ref       `json:"area"`
	Type          *Ref       `json:"type"`
	Address       Name       `json:"address"`
	Coordinates   *Geo       `json:"coordinates"`
	Status        *string    `json:"source_status"`
	UpdatedAt     *string    `json:"updated_at"`
	Hours         *Hours     `json:"hours,omitempty"`
	Admission     *Admission `json:"admission,omitempty"`
	Access        *Name      `json:"access,omitempty"`
	OfficialLinks *Name      `json:"official_links,omitempty"`
	Description   *Name      `json:"description,omitempty"`
}
type Event struct {
	ID               string     `json:"id"`
	Name             Name       `json:"name"`
	URL              *string    `json:"url"`
	URLJA            *string    `json:"url_ja"`
	RawStarts        *string    `json:"source_starts"`
	RawEnds          *string    `json:"source_ends"`
	StartYear        *int       `json:"start_year"`
	ArchiveYear      *int       `json:"archive_year"`
	Starts           *string    `json:"starts"`
	Ends             *string    `json:"ends"`
	EndUnconfirmed   *bool      `json:"end_unconfirmed"`
	Permanent        *bool      `json:"permanent"`
	SpanStatus       string     `json:"span_status"`
	Artists          Name       `json:"artists"`
	Venue            *Venue     `json:"venue"`
	Categories       []Ref      `json:"categories"`
	UpdatedAt        *string    `json:"updated_at"`
	Hours            *Hours     `json:"event_hours,omitempty"`
	Admission        *Admission `json:"admission,omitempty"`
	Reservation      any        `json:"reservation,omitempty"`
	ReservationNotes *Name      `json:"reservation_notes,omitempty"`
	OfficialLinks    *Name      `json:"official_links,omitempty"`
	TicketLinks      []string   `json:"ticket_links,omitempty"`
	Description      *Name      `json:"description,omitempty"`
	Day              *Day       `json:"day,omitempty"`
	DistanceKM       *float64   `json:"distance_km,omitempty"`
}
type Day struct {
	Date        string   `json:"date"`
	Status      string   `json:"status"`
	Reasons     []string `json:"reasons"`
	HoursSource string   `json:"hours_source"`
}

var jst = time.FixedZone("Asia/Tokyo", 9*3600)

func Today() string { return time.Now().In(jst).Format("2006-01-02") }
func value(e Entry, k string) any {
	m := e.Fields[k]
	for _, l := range []string{"en-US", "ja-JP", "en", "ja"} {
		if x, ok := m[l]; ok {
			return x
		}
	}
	return nil
}
func text(e Entry, k string) *string {
	s, ok := value(e, k).(string)
	if !ok || strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}
func boolean(e Entry, k string) *bool {
	x, ok := value(e, k).(bool)
	if !ok {
		return nil
	}
	return &x
}
func names(e Entry, k string) Name {
	m := e.Fields[k]
	var out Name
	for _, p := range []struct {
		key  string
		dest **string
	}{{"en-US", &out.EN}, {"ja-JP", &out.JA}} {
		if s, ok := m[p.key].(string); ok && strings.TrimSpace(s) != "" {
			s = clean(s)
			*p.dest = &s
		}
	}
	return out
}
func clean(s string) string {
	s = strings.ReplaceAll(s, "<br />", "\n")
	s = strings.ReplaceAll(s, "<br>", "\n")
	return cliutil.CleanText(s)
}
func linkID(x any) string {
	m, _ := x.(map[string]any)
	sys, _ := m["sys"].(map[string]any)
	s, _ := sys["id"].(string)
	return s
}
func linkedIDs(e Entry, k string) []string {
	a, _ := value(e, k).([]any)
	out := []string{}
	for _, x := range a {
		if s := linkID(x); s != "" {
			out = append(out, s)
		}
	}
	return out
}
func days(e Entry) []string {
	a, ok := value(e, "closedDays").([]any)
	if !ok {
		return nil
	}
	out := []string{}
	for _, x := range a {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
func str(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
func ref(e Entry) Ref { return Ref{e.Sys.ID, names(e, "name")} }
func index(es []Entry) map[string]Entry {
	out := map[string]Entry{}
	for _, e := range es {
		out[e.Sys.ID] = e
	}
	return out
}
func reference(id string, idx map[string]Entry) *Ref {
	if id == "" {
		return nil
	}
	if e, ok := idx[id]; ok {
		r := ref(e)
		return &r
	}
	return &Ref{ID: id}
}
func hours(e Entry) Hours {
	return Hours{Opens: text(e, "openingHoursOpens"), Closes: text(e, "openingHoursCloses"), ClosedDays: days(e), HiddenClosedDays: boolean(e, "hideClosedDays"), SpecialCases: names(e, "scheduleSpecialCases"), Additional: names(e, "additionalInfoOnOpeningHoursdays"), HiddenSpecialCases: boolean(e, "hideScheduleSpecialCases"), LastAdmissionText: lastAdmissionText(e), LastAdmission: nil}
}
func sourceURL(kind string, e Entry) *string {
	slug := text(e, "slug")
	if slug == nil {
		return nil
	}
	parts := strings.Split(*slug, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return str("https://www.tokyoartbeat.com/en/" + kind + "/-/" + strings.Join(parts, "/"))
}
func makeVenue(e Entry, idx map[string]Entry, detail bool) Venue {
	v := Venue{URLJA: jpURL(sourceURL("venues", e)), ID: e.Sys.ID, Name: names(e, "fullName"), URL: sourceURL("venues", e), Area: reference(linkID(value(e, "localArea")), idx), Type: reference(linkID(value(e, "venueType")), idx), Address: names(e, "address"), Status: text(e, "venueStatus"), UpdatedAt: str(e.Sys.UpdatedAt)}
	if x, ok := value(e, "geoInfo").(map[string]any); ok {
		lat, lok := x["lat"].(float64)
		lon, ok := x["lon"].(float64)
		if lok && ok && lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180 {
			v.Coordinates = &Geo{lat, lon}
		}
	}
	if detail {
		h := hours(e)
		a := admission(names(e, "admissionFee"), nil)
		access := names(e, "howtoAccess")
		links := safeNames(names(e, "homePage"))
		desc := names(e, "description")
		v.Hours = &h
		v.Admission = &a
		v.Access = &access
		v.OfficialLinks = &links
		v.Description = &desc
	}
	return v
}
func makeEvent(e Entry, vs map[string]Venue, cats map[string]Entry, today string, detail bool) Event {
	out := Event{URLJA: jpURL(sourceURL("events", e)), RawStarts: text(e, "scheduleStartsOn"), RawEnds: text(e, "scheduleEndsOn"), ID: e.Sys.ID, Name: names(e, "eventName"), URL: sourceURL("events", e), Starts: sourceDate(text(e, "scheduleStartsOn")), Ends: sourceDate(text(e, "scheduleEndsOn")), EndUnconfirmed: boolean(e, "scheduleEndDateUnfix"), Permanent: boolean(e, "permanentShow"), Artists: names(e, "artists"), Categories: []Ref{}, UpdatedAt: str(e.Sys.UpdatedAt)}
	if out.Starts != nil && len(*out.Starts) >= 4 {
		var y int
		if _, err := fmt.Sscanf((*out.Starts)[:4], "%d", &y); err == nil {
			out.StartYear = &y
		}
	}
	out.ArchiveYear = editionYear(e)
	out.SpanStatus = SpanStatus(out.Starts, out.Ends, out.EndUnconfirmed, today)
	id := linkID(value(e, "venue"))
	if v, ok := vs[id]; ok {
		out.Venue = &v
	} else if id != "" {
		out.Venue = &Venue{ID: id}
	}
	for _, id := range linkedIDs(e, "categories") {
		r := reference(id, cats)
		out.Categories = append(out.Categories, *r)
	}
	if detail {
		h := hours(e)
		a := admission(names(e, "fee"), boolean(e, "coupon"))
		desc := names(e, "description")
		links := safeNames(names(e, "showsWebpage"))
		notes := names(e, "scheduleSpecialCases")
		out.Hours = &h
		out.Admission = &a
		out.Description = &desc
		out.OfficialLinks = &links
		if m := e.Fields["reservation"]; len(m) > 0 {
			out.Reservation = m
		}
		if out.Reservation == nil {
			out.Reservation = map[string]any{"status": "unknown"}
		}
		out.ReservationNotes = &notes
		out.TicketLinks = []string{}
		for _, key := range []string{"ticketUrl", "ticketURL", "ticketWebpage"} {
			if s := text(e, key); s != nil && safeURL(*s) {
				out.TicketLinks = append(out.TicketLinks, *s)
			}
		}
	}
	return out
}
func safeURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" && u.User == nil
}
func safeNames(n Name) Name {
	if n.EN != nil && !safeURL(*n.EN) {
		n.EN = nil
	}
	if n.JA != nil && !safeURL(*n.JA) {
		n.JA = nil
	}
	return n
}

var price = regexp.MustCompile(`(?i)(?:[¥￥]\s*[1-9][0-9,]*|[1-9][0-9,]*\s*(?:円|yen))`)

func admission(n Name, coupon *bool) Admission {
	a := Admission{Status: "unknown", Text: n, Coupon: coupon, DiscountRequiresMembership: coupon != nil && *coupon}
	s := ""
	if n.EN != nil {
		s += *n.EN
	}
	if n.JA != nil {
		s += " " + *n.JA
	}
	lower := strings.ToLower(strings.TrimSpace(s))
	if price.MatchString(s) {
		a.Status = "priced"
	} else if lower == "free" || lower == "free admission" || strings.TrimSpace(s) == "無料" || strings.TrimSpace(s) == "入場無料" || lower == "free 無料" || lower == "free admission 無料" {
		a.Status = "free"
	}
	return a
}
func ParseDate(s string) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", s, jst)
	if err != nil || t.Format("2006-01-02") != s {
		return time.Time{}, Fail("invalid_date", fmt.Sprintf("%q must be an ISO date YYYY-MM-DD with a year", s), 2)
	}
	return t, nil
}
func SpanStatus(start, end *string, unconfirmed *bool, today string) string {
	if _, err := ParseDate(today); err != nil {
		return "unknown"
	}
	if start != nil && end != nil && *end < *start {
		return "unknown"
	}
	if start != nil {
		if _, err := ParseDate(*start); err != nil {
			return "unknown"
		}
		if *start > today {
			return "upcoming"
		}
	}
	if end != nil {
		if _, err := ParseDate(*end); err != nil {
			return "unknown"
		}
		if unconfirmed != nil && *unconfirmed {
			return "end_unconfirmed"
		}
		if *end < today {
			return "archived"
		}
	}
	if start == nil || end == nil {
		return "unknown"
	}
	if *end < *start {
		return "unknown"
	}
	return "in_span"
}
func anyNotes(h Hours) bool {
	return h.SpecialCases.EN != nil || h.SpecialCases.JA != nil || h.Additional.EN != nil || h.Additional.JA != nil
}
func AssessDay(e Event, date string) (Day, error) {
	d := Day{Date: date, Status: "unknown", Reasons: []string{}, HoursSource: "unknown"}
	t, err := ParseDate(date)
	if err != nil {
		return d, err
	}
	if e.Starts != nil {
		if _, err = ParseDate(*e.Starts); err != nil {
			return d, nil
		}
		if date < *e.Starts {
			d.Status = "outside_span"
			d.Reasons = append(d.Reasons, "Before event start date")
			return d, nil
		}
	}
	if e.Ends != nil && !(e.EndUnconfirmed != nil && *e.EndUnconfirmed) {
		if _, err = ParseDate(*e.Ends); err != nil {
			return d, nil
		}
		if date > *e.Ends {
			d.Status = "outside_span"
			d.Reasons = append(d.Reasons, "After event end date")
			return d, nil
		}
	}
	if e.Starts == nil || e.Ends == nil || *e.Ends < *e.Starts || (e.EndUnconfirmed != nil && *e.EndUnconfirmed) {
		d.Reasons = append(d.Reasons, "Overall date boundary is incomplete or unconfirmed")
		return d, nil
	}
	var eh, vh Hours
	if e.Hours != nil {
		eh = *e.Hours
	}
	if e.Venue != nil && e.Venue.Hours != nil {
		vh = *e.Venue.Hours
	}
	if e.Venue != nil && e.Venue.Status != nil && (strings.Contains(strings.ToLower(*e.Venue.Status), "closed") || strings.Contains(strings.ToLower(*e.Venue.Status), "close")) {
		d.Reasons = append(d.Reasons, "Venue source status: "+*e.Venue.Status)
		return d, nil
	}
	if (eh.HiddenClosedDays != nil && *eh.HiddenClosedDays) || (eh.HiddenSpecialCases != nil && *eh.HiddenSpecialCases) {
		d.Reasons = append(d.Reasons, "Source hides event closure information")
		return d, nil
	}
	closed := eh.ClosedDays
	d.HoursSource = "event"
	if closed == nil {
		closed = vh.ClosedDays
		d.HoursSource = "venue"
	}
	if anyNotes(eh) || anyNotes(vh) || (vh.HiddenClosedDays != nil && *vh.HiddenClosedDays) || (vh.HiddenSpecialCases != nil && *vh.HiddenSpecialCases) {
		d.Reasons = append(d.Reasons, "Source has exceptional schedule notes; confirm on official site")
		return d, nil
	}
	for _, s := range closed {
		if strings.EqualFold(s, "Holidays") || strings.EqualFold(s, "Irregular") {
			d.Reasons = append(d.Reasons, "Holiday or irregular closures require official confirmation")
			return d, nil
		}
	}
	for _, s := range closed {
		if strings.EqualFold(s, t.Weekday().String()) {
			d.Status = "weekly_closure"
			d.Reasons = append(d.Reasons, "Listed weekly closure: "+s)
			return d, nil
		}
	}
	if closed == nil {
		d.Reasons = append(d.Reasons, "Weekly closure information is missing")
		return d, nil
	}
	d.Status = "no_listed_weekly_closure"
	d.Reasons = append(d.Reasons, "Within date span; this is not confirmation of opening or ticket availability")
	return d, nil
}
func Distance(a, b Geo) float64 {
	r := math.Pi / 180
	dl := (b.Lat - a.Lat) * r
	dn := (b.Lon - a.Lon) * r
	h := math.Sin(dl/2)*math.Sin(dl/2) + math.Cos(a.Lat*r)*math.Cos(b.Lat*r)*math.Sin(dn/2)*math.Sin(dn/2)
	return 6371 * 2 * math.Atan2(math.Sqrt(h), math.Sqrt(math.Max(0, 1-h)))
}

func jpURL(s *string) *string {
	if s == nil {
		return nil
	}
	return str(strings.Replace(*s, ".com/en/", ".com/", 1))
}
func sourceDate(s *string) *string {
	if s == nil {
		return nil
	}
	if _, err := ParseDate(*s); err == nil {
		return s
	}
	if t, err := time.Parse(time.RFC3339, *s); err == nil {
		return str(t.Format("2006-01-02"))
	}
	return nil
}

var lastAdmissionPattern = regexp.MustCompile(`(?i)(?:[^.。\n]*(?:last (?:admission|entry)|最終入場|入館は|入場は)[^.。\n]*[.。]?)`)

func lastAdmissionText(e Entry) Name {
	out := Name{}
	for _, k := range []string{"scheduleSpecialCases", "additionalInfoOnOpeningHoursdays"} {
		n := names(e, k)
		for _, x := range []struct {
			src  *string
			dest **string
		}{{n.EN, &out.EN}, {n.JA, &out.JA}} {
			if x.src != nil {
				if m := lastAdmissionPattern.FindString(*x.src); m != "" {
					*x.dest = str(strings.TrimSpace(m))
				}
			}
		}
	}
	return out
}

var editionYearPattern = regexp.MustCompile(`^(?:import_event_record__)?([0-9]{4})(?:/|_)`)

func editionYear(e Entry) *int {
	for _, s := range []string{e.Sys.ID, func() string {
		if p := text(e, "slug"); p != nil {
			return *p
		}
		return ""
	}()} {
		if m := editionYearPattern.FindStringSubmatch(s); len(m) == 2 {
			var y int
			if _, err := fmt.Sscanf(m[1], "%d", &y); err == nil {
				return &y
			}
		}
	}
	return nil
}
