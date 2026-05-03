package airbnb

type SearchParams struct {
	Slug      string
	Location  string
	Checkin   string
	Checkout  string
	Adults    int
	Children  int
	Infants   int
	Pets      int
	MinPrice  int
	MaxPrice  int
	RoomTypes []string
	Cursor    string
	NE, SW    *Coord
}

type GetParams struct {
	Checkin  string
	Checkout string
	Adults   int
}

type Coord struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Bbox struct {
	NELat float64 `json:"ne_lat"`
	NELng float64 `json:"ne_lng"`
	SWLat float64 `json:"sw_lat"`
	SWLng float64 `json:"sw_lng"`
}

type PriceLine struct {
	Price           string  `json:"price,omitempty"`
	DiscountedPrice string  `json:"discounted_price,omitempty"`
	OriginalPrice   string  `json:"original_price,omitempty"`
	Qualifier       string  `json:"qualifier,omitempty"`
	Amount          float64 `json:"amount,omitempty"`
}

type PriceBreakdown struct {
	Currency string             `json:"currency,omitempty"`
	PerNight float64            `json:"per_night,omitempty"`
	Total    float64            `json:"total,omitempty"`
	Fees     map[string]float64 `json:"fees,omitempty"`
	Subtotal float64            `json:"subtotal,omitempty"`
	Raw      any                `json:"raw,omitempty"`
}

type Listing struct {
	ID                     string          `json:"id,omitempty"`
	Name                   string          `json:"name,omitempty"`
	Title                  string          `json:"title,omitempty"`
	URL                    string          `json:"url,omitempty"`
	City                   string          `json:"city,omitempty"`
	Region                 string          `json:"region,omitempty"`
	Coordinate             *Coord          `json:"coordinate,omitempty"`
	PrimaryLine            []string        `json:"primary_line,omitempty"`
	SecondaryLine          []string        `json:"secondary_line,omitempty"`
	AvgRatingLocalized     string          `json:"avg_rating_localized,omitempty"`
	ReviewsCount           int             `json:"reviews_count,omitempty"`
	PrimaryPrice           *PriceLine      `json:"primary_price,omitempty"`
	SecondaryPrice         *PriceLine      `json:"secondary_price,omitempty"`
	PriceTotal             float64         `json:"price_total,omitempty"`
	PerNightPrice          float64         `json:"per_night_price,omitempty"`
	Badges                 []string        `json:"badges,omitempty"`
	Amenities              []string        `json:"amenities,omitempty"`
	HouseRules             []string        `json:"house_rules,omitempty"`
	Highlights             []string        `json:"highlights,omitempty"`
	Description            string          `json:"description,omitempty"`
	Policies               []string        `json:"policies,omitempty"`
	HostName               string          `json:"host_name,omitempty"`
	HostBio                string          `json:"host_bio,omitempty"`
	Photos                 []string        `json:"photos,omitempty"`
	Beds                   int             `json:"beds,omitempty"`
	Baths                  float64         `json:"baths,omitempty"`
	SleepsMax              int             `json:"sleeps_max,omitempty"`
	PriceBreakdown         *PriceBreakdown `json:"price_breakdown,omitempty"`
	RawSections            map[string]any  `json:"raw_sections,omitempty"`
	PropertyManagementName string          `json:"property_management_name,omitempty"`
}

type Pagination struct {
	Cursors []string `json:"cursors,omitempty"`
	Next    string   `json:"next,omitempty"`
}

type Wishlist struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Count int    `json:"count,omitempty"`
	Raw   any    `json:"raw,omitempty"`
}

type WishlistItem struct {
	ListingID  string `json:"listing_id,omitempty"`
	WishlistID string `json:"wishlist_id,omitempty"`
	Title      string `json:"title,omitempty"`
	Raw        any    `json:"raw,omitempty"`
}
