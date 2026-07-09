package june

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

// RecordResult summarizes a completed recording. Cook is June's cook-plan name
// for the session (see cookstore Session.Cook).
type RecordResult struct {
	SessionID    int64  `json:"session_id"`
	Cook         string `json:"cook"`
	TargetF      int    `json:"target_f"`
	Samples      int    `json:"temperature_samples"`
	CameraFrames int    `json:"camera_frames"`
	Outcome      string `json:"outcome"`
	Note         string `json:"note,omitempty"`
}

// Record attaches to the live cook and persists the session plus every telemetry
// sample into the cook store until the cook ends or ctx is cancelled. Requires an
// active cook (starts from the current status's mode/target). Some June firmware
// streams only camera frames and no cavity telemetry; the session and its outcome
// are still recorded, and the camera-frame count is reported.
func Record(ctx context.Context, id *Identity, cs *CookStore, label string) (RecordResult, error) {
	raw, err := NewClient(id).Status(ctx)
	if err != nil {
		return RecordResult{}, err
	}
	st, err := ParseStatus(raw)
	if err != nil {
		return RecordResult{}, err
	}
	if st.State != "active" {
		return RecordResult{}, fmt.Errorf("no active cook to record — start one with 'preheat' first")
	}
	target := 0
	if st.TargetF != nil {
		target = *st.TargetF
	}
	sessionID, err := cs.StartSession(ctx, label, st.CookName, target, time.Now())
	if err != nil {
		return RecordResult{}, err
	}

	// Final writes must not use ctx (it may be the recording timeout, already
	// expired by the time Watch returns), or the outcome silently never persists.
	writeCtx := context.WithoutCancel(ctx)
	samples, frames := 0, 0
	outcome := "interrupted"
	watchErr := Watch(ctx, id, func(ev TelemetryEvent) {
		switch ev.Type {
		case "telemetry":
			cavity, progress := 0, 0
			if ev.CurrentF != nil {
				cavity = *ev.CurrentF
			}
			if ev.Progress != nil {
				progress = *ev.Progress
			}
			if cs.AppendSample(writeCtx, sessionID, time.Now(), cavity, progress) == nil {
				samples++
			}
		case "camera":
			frames++
		case "cancelled":
			outcome = "cancelled"
		case "state":
			if ev.State == "idle" && outcome == "interrupted" {
				outcome = "completed"
			}
		}
	})
	_ = cs.EndSession(writeCtx, sessionID, outcome, time.Now())
	res := RecordResult{SessionID: sessionID, Cook: st.CookName, TargetF: target, Samples: samples, CameraFrames: frames, Outcome: outcome}
	if samples == 0 && frames > 0 {
		res.Note = "this oven streamed camera frames but no cavity temperature; session logged, but temperature-curve features (curve/preheat-stats) have no data for it"
	}
	if watchErr != nil && ctx.Err() == nil {
		return res, watchErr
	}
	return res, nil
}

// ReadyResult reports the outcome of a blocking wait-until-preheated.
type ReadyResult struct {
	Ready      bool   `json:"ready"`
	TargetF    int    `json:"target_f"`
	FinalF     int    `json:"final_f"`
	ElapsedSec int    `json:"elapsed_seconds"`
	TimedOut   bool   `json:"timed_out"`
	Note       string `json:"note,omitempty"`
}

// noTelemetryProbe is how long WaitReady/LiveETA wait for a first cavity-telemetry
// frame before concluding the oven does not stream temperature.
const noTelemetryProbe = 25 * time.Second

// WaitReady blocks until the oven's cavity reaches the active target (within
// tolerance °F) or the timeout elapses. If the oven streams no cavity telemetry
// (some firmware sends only camera frames), it bails early with a clear note
// rather than hanging the full timeout. Returns Ready=false on timeout so callers
// can map it to a non-zero exit.
func WaitReady(ctx context.Context, id *Identity, tolerance int, timeout time.Duration) (ReadyResult, error) {
	raw, err := NewClient(id).Status(ctx)
	if err != nil {
		return ReadyResult{}, err
	}
	st, err := ParseStatus(raw)
	if err != nil {
		return ReadyResult{}, err
	}
	if st.State != "active" || st.TargetF == nil {
		return ReadyResult{}, fmt.Errorf("oven is not preheating — start a cook with 'preheat' first")
	}
	target := *st.TargetF

	wctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	start := time.Now()
	res := ReadyResult{TargetF: target}
	var sawTelemetry atomic.Bool

	// Early bail: if no cavity telemetry frame arrives within the probe window,
	// this oven cannot report readiness by temperature.
	go func() {
		t := time.NewTimer(noTelemetryProbe)
		defer t.Stop()
		select {
		case <-wctx.Done():
		case <-t.C:
			if !sawTelemetry.Load() {
				cancel()
			}
		}
	}()

	err = Watch(wctx, id, func(ev TelemetryEvent) {
		if ev.Type == "telemetry" && ev.CurrentF != nil {
			sawTelemetry.Store(true)
			res.FinalF = *ev.CurrentF
			if *ev.CurrentF >= target-tolerance {
				res.Ready = true
				cancel()
			}
		}
	})
	res.ElapsedSec = int(time.Since(start).Seconds())
	if !res.Ready {
		res.TimedOut = true
		if !sawTelemetry.Load() {
			res.Note = "this oven streams no live cavity temperature (camera frames only), so readiness cannot be detected by temperature. Use 'watch' to observe the cook, or the oven's own screen/app for the preheat chime."
		}
	}
	if err != nil && wctx.Err() == nil && ctx.Err() == nil {
		return res, err
	}
	return res, nil
}

// ETAResult is a non-blocking predicted time-to-ready.
type ETAResult struct {
	TargetF      int     `json:"target_f"`
	CurrentF     int     `json:"current_f"`
	ClimbFPerMin float64 `json:"climb_f_per_min"`
	ETASeconds   int     `json:"eta_seconds"`
	Note         string  `json:"note,omitempty"`
}

// LiveETA samples telemetry for a few seconds to estimate the climb rate and
// linearly extrapolates the time to reach the target. Returns immediately.
func LiveETA(ctx context.Context, id *Identity, sampleFor time.Duration) (ETAResult, error) {
	raw, err := NewClient(id).Status(ctx)
	if err != nil {
		return ETAResult{}, err
	}
	st, err := ParseStatus(raw)
	if err != nil {
		return ETAResult{}, err
	}
	if st.State != "active" || st.TargetF == nil {
		return ETAResult{}, fmt.Errorf("oven is not preheating — no ETA to compute")
	}
	target := *st.TargetF

	type point struct {
		t time.Time
		f int
	}
	var pts []point
	wctx, cancel := context.WithTimeout(ctx, sampleFor)
	defer cancel()
	_ = Watch(wctx, id, func(ev TelemetryEvent) {
		if ev.Type == "telemetry" && ev.CurrentF != nil {
			pts = append(pts, point{time.Now(), *ev.CurrentF})
		}
	})

	res := ETAResult{TargetF: target}
	if len(pts) == 0 {
		res.Note = "no telemetry frames observed in the sampling window"
		return res, nil
	}
	res.CurrentF = pts[len(pts)-1].f
	if len(pts) < 2 {
		res.Note = "only one telemetry frame; climb rate needs at least two"
		return res, nil
	}
	first, last := pts[0], pts[len(pts)-1]
	dtMin := last.t.Sub(first.t).Minutes()
	if dtMin <= 0 {
		res.Note = "insufficient time spread to estimate climb rate"
		return res, nil
	}
	rate := float64(last.f-first.f) / dtMin
	res.ClimbFPerMin = rate
	if res.CurrentF >= target {
		res.ETASeconds = 0
		res.Note = "already at target"
		return res, nil
	}
	if rate <= 0 {
		res.Note = "temperature not rising; cannot estimate ETA"
		return res, nil
	}
	res.ETASeconds = int(float64(target-res.CurrentF) / rate * 60)
	return res, nil
}
