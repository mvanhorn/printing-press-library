package fares

import (
	"archive/zip"
	"fmt"
	"path/filepath"
	"strings"
)

// ParseFeedZip opens an RJFAF feed zip and parses the files v1 uses into a FeedData.
// Unrecognised entries are ignored. The LOC entry is parsed twice (locations + group members).
func ParseFeedZip(zipPath string) (*FeedData, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("fares: ParseFeedZip: open zip: %w", err)
	}
	defer r.Close()

	var data FeedData
	for _, f := range r.File {
		ext := strings.ToUpper(filepath.Ext(f.Name))
		switch ext {
		case ".LOC":
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("fares: ParseFeedZip: %s: %w", f.Name, err)
			}
			locs, err := ParseLOC(rc)
			rc.Close()
			if err != nil {
				return nil, fmt.Errorf("fares: ParseFeedZip: %s: %w", f.Name, err)
			}
			data.Locations = locs

			rc2, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("fares: ParseFeedZip: %s (group members): %w", f.Name, err)
			}
			members, err := ParseLOCGroupMembers(rc2)
			rc2.Close()
			if err != nil {
				return nil, fmt.Errorf("fares: ParseFeedZip: %s (group members): %w", f.Name, err)
			}
			data.GroupMembers = members

		case ".FFL":
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("fares: ParseFeedZip: %s: %w", f.Name, err)
			}
			flows, fares, err := ParseFFL(rc)
			rc.Close()
			if err != nil {
				return nil, fmt.Errorf("fares: ParseFeedZip: %s: %w", f.Name, err)
			}
			data.Flows = flows
			data.Fares = fares

		case ".FSC":
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("fares: ParseFeedZip: %s: %w", f.Name, err)
			}
			clusters, err := ParseFSC(rc)
			rc.Close()
			if err != nil {
				return nil, fmt.Errorf("fares: ParseFeedZip: %s: %w", f.Name, err)
			}
			data.Clusters = clusters

		case ".NFO":
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("fares: ParseFeedZip: %s: %w", f.Name, err)
			}
			ndf, err := ParseNFO(rc)
			rc.Close()
			if err != nil {
				return nil, fmt.Errorf("fares: ParseFeedZip: %s: %w", f.Name, err)
			}
			data.NDF = ndf

		case ".TTY":
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("fares: ParseFeedZip: %s: %w", f.Name, err)
			}
			tickets, err := ParseTTY(rc)
			rc.Close()
			if err != nil {
				return nil, fmt.Errorf("fares: ParseFeedZip: %s: %w", f.Name, err)
			}
			data.Tickets = tickets

		case ".RLC":
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("fares: ParseFeedZip: %s: %w", f.Name, err)
			}
			railcards, err := ParseRLC(rc)
			rc.Close()
			if err != nil {
				return nil, fmt.Errorf("fares: ParseFeedZip: %s: %w", f.Name, err)
			}
			data.Railcards = railcards

		case ".RST":
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("fares: ParseFeedZip: %s: %w", f.Name, err)
			}
			restrictions, err := ParseRSTHeaders(rc)
			rc.Close()
			if err != nil {
				return nil, fmt.Errorf("fares: ParseFeedZip: %s: %w", f.Name, err)
			}
			data.Restrictions = restrictions

		default:
			// unknown extension: skip
		}
	}
	if len(data.Locations) == 0 || len(data.Flows) == 0 || len(data.Fares) == 0 || len(data.Tickets) == 0 {
		return nil, fmt.Errorf(
			"fares: ParseFeedZip: archive has no usable core fare data (locations=%d, flows=%d, fares=%d, tickets=%d)",
			len(data.Locations), len(data.Flows), len(data.Fares), len(data.Tickets),
		)
	}
	flows := make(map[string]bool, len(data.Flows))
	for _, flow := range data.Flows {
		flows[flow.FlowID] = true
	}
	tickets := make(map[string]bool, len(data.Tickets))
	for _, ticket := range data.Tickets {
		tickets[ticket.Code] = true
	}
	for _, fare := range data.Fares {
		if flows[fare.FlowID] && tickets[fare.TicketCode] {
			return &data, nil
		}
	}
	return nil, fmt.Errorf("fares: ParseFeedZip: archive has no fare linked to both a flow and a ticket type")
}
