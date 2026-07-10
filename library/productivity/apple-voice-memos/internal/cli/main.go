package cli

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

const (
	cliName                = "apple-voice-memos-pp-cli"
	coreDataEpochUnix      = 978307200 // 2001-01-01T00:00:00Z
	maxTranscriptAtomBytes = 8 << 20
)

var version = "dev"

type config struct {
	DBPath        string
	RecordingsDir string
	JSON          bool
	Agent         bool
	NoColor       bool
	DaemonWait    time.Duration
	AppWait       time.Duration
	PollInterval  time.Duration
	SettleWait    time.Duration
}

type memo struct {
	ID            int64     `json:"id"`
	UUID          string    `json:"uuid,omitempty"`
	Title         string    `json:"title"`
	Date          time.Time `json:"date"`
	DurationSec   float64   `json:"duration_seconds"`
	Duration      string    `json:"duration"`
	Filename      string    `json:"filename"`
	Path          string    `json:"path"`
	HasTranscript bool      `json:"has_transcript"`
	Exists        bool      `json:"exists"`
}

var cfg config

func Main() int {
	if err := buildRootCommand().Execute(); err != nil {
		if cfg.Agent {
			b, marshalErr := json.Marshal(map[string]any{"schema_version": 1, "error": err.Error()})
			if marshalErr == nil {
				fmt.Fprintln(os.Stderr, string(b))
			}
		} else {
			fmt.Fprintln(os.Stderr, "Error:", err)
		}
		return 1
	}
	return 0
}

func buildRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           cliName,
		Short:         "Local, read-only CLI for Apple Voice Memos on macOS",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Long: `Local, read-only CLI for Apple Voice Memos on macOS.

Reads the Voice Memos SQLite store and recording media files synced by iCloud. By default it
uses Apple's modern path:
~/Library/Group Containers/group.com.apple.VoiceMemos.shared/Recordings/CloudRecordings.db

No network calls. No writes to Apple's database. Export copies audio files only when requested.`,
	}
	root.PersistentFlags().StringVar(&cfg.DBPath, "db", "", "path to CloudRecordings.db")
	root.PersistentFlags().StringVar(&cfg.RecordingsDir, "recordings-dir", "", "directory containing Voice Memos .m4a files")
	root.PersistentFlags().BoolVar(&cfg.JSON, "json", false, "emit JSON")
	root.PersistentFlags().BoolVar(&cfg.Agent, "agent", false, "agent mode: JSON, no color, non-interactive")
	root.PersistentFlags().BoolVar(&cfg.NoColor, "no-color", false, "disable color output")
	root.PersistentFlags().DurationVar(&cfg.DaemonWait, "daemon-wait", 3*time.Second, "time to wait for voicememod before hidden-app fallback")
	root.PersistentFlags().DurationVar(&cfg.AppWait, "app-wait", 12*time.Second, "time to wait after launching Voice Memos hidden")
	root.PersistentFlags().DurationVar(&cfg.PollInterval, "poll-interval", 500*time.Millisecond, "store-change polling interval")
	root.PersistentFlags().DurationVar(&cfg.SettleWait, "settle-wait", 2*time.Second, "additional wait after the store first changes")
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if cfg.Agent {
			cfg.JSON = true
			cfg.NoColor = true
		}
		if cfg.DaemonWait < 0 || cfg.AppWait < 0 || cfg.SettleWait < 0 || cfg.PollInterval <= 0 {
			return errors.New("sync waits must be non-negative and poll-interval must be positive")
		}
		return nil
	}
	root.AddCommand(cmdDoctor(), cmdSync(), cmdList(), cmdRecent(), cmdTranscript(), cmdExport(), cmdAgentContext(), cmdWhich())
	return root
}

func cmdDoctor() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check access to the local Apple Voice Memos store",
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath, recDir := resolvePaths()
			out := map[string]any{"schema_version": 1, "cli": cliName, "version": version, "db_path": dbPath, "recordings_dir": recDir}
			if _, err := os.Stat(dbPath); err != nil {
				out["ok"] = false
				out["error"] = err.Error()
				if !cfg.Agent {
					printJSON(out)
				}
				return fmt.Errorf("voice memos database not accessible: %w", err)
			}
			db, err := openDB(dbPath)
			if err != nil {
				out["ok"] = false
				out["error"] = err.Error()
				if !cfg.Agent {
					printJSON(out)
				}
				return err
			}
			defer func() { _ = db.Close() }()
			if err := validateVoiceMemosSchema(db); err != nil {
				out["ok"] = false
				out["schema_compatible"] = false
				out["error"] = err.Error()
				if !cfg.Agent {
					printJSON(out)
				}
				return err
			}
			var count int
			if err := db.QueryRow("SELECT count(*) FROM ZCLOUDRECORDING").Scan(&count); err != nil {
				out["ok"] = false
				out["error"] = err.Error()
				if !cfg.Agent {
					printJSON(out)
				}
				return err
			}
			out["ok"] = true
			out["schema_compatible"] = true
			out["recording_count"] = count
			printJSON(out)
			return nil
		},
	}
}

func cmdSync() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Refresh the local Voice Memos store through voicememod with a hidden-app fallback",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := runSync(cmd.Context())
			if err != nil {
				return err
			}
			printJSON(result)
			return nil
		},
	}
}

func cmdList() *cobra.Command {
	var limit, offset int
	var search, after, before, format string
	var fresh bool
	c := &cobra.Command{
		Use:   "list",
		Short: "List Voice Memos metadata",
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit < 0 {
				return errors.New("--limit must be non-negative")
			}
			if offset < 0 {
				return errors.New("--offset must be non-negative")
			}
			if format != "table" && format != "json" && format != "csv" {
				return fmt.Errorf("unsupported --format %q; use table, json, or csv", format)
			}
			var refreshResult *syncResult
			if fresh {
				result, err := runSync(cmd.Context())
				if err != nil {
					return fmt.Errorf("refresh Voice Memos store: %w", err)
				}
				refreshResult = &result
				if result.Warning != "" && !cfg.JSON {
					cmd.PrintErrf("sync: %s\n", result.Warning)
				}
			}
			memos, err := queryMemos(limit, offset, search, after, before)
			if err != nil {
				return err
			}
			switch format {
			case "json":
				mode := "cached"
				if fresh {
					mode = "fresh"
				}
				printJSON(newMemoListOutput(memos, mode, refreshResult))
			case "csv":
				return printCSV(memos)
			default:
				printTable(memos)
			}
			return nil
		},
	}
	c.Flags().IntVar(&limit, "limit", 20, "maximum rows")
	c.Flags().IntVar(&offset, "offset", 0, "skip rows")
	c.Flags().StringVar(&search, "search", "", "case-insensitive title substring")
	c.Flags().StringVar(&after, "after", "", "only recordings on/after YYYY-MM-DD")
	c.Flags().StringVar(&before, "before", "", "only recordings on/before YYYY-MM-DD")
	c.Flags().StringVar(&format, "format", "table", "output format: table, json, csv")
	c.Flags().BoolVar(&fresh, "fresh", false, "refresh through iCloud before listing")
	c.PreRun = func(cmd *cobra.Command, args []string) {
		if cfg.JSON || cfg.Agent {
			format = "json"
		}
	}
	return c
}

func cmdRecent() *cobra.Command {
	var limit int
	var cached bool
	c := &cobra.Command{
		Use:   "recent",
		Short: "Refresh and list recent Voice Memos",
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit < 0 {
				return errors.New("--limit must be non-negative")
			}
			var refreshResult *syncResult
			if !cached {
				result, err := runSync(cmd.Context())
				if err != nil {
					return fmt.Errorf("refresh Voice Memos store: %w (pass --cached to use local data)", err)
				}
				refreshResult = &result
				if result.Warning != "" && !cfg.JSON {
					cmd.PrintErrf("sync: %s\n", result.Warning)
				}
			}
			memos, err := queryMemos(limit, 0, "", "", "")
			if err != nil {
				return err
			}
			if cfg.JSON {
				mode := "fresh"
				if cached {
					mode = "cached"
				}
				printJSON(newMemoListOutput(memos, mode, refreshResult))
			} else {
				printTable(memos)
			}
			return nil
		},
	}
	c.Flags().IntVar(&limit, "limit", 10, "maximum rows")
	c.Flags().BoolVar(&cached, "cached", false, "skip iCloud refresh and use the local store immediately")
	return c
}

func requireMemoLocal(m memo) error {
	if !m.Exists || m.Path == "" {
		return fmt.Errorf("recording %d is not downloaded locally; run sync to refresh iCloud", m.ID)
	}
	return nil
}

func cmdTranscript() *cobra.Command {
	var raw bool
	c := &cobra.Command{
		Use:   "transcript <id|uuid|filename>",
		Short: "Extract Apple's embedded transcript from a memo .m4a tsrp atom",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := findMemo(args[0])
			if err != nil {
				return err
			}
			if err := requireMemoLocal(m); err != nil {
				return err
			}
			text, segments, err := extractTranscript(m.Path)
			if err != nil {
				return err
			}
			if cfg.JSON {
				printJSON(map[string]any{"memo": m, "transcript": text, "segments": segments})
				return nil
			}
			if raw {
				fmt.Print(text)
				if !strings.HasSuffix(text, "\n") {
					fmt.Println()
				}
				return nil
			}
			fmt.Printf("# %s\n\nDate: %s\nDuration: %s\nSource: %s\n\n%s\n", m.Title, m.Date.Format(time.RFC3339), m.Duration, m.Filename, text)
			return nil
		},
	}
	c.Flags().BoolVar(&raw, "raw", false, "print transcript only")
	return c
}

func cmdExport() *cobra.Command {
	var outDir string
	var overwrite bool
	c := &cobra.Command{
		Use:   "export <id|uuid|filename>",
		Short: "Copy a memo audio file to a directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := findMemo(args[0])
			if err != nil {
				return err
			}
			if err := requireMemoLocal(m); err != nil {
				return err
			}
			if outDir == "" {
				outDir = "."
			}
			if err := os.MkdirAll(outDir, 0755); err != nil {
				return err
			}
			name := safeFilename(strings.TrimSpace(m.Title))
			if name == "" {
				name = strings.TrimSuffix(m.Filename, filepath.Ext(m.Filename))
			}
			dest := filepath.Join(outDir, fmt.Sprintf("%s - %s%s", m.Date.Format("2006-01-02 150405"), name, filepath.Ext(m.Filename)))
			if !overwrite {
				if _, err := os.Stat(dest); err == nil {
					return fmt.Errorf("destination exists: %s (pass --overwrite)", dest)
				}
			}
			if err := copyFile(m.Path, dest); err != nil {
				return err
			}
			if cfg.JSON {
				printJSON(map[string]any{"source": m.Path, "destination": dest})
			} else {
				fmt.Println(dest)
			}
			return nil
		},
	}
	c.Flags().StringVar(&outDir, "out", ".", "output directory")
	c.Flags().BoolVar(&overwrite, "overwrite", false, "overwrite existing destination")
	return c
}

func cmdAgentContext() *cobra.Command {
	var pretty bool
	c := &cobra.Command{Use: "agent-context", Short: "Describe CLI capabilities for agents", RunE: func(cmd *cobra.Command, args []string) error {
		ctx := map[string]any{
			"cli": cliName, "version": version, "side_effects": "reads local metadata; refresh may trigger voicememod and launch then clean up a hidden Voice Memos instance; export copies selected audio files", "auth": "none", "network": false,
			"commands":   []string{"doctor", "sync", "list", "recent", "transcript", "export", "which", "agent-context"},
			"default_db": defaultDBPath(), "default_recordings_dir": defaultRecordingsDir(),
		}
		b, _ := json.Marshal(ctx)
		if pretty {
			b, _ = json.MarshalIndent(ctx, "", "  ")
		}
		fmt.Println(string(b))
		return nil
	}}
	c.Flags().BoolVar(&pretty, "pretty", false, "pretty-print JSON")
	return c
}

func cmdWhich() *cobra.Command {
	return &cobra.Command{Use: "which <capability>", Short: "Map a task to the right command", Args: cobra.ExactArgs(1), Run: func(cmd *cobra.Command, args []string) {
		q := strings.ToLower(args[0])
		var ans string
		switch {
		case strings.Contains(q, "transcript") || strings.Contains(q, "text") || strings.Contains(q, "summar"):
			ans = "transcript <id|uuid|filename>"
		case strings.Contains(q, "export") || strings.Contains(q, "copy") || strings.Contains(q, "audio"):
			ans = "export <id|uuid|filename> --out <dir>"
		case strings.Contains(q, "recent"):
			ans = "recent --limit 10"
		default:
			ans = "list --search <term> --json"
		}
		if cfg.JSON {
			printJSON(map[string]string{"command": ans})
		} else {
			fmt.Println(ans)
		}
	}}
}

func defaultRecordingsDir() string {
	return filepath.Join(os.Getenv("HOME"), "Library/Group Containers/group.com.apple.VoiceMemos.shared/Recordings")
}
func defaultDBPath() string { return filepath.Join(defaultRecordingsDir(), "CloudRecordings.db") }
func resolvePaths() (string, string) {
	rec := cfg.RecordingsDir
	if rec == "" {
		rec = defaultRecordingsDir()
	}
	db := cfg.DBPath
	if db == "" {
		db = filepath.Join(rec, "CloudRecordings.db")
	}
	return db, rec
}
func openDB(path string) (*sql.DB, error) {
	u := &url.URL{Scheme: "file", Path: filepath.Clean(path)}
	q := u.Query()
	q.Set("mode", "ro")
	q.Add("_pragma", "query_only(1)")
	dsn := "file:" + u.EscapedPath() + "?" + q.Encode()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func resolveRecordingPath(base, name string) (string, error) {
	if name == "" || filepath.IsAbs(name) {
		return "", errors.New("recording path must be relative")
	}
	base = filepath.Clean(base)
	joined := filepath.Join(base, filepath.Clean(name))
	rel, err := filepath.Rel(base, joined)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("recording path escapes recordings directory")
	}
	if resolved, err := filepath.EvalSymlinks(joined); err == nil {
		resolvedBase, baseErr := filepath.EvalSymlinks(base)
		if baseErr == nil {
			rel, err = filepath.Rel(resolvedBase, resolved)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return "", errors.New("recording symlink escapes recordings directory")
			}
			joined = resolved
		}
	}
	return joined, nil
}

func validateVoiceMemosSchema(db *sql.DB) error {
	rows, err := db.Query("PRAGMA table_info(ZCLOUDRECORDING)")
	if err != nil {
		return fmt.Errorf("inspect Voice Memos schema: %w", err)
	}
	defer func() { _ = rows.Close() }()
	columns := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return fmt.Errorf("inspect Voice Memos schema: %w", err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("inspect Voice Memos schema: %w", err)
	}
	required := []string{"Z_PK", "ZDATE", "ZPATH", "ZDURATION", "ZLOCALDURATION", "ZUNIQUEID", "ZENCRYPTEDTITLE", "ZCUSTOMLABEL"}
	var missing []string
	for _, name := range required {
		if !columns[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("unsupported Voice Memos schema: missing columns %s", strings.Join(missing, ", "))
	}
	return nil
}

func queryMemos(limit, offset int, search, after, before string) ([]memo, error) {
	return queryMemosWithTranscriptProbe(limit, offset, search, after, before, true)
}

func queryMemosWithTranscriptProbe(limit, offset int, search, after, before string, probeTranscripts bool) ([]memo, error) {
	dbPath, recDir := resolvePaths()
	db, err := openDB(dbPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	if err := validateVoiceMemosSchema(db); err != nil {
		return nil, err
	}
	clauses := []string{}
	args := []any{}
	if search != "" {
		clauses = append(clauses, "(COALESCE(ZENCRYPTEDTITLE, ZCUSTOMLABEL, ZPATH) LIKE ? COLLATE NOCASE)")
		args = append(args, "%"+search+"%")
	}
	if after != "" {
		ts, err := dateToCoreData(after, false)
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, "ZDATE >= ?")
		args = append(args, ts)
	}
	if before != "" {
		ts, err := dateToCoreData(before, true)
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, "ZDATE <= ?")
		args = append(args, ts)
	}
	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}
	q := "SELECT Z_PK, COALESCE(ZUNIQUEID,''), COALESCE(ZENCRYPTEDTITLE, ZCUSTOMLABEL, ZPATH, ''), ZDATE, COALESCE(ZDURATION, ZLOCALDURATION, 0), COALESCE(ZPATH,'') FROM ZCLOUDRECORDING" + where + " ORDER BY ZDATE DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []memo
	for rows.Next() {
		var id int64
		var uuid, title string
		var zdate, dur float64
		var filename string
		if err := rows.Scan(&id, &uuid, &title, &zdate, &dur, &filename); err != nil {
			return nil, err
		}
		if math.IsNaN(zdate) || math.IsInf(zdate, 0) || math.IsNaN(dur) || math.IsInf(dur, 0) {
			return nil, fmt.Errorf("recording %d contains non-finite date or duration", id)
		}
		path := ""
		exists := false
		if filename != "" {
			var pathErr error
			path, pathErr = resolveRecordingPath(recDir, filename)
			if pathErr != nil {
				return nil, fmt.Errorf("recording %d has unsafe path %q: %w", id, filename, pathErr)
			}
			exists = fileExists(path)
		}
		out = append(out, memo{ID: id, UUID: uuid, Title: title, Date: coreDataToTime(zdate), DurationSec: dur, Duration: formatDuration(dur), Filename: filename, Path: path, HasTranscript: probeTranscripts && exists && transcriptProbe(path), Exists: exists})
	}
	return out, rows.Err()
}

func probeSelectedMemo(m memo) memo {
	m.HasTranscript = m.Exists && transcriptProbe(m.Path)
	return m
}

func findMemo(key string) (memo, error) {
	memos, err := queryMemosWithTranscriptProbe(10000, 0, "", "", "", false)
	if err != nil {
		return memo{}, err
	}
	keyLower := strings.ToLower(key)
	if id, err := strconv.ParseInt(key, 10, 64); err == nil {
		for _, m := range memos {
			if m.ID == id {
				return probeSelectedMemo(m), nil
			}
		}
	}
	for _, m := range memos {
		if strings.EqualFold(m.UUID, key) || strings.EqualFold(m.Filename, key) {
			return probeSelectedMemo(m), nil
		}
	}
	var matches []memo
	for _, m := range memos {
		if strings.Contains(strings.ToLower(m.Title), keyLower) {
			matches = append(matches, m)
		}
	}
	if len(matches) == 1 {
		return probeSelectedMemo(matches[0]), nil
	}
	if len(matches) > 1 {
		return memo{}, fmt.Errorf("ambiguous memo %q: %d title matches; use id or filename", key, len(matches))
	}
	return memo{}, fmt.Errorf("memo not found: %s", key)
}

func coreDataToTime(seconds float64) time.Time {
	return time.Unix(coreDataEpochUnix+int64(seconds), int64((seconds-math.Floor(seconds))*1e9)).In(time.Local)
}
func dateToCoreData(s string, endOfDay bool) (float64, error) {
	t, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil {
		return 0, fmt.Errorf("expected YYYY-MM-DD: %w", err)
	}
	if endOfDay {
		t = t.Add(24*time.Hour - time.Second)
	}
	return float64(t.Unix() - coreDataEpochUnix), nil
}
func formatDuration(sec float64) string {
	total := int(sec + 0.5)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func readAtomHeader(r io.ReadSeeker) (typ string, size int64, header int64, err error) {
	buf := make([]byte, 8)
	if _, err = io.ReadFull(r, buf); err != nil {
		return "", 0, 0, err
	}
	size = int64(uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3]))
	typ = string(buf[4:8])
	header = 8
	if size == 1 {
		ext := make([]byte, 8)
		if _, err = io.ReadFull(r, ext); err != nil {
			return "", 0, 0, err
		}
		var extended uint64
		for _, b := range ext {
			extended = (extended << 8) | uint64(b)
		}
		if extended > math.MaxInt64 {
			return "", 0, 0, errors.New("atom size exceeds supported range")
		}
		size = int64(extended)
		header = 16
	}
	return
}
func findAtom(r io.ReadSeeker, end int64, target string) (atomEnd int64, dataStart int64, err error) {
	for {
		pos, _ := r.Seek(0, io.SeekCurrent)
		if pos >= end {
			return 0, 0, os.ErrNotExist
		}
		typ, size, header, err := readAtomHeader(r)
		if err != nil {
			return 0, 0, err
		}
		if size == 0 {
			size = end - pos
		}
		atomEnd = pos + size
		dataStart = pos + header
		if size < header || atomEnd < dataStart || atomEnd > end {
			return 0, 0, fmt.Errorf("invalid %s atom bounds", typ)
		}
		if typ == target {
			return atomEnd, dataStart, nil
		}
		if _, err := r.Seek(atomEnd, io.SeekStart); err != nil {
			return 0, 0, err
		}
	}
}
func findTranscriptAtom(r io.ReadSeeker, moovEnd int64) (int64, int64, error) {
	for {
		trakEnd, _, err := findAtom(r, moovEnd, "trak")
		if err != nil {
			return 0, 0, errors.New("tsrp transcript atom not found")
		}
		udtaEnd, _, udtaErr := findAtom(r, trakEnd, "udta")
		if udtaErr == nil {
			if tsrpEnd, dataStart, tsrpErr := findAtom(r, udtaEnd, "tsrp"); tsrpErr == nil {
				return tsrpEnd, dataStart, nil
			}
		}
		if _, err := r.Seek(trakEnd, io.SeekStart); err != nil {
			return 0, 0, err
		}
	}
}

func hasTranscript(path string) bool { _, _, err := readTranscriptJSON(path); return err == nil }

var transcriptProbe = hasTranscript

func readTranscriptJSON(path string) (map[string]any, []byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = f.Close() }()
	end, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, nil, err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, nil, err
	}
	moovEnd, _, err := findAtom(f, end, "moov")
	if err != nil {
		return nil, nil, errors.New("moov atom not found")
	}
	tsrpEnd, dataStart, err := findTranscriptAtom(f, moovEnd)
	if err != nil {
		return nil, nil, err
	}
	dataSize := tsrpEnd - dataStart
	if dataSize <= 0 {
		return nil, nil, errors.New("empty tsrp transcript atom")
	}
	if dataSize > maxTranscriptAtomBytes {
		return nil, nil, fmt.Errorf("tsrp transcript atom too large: %d bytes", dataSize)
	}
	if _, err := f.Seek(dataStart, io.SeekStart); err != nil {
		return nil, nil, err
	}
	data := make([]byte, dataSize)
	if _, err := io.ReadFull(f, data); err != nil {
		return nil, nil, err
	}
	data = bytes.Trim(data, "\x00\r\n\t ")
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, data, fmt.Errorf("parse tsrp JSON: %w", err)
	}
	return obj, data, nil
}

type segment struct {
	Time float64 `json:"time"`
	Text string  `json:"text"`
}

type freshnessOutput struct {
	Mode   string      `json:"mode"`
	Result *syncResult `json:"result,omitempty"`
}

type memoListOutput struct {
	SchemaVersion int             `json:"schema_version"`
	Freshness     freshnessOutput `json:"freshness"`
	Memos         []memo          `json:"memos"`
}

func newMemoListOutput(memos []memo, mode string, result *syncResult) memoListOutput {
	return memoListOutput{
		SchemaVersion: 1,
		Freshness:     freshnessOutput{Mode: mode, Result: result},
		Memos:         memos,
	}
}

func extractTranscript(path string) (string, []segment, error) {
	obj, _, err := readTranscriptJSON(path)
	if err != nil {
		return "", nil, err
	}
	segments := extractSegments(obj["attributedString"])
	if len(segments) == 0 {
		return "", nil, errors.New("no transcript text segments found")
	}
	lines := groupSegments(segments)
	return strings.Join(lines, "\n"), segments, nil
}
func extractSegments(v any) []segment {
	var segs []segment
	switch x := v.(type) {
	case []any:
		cur := 0.0
		for i, item := range x {
			if s, ok := item.(string); ok {
				if i+1 < len(x) {
					if m, ok := x[i+1].(map[string]any); ok {
						if tr, ok := m["timeRange"].([]any); ok && len(tr) > 0 {
							cur = asFloat(tr[0])
						}
					}
				}
				if strings.TrimSpace(s) != "" {
					segs = append(segs, segment{cur, s})
				}
			}
		}
	case map[string]any:
		runs, _ := x["runs"].([]any)
		attrs, _ := x["attributeTable"].([]any)
		cur := 0.0
		for i := 0; i < len(runs); i += 2 {
			s, _ := runs[i].(string)
			if i+1 < len(runs) {
				idx := int(asFloat(runs[i+1]))
				if idx >= 0 && idx < len(attrs) {
					if m, ok := attrs[idx].(map[string]any); ok {
						if tr, ok := m["timeRange"].([]any); ok && len(tr) > 0 {
							cur = asFloat(tr[0])
						}
					}
				}
			}
			if strings.TrimSpace(s) != "" {
				segs = append(segs, segment{cur, s})
			}
		}
	}
	sort.SliceStable(segs, func(i, j int) bool { return segs[i].Time < segs[j].Time })
	return segs
}
func asFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	default:
		return 0
	}
}
func groupSegments(segs []segment) []string {
	var lines []string
	var parts []string
	lineTime := segs[0].Time
	prev := segs[0].Time
	flush := func() {
		txt := cleanText(strings.Join(parts, " "))
		if txt != "" {
			lines = append(lines, fmt.Sprintf("%s %s", fmtTS(lineTime), txt))
		}
		parts = nil
	}
	for _, s := range segs {
		txt := strings.TrimSpace(s.Text)
		if txt == "" {
			continue
		}
		gap := s.Time - prev
		words := len(strings.Fields(strings.Join(parts, " ")))
		joined := strings.Join(parts, " ")
		if len(parts) > 0 && shouldBreak(joined, gap, words) {
			flush()
			lineTime = s.Time
		}
		parts = append(parts, txt)
		prev = s.Time
	}
	if len(parts) > 0 {
		flush()
	}
	return lines
}
func shouldBreak(text string, gap float64, words int) bool {
	t := strings.TrimSpace(text)
	if words >= 25 {
		return true
	}
	if words >= 4 && (strings.HasSuffix(t, ".") || strings.HasSuffix(t, "!") || strings.HasSuffix(t, "?")) {
		return true
	}
	if gap >= 4 && words >= 6 {
		return true
	}
	if gap >= 2 && words >= 12 {
		return true
	}
	return false
}
func fmtTS(sec float64) string {
	total := int(sec)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("[%d:%02d:%02d]", h, m, s)
	}
	return fmt.Sprintf("[%d:%02d]", m, s)
}

var multiSpace = regexp.MustCompile(`\s+`)
var filler = regexp.MustCompile(`(?i)\b(uh|um)\b\.?\s*`)

func cleanText(s string) string {
	s = strings.ReplaceAll(s, "...", " ")
	s = filler.ReplaceAllString(s, "")
	s = multiSpace.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, " ,", ",")
	s = strings.ReplaceAll(s, " .", ".")
	return strings.TrimSpace(s)
}

func printJSON(v any) { b, _ := json.MarshalIndent(v, "", "  "); fmt.Println(string(b)) }
func printTable(memos []memo) {
	fmt.Printf("%-5s  %-20s  %-8s  %-3s  %s\n", "ID", "DATE", "DURATION", "TXT", "TITLE")
	for _, m := range memos {
		txt := "no"
		if m.HasTranscript {
			txt = "yes"
		}
		fmt.Printf("%-5d  %-20s  %-8s  %-3s  %s\n", m.ID, m.Date.Format("2006-01-02 15:04"), m.Duration, txt, m.Title)
	}
}
func printCSV(memos []memo) error {
	w := csv.NewWriter(os.Stdout)
	if err := w.Write([]string{"id", "uuid", "title", "date", "duration", "filename", "path", "has_transcript", "exists"}); err != nil {
		return err
	}
	for _, m := range memos {
		if err := w.Write([]string{strconv.FormatInt(m.ID, 10), m.UUID, m.Title, m.Date.Format(time.RFC3339), m.Duration, m.Filename, m.Path, strconv.FormatBool(m.HasTranscript), strconv.FormatBool(m.Exists)}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	if info, err := os.Lstat(dst); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing symlink destination: %s", dst)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	fd, err := unix.Open(dst, unix.O_WRONLY|unix.O_CREAT|unix.O_TRUNC|unix.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	out := os.NewFile(uintptr(fd), dst)
	if out == nil {
		_ = unix.Close(fd)
		return errors.New("create destination file")
	}
	if err := out.Chmod(0600); err != nil {
		_ = out.Close()
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

var unsafeFile = regexp.MustCompile(`[^a-zA-Z0-9._ -]+`)

func safeFilename(s string) string {
	s = unsafeFile.ReplaceAllString(s, "_")
	s = strings.Trim(s, " .")
	if len(s) > 120 {
		s = s[:120]
	}
	return s
}
