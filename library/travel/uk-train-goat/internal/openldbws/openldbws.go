// Package openldbws is the uk-train-goat adapter over the martinsirbe
// OpenLDBWS Go client. It centralizes Client construction, auth wiring,
// and option translation so individual CLI commands stay thin.
//
// The wrapper's types are re-exported as type aliases so command code
// only imports from this package; if the wrapper is ever swapped out,
// only this file changes.
package openldbws

import (
	"fmt"

	nr "github.com/martinsirbe/go-national-rail-client/nationalrail"
)

// Type aliases re-exported from the wrapper so CLI commands only import
// from internal/openldbws.
type (
	StationBoard        = nr.StationBoard
	TrainService        = nr.TrainService
	TrainServiceDetails = nr.TrainServiceDetails
	Location            = nr.Location
	CRSCode             = nr.CRSCode
)

// StationCodeToNameMap is the wrapper's full UK station enum re-exported
// so the local sync command can populate the SQLite store without an
// additional API call.
var StationCodeToNameMap = nr.StationCodeToNameMap

// Client wraps nr.Client with adapter-level concerns.
type Client struct {
	inner *nr.Client
	token string
}

// New returns a configured OpenLDBWS adapter. The token is passed
// explicitly via AccessTokenOpt so the wrapper's NR_ACCESS_TOKEN
// env-pickup is bypassed and irrelevant; uk-train-goat is canonical on
// LDBWS_API_TOKEN.
func New(token string) (*Client, error) {
	if token == "" {
		return nil, fmt.Errorf("OpenLDBWS access token required: set LDBWS_API_TOKEN or run `uk-train-goat-pp-cli auth set-token`. Register at https://realtime.nationalrail.co.uk/OpenLDBWSRegistration/")
	}
	c, err := nr.NewClient(nr.AccessTokenOpt(token))
	if err != nil {
		return nil, fmt.Errorf("constructing OpenLDBWS client: %w", err)
	}
	return &Client{inner: c, token: token}, nil
}

// Departures returns a departure board for the given CRS. Optional
// destination CRS narrows the board to services going to that station.
// numRows caps the results (1-150).
//
// pp:client-call — wraps OpenLDBWS GetDepartureBoard.
func (c *Client) Departures(crs string, dest string, numRows int, offsetMin int, windowMin int) (*StationBoard, error) {
	opts := buildOpts(dest, numRows, offsetMin, windowMin, true)
	return c.inner.GetDepartures(CRSCode(crs), opts...)
}

// DeparturesWithDetails fetches the same as Departures but with calling
// points populated for every service.
//
// pp:client-call — wraps OpenLDBWS GetDepartureBoardWithDetails.
func (c *Client) DeparturesWithDetails(crs string, dest string, numRows int, offsetMin int, windowMin int) (*StationBoard, error) {
	opts := buildOpts(dest, numRows, offsetMin, windowMin, true)
	return c.inner.GetDeparturesWithDetails(CRSCode(crs), opts...)
}

// Arrivals returns an arrivals board for the given CRS. Optional origin
// CRS narrows the board to services arriving from that station.
//
// pp:client-call — wraps OpenLDBWS GetArrivalBoard.
func (c *Client) Arrivals(crs string, origin string, numRows int, offsetMin int, windowMin int) (*StationBoard, error) {
	opts := buildOpts(origin, numRows, offsetMin, windowMin, false)
	return c.inner.GetArrivals(CRSCode(crs), opts...)
}

// Service fetches detailed status for a single service ID.
//
// pp:client-call — wraps OpenLDBWS GetServiceDetails.
func (c *Client) Service(serviceID string) (*TrainServiceDetails, error) {
	if serviceID == "" {
		return nil, fmt.Errorf("service ID required")
	}
	return c.inner.GetServiceDetails(serviceID)
}

// buildOpts assembles the wrapper RequestOption list. asDestination
// controls whether the filter CRS is treated as a destination
// (TerminatingAtOpt, used for departures) or an origin
// (OriginatingFromOpt, used for arrivals).
func buildOpts(filterCRS string, numRows, offsetMin, windowMin int, asDestination bool) []nr.RequestOption {
	var opts []nr.RequestOption
	if numRows > 0 {
		opts = append(opts, nr.NumRowsOpt(numRows))
	}
	if offsetMin > 0 {
		opts = append(opts, nr.TimeOffsetMinutesOpt(offsetMin))
	}
	if windowMin > 0 {
		opts = append(opts, nr.TimeWindowMinutesOpt(windowMin))
	}
	if filterCRS != "" {
		if asDestination {
			opts = append(opts, nr.TerminatingAtOpt(CRSCode(filterCRS)))
		} else {
			opts = append(opts, nr.OriginatingFromOpt(CRSCode(filterCRS)))
		}
	}
	return opts
}
