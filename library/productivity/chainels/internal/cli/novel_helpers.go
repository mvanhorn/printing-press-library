package cli

import (
	"fmt"
	"strconv"
	"strings"
)

func parseDaysFlag(s string) (int, error) {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "d"))
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid days: %q (use forms like 14d or 30)", s)
	}
	return n, nil
}
