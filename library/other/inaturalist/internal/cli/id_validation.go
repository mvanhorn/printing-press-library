// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strconv"
	"strings"
)

// requirePositiveNumericIDs validates the comma-delimited public IDs accepted
// by iNaturalist place and project lookup endpoints before making a request.
func requirePositiveNumericIDs(value, name string) error {
	for _, raw := range strings.Split(value, ",") {
		id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil || id < 1 {
			return usageErr(fmt.Errorf("%s must contain one or more positive numeric IDs", name))
		}
	}
	return nil
}
