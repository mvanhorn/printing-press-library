// PATCH(library): switch/OTG timing parity with Python kvmctl/switch.py.
// Package switcher plans and executes sequential HID switch protocols.
package switcher

import (
	"context"
	"fmt"
	"time"
)

type Event struct {
	Key   string `json:"key"`
	State string `json:"state"`
}
type Profile struct {
	Name                       string
	MinPort, MaxPort           int
	Sequence                   [][2]string
	InterKeyDelay, SettleDelay time.Duration
	Hold                       time.Duration // optional hold duration for held-key mode
}

var TH413 = Profile{Name: "terived-th41-3", MinPort: 1, MaxPort: 4, Sequence: [][2]string{{"ControlRight", "tap"}, {"ControlRight", "tap"}, {"Digit{port}", "tap"}, {"Enter", "tap"}}, InterKeyDelay: 200 * time.Millisecond, SettleDelay: time.Second}

// Held-key profile mirrors Python's held-key recipe: 120ms hold, 150ms gap.
var TH413Held = Profile{Name: "terived-th41-3-held", MinPort: 1, MaxPort: 4, Sequence: [][2]string{{"ControlRight", "tap"}, {"ControlRight", "tap"}, {"Digit{port}", "tap"}, {"Enter", "tap"}}, InterKeyDelay: 150 * time.Millisecond, SettleDelay: 5 * time.Second, Hold: 120 * time.Millisecond}

const (
	OTGOnWait  = 8 * time.Second
	OTGOffWait = 12 * time.Second
	HoldMS     = 120 * time.Millisecond
	GapMS      = 150 * time.Millisecond
)

func Plan(p Profile, port int) ([]Event, error) {
	if port < p.MinPort || port > p.MaxPort {
		return nil, fmt.Errorf("port %d out of range %d-%d for profile %s", port, p.MinPort, p.MaxPort, p.Name)
	}
	out := []Event{}
	for _, step := range p.Sequence {
		key := step[0]
		for i := 0; i+len("{port}") <= len(key); i++ {
			if key[i:i+len("{port}")] == "{port}" {
				key = key[:i] + fmt.Sprint(port) + key[i+len("{port}"):]
				break
			}
		}
		switch step[1] {
		case "tap":
			out = append(out, Event{key, "down"}, Event{key, "up"})
		case "press":
			out = append(out, Event{key, "down"})
		case "release":
			out = append(out, Event{key, "up"})
		default:
			return nil, fmt.Errorf("unknown action %q", step[1])
		}
	}
	return out, nil
}

// HIDClient emits key events.
type HIDClient interface {
	KeyDown(ctx context.Context, key string) error
	KeyUp(ctx context.Context, key string) error
}

// OTGClient toggles OTG gadget functions.
type OTGClient interface {
	Request(ctx context.Context, method, path string, params map[string]string) error
}

func sleepCtx(ctx context.Context, d time.Duration, sleep func(context.Context, time.Duration) error) error {
	if d <= 0 {
		return nil
	}
	if sleep != nil {
		return sleep(ctx, d)
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Rearm toggles /api/system/otg_functions true then false with exact Python sleeps (8s / 12s).
func Rearm(ctx context.Context, c OTGClient, sleep func(context.Context, time.Duration) error) error {
	if err := c.Request(ctx, "POST", "/api/system/otg_functions", map[string]string{"start_cdrom": "true", "start_flash": "true"}); err != nil {
		return fmt.Errorf("OTG bounce failed before any keys were sent: %w", err)
	}
	if err := sleepCtx(ctx, OTGOnWait, sleep); err != nil {
		return err
	}
	if err := c.Request(ctx, "POST", "/api/system/otg_functions", map[string]string{"start_cdrom": "false", "start_flash": "false"}); err != nil {
		return fmt.Errorf("OTG bounce failed: %w", err)
	}
	return sleepCtx(ctx, OTGOffWait, sleep)
}

// Execute emits the profile's events with exact sleep sequencing.
// Held-key mode (Hold >0): down -> sleep(Hold) -> up -> sleep(InterKeyDelay) per tap, then Settle.
// Legacy tap mode: gap before every event after first, then Settle.
func Execute(ctx context.Context, c HIDClient, p Profile, port int, sleep func(context.Context, time.Duration) error) ([]Event, error) {
	events, err := Plan(p, port)
	if err != nil {
		return nil, err
	}
	if p.Hold > 0 {
		// strict tap pairs
		type pair struct{ down, up Event }
		var pairs []pair
		for i := 0; i < len(events); i += 2 {
			if i+1 >= len(events) || events[i].State != "down" || events[i+1].State != "up" || events[i].Key != events[i+1].Key {
				return nil, fmt.Errorf("held-key mode requires tap steps; mismatch at %d", i)
			}
			pairs = append(pairs, pair{events[i], events[i+1]})
		}
		for _, pr := range pairs {
			if err := ctx.Err(); err != nil {
				return events, err
			}
			if err := c.KeyDown(ctx, pr.down.Key); err != nil {
				return events, err
			}
			if err := sleepCtx(ctx, p.Hold, sleep); err != nil {
				_ = c.KeyUp(context.WithoutCancel(ctx), pr.down.Key)
				return events, err
			}
			if err := c.KeyUp(ctx, pr.up.Key); err != nil {
				return events, err
			}
			if err := sleepCtx(ctx, p.InterKeyDelay, sleep); err != nil {
				return events, err
			}
		}
	} else {
		for i, ev := range events {
			if err := ctx.Err(); err != nil {
				return events, err
			}
			if i > 0 {
				if err := sleepCtx(ctx, p.InterKeyDelay, sleep); err != nil {
					return events, err
				}
			}
			if ev.State == "down" {
				if err := c.KeyDown(ctx, ev.Key); err != nil {
					return events, err
				}
			} else {
				if err := c.KeyUp(ctx, ev.Key); err != nil {
					return events, err
				}
			}
		}
	}
	if err := sleepCtx(ctx, p.SettleDelay, sleep); err != nil {
		return events, err
	}
	return events, nil
}
