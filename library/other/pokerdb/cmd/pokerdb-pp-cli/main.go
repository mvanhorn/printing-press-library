package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
)

const version = "0.1.0"

type row struct {
	Player    string `json:"player,omitempty"`
	PlayerID  string `json:"player_id,omitempty"`
	Country   string `json:"country,omitempty"`
	Earnings  string `json:"earnings,omitempty"`
	Rank      string `json:"rank,omitempty"`
	Event     string `json:"event,omitempty"`
	EventID   string `json:"event_id,omitempty"`
	Date      string `json:"date,omitempty"`
	Venue     string `json:"venue,omitempty"`
	City      string `json:"city,omitempty"`
	Place     string `json:"place,omitempty"`
	Buyin     string `json:"buyin,omitempty"`
	SourceURL string `json:"source_url,omitempty"`
	Kind      string `json:"kind,omitempty"`
	Raw       map[string]string `json:"raw,omitempty"`
}

type options struct {
	file    string
	out     string
	json    bool
	compact bool
	limit   int
	country string
	year    string
}

var aliases = map[string][]string{
	"player":     {"player", "name", "player_name", "full_name"},
	"player_id":  {"player_id", "playerid", "id", "thm_id"},
	"country":    {"country", "nationality", "flag"},
	"earnings":   {"earnings", "total_earnings", "winnings", "prize", "amount"},
	"rank":       {"rank", "ranking"},
	"event":      {"event", "tournament", "tournament_name"},
	"event_id":   {"event_id", "eventid", "tournament_id"},
	"date":       {"date", "start_date", "event_date", "year"},
	"venue":      {"venue", "casino", "festival"},
	"city":       {"city", "location"},
	"place":      {"place", "position", "finish"},
	"buyin":      {"buyin", "buy_in", "buy-in"},
	"source_url": {"source_url", "url", "link"},
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitCode(err))
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printHelp(stdout)
		return nil
	}
	if args[0] == "version" || args[0] == "--version" || args[0] == "-v" {
		fmt.Fprintln(stdout, version)
		return nil
	}

	switch args[0] {
	case "doctor":
		opts, err := parseOptions(args[1:])
		if err != nil {
			return err
		}
		return doctor(opts, stdout)
	case "schema":
		opts, err := parseOptions(args[1:])
		if err != nil {
			return err
		}
		return printSchema(opts, stdout)
	case "import":
		opts, rest, err := parseOptionsRest(args[1:])
		if err != nil {
			return err
		}
		if len(rest) != 1 {
			return usageError("import requires exactly one CSV or JSON input file")
		}
		return importRows(rest[0], opts, stdout)
	case "players", "events", "results":
		return searchCommand(args[0], args[1:], stdout)
	default:
		return usageError("unknown command: " + args[0])
	}
}

func parseOptions(args []string) (options, error) {
	opts, _, err := parseOptionsRest(args)
	return opts, err
}

func parseOptionsRest(args []string) (options, []string, error) {
	opts := options{limit: 25}
	rest := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			rest = append(rest, arg)
			continue
		}
		name, value, hasValue := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
		readValue := func() (string, error) {
			if hasValue {
				return value, nil
			}
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
				return "", usageError("--" + name + " requires a value")
			}
			i++
			return args[i], nil
		}
		switch name {
		case "file":
			v, err := readValue()
			if err != nil {
				return opts, nil, err
			}
			opts.file = v
		case "out":
			v, err := readValue()
			if err != nil {
				return opts, nil, err
			}
			opts.out = v
		case "json":
			opts.json = true
		case "compact":
			opts.compact = true
		case "limit":
			v, err := readValue()
			if err != nil {
				return opts, nil, err
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return opts, nil, usageError("--limit must be an integer")
			}
			opts.limit = n
		case "country":
			v, err := readValue()
			if err != nil {
				return opts, nil, err
			}
			opts.country = v
		case "year":
			v, err := readValue()
			if err != nil {
				return opts, nil, err
			}
			opts.year = v
		default:
			return opts, nil, usageError("unknown flag: --" + name)
		}
	}
	if opts.file == "" {
		opts.file = os.Getenv("POKERDB_FILE")
	}
	if opts.file == "" {
		opts.file = "pokerdb.local.json"
	}
	if opts.limit < 1 {
		opts.limit = 25
	}
	return opts, rest, nil
}

func searchCommand(scope string, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageError(scope + " requires an action: search or list")
	}
	action := args[0]
	if action != "search" && action != "list" {
		return usageError(scope + " action must be search or list")
	}
	opts, rest, err := parseOptionsRest(args[1:])
	if err != nil {
		return err
	}
	query := strings.Join(rest, " ")
	if action == "search" && strings.TrimSpace(query) == "" {
		return usageError(scope + " search requires a query")
	}
	rows, err := loadRows(opts.file)
	if err != nil {
		return err
	}
	found := filterRows(rows, scope, query, opts)
	if len(found) > opts.limit {
		found = found[:opts.limit]
	}
	return printRows(found, opts, stdout)
}

func loadRows(path string) ([]row, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, dataError(fmt.Sprintf("data file not found: %s", path))
	}
	defer f.Close()

	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		var decoded any
		if err := json.NewDecoder(f).Decode(&decoded); err != nil {
			return nil, dataError("invalid JSON: " + err.Error())
		}
		return rowsFromJSON(decoded)
	default:
		return rowsFromCSV(f)
	}
}

func rowsFromJSON(v any) ([]row, error) {
	var items []any
	switch x := v.(type) {
	case []any:
		items = x
	case map[string]any:
		for _, key := range []string{"rows", "results", "players", "events"} {
			if arr, ok := x[key].([]any); ok {
				items = arr
				break
			}
		}
	}
	if items == nil {
		return nil, dataError("JSON must be an array or an object with rows/results/players/events")
	}

	out := make([]row, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		raw := make(map[string]string, len(m))
		for k, v := range m {
			raw[k] = strings.TrimSpace(fmt.Sprint(v))
		}
		out = append(out, normalize(raw))
	}
	return out, nil
}

func rowsFromCSV(r io.Reader) ([]row, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	records, err := cr.ReadAll()
	if err != nil {
		return nil, dataError("invalid CSV: " + err.Error())
	}
	if len(records) == 0 {
		return nil, nil
	}
	headers := records[0]
	out := make([]row, 0, len(records)-1)
	for _, record := range records[1:] {
		raw := make(map[string]string, len(headers))
		empty := true
		for i, header := range headers {
			value := ""
			if i < len(record) {
				value = strings.TrimSpace(record[i])
			}
			if value != "" {
				empty = false
			}
			raw[header] = value
		}
		if !empty {
			out = append(out, normalize(raw))
		}
	}
	return out, nil
}

func normalize(raw map[string]string) row {
	index := make(map[string]string, len(raw))
	for k, v := range raw {
		index[normalizeKey(k)] = v
	}
	get := func(field string) string {
		for _, name := range aliases[field] {
			if v := index[normalizeKey(name)]; v != "" {
				return v
			}
		}
		return ""
	}
	r := row{
		Player: get("player"), PlayerID: get("player_id"), Country: get("country"),
		Earnings: get("earnings"), Rank: get("rank"), Event: get("event"), EventID: get("event_id"),
		Date: get("date"), Venue: get("venue"), City: get("city"), Place: get("place"),
		Buyin: get("buyin"), SourceURL: get("source_url"), Raw: raw,
	}
	switch {
	case r.Player != "" && r.Event != "":
		r.Kind = "result"
	case r.Event != "":
		r.Kind = "event"
	default:
		r.Kind = "player"
	}
	return r
}

func normalizeKey(s string) string {
	var b strings.Builder
	lastUnderscore := false
	for _, ch := range strings.ToLower(strings.TrimSpace(s)) {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			b.WriteRune(ch)
			lastUnderscore = false
		} else if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

func filterRows(rows []row, scope, query string, opts options) []row {
	query = strings.ToLower(strings.TrimSpace(query))
	country := strings.ToLower(strings.TrimSpace(opts.country))
	year := strings.ToLower(strings.TrimSpace(opts.year))
	out := make([]row, 0, len(rows))
	for _, r := range rows {
		if !matchesScope(r, scope) {
			continue
		}
		if country != "" && strings.ToLower(r.Country) != country {
			continue
		}
		if year != "" && !strings.Contains(strings.ToLower(r.Date), year) {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(searchText(r)), query) {
			continue
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Player+out[i].Event+out[i].Date < out[j].Player+out[j].Event+out[j].Date
	})
	return out
}

func matchesScope(r row, scope string) bool {
	switch scope {
	case "players":
		return r.Player != ""
	case "events":
		return r.Event != ""
	case "results":
		return r.Player != "" && r.Event != ""
	default:
		return false
	}
}

func searchText(r row) string {
	return strings.Join([]string{r.Player, r.PlayerID, r.Country, r.Earnings, r.Rank, r.Event, r.EventID, r.Date, r.Venue, r.City, r.Place, r.Buyin, r.SourceURL}, " ")
}

func printRows(rows []row, opts options, w io.Writer) error {
	if opts.json {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if opts.compact {
			return enc.Encode(compactRows(rows))
		}
		return enc.Encode(rows)
	}
	if len(rows) == 0 {
		fmt.Fprintln(w, "No rows found.")
		return nil
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PLAYER\tCOUNTRY\tEARNINGS\tEVENT\tDATE\tPLACE")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", r.Player, r.Country, r.Earnings, r.Event, r.Date, r.Place)
	}
	return tw.Flush()
}

func compactRows(rows []row) []map[string]string {
	out := make([]map[string]string, 0, len(rows))
	for _, r := range rows {
		m := map[string]string{}
		for key, value := range map[string]string{
			"player": r.Player, "country": r.Country, "earnings": r.Earnings, "rank": r.Rank,
			"event": r.Event, "date": r.Date, "place": r.Place, "venue": r.Venue, "city": r.City,
			"source_url": r.SourceURL,
		} {
			if value != "" {
				m[key] = value
			}
		}
		out = append(out, m)
	}
	return out
}

func importRows(input string, opts options, w io.Writer) error {
	rows, err := loadRows(input)
	if err != nil {
		return err
	}
	out, err := os.Create(opts.out)
	if err != nil {
		return err
	}
	defer out.Close()
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(compactRows(rows)); err != nil {
		return err
	}
	fmt.Fprintf(w, "Imported %d rows to %s\n", len(rows), opts.out)
	return nil
}

func doctor(opts options, w io.Writer) error {
	abs, _ := filepath.Abs(opts.file)
	_, statErr := os.Stat(opts.file)
	status := map[string]any{
		"cli":              "pokerdb-pp-cli",
		"version":          version,
		"mode":             "local-only",
		"data_file":        abs,
		"data_file_exists": statErr == nil,
		"network":          "disabled",
		"api":              "none",
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(status)
}

func printSchema(opts options, w io.Writer) error {
	schema := map[string]any{
		"accepted_formats": []string{"csv", "json"},
		"env": map[string]string{
			"POKERDB_FILE": "Default local CSV/JSON export path",
		},
		"fields": aliases,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(schema)
}

func printHelp(w io.Writer) {
	fmt.Fprint(w, `pokerdb-pp-cli `+version+`

Local-only PokerDB export explorer. It reads CSV/JSON files supplied by the
user and never calls PokerDB, Hendon Mob, or any other network API.

Usage:
  pokerdb-pp-cli players search <query> --file ./export.csv [--json] [--limit 25]
  pokerdb-pp-cli players list --file ./export.csv
  pokerdb-pp-cli events search <query> --file ./export.csv [--country Canada] [--year 2025]
  pokerdb-pp-cli results search <query> --file ./export.csv [--json --compact]
  pokerdb-pp-cli import ./export.csv --out ./pokerdb.local.json
  pokerdb-pp-cli schema --json
  pokerdb-pp-cli doctor --file ./pokerdb.local.json

Data:
  Set --file or POKERDB_FILE to a local CSV/JSON export.
  No API key exists because this CLI does not use an API.
`)
}

type codedError struct {
	code int
	msg  string
}

func (e codedError) Error() string { return e.msg }

func usageError(msg string) error { return codedError{code: 2, msg: msg} }
func dataError(msg string) error  { return codedError{code: 3, msg: msg} }

func exitCode(err error) int {
	var coded codedError
	if errors.As(err, &coded) {
		return coded.code
	}
	return 1
}
