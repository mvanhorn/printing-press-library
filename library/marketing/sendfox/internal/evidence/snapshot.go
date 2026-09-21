// Package evidence evaluates explicitly supplied snapshots; it never makes network calls.
package evidence

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"net/mail"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Row = map[string]any
type Resource struct {
	Items          []Row  `json:"items" yaml:"items"`
	Source         string `json:"source" yaml:"source"`
	ObservedAt     string `json:"observed_at" yaml:"observed_at"`
	Complete       *bool  `json:"complete" yaml:"complete"`
	Capped         bool   `json:"capped" yaml:"capped"`
	Total          *int   `json:"total,omitempty" yaml:"total"`
	Pages          int    `json:"pages" yaml:"pages"`
	ScopesComplete bool   `json:"scopes_complete" yaml:"scopes_complete"`
}
type Mapping struct {
	Kind     string `json:"kind" yaml:"kind"`
	Source   string `json:"source" yaml:"source"`
	Target   string `json:"target" yaml:"target"`
	Approved bool   `json:"approved" yaml:"approved"`
}
type Snapshot struct {
	SchemaVersion int                 `json:"schema_version" yaml:"schema_version"`
	Resources     map[string]Resource `json:"resources" yaml:"resources"`
	Campaign      Row                 `json:"campaign,omitempty" yaml:"campaign"`
	Automation    Row                 `json:"automation,omitempty" yaml:"automation"`
	Kit           *Snapshot           `json:"kit,omitempty" yaml:"kit"`
	Previous      *Snapshot           `json:"previous,omitempty" yaml:"previous"`
	Mappings      []Mapping           `json:"mappings,omitempty" yaml:"mappings"`
	Requirements  []string            `json:"requirements,omitempty" yaml:"requirements"`
}
type Report struct {
	SchemaVersion   int                 `json:"schema_version"`
	Workflow        string              `json:"workflow"`
	Ready           bool                `json:"ready"`
	Findings        []Row               `json:"findings"`
	Blockers        []string            `json:"blockers"`
	Unknowns        []string            `json:"unknowns"`
	SourceEvidence  map[string]Resource `json:"source_evidence"`
	ProposedActions []Row               `json:"proposed_actions"`
	Data            Row                 `json:"data"`
}

type SnapshotArtifact struct {
	Path       string `json:"path"`
	SHA256     string `json:"sha256"`
	Bytes      int    `json:"bytes"`
	ObservedAt string `json:"observed_at"`
}

func NewReport(name string, s Snapshot) Report {
	meta := map[string]Resource{}
	for k, r := range s.Resources {
		r.Items = nil
		meta[k] = r
	}
	return Report{SchemaVersion: 1, Workflow: name, Findings: []Row{}, Blockers: []string{}, Unknowns: []string{}, SourceEvidence: meta, ProposedActions: []Row{}, Data: Row{}}
}
func (r *Report) Finish() {
	sort.Strings(r.Blockers)
	sort.Strings(r.Unknowns)
	r.Ready = len(r.Blockers) == 0 && len(r.Unknowns) == 0
}
func (r *Report) Require(s Snapshot, now time.Time, maxAge time.Duration, names ...string) {
	for _, name := range names {
		v, ok := s.Resources[name]
		if !ok {
			r.Unknowns = append(r.Unknowns, name+": absent evidence")
			continue
		}
		if v.Complete == nil || !*v.Complete || v.Capped || v.Total != nil && *v.Total != len(v.Items) {
			r.Unknowns = append(r.Unknowns, name+": incomplete or capped evidence")
		}
		when, err := time.Parse(time.RFC3339, v.ObservedAt)
		if err != nil || v.Source == "" {
			r.Unknowns = append(r.Unknowns, name+": source or observation time missing")
		} else if when.After(now.Add(5*time.Minute)) || maxAge > 0 && now.Sub(when) > maxAge {
			r.Unknowns = append(r.Unknowns, name+": stale or future-dated evidence")
		}
	}
}
func Load(file string) (Snapshot, error) {
	b, err := readEvidenceFile(file)
	if err != nil {
		return Snapshot{}, err
	}
	return Decode(b)
}

// WriteSnapshotHistory persists one canonical snapshot with restrictive
// permissions. The temporary write, fsync and rename keep readers from seeing
// a partial history entry.
func WriteSnapshotHistory(dir string, s Snapshot, now time.Time) (SnapshotArtifact, error) {
	if strings.TrimSpace(dir) == "" {
		return SnapshotArtifact{}, fmt.Errorf("snapshot history directory is required")
	}
	dir = filepath.Clean(dir)
	if info, err := os.Lstat(dir); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return SnapshotArtifact{}, fmt.Errorf("snapshot history directory must not be a symlink")
	} else if err != nil && !os.IsNotExist(err) {
		return SnapshotArtifact{}, fmt.Errorf("inspect snapshot history directory: %w", err)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return SnapshotArtifact{}, fmt.Errorf("create snapshot history directory: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return SnapshotArtifact{}, fmt.Errorf("restrict snapshot history directory: %w", err)
	}

	payload, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return SnapshotArtifact{}, fmt.Errorf("encode snapshot history: %w", err)
	}
	payload = append(payload, '\n')
	sum := sha256.Sum256(payload)
	checksum := hex.EncodeToString(sum[:])
	stamp := now.UTC().Format("20060102T150405.000000000Z")
	name := fmt.Sprintf("sendfox-snapshot-%s-%s.json", stamp, checksum[:12])
	finalPath := filepath.Join(dir, name)

	temp, err := os.CreateTemp(dir, ".sendfox-snapshot-*.tmp")
	if err != nil {
		return SnapshotArtifact{}, fmt.Errorf("create snapshot history temporary file: %w", err)
	}
	tempPath := temp.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tempPath)
		}
	}()
	if err := temp.Chmod(0600); err != nil {
		_ = temp.Close()
		return SnapshotArtifact{}, fmt.Errorf("restrict snapshot history temporary file: %w", err)
	}
	if _, err := temp.Write(payload); err != nil {
		_ = temp.Close()
		return SnapshotArtifact{}, fmt.Errorf("write snapshot history: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return SnapshotArtifact{}, fmt.Errorf("sync snapshot history: %w", err)
	}
	if err := temp.Close(); err != nil {
		return SnapshotArtifact{}, fmt.Errorf("close snapshot history: %w", err)
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		return SnapshotArtifact{}, fmt.Errorf("commit snapshot history: %w", err)
	}
	removeTemp = false
	if err := os.Chmod(finalPath, 0600); err != nil {
		return SnapshotArtifact{}, fmt.Errorf("restrict committed snapshot history: %w", err)
	}
	directory, err := os.Open(dir)
	if err != nil {
		return SnapshotArtifact{}, fmt.Errorf("open snapshot history directory for sync: %w", err)
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()
		return SnapshotArtifact{}, fmt.Errorf("sync snapshot history directory: %w", err)
	}
	if err := directory.Close(); err != nil {
		return SnapshotArtifact{}, fmt.Errorf("close snapshot history directory: %w", err)
	}
	absolutePath, err := filepath.Abs(finalPath)
	if err != nil {
		absolutePath = finalPath
	}
	return SnapshotArtifact{Path: absolutePath, SHA256: checksum, Bytes: len(payload), ObservedAt: now.UTC().Format(time.RFC3339Nano)}, nil
}
func readEvidenceFile(file string) ([]byte, error) {
	// #nosec G304 -- Local CLI/MCP caller explicitly selects the evidence file; arbitrary paths are part of this interface.
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(io.LimitReader(f, (64<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(b) > 64<<20 {
		return nil, fmt.Errorf("evidence exceeds 64 MiB; split or use --db")
	}
	return b, nil
}
func Decode(b []byte) (Snapshot, error) {
	if len(b) > 64<<20 {
		return Snapshot{}, fmt.Errorf("snapshot exceeds 64 MiB; split or use --db")
	}
	text := strings.TrimSpace(string(b))
	if strings.HasPrefix(text, "---\n") {
		parts := strings.SplitN(text[4:], "\n---", 2)
		text = parts[0]
	} else if strings.HasPrefix(text, "```yaml\n") {
		text = strings.TrimSuffix(strings.TrimPrefix(text, "```yaml\n"), "```")
	}
	var s Snapshot
	d := yaml.NewDecoder(strings.NewReader(text))
	d.KnownFields(true)
	if err := d.Decode(&s); err != nil {
		return s, fmt.Errorf("snapshot JSON/YAML: %w", err)
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return s, fmt.Errorf("snapshot must contain exactly one document")
	}
	if s.SchemaVersion != 1 {
		return s, fmt.Errorf("schema_version must be 1")
	}
	if s.Resources == nil {
		s.Resources = map[string]Resource{}
	}
	return s, nil
}
func Text(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
func Email(v any) string { return strings.ToLower(strings.TrimSpace(Text(v))) }
func ValidEmail(v string) bool {
	a, err := mail.ParseAddress(v)
	return err == nil && a.Address == v && strings.Contains(v, "@")
}
func IDs(v any) []string {
	out := []string{}
	switch a := v.(type) {
	case []any:
		for _, x := range a {
			out = append(out, Text(x))
		}
	case []int:
		for _, x := range a {
			out = append(out, Text(x))
		}
	}
	return out
}
func Rows(v any) []Row {
	out := []Row{}
	if a, ok := v.([]any); ok {
		for _, x := range a {
			if r, ok := x.(map[string]any); ok {
				out = append(out, r)
			}
		}
	}
	return out
}
func Index(rows []Row, key string) map[string]Row {
	out := map[string]Row{}
	for _, r := range rows {
		out[Text(r[key])] = r
	}
	return out
}
func Hash(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func Suppressed(s Snapshot) map[string]bool {
	out := map[string]bool{}
	for _, c := range s.Resources["unsubscribed"].Items {
		out[Email(c["email"])] = true
	}
	for _, c := range s.Resources["contacts"].Items {
		if Text(c["unsubscribed_at"]) != "" || c["status"] == "unsubscribed" || c["status"] == "bounced" || c["status"] == "invalid" {
			out[Email(c["email"])] = true
		}
	}
	return out
}
func ReadCSV(file string) ([]Row, error) {
	b, err := readEvidenceFile(file)
	if err != nil {
		return nil, err
	}
	r := csv.NewReader(bytes.NewReader(b))
	r.TrimLeadingSpace = true
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("CSV needs email header")
	}
	head := map[string]int{}
	for i, k := range rows[0] {
		k = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(k)), "\ufeff")
		if _, ok := head[k]; ok {
			return nil, fmt.Errorf("duplicate CSV header %s", k)
		}
		head[k] = i
	}
	if _, ok := head["email"]; !ok {
		return nil, fmt.Errorf("CSV needs email header")
	}
	out := []Row{}
	for i, row := range rows[1:] {
		v := Row{"row": i + 2}
		for _, key := range []string{"email", "first_name", "last_name", "status", "unsubscribed_at"} {
			if idx, ok := head[key]; ok {
				v[key] = strings.TrimSpace(row[idx])
			}
		}
		for key, alias := range map[string]string{"first_name": "first", "last_name": "last"} {
			if _, ok := v[key]; !ok {
				if idx, ok := head[alias]; ok {
					v[key] = strings.TrimSpace(row[idx])
				}
			}
		}
		out = append(out, v)
	}
	return out, nil
}
func ReconcileCSV(rows []Row, s Snapshot) Report {
	r := NewReport("contacts-reconcile-csv", s)
	seen := map[string]bool{}
	existing := map[string]int{}
	for _, c := range s.Resources["contacts"].Items {
		existing[Email(c["email"])]++
	}
	suppressed := Suppressed(s)
	create := []Row{}
	skip := []Row{}
	for _, c := range rows {
		email := Email(c["email"])
		reason := ""
		switch {
		case !ValidEmail(email):
			reason = "invalid_email"
		case seen[email]:
			reason = "duplicate_input"
		case existing[email] > 1:
			reason = "ambiguous_existing_identity"
		case suppressed[email] || Text(c["unsubscribed_at"]) != "" || c["status"] == "unsubscribed" || c["status"] == "bounced" || c["status"] == "invalid" || c["status"] == "cancelled":
			reason = "suppressed"
		case existing[email] == 1:
			reason = "already_exists"
		}
		seen[email] = true
		if reason != "" {
			skip = append(skip, Row{"email": email, "row": c["row"], "reason": reason})
			continue
		}
		body := Row{"email": email}
		for _, k := range []string{"first_name", "last_name"} {
			if v, ok := c[k]; ok {
				body[k] = v
			}
		}
		create = append(create, body)
	}
	r.Data = Row{"input_count": len(rows), "to_create": create, "to_create_count": len(create), "skips": skip, "skip_count": len(skip)}
	return r
}
