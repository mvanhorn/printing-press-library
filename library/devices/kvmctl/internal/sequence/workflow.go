package sequence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// JournalSink receives redacted lifecycle records.
type JournalSink interface{ Append(map[string]any) error }

// deviceEndpoint is implemented by devices that can report the address of the
// physical KVM they drive. The lock must key on this rather than on the
// user-supplied target label: two operators may reach one device under
// different aliases, and only the endpoint identifies the real hardware.
type deviceEndpoint interface {
	Endpoint() string
}

// lockIdentity returns the string that identifies the physical device for
// serialization. It prefers the device's real endpoint and falls back to the
// target label when the device cannot report one.
func lockIdentity(d Device, target string) string {
	if e, ok := d.(deviceEndpoint); ok {
		if ep := strings.TrimSpace(e.Endpoint()); ep != "" {
			return ep
		}
	}
	return target
}

// ExecuteAuthorized consumes token only after all binding and context checks pass.
//
// Lock ordering matters: the device lock is acquired BEFORE the single-use
// authorization is consumed. Acquiring the lock can fail (permissions, a
// misconfigured shared directory, cancellation), and consuming first would burn
// the caller's one-time token on a workflow that never touched the device,
// forcing a re-authorization. Taking the lock first makes those failures
// retryable with the same token.
//
// The lock keys on the device's real endpoint when it can report one, so two
// aliases for the same hardware still serialize; the authorization itself stays
// bound to the caller-supplied target label.
func ExecuteAuthorized(ctx context.Context, a *Authorizer, e *Executor, d Device, token, target string, p Plan, j JournalSink) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return withTargetLock(lockIdentity(d, target), func() error {
		// Re-check cancellation: acquiring the lock may have blocked for a while.
		if err := ctx.Err(); err != nil {
			return err
		}
		bound, expires, err := a.takeWithExpiry(ctx, token, target, p)
		if err != nil {
			return err
		}
		hash, err := bound.Hash()
		if err != nil {
			return err
		}
		appendRecord := func(r map[string]any) error {
			r["target"], r["plan_hash"] = target, hash
			if j != nil {
				return j.Append(r)
			}
			return nil
		}
		if err := appendRecord(map[string]any{"event": "sequence_start", "actions": len(bound.Actions)}); err != nil {
			return err
		}
		err = e.executeUntil(ctx, d, bound, expires, func(index int, a Action) error {
			return appendRecord(map[string]any{"event": "action", "index": index, "type": a.Type})
		})
		end := map[string]any{"event": "sequence_end", "ok": err == nil}
		if err != nil {
			end["error"] = err.Error()
		}
		if journalErr := appendRecord(end); err == nil && journalErr != nil {
			err = journalErr
		}
		return err
	})
}

func (a *Authorizer) takeWithExpiry(ctx context.Context, token, target string, p Plan) (Plan, time.Time, error) {
	select {
	case <-ctx.Done():
		return Plan{}, time.Time{}, ctx.Err()
	default:
	}
	h, err := p.Hash()
	if err != nil {
		return Plan{}, time.Time{}, err
	}
	auth, err := a.store.takeMatching(token, func(auth authorization) error {
		if auth.Target != target || auth.Plan.Target != target {
			return errors.New("authorization target mismatch")
		}
		if !a.now().Before(auth.ExpiresAt) {
			return errors.New("authorization expired")
		}
		if h != auth.Hash {
			return errors.New("plan mismatch")
		}
		return nil
	})
	if err != nil {
		return Plan{}, time.Time{}, err
	}
	return auth.Plan, auth.ExpiresAt, nil
}

func (e *Executor) executeUntil(ctx context.Context, d Device, p Plan, expires time.Time, onAction func(int, Action) error) error {
	if err := p.validate(); err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	start := e.now()
	defer func() { _ = d.ReleaseAll(context.Background()) }()
	for i, a := range p.Actions {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !e.now().Before(expires) {
			return errors.New("authorization expired")
		}
		if e.now().Sub(start) >= p.MaxDuration {
			return errors.New("sequence deadline exceeded")
		}
		if onAction != nil {
			if err := onAction(i, a); err != nil {
				return err
			}
		}
		action := a
		if action.Type == "wait" {
			remaining := p.MaxDuration - e.now().Sub(start)
			if exp := expires.Sub(e.now()); exp < remaining {
				remaining = exp
			}
			if remaining <= 0 {
				return errors.New("sequence deadline exceeded")
			}
			if waitFor := time.Duration(action.DurationMS) * time.Millisecond; waitFor > remaining {
				action.DurationMS = int(remaining / time.Millisecond)
				if action.DurationMS < 1 {
					return errors.New("sequence deadline exceeded")
				}
			}
		}
		err := e.executeAction(ctx, d, action)
		if err != nil {
			return err
		}
		if !e.now().Before(expires) {
			return errors.New("authorization expired")
		}
		if e.now().Sub(start) >= p.MaxDuration {
			return errors.New("sequence deadline exceeded")
		}
	}
	return nil
}

// KVMDAPI is the subset of the KVMD client required by a sequence.
type KVMDAPI interface {
	KVMDKey(context.Context, string, bool) error
}

type KVMDDevice struct {
	api  KVMDAPI
	mu   sync.Mutex
	held map[string]bool
}

type shortcutAPI interface {
	KVMDShortcut(context.Context, string) error
}
type mouseAPI interface {
	KVMDMouseMove(context.Context, int, int) error
	KVMDMouseButton(context.Context, string, bool) error
	KVMDMouseWheel(context.Context, int, int) error
}

func NewKVMDDevice(api KVMDAPI) *KVMDDevice { return &KVMDDevice{api: api, held: map[string]bool{}} }

// endpointAPI is satisfied by the generated KVMD client, which knows the base
// URL of the device it talks to.
type endpointAPI interface {
	RequestBaseURL() string
}

// Endpoint reports the address of the physical KVM this device drives so that
// serialization keys on the hardware rather than on a user-chosen alias.
// It returns "" when the underlying client cannot report one, in which case the
// caller falls back to the target label.
func (d *KVMDDevice) Endpoint() string {
	if a, ok := d.api.(endpointAPI); ok {
		return a.RequestBaseURL()
	}
	return ""
}

func (d *KVMDDevice) Chord(ctx context.Context, keys string) error {
	if a, ok := d.api.(shortcutAPI); ok {
		return a.KVMDShortcut(ctx, strings.ReplaceAll(keys, "+", ","))
	}
	return d.Key(ctx, keys)
}
func (d *KVMDDevice) HoldKey(ctx context.Context, key string, dur time.Duration) error {
	if err := d.api.KVMDKey(ctx, key, true); err != nil {
		return err
	}
	d.mu.Lock()
	d.held[key] = true
	d.mu.Unlock()
	t := time.NewTimer(dur)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
	}
	err := d.api.KVMDKey(ctx, key, false)
	d.mu.Lock()
	delete(d.held, key)
	d.mu.Unlock()
	return err
}
func (d *KVMDDevice) MouseMove(ctx context.Context, x, y int) error {
	a, ok := d.api.(mouseAPI)
	if !ok {
		return errors.New("mouse unavailable")
	}
	return a.KVMDMouseMove(ctx, x, y)
}
func (d *KVMDDevice) MouseMovePct(context.Context, float64, float64) error {
	return errors.New("mouse percentage unavailable")
}
func (d *KVMDDevice) MouseClick(ctx context.Context, button string, count int) error {
	a, ok := d.api.(mouseAPI)
	if !ok {
		return errors.New("mouse unavailable")
	}
	for i := 0; i < count; i++ {
		if err := a.KVMDMouseButton(ctx, button, true); err != nil {
			return err
		}
		if err := a.KVMDMouseButton(ctx, button, false); err != nil {
			return err
		}
	}
	return nil
}
func (d *KVMDDevice) MouseScroll(ctx context.Context, dx, dy int) error {
	a, ok := d.api.(mouseAPI)
	if !ok {
		return errors.New("mouse unavailable")
	}
	return a.KVMDMouseWheel(ctx, dx, dy)
}
func (d *KVMDDevice) Key(ctx context.Context, key string) error {
	if err := d.api.KVMDKey(ctx, key, true); err != nil {
		return err
	}
	d.mu.Lock()
	d.held[key] = true
	d.mu.Unlock()
	if err := d.api.KVMDKey(ctx, key, false); err != nil {
		return err
	}
	d.mu.Lock()
	delete(d.held, key)
	d.mu.Unlock()
	return nil
}
func (d *KVMDDevice) Text(ctx context.Context, text string) error {
	for _, r := range text {
		key, shift, ok := asciiKey(r)
		if !ok {
			return fmt.Errorf("unsupported character %q", r)
		}
		if shift {
			if err := d.api.KVMDKey(ctx, "ShiftLeft", true); err != nil {
				return err
			}
		}
		if err := d.Key(ctx, key); err != nil {
			return err
		}
		if shift {
			if err := d.api.KVMDKey(ctx, "ShiftLeft", false); err != nil {
				return err
			}
		}
	}
	return nil
}
func (d *KVMDDevice) ReleaseAll(ctx context.Context) error {
	d.mu.Lock()
	keys := make([]string, 0, len(d.held))
	for k := range d.held {
		keys = append(keys, k)
	}
	d.held = map[string]bool{}
	d.mu.Unlock()
	var first error
	for _, k := range keys {
		if err := d.api.KVMDKey(ctx, k, false); err != nil && first == nil {
			first = err
		}
	}
	return first
}
func asciiKey(r rune) (string, bool, bool) {
	if r >= 'a' && r <= 'z' {
		return "Key" + string(r-'a'+'A'), false, true
	}
	if r >= 'A' && r <= 'Z' {
		return "Key" + string(r), true, true
	}
	if r >= '0' && r <= '9' {
		return "Digit" + string(r), false, true
	}
	if r == ' ' {
		return "Space", false, true
	}
	return "", false, false
}
