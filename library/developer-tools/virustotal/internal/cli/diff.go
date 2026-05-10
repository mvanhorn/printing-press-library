// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/ca7ai/pp-virustotal/internal/vtstore"
	"github.com/spf13/cobra"
)

func newDiffCmd(flags *rootFlags) *cobra.Command {
	var detailed bool

	cmd := &cobra.Command{
		Use:   "diff <hash1> <hash2>",
		Short: "Compare two file reports",
		Long: `Compare detection results, metadata, and behavior between two files.

Shows:
- Detection differences (which engines detect one but not the other)
- Metadata changes (compile time, sections, imports)
- Behavioral differences
- File properties comparison

Examples:
  # Basic diff
  virustotal-pp-cli diff <sha256_1> <sha256_2>

  # Detailed diff with full engine lists
  virustotal-pp-cli diff <hash1> <hash2> --detailed`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			hash1, hash2 := args[0], args[1]

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			store, err := vtstore.Open()
			if err != nil {
				return fmt.Errorf("opening cache: %w", err)
			}
			defer store.Close()

			// Fetch both reports
			data1, err := fetchIOCData(c, store, "file", hash1)
			if err != nil {
				return fmt.Errorf("fetching %s: %w", hash1, err)
			}

			data2, err := fetchIOCData(c, store, "file", hash2)
			if err != nil {
				return fmt.Errorf("fetching %s: %w", hash2, err)
			}

			// Compute diff
			diff := computeFileDiff(data1, data2, hash1, hash2)

			// Output
			if flags.asJSON {
				out, _ := json.MarshalIndent(diff, "", "  ")
				fmt.Println(string(out))
			} else {
				printFileDiff(cmd.OutOrStdout(), diff, detailed)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&detailed, "detailed", false, "Show detailed engine-level differences")

	return cmd
}

// FileDiff represents comparison between two files
type FileDiff struct {
	Hash1            string               `json:"hash1"`
	Hash2            string               `json:"hash2"`
	Metadata         MetadataDiff         `json:"metadata"`
	Detection        DetectionDiff        `json:"detection"`
	Behavior         BehaviorDiff         `json:"behavior,omitempty"`
}

type MetadataDiff struct {
	Size1            int64  `json:"size1"`
	Size2            int64  `json:"size2"`
	Type1            string `json:"type1"`
	Type2            string `json:"type2"`
	FirstSeen1       int64  `json:"first_seen1"`
	FirstSeen2       int64  `json:"first_seen2"`
	TimesSubmitted1  int    `json:"times_submitted1"`
	TimesSubmitted2  int    `json:"times_submitted2"`
}

type DetectionDiff struct {
	Malicious1       int      `json:"malicious1"`
	Malicious2       int      `json:"malicious2"`
	Harmless1        int      `json:"harmless1"`
	Harmless2        int      `json:"harmless2"`
	OnlyInFirst      []string `json:"only_in_first"`
	OnlyInSecond     []string `json:"only_in_second"`
	BothDetected     []string `json:"both_detected"`
	NeitherDetected  []string `json:"neither_detected,omitempty"`
}

type BehaviorDiff struct {
	Similarities     []string `json:"similarities,omitempty"`
	Differences      []string `json:"differences,omitempty"`
}

func computeFileDiff(data1, data2 json.RawMessage, hash1, hash2 string) *FileDiff {
	diff := &FileDiff{
		Hash1: hash1,
		Hash2: hash2,
	}

	var parsed1, parsed2 map[string]any
	json.Unmarshal(data1, &parsed1)
	json.Unmarshal(data2, &parsed2)

	attr1, _ := getNestedMap(parsed1, "data", "attributes")
	attr2, _ := getNestedMap(parsed2, "data", "attributes")

	// Metadata diff
	if size, ok := attr1["size"].(float64); ok {
		diff.Metadata.Size1 = int64(size)
	}
	if size, ok := attr2["size"].(float64); ok {
		diff.Metadata.Size2 = int64(size)
	}

	if typ, ok := attr1["type_description"].(string); ok {
		diff.Metadata.Type1 = typ
	}
	if typ, ok := attr2["type_description"].(string); ok {
		diff.Metadata.Type2 = typ
	}

	if fs, ok := attr1["first_submission_date"].(float64); ok {
		diff.Metadata.FirstSeen1 = int64(fs)
	}
	if fs, ok := attr2["first_submission_date"].(float64); ok {
		diff.Metadata.FirstSeen2 = int64(fs)
	}

	if ts, ok := attr1["times_submitted"].(float64); ok {
		diff.Metadata.TimesSubmitted1 = int(ts)
	}
	if ts, ok := attr2["times_submitted"].(float64); ok {
		diff.Metadata.TimesSubmitted2 = int(ts)
	}

	// Detection diff
	results1 := extractAnalysisResults(attr1)
	results2 := extractAnalysisResults(attr2)

	diff.Detection = compareDetections(results1, results2)

	// Behavior diff (if available)
	diff.Behavior = compareBehavior(attr1, attr2)

	return diff
}

func extractAnalysisResults(attributes map[string]any) map[string]DetectionResult {
	results := make(map[string]DetectionResult)

	analysisResults, ok := attributes["last_analysis_results"].(map[string]interface{})
	if !ok {
		return results
	}

	for engine, data := range analysisResults {
		if engineData, ok := data.(map[string]interface{}); ok {
			result := DetectionResult{
				Engine: engine,
			}

			if cat, ok := engineData["category"].(string); ok {
				result.Category = cat
			}
			if res, ok := engineData["result"].(string); ok {
				result.Result = res
			}

			results[engine] = result
		}
	}

	return results
}

type DetectionResult struct {
	Engine   string
	Category string
	Result   string
}

func compareDetections(results1, results2 map[string]DetectionResult) DetectionDiff {
	diff := DetectionDiff{}

	allEngines := make(map[string]bool)
	for engine := range results1 {
		allEngines[engine] = true
	}
	for engine := range results2 {
		allEngines[engine] = true
	}

	for engine := range allEngines {
		r1, exists1 := results1[engine]
		r2, exists2 := results2[engine]

		malicious1 := exists1 && isMaliciousCategory(r1.Category)
		malicious2 := exists2 && isMaliciousCategory(r2.Category)

		if malicious1 {
			diff.Malicious1++
		} else if exists1 {
			diff.Harmless1++
		}

		if malicious2 {
			diff.Malicious2++
		} else if exists2 {
			diff.Harmless2++
		}

		if malicious1 && !malicious2 {
			diff.OnlyInFirst = append(diff.OnlyInFirst, engine)
		} else if !malicious1 && malicious2 {
			diff.OnlyInSecond = append(diff.OnlyInSecond, engine)
		} else if malicious1 && malicious2 {
			diff.BothDetected = append(diff.BothDetected, engine)
		}
	}

	sort.Strings(diff.OnlyInFirst)
	sort.Strings(diff.OnlyInSecond)
	sort.Strings(diff.BothDetected)

	return diff
}

func isMaliciousCategory(category string) bool {
	maliciousCategories := []string{
		"malicious", "malware", "trojan", "virus", "worm",
		"ransomware", "backdoor", "rootkit", "spyware",
		"adware", "suspicious", "detected",
	}

	lower := strings.ToLower(category)
	for _, mal := range maliciousCategories {
		if strings.Contains(lower, mal) {
			return true
		}
	}
	return false
}

func compareBehavior(attr1, attr2 map[string]any) BehaviorDiff {
	diff := BehaviorDiff{}

	// Compare behavioral indicators
	behaviors1 := extractBehaviorIndicators(attr1)
	behaviors2 := extractBehaviorIndicators(attr2)

	for behavior := range behaviors1 {
		if behaviors2[behavior] {
			diff.Similarities = append(diff.Similarities, behavior)
		} else {
			diff.Differences = append(diff.Differences, behavior+" (only in first)")
		}
	}

	for behavior := range behaviors2 {
		if !behaviors1[behavior] {
			diff.Differences = append(diff.Differences, behavior+" (only in second)")
		}
	}

	sort.Strings(diff.Similarities)
	sort.Strings(diff.Differences)

	return diff
}

func extractBehaviorIndicators(attributes map[string]any) map[string]bool {
	indicators := make(map[string]bool)

	// Signature matches
	if sigMatches, ok := attributes["signature_matches"].([]interface{}); ok {
		for _, match := range sigMatches {
			if matchMap, ok := match.(map[string]interface{}); ok {
				if name, ok := matchMap["name"].(string); ok {
					indicators[name] = true
				}
			}
		}
	}

	// Behavioral tags
	if tags, ok := attributes["tags"].([]interface{}); ok {
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				indicators["tag:"+tagStr] = true
			}
		}
	}

	return indicators
}

func printFileDiff(w interface{ Write([]byte) (int, error) }, diff *FileDiff, detailed bool) {
	fmt.Fprintf(w, "File Comparison\n")
	fmt.Fprintf(w, "===============\n\n")

	fmt.Fprintf(w, "Hash 1: %s\n", truncateHash(diff.Hash1))
	fmt.Fprintf(w, "Hash 2: %s\n\n", truncateHash(diff.Hash2))

	// Metadata
	fmt.Fprintf(w, "Metadata:\n")
	fmt.Fprintf(w, "  Size:            %s vs %s\n",
		formatBytes(diff.Metadata.Size1),
		formatBytes(diff.Metadata.Size2))
	fmt.Fprintf(w, "  Type:            %s vs %s\n",
		diff.Metadata.Type1,
		diff.Metadata.Type2)
	fmt.Fprintf(w, "  Times Submitted: %d vs %d\n\n",
		diff.Metadata.TimesSubmitted1,
		diff.Metadata.TimesSubmitted2)

	// Detection
	fmt.Fprintf(w, "Detection:\n")
	fmt.Fprintf(w, "  Malicious:       %d/%d vs %d/%d\n",
		diff.Detection.Malicious1,
		diff.Detection.Malicious1+diff.Detection.Harmless1,
		diff.Detection.Malicious2,
		diff.Detection.Malicious2+diff.Detection.Harmless2)

	if len(diff.Detection.OnlyInFirst) > 0 {
		fmt.Fprintf(w, "  Only in first:   %d engines\n", len(diff.Detection.OnlyInFirst))
		if detailed {
			for _, engine := range diff.Detection.OnlyInFirst {
				fmt.Fprintf(w, "    - %s\n", engine)
			}
		}
	}

	if len(diff.Detection.OnlyInSecond) > 0 {
		fmt.Fprintf(w, "  Only in second:  %d engines\n", len(diff.Detection.OnlyInSecond))
		if detailed {
			for _, engine := range diff.Detection.OnlyInSecond {
				fmt.Fprintf(w, "    - %s\n", engine)
			}
		}
	}

	if len(diff.Detection.BothDetected) > 0 {
		fmt.Fprintf(w, "  Both detected:   %d engines\n", len(diff.Detection.BothDetected))
		if detailed {
			for _, engine := range diff.Detection.BothDetected {
				fmt.Fprintf(w, "    - %s\n", engine)
			}
		}
	}

	// Behavior
	if len(diff.Behavior.Similarities) > 0 || len(diff.Behavior.Differences) > 0 {
		fmt.Fprintf(w, "\nBehavior:\n")

		if len(diff.Behavior.Similarities) > 0 {
			fmt.Fprintf(w, "  Common indicators: %d\n", len(diff.Behavior.Similarities))
			if detailed {
				for _, sim := range diff.Behavior.Similarities {
					fmt.Fprintf(w, "    - %s\n", sim)
				}
			}
		}

		if len(diff.Behavior.Differences) > 0 {
			fmt.Fprintf(w, "  Differences: %d\n", len(diff.Behavior.Differences))
			if detailed {
				for _, diff := range diff.Behavior.Differences {
					fmt.Fprintf(w, "    - %s\n", diff)
				}
			}
		}
	}
}

func truncateHash(hash string) string {
	if len(hash) <= 16 {
		return hash
	}
	return hash[:8] + "..." + hash[len(hash)-8:]
}

func formatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	}
	if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(bytes)/(1024*1024*1024))
}
