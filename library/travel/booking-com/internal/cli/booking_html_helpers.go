package cli

import "strconv"

// htmlParams drops empty values from a string-string map so query strings
// don't carry "checkin=&dest_id=&..." that booking.com may reject.
func htmlParams(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		if v != "" {
			out[k] = v
		}
	}
	return out
}

// itoaIfNonZero returns the decimal string for n if n != 0, else "".
// Used so generated commands can pass int flags through htmlParams without
// emitting "group_adults=0" when the user didn't set the flag.
func itoaIfNonZero(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}
