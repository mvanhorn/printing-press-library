package vrbo

type SearchParams struct {
	Location string
	Checkin  string
	Checkout string
	Adults   int
	Children int
	Page     string
	PageSize int
}

type GetParams struct {
	Checkin  string
	Checkout string
	Adults   int
	Children int
}

type Coord struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type PriceBreakdown struct {
	Currency string             `json:"currency,omitempty"`
	PerNight float64            `json:"per_night,omitempty"`
	Total    float64            `json:"total,omitempty"`
	Fees     map[string]float64 `json:"fees,omitempty"`
	Subtotal float64            `json:"subtotal,omitempty"`
	Raw      any                `json:"raw,omitempty"`
}

type Property struct {
	ID                     string          `json:"id,omitempty"`
	Name                   string          `json:"name,omitempty"`
	Title                  string          `json:"title,omitempty"`
	URL                    string          `json:"url,omitempty"`
	City                   string          `json:"city,omitempty"`
	Coordinate             *Coord          `json:"coordinate,omitempty"`
	AvgRatingValue         float64         `json:"avg_rating_value,omitempty"`
	ReviewCount            int             `json:"review_count,omitempty"`
	Amenities              []string        `json:"amenities,omitempty"`
	Description            string          `json:"description,omitempty"`
	HostName               string          `json:"host_name,omitempty"`
	HostBio                string          `json:"host_bio,omitempty"`
	PropertyManagementName string          `json:"property_management_name,omitempty"`
	Photos                 []string        `json:"photos,omitempty"`
	Beds                   int             `json:"beds,omitempty"`
	Baths                  float64         `json:"baths,omitempty"`
	SleepsMax              int             `json:"sleeps_max,omitempty"`
	PriceBreakdown         *PriceBreakdown `json:"price_breakdown,omitempty"`
	Raw                    any             `json:"raw,omitempty"`
}

type Pagination struct {
	StartingIndex int  `json:"starting_index,omitempty"`
	PageSize      int  `json:"page_size,omitempty"`
	HasNext       bool `json:"has_next,omitempty"`
}
