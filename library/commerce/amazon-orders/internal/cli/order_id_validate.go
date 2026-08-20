package cli

import "regexp"

// isValidOrderID returns true when the given string matches the canonical
// Amazon order ID shape: XXX-XXXXXXX-XXXXXXX for physical orders or
// D01-XXXXXXX-XXXXXXX for digital orders.
//
// Used by `orders get`, `orders invoice`, and `track` to reject obvious
// junk before issuing an HTTP request — Amazon rewrites unknown order IDs
// into a "Your Orders" default page, which our parser would then extract,
// so command-level validation is the only way to surface the user mistake.
var orderIDValidRegex = regexp.MustCompile(`^(?:\d{3}-\d{7}-\d{7}|D\d{2}-\d{7}-\d{7})$`)

func isValidOrderID(s string) bool {
	return orderIDValidRegex.MatchString(s)
}
