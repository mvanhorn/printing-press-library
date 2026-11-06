// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tella/internal/client"
	"github.com/spf13/cobra"
)

func videoPath(id string) string         { return "/v1/videos/" + url.PathEscape(id) }
func clipPath(video, clip string) string { return videoPath(video) + "/clips/" + url.PathEscape(clip) }

func workflowArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return nil
		}
		if err := cobra.ExactArgs(n)(cmd, args); err != nil {
			return err
		}
		for _, id := range args {
			if strings.TrimSpace(id) == "" || strings.ContainsAny(id, "/\\?#\x00") || id == "." || id == ".." {
				return fmt.Errorf("invalid target ID")
			}
		}
		return nil
	}
}

// Harness sweeps without fixtures must never reach the network.
func workflowOffline(flags *rootFlags) bool { return flags.dryRun && requiredInputWaived() }

func workflowClient(flags *rootFlags) (*client.Client, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	c.DryRun = false
	c.NoCache = true
	return c, nil
}

type lessonTimeline struct {
	Video struct {
		ID         string  `json:"id"`
		Name       string  `json:"name"`
		DurationMs float64 `json:"durationMs"`
	} `json:"video"`
	Clips []struct {
		ID              string  `json:"id"`
		Name            string  `json:"name"`
		TimelineStartMs float64 `json:"timelineStartMs"`
		DurationMs      float64 `json:"durationMs"`
	} `json:"clips"`
}

func readLessonTimeline(ctx context.Context, c *client.Client, id string) (lessonTimeline, json.RawMessage, error) {
	raw, err := c.Get(ctx, videoPath(id)+"/timeline", map[string]string{"include": "all"})
	var timeline lessonTimeline
	if err != nil {
		return timeline, nil, err
	}
	if err = json.Unmarshal(raw, &timeline); err != nil {
		return timeline, nil, err
	}
	if timeline.Video.ID == "" || timeline.Clips == nil {
		return timeline, nil, fmt.Errorf("timeline missing video identity or clips")
	}
	if timeline.Video.ID != id {
		return timeline, nil, fmt.Errorf("timeline video identity does not match requested video %s", id)
	}
	var positions struct {
		Clips []struct {
			Start *float64 `json:"timelineStartMs"`
		} `json:"clips"`
	}
	if err = json.Unmarshal(raw, &positions); err != nil {
		return timeline, nil, err
	}
	seen := map[string]bool{}
	previousStart := float64(-1)
	for i, clip := range timeline.Clips {
		start := positions.Clips[i].Start
		if start == nil || *start < 0 || *start < previousStart {
			return timeline, nil, fmt.Errorf("timeline clip %s has missing, negative or unordered timelineStartMs", clip.ID)
		}
		previousStart = *start
		if clip.ID == "" || seen[clip.ID] || clip.DurationMs < 0 {
			return timeline, nil, fmt.Errorf("invalid timeline clip")
		}
		seen[clip.ID] = true
	}
	return timeline, raw, nil
}

type livePreview struct {
	URL       string `json:"url"`
	ExpiresAt string `json:"expiresAt"`
	Expiry    string `json:"expiry"`
}

func createLessonPreview(ctx context.Context, c *client.Client, id string) (livePreview, error) {
	raw, _, err := c.Post(ctx, videoPath(id)+"/live-preview", nil)
	var p livePreview
	if err != nil {
		return p, err
	}
	if err = json.Unmarshal(raw, &p); err != nil {
		return p, err
	}
	u, e := url.Parse(p.URL)
	if e != nil || u.Scheme != "https" || u.Host == "" || p.ExpiresAt == "" {
		return p, fmt.Errorf("preview response missing URL or expiresAt")
	}
	if _, err = time.Parse(time.RFC3339, p.ExpiresAt); err != nil {
		return p, fmt.Errorf("invalid preview expiresAt: %w", err)
	}
	p.Expiry = "Expires after 24 hours; this URL is a bearer credential."
	return p, nil
}

func printPreview(cmd *cobra.Command, flags *rootFlags, p livePreview) error {
	if flags.asJSON || flags.selectFields != "" || flags.compact {
		return printJSONFiltered(cmd.OutOrStdout(), p, flags)
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\nExpires at %s (24 hours).\n", p.URL, p.ExpiresAt)
	return err
}

type autoOptions struct {
	Zooms     bool   `json:"zooms"`
	Crop      bool   `json:"crop"`
	Fillers   bool   `json:"fillers"`
	Silences  string `json:"silences,omitempty"`
	ZoomStyle string `json:"zoomStyle,omitempty"`
}

func (o autoOptions) validate() error {
	if o.Silences != "" && o.Silences != "natural" && o.Silences != "fast" && o.Silences != "faster" {
		return fmt.Errorf("silences must be natural, fast, or faster")
	}
	if o.ZoomStyle != "" && !strings.Contains("|gentle|subtle|moderate|focused|intense|slow|medium|fast|", "|"+o.ZoomStyle+"|") {
		return fmt.Errorf("invalid zoom-style")
	}
	return nil
}

type autoPlan struct {
	VideoID    string                     `json:"video_id"`
	Options    autoOptions                `json:"options"`
	Plans      []cleanClipPlan            `json:"clips"`
	Snapshots  []cutSnapshot              `json:"-"`
	VideoState json.RawMessage            `json:"-"`
	Timeline   json.RawMessage            `json:"-"`
	ClipState  map[string]json.RawMessage `json:"-"`
}

func planAutomatics(ctx context.Context, c *client.Client, id string, o autoOptions) (autoPlan, error) {
	p := autoPlan{VideoID: id, Options: o, Plans: []cleanClipPlan{}, ClipState: map[string]json.RawMessage{}}
	t, raw, err := readLessonTimeline(ctx, c, id)
	if err != nil {
		return p, err
	}
	p.Timeline = raw
	p.VideoState, err = c.Get(ctx, videoPath(id), nil)
	if err != nil {
		return p, err
	}
	for _, clip := range t.Clips {
		plan, snapshot, err := planCleanClip(ctx, c, id, clip.ID, cleanOptions{RemoveFillers: o.Fillers, RemoveSilences: o.Silences})
		if err != nil {
			return p, err
		}
		state, err := c.Get(ctx, clipPath(id, clip.ID), nil)
		if err != nil {
			return p, err
		}
		p.ClipState[clip.ID] = state
		p.Plans = append(p.Plans, plan)
		p.Snapshots = append(p.Snapshots, snapshot)
	}
	return p, nil
}

const lessonRecoveryLimit = "Cut snapshots restore cuts only, with divergence checks and exclusive editing confirmation. Recorded timeline/clip settings, crops, layouts, zooms, music, captions, chapters, thumbnail and inserted media are reference evidence for manual recovery; full editor undo is unavailable."

type autoRecovery struct {
	Cuts   map[string]string `json:"cut_snapshots"`
	State  string            `json:"recorded_state"`
	Limits string            `json:"limits"`
}

func saveAutoRecovery(p autoPlan) (autoRecovery, error) {
	r := autoRecovery{Cuts: map[string]string{}, Limits: lessonRecoveryLimit}
	for _, s := range p.Snapshots {
		name, err := persistCutSnapshot(s)
		if err != nil {
			return r, err
		}
		r.Cuts[s.ClipID] = name
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return r, err
	}
	dir := filepath.Join(base, "tella-pp-cli", "lesson-snapshots")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return r, err
	}
	f, err := os.CreateTemp(dir, "state-*.json")
	if err != nil {
		return r, err
	}
	r.State = f.Name()
	data, err := json.MarshalIndent(map[string]any{"video": jsonRawToAny(p.VideoState), "timeline": jsonRawToAny(p.Timeline), "clips": p.ClipState, "limits": r.Limits}, "", "  ")
	err = errors.Join(err, f.Chmod(0600))
	if err == nil {
		_, err = f.Write(data)
	}
	return r, errors.Join(err, f.Close())
}

func applyAutomatics(ctx context.Context, c *client.Client, p autoPlan, r autoRecovery) error {
	if len(p.Plans) == 0 {
		return fmt.Errorf("video has no clips; no automatic edits can be applied")
	}
	for _, snapshot := range p.Snapshots {
		current, err := captureCutSnapshot(ctx, c, snapshot.VideoID, snapshot.ClipID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(current.Cuts, snapshot.Cuts) {
			return fmt.Errorf("cuts changed since planning for clip %s; review a fresh plan", snapshot.ClipID)
		}
	}
	result, err := applyCleanPlans(ctx, c, p.Plans, p.Snapshots)
	err = errors.Join(err, updateCutSnapshotFiles(r.Cuts, p.Snapshots))
	if err != nil {
		return fmt.Errorf("automatic cuts stopped (%d operations applied): %w", result.AppliedOps, err)
	}
	for _, plan := range p.Plans {
		base := clipPath(p.VideoID, plan.ClipID)
		if p.Options.Zooms {
			body := map[string]any{"replaceExisting": false}
			if p.Options.ZoomStyle != "" {
				body["intensity"] = p.Options.ZoomStyle
			}
			if _, _, err = c.Post(ctx, base+"/zooms/auto", body); err != nil {
				return err
			}
		}
		if p.Options.Crop {
			if _, _, err = c.Post(ctx, base+"/auto-crop", map[string]any{}); err != nil {
				return err
			}
		}
	}
	return nil
}

func workflowFailure(cmd *cobra.Command, flags *rootFlags, recovery any, err error) error {
	if err != nil {
		recoveryFlags := *flags
		recoveryFlags.selectFields = ""
		recoveryFlags.compact = false
		recoveryFlags.asJSON = true
		_ = printJSONFiltered(cmd.ErrOrStderr(), map[string]any{"error": err.Error(), "recovery": recovery, "stopped": true}, &recoveryFlags)
	}
	return err
}
