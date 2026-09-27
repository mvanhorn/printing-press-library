package ikyu

import (
	"context"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/ikyu/internal/cliutil"
	"strings"
	"time"
)

type connection[T any] struct {
	Edges []struct {
		Node T `json:"node"`
	} `json:"edges"`
	Total *int `json:"totalCount"`
}
type rawRating struct {
	Count        *int     `json:"count"`
	Average      *float64 `json:"average"`
	Equipment    *float64 `json:"accommodationEquipment"`
	Bath         *float64 `json:"bathroomSpring"`
	Service      *float64 `json:"customerService"`
	Meal         *float64 `json:"meal"`
	Room         *float64 `json:"roomAmenity"`
	Satisfaction *float64 `json:"satisfaction"`
}
type rawProperty struct {
	ID         string   `json:"accommodationId"`
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Area       string   `json:"areaName"`
	Address    string   `json:"address"`
	Latitude   *float64 `json:"latitude"`
	Longitude  *float64 `json:"longitude"`
	Attributes []struct {
		Available *bool     `json:"available"`
		Attribute Attribute `json:"attribute"`
	} `json:"attributes"`
	Notes        string    `json:"notes"`
	GeneralNotes string    `json:"generalNotes"`
	Rating       rawRating `json:"rating2"`
	Baths        []struct {
		Spring *bool `json:"springGround"`
		Code   struct {
			Name string `json:"name"`
		} `json:"code"`
		Note string `json:"note"`
	} `json:"baths"`
	Amount      *rawAmount `json:"amount2"`
	SearchRooms *struct {
		Rooms *connection[rawRoom] `json:"rooms"`
	} `json:"searchRooms2"`
	RoomPlan *rawRoomPlan `json:"roomPlan"`
}
type rawRoomPlan struct {
	Room     *rawRoom `json:"room"`
	Plan     *rawPlan `json:"plan"`
	Children struct {
		Children []struct {
			Kind   string `json:"kind"`
			Amount struct {
				Type   string `json:"type"`
				Rate   *int   `json:"rate"`
				Amount *int64 `json:"amount"`
			} `json:"amount"`
		} `json:"children"`
	} `json:"children"`
	Booking *struct {
		Amount *rawAmount `json:"amount"`
		Error  *string    `json:"error"`
	} `json:"booking"`
	Error string `json:"error"`
}
type rawRoom struct {
	CheckInFrom *string `json:"checkInTimeFrom"`
	CheckInTo   *string `json:"checkInTimeTo"`
	CheckOut    *string `json:"checkOutTime"`
	ID          string  `json:"roomId"`
	Name        string  `json:"name"`
	Type        struct {
		Code string `json:"code"`
		Name string `json:"name"`
	} `json:"type"`
	MeterFrom    *float64 `json:"meterFrom"`
	MeterTo      *float64 `json:"meterTo"`
	CapacityMin  *int     `json:"capacityMin"`
	CapacityMax  *int     `json:"capacityMax"`
	FloorPlan    string   `json:"floorPlan"`
	Presentation string   `json:"presentationText"`
	Beds         []struct {
		Count  *int `json:"count"`
		People *int `json:"peopleCount"`
		Width  *int `json:"width"`
		Length *int `json:"length"`
	} `json:"beds"`
	Attributes []Attribute            `json:"attributes"`
	Amounts    *connection[rawAmount] `json:"amounts"`
}
type rawPlan struct {
	Meals []struct {
		Type        *string `json:"type"`
		Name        *string `json:"name"`
		Place       *string `json:"place"`
		Menu        *string `json:"menu"`
		Description *string `json:"prText"`
		Notes       *string `json:"notes"`
		OpenAllDay  *bool   `json:"openAllDay"`
		StartTime   *string `json:"startTime"`
		EndTime     *string `json:"endTime"`
		LastOrder   *string `json:"lastOrderTime"`
	} `json:"meals"`
	CheckInFrom    *string `json:"checkInTimeFrom"`
	CheckInTo      *string `json:"checkInTimeTo"`
	CheckOut       *string `json:"checkOutTime"`
	UseCheckInOut  *bool   `json:"useCheckInOut"`
	PointVariation *int    `json:"uwanosePointVariation"`
	ID             string  `json:"planId"`
	Name           string  `json:"name"`
	Meal           Meal    `json:"meal"`
	Payment        string  `json:"cardSettlement"`
	Notes          string  `json:"notes"`
	Content        string  `json:"content"`
	MemberRank     *string `json:"memberRank"`
	BathTaxApply   *bool   `json:"bathTaxApply"`
	MinRooms       *int    `json:"minRoomCount"`
	MaxRooms       *int    `json:"maxRoomCount"`
	MinNights      *int    `json:"minLodgingCount"`
	MaxNights      *int    `json:"maxLodgingCount"`
	Cancel         *struct {
		ID    string `json:"id"`
		Rules []struct {
			Type        string  `json:"__typename"`
			Day         *int    `json:"day"`
			HourMinutes *string `json:"hourMinutes"`
			Amount      struct {
				Rate   *int   `json:"rate"`
				Amount *int64 `json:"amount"`
			} `json:"amount"`
		} `json:"rules"`
	} `json:"cancelPolicy"`
}
type rawAmount struct {
	Amount             *int64   `json:"amount"`
	BaseDiscountAmount *int64   `json:"baseDiscountAmount"`
	DiscountAmount     *int64   `json:"discountAmount"`
	DiscountAmountEarn *int64   `json:"discountAmountEarn"`
	WithoutCoupon      *int64   `json:"discountAmountWithoutCoupon"`
	Point              *int64   `json:"point"`
	PointRate          *float64 `json:"pointRate"`
	InstantPoint       *int64   `json:"instantPoint"`
	InstantPointRate   *float64 `json:"instantPointRate"`
	Inventory          *int     `json:"inventory"`
	Adults             *int     `json:"peopleCount"`
	Rooms              *int     `json:"roomCount"`
	Nights             *int     `json:"lodgingCount"`
	A                  *int     `json:"childACount"`
	B                  *int     `json:"childBCount"`
	C                  *int     `json:"childCCount"`
	D                  *int     `json:"childDCount"`
	E                  *int     `json:"childECount"`
	F                  *int     `json:"childFCount"`
	Details            []struct {
		Date string `json:"date"`
	} `json:"details"`
	Coupon *struct {
		Price *int64 `json:"price"`
	} `json:"coupon"`
	Plan *rawPlan `json:"plan"`
	Room *rawRoom `json:"room"`
}

const amountFields = `amount baseDiscountAmount discountAmount discountAmountEarn discountAmountWithoutCoupon point pointRate instantPoint instantPointRate inventory lodgingCount peopleCount roomCount childACount childBCount childCCount childDCount childECount childFCount details { date } coupon { price }`
const roomFields = `roomId name checkInTimeFrom checkInTimeTo checkOutTime type { code name } meterFrom meterTo capacityMin capacityMax floorPlan presentationText attributes { value name } beds { count peopleCount width length }`
const propertyFields = `accommodationId name type areaName address latitude longitude notes generalNotes attributes { available attribute { value name } } rating2 { count average accommodationEquipment bathroomSpring customerService meal roomAmenity satisfaction } baths { springGround code { name } note }`
const propertyQuery = `query StayProperty($id:AccommodationIdScalar!){accommodation(accommodationId:$id){` + propertyFields + `}}`
const roomsQuery = `query StayRooms($id:AccommodationIdScalar!,$input:SearchRoomsInput!,$amountInput:RoomAmountInput!,$first:Int!,$offset:Int!,$planFirst:Int!,$planOffset:Int!){accommodation(accommodationId:$id){accommodationId name searchRooms2(input:$input,first:$first,offset:$offset){rooms{totalCount edges{node{` + roomFields + ` amounts(input:$amountInput,first:$planFirst,offset:$planOffset){totalCount edges{node{` + amountFields + ` plan{planId name uwanosePointVariation meal{code name}}}}}}}}}}}`
const offerQuery = `query StayOffer($id:AccommodationIdScalar!,$room:RoomIdScalar!,$plan:PlanIdScalar!,$input:RoomPlanBookingInput!){accommodation(accommodationId:$id){accommodationId name type areaName generalNotes roomPlan(roomId:$room,planId:$plan){error room{` + roomFields + `} plan{planId name meal{code name} meals{type name prText place menu notes openAllDay startTime endTime lastOrderTime} checkInTimeFrom checkInTimeTo checkOutTime useCheckInOut cardSettlement notes content memberRank bathTaxApply minRoomCount maxRoomCount minLodgingCount maxLodgingCount uwanosePointVariation cancelPolicy{id rules{__typename ... on CancelPolicyRuleDay{day hourMinutes} amount{__typename ... on CancelPolicyRuleAmountRate{rate} ... on CancelPolicyRuleAmountFixed{amount}}}}} children{children{kind amount{type ... on RoomPlanChildAmountFixed{amount} ... on RoomPlanChildAmountRate{rate}}}} booking(input:$input){error amount{` + amountFields + `}}}}}`

func (c *Client) Property(ctx context.Context, id string) (PropertyResult, error) {
	if err := validateID("property ID", id); err != nil {
		return PropertyResult{}, err
	}
	var data struct {
		Accommodation *rawProperty `json:"accommodation"`
	}
	f, err := c.query(ctx, "StayProperty", propertyQuery, map[string]any{"id": id}, &data, staticTTL)
	if err != nil {
		return PropertyResult{}, err
	}
	p, err := normalizeProperty(data.Accommodation)
	if err != nil {
		return PropertyResult{}, err
	}
	if p.ID != id {
		return PropertyResult{}, &SchemaError{Message: "property ID changed"}
	}
	return PropertyResult{Data: p, Freshness: f}, nil
}
func (c *Client) Rooms(ctx context.Context, r RoomsRequest) (RoomsResult, error) {
	if err := validateID("property ID", r.PropertyID); err != nil {
		return RoomsResult{}, err
	}
	if err := ValidateStay(r.Stay, c.opts.Now()); err != nil {
		return RoomsResult{}, err
	}
	if err := bounds(r.Limit, r.Offset); err != nil {
		return RoomsResult{}, err
	}
	if err := validatePreferences(r.Preferences); err != nil {
		return RoomsResult{}, err
	}
	if r.PlanLimit == 0 {
		r.PlanLimit = 5
	}
	if err := bounds(r.PlanLimit, r.PlanOffset); err != nil {
		return RoomsResult{}, err
	}
	if r.Limit*r.PlanLimit > 100 {
		return RoomsResult{}, invalidf("--limit times --plan-limit must be at most 100; use room/plan offsets to page")
	}
	input := inputFor(r.Stay, r.Preferences)
	input["preferBookable"] = true
	amount := inputFor(r.Stay, r.Preferences)
	var data struct {
		Accommodation *rawProperty `json:"accommodation"`
	}
	f, err := c.query(ctx, "StayRooms", roomsQuery, map[string]any{"id": r.PropertyID, "input": input, "amountInput": amount, "first": r.Limit, "offset": r.Offset, "planFirst": r.PlanLimit, "planOffset": r.PlanOffset}, &data, availabilityTTL)
	if err != nil {
		return RoomsResult{}, err
	}
	p := data.Accommodation
	if p == nil || p.ID != r.PropertyID || p.Name == "" || p.SearchRooms == nil || p.SearchRooms.Rooms == nil || p.SearchRooms.Rooms.Total == nil {
		return RoomsResult{}, &SchemaError{Message: "missing room connection or property identity"}
	}
	conn := p.SearchRooms.Rooms
	out := RoomsResult{OccupancyEchoVerified: false, DateEchoVerified: false, Data: []Room{}, Stay: r.Stay, Freshness: f, Coverage: coverage(r.Preferences, "source room filters; budget, meal and size filter this returned page and bounded plan window")}
	out.Coverage.Note = "Bulk room prices echo occupancy but omit nightly dates; inspect exact stay offer before relying on price/conditions. Plan previews are paged separately with --plan-limit/--plan-offset."
	for _, edge := range conn.Edges {
		room, err := normalizeRoom(edge.Node, r.PropertyID, &r.Stay)
		if err != nil {
			return RoomsResult{}, err
		}
		if edge.Node.Amounts == nil || edge.Node.Amounts.Total == nil {
			return RoomsResult{}, &SchemaError{Message: "missing room plan amounts"}
		}
		room.PlanTotal = edge.Node.Amounts.Total
		room.Plans = []PlanSummary{}
		for _, e := range edge.Node.Amounts.Edges {
			a := e.Node
			if err := verifyAmount(a, r.Stay, false); err != nil {
				return RoomsResult{}, err
			}
			out.OccupancyEchoVerified = true
			if a.Plan == nil || a.Plan.ID == "" || cleanSourceText(a.Plan.Name) == "" {
				return RoomsResult{}, &SchemaError{Message: "missing plan identity"}
			}
			if err := validateID("source plan ID", a.Plan.ID); err != nil {
				return RoomsResult{}, &SchemaError{Message: err.Error()}
			}
			link, linkErr := CanonicalURL(r.PropertyID, room.ID, a.Plan.ID, &r.Stay)
			if linkErr != nil {
				return RoomsResult{}, &SchemaError{Message: linkErr.Error()}
			}
			plan := PlanSummary{PointVariation: a.Plan.PointVariation, ID: a.Plan.ID, Name: cleanSourceText(a.Plan.Name), Meal: cleanMeal(a.Plan.Meal), Price: normalizePrice(a), Inventory: a.Inventory, URL: link}
			if planMatches(plan, r.Preferences) {
				room.Plans = append(room.Plans, plan)
			}
		}
		room.PlanPagination = page(r.PlanLimit, r.PlanOffset, len(edge.Node.Amounts.Edges), len(room.Plans), *edge.Node.Amounts.Total)
		if roomMatches(room, r.Preferences) {
			out.Data = append(out.Data, room)
		}
	}
	out.Pagination = page(r.Limit, r.Offset, len(conn.Edges), len(out.Data), *conn.Total)
	if len(out.Data) == 0 {
		out.Coverage.Note += " No matches in this page and bounded plan window; use --offset/--plan-offset to inspect remaining source records."
	}
	return out, nil
}
func (c *Client) Offer(ctx context.Context, r OfferRequest) (OfferResult, error) {
	for label, id := range map[string]string{"property ID": r.PropertyID, "room ID": r.RoomID, "plan ID": r.PlanID} {
		if err := validateID(label, id); err != nil {
			return OfferResult{}, err
		}
	}
	if err := ValidateStay(r.Stay, c.opts.Now()); err != nil {
		return OfferResult{}, err
	}
	input := inputFor(r.Stay, Preferences{})
	delete(input, "sortItem")
	delete(input, "sortOrder")
	delete(input, "currency")
	input["searchType"] = "1"
	var data struct {
		Accommodation *rawProperty `json:"accommodation"`
	}
	f, err := c.query(ctx, "StayOffer", offerQuery, map[string]any{"id": r.PropertyID, "room": r.RoomID, "plan": r.PlanID, "input": input, "device": "PC"}, &data, availabilityTTL)
	if err != nil {
		return OfferResult{}, err
	}
	p, err := normalizeProperty(data.Accommodation)
	if err != nil {
		return OfferResult{}, err
	}
	if p.ID != r.PropertyID || data.Accommodation.RoomPlan == nil {
		return OfferResult{}, &SchemaError{Message: "missing requested room-plan"}
	}
	rp := data.Accommodation.RoomPlan
	if rp.Error != "" {
		return OfferResult{}, &SchemaError{Message: rp.Error}
	}
	if rp.Room == nil || rp.Plan == nil || rp.Booking == nil || rp.Booking.Error == nil {
		return OfferResult{}, &SchemaError{Message: "missing room-plan details/booking state"}
	}
	room, err := normalizeRoom(*rp.Room, r.PropertyID, &r.Stay)
	if err != nil {
		return OfferResult{}, err
	}
	if room.ID != r.RoomID || rp.Plan.ID != r.PlanID || rp.Plan.Name == "" {
		return OfferResult{}, &SchemaError{Message: "room or plan identity changed"}
	}
	plan := normalizePlan(*rp.Plan)
	for _, cp := range rp.Children.Children {
		plan.Children = append(plan.Children, ChildPrice{Kind: cp.Kind, Type: cp.Amount.Type, Rate: cp.Amount.Rate, Amount: cp.Amount.Amount})
	}
	link, _ := CanonicalURL(r.PropertyID, r.RoomID, r.PlanID, &r.Stay)
	offer := DatedOffer{Stay: r.Stay}
	if rp.Booking.Amount != nil && *rp.Booking.Error != "" {
		return OfferResult{}, &SchemaError{Message: *rp.Booking.Error}
	}
	if rp.Booking.Amount == nil {
		if *rp.Booking.Error == "" {
			return OfferResult{}, &SchemaError{Message: "null quote without unavailable reason"}
		}
		b := false
		offer.Available = &b
		offer.UnavailableReason = *rp.Booking.Error
	} else {
		a := *rp.Booking.Amount
		if err := verifyAmount(a, r.Stay, true); err != nil {
			return OfferResult{}, err
		}
		b := true
		if a.Inventory != nil && *a.Inventory < r.Stay.Rooms {
			b = false
		}
		offer.Available = &b
		offer.Inventory = a.Inventory
		offer.EchoStay = &r.Stay
		offer.DateVerified = true
		price := normalizePrice(a)
		offer.Price = &price
	}
	return OfferResult{Data: OfferData{Property: p, Room: room, Plan: plan, Offer: offer, URL: link}, Freshness: f}, nil
}
func normalizeProperty(r *rawProperty) (Property, error) {
	if r == nil || r.ID == "" || cleanSourceText(r.Name) == "" {
		return Property{}, &SchemaError{Message: "missing property ID/name"}
	}
	if err := validateID("source property ID", r.ID); err != nil {
		return Property{}, &SchemaError{Message: err.Error()}
	}
	link, _ := CanonicalURL(r.ID, "", "", nil)
	p := Property{ID: r.ID, Name: cleanSourceText(r.Name), Type: optionalText(r.Type), URL: link, Area: optionalText(r.Area), Address: optionalText(r.Address), Latitude: r.Latitude, Longitude: r.Longitude, Notes: optionalText(strings.TrimSpace(r.GeneralNotes + "\n" + r.Notes)), Scores: CategoryScores{Count: r.Rating.Count, Average: r.Rating.Average, Equipment: r.Rating.Equipment, Bath: r.Rating.Bath, CustomerService: r.Rating.Service, Meal: r.Rating.Meal, RoomAmenity: r.Rating.Room, Satisfaction: r.Rating.Satisfaction}}
	for _, a := range r.Attributes {
		x := a.Attribute
		x.Available = a.Available
		x.Name = cleanSourceText(x.Name)
		p.Attributes = append(p.Attributes, x)
	}
	for _, b := range r.Baths {
		p.BathFacilities = append(p.BathFacilities, BathFacility{Name: b.Code.Name, HotSpring: b.Spring, Note: b.Note})
	}
	if r.Amount != nil {
		price := normalizePrice(*r.Amount)
		p.Price = &price
	}
	return p, nil
}
func normalizeRoom(r rawRoom, id string, s *Stay) (Room, error) {
	if r.ID == "" || cleanSourceText(r.Name) == "" {
		return Room{}, &SchemaError{Message: "missing room ID/name"}
	}
	if err := validateID("source room ID", r.ID); err != nil {
		return Room{}, &SchemaError{Message: err.Error()}
	}
	link, _ := CanonicalURL(id, r.ID, "", s)
	room := Room{CheckInFrom: cleanPointer(r.CheckInFrom), CheckInTo: cleanPointer(r.CheckInTo), CheckOut: cleanPointer(r.CheckOut), ID: r.ID, PropertyID: id, Name: cleanSourceText(r.Name), URL: link, Type: optionalText(r.Type.Name), SizeM2: r.MeterFrom, SizeMinM2: r.MeterFrom, SizeMaxM2: r.MeterTo, CapacityMin: r.CapacityMin, CapacityMax: r.CapacityMax, Layout: optionalText(r.FloorPlan), Description: optionalText(r.Presentation), Attributes: cleanAttributes(r.Attributes), Bath: bathEvidence(r.Attributes)}
	if r.MeterFrom != nil {
		t := fmt.Sprintf("%g㎡", *r.MeterFrom)
		if r.MeterTo != nil && *r.MeterTo != *r.MeterFrom {
			t = fmt.Sprintf("%g–%g㎡", *r.MeterFrom, *r.MeterTo)
		}
		room.SizeText = &t
	}
	for _, b := range r.Beds {
		room.Beds = append(room.Beds, Bed{Count: b.Count, People: b.People, WidthCM: b.Width, LengthCM: b.Length})
	}
	if len(r.Beds) > 0 {
		t := cleanSourceText(r.Type.Name)
		room.BeddingText = &t
	}
	for _, line := range strings.FieldsFunc(r.Presentation, func(c rune) bool { return c == '\n' || c == '。' }) {
		if strings.Contains(line, "眺") || strings.Contains(line, "臨む") || strings.Contains(line, "見下ろ") || strings.Contains(line, "ビュー") {
			room.ViewEvidence = append(room.ViewEvidence, cleanSourceText(line))
		}
	}
	return room, nil
}
func bathEvidence(attrs []Attribute) BathEvidence {
	b := BathEvidence{}
	yes := true
	for _, a := range cleanAttributes(attrs) {
		switch a.Value {
		case "16":
			b.Private = &yes
			b.Outdoor = &yes
			b.HotSpring = &yes
			b.Proof = append(b.Proof, a)
		case "18":
			b.Private = &yes
			b.Outdoor = &yes
			b.Proof = append(b.Proof, a)
		case "36":
			b.Proof = append(b.Proof, a)
		}
		if strings.Contains(a.Name, "半露天") {
			b.SemiOutdoor = &yes
			b.Private = &yes
			b.Proof = append(b.Proof, a)
		}
	}
	return b
}
func normalizePlan(r rawPlan) Plan {
	p := Plan{CheckInFrom: cleanPointer(r.CheckInFrom), CheckInTo: cleanPointer(r.CheckInTo), CheckOut: cleanPointer(r.CheckOut), UseCheckInOut: r.UseCheckInOut, PointVariation: r.PointVariation, ID: r.ID, Name: cleanSourceText(r.Name), Meal: cleanMeal(r.Meal), Payment: optionalText(r.Payment), Notes: optionalText(r.Notes), Content: optionalText(r.Content), MemberRank: r.MemberRank, BathTaxApply: r.BathTaxApply, MinRooms: r.MinRooms, MaxRooms: r.MaxRooms, MinNights: r.MinNights, MaxNights: r.MaxNights}
	p.MealDetailsKnown = r.Meal.Code == "000"
	if p.MealDetailsKnown {
		p.MealDetails = []MealDetail{}
	}
	if len(r.Meals) > 0 {
		p.MealDetailsKnown = true
		for _, m := range r.Meals {
			entry := MealDetail{Type: cleanPointer(m.Type), Name: cleanPointer(m.Name), Place: cleanPointer(m.Place), Menu: cleanPointer(m.Menu), Description: cleanPointer(m.Description), Notes: cleanPointer(m.Notes), OpenAllDay: m.OpenAllDay, StartTime: cleanPointer(m.StartTime), EndTime: cleanPointer(m.EndTime), LastOrder: cleanPointer(m.LastOrder)}
			if entry.Type == nil || entry.Name == nil || entry.Place == nil || entry.Menu == nil {
				p.MealDetailsKnown = false
			}
			p.MealDetails = append(p.MealDetails, entry)
		}
	}
	if r.Cancel != nil {
		p.Cancellation = Cancellation{ID: r.Cancel.ID, Known: len(r.Cancel.Rules) > 0, Rules: []CancellationRule{}}
		for _, x := range r.Cancel.Rules {
			p.Cancellation.Rules = append(p.Cancellation.Rules, CancellationRule{Type: x.Type, Day: x.Day, HourMinutes: x.HourMinutes, Rate: x.Amount.Rate, Amount: x.Amount.Amount})
		}
	}
	return p
}
func normalizePrice(r rawAmount) Price {
	p := Price{HeadlineBeforePoints: r.BaseDiscountAmount, Scenarios: []PriceScenario{{Name: "earn_points", PublishedPayable: r.DiscountAmountEarn, PotentialPointsEarned: r.Point, EligibilityKnown: false}, {Name: "use_points_now", PublishedPayable: r.DiscountAmount, PotentialPointsApplied: r.InstantPoint, EligibilityKnown: false}}, Currency: "JPY", Unit: "booking_total", Amount: r.Amount, BaseDiscountAmount: r.BaseDiscountAmount, DiscountAmount: r.DiscountAmount, DiscountAmountEarn: r.DiscountAmountEarn, DiscountAmountWithoutCoupon: r.WithoutCoupon, Point: r.Point, PointRate: r.PointRate, InstantPoint: r.InstantPoint, InstantPointRate: r.InstantPointRate, EligibilityKnown: false, Assumptions: []string{"anonymous source quote; adults and A–F children are per room", "amounts cover all requested rooms and nights", "points-price eligibility and final taxes are not checkout-confirmed", "source_amount is original; headline_before_points is the published sale/earn-mode amount; earned/applied points are alternative source scenarios", "exact inspection supports upstream default point variation 0; listed variations remain distinct"}}
	if r.Coupon != nil {
		p.Coupon = &Coupon{DiscountAmount: r.Coupon.Price, EligibilityKnown: false}
	}
	return p
}
func verifyAmount(a rawAmount, s Stay, dates bool) error {
	if a.Amount == nil || a.Adults == nil || a.Rooms == nil || a.Nights == nil {
		return &SchemaError{Message: "missing amount/occupancy echo"}
	}
	if *a.Adults != s.Adults || *a.Rooms != s.Rooms || *a.Nights != nights(s) {
		return &SchemaError{Message: "source normalized adult/room/night inputs"}
	}
	for i, n := range []*int{a.A, a.B, a.C, a.D, a.E, a.F} {
		if n == nil || *n != s.Children[i] {
			return &SchemaError{Message: fmt.Sprintf("missing or changed child category %c", 'A'+i)}
		}
	}
	if dates {
		if len(a.Details) != nights(s) {
			return &SchemaError{Message: "missing nightly date echo"}
		}
		start, _ := time.Parse("2006-01-02", s.CheckIn)
		for i, d := range a.Details {
			if d.Date != start.AddDate(0, 0, i).Format("2006-01-02") {
				return &SchemaError{Message: "source quote date changed"}
			}
		}
	}
	return nil
}
func page(limit, offset, scanned, returned, total int) Pagination {
	next := offset + scanned
	var nextOffset *int
	if next < total {
		nextOffset = &next
	}
	return Pagination{NextOffset: nextOffset, Limit: limit, Offset: offset, Returned: returned, Total: total, HasNext: offset+scanned < total, Complete: offset == 0 && scanned >= total, Scanned: scanned}
}
func coverage(p Preferences, basis string) Coverage {
	c := Coverage{Applied: []string{}, Unknown: []string{}, Basis: basis}
	if p.MinBudget != nil || p.MaxBudget != nil {
		c.Applied = append(c.Applied, "budget: anonymous instant-points JPY booking total")
	}
	if len(p.Meals) > 0 {
		c.Applied = append(c.Applied, "meal")
	}
	if p.OutdoorBath {
		c.Applied = append(c.Applied, "room outdoor bath")
	}
	if p.HotSpringBath {
		c.Applied = append(c.Applied, "room hot-spring outdoor bath")
	}
	if p.Nonsmoking {
		c.Applied = append(c.Applied, "nonsmoking")
	}
	if p.MinSizeM2 != nil {
		c.Applied = append(c.Applied, "minimum room size")
	}
	return c
}
func priceMatches(p Price, prefs Preferences) bool {
	if prefs.MinBudget != nil || prefs.MaxBudget != nil {
		if p.DiscountAmount == nil {
			return false
		}
		if prefs.MinBudget != nil && *p.DiscountAmount < *prefs.MinBudget || prefs.MaxBudget != nil && *p.DiscountAmount > *prefs.MaxBudget {
			return false
		}
	}
	return true
}
func planMatches(plan PlanSummary, prefs Preferences) bool {
	meal := len(prefs.Meals) == 0
	for _, code := range prefs.Meals {
		meal = meal || code == plan.Meal.Code
	}
	return meal && priceMatches(plan.Price, prefs)
}
func roomMatches(r Room, p Preferences) bool {
	if p.MinSizeM2 != nil && (r.SizeMinM2 == nil || *r.SizeMinM2 < *p.MinSizeM2) {
		return false
	}
	if p.OutdoorBath && (r.Bath.Outdoor == nil || !*r.Bath.Outdoor) {
		return false
	}
	if p.HotSpringBath && (r.Bath.HotSpring == nil || !*r.Bath.HotSpring) {
		return false
	}
	if p.Nonsmoking {
		found := false
		for _, a := range r.Attributes {
			found = found || a.Value == "20"
		}
		if !found {
			return false
		}
	}
	if p.MinBudget != nil || p.MaxBudget != nil || len(p.Meals) > 0 {
		for _, a := range r.Plans {
			if planMatches(a, p) {
				return true
			}
		}
		return false
	}
	return true
}

func optionalText(s string) *string {
	s = cleanSourceText(s)
	if s == "" {
		return nil
	}
	return &s
}

func cleanSourceText(s string) string {
	s = cliutil.CleanText(s)
	return strings.Map(func(r rune) rune {
		if r == '\r' {
			return -1
		}
		if (r < 0x20 && r != '\n' && r != '\t') || (r >= 0x7f && r <= 0x9f) {
			return -1
		}
		return r
	}, s)
}
func cleanPointer(s *string) *string {
	if s == nil {
		return nil
	}
	return optionalText(*s)
}

func cleanAttributes(a []Attribute) []Attribute {
	out := make([]Attribute, len(a))
	for i, x := range a {
		x.Name = cleanSourceText(x.Name)
		out[i] = x
	}
	return out
}

func cleanMeal(m Meal) Meal {
	m.Name = cleanSourceText(m.Name)
	return m
}
