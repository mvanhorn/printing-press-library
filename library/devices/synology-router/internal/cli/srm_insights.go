// Copyright 2026 eric-jung. Licensed under Apache-2.0. See LICENSE.
// SRM insight helpers — enriches raw API data with human-readable formatting
// for bandwidth, online counts, and sorted views.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
)

// formatBytes converts a byte count to a human-readable string (B/KB/MB/GB).
// Values are rounded to one decimal place at KB and above.
func formatBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// toInt64 converts common JSON number representations to int64.
// Handles float64 (standard JSON unmarshalling) and string representations.
func toInt64(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	case json.Number:
		n, _ := t.Int64()
		return n
	}
	return 0
}

// enrichTrafficItems adds human-readable "download_hr" and "upload_hr" fields
// to each traffic item when download/upload byte fields are present.
// The raw byte fields are left intact for --json consumers.
func enrichTrafficItems(items []map[string]any) []map[string]any {
	downloadKeys := []string{"download", "rx", "bytes_in", "bps_download", "total_download"}
	uploadKeys := []string{"upload", "tx", "bytes_out", "bps_upload", "total_upload"}

	for _, item := range items {
		for _, k := range downloadKeys {
			if v, ok := item[k]; ok {
				b := toInt64(v)
				if b > 0 {
					item["download_hr"] = formatBytes(b)
					break
				}
			}
		}
		for _, k := range uploadKeys {
			if v, ok := item[k]; ok {
				b := toInt64(v)
				if b > 0 {
					item["upload_hr"] = formatBytes(b)
					break
				}
			}
		}
	}
	return items
}

// sortTrafficByDownload sorts traffic items descending by their download byte
// count. Items without a recognizable download field are sorted last.
func sortTrafficByDownload(items []map[string]any) {
	downloadKeys := []string{"download", "rx", "bytes_in", "bps_download", "total_download"}

	getDownload := func(item map[string]any) int64 {
		for _, k := range downloadKeys {
			if v, ok := item[k]; ok {
				return toInt64(v)
			}
		}
		return 0
	}

	sort.SliceStable(items, func(i, j int) bool {
		return getDownload(items[i]) > getDownload(items[j])
	})
}

// deviceOnlineSummary counts online and offline devices from a list.
// It looks for common boolean/string "online" or "is_online" fields.
func deviceOnlineSummary(items []map[string]any) (online, offline int) {
	onlineKeys := []string{"online", "is_online", "connected", "status"}
	for _, item := range items {
		for _, k := range onlineKeys {
			if v, ok := item[k]; ok {
				switch val := v.(type) {
				case bool:
					if val {
						online++
					} else {
						offline++
					}
					goto next
				case string:
					if val == "true" || val == "online" || val == "connected" || val == "1" {
						online++
					} else {
						offline++
					}
					goto next
				case float64:
					if val != 0 {
						online++
					} else {
						offline++
					}
					goto next
				}
			}
		}
	next:
	}
	return online, offline
}
