package ebay

import "time"

// Listing is a structured representation of an eBay search-result card.
type Listing struct {
	ItemID    string    `json:"item_id"`
	Title     string    `json:"title"`
	Price     float64   `json:"price"`
	Currency  string    `json:"currency"`
	Condition string    `json:"condition,omitempty"`
	Seller    string    `json:"seller,omitempty"`
	Bids      int       `json:"bids"`
	BestOffer bool      `json:"best_offer"`
	Auction   bool      `json:"auction"`
	BIN       bool      `json:"buy_it_now"`
	TimeLeft  string    `json:"time_left,omitempty"`
	EndsAt    time.Time `json:"ends_at,omitempty"`
	URL       string    `json:"url"`
	ImageURL  string    `json:"image_url,omitempty"`
	Location  string    `json:"location,omitempty"`
	Shipping  string    `json:"shipping,omitempty"`
}

// SoldItem is a structured representation of an eBay sold-listing card.
type SoldItem struct {
	ItemID    string    `json:"item_id"`
	Title     string    `json:"title"`
	SoldPrice float64   `json:"sold_price"`
	Currency  string    `json:"currency"`
	Condition string    `json:"condition,omitempty"`
	SoldDate  time.Time `json:"sold_date"`
	BestOffer bool      `json:"best_offer_accepted,omitempty"`
	URL       string    `json:"url"`
	ImageURL  string    `json:"image_url,omitempty"`
}

// CompStats is the analytics output of comp <query>.
type CompStats struct {
	Query        string     `json:"query"`
	WindowDays   int        `json:"window_days"`
	SampleSize   int        `json:"sample_size"`
	UsedSize     int        `json:"used_size"` // after outlier trim
	Mean         float64    `json:"mean"`
	Median       float64    `json:"median"`
	Min          float64    `json:"min"`
	Max          float64    `json:"max"`
	StdDev       float64    `json:"std_dev"`
	P25          float64    `json:"p25"`
	P75          float64    `json:"p75"`
	OutliersTrim int        `json:"outliers_trimmed"`
	FirstSold    time.Time  `json:"first_sold,omitempty"`
	LastSold     time.Time  `json:"last_sold,omitempty"`
	Currency     string     `json:"currency"`
	Items        []SoldItem `json:"items,omitempty"` // populated when --include-items
}

// SearchOptions filters an active-listings search.
type SearchOptions struct {
	Query      string
	Category   string
	MinPrice   float64
	MaxPrice   float64
	Auction    bool
	BIN        bool
	Sort       string // ending-soonest, newest, price-asc, price-desc
	PerPage    int
	Page       int
	Condition  string        // new, used, refurb
	HasBids    bool          // filter to listings with at least one bid
	MinBids    int           // minimum bid count
	MaxBids    int           // 0 = no max
	EndsWithin time.Duration // 0 = ignore
}

// SoldOptions filters sold-listings search.
type SoldOptions struct {
	Query      string
	Category   string
	MinPrice   float64
	MaxPrice   float64
	Condition  string
	PerPage    int
	Page       int
	WindowDays int // not eBay-side; affects local date filter only (defaults to 90)
}

// BidPlan describes a planned bid placement.
type BidPlan struct {
	ItemID      string  `json:"item_id"`
	MaxAmount   float64 `json:"max_amount"`
	Currency    string  `json:"currency"`
	LeadSeconds int     `json:"lead_seconds"`
	Group       string  `json:"group,omitempty"`
	Simulate    bool    `json:"simulate,omitempty"`
}

// BidResult is the outcome of a bid placement.
type BidResult struct {
	ItemID     string    `json:"item_id"`
	Amount     float64   `json:"amount"`
	Currency   string    `json:"currency"`
	Placed     bool      `json:"placed"`
	Status     string    `json:"status"` // accepted | rejected | dry-run | simulate
	AttemptID  string    `json:"attempt_id,omitempty"`
	PlacedAt   time.Time `json:"placed_at,omitempty"`
	Message    string    `json:"message,omitempty"`
	WaitedSecs int       `json:"waited_seconds,omitempty"`
	BidURL     string    `json:"bid_url,omitempty"`
}
