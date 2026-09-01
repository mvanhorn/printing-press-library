// Package sequence implements bounded, target-bound KVM workflows.
package sequence

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Action struct {
	Type       string  `json:"type"`
	Value      string  `json:"value,omitempty"`
	Key        string  `json:"key,omitempty"`
	DurationMS int     `json:"duration_ms,omitempty"`
	X          int     `json:"x,omitempty"`
	Y          int     `json:"y,omitempty"`
	XPct       float64 `json:"x_pct,omitempty"`
	YPct       float64 `json:"y_pct,omitempty"`
	Button     string  `json:"button,omitempty"`
	Count      int     `json:"count,omitempty"`
	DX         int     `json:"dx,omitempty"`
	DY         int     `json:"dy,omitempty"`
	Contains   string  `json:"contains,omitempty"`
}
type Plan struct {
	Target                 string        `json:"target"`
	Actions                []Action      `json:"actions"`
	MaxDuration            time.Duration `json:"max_duration_ns"`
	UnexpectedScreenPolicy string        `json:"unexpected_screen_policy,omitempty"`
}

func (p Plan) validate() error {
	if strings.TrimSpace(p.Target) == "" {
		return errors.New("target is required")
	}
	if len(p.Actions) == 0 || len(p.Actions) > 10 {
		return errors.New("actions must contain 1..10 items")
	}
	if p.MaxDuration <= 0 || p.MaxDuration > 30*time.Second {
		return errors.New("max duration must be between 1ms and 30s")
	}
	if p.UnexpectedScreenPolicy != "" && p.UnexpectedScreenPolicy != "abort" {
		return errors.New("unsupported unexpected_screen_policy")
	}
	for _, a := range p.Actions {
		switch a.Type {
		case "key", "text":
			if a.Value == "" {
				return errors.New("action value is required")
			}
		case "hold_key":
			if a.Key == "" && a.Value == "" {
				return errors.New("hold key is required")
			}
			if a.DurationMS < 1 || a.DurationMS > 5000 {
				return errors.New("hold duration out of range")
			}
		case "release_all":
		case "mouse_move":
			if a.X < -32768 || a.X > 32767 || a.Y < -32768 || a.Y > 32767 {
				return errors.New("mouse coordinates out of range")
			}
		case "mouse_move_pct":
			if a.XPct < 0 || a.XPct > 100 || a.YPct < 0 || a.YPct > 100 {
				return errors.New("mouse percentages out of range")
			}
		case "mouse_click":
			if a.Button == "" {
				a.Button = "left"
			}
			if a.Count == 0 {
				a.Count = 1
			}
			if (a.Button != "left" && a.Button != "middle" && a.Button != "right" && a.Button != "up" && a.Button != "down") || a.Count < 1 || a.Count > 5 {
				return errors.New("invalid mouse click")
			}
		case "mouse_scroll":
			if a.DX < -127 || a.DX > 127 || a.DY < -127 || a.DY > 127 {
				return errors.New("mouse scroll out of range")
			}
		case "wait":
			if a.DurationMS < 1 || a.DurationMS > 30000 {
				return errors.New("wait duration out of range")
			}
		case "assert_screen":
			if a.Contains == "" || len([]rune(a.Contains)) > 200 {
				return errors.New("screen assertion must contain 1-200 characters")
			}
		default:
			return fmt.Errorf("unsupported action %q", a.Type)
		}
	}
	return nil
}
func (p Plan) Hash() (string, error) {
	if err := p.validate(); err != nil {
		return "", err
	}
	b, _ := json.Marshal(struct {
		Target  string   `json:"target"`
		Actions []Action `json:"actions"`
		Max     int64    `json:"max_duration_ms"`
		Policy  string   `json:"unexpected_screen_policy"`
	}{strings.TrimSpace(p.Target), p.Actions, p.MaxDuration.Milliseconds(), "abort"})
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:]), nil
}

type authorization struct {
	Token     string    `json:"token"`
	Target    string    `json:"target"`
	Plan      Plan      `json:"plan"`
	Hash      string    `json:"hash"`
	ExpiresAt time.Time `json:"expires_at"`
}
type Store struct {
	path string
	mu   sync.Mutex
}

func NewStore(path string) *Store { return &Store{path: path} }
func (s *Store) read() (map[string]authorization, error) {
	if info, statErr := os.Lstat(s.path); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("unsafe authorization store")
	}
	b, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return map[string]authorization{}, nil
	}
	if err != nil {
		return nil, err
	}
	var v map[string]authorization
	if json.Unmarshal(b, &v) != nil {
		return nil, errors.New("invalid authorization store")
	}
	return v, nil
}
func (s *Store) write(v map[string]authorization) error {
	dir := filepath.Dir(s.path)
	if err := secureDir(dir); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".auth-")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	b, _ := json.Marshal(v)
	if _, err = tmp.Write(b); err == nil {
		err = tmp.Sync()
	}
	if e := tmp.Close(); err == nil {
		err = e
	}
	if err != nil {
		return err
	}
	return os.Rename(name, s.path)
}
func secureDir(dir string) error {
	info, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		if err = os.MkdirAll(dir, 0700); err != nil {
			return err
		}
		info, err = os.Lstat(dir)
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("unsafe authorization directory: %s mode=%o symlink=%v dir=%v", dir, info.Mode().Perm(), info.Mode()&os.ModeSymlink != 0, info.IsDir())
	}
	return nil
}
func (s *Store) put(a authorization) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read()
	if e != nil {
		return e
	}
	v[a.Token] = a
	return s.write(v)
}
func (s *Store) take(token string) (authorization, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read()
	if e != nil {
		return authorization{}, e
	}
	a, ok := v[token]
	if !ok {
		return authorization{}, errors.New("authorization invalid")
	}
	delete(v, token)
	if e = s.write(v); e != nil {
		return authorization{}, e
	}
	return a, nil
}
func (s *Store) peek(token string) (authorization, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read()
	if e != nil {
		return authorization{}, e
	}
	a, ok := v[token]
	if !ok {
		return authorization{}, errors.New("authorization invalid")
	}
	return a, nil
}

type Authorizer struct {
	store *Store
	now   func() time.Time
}

func NewAuthorizer(s *Store, now func() time.Time) *Authorizer {
	if now == nil {
		now = time.Now
	}
	return &Authorizer{store: s, now: now}
}
func (a *Authorizer) Authorize(p Plan, target string, approved bool, ttl time.Duration) (string, error) {
	if !approved {
		return "", errors.New("explicit approval required")
	}
	if target != p.Target {
		return "", errors.New("target mismatch")
	}
	if ttl <= 0 || ttl > 30*time.Second {
		return "", errors.New("authorization ttl out of range")
	}
	h, e := p.Hash()
	if e != nil {
		return "", e
	}
	raw := make([]byte, 32)
	if _, e = rand.Read(raw); e != nil {
		return "", e
	}
	tok := hex.EncodeToString(raw)
	return tok, a.store.put(authorization{tok, target, p, h, a.now().Add(ttl)})
}
func (a *Authorizer) Take(ctx context.Context, token, target string, p Plan) (Plan, error) {
	select {
	case <-ctx.Done():
		return Plan{}, ctx.Err()
	default:
	}
	auth, e := a.store.peek(token)
	if e != nil {
		return Plan{}, e
	}
	if auth.Target != target || auth.Plan.Target != target {
		return Plan{}, errors.New("authorization target mismatch")
	}
	if !a.now().Before(auth.ExpiresAt) {
		return Plan{}, errors.New("authorization expired")
	}
	h, e := p.Hash()
	if e != nil || h != auth.Hash {
		return Plan{}, errors.New("plan mismatch")
	}
	if _, e = a.store.take(token); e != nil {
		return Plan{}, e
	}
	return auth.Plan, nil
}

type Device interface {
	Key(ctx context.Context, key string) error
	Text(ctx context.Context, text string) error
	ReleaseAll(ctx context.Context) error
}
type Executor struct {
	mu    sync.Mutex
	now   func() time.Time
	sleep func(context.Context, time.Duration) error
}

type ExtendedDevice interface {
	Device
	Chord(context.Context, string) error
	HoldKey(context.Context, string, time.Duration) error
	MouseMove(context.Context, int, int) error
	MouseMovePct(context.Context, float64, float64) error
	MouseClick(context.Context, string, int) error
	MouseScroll(context.Context, int, int) error
}
type ScreenAsserter interface {
	AssertScreen(context.Context, string) error
}

func NewExecutor() *Executor {
	return &Executor{now: time.Now, sleep: func(ctx context.Context, d time.Duration) error {
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			return nil
		}
	}}
}
func (e *Executor) executeAction(ctx context.Context, d Device, a Action) error {
	if x, ok := d.(ExtendedDevice); ok {
		switch a.Type {
		case "key":
			return x.Chord(ctx, a.Value)
		case "hold_key":
			key := a.Key
			if key == "" {
				key = a.Value
			}
			return x.HoldKey(ctx, key, time.Duration(a.DurationMS)*time.Millisecond)
		case "release_all":
			return d.ReleaseAll(ctx)
		case "mouse_move":
			return x.MouseMove(ctx, a.X, a.Y)
		case "mouse_move_pct":
			return x.MouseMovePct(ctx, a.XPct, a.YPct)
		case "mouse_click":
			count := a.Count
			if count == 0 {
				count = 1
			}
			button := a.Button
			if button == "" {
				button = "left"
			}
			return x.MouseClick(ctx, button, count)
		case "mouse_scroll":
			return x.MouseScroll(ctx, a.DX, a.DY)
		}
	}
	if a.Type == "assert_screen" {
		if x, ok := d.(ScreenAsserter); ok {
			return x.AssertScreen(ctx, a.Contains)
		}
		return errors.New("unexpected screen: assertion unavailable")
	}
	switch a.Type {
	case "key":
		return d.Key(ctx, a.Value)
	case "text":
		return d.Text(ctx, a.Value)
	case "wait":
		return e.sleep(ctx, time.Duration(a.DurationMS)*time.Millisecond)
	}
	return nil
}

type ExecutionResult struct {
	OK               bool     `json:"ok"`
	CompletedActions int      `json:"completed_actions"`
	CleanupOK        bool     `json:"cleanup_ok"`
	CleanupErrors    []string `json:"cleanup_errors,omitempty"`
	Error            string   `json:"error,omitempty"`
}

func (e *Executor) ExecuteDetailed(ctx context.Context, d Device, p Plan) ExecutionResult {
	result := ExecutionResult{CleanupOK: true}
	if err := p.validate(); err != nil {
		result.Error = err.Error()
		return result
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	start := e.now()
	defer func() {
		if err := d.ReleaseAll(context.Background()); err != nil {
			result.CleanupOK = false
			result.CleanupErrors = []string{err.Error()}
		}
	}()
	for _, a := range p.Actions {
		if err := ctx.Err(); err != nil {
			result.Error = err.Error()
			return result
		}
		if e.now().Sub(start) >= p.MaxDuration {
			result.Error = "sequence deadline exceeded"
			return result
		}
		if err := e.executeAction(ctx, d, a); err != nil {
			result.Error = err.Error()
			return result
		}
		result.CompletedActions++
	}
	result.OK = true
	return result
}
func (e *Executor) Execute(ctx context.Context, d Device, p Plan) error {
	r := e.ExecuteDetailed(ctx, d, p)
	if r.Error != "" {
		return errors.New(r.Error)
	}
	return nil
}

type Journal struct {
	path string
	mu   sync.Mutex
}

func NewJournal(path string) *Journal { return &Journal{path: path} }
func redact(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, value := range x {
			lower := strings.ToLower(k)
			if strings.Contains(lower, "token") || strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.Contains(lower, "credential") {
				continue
			}
			out[k] = redact(value)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, value := range x {
			out[i] = redact(value)
		}
		return out
	default:
		return v
	}
}
func (j *Journal) Append(v map[string]any) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	v = redact(v).(map[string]any)
	if err := secureDir(filepath.Dir(j.path)); err != nil {
		return err
	}
	f, e := openJournalFile(j.path)
	if e != nil {
		return e
	}
	defer f.Close()
	b, _ := json.Marshal(v)
	_, e = f.Write(append(b, '\n'))
	return e
}
func readJournal(path string) ([]byte, error) { return os.ReadFile(path) }
