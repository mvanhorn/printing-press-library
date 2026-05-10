// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ca7ai/pp-virustotal/internal/vtstore"
	"github.com/spf13/cobra"
)

func newEnrichCmd(flags *rootFlags) *cobra.Command {
	var inputFile string
	var outputFile string
	var concurrency int
	var iocType string

	cmd := &cobra.Command{
		Use:   "enrich",
		Short: "Batch IOC enrichment pipeline",
		Long: `Enrich multiple IOCs in parallel and generate structured reports.

Reads newline-delimited IOCs (hashes, IPs, domains) from file or stdin,
queries VirusTotal API, caches results, and generates enrichment report.

Examples:
  # Enrich from file
  virustotal-pp-cli enrich --input iocs.txt --output report.json

  # Enrich from stdin
  cat hashes.txt | virustotal-pp-cli enrich --type file

  # High concurrency for large batches
  virustotal-pp-cli enrich --input iocs.txt --concurrency 10`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			store, err := vtstore.Open()
			if err != nil {
				return fmt.Errorf("opening cache: %w", err)
			}
			defer store.Close()

			// Read IOCs
			var iocs []IOC
			if inputFile != "" {
				iocs, err = readIOCsFromFile(inputFile, iocType)
			} else {
				iocs, err = readIOCsFromStdin(iocType)
			}
			if err != nil {
				return err
			}

			if len(iocs) == 0 {
				return fmt.Errorf("no IOCs to enrich")
			}

			fmt.Fprintf(os.Stderr, "Enriching %d IOCs...\n", len(iocs))

			// Execute enrichment
			report := enrichIOCs(c, store, iocs, concurrency)

			// Output results
			var output interface{ Write([]byte) (int, error) }
			if outputFile != "" {
				f, err := os.Create(outputFile)
				if err != nil {
					return fmt.Errorf("creating output file: %w", err)
				}
				defer f.Close()
				output = f
			} else {
				output = cmd.OutOrStdout()
			}

			if flags.asJSON {
				data, _ := json.MarshalIndent(report, "", "  ")
				fmt.Fprintln(output, string(data))
			} else {
				printEnrichmentReport(output, report)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&inputFile, "input", "i", "", "Input file with newline-delimited IOCs")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file for report (default: stdout)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 5, "Number of parallel workers")
	cmd.Flags().StringVar(&iocType, "type", "auto", "IOC type: auto, file, domain, ip")

	return cmd
}

// IOC represents an indicator to enrich
type IOC struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// EnrichmentReport contains batch enrichment results
type EnrichmentReport struct {
	TotalIOCs       int                `json:"total_iocs"`
	Enriched        int                `json:"enriched"`
	Failed          int                `json:"failed"`
	Cached          int                `json:"cached"`
	MaliciousCount  int                `json:"malicious_count"`
	CleanCount      int                `json:"clean_count"`
	Duration        time.Duration      `json:"duration"`
	Results         []EnrichmentResult `json:"results"`
	Summary         EnrichmentSummary  `json:"summary"`
}

// EnrichmentResult represents a single IOC result
type EnrichmentResult struct {
	IOC        IOC             `json:"ioc"`
	Status     string          `json:"status"` // success, failed, cached
	Malicious  bool            `json:"malicious"`
	DetectionRatio string      `json:"detection_ratio,omitempty"`
	Reputation int             `json:"reputation,omitempty"`
	Error      string          `json:"error,omitempty"`
	Data       json.RawMessage `json:"data,omitempty"`
}

// EnrichmentSummary provides aggregate statistics
type EnrichmentSummary struct {
	ByType     map[string]int `json:"by_type"`
	ByStatus   map[string]int `json:"by_status"`
	TopThreats []ThreatInfo   `json:"top_threats,omitempty"`
}

// ThreatInfo represents a high-confidence malicious IOC
type ThreatInfo struct {
	IOC            IOC    `json:"ioc"`
	DetectionRatio string `json:"detection_ratio"`
	MaliciousVotes int    `json:"malicious_votes"`
}

func readIOCsFromFile(path, iocType string) ([]IOC, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var iocs []IOC
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		typ := iocType
		if typ == "auto" {
			typ = detectIOCType(line)
		}

		iocs = append(iocs, IOC{Type: typ, Value: line})
	}

	return iocs, scanner.Err()
}

func readIOCsFromStdin(iocType string) ([]IOC, error) {
	var iocs []IOC
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		typ := iocType
		if typ == "auto" {
			typ = detectIOCType(line)
		}

		iocs = append(iocs, IOC{Type: typ, Value: line})
	}

	return iocs, scanner.Err()
}

var (
	sha256Regex = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)
	md5Regex    = regexp.MustCompile(`^[a-fA-F0-9]{32}$`)
	sha1Regex   = regexp.MustCompile(`^[a-fA-F0-9]{40}$`)
	ipRegex     = regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
	domainRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)
)

func detectIOCType(value string) string {
	if sha256Regex.MatchString(value) || md5Regex.MatchString(value) || sha1Regex.MatchString(value) {
		return "file"
	}
	if ipRegex.MatchString(value) {
		return "ip"
	}
	if domainRegex.MatchString(value) {
		return "domain"
	}
	return "unknown"
}

func enrichIOCs(c interface{}, store *vtstore.VTStore, iocs []IOC, workers int) *EnrichmentReport {
	start := time.Now()

	report := &EnrichmentReport{
		TotalIOCs: len(iocs),
		Summary: EnrichmentSummary{
			ByType:   make(map[string]int),
			ByStatus: make(map[string]int),
		},
	}

	// Worker pool
	jobs := make(chan IOC, len(iocs))
	results := make(chan EnrichmentResult, len(iocs))

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ioc := range jobs {
				result := enrichSingleIOC(c, store, ioc)
				results <- result
			}
		}()
	}

	// Feed jobs
	for _, ioc := range iocs {
		jobs <- ioc
	}
	close(jobs)

	// Wait for completion
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	for result := range results {
		report.Results = append(report.Results, result)

		report.Summary.ByType[result.IOC.Type]++
		report.Summary.ByStatus[result.Status]++

		if result.Status == "success" {
			report.Enriched++
		} else if result.Status == "failed" {
			report.Failed++
		} else if result.Status == "cached" {
			report.Cached++
		}

		if result.Malicious {
			report.MaliciousCount++

			// Track top threats
			if result.DetectionRatio != "" {
				malVotes := parseDetectionRatio(result.DetectionRatio)
				if malVotes >= 10 {
					report.Summary.TopThreats = append(report.Summary.TopThreats, ThreatInfo{
						IOC:            result.IOC,
						DetectionRatio: result.DetectionRatio,
						MaliciousVotes: malVotes,
					})
				}
			}
		} else if result.Status == "success" {
			report.CleanCount++
		}
	}

	report.Duration = time.Since(start)

	return report
}

func enrichSingleIOC(c interface{}, store *vtstore.VTStore, ioc IOC) EnrichmentResult {
	result := EnrichmentResult{
		IOC: ioc,
	}

	// Try cache first
	var data json.RawMessage
	var err error
	cached := false

	switch ioc.Type {
	case "file":
		if report, _ := store.GetFile(ioc.Value); report != nil {
			data = report.Data
			cached = true
			result.DetectionRatio = report.DetectionRatio
			result.Malicious = report.MaliciousVotes > 0
		}
	case "domain":
		if d, _ := store.GetDomain(ioc.Value); d != nil {
			data = d
			cached = true
		}
	case "ip":
		if d, _ := store.GetIP(ioc.Value); d != nil {
			data = d
			cached = true
		}
	}

	// Fetch from API if not cached
	if !cached {
		client := c.(interface{ Get(string, map[string]string) (json.RawMessage, error) })

		var path string
		switch ioc.Type {
		case "file":
			path = "/files/" + ioc.Value
		case "domain":
			path = "/domains/" + ioc.Value
		case "ip":
			path = "/ip_addresses/" + ioc.Value
		default:
			result.Status = "failed"
			result.Error = "unsupported IOC type"
			return result
		}

		data, err = client.Get(path, nil)
		if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			return result
		}

		result.Status = "success"

		// Store in cache
		switch ioc.Type {
		case "file":
			storeFileData(store, ioc.Value, data)
		case "domain":
			store.StoreDomain(ioc.Value, data)
		case "ip":
			store.StoreIP(ioc.Value, data)
		}
	} else {
		result.Status = "cached"
	}

	result.Data = data

	// Extract malicious indicators
	var parsed map[string]any
	if json.Unmarshal(data, &parsed) == nil {
		if attributes, ok := getNestedMap(parsed, "data", "attributes"); ok {
			// File detection stats
			if stats, ok := attributes["last_analysis_stats"].(map[string]interface{}); ok {
				malicious := 0
				if m, ok := stats["malicious"].(float64); ok {
					malicious = int(m)
				}
				harmless := 0
				if h, ok := stats["harmless"].(float64); ok {
					harmless = int(h)
				}
				total := malicious + harmless
				if total > 0 {
					result.DetectionRatio = fmt.Sprintf("%d/%d", malicious, total)
					result.Malicious = malicious > 0
				}
			}

			// Reputation (domain/IP)
			if rep, ok := attributes["reputation"].(float64); ok {
				result.Reputation = int(rep)
				result.Malicious = rep < 0
			}
		}
	}

	return result
}

func parseDetectionRatio(ratio string) int {
	parts := strings.Split(ratio, "/")
	if len(parts) != 2 {
		return 0
	}
	var mal int
	fmt.Sscanf(parts[0], "%d", &mal)
	return mal
}

func printEnrichmentReport(w interface{ Write([]byte) (int, error) }, report *EnrichmentReport) {
	fmt.Fprintf(w, "Enrichment Report\n")
	fmt.Fprintf(w, "=================\n\n")

	fmt.Fprintf(w, "Total IOCs:    %d\n", report.TotalIOCs)
	fmt.Fprintf(w, "Enriched:      %d\n", report.Enriched)
	fmt.Fprintf(w, "Cached:        %d\n", report.Cached)
	fmt.Fprintf(w, "Failed:        %d\n", report.Failed)
	fmt.Fprintf(w, "Malicious:     %d\n", report.MaliciousCount)
	fmt.Fprintf(w, "Clean:         %d\n", report.CleanCount)
	fmt.Fprintf(w, "Duration:      %s\n\n", report.Duration.Round(time.Millisecond))

	if len(report.Summary.TopThreats) > 0 {
		fmt.Fprintf(w, "High-Confidence Threats:\n")
		for i, threat := range report.Summary.TopThreats {
			if i >= 10 {
				break
			}
			fmt.Fprintf(w, "  [%s] %s - %s detections\n",
				threat.IOC.Type, threat.IOC.Value, threat.DetectionRatio)
		}
		fmt.Fprintf(w, "\n")
	}

	fmt.Fprintf(w, "By Type:\n")
	for typ, count := range report.Summary.ByType {
		fmt.Fprintf(w, "  %s: %d\n", typ, count)
	}
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "By Status:\n")
	for status, count := range report.Summary.ByStatus {
		fmt.Fprintf(w, "  %s: %d\n", status, count)
	}
}
