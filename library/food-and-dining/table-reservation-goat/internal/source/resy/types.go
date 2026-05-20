// Package resy wraps Resy's consumer surface. Unlike OpenTable and Tock, Resy
// has no Akamai/Cloudflare bot defense in front of its API — the underlying
// endpoints accept bearer-style headers from any client. Auth is the
// well-known public ResyAPI api_key + a per-user X-Resy-Auth-Token JWT
// obtained from email+password.
//
// PATCH: resy-source-port — see .printing-press-patches.json for the change-set rationale.

package resy

// PublicClientID is Resy's static public API key. Every resy.com browser ships
// it baked into the JS bundle; it is NOT a per-user secret. Every Resy OSS
// tool hardcodes the same value. Storing it in config.yaml as plaintext is
// appropriate.
const PublicClientID = "VbWk7s3L4KiK5fzlO7JD3Q5EYolJI7n5"

// Origin is the Resy consumer host. Used for URL minting only — API calls
// target ApiBase.
const Origin = "https://resy.com"

// ApiBase is the Resy public-API host. All endpoints in this package live
// underneath it.
const ApiBase = "https://api.resy.com"

// Venue is a search-result row.
type Venue struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	City         string  `json:"city,omitempty"`
	CityCode     string  `json:"city_code,omitempty"`
	Region       string  `json:"region,omitempty"`
	Slug         string  `json:"slug,omitempty"`
	URL          string  `json:"url,omitempty"`
	Latitude     float64 `json:"latitude,omitempty"`
	Longitude    float64 `json:"longitude,omitempty"`
	Neighborhood string  `json:"neighborhood,omitempty"`
}

// Slot is one open reservation slot returned by /4/find.
type Slot struct {
	// Token is the opaque book token (config_id in /3/details).
	Token string `json:"token"`
	// Time is the venue-local "HH:MM" extracted from the raw date string.
	Time string `json:"time"`
	// ConfigID, when present, is the slot's stable config identifier; it is
	// usually equal to Token but Resy occasionally returns them as distinct
	// values.
	ConfigID string `json:"config_id,omitempty"`
	// Type is the seating-area label ("Dining Room", "Bar", "Patio", etc.).
	Type string `json:"type,omitempty"`
	// PartySize, when present, is the maximum party Resy will quote for this
	// slot. Empty when the API omits the field.
	PartySize int `json:"party_size,omitempty"`
}

// UpcomingReservation is one row of /3/user/reservations after the parser
// has folded Resy's two payload shapes (modern bare time_slot + share-message
// venue name; legacy time_slot object + venue.name) into a single struct.
type UpcomingReservation struct {
	// ID is resy_token preferred, falling back to reservation_id stringified.
	ID        string `json:"id"`
	VenueID   string `json:"venue_id,omitempty"`
	VenueName string `json:"venue_name,omitempty"`
	Date      string `json:"date"` // YYYY-MM-DD
	Time      string `json:"time"` // HH:MM (24h, venue-local)
	PartySize int    `json:"party_size"`
	Status    string `json:"status,omitempty"` // "Completed", "Cancelled", etc.
}

// BookRequest is the input to the two-step Book() flow. SlotToken is the
// `token` value harvested from a prior Availability() call (== `config_id`
// for /3/details).
type BookRequest struct {
	VenueID   string
	Date      string // YYYY-MM-DD
	Time      string // HH:MM (24h)
	PartySize int
	SlotToken string
	// PaymentMethodID, when non-zero, overrides Resy's default-card selection.
	PaymentMethodID string
}

// LocationInput is the location signal a Resy client call accepts. City is
// Resy's two/three-letter city code ("ny", "sea", "la", "sf", "chi", ...) —
// the same value Resy's web UI passes to /3/venuesearch/search via the
// `city` body field. Lat/Lng anchor client-side geo-filtering of search
// results; the values are read off the venue rows in the response, not
// pushed up to Resy's gateway (Resy's gateway dropped support for the
// `location` body field in 2026 — rejecting it as "Unknown field." HTTP
// 400 — so lat/lng cannot be pre-filtered server-side and must be applied
// post-hoc). Callers construct this via cli.GeoContext.ForResy().
//
// PATCH: location-native-redesign + resy-source-port — typed projection
// of GeoContext mirroring opentable.LocationInput and tock.LocationInput.
type LocationInput struct {
	City string
	Lat  float64
	Lng  float64
}

// BookResponse carries the result of a successful /3/book.
type BookResponse struct {
	// ResyToken is Resy's reservation identifier. Both /3/cancel and the
	// upcoming-reservations row key on this value.
	ResyToken     string `json:"resy_token"`
	ReservationID string `json:"reservation_id,omitempty"`
	VenueName     string `json:"venue_name,omitempty"`
	Date          string `json:"date,omitempty"`
	Time          string `json:"time,omitempty"`
	PartySize     int    `json:"party_size,omitempty"`
}
