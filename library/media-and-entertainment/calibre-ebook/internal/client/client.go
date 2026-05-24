package client

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"calibre-ebook-pp-cli/internal/config"
)

const BinaryResponseHeader = "X-Printing-Press-Binary-Response"

type cacheEntry struct {
	Data     []byte
	ModTime  time.Time
	TTL      time.Duration
}

type Client struct {
	Config       *config.Config
	DryRun       bool
	NoCache      bool
	Calibredb    string
	EbookMeta    string
	EbookConvert string
	EbookPolish  string
	LibraryPath  string
	cache        map[string]cacheEntry
}

func readCache(c *Client, key string) ([]byte, bool) {
	if c.NoCache || c.cache == nil {
		return nil, false
	}
	e, ok := c.cache[key]
	if !ok {
		return nil, false
	}
	if e.TTL > 0 && time.Since(e.ModTime) > e.TTL {
		return nil, false
	}
	return e.Data, true
}

func writeCache(c *Client, key string, data []byte, ttl time.Duration) {
	if c.cache == nil {
		c.cache = make(map[string]cacheEntry)
	}
	c.cache[key] = cacheEntry{Data: data, ModTime: time.Now(), TTL: ttl}
}

type APIError struct {
	Method     string
	Path       string
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s %s failed (exit %d): %s", e.Method, e.Path, e.StatusCode, e.Body)
}

func New(cfg *config.Config, timeout time.Duration, rateLimit float64) *Client {
	calibredb := resolveBinary("CALIBREDB_PATH", "calibredb")
	ebookMeta := resolveBinary("EBOOK_META_PATH", "ebook-meta")
	ebookConvert := resolveBinary("EBOOK_CONVERT_PATH", "ebook-convert")
	ebookPolish := resolveBinary("EBOOK_POLISH_PATH", "ebook-polish")

	libPath := ""
	if cfg != nil && cfg.LibraryPath != "" {
		libPath = cfg.LibraryPath
	} else if env := os.Getenv("CALIBRE_LIBRARY_PATH"); env != "" {
		libPath = env
	}

	return &Client{
		Config:       cfg,
		Calibredb:    calibredb,
		EbookMeta:    ebookMeta,
		EbookConvert: ebookConvert,
		EbookPolish:  ebookPolish,
		LibraryPath:  libPath,
	}
}

func resolveBinary(envVar, name string) string {
	if p := os.Getenv(envVar); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	candidates := []string{
		filepath.Join("/Applications/calibre.app/Contents/MacOS", name),
		filepath.Join("/usr/bin", name),
		filepath.Join("/usr/local/bin", name),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return name
}

func (c *Client) RateLimit() float64 { return 0 }

func (c *Client) ConfiguredTimeout() time.Duration { return 30 * time.Second }

func (c *Client) ProbeGet(path string) (int, error) {
	if _, err := os.Stat(c.Calibredb); err != nil {
		return 0, err
	}
	return 200, nil
}

func (c *Client) RunCalibredb(subcmd string, args ...string) ([]byte, int, error) {
	fullArgs := []string{subcmd}
	fullArgs = append(fullArgs, args...)
	if c.LibraryPath != "" {
		fullArgs = append(fullArgs, "--library-path", c.LibraryPath)
	}
	if c.DryRun {
		dryArgs := append([]string{c.Calibredb}, fullArgs...)
		fmt.Fprintf(os.Stderr, "  (dry run) %s\n", strings.Join(dryArgs, " "))
		return []byte(`{"dry_run": true}`), 0, nil
	}

	var stdout, stderr bytes.Buffer
	backoff := 500 * time.Millisecond
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		stdout.Reset()
		stderr.Reset()
		cmd := exec.Command(c.Calibredb, fullArgs...)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 1
			}
		}
		if exitCode == 0 {
			return stdout.Bytes(), 0, nil
		}
		stderrStr := stderr.String()
		isDBLock := strings.Contains(stderrStr, "database is locked") ||
			strings.Contains(stderrStr, "is currently busy") ||
			strings.Contains(stderrStr, "OperationalError")
		if isDBLock && attempt < maxRetries-1 {
			fmt.Fprintf(os.Stderr, "  calibre DB locked, retrying in %s (attempt %d/%d)...\n", backoff, attempt+1, maxRetries)
			time.Sleep(backoff)
			backoff = backoff * 2
			continue
		}
		return nil, exitCode, &APIError{
			Method:     "calibredb",
			Path:       subcmd,
			StatusCode: exitCode,
			Body:       strings.TrimSpace(stderrStr),
		}
	}
	return stdout.Bytes(), 0, nil
}

func (c *Client) runEbookMeta(args ...string) ([]byte, int, error) {
	if c.DryRun {
		dryArgs := append([]string{c.EbookMeta}, args...)
		fmt.Fprintf(os.Stderr, "  (dry run) %s\n", strings.Join(dryArgs, " "))
		return []byte(`{"dry_run": true}`), 0, nil
	}
	cmd := exec.Command(c.EbookMeta, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	if exitCode != 0 {
		return nil, exitCode, &APIError{
			Method:     "ebook-meta",
			Path:       args[0],
			StatusCode: exitCode,
			Body:       strings.TrimSpace(stderr.String()),
		}
	}
	return stdout.Bytes(), 0, nil
}

func (c *Client) runEbookPolish(flags []string, file string) ([]byte, int, error) {
	args := append(flags, file)
	if c.DryRun {
		dryArgs := append([]string{c.EbookPolish}, args...)
		fmt.Fprintf(os.Stderr, "  (dry run) %s\n", strings.Join(dryArgs, " "))
		return []byte(`{"dry_run": true}`), 0, nil
	}
	cmd := exec.Command(c.EbookPolish, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	result := map[string]any{
		"file":     file,
		"stdout":   strings.TrimSpace(stdout.String()),
		"stderr":   strings.TrimSpace(stderr.String()),
		"success":  exitCode == 0,
		"exitCode": exitCode,
	}
	resultJSON, _ := json.Marshal(result)
	if exitCode != 0 {
		return nil, exitCode, &APIError{
			Method:     "ebook-polish",
			Path:       file,
			StatusCode: exitCode,
			Body:       strings.TrimSpace(stderr.String()),
		}
	}
	return resultJSON, 0, nil
}

func (c *Client) runEbookConvert(input, output string, extraArgs ...string) ([]byte, int, error) {
	args := append([]string{input, output}, extraArgs...)
	if c.DryRun {
		dryArgs := append([]string{c.EbookConvert}, args...)
		fmt.Fprintf(os.Stderr, "  (dry run) %s\n", strings.Join(dryArgs, " "))
		return []byte(`{"dry_run": true}`), 0, nil
	}
	cmd := exec.Command(c.EbookConvert, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	result := map[string]any{
		"input":    input,
		"output":   output,
		"stdout":   strings.TrimSpace(stdout.String()),
		"stderr":   strings.TrimSpace(stderr.String()),
		"success":  exitCode == 0,
		"exitCode": exitCode,
	}
	resultJSON, _ := json.Marshal(result)
	if exitCode != 0 {
		return nil, exitCode, &APIError{
			Method:     "ebook-convert",
			Path:       input,
			StatusCode: exitCode,
			Body:       strings.TrimSpace(stderr.String()),
		}
	}
	return resultJSON, 0, nil
}

// Route dispatches a synthetic REST path to the right calibredb command.
// The command files still call Get/Post/Put/Delete with the same path strings
// from the OpenAPI spec — we intercept them here.
func (c *Client) route(method, path string, params map[string]string, body map[string]any) (json.RawMessage, int, error) {
	cacheKey := method + ":" + path
	if method == "GET" {
		if data, ok := readCache(c, cacheKey); ok {
			return data, 200, nil
		}
	}
	switch {
	case path == "/books" && method == "GET":
		return c.handleBooksList(params)
	case path == "/books/search" && method == "GET":
		return c.handleBooksSearch(params)
	case strings.HasPrefix(path, "/books/") && strings.Contains(path, "/embed-metadata") && method == "POST":
		id := extractID(path, "/books/", "/embed-metadata")
		return c.handleEmbedMetadata(id)
	case strings.HasPrefix(path, "/books/") && strings.Contains(path, "/formats") && method == "GET":
		id := extractID(path, "/books/", "/formats")
		return c.handleShowMetadata(id)
	case strings.HasPrefix(path, "/books/") && strings.Contains(path, "/formats") && method == "POST":
		id := extractID(path, "/books/", "/formats")
		return c.handleAddFormat(id, body)
	case strings.HasPrefix(path, "/books/") && strings.Contains(path, "/formats") && method == "DELETE":
		id := extractID(path, "/books/", "/formats")
		return c.handleRemoveFormat(id, params)
	case strings.HasPrefix(path, "/books/") && method == "GET":
		id := extractID(path, "/books/", "")
		return c.handleShowMetadata(id)
	case strings.HasPrefix(path, "/books/") && method == "PUT":
		id := extractID(path, "/books/", "")
		return c.handleSetMetadata(id, body)
	case strings.HasPrefix(path, "/books/") && method == "DELETE":
		id := extractID(path, "/books/", "")
		return c.handleRemoveBook(id)
	case path == "/books/add" && method == "POST":
		return c.handleAddBook(body)
	case path == "/library/check" && method == "GET":
		return c.handleLibraryCheck(params)
	case path == "/library/categories" && method == "GET":
		return c.handleLibraryCategories()
	case path == "/library/custom-columns" && method == "GET":
		return c.handleLibraryCustomColumns()
	case path == "/library/saved-searches" && method == "GET":
		return c.handleLibrarySavedSearches()
	case path == "/library/saved-searches" && method == "POST":
		return c.handleLibraryAddSavedSearch(body)
	case path == "/library/saved-searches" && method == "DELETE":
		return c.handleLibraryRemoveSavedSearch(params)
	case path == "/library/export" && method == "POST":
		return c.handleLibraryExport(body)
	case path == "/library/backup-metadata" && method == "POST":
		return c.handleLibraryBackupMetadata()
	case path == "/library/restore-database" && method == "POST":
		return c.handleLibraryRestoreDatabase()
	case path == "/fts/index" && method == "GET":
		return c.handleFtsIndexStatus()
	case path == "/fts/index" && method == "POST":
		return c.handleFtsIndexReindex(body)
	case path == "/fts/search" && method == "GET":
		return c.handleFtsSearch(params)
	case path == "/convert" && method == "POST":
		return c.handleConvert(body)
	case path == "/polish" && method == "POST":
		return c.handlePolish(body)
	case path == "/file-meta" && method == "GET":
		return c.handleFileMeta(params)
	case path == "/":
		return json.RawMessage(`{"status":"ok","backend":"calibredb"}`), 200, nil
	default:
		return nil, 404, fmt.Errorf("no calibredb route for %s %s", method, path)
	}
}

func (c *Client) cacheResult(method, path string, data json.RawMessage) {
	if method == "GET" && len(data) > 0 {
		ttl := c.ttlForPath(path)
		writeCache(c, method+":"+path, data, ttl)
	}
}

func (c *Client) ttlForPath(path string) time.Duration {
	if strings.HasPrefix(path, "/books/") && strings.Contains(path, "formats") {
		return 10 * time.Minute
	}
	if strings.HasPrefix(path, "/books/") {
		return 10 * time.Minute
	}
	if strings.HasPrefix(path, "/library/categories") || strings.HasPrefix(path, "/library/custom-columns") || strings.HasPrefix(path, "/library/saved-searches") {
		return 30 * time.Minute
	}
	if strings.HasPrefix(path, "/fts/") {
		return time.Hour
	}
	return 5 * time.Minute
}

// HTTP-method facade methods — these are what the generated command files call.

func (c *Client) Get(path string, params map[string]string) (json.RawMessage, error) {
	data, _, err := c.route("GET", path, params, nil)
	if err == nil {
		c.cacheResult("GET", path, data)
	}
	return data, err
}

func (c *Client) GetWithHeaders(path string, params map[string]string, _ map[string]string) (json.RawMessage, error) {
	data, _, err := c.route("GET", path, params, nil)
	if err == nil {
		c.cacheResult("GET", path, data)
	}
	return data, err
}

func (c *Client) GetNoCache(path string, params map[string]string) (json.RawMessage, error) {
	c.NoCache = true
	defer func() { c.NoCache = false }()
	data, _, err := c.route("GET", path, params, nil)
	return data, err
}

func (c *Client) GetWithHeadersNoCache(path string, params map[string]string, _ map[string]string) (json.RawMessage, error) {
	c.NoCache = true
	defer func() { c.NoCache = false }()
	data, _, err := c.route("GET", path, params, nil)
	return data, err
}

func (c *Client) Post(path string, body any) (json.RawMessage, int, error) {
	return c.route("POST", path, nil, bodyToMap(body))
}

func (c *Client) PostWithParams(path string, params map[string]string, body any) (json.RawMessage, int, error) {
	return c.route("POST", path, params, bodyToMap(body))
}

func (c *Client) PostWithHeaders(path string, body any, _ map[string]string) (json.RawMessage, int, error) {
	return c.route("POST", path, nil, bodyToMap(body))
}

func (c *Client) PostWithParamsAndHeaders(path string, params map[string]string, body any, _ map[string]string) (json.RawMessage, int, error) {
	return c.route("POST", path, params, bodyToMap(body))
}

func (c *Client) Delete(path string) (json.RawMessage, int, error) {
	return c.route("DELETE", path, nil, nil)
}

func (c *Client) DeleteWithParams(path string, params map[string]string) (json.RawMessage, int, error) {
	return c.route("DELETE", path, params, nil)
}

func (c *Client) DeleteWithHeaders(path string, _ map[string]string) (json.RawMessage, int, error) {
	return c.route("DELETE", path, nil, nil)
}

func (c *Client) DeleteWithParamsAndHeaders(path string, params map[string]string, _ map[string]string) (json.RawMessage, int, error) {
	return c.route("DELETE", path, params, nil)
}

func (c *Client) Put(path string, body any) (json.RawMessage, int, error) {
	return c.route("PUT", path, nil, bodyToMap(body))
}

func (c *Client) PutWithParams(path string, params map[string]string, body any) (json.RawMessage, int, error) {
	return c.route("PUT", path, params, bodyToMap(body))
}

func (c *Client) PutWithHeaders(path string, body any, _ map[string]string) (json.RawMessage, int, error) {
	return c.route("PUT", path, nil, bodyToMap(body))
}

func (c *Client) PutWithParamsAndHeaders(path string, params map[string]string, body any, _ map[string]string) (json.RawMessage, int, error) {
	return c.route("PUT", path, params, bodyToMap(body))
}

func (c *Client) Patch(path string, body any) (json.RawMessage, int, error) {
	return c.route("PATCH", path, nil, bodyToMap(body))
}

func (c *Client) PatchWithParams(path string, params map[string]string, body any) (json.RawMessage, int, error) {
	return c.route("PATCH", path, params, bodyToMap(body))
}

func (c *Client) PatchWithHeaders(path string, body any, _ map[string]string) (json.RawMessage, int, error) {
	return c.route("PATCH", path, nil, bodyToMap(body))
}

func (c *Client) PatchWithParamsAndHeaders(path string, params map[string]string, body any, _ map[string]string) (json.RawMessage, int, error) {
	return c.route("PATCH", path, params, bodyToMap(body))
}

// Handler implementations — each maps to a calibredb subcommand.

func (c *Client) handleBooksList(params map[string]string) (json.RawMessage, int, error) {
	args := []string{"--for-machine"}
	if v, ok := params["fields"]; ok && v != "" {
		args = append(args, "--fields", v)
	}
	if v, ok := params["search"]; ok && v != "" && v != "false" {
		args = append(args, "--search", v)
	}
	if v, ok := params["sort_by"]; ok && v != "" {
		args = append(args, "--sort-by", v)
	}
	if v, ok := params["limit"]; ok && v != "" && v != "0" {
		args = append(args, "--limit", v)
	}
	if v, ok := params["ascending"]; ok && v == "true" {
		args = append(args, "--ascending")
	}
	out, code, err := c.RunCalibredb("list", args...)
	if err != nil {
		return nil, code, err
	}
	return normalizeJSONArray(out), 200, nil
}

func (c *Client) handleBooksSearch(params map[string]string) (json.RawMessage, int, error) {
	query := params["query"]
	args := []string{query}
	if v, ok := params["limit"]; ok && v != "" && v != "0" {
		args = append(args, "--limit", v)
	}
	out, code, err := c.RunCalibredb("search", args...)
	if err != nil {
		return nil, code, err
	}
	ids := strings.Split(strings.TrimSpace(string(out)), ",")
	results := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		results = append(results, map[string]any{"id": id})
	}
	data, _ := json.Marshal(results)
	return data, 200, nil
}

func (c *Client) handleShowMetadata(id string) (json.RawMessage, int, error) {
	out, code, err := c.RunCalibredb("show_metadata", id)
	if err != nil {
		return nil, code, err
	}
	parsed := parseCalibredbKeyValue(string(out))
	data, _ := json.Marshal(parsed)
	return data, 200, nil
}

func (c *Client) handleSetMetadata(id string, body map[string]any) (json.RawMessage, int, error) {
	args := []string{id}
	for key, val := range body {
		args = append(args, "--field", fmt.Sprintf("%s:%v", key, val))
	}
	out, code, err := c.RunCalibredb("set_metadata", args...)
	if err != nil {
		return nil, code, err
	}
	result := map[string]any{
		"id":      id,
		"success": true,
		"output":  strings.TrimSpace(string(out)),
	}
	data, _ := json.Marshal(result)
	return data, 200, nil
}

func (c *Client) handleRemoveBook(id string) (json.RawMessage, int, error) {
	out, code, err := c.RunCalibredb("remove", id)
	if err != nil {
		return nil, code, err
	}
	result := map[string]any{"id": id, "success": true, "output": strings.TrimSpace(string(out))}
	data, _ := json.Marshal(result)
	return data, 200, nil
}

func (c *Client) handleAddBook(body map[string]any) (json.RawMessage, int, error) {
	path, _ := body["path"].(string)
	args := []string{path}
	if recurse, ok := body["recurse"].(bool); ok && recurse {
		args = append(args, "--recurse")
	}
	out, code, err := c.RunCalibredb("add", args...)
	if err != nil {
		return nil, code, err
	}
	result := map[string]any{"path": path, "success": true, "output": strings.TrimSpace(string(out))}
	data, _ := json.Marshal(result)
	return data, 200, nil
}

func (c *Client) handleEmbedMetadata(id string) (json.RawMessage, int, error) {
	out, code, err := c.RunCalibredb("embed_metadata", id)
	if err != nil {
		return nil, code, err
	}
	result := map[string]any{"id": id, "success": true, "output": strings.TrimSpace(string(out))}
	data, _ := json.Marshal(result)
	return data, 200, nil
}

func (c *Client) handleAddFormat(id string, body map[string]any) (json.RawMessage, int, error) {
	path, _ := body["path"].(string)
	out, code, err := c.RunCalibredb("add_format", id, path)
	if err != nil {
		return nil, code, err
	}
	result := map[string]any{"id": id, "path": path, "success": true, "output": strings.TrimSpace(string(out))}
	data, _ := json.Marshal(result)
	return data, 200, nil
}

func (c *Client) handleRemoveFormat(id string, params map[string]string) (json.RawMessage, int, error) {
	format := params["format"]
	out, code, err := c.RunCalibredb("remove_format", id, format)
	if err != nil {
		return nil, code, err
	}
	result := map[string]any{"id": id, "format": format, "success": true, "output": strings.TrimSpace(string(out))}
	data, _ := json.Marshal(result)
	return data, 200, nil
}

func (c *Client) handleLibraryCheck(params map[string]string) (json.RawMessage, int, error) {
	args := []string{}
	if v, ok := params["report"]; ok && v != "" && v != "false" {
		args = append(args, "--report", v)
	}
	if v, ok := params["csv"]; ok && v == "true" {
		args = append(args, "--csv")
	}
	out, code, err := c.RunCalibredb("check_library", args...)
	if err != nil {
		return nil, code, err
	}
	text := strings.TrimSpace(string(out))
	if v, ok := params["csv"]; ok && v == "true" {
		parsed, parseErr := parseCSVToJSON(text)
		if parseErr == nil {
			return parsed, 200, nil
		}
	}
	result := map[string]any{"report": text, "format": "text"}
	data, _ := json.Marshal(result)
	return data, 200, nil
}

func (c *Client) handleLibraryCategories() (json.RawMessage, int, error) {
	out, code, err := c.RunCalibredb("list_categories")
	if err != nil {
		return nil, code, err
	}
	result := map[string]any{"output": strings.TrimSpace(string(out))}
	data, _ := json.Marshal(result)
	return data, 200, nil
}

func (c *Client) handleLibraryCustomColumns() (json.RawMessage, int, error) {
	out, code, err := c.RunCalibredb("custom_columns")
	if err != nil {
		return nil, code, err
	}
	result := map[string]any{"output": strings.TrimSpace(string(out))}
	data, _ := json.Marshal(result)
	return data, 200, nil
}

func (c *Client) handleLibrarySavedSearches() (json.RawMessage, int, error) {
	out, code, err := c.RunCalibredb("saved_searches", "list")
	if err != nil {
		return nil, code, err
	}
	result := map[string]any{"output": strings.TrimSpace(string(out))}
	data, _ := json.Marshal(result)
	return data, 200, nil
}

func (c *Client) handleLibraryAddSavedSearch(body map[string]any) (json.RawMessage, int, error) {
	name, _ := body["name"].(string)
	expression, _ := body["expression"].(string)
	out, code, err := c.RunCalibredb("saved_searches", "add", name, expression)
	if err != nil {
		return nil, code, err
	}
	result := map[string]any{"name": name, "success": true, "output": strings.TrimSpace(string(out))}
	data, _ := json.Marshal(result)
	return data, 200, nil
}

func (c *Client) handleLibraryRemoveSavedSearch(params map[string]string) (json.RawMessage, int, error) {
	name := params["name"]
	out, code, err := c.RunCalibredb("saved_searches", "remove", name)
	if err != nil {
		return nil, code, err
	}
	result := map[string]any{"name": name, "success": true, "output": strings.TrimSpace(string(out))}
	data, _ := json.Marshal(result)
	return data, 200, nil
}

func (c *Client) handleLibraryExport(body map[string]any) (json.RawMessage, int, error) {
	ids, _ := body["ids"].(string)
	toDir, _ := body["to_dir"].(string)
	args := []string{ids, "--to-dir", toDir}
	if formats, ok := body["formats"].(string); ok && formats != "" {
		args = append(args, "--formats", formats)
	}
	out, code, err := c.RunCalibredb("export", args...)
	if err != nil {
		return nil, code, err
	}
	result := map[string]any{"ids": ids, "to_dir": toDir, "success": true, "output": strings.TrimSpace(string(out))}
	data, _ := json.Marshal(result)
	return data, 200, nil
}

func (c *Client) handleLibraryBackupMetadata() (json.RawMessage, int, error) {
	out, code, err := c.RunCalibredb("backup_metadata", "--all")
	if err != nil {
		return nil, code, err
	}
	result := map[string]any{"success": true, "output": strings.TrimSpace(string(out))}
	data, _ := json.Marshal(result)
	return data, 200, nil
}

func (c *Client) handleLibraryRestoreDatabase() (json.RawMessage, int, error) {
	out, code, err := c.RunCalibredb("restore_database", "--really-do-it")
	if err != nil {
		return nil, code, err
	}
	result := map[string]any{"success": true, "output": strings.TrimSpace(string(out))}
	data, _ := json.Marshal(result)
	return data, 200, nil
}

func (c *Client) handleFtsIndexStatus() (json.RawMessage, int, error) {
	out, code, err := c.RunCalibredb("fts_index", "status")
	if err != nil {
		return nil, code, err
	}
	result := map[string]any{"status": strings.TrimSpace(string(out))}
	data, _ := json.Marshal(result)
	return data, 200, nil
}

func (c *Client) handleFtsIndexReindex(body map[string]any) (json.RawMessage, int, error) {
	args := []string{"reindex"}
	if ids, ok := body["ids"].(string); ok && ids != "" {
		for _, id := range strings.Split(ids, ",") {
			args = append(args, strings.TrimSpace(id))
		}
	}
	if speed, ok := body["speed"].(string); ok && speed == "fast" {
		args = append(args, "--indexing-speed", "fast")
	}
	if wait, ok := body["wait"].(bool); ok && wait {
		args = append(args, "--wait-for-completion")
	}
	out, code, err := c.RunCalibredb("fts_index", args...)
	if err != nil {
		return nil, code, err
	}
	result := map[string]any{"success": true, "output": strings.TrimSpace(string(out))}
	data, _ := json.Marshal(result)
	return data, 200, nil
}

func (c *Client) handleFtsSearch(params map[string]string) (json.RawMessage, int, error) {
	query := params["query"]
	args := []string{query}
	if v, ok := params["include_snippets"]; ok && v == "true" {
		args = append(args, "--include-snippets")
	}
	if v, ok := params["restrict_to"]; ok && v != "" {
		args = append(args, "--restrict-to", v)
	}
	args = append(args, "--output-format", "json")
	out, code, err := c.RunCalibredb("fts_search", args...)
	if err != nil {
		return nil, code, err
	}
	return normalizeJSONArray(out), 200, nil
}

func (c *Client) handleConvert(body map[string]any) (json.RawMessage, int, error) {
	input, _ := body["input"].(string)
	output, _ := body["output"].(string)
	if input == "" || output == "" {
		return nil, 400, fmt.Errorf("input and output paths required")
	}
	var extraArgs []string
	if v, ok := body["title"].(string); ok && v != "" {
		extraArgs = append(extraArgs, "--title", v)
	}
	if v, ok := body["authors"].(string); ok && v != "" {
		extraArgs = append(extraArgs, "--authors", v)
	}
	if v, ok := body["series"].(string); ok && v != "" {
		extraArgs = append(extraArgs, "--series", v)
	}
	if v, ok := body["series_index"].(float64); ok && v != 0 {
		extraArgs = append(extraArgs, "--series-index", strconv.FormatFloat(v, 'f', -1, 64))
	}
	if v, ok := body["language"].(string); ok && v != "" {
		extraArgs = append(extraArgs, "--language", v)
	}
	if v, ok := body["smarten_punctuation"].(bool); ok && v {
		extraArgs = append(extraArgs, "--smarten-punctuation")
	}
	return c.runEbookConvert(input, output, extraArgs...)
}

func (c *Client) handlePolish(body map[string]any) (json.RawMessage, int, error) {
	file, _ := body["path"].(string)
	if file == "" {
		return nil, 400, fmt.Errorf("path required")
	}
	var flags []string
	if v, ok := body["smarten_punctuation"].(bool); ok && v {
		flags = append(flags, "--smarten-punctuation")
	}
	if v, ok := body["subset_fonts"].(bool); ok && v {
		flags = append(flags, "--subset-fonts")
	}
	if v, ok := body["embed_fonts"].(bool); ok && v {
		flags = append(flags, "--embed-fonts")
	}
	if v, ok := body["compress_images"].(bool); ok && v {
		flags = append(flags, "--compress-images")
	}
	if v, ok := body["remove_unused_css"].(bool); ok && v {
		flags = append(flags, "--remove-unused-css")
	}
	if v, ok := body["upgrade_book"].(bool); ok && v {
		flags = append(flags, "--upgrade-book")
	}
	if v, ok := body["download_external_resources"].(bool); ok && v {
		flags = append(flags, "--download-external-resources")
	}
	if v, ok := body["add_soft_hyphens"].(bool); ok && v {
		flags = append(flags, "--add-soft-hyphens")
	}
	if v, ok := body["add_jacket"].(bool); ok && v {
		flags = append(flags, "--jacket")
	}
	if v, ok := body["cover"].(string); ok && v != "" {
		flags = append(flags, "--cover", v)
	}
	return c.runEbookPolish(flags, file)
}

func (c *Client) handleFileMeta(params map[string]string) (json.RawMessage, int, error) {
	path := params["path"]
	if path == "" {
		return nil, 400, fmt.Errorf("path required")
	}
	out, code, err := c.runEbookMeta(path)
	if err != nil {
		return nil, code, err
	}
	parsed := parseEbookMetaOutput(string(out))
	data, _ := json.Marshal(parsed)
	return data, 200, nil
}

// Parsing helpers

func extractID(path, prefix, suffix string) string {
	s := strings.TrimPrefix(path, prefix)
	s = strings.TrimSuffix(s, suffix)
	return s
}

func bodyToMap(body any) map[string]any {
	if body == nil {
		return nil
	}
	if m, ok := body.(map[string]any); ok {
		return m
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	return m
}

func normalizeJSONArray(raw []byte) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return json.RawMessage(`[]`)
	}
	if trimmed[0] == '[' || trimmed[0] == '{' {
		return json.RawMessage(trimmed)
	}
	return json.RawMessage(raw)
}

// parseCalibredbKeyValue parses key-value output like:
//
//	Title     : Dune
//	Author(s) : Frank Herbert
func parseCalibredbKeyValue(raw string) map[string]any {
	result := map[string]any{}
	for _, line := range strings.Split(raw, "\n") {
		if idx := strings.Index(line, ":"); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			if key != "" {
				result[key] = val
			}
		}
	}
	return result
}

// parseEbookMetaOutput parses ebook-meta output like:
//
//	Title               : Dune
//	Author(s)           : Frank Herbert
//	Publisher           : Ace Books
func parseEbookMetaOutput(raw string) map[string]any {
	return parseCalibredbKeyValue(raw)
}

// parseCSVToJSON converts CSV text to a JSON array of objects.
func parseCSVToJSON(csvText string) (json.RawMessage, error) {
	reader := csv.NewReader(strings.NewReader(csvText))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return json.RawMessage(`[]`), nil
	}
	headers := records[0]
	var results []map[string]string
	for _, row := range records[1:] {
		obj := map[string]string{}
		for i, val := range row {
			if i < len(headers) {
				obj[strings.TrimSpace(headers[i])] = strings.TrimSpace(val)
			}
		}
		results = append(results, obj)
	}
	data, err := json.Marshal(results)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

// sanitizeJSONResponse and maskToken kept for interface compat.
func sanitizeJSONResponse(body []byte) []byte { return body }
func maskToken(token string) string {
	if len(token) <= 4 {
		return "****"
	}
	return "****" + token[len(token)-4:]
}

func truncateBody(b []byte) string {
	const maxBytes = 4096
	if len(b) <= maxBytes {
		return string(b)
	}
	return strings.ToValidUTF8(string(b[:maxBytes]), "") + "..."
}
