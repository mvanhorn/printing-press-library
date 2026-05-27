// Novel command: unified types for fan-out search across all reno-goat sources.

package cli

// NormalizedProduct is the unified product representation that all per-source
// normalizers convert into. Field names mirror the JSON tags so agent
// consumers (--json, --agent) see a stable contract regardless of which
// upstream source produced the row.
type NormalizedProduct struct {
	ID              string  `json:"id"`
	Source          string  `json:"source"`
	Title           string  `json:"title"`
	Brand           string  `json:"brand,omitempty"`
	URL             string  `json:"url"`
	ImageURL        string  `json:"image_url,omitempty"`
	Description     string  `json:"description,omitempty"`
	PriceMin        float64 `json:"price_min"`
	PriceMax        float64 `json:"price_max"`
	RegularPriceMin float64 `json:"regular_price_min,omitempty"`
	RegularPriceMax float64 `json:"regular_price_max,omitempty"`
	SalePriceMin    float64 `json:"sale_price_min,omitempty"`
	SalePriceMax    float64 `json:"sale_price_max,omitempty"`
	OnSale          bool    `json:"on_sale,omitempty"`
	DiscountPercent float64 `json:"discount_percent,omitempty"`
	Rating          float64 `json:"rating,omitempty"`
	ReviewCount     int     `json:"review_count,omitempty"`
	Category        string  `json:"category,omitempty"`
}

// FanoutResult is the top-level envelope for a fan-out search. It captures
// the merged product list plus metadata about which sources participated and
// which failed (partial-failure tolerance).
type FanoutResult struct {
	Query          string              `json:"query"`
	TotalResults   int                 `json:"total_results"`
	SourcesQueried []string            `json:"sources_queried"`
	SourcesFailed  []string            `json:"sources_failed,omitempty"`
	Products       []NormalizedProduct `json:"products"`
	Categories     []string            `json:"categories,omitempty"`
	Room           string              `json:"room,omitempty"`
}
