package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/ncbi-entrez/internal/store"

	"github.com/spf13/cobra"
)

// citeMatchEntry represents a single citation to resolve.
type citeMatchEntry struct {
	Journal string `json:"journal"`
	Year    string `json:"year"`
	Volume  string `json:"volume"`
	Page    string `json:"page"`
	Author  string `json:"author"`
	Key     string `json:"key,omitempty"`
	Title   string `json:"title,omitempty"`
}

// citeMatchResult represents the outcome for one citation.
type citeMatchResult struct {
	Input     citeMatchEntry `json:"input"`
	PMID      string         `json:"pmid,omitempty"`
	Matched   bool           `json:"matched"`
	Retracted bool           `json:"retracted"`
	Status    string         `json:"status"`
}

func ensureCiteMatchTables(db *store.Store) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS cite_matches (
			key TEXT PRIMARY KEY,
			journal TEXT,
			year TEXT,
			volume TEXT,
			page TEXT,
			author TEXT,
			pmid TEXT,
			retracted INTEGER NOT NULL DEFAULT 0,
			resolved_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, s := range stmts {
		if _, err := db.DB().Exec(s); err != nil {
			return fmt.Errorf("creating cite_matches table: %w", err)
		}
	}
	return nil
}

func newCiteMatchCmd(flags *rootFlags) *cobra.Command {
	var flagInput string
	var flagBib string
	var flagCheckRetractions bool

	cmd := &cobra.Command{
		Use:   "cite-match",
		Short: "Batch-resolve citations to PMIDs and check for retractions",
		Long: `Batch Citation Match with Retraction Check — reads a CSV or BibTeX
file of references, calls ECitMatch to resolve each to a PMID, and optionally
checks whether resolved papers have been retracted.

CSV format: journal,year,volume,page,author (header row expected).
BibTeX: parses @article entries for title, author, year, journal, volume, pages.`,
		Example: `  ncbi-entrez-pp-cli cite-match --input refs.csv
  ncbi-entrez-pp-cli cite-match --bib refs.bib --check-retractions
  ncbi-entrez-pp-cli cite-match --input refs.csv --check-retractions --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagInput == "" && flagBib == "" && !flags.dryRun {
				return fmt.Errorf("either --input (CSV) or --bib (BibTeX) is required")
			}
			if dryRunOK(flags) {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			dbPath := defaultDBPath("ncbi-entrez-pp-cli")
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := ensureCiteMatchTables(db); err != nil {
				return err
			}

			// Parse input entries
			var entries []citeMatchEntry
			if flagBib != "" {
				entries, err = parseBibTeX(flagBib)
			} else {
				entries, err = parseCSVCitations(flagInput)
			}
			if err != nil {
				return fmt.Errorf("parsing input: %w", err)
			}

			if len(entries) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"status":  "no_entries",
					"message": "No citation entries found in input file",
				}, flags)
			}

			// Process in batches (ECitMatch supports multiple per call via bdata)
			batchSize := 25
			var results []citeMatchResult

			for i := 0; i < len(entries); i += batchSize {
				end := i + batchSize
				if end > len(entries) {
					end = len(entries)
				}
				batch := entries[i:end]

				// Build bdata string: journal|year|volume|page|author|key|
				var bdataParts []string
				for idx, e := range batch {
					key := e.Key
					if key == "" {
						key = fmt.Sprintf("ref_%d", i+idx)
					}
					bdata := fmt.Sprintf("%s|%s|%s|%s|%s|%s|",
						e.Journal, e.Year, e.Volume, e.Page, e.Author, key)
					bdataParts = append(bdataParts, bdata)
				}

				params := map[string]string{
					"db":      "pubmed",
					"rettype": "xml",
					"bdata":   strings.Join(bdataParts, "\r"),
				}

				data, err := c.Get("/ecitmatch.cgi", params)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: ECitMatch batch failed: %v\n", err)
					// Mark all in batch as unmatched
					for _, e := range batch {
						results = append(results, citeMatchResult{
							Input:   e,
							Matched: false,
							Status:  "api_error",
						})
					}
					continue
				}

				// Parse ECitMatch response — returns pipe-delimited text
				pmidMap := parseEcitmatchResponse(data)

				for idx, e := range batch {
					key := e.Key
					if key == "" {
						key = fmt.Sprintf("ref_%d", i+idx)
					}

					pmid := ""
					if resolved, ok := pmidMap[key]; ok {
						pmid = resolved
					}

					r := citeMatchResult{
						Input:   e,
						PMID:    pmid,
						Matched: pmid != "" && pmid != "AMBIGUOUS",
						Status:  "resolved",
					}

					if pmid == "AMBIGUOUS" {
						r.Status = "ambiguous"
						r.PMID = ""
					} else if pmid == "" {
						r.Status = "not_found"
					}

					results = append(results, r)

					// Store in database
					if r.Matched {
						_, _ = db.DB().Exec(
							`INSERT OR REPLACE INTO cite_matches (key, journal, year, volume, page, author, pmid, resolved_at)
							 VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
							key, e.Journal, e.Year, e.Volume, e.Page, e.Author, pmid,
						)
					}
				}
			}

			// Optional retraction check
			if flagCheckRetractions {
				fmt.Fprintf(os.Stderr, "checking retractions...\n")
				for i := range results {
					if !results[i].Matched {
						continue
					}
					retracted := checkRetraction(c, results[i].PMID)
					results[i].Retracted = retracted
					if retracted {
						results[i].Status = "retracted"
						// Update in DB
						_, _ = db.DB().Exec(
							`UPDATE cite_matches SET retracted = 1 WHERE pmid = ?`,
							results[i].PMID,
						)
					}
				}
			}

			// Summary
			matched := 0
			retracted := 0
			for _, r := range results {
				if r.Matched {
					matched++
				}
				if r.Retracted {
					retracted++
				}
			}

			output := map[string]any{
				"total":     len(results),
				"matched":   matched,
				"unmatched": len(results) - matched,
				"retracted": retracted,
				"results":   results,
			}

			return printJSONFiltered(cmd.OutOrStdout(), output, flags)
		},
	}

	cmd.Flags().StringVar(&flagInput, "input", "", "Path to CSV file (journal,year,volume,page,author)")
	cmd.Flags().StringVar(&flagBib, "bib", "", "Path to BibTeX file")
	cmd.Flags().BoolVar(&flagCheckRetractions, "check-retractions", false, "Check resolved PMIDs for retraction status")

	return cmd
}

// parseCSVCitations reads a CSV file with columns: journal,year,volume,page,author
func parseCSVCitations(path string) ([]citeMatchEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening CSV: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var entries []citeMatchEntry
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Skip header row
		if lineNum == 1 {
			lower := strings.ToLower(line)
			if strings.Contains(lower, "journal") || strings.Contains(lower, "author") {
				continue
			}
		}

		parts := strings.SplitN(line, ",", 6)
		if len(parts) < 5 {
			fmt.Fprintf(os.Stderr, "warning: line %d has fewer than 5 fields, skipping\n", lineNum)
			continue
		}

		e := citeMatchEntry{
			Journal: strings.TrimSpace(parts[0]),
			Year:    strings.TrimSpace(parts[1]),
			Volume:  strings.TrimSpace(parts[2]),
			Page:    strings.TrimSpace(parts[3]),
			Author:  strings.TrimSpace(parts[4]),
		}
		if len(parts) > 5 {
			e.Key = strings.TrimSpace(parts[5])
		}

		entries = append(entries, e)
	}

	return entries, scanner.Err()
}

// parseBibTeX does a lightweight parse of BibTeX @article entries.
func parseBibTeX(path string) ([]citeMatchEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading BibTeX: %w", err)
	}

	content := string(data)
	var entries []citeMatchEntry

	// Split on @article (case-insensitive)
	lower := strings.ToLower(content)
	idx := 0
	for {
		pos := strings.Index(lower[idx:], "@article")
		if pos < 0 {
			break
		}
		startPos := idx + pos

		// Find the matching closing brace
		braceDepth := 0
		entryEnd := startPos
		for i := startPos; i < len(content); i++ {
			if content[i] == '{' {
				braceDepth++
			} else if content[i] == '}' {
				braceDepth--
				if braceDepth == 0 {
					entryEnd = i + 1
					break
				}
			}
		}
		if entryEnd <= startPos {
			idx = startPos + 8
			continue
		}

		entryText := content[startPos:entryEnd]

		// Extract the citation key
		keyStart := strings.Index(entryText, "{")
		keyEnd := strings.Index(entryText, ",")
		key := ""
		if keyStart >= 0 && keyEnd > keyStart {
			key = strings.TrimSpace(entryText[keyStart+1 : keyEnd])
		}

		e := citeMatchEntry{Key: key}

		// Extract fields
		e.Title = extractBibField(entryText, "title")
		e.Author = extractBibField(entryText, "author")
		e.Year = extractBibField(entryText, "year")
		e.Journal = extractBibField(entryText, "journal")
		e.Volume = extractBibField(entryText, "volume")
		e.Page = extractBibField(entryText, "pages")
		if e.Page == "" {
			e.Page = extractBibField(entryText, "page")
		}

		// Clean up first page only
		if strings.Contains(e.Page, "-") {
			e.Page = strings.Split(e.Page, "-")[0]
			e.Page = strings.TrimSpace(e.Page)
		}

		// Simplify author to first author last name
		if strings.Contains(e.Author, " and ") {
			e.Author = strings.Split(e.Author, " and ")[0]
		}
		e.Author = strings.TrimSpace(e.Author)

		entries = append(entries, e)
		idx = entryEnd
	}

	return entries, nil
}

// extractBibField extracts a field value from a BibTeX entry.
func extractBibField(entry, field string) string {
	lower := strings.ToLower(entry)
	fieldTag := strings.ToLower(field) + "="

	pos := strings.Index(lower, fieldTag)
	if pos < 0 {
		// Try with spaces around =
		fieldTag = strings.ToLower(field) + " ="
		pos = strings.Index(lower, fieldTag)
		if pos < 0 {
			return ""
		}
	}

	// Find the value after =
	eqPos := strings.Index(entry[pos:], "=")
	if eqPos < 0 {
		return ""
	}
	valStart := pos + eqPos + 1

	// Skip whitespace
	for valStart < len(entry) && (entry[valStart] == ' ' || entry[valStart] == '\t') {
		valStart++
	}

	if valStart >= len(entry) {
		return ""
	}

	// Value is either in {braces} or "quotes" or bare
	var value string
	switch entry[valStart] {
	case '{':
		depth := 0
		end := valStart
		for i := valStart; i < len(entry); i++ {
			if entry[i] == '{' {
				depth++
			} else if entry[i] == '}' {
				depth--
				if depth == 0 {
					end = i
					break
				}
			}
		}
		value = entry[valStart+1 : end]
	case '"':
		end := strings.Index(entry[valStart+1:], "\"")
		if end >= 0 {
			value = entry[valStart+1 : valStart+1+end]
		}
	default:
		end := strings.IndexAny(entry[valStart:], ",}")
		if end >= 0 {
			value = entry[valStart : valStart+end]
		}
	}

	return strings.TrimSpace(value)
}

// parseEcitmatchResponse parses the pipe-delimited ECitMatch response.
// Each line: journal|year|volume|page|author|key|PMID (or empty/AMBIGUOUS)
func parseEcitmatchResponse(data json.RawMessage) map[string]string {
	result := make(map[string]string)

	// ECitMatch returns plain text, not JSON
	text := string(data)

	// Strip JSON wrapping if present
	text = strings.Trim(text, "\"")
	text = strings.ReplaceAll(text, "\\n", "\n")

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 7 {
			continue
		}

		key := strings.TrimSpace(parts[5])
		pmid := strings.TrimSpace(parts[6])

		if key != "" && pmid != "" {
			result[key] = pmid
		}
	}

	return result
}

// checkRetraction calls EFetch for a PMID and checks if the paper has been retracted.
func checkRetraction(c interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}, pmid string) bool {
	params := map[string]string{
		"db":      "pubmed",
		"id":      pmid,
		"rettype": "xml",
		"retmode": "text",
	}

	data, err := c.Get("/efetch.fcgi", params)
	if err != nil {
		return false
	}

	text := strings.ToLower(string(data))
	return strings.Contains(text, "retracted publication") ||
		strings.Contains(text, "retraction of publication") ||
		strings.Contains(text, "retraction in") ||
		strings.Contains(text, "publicationtype=\"retracted") ||
		strings.Contains(text, "\"retracted publication\"")
}
