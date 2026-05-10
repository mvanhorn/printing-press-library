// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// FormatForLLM restructures VT API responses into human-readable format optimized for LLM consumption
func FormatForLLM(data json.RawMessage, iocType string) string {
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return string(data)
	}

	attributes, ok := getNestedMap(parsed, "data", "attributes")
	if !ok {
		// Try direct attributes
		if attr, ok := parsed["attributes"].(map[string]any); ok {
			attributes = attr
		} else {
			return string(data)
		}
	}

	switch iocType {
	case "file":
		return formatFileForLLM(attributes)
	case "domain":
		return formatDomainForLLM(attributes)
	case "ip", "ip-address":
		return formatIPForLLM(attributes)
	case "url":
		return formatURLForLLM(attributes)
	default:
		return formatGenericForLLM(attributes)
	}
}

func formatFileForLLM(attr map[string]any) string {
	var sb strings.Builder

	// Detection summary
	if stats, ok := attr["last_analysis_stats"].(map[string]interface{}); ok {
		malicious := getInt(stats, "malicious")
		suspicious := getInt(stats, "suspicious")
		harmless := getInt(stats, "harmless")
		undetected := getInt(stats, "undetected")
		total := malicious + suspicious + harmless + undetected

		sb.WriteString(fmt.Sprintf("Detection: %d/%d engines flagged as malicious or suspicious\n",
			malicious+suspicious, total))

		if malicious > 0 {
			sb.WriteString(fmt.Sprintf("  - Malicious: %d\n", malicious))
		}
		if suspicious > 0 {
			sb.WriteString(fmt.Sprintf("  - Suspicious: %d\n", suspicious))
		}
		if harmless > 0 {
			sb.WriteString(fmt.Sprintf("  - Harmless: %d\n", harmless))
		}
		if undetected > 0 {
			sb.WriteString(fmt.Sprintf("  - Undetected: %d\n", undetected))
		}
	}

	// Consensus verdict
	if results, ok := attr["last_analysis_results"].(map[string]interface{}); ok {
		verdicts := make(map[string]int)
		for _, res := range results {
			if resMap, ok := res.(map[string]interface{}); ok {
				if result, ok := resMap["result"].(string); ok && result != "" && result != "unrated" {
					verdicts[result]++
				}
			}
		}

		// Find most common verdict
		var maxVerdict string
		var maxCount int
		for verdict, count := range verdicts {
			if count > maxCount {
				maxVerdict = verdict
				maxCount = count
			}
		}

		if maxVerdict != "" && maxCount >= 3 {
			sb.WriteString(fmt.Sprintf("\nConsensus verdict: %s (reported by %d engines)\n", maxVerdict, maxCount))
		}
	}

	// Reputation
	if rep, ok := attr["reputation"].(float64); ok {
		sb.WriteString(fmt.Sprintf("\nReputation: %.0f", rep))
		if rep < -50 {
			sb.WriteString(" (highly malicious)")
		} else if rep < 0 {
			sb.WriteString(" (malicious)")
		} else if rep > 50 {
			sb.WriteString(" (trusted)")
		}
		sb.WriteString("\n")
	}

	// Timestamps
	if firstSeen, ok := attr["first_submission_date"].(float64); ok {
		t := time.Unix(int64(firstSeen), 0)
		sb.WriteString(fmt.Sprintf("\nFirst seen: %s\n", t.Format("2006-01-02 15:04:05 UTC")))
	}
	if lastSeen, ok := attr["last_submission_date"].(float64); ok {
		t := time.Unix(int64(lastSeen), 0)
		sb.WriteString(fmt.Sprintf("Last seen: %s\n", t.Format("2006-01-02 15:04:05 UTC")))
	}

	// File metadata
	if sha256, ok := attr["sha256"].(string); ok {
		sb.WriteString(fmt.Sprintf("\nSHA256: %s\n", sha256))
	}
	if md5, ok := attr["md5"].(string); ok {
		sb.WriteString(fmt.Sprintf("MD5: %s\n", md5))
	}
	if size, ok := attr["size"].(float64); ok {
		sb.WriteString(fmt.Sprintf("Size: %s\n", formatBytes(int64(size))))
	}
	if typ, ok := attr["type_description"].(string); ok {
		sb.WriteString(fmt.Sprintf("Type: %s\n", typ))
	}

	// Behavioral indicators
	if sigMatches, ok := attr["signature_matches"].([]interface{}); ok && len(sigMatches) > 0 {
		sb.WriteString("\nBehavioral indicators:\n")
		count := 0
		for _, match := range sigMatches {
			if count >= 10 {
				sb.WriteString(fmt.Sprintf("  ... and %d more\n", len(sigMatches)-count))
				break
			}
			if matchMap, ok := match.(map[string]interface{}); ok {
				if name, ok := matchMap["name"].(string); ok {
					sb.WriteString(fmt.Sprintf("  - %s\n", name))
					count++
				}
			}
		}
	}

	// Network behavior
	if domains, ok := attr["contacted_domains"].([]interface{}); ok && len(domains) > 0 {
		sb.WriteString(fmt.Sprintf("\nContacted domains: %d\n", len(domains)))
		for i, d := range domains {
			if i >= 5 {
				sb.WriteString(fmt.Sprintf("  ... and %d more\n", len(domains)-i))
				break
			}
			sb.WriteString(fmt.Sprintf("  - %v\n", d))
		}
	}

	if ips, ok := attr["contacted_ips"].([]interface{}); ok && len(ips) > 0 {
		sb.WriteString(fmt.Sprintf("\nContacted IPs: %d\n", len(ips)))
		for i, ip := range ips {
			if i >= 5 {
				sb.WriteString(fmt.Sprintf("  ... and %d more\n", len(ips)-i))
				break
			}
			sb.WriteString(fmt.Sprintf("  - %v\n", ip))
		}
	}

	// Tags
	if tags, ok := attr["tags"].([]interface{}); ok && len(tags) > 0 {
		sb.WriteString("\nTags: ")
		tagStrs := make([]string, 0, len(tags))
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				tagStrs = append(tagStrs, tagStr)
			}
		}
		sb.WriteString(strings.Join(tagStrs, ", "))
		sb.WriteString("\n")
	}

	return sb.String()
}

func formatDomainForLLM(attr map[string]any) string {
	var sb strings.Builder

	if domain, ok := attr["id"].(string); ok {
		sb.WriteString(fmt.Sprintf("Domain: %s\n\n", domain))
	}

	// Reputation
	if rep, ok := attr["reputation"].(float64); ok {
		sb.WriteString(fmt.Sprintf("Reputation: %.0f", rep))
		if rep < -50 {
			sb.WriteString(" (malicious)")
		} else if rep > 50 {
			sb.WriteString(" (trusted)")
		}
		sb.WriteString("\n")
	}

	// Categories
	if cats, ok := attr["categories"].(map[string]interface{}); ok {
		sb.WriteString("\nCategories:\n")
		for source, cat := range cats {
			sb.WriteString(fmt.Sprintf("  - %s: %v\n", source, cat))
		}
	}

	// Detection stats
	if stats, ok := attr["last_analysis_stats"].(map[string]interface{}); ok {
		malicious := getInt(stats, "malicious")
		suspicious := getInt(stats, "suspicious")
		harmless := getInt(stats, "harmless")
		undetected := getInt(stats, "undetected")
		total := malicious + suspicious + harmless + undetected

		if total > 0 {
			sb.WriteString(fmt.Sprintf("\nDetection: %d/%d security vendors flagged as malicious\n",
				malicious+suspicious, total))
		}
	}

	// DNS records
	if records, ok := attr["last_dns_records"].([]interface{}); ok && len(records) > 0 {
		sb.WriteString("\nDNS Records:\n")
		for i, rec := range records {
			if i >= 5 {
				sb.WriteString(fmt.Sprintf("  ... and %d more\n", len(records)-i))
				break
			}
			if recMap, ok := rec.(map[string]interface{}); ok {
				typ := recMap["type"]
				value := recMap["value"]
				sb.WriteString(fmt.Sprintf("  - %v %v\n", typ, value))
			}
		}
	}

	// Creation date
	if created, ok := attr["creation_date"].(float64); ok {
		t := time.Unix(int64(created), 0)
		sb.WriteString(fmt.Sprintf("\nCreated: %s\n", t.Format("2006-01-02")))
	}

	return sb.String()
}

func formatIPForLLM(attr map[string]any) string {
	var sb strings.Builder

	if ip, ok := attr["id"].(string); ok {
		sb.WriteString(fmt.Sprintf("IP Address: %s\n\n", ip))
	}

	// Reputation
	if rep, ok := attr["reputation"].(float64); ok {
		sb.WriteString(fmt.Sprintf("Reputation: %.0f", rep))
		if rep < -50 {
			sb.WriteString(" (malicious)")
		} else if rep > 50 {
			sb.WriteString(" (trusted)")
		}
		sb.WriteString("\n")
	}

	// Location
	if country, ok := attr["country"].(string); ok {
		sb.WriteString(fmt.Sprintf("Country: %s\n", country))
	}
	if continent, ok := attr["continent"].(string); ok {
		sb.WriteString(fmt.Sprintf("Continent: %s\n", continent))
	}

	// ASN info
	if asn, ok := attr["asn"].(float64); ok {
		sb.WriteString(fmt.Sprintf("\nASN: %.0f\n", asn))
	}
	if owner, ok := attr["as_owner"].(string); ok {
		sb.WriteString(fmt.Sprintf("AS Owner: %s\n", owner))
	}

	// Detection stats
	if stats, ok := attr["last_analysis_stats"].(map[string]interface{}); ok {
		malicious := getInt(stats, "malicious")
		suspicious := getInt(stats, "suspicious")
		harmless := getInt(stats, "harmless")
		undetected := getInt(stats, "undetected")
		total := malicious + suspicious + harmless + undetected

		if total > 0 {
			sb.WriteString(fmt.Sprintf("\nDetection: %d/%d security vendors flagged as malicious\n",
				malicious+suspicious, total))
		}
	}

	return sb.String()
}

func formatURLForLLM(attr map[string]any) string {
	var sb strings.Builder

	if url, ok := attr["url"].(string); ok {
		sb.WriteString(fmt.Sprintf("URL: %s\n\n", url))
	}

	// Detection stats
	if stats, ok := attr["last_analysis_stats"].(map[string]interface{}); ok {
		malicious := getInt(stats, "malicious")
		suspicious := getInt(stats, "suspicious")
		harmless := getInt(stats, "harmless")
		undetected := getInt(stats, "undetected")
		total := malicious + suspicious + harmless + undetected

		if total > 0 {
			sb.WriteString(fmt.Sprintf("Detection: %d/%d engines flagged as malicious\n",
				malicious+suspicious, total))
		}
	}

	// Last scan
	if scanDate, ok := attr["last_analysis_date"].(float64); ok {
		t := time.Unix(int64(scanDate), 0)
		sb.WriteString(fmt.Sprintf("\nLast scanned: %s\n", t.Format("2006-01-02 15:04:05 UTC")))
	}

	return sb.String()
}

func formatGenericForLLM(attr map[string]any) string {
	// Fallback generic formatter
	data, _ := json.MarshalIndent(attr, "", "  ")
	return string(data)
}

func getInt(m map[string]interface{}, key string) int {
	if val, ok := m[key].(float64); ok {
		return int(val)
	}
	return 0
}
