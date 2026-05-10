package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const (
	baseURL = "https://thevc.kr"
	version = "0.1.0"
)

// ── Data Types ──────────────────────────────────────────────

type RankedOrg struct {
	Name        string        `json:"name"`
	ProfilePage string        `json:"profilePage"`
	Type        string        `json:"type"`
	ProductName string        `json:"productName"`
	Logo        string        `json:"logo"`
	TenWeeksHit []WeeklyHit   `json:"tenWeeksHit"`
}

type WeeklyHit struct {
	Year  int `json:"year"`
	Week  int `json:"week"`
	Count int `json:"count"`
}

type Profile struct {
	ID          string      `json:"_id"`
	Name        string      `json:"name"`
	NameEn      string      `json:"nameEn"`
	ProfilePage string      `json:"profilePage"`
	Type        string      `json:"type"`
	CorpType    string      `json:"corpType"`
	FoundedOn   string      `json:"foundedOn"`
	Website     string      `json:"website"`
	Status      string      `json:"status"`
	TotalFundingAmount interface{} `json:"totalFundingAmount"`
	TotalFundingCount  interface{} `json:"totalFundingCount"`
	LastFundedOn       string      `json:"lastFundedOn"`
	LastRound          string      `json:"lastRound"`
	InvestorCount      interface{} `json:"investorCount"`
	AvgAmount          interface{} `json:"avgAmount"`
	MaxAmount          interface{} `json:"maxAmount"`
	Products           []Product   `json:"products"`
	Fundings           []Funding   `json:"fundings"`
	EmployeeHistory    []EmpCount  `json:"employeeHistory"`
	Nationality        Nationality `json:"nationality"`
	raw                map[string]interface{}
}

type Nationality struct {
	Country string `json:"country"`
}

// Nationality can be a string or object in the API
func (n *Nationality) UnmarshalJSON(data []byte) error {
	// Try as string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		n.Country = s
		return nil
	}
	// Try as object
	type alias Nationality
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*n = Nationality(a)
	return nil
}

type Product struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Desc        string `json:"desc"`
}

type Funding struct {
	Round         string   `json:"round"`
	Amount        string   `json:"amount"`
	InvestorTypes []string `json:"investorTypes"`
}

type EmpCount struct {
	Count EmpCountValue `json:"count"`
	Date  string        `json:"date"`
}

// EmpCountValue handles count being either int or nested object
type EmpCountValue int

func (e *EmpCountValue) UnmarshalJSON(data []byte) error {
	// Try as integer
	var i int
	if err := json.Unmarshal(data, &i); err == nil {
		*e = EmpCountValue(i)
		return nil
	}
	// Try as object with nested count
	var obj struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	*e = EmpCountValue(obj.Count)
	return nil
}

// ── Database ─────────────────────────────────────────────────

func dbPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".pp-thevc", "thevc.db")
}

func openDB() (*sql.DB, error) {
	path := dbPath()
	os.MkdirAll(filepath.Dir(path), 0755)
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS rankings (
			slug TEXT PRIMARY KEY,
			name TEXT,
			type TEXT,
			product_name TEXT,
			weekly_views TEXT,
			fetched_at TEXT
		);
		CREATE TABLE IF NOT EXISTS companies (
			slug TEXT PRIMARY KEY,
			name TEXT,
			org_type TEXT,
			corp_type TEXT,
			website TEXT,
			founded_date TEXT,
			employee_count INTEGER,
			total_funding REAL,
			funding_count INTEGER,
			last_round TEXT,
			investor_count INTEGER,
			avg_amount REAL,
			max_amount REAL,
			status TEXT,
			nationality TEXT,
			fetched_at TEXT
		);
		CREATE TABLE IF NOT EXISTS products (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			company_slug TEXT REFERENCES companies(slug),
			name TEXT,
			description TEXT
		);
		CREATE TABLE IF NOT EXISTS investments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			company_slug TEXT REFERENCES companies(slug),
			round TEXT,
			amount TEXT,
			investor_types TEXT,
			fetched_at TEXT
		);
	`)
	return db, err
}

// ── Helpers ──────────────────────────────────────────────────

func toFloat(v interface{}) *float64 {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case float64:
		return &val
	case json.Number:
		f, _ := val.Float64()
		return &f
	case map[string]interface{}:
		// Paywalled - gated behind plan requirements
		return nil
	}
	return nil
}

func toInt(v interface{}) *int {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case float64:
		i := int(val)
		return &i
	case json.Number:
		i, _ := val.Int64()
		ii := int(i)
		return &ii
	case map[string]interface{}:
		// Try to extract "total" field from nested objects like investorCount
		if total, ok := val["total"]; ok {
			if f, ok := total.(float64); ok {
				i := int(f)
				return &i
			}
		}
		// Paywalled
		return nil
	}
	return nil
}

func fetchJSON(url string, target interface{}) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "pp-thevc/"+version)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

// ── Commands ─────────────────────────────────────────────────

func cmdScrape(source string, limit int) {
	db, err := openDB()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	url := fmt.Sprintf("%s/api/interaction/hits/organizations/rankings/%s", baseURL, source)

	var resp struct {
		DATE []RankedOrg `json:"DATE"`
		WEEK []RankedOrg `json:"WEEK"`
	}
	if err := fetchJSON(url, &resp); err != nil {
		fmt.Fprintf(os.Stderr, "error fetching rankings: %v\n", err)
		os.Exit(1)
	}

	items := append(resp.DATE, resp.WEEK...)
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}

	added := 0
	now := time.Now().Format(time.RFC3339)
	for _, item := range items {
		if item.ProfilePage == "" {
			continue
		}
		hitsJSON, _ := json.Marshal(item.TenWeeksHit)
		_, err := db.Exec(
			`INSERT OR REPLACE INTO rankings (slug, name, type, product_name, weekly_views, fetched_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			item.ProfilePage, item.Name, item.Type, item.ProductName, string(hitsJSON), now,
		)
		if err == nil {
			added++
		}
	}

	fmt.Fprintf(os.Stderr, "Added/updated %d companies to rankings\n", added)
}

func cmdFetch(slug string, all bool) {
	db, err := openDB()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	var slugs []string
	if all {
		rows, _ := db.Query("SELECT slug FROM rankings")
		defer rows.Close()
		for rows.Next() {
			var s string
			rows.Scan(&s)
			slugs = append(slugs, s)
		}
	} else if slug != "" {
		slugs = []string{slug}
	} else {
		rows, _ := db.Query(`SELECT slug FROM rankings WHERE slug NOT IN (SELECT slug FROM companies)`)
		defer rows.Close()
		for rows.Next() {
			var s string
			rows.Scan(&s)
			slugs = append(slugs, s)
		}
	}

	success, failed := 0, 0
	for _, s := range slugs {
		detail, err := fetchProfile(s)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ %s — %v\n", s, err)
			failed++
			continue
		}
		saveCompany(db, detail)
		fmt.Fprintf(os.Stderr, "  ✓ %s — %s (%s)\n", s, detail.Name, detail.Type)
		success++
		time.Sleep(300 * time.Millisecond)
	}
	fmt.Fprintf(os.Stderr, "\nDone: %d success, %d failed\n", success, failed)
}

func fetchProfile(slug string) (*Profile, error) {
	url := fmt.Sprintf("%s/api/information/organizations/profiles/%s", baseURL, slug)
	var p Profile
	if err := fetchJSON(url, &p); err != nil {
		return nil, err
	}
	if p.Name == "" {
		return nil, fmt.Errorf("empty profile")
	}
	return &p, nil
}

func saveCompany(db *sql.DB, p *Profile) {
	now := time.Now().Format(time.RFC3339)
	empCount := 0
	if len(p.EmployeeHistory) > 0 {
		empCount = int(p.EmployeeHistory[len(p.EmployeeHistory)-1].Count)
	}

	db.Exec(
		`INSERT OR REPLACE INTO companies (slug, name, org_type, corp_type, website, founded_date,
		 employee_count, total_funding, funding_count, last_round, investor_count,
		 avg_amount, max_amount, status, nationality, fetched_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ProfilePage, p.Name, p.Type, p.CorpType, p.Website, p.FoundedOn,
		empCount, toFloat(p.TotalFundingAmount), toInt(p.TotalFundingCount),
		p.LastRound, toInt(p.InvestorCount),
		toFloat(p.AvgAmount), toFloat(p.MaxAmount),
		p.Status, p.Nationality.Country, now,
	)

	db.Exec("DELETE FROM products WHERE company_slug = ?", p.ProfilePage)
	for _, prod := range p.Products {
		desc := prod.Description
		if desc == "" {
			desc = prod.Desc
		}
		db.Exec("INSERT INTO products (company_slug, name, description) VALUES (?, ?, ?)",
			p.ProfilePage, prod.Name, desc)
	}

	db.Exec("DELETE FROM investments WHERE company_slug = ?", p.ProfilePage)
	for _, f := range p.Fundings {
		types := strings.Join(f.InvestorTypes, ",")
		db.Exec("INSERT INTO investments (company_slug, round, amount, investor_types, fetched_at) VALUES (?, ?, ?, ?, ?)",
			p.ProfilePage, f.Round, f.Amount, types, now)
	}
}

func cmdSQL(query string, jsonOut bool) {
	db, err := openDB()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	rows, err := db.Query(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "query error: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	var results []map[string]interface{}

	for rows.Next() {
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		rows.Scan(valuePtrs...)

		row := make(map[string]interface{})
		for i, col := range cols {
			val := values[i]
			b, ok := val.([]byte)
			if ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}

	if len(results) == 0 {
		fmt.Println("(no results)")
		return
	}

	if jsonOut {
		out, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(out))
		return
	}

	// Table output
	widths := make([]int, len(cols))
	for i, c := range cols {
		widths[i] = len(c)
	}
	for _, row := range results {
		for i, c := range cols {
			s := fmt.Sprintf("%v", row[c])
			if s == "<nil>" {
				s = ""
			}
			if len(s) > widths[i] {
				widths[i] = len(s)
			}
		}
	}

	// Header
	for i, c := range cols {
		fmt.Print(pad(c, widths[i]))
		if i < len(cols)-1 {
			fmt.Print(" | ")
		}
	}
	fmt.Println()
	for i, w := range widths {
		fmt.Print(strings.Repeat("-", w))
		if i < len(cols)-1 {
			fmt.Print("-+-")
		}
	}
	fmt.Println()

	// Rows
	for _, row := range results {
		for i, c := range cols {
			s := fmt.Sprintf("%v", row[c])
			if s == "<nil>" {
				s = ""
			}
			fmt.Print(pad(s, widths[i]))
			if i < len(cols)-1 {
				fmt.Print(" | ")
			}
		}
		fmt.Println()
	}
}

func pad(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

func cmdExport(format string) {
	db, err := openDB()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT c.*, GROUP_CONCAT(p.name, '; ') as products
		FROM companies c LEFT JOIN products p ON c.slug = p.company_slug
		GROUP BY c.slug ORDER BY c.founded_date DESC`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "query error: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	var results []map[string]interface{}

	for rows.Next() {
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		rows.Scan(valuePtrs...)
		row := make(map[string]interface{})
		for i, col := range cols {
			if b, ok := values[i].([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = values[i]
			}
		}
		results = append(results, row)
	}

	switch format {
	case "json":
		out, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(out))
	case "csv":
		for i, c := range cols {
			if i > 0 {
				fmt.Print(",")
			}
			fmt.Printf("\"%s\"", c)
		}
		fmt.Println()
		for _, row := range results {
			for i, c := range cols {
				if i > 0 {
					fmt.Print(",")
				}
				s := fmt.Sprintf("%v", row[c])
				if s == "<nil>" {
					s = ""
				}
				s = strings.ReplaceAll(s, "\"", "\"\"")
				fmt.Printf("\"%s\"", s)
			}
			fmt.Println()
		}
	}
}

func cmdStats() {
	db, err := openDB()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	fmt.Println("theVC.kr Database Stats")
	fmt.Println(strings.Repeat("=", 40))

	tables := []string{"rankings", "companies", "products", "investments"}
	for _, t := range tables {
		var count int
		db.QueryRow("SELECT COUNT(*) FROM " + t).Scan(&count)
		fmt.Printf("  %s: %d rows\n", t, count)
	}
	fmt.Println()

	// Type breakdown
	rows, _ := db.Query("SELECT org_type, COUNT(*) as cnt FROM companies GROUP BY org_type")
	defer rows.Close()
	var hasTypes bool
	for rows.Next() {
		if !hasTypes {
			fmt.Println("Company types:")
			hasTypes = true
		}
		var t string
		var c int
		rows.Scan(&t, &c)
		fmt.Printf("  %s: %d\n", t, c)
	}
}

func cmdSearch(query string) {
	db, err := openDB()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	pattern := "%" + query + "%"
	rows, err := db.Query(`SELECT slug, name, org_type, website FROM companies 
		WHERE name LIKE ? OR slug LIKE ? ORDER BY name LIMIT 10`, pattern, pattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "query error: %v\n", err)
		return
	}
	defer rows.Close()

	fmt.Println(strings.Repeat("=", 60))
	found := false
	for rows.Next() {
		var slug, name, oType, website sql.NullString
		rows.Scan(&slug, &name, &oType, &website)
		fmt.Printf("%-20s %-15s %s\n", slug.String, oType.String, name.String)
		if website.Valid && website.String != "" {
			fmt.Printf("  %s\n", website.String)
		}
		found = true
	}
	if !found {
		fmt.Println("No results. Try 'scrape' and 'fetch' first.")
	}
}

func printUsage() {
	fmt.Println(`pp-thevc — Korean startup data from theVC.kr

Usage:
  pp-thevc scrape [--source ALL|STARTUP] [--limit N]
  pp-thevc fetch [--slug NAME] [--all]
  pp-thevc sql "QUERY" [--json]
  pp-thevc search QUERY
  pp-thevc export [--format csv|json]
  pp-thevc stats

No API key required — theVC.kr's profile API is fully open.
`)
}

// ── Main ────────────────────────────────────────────────────

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "scrape":
		source := "ALL"
		limit := 0
		for i := 0; i < len(args); i++ {
			switch args[i] {
			case "--source":
				if i+1 < len(args) {
					source = args[i+1]
					i++
				}
			case "--limit":
				if i+1 < len(args) {
					fmt.Sscanf(args[i+1], "%d", &limit)
					i++
				}
			}
		}
		cmdScrape(source, limit)

	case "fetch":
		slug := ""
		all := false
		for i := 0; i < len(args); i++ {
			switch args[i] {
			case "--slug":
				if i+1 < len(args) {
					slug = args[i+1]
					i++
				}
			case "--all":
				all = true
			}
		}
		cmdFetch(slug, all)

	case "sql":
		query := ""
		jsonOut := false
		for i := 0; i < len(args); i++ {
			switch args[i] {
			case "--json":
				jsonOut = true
			default:
				query = args[i]
			}
		}
		if query == "" {
			fmt.Fprintln(os.Stderr, "error: SQL query required")
			os.Exit(1)
		}
		cmdSQL(query, jsonOut)

	case "search":
		if len(args) == 0 {
			fmt.Fprintln(os.Stderr, "error: search query required")
			os.Exit(1)
		}
		cmdSearch(strings.Join(args, " "))

	case "export":
		format := "csv"
		for i := 0; i < len(args); i++ {
			if args[i] == "--format" && i+1 < len(args) {
				format = args[i+1]
				i++
			}
		}
		cmdExport(format)

	case "stats":
		cmdStats()

	default:
		printUsage()
		os.Exit(1)
	}
}
