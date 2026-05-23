// Package cmuxclient is a hidden subprocess client that calls the local cmux
// binary over its Unix socket. It exists so the generated HTTP client can be
// transparently redirected to cmux RPC calls without rewriting every promoted
// command.
//
// pp:client-call: every exported function here performs a real external IPC
// call to the cmux app's Unix socket via the cmux CLI binary.
package cmuxclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// DefaultBinary is the cmux binary path used when CMUX_BIN is unset.
const DefaultBinary = "/Applications/cmux.app/Contents/Resources/bin/cmux"

// Binary returns the cmux binary to use (CMUX_BIN env or the default).
func Binary() string {
	if b := os.Getenv("CMUX_BIN"); b != "" {
		return b
	}
	if _, err := exec.LookPath("cmux"); err == nil {
		return "cmux"
	}
	return DefaultBinary
}

// SessionJSONPath returns the path to cmux's session JSON file.
func SessionJSONPath() string {
	if p := os.Getenv("CMUX_SESSION_JSON"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "cmux", "session-com.cmuxterm.app.json")
}

// Run invokes the cmux binary with the given args and returns stdout.
// Used by the hidden cmuxDispatch shim, not directly by command code.
//
// pp:client-call
func Run(ctx context.Context, args ...string) ([]byte, error) {
	bin := Binary()
	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("cmux %s exited %d: %s", strings.Join(args, " "), exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("cmux %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

// RunJSON invokes cmux with --json and the given args.
//
// pp:client-call
func RunJSON(ctx context.Context, args ...string) ([]byte, error) {
	all := append([]string{"--json"}, args...)
	return Run(ctx, all...)
}

// Ping returns nil when the socket answers PONG.
//
// pp:client-call
func Ping(ctx context.Context) error {
	out, err := Run(ctx, "ping")
	if err != nil {
		return err
	}
	if !strings.Contains(string(out), "PONG") {
		return fmt.Errorf("cmux ping returned unexpected: %q", strings.TrimSpace(string(out)))
	}
	return nil
}

// Version returns the cmux version string.
//
// pp:client-call
func Version(ctx context.Context) (string, error) {
	out, err := Run(ctx, "version")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// SessionJSON reads and decodes the session JSON file.
type SessionJSON struct {
	CreatedAt json.RawMessage `json:"createdAt"`
	Version   json.RawMessage `json:"version"`
	Windows   []SessionWindow `json:"windows"`
}

type SessionWindow struct {
	TabManager SessionTabManager `json:"tabManager"`
}

type SessionTabManager struct {
	SelectedWorkspaceIndex int                `json:"selectedWorkspaceIndex"`
	Workspaces             []SessionWorkspace `json:"workspaces"`
}

type SessionWorkspace struct {
	CurrentDirectory string               `json:"currentDirectory"`
	CustomTitle      string               `json:"customTitle"`
	GitBranch        json.RawMessage      `json:"gitBranch"`
	IsPinned         bool                 `json:"isPinned"`
	ProcessTitle     string               `json:"processTitle"`
	StatusEntries    []SessionStatusEntry `json:"statusEntries"`
	LogEntries       []SessionLogEntry    `json:"logEntries"`
	Panels           []json.RawMessage    `json:"panels"`
	FocusedPanelID   string               `json:"focusedPanelId"`
}

// GitBranchString flattens GitBranch which may be a string or an object.
func (s *SessionWorkspace) GitBranchString() string {
	if len(s.GitBranch) == 0 {
		return ""
	}
	var str string
	if err := json.Unmarshal(s.GitBranch, &str); err == nil {
		return strings.TrimSpace(str)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(s.GitBranch, &obj); err == nil {
		// Try common nested shapes
		for _, k := range []string{"branch", "name", "value"} {
			if v, ok := obj[k]; ok {
				var s2 string
				if json.Unmarshal(v, &s2) == nil {
					return strings.TrimSpace(s2)
				}
			}
		}
	}
	return ""
}

type SessionStatusEntry struct {
	Color     string  `json:"color"`
	Icon      string  `json:"icon"`
	Key       string  `json:"key"`
	Timestamp float64 `json:"timestamp"`
	Value     string  `json:"value"`
}

type SessionLogEntry struct {
	Level     string  `json:"level"`
	Message   string  `json:"message"`
	Source    string  `json:"source"`
	Timestamp float64 `json:"timestamp"`
}

// ReadSession reads the on-disk session JSON.
//
// pp:client-call
func ReadSession() (*SessionJSON, error) {
	path := SessionJSONPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading cmux session JSON at %s: %w", path, err)
	}
	var s SessionJSON
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("parsing cmux session JSON: %w", err)
	}
	return &s, nil
}

// Workspace is the canonical workspace shape we return from Dispatch.
type Workspace struct {
	ID        string `json:"id,omitempty"`
	Ref       string `json:"ref"`
	Title     string `json:"title"`
	Index     int    `json:"index"`
	Selected  bool   `json:"selected"`
	Pinned    bool   `json:"pinned"`
	CWD       string `json:"cwd,omitempty"`
	GitBranch string `json:"git_branch,omitempty"`
}

// ListWorkspaces returns the merged view of `cmux list-workspaces` (refs,
// titles, selected) plus session JSON (cwd, git_branch).
//
// pp:client-call
func ListWorkspaces(ctx context.Context) ([]Workspace, error) {
	out, err := RunJSON(ctx, "list-workspaces")
	if err != nil {
		return nil, err
	}
	var raw struct {
		Workspaces []struct {
			Ref      string `json:"ref"`
			Title    string `json:"title"`
			Index    int    `json:"index"`
			Selected bool   `json:"selected"`
			Pinned   bool   `json:"pinned"`
		} `json:"workspaces"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing list-workspaces: %w", err)
	}
	session, _ := ReadSession() // best-effort enrichment
	result := make([]Workspace, 0, len(raw.Workspaces))
	for _, w := range raw.Workspaces {
		ws := Workspace{
			Ref:      w.Ref,
			Title:    w.Title,
			Index:    w.Index,
			Selected: w.Selected,
			Pinned:   w.Pinned,
		}
		if session != nil && len(session.Windows) > 0 && w.Index < len(session.Windows[0].TabManager.Workspaces) {
			s := session.Windows[0].TabManager.Workspaces[w.Index]
			ws.CWD = s.CurrentDirectory
			ws.GitBranch = s.GitBranchString()
		}
		result = append(result, ws)
	}
	return result, nil
}

// ResolveWorkspaceRef accepts a ref ("workspace:6"), an index ("0"), or a
// title and returns the canonical ref. Returns the input unchanged when the
// input already looks like a ref.
//
// pp:client-call
func ResolveWorkspaceRef(ctx context.Context, input string) (string, error) {
	if input == "" {
		return "", nil
	}
	if strings.HasPrefix(input, "workspace:") {
		return input, nil
	}
	wss, err := ListWorkspaces(ctx)
	if err != nil {
		return "", err
	}
	// numeric index
	if n, err := strconv.Atoi(input); err == nil {
		for _, w := range wss {
			if w.Index == n {
				return w.Ref, nil
			}
		}
	}
	// title (case-insensitive substring)
	lo := strings.ToLower(input)
	for _, w := range wss {
		if strings.EqualFold(w.Title, input) {
			return w.Ref, nil
		}
	}
	for _, w := range wss {
		if strings.Contains(strings.ToLower(w.Title), lo) {
			return w.Ref, nil
		}
	}
	return "", fmt.Errorf("no workspace matches %q", input)
}

// Notification mirrors the cmux notifications array shape.
type Notification struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	SurfaceID   string `json:"surface_id"`
	Title       string `json:"title"`
	Subtitle    string `json:"subtitle"`
	Body        string `json:"body"`
	IsRead      bool   `json:"is_read"`
}

// ListNotifications returns all current cmux notifications.
//
// pp:client-call
func ListNotifications(ctx context.Context) ([]Notification, error) {
	out, err := RunJSON(ctx, "list-notifications")
	if err != nil {
		return nil, err
	}
	var raw []Notification
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing list-notifications: %w", err)
	}
	return raw, nil
}

// Surface mirrors the cmux list-pane-surfaces.surfaces[] shape.
type Surface struct {
	Ref          string `json:"ref"`
	Title        string `json:"title"`
	Type         string `json:"type"`
	Index        int    `json:"index"`
	Selected     bool   `json:"selected"`
	WorkspaceRef string `json:"workspace_ref,omitempty"`
}

// ListSurfaces returns surfaces in a workspace.
//
// pp:client-call
func ListSurfaces(ctx context.Context, workspaceRef string) ([]Surface, error) {
	args := []string{"list-pane-surfaces"}
	if workspaceRef != "" {
		args = append(args, "--workspace", workspaceRef)
	}
	out, err := RunJSON(ctx, args...)
	if err != nil {
		return nil, err
	}
	var raw struct {
		WorkspaceRef string    `json:"workspace_ref"`
		Surfaces     []Surface `json:"surfaces"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing list-pane-surfaces: %w", err)
	}
	for i := range raw.Surfaces {
		raw.Surfaces[i].WorkspaceRef = raw.WorkspaceRef
	}
	return raw.Surfaces, nil
}

// SurfaceHealthEntry is the cmux surface-health response item.
type SurfaceHealthEntry struct {
	Ref          string `json:"ref"`
	Type         string `json:"type"`
	Index        int    `json:"index"`
	InWindow     bool   `json:"in_window"`
	WorkspaceRef string `json:"workspace_ref,omitempty"`
}

// SurfaceHealth returns surface-health for a workspace.
//
// pp:client-call
func SurfaceHealth(ctx context.Context, workspaceRef string) ([]SurfaceHealthEntry, error) {
	args := []string{"surface-health"}
	if workspaceRef != "" {
		args = append(args, "--workspace", workspaceRef)
	}
	out, err := RunJSON(ctx, args...)
	if err != nil {
		return nil, err
	}
	var raw struct {
		WorkspaceRef string               `json:"workspace_ref"`
		Surfaces     []SurfaceHealthEntry `json:"surfaces"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing surface-health: %w", err)
	}
	for i := range raw.Surfaces {
		raw.Surfaces[i].WorkspaceRef = raw.WorkspaceRef
	}
	return raw.Surfaces, nil
}

// StatusEntry is a per-workspace status row.
type StatusEntry struct {
	WorkspaceRef string  `json:"workspace_ref"`
	Key          string  `json:"key"`
	Value        string  `json:"value"`
	Icon         string  `json:"icon"`
	Color        string  `json:"color"`
	Timestamp    float64 `json:"timestamp"`
}

// ListStatusEntries returns status entries for all workspaces (or one if ref
// is set), pulled from the session JSON which has full timestamp + color + icon.
//
// pp:client-call
func ListStatusEntries(ctx context.Context, workspaceRef string) ([]StatusEntry, error) {
	session, err := ReadSession()
	if err != nil {
		return nil, err
	}
	wss, err := ListWorkspaces(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]StatusEntry, 0)
	for _, w := range wss {
		if workspaceRef != "" && w.Ref != workspaceRef {
			continue
		}
		if w.Index >= len(session.Windows[0].TabManager.Workspaces) {
			continue
		}
		s := session.Windows[0].TabManager.Workspaces[w.Index]
		for _, se := range s.StatusEntries {
			out = append(out, StatusEntry{
				WorkspaceRef: w.Ref,
				Key:          se.Key,
				Value:        se.Value,
				Icon:         se.Icon,
				Color:        se.Color,
				Timestamp:    se.Timestamp,
			})
		}
	}
	return out, nil
}

// LogEntry mirrors workspace log rows from the session JSON.
type LogEntry struct {
	WorkspaceRef string  `json:"workspace_ref"`
	Level        string  `json:"level"`
	Source       string  `json:"source"`
	Message      string  `json:"message"`
	Timestamp    float64 `json:"timestamp"`
}

// ListLogEntries returns log entries for a workspace (or all when ref empty).
//
// pp:client-call
func ListLogEntries(ctx context.Context, workspaceRef string, limit int) ([]LogEntry, error) {
	session, err := ReadSession()
	if err != nil {
		return nil, err
	}
	wss, err := ListWorkspaces(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]LogEntry, 0)
	for _, w := range wss {
		if workspaceRef != "" && w.Ref != workspaceRef {
			continue
		}
		if w.Index >= len(session.Windows[0].TabManager.Workspaces) {
			continue
		}
		s := session.Windows[0].TabManager.Workspaces[w.Index]
		for _, le := range s.LogEntries {
			out = append(out, LogEntry{
				WorkspaceRef: w.Ref,
				Level:        le.Level,
				Source:       le.Source,
				Message:      le.Message,
				Timestamp:    le.Timestamp,
			})
			if limit > 0 && len(out) >= limit {
				return out, nil
			}
		}
	}
	return out, nil
}

// Window is the cmux list-windows row shape.
type Window struct {
	ID                  string `json:"id"`
	Index               int    `json:"index"`
	WorkspaceCount      int    `json:"workspace_count"`
	SelectedWorkspaceID string `json:"selected_workspace_id"`
}

// ListWindows returns the cmux windows.
//
// pp:client-call
func ListWindows(ctx context.Context) ([]Window, error) {
	out, err := RunJSON(ctx, "list-windows")
	if err != nil {
		return nil, err
	}
	var raw []Window
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing list-windows: %w", err)
	}
	return raw, nil
}

// Pane is the cmux list-panes row shape.
type Pane struct {
	Ref          string   `json:"ref"`
	Index        int      `json:"index"`
	WorkspaceRef string   `json:"workspace_ref,omitempty"`
	SurfaceRefs  []string `json:"surface_refs"`
	SurfaceCount int      `json:"surface_count"`
	Focused      bool     `json:"focused"`
}

// ListPanes returns panes for a workspace.
//
// pp:client-call
func ListPanes(ctx context.Context, workspaceRef string) ([]Pane, error) {
	args := []string{"list-panes"}
	if workspaceRef != "" {
		args = append(args, "--workspace", workspaceRef)
	}
	out, err := RunJSON(ctx, args...)
	if err != nil {
		return nil, err
	}
	var raw struct {
		WorkspaceRef string `json:"workspace_ref"`
		Panes        []Pane `json:"panes"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing list-panes: %w", err)
	}
	for i := range raw.Panes {
		raw.Panes[i].WorkspaceRef = raw.WorkspaceRef
	}
	return raw.Panes, nil
}

// Capabilities returns the methods the running cmux exposes.
//
// pp:client-call
func Capabilities(ctx context.Context) ([]string, error) {
	out, err := RunJSON(ctx, "capabilities")
	if err != nil {
		return nil, err
	}
	var raw struct {
		Methods []string `json:"methods"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing capabilities: %w", err)
	}
	return raw.Methods, nil
}

// Hooks returns the configured hook map (event -> command).
//
// pp:client-call
func Hooks(ctx context.Context) (map[string]string, error) {
	out, err := RunJSON(ctx, "set-hook", "--list")
	if err != nil {
		return nil, err
	}
	var raw struct {
		Hooks map[string]string `json:"hooks"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parsing set-hook: %w", err)
	}
	return raw.Hooks, nil
}

// Buffers returns the cmux paste buffers. Text format parsing.
//
// pp:client-call
func Buffers(ctx context.Context) ([]map[string]string, error) {
	out, err := Run(ctx, "list-buffers")
	if err != nil {
		return nil, err
	}
	// text-formatted; parse line by line
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	res := make([]map[string]string, 0)
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "No buffer") {
			continue
		}
		parts := strings.SplitN(ln, ":", 2)
		name := strings.TrimSpace(parts[0])
		text := ""
		if len(parts) > 1 {
			text = strings.TrimSpace(parts[1])
		}
		res = append(res, map[string]string{"name": name, "text": text})
	}
	return res, nil
}

// ReadScreen returns the visible text of a surface.
//
// pp:client-call
func ReadScreen(ctx context.Context, workspaceRef, surfaceRef string, scrollback bool, lines int) (string, error) {
	args := []string{"read-screen"}
	if workspaceRef != "" {
		args = append(args, "--workspace", workspaceRef)
	}
	if surfaceRef != "" {
		args = append(args, "--surface", surfaceRef)
	}
	if scrollback {
		args = append(args, "--scrollback")
	}
	if lines > 0 {
		args = append(args, "--lines", strconv.Itoa(lines))
	}
	out, err := RunJSON(ctx, args...)
	if err != nil {
		return "", err
	}
	var raw struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return "", fmt.Errorf("parsing read-screen: %w", err)
	}
	return raw.Text, nil
}

// FocusSurface calls cmux to bring a surface to the foreground. Write side —
// only used when explicitly opted in via --switch.
//
// pp:client-call
func FocusSurface(ctx context.Context, workspaceRef, surfaceRef string) error {
	args := []string{"focus-pane"}
	if workspaceRef != "" {
		args = append(args, "--workspace", workspaceRef)
	}
	if surfaceRef != "" {
		// focus-pane takes a pane id; cmux's surface focus is implicit when
		// the surface ref is passed via tab-action. We invoke tab-action for
		// the surface so the selected tab/surface changes too.
		args = []string{"tab-action", "--action", "select"}
		if workspaceRef != "" {
			args = append(args, "--workspace", workspaceRef)
		}
		args = append(args, "--surface", surfaceRef)
	}
	if _, err := Run(ctx, args...); err != nil {
		return fmt.Errorf("focusing surface: %w", err)
	}
	return nil
}

// Dispatch translates an HTTP-style call from the generated client into the
// equivalent cmux invocation. Only GET is supported; the generated commands
// for cmux are all read-only. Returns JSON-encoded bytes matching the spec's
// declared response shape.
//
// pp:client-call
func Dispatch(ctx context.Context, method, path string, params map[string]string) (json.RawMessage, error) {
	if !strings.EqualFold(method, "GET") {
		return nil, fmt.Errorf("cmux dispatch: unsupported method %s", method)
	}
	wsParam := params["workspace"]
	switch {
	case path == "/":
		// Used by doctor's reachability probe — ping cmux and return version.
		if err := Ping(ctx); err != nil {
			return nil, err
		}
		ver, err := Version(ctx)
		if err != nil {
			return nil, err
		}
		return json.Marshal(map[string]string{"status": "ok", "version": ver})
	case path == "/workspaces":
		wss, err := ListWorkspaces(ctx)
		if err != nil {
			return nil, err
		}
		return json.Marshal(wss)
	case path == "/workspaces/current":
		wss, err := ListWorkspaces(ctx)
		if err != nil {
			return nil, err
		}
		for _, w := range wss {
			if w.Selected {
				return json.Marshal(w)
			}
		}
		return json.Marshal(Workspace{})
	case strings.HasPrefix(path, "/workspaces/"):
		id := strings.TrimPrefix(path, "/workspaces/")
		id, err := url.PathUnescape(id)
		if err != nil {
			return nil, fmt.Errorf("decoding workspace id: %w", err)
		}
		ref, err := ResolveWorkspaceRef(ctx, id)
		if err != nil {
			return nil, err
		}
		wss, err := ListWorkspaces(ctx)
		if err != nil {
			return nil, err
		}
		for _, w := range wss {
			if w.Ref == ref {
				return json.Marshal(w)
			}
		}
		return nil, fmt.Errorf("workspace not found: %s", id)
	case path == "/windows":
		ws, err := ListWindows(ctx)
		if err != nil {
			return nil, err
		}
		return json.Marshal(ws)
	case path == "/windows/current":
		out, err := RunJSON(ctx, "current-window")
		if err != nil {
			return nil, err
		}
		return out, nil
	case path == "/panes":
		ref, _ := ResolveWorkspaceRef(ctx, wsParam)
		panes, err := ListPanes(ctx, ref)
		if err != nil {
			return nil, err
		}
		return json.Marshal(panes)
	case path == "/surfaces":
		ref, _ := ResolveWorkspaceRef(ctx, wsParam)
		surfaces, err := ListSurfaces(ctx, ref)
		if err != nil {
			return nil, err
		}
		return json.Marshal(surfaces)
	case path == "/surfaces/health":
		ref, _ := ResolveWorkspaceRef(ctx, wsParam)
		health, err := SurfaceHealth(ctx, ref)
		if err != nil {
			return nil, err
		}
		return json.Marshal(health)
	case path == "/status":
		ref, _ := ResolveWorkspaceRef(ctx, wsParam)
		entries, err := ListStatusEntries(ctx, ref)
		if err != nil {
			return nil, err
		}
		return json.Marshal(entries)
	case path == "/logs":
		ref, _ := ResolveWorkspaceRef(ctx, wsParam)
		limit := 0
		if l, ok := params["limit"]; ok {
			limit, _ = strconv.Atoi(l)
		}
		entries, err := ListLogEntries(ctx, ref, limit)
		if err != nil {
			return nil, err
		}
		return json.Marshal(entries)
	case path == "/notifications":
		notes, err := ListNotifications(ctx)
		if err != nil {
			return nil, err
		}
		return json.Marshal(notes)
	case path == "/hooks":
		hooks, err := Hooks(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]map[string]string, 0, len(hooks))
		for k, v := range hooks {
			out = append(out, map[string]string{"event": k, "command": v})
		}
		return json.Marshal(out)
	case path == "/buffers":
		bufs, err := Buffers(ctx)
		if err != nil {
			return nil, err
		}
		return json.Marshal(bufs)
	case path == "/capabilities":
		caps, err := Capabilities(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]map[string]string, 0, len(caps))
		for _, m := range caps {
			out = append(out, map[string]string{"method": m})
		}
		return json.Marshal(out)
	}
	return nil, fmt.Errorf("cmux dispatch: unknown path %s", path)
}
