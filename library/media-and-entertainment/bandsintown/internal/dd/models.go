package dd

// Artist mirrors the Bandsintown ArtistData definition, normalised to Go-typed
// fields suitable for SQLite upsert and JSON output.
type Artist struct {
	Name               string `json:"name"`
	ID                 string `json:"id,omitempty"`
	MBID               string `json:"mbid,omitempty"`
	URL                string `json:"url,omitempty"`
	ImageURL           string `json:"image_url,omitempty"`
	ThumbURL           string `json:"thumb_url,omitempty"`
	FacebookPageURL    string `json:"facebook_page_url,omitempty"`
	TrackerCount       int    `json:"tracker_count"`
	UpcomingEventCount int    `json:"upcoming_event_count"`
}

// Event mirrors EventData with venue and offers flattened for query ergonomics.
type Event struct {
	ID           string   `json:"id"`
	ArtistName   string   `json:"artist_name"`
	Datetime     string   `json:"datetime"`
	Description  string   `json:"description,omitempty"`
	URL          string   `json:"url,omitempty"`
	OnSaleAt     string   `json:"on_sale_datetime,omitempty"`
	VenueName    string   `json:"venue_name,omitempty"`
	VenueCity    string   `json:"venue_city,omitempty"`
	VenueRegion  string   `json:"venue_region,omitempty"`
	VenueCountry string   `json:"venue_country,omitempty"`
	VenueLat     float64  `json:"venue_lat,omitempty"`
	VenueLng     float64  `json:"venue_lng,omitempty"`
	Lineup       []string `json:"lineup,omitempty"`
	Offers       []Offer  `json:"offers,omitempty"`
}

// Offer is a single ticket offer attached to an event.
type Offer struct {
	Type   string `json:"type"`
	Status string `json:"status"`
	URL    string `json:"url"`
}

// TrackedArtist is the watchlist row.
type TrackedArtist struct {
	Name    string `json:"name"`
	MBID    string `json:"mbid,omitempty"`
	Tier    string `json:"tier,omitempty"`
	AddedAt string `json:"added_at"`
}
