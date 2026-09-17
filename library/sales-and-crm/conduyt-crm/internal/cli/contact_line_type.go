// Copyright 2026 Paul Taramona and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import "strings"

func contactLineType(m map[string]any) string {
	for _, key := range []string{"lineType", "line_type"} {
		if lineType := strings.TrimSpace(strAny(m, key)); lineType != "" {
			return lineType
		}
	}
	if customFields, ok := m["customFields"].(map[string]any); ok {
		return strings.TrimSpace(strAny(customFields, "sms_line_type"))
	}
	return ""
}
