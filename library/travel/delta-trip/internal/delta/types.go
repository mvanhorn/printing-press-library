// Package delta provides types and browser-based scraping for delta.com My Trips.
package delta

// TripResult is the top-level result from a trip lookup.
type TripResult struct {
	ConfirmationNumber string    `json:"confirmationNumber"`
	Destination        string    `json:"destination,omitempty"`
	DepartureDate      string    `json:"departureDate,omitempty"`
	TripType           string    `json:"tripType,omitempty"`
	PassengerCount     int       `json:"passengerCount,omitempty"`
	TicketExpiration   string    `json:"ticketExpiration,omitempty"`
	StatusBadge        string    `json:"statusBadge,omitempty"`
	Alerts             []string  `json:"alerts,omitempty"`
	Flights            []*Flight `json:"flights"`
}

// Flight represents a single leg of an itinerary.
type Flight struct {
	FlightIndex  string          `json:"flightIndex"`  // "1 of 3"
	FlightNumber string          `json:"flightNumber"` // "DL5597"
	CarrierCode  string          `json:"carrierCode"`  // "DL"
	Aircraft     string          `json:"aircraft,omitempty"`
	OperatedBy   string          `json:"operatedBy,omitempty"`
	Status       string          `json:"status,omitempty"`
	Duration     string          `json:"duration,omitempty"`
	Departure    FlightStop      `json:"departure"`
	Arrival      FlightStop      `json:"arrival"`
	Layover      *Layover        `json:"layover,omitempty"`
	Passengers   []*PassengerSeg `json:"passengers"`
}

// FlightStop holds departure or arrival data for a flight leg.
type FlightStop struct {
	Time     string `json:"time"`
	Date     string `json:"date"`
	Airport  string `json:"airport"` // IATA code, e.g. "JAX"
	City     string `json:"city,omitempty"`
	Terminal string `json:"terminal,omitempty"`
	Gate     string `json:"gate,omitempty"`
}

// Layover describes a connection between two flight legs.
type Layover struct {
	Duration      string `json:"duration"` // "1h 21m"
	Airport       string `json:"airport"`  // IATA code
	City          string `json:"city,omitempty"`
	RiskLevel     string `json:"riskLevel,omitempty"`   // "OK", "TIGHT", "HIGH"
	RiskMinutes   int    `json:"riskMinutes,omitempty"` // connection window in minutes
	International bool   `json:"international,omitempty"`
}

// PassengerSeg is a per-passenger record for a single flight leg.
type PassengerSeg struct {
	Name         string   `json:"name"`
	Seat         string   `json:"seat"`                   // "15C" or "--"
	FareClass    string   `json:"fareClass"`              // "Delta Main Classic (Q)"
	FareCode     string   `json:"fareCode,omitempty"`     // "Q"
	ETicket      string   `json:"eTicket,omitempty"`      // "#0067000000001"
	LoyaltyTier  string   `json:"loyaltyTier,omitempty"`  // "SkyMiles Member"
	BoardingZone string   `json:"boardingZone,omitempty"` // "Zone 6 or 7"
	FareFeatures []string `json:"fareFeatures,omitempty"`
}

// SeatMapResult is the full seat availability map for one flight.
type SeatMapResult struct {
	ConfirmationNumber string          `json:"confirmationNumber"`
	FlightNumber       string          `json:"flightNumber,omitempty"`
	Aircraft           string          `json:"aircraft,omitempty"`
	Route              string          `json:"route,omitempty"`
	Cabins             []*SeatMapCabin `json:"cabins"`
	TotalSeats         int             `json:"totalSeats"`
	AvailableSeats     int             `json:"availableSeats"`
	OccupiedSeats      int             `json:"occupiedSeats"`
	BlockedSeats       int             `json:"blockedSeats,omitempty"`
}

// SeatMapCabin is one cabin section within the aircraft (e.g. "First Class", "Main Cabin").
type SeatMapCabin struct {
	Name string        `json:"name"`
	Rows []*SeatMapRow `json:"rows"`
}

// SeatMapRow is one row of seats within a cabin.
type SeatMapRow struct {
	Number   int            `json:"row"`
	Seats    []*SeatMapSeat `json:"seats"`
	ExitRow  bool           `json:"exitRow,omitempty"`
	Bulkhead bool           `json:"bulkhead,omitempty"`
}

// SeatMapSeat is a single seat with availability status.
type SeatMapSeat struct {
	Number  string `json:"number"`         // "15C"
	Status  string `json:"status"`         // "available", "occupied", "blocked", "your-seat"
	Type    string `json:"type,omitempty"` // "window", "middle", "aisle"
	ExitRow bool   `json:"exitRow,omitempty"`
}
