package cli

import (
	"regexp"
	"strings"
)

// venueParts holds the complex and sub-room components extracted from a
// canonicalized venue name. Room is empty when no sub-space is detected.
type venueParts struct {
	Complex string // the physical complex/building
	Room    string // sub-space within the complex; "" if none
}

// Compiled package-level regexps for venue room splitting.
// All operate on already-canonicalized lowercase input.
var (
	// "the <room> at <complex>" — reorder so complex is the base.
	reVenueTheRoomAt = regexp.MustCompile(`^the (.+?) at (.+)$`)
	// "<complex> (<room>)"
	reVenueParens = regexp.MustCompile(`^(.+?)\s+\((.+)\)$`)
	// "<complex> - <room>"
	reVenueDash = regexp.MustCompile(`^(.+?)\s+-\s+(.+)$`)
	// "<complex> [<room>]"
	reVenueBrackets = regexp.MustCompile(`^(.+?)\s+\[(.+)\]$`)
)

// extractVenueParts splits a canonicalized venue name into the physical
// complex and an optional sub-room using structural heuristics.
func extractVenueParts(s string) venueParts {
	s = strings.TrimSpace(s)

	// "the <room> at <complex>" pattern — reorder.
	if m := reVenueTheRoomAt.FindStringSubmatch(s); m != nil {
		return venueParts{
			Complex: strings.TrimSpace(m[2]),
			Room:    strings.TrimSpace(m[1]),
		}
	}

	// "<complex> (<room>)"
	if m := reVenueParens.FindStringSubmatch(s); m != nil {
		return venueParts{
			Complex: strings.TrimSpace(m[1]),
			Room:    strings.TrimSpace(m[2]),
		}
	}

	// "<complex> - <room>"
	if m := reVenueDash.FindStringSubmatch(s); m != nil {
		return venueParts{
			Complex: strings.TrimSpace(m[1]),
			Room:    strings.TrimSpace(m[2]),
		}
	}

	// "<complex> [<room>]"
	if m := reVenueBrackets.FindStringSubmatch(s); m != nil {
		return venueParts{
			Complex: strings.TrimSpace(m[1]),
			Room:    strings.TrimSpace(m[2]),
		}
	}

	// No room separator detected — whole string is the complex.
	return venueParts{Complex: s}
}
