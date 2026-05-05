package cli

// toStringIDs converts a slice of int IDs to strings.
func toStringIDs(ids []int) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = intToStr(id)
	}
	return out
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
