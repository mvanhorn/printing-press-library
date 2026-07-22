package cli

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/bambu/internal/bambu"
	"github.com/mvanhorn/printing-press-library/library/devices/bambu/internal/cliutil"
	"github.com/spf13/cobra"
)

type lifecyclePayload struct {
	SchemaVersion string               `json:"schema_version"`
	Type          string               `json:"type"`
	OccurredAt    time.Time            `json:"occurred_at"`
	Printer       lifecyclePrinter     `json:"printer"`
	Job           lifecycleJob         `json:"job"`
	Attachment    *lifecycleAttachment `json:"attachment,omitempty"`
	Outcome       *lifecycleOutcome    `json:"outcome,omitempty"`
	Warnings      []string             `json:"warnings"`
}

type lifecyclePrinter struct {
	Serial string `json:"serial"`
	Name   string `json:"name,omitempty"`
	Model  string `json:"model,omitempty"`
}

type lifecycleJob struct {
	Name              string            `json:"name"`
	SourceName        string            `json:"source_name,omitempty"`
	ProjectName       string            `json:"project_name,omitempty"`
	ProfileName       string            `json:"profile_name,omitempty"`
	TaskID            string            `json:"task_id,omitempty"`
	SubtaskID         string            `json:"subtask_id,omitempty"`
	File              string            `json:"file,omitempty"`
	Plate             *int              `json:"plate,omitempty"`
	Percent           *int              `json:"percent,omitempty"`
	RemainingMinutes  *int              `json:"remaining_minutes,omitempty"`
	EstimatedFinishAt *time.Time        `json:"estimated_finish_at,omitempty"`
	CurrentLayer      *int              `json:"current_layer,omitempty"`
	TotalLayers       *int              `json:"total_layers,omitempty"`
	WeightGrams       *float64          `json:"weight_grams,omitempty"`
	Objects           []lifecycleObject `json:"objects,omitempty"`
}

type lifecycleObject struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Skipped bool   `json:"skipped"`
}

type lifecycleAttachment struct {
	Path        string `json:"path"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Bytes       int    `json:"bytes"`
}

type lifecycleOutcome struct {
	Status        string    `json:"status"`
	FinishedAt    time.Time `json:"finished_at"`
	ActualSeconds *int64    `json:"actual_seconds,omitempty"`
	PrintError    int64     `json:"print_error,omitempty"`
}

func newBambuEventsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "events", Short: "Emit provider-neutral print lifecycle events", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newBambuEventsWatchCmd(flags), newBambuEventsMonitorCmd(flags))
	return cmd
}

type lifecycleWatchOptions struct {
	host, assetDir                 string
	maxEvents                      int
	emitCurrent, exitAfterTerminal bool
}

type pendingLifecycleStart struct {
	snapshot   bambu.Snapshot
	occurredAt time.Time
	warnings   []string
}

func newBambuEventsWatchCmd(flags *rootFlags) *cobra.Command {
	var host, assetDir string
	var maxEvents int
	var emitCurrent, exitAfterTerminal bool
	cmd := &cobra.Command{
		Use:         "watch",
		Short:       "Emit print.started and terminal lifecycle payloads as NDJSON",
		Annotations: map[string]string{"pp:interactive": "true", "mcp:local-write": "true"},
		Long:        "Watch local MQTT state and emit provider-neutral NDJSON. The start payload includes estimated completion, exact 3MF weight, and an optional plate-preview attachment reference. The terminal payload includes actual completion time, outcome, and elapsed duration observed by this watcher.",
		Example: strings.Trim(`
  bambu-pp-cli events watch --agent --asset-dir ./bambu-events --max-events 1
  bambu-pp-cli events watch --json --asset-dir ./bambu-events --timeout 24h`, "\n"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if maxEvents < 0 {
				return usageErr(fmt.Errorf("--max-events must be zero or greater"))
			}
			if dryRunOK(flags) {
				preview := lifecyclePayload{SchemaVersion: "1.0", Type: "print.started", OccurredAt: time.Unix(0, 0).UTC(), Printer: lifecyclePrinter{Serial: "configured-printer"}, Job: lifecycleJob{Name: "current print"}, Warnings: []string{}}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(preview)
			}
			watchCtx, watchCancel := eventWatchContext(cmd.Context(), flags, cmd.Flags().Changed("timeout"))
			defer watchCancel()
			encoder := json.NewEncoder(cmd.OutOrStdout())
			return runLifecycleWatch(watchCtx, flags, lifecycleWatchOptions{host: host, assetDir: assetDir, maxEvents: maxEvents, emitCurrent: emitCurrent, exitAfterTerminal: exitAfterTerminal}, func(payload lifecyclePayload) error { return encoder.Encode(payload) }, cmd.ErrOrStderr())
		},
	}
	addHostFlag(cmd, &host)
	cmd.Flags().StringVar(&assetDir, "asset-dir", "", "Directory for validated plate-preview attachments; empty emits payloads without files")
	cmd.Flags().IntVar(&maxEvents, "max-events", 0, "Stop after this many lifecycle payloads; zero watches until timeout or cancellation")
	cmd.Flags().BoolVar(&emitCurrent, "emit-current", true, "Emit print.started immediately when the watcher attaches to an active print")
	cmd.Flags().BoolVar(&exitAfterTerminal, "exit-after-terminal", false, "Exit successfully after a finished, failed, or canceled event")
	return cmd
}

func newBambuEventsMonitorCmd(flags *rootFlags) *cobra.Command {
	var host, outputDir string
	var emitCurrent bool
	cmd := &cobra.Command{
		Use:         "monitor",
		Short:       "Monitor one print and manage its event log and preview assets",
		Annotations: map[string]string{"pp:interactive": "true", "mcp:local-write": "true"},
		Long:        "Wait for one print, emit its start and terminal payloads as NDJSON, and automatically retain the same payloads plus any available plate preview in a private timestamped run directory.",
		Example: strings.Trim(`
  bambu-pp-cli events monitor --agent
  bambu-pp-cli events monitor --printer office --agent --timeout 12h`, "\n"),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dryRunOK(flags) {
				preview := lifecyclePayload{SchemaVersion: "1.0", Type: "print.started", OccurredAt: time.Unix(0, 0).UTC(), Printer: lifecyclePrinter{Serial: "configured-printer"}, Job: lifecycleJob{Name: "next print"}, Warnings: []string{}}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(preview)
			}
			runDir, err := lifecycleMonitorRunDir(outputDir, time.Now().UTC())
			if err != nil {
				return configErr(err)
			}
			assetDir := filepath.Join(runDir, "assets")
			if err := os.MkdirAll(assetDir, 0o700); err != nil {
				return configErr(fmt.Errorf("create monitor run directory: %w", err))
			}
			if err := os.Chmod(runDir, 0o700); err != nil {
				return configErr(fmt.Errorf("secure monitor run directory: %w", err))
			}
			eventsPath := filepath.Join(runDir, "events.ndjson")
			eventsFile, err := os.OpenFile(eventsPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
			if err != nil {
				return configErr(fmt.Errorf("create monitor event log: %w", err))
			}
			defer eventsFile.Close()
			fmt.Fprintf(cmd.ErrOrStderr(), "monitor run: %s\n", runDir)
			watchCtx, watchCancel := eventWatchContext(cmd.Context(), flags, cmd.Flags().Changed("timeout"))
			defer watchCancel()
			encoder := json.NewEncoder(io.MultiWriter(cmd.OutOrStdout(), eventsFile))
			return runLifecycleWatch(watchCtx, flags, lifecycleWatchOptions{host: host, assetDir: assetDir, emitCurrent: emitCurrent, exitAfterTerminal: true}, func(payload lifecyclePayload) error { return encoder.Encode(payload) }, cmd.ErrOrStderr())
		},
	}
	addHostFlag(cmd, &host)
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "Run directory for events.ndjson and preview assets; default is a private timestamped data directory")
	cmd.Flags().BoolVar(&emitCurrent, "emit-current", true, "Emit print.started immediately when the monitor attaches to an active print")
	return cmd
}

func lifecycleMonitorRunDir(outputDir string, now time.Time) (string, error) {
	if outputDir == "" {
		dataDir, err := cliutil.DataDir()
		if err != nil {
			return "", err
		}
		outputDir = filepath.Join(dataDir, "runs", "print-"+now.UTC().Format("20060102T150405.000000000Z"))
	}
	return filepath.Abs(outputDir)
}

func runLifecycleWatch(ctx context.Context, flags *rootFlags, options lifecycleWatchOptions, emit func(lifecyclePayload) error, stderr io.Writer) error {
	connectCtx, connectCancel := context.WithTimeout(ctx, eventConnectTimeout(flags))
	device, err := resolveBambuDevice(connectCtx, flags, options.host)
	if err != nil {
		connectCancel()
		return err
	}
	client, err := bambu.Connect(connectCtx, device.Host, device.Serial, device.AccessCode)
	if err != nil {
		connectCancel()
		return apiErr(err)
	}
	defer client.Close()
	if err := client.RequestPushAll(connectCtx); err != nil {
		connectCancel()
		return apiErr(err)
	}
	connectCancel()
	var startedAt time.Time
	var pendingStart *pendingLifecycleStart
	var startedJob *lifecycleJob
	emitted := 0
	initialized := false
	for options.maxEvents <= 0 || emitted < options.maxEvents {
		snapshot, transitions, err := client.Next(ctx)
		if err != nil {
			return apiErr(err)
		}
		if snapshot.ObservedAt.IsZero() {
			continue
		}
		if pendingStart != nil && lifecycleStartReady(*pendingStart, snapshot) {
			payload := buildPendingLifecyclePayload(ctx, device, *pendingStart, snapshot, options.assetDir)
			if err := emit(payload); err != nil {
				return err
			}
			startedJob = copyLifecycleJob(payload.Job)
			pendingStart = nil
			emitted++
			if options.maxEvents > 0 && emitted >= options.maxEvents {
				return nil
			}
		}
		if !initialized {
			initialized = true
			if options.emitCurrent && (snapshot.State == "RUNNING" || snapshot.State == "PAUSE" || snapshot.State == "PREPARE") {
				startedAt = snapshot.ObservedAt
				attachmentWarning := "started event reflects watcher attachment; authoritative duration requires observing the printer transition"
				if lifecycleStartNeedsETA(snapshot) {
					pendingStart = &pendingLifecycleStart{snapshot: snapshot, occurredAt: startedAt, warnings: []string{attachmentWarning}}
					_ = client.RequestPushAll(ctx)
					continue
				}
				payload := buildLifecyclePayload(ctx, device, snapshot, "started", options.assetDir, time.Time{})
				payload.Warnings = append(payload.Warnings, attachmentWarning)
				if err := emit(payload); err != nil {
					return err
				}
				startedJob = copyLifecycleJob(payload.Job)
				emitted++
				if options.maxEvents > 0 && emitted >= options.maxEvents {
					return nil
				}
			}
		}
		for _, transition := range transitions {
			if pendingStart != nil && (transition.Kind == "finished" || transition.Kind == "failed" || transition.Kind == "canceled") {
				payload := buildPendingLifecyclePayload(ctx, device, *pendingStart, transition.Snapshot, options.assetDir)
				if err := emit(payload); err != nil {
					return err
				}
				startedJob = copyLifecycleJob(payload.Job)
				pendingStart = nil
				emitted++
				if options.maxEvents > 0 && emitted >= options.maxEvents {
					return nil
				}
			}
			if transition.Kind == "started" && lifecycleStartNeedsETA(transition.Snapshot) {
				persistErr := persistEvent(ctx, transition)
				warnings := []string{}
				if persistErr != nil {
					warnings = append(warnings, "lifecycle event could not be persisted: "+persistErr.Error())
				}
				startedAt = transition.Snapshot.ObservedAt
				pendingStart = &pendingLifecycleStart{snapshot: transition.Snapshot, occurredAt: startedAt, warnings: warnings}
				_ = client.RequestPushAll(ctx)
				continue
			}
			payload, persistErr := prepareLifecycleTransition(ctx, device, transition, options.assetDir, startedAt, persistEvent)
			if payload == nil {
				if persistErr != nil {
					fmt.Fprintln(stderr, "warning: lifecycle event could not be persisted:", persistErr)
				}
				continue
			}
			if transition.Kind == "started" {
				startedAt = payload.OccurredAt
			}
			if transition.Kind == "finished" || transition.Kind == "failed" || transition.Kind == "canceled" {
				carryLifecycleJobIdentity(&payload.Job, startedJob)
			}
			if err := emit(*payload); err != nil {
				return err
			}
			if transition.Kind == "started" {
				startedJob = copyLifecycleJob(payload.Job)
			}
			emitted++
			if options.exitAfterTerminal && (transition.Kind == "finished" || transition.Kind == "failed" || transition.Kind == "canceled") {
				return nil
			}
			if options.maxEvents > 0 && emitted >= options.maxEvents {
				return nil
			}
		}
	}
	return nil
}

func lifecycleStartNeedsETA(snapshot bambu.Snapshot) bool {
	return snapshot.RemainingMinutes == nil || *snapshot.RemainingMinutes <= 0
}

func buildPendingLifecyclePayload(ctx context.Context, device deviceContext, pending pendingLifecycleStart, current bambu.Snapshot, assetDir string) lifecyclePayload {
	payloadSnapshot := pending.snapshot
	if sameLifecycleJob(pending.snapshot, current) {
		payloadSnapshot = current
	}
	payload := buildLifecyclePayload(ctx, device, payloadSnapshot, "started", assetDir, time.Time{})
	payload.OccurredAt = pending.occurredAt
	payload.Warnings = append(payload.Warnings, pending.warnings...)
	if payload.Job.EstimatedFinishAt != nil {
		payload.Warnings = append(payload.Warnings, "start payload waited for initialized ETA telemetry")
	}
	return payload
}

func lifecycleStartReady(pending pendingLifecycleStart, current bambu.Snapshot) bool {
	if !sameLifecycleJob(pending.snapshot, current) || current.State == "FINISH" || current.State == "FAILED" || current.State == "CANCELED" {
		return true
	}
	if !lifecycleStartNeedsETA(current) {
		return true
	}
	return current.ObservedAt.Sub(pending.occurredAt) >= 30*time.Second
}

func sameLifecycleJob(first, second bambu.Snapshot) bool {
	if first.TaskID != "" || second.TaskID != "" {
		return first.TaskID != "" && first.TaskID == second.TaskID
	}
	if first.SubtaskID != "" || second.SubtaskID != "" {
		return first.SubtaskID != "" && first.SubtaskID == second.SubtaskID
	}
	return first.GCodeFile != "" && first.GCodeFile == second.GCodeFile
}

func copyLifecycleJob(job lifecycleJob) *lifecycleJob {
	job.Objects = append([]lifecycleObject(nil), job.Objects...)
	return &job
}

func carryLifecycleJobIdentity(terminal *lifecycleJob, started *lifecycleJob) {
	if terminal == nil || started == nil {
		return
	}
	if terminal.TaskID != "" || started.TaskID != "" {
		if terminal.TaskID == "" || terminal.TaskID != started.TaskID {
			return
		}
	} else if terminal.SubtaskID == "" || terminal.SubtaskID != started.SubtaskID {
		return
	}
	terminal.Name = started.Name
	terminal.SourceName = started.SourceName
	terminal.ProjectName = started.ProjectName
	terminal.ProfileName = started.ProfileName
	terminal.WeightGrams = started.WeightGrams
	terminal.Objects = append([]lifecycleObject(nil), started.Objects...)
}

func prepareLifecycleTransition(ctx context.Context, device deviceContext, transition bambu.Event, assetDir string, startedAt time.Time, persist func(context.Context, bambu.Event) error) (*lifecyclePayload, error) {
	persistErr := persist(ctx, transition)
	if !emitsLifecyclePayload(transition.Kind) {
		return nil, persistErr
	}
	payload := buildLifecyclePayload(ctx, device, transition.Snapshot, transition.Kind, assetDir, startedAt)
	if persistErr != nil {
		payload.Warnings = append(payload.Warnings, "lifecycle event could not be persisted: "+persistErr.Error())
	}
	return &payload, persistErr
}

func emitsLifecyclePayload(kind string) bool {
	switch kind {
	case "started", "finished", "failed", "canceled":
		return true
	default:
		return false
	}
}

func eventWatchContext(parent context.Context, flags *rootFlags, timeoutExplicit bool) (context.Context, context.CancelFunc) {
	if timeoutExplicit {
		return boundCtx(parent, flags)
	}
	return parent, func() {}
}

func eventConnectTimeout(flags *rootFlags) time.Duration {
	if flags != nil && flags.timeout > 0 {
		return flags.timeout
	}
	return 60 * time.Second
}

func buildLifecyclePayload(ctx context.Context, device deviceContext, snapshot bambu.Snapshot, kind, assetDir string, startedAt time.Time) lifecyclePayload {
	occurredAt := snapshot.ObservedAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	payload := lifecyclePayload{
		SchemaVersion: "1.0", Type: "print." + kind, OccurredAt: occurredAt,
		Printer:  lifecyclePrinter{Serial: redactSerial(device.Serial), Name: device.Discovery.Name, Model: device.Discovery.Model},
		Job:      lifecycleJob{Name: snapshot.JobName, TaskID: snapshot.TaskID, SubtaskID: snapshot.SubtaskID, File: snapshot.GCodeFile, Plate: snapshot.PlateNumber, Percent: snapshot.Percent, CurrentLayer: snapshot.CurrentLayer, TotalLayers: snapshot.TotalLayers},
		Warnings: []string{},
	}
	if kind == "started" {
		if snapshot.RemainingMinutes != nil && *snapshot.RemainingMinutes > 0 {
			payload.Job.RemainingMinutes = snapshot.RemainingMinutes
		}
		if snapshot.Percent != nil && *snapshot.Percent == 0 && snapshot.CurrentLayer != nil && *snapshot.CurrentLayer > 1 {
			payload.Job.CurrentLayer = nil
			payload.Warnings = append(payload.Warnings, "layer telemetry was not initialized for the new print and was omitted")
		}
	}
	if kind == "started" && snapshot.RemainingMinutes != nil && *snapshot.RemainingMinutes > 0 {
		finish := occurredAt.Add(time.Duration(*snapshot.RemainingMinutes) * time.Minute)
		payload.Job.EstimatedFinishAt = &finish
	}
	if kind == "started" {
		metadataCtx, metadataCancel := context.WithTimeout(ctx, 30*time.Second)
		metadata, err := fetchLifecycleMetadata(metadataCtx, device, snapshot)
		metadataCancel()
		if err != nil {
			payload.Warnings = append(payload.Warnings, "current 3MF metadata unavailable: "+err.Error())
		} else {
			applyLifecycleMetadata(&payload, metadata)
			if err := persistMetadata(ctx, snapshot, metadata); err != nil {
				payload.Warnings = append(payload.Warnings, "3MF metadata could not be persisted: "+err.Error())
			}
			if assetDir != "" && len(metadata.Thumbnail) > 0 {
				filename := lifecycleThumbnailFilename(device, snapshot, metadata)
				output := filepath.Join(assetDir, filename)
				if err := writePrivateFile(output, metadata.Thumbnail); err != nil {
					payload.Warnings = append(payload.Warnings, "plate preview could not be written: "+err.Error())
				} else {
					payload.Attachment = &lifecycleAttachment{Path: output, Filename: filename, ContentType: "image/png", Bytes: len(metadata.Thumbnail)}
				}
			}
		}
	}
	if kind == "finished" || kind == "failed" || kind == "canceled" {
		outcome := &lifecycleOutcome{Status: kind, FinishedAt: occurredAt, PrintError: snapshot.PrintError}
		if !startedAt.IsZero() && occurredAt.After(startedAt) {
			seconds := int64(occurredAt.Sub(startedAt).Seconds())
			outcome.ActualSeconds = &seconds
		}
		payload.Outcome = outcome
	}
	return payload
}

func applyLifecycleMetadata(payload *lifecyclePayload, metadata bambu.Metadata) {
	payload.Job.WeightGrams = metadata.WeightGrams
	payload.Job.ProjectName = metadata.ProjectName
	payload.Job.ProfileName = metadata.ProfileName
	payload.Job.Objects = make([]lifecycleObject, 0, len(metadata.Objects))
	printableNames := make([]string, 0, len(metadata.Objects))
	for _, object := range metadata.Objects {
		payload.Job.Objects = append(payload.Job.Objects, lifecycleObject{ID: object.ID, Name: object.Name, Skipped: object.Skipped})
		if !object.Skipped && strings.TrimSpace(object.Name) != "" {
			printableNames = append(printableNames, object.Name)
		}
	}
	displayName := strings.TrimSpace(metadata.ProjectName)
	if displayName == "" && len(printableNames) == 1 {
		displayName = filepath.Base(printableNames[0])
		displayName = strings.TrimSuffix(displayName, filepath.Ext(displayName))
		displayName = strings.TrimSpace(displayName)
	}
	if displayName == "" || displayName == payload.Job.Name {
		return
	}
	payload.Job.SourceName = payload.Job.Name
	payload.Job.Name = displayName
}

func lifecycleThumbnailFilename(device deviceContext, snapshot bambu.Snapshot, metadata bambu.Metadata) string {
	plate := 0
	if metadata.PlateNumber != nil {
		plate = *metadata.PlateNumber
	} else if snapshot.PlateNumber != nil {
		plate = *snapshot.PlateNumber
	}
	identity := strings.Join([]string{device.Serial, snapshot.TaskID, snapshot.SubtaskID, snapshot.GCodeFile, snapshot.ObservedAt.UTC().Format(time.RFC3339Nano)}, "\x00")
	digest := sha256.Sum256([]byte(identity))
	return fmt.Sprintf("bambu_%x_plate_%d.png", digest[:8], plate)
}

func fetchLifecycleMetadata(ctx context.Context, device deviceContext, snapshot bambu.Snapshot) (bambu.Metadata, error) {
	ftps, err := bambu.DialFTPS(ctx, device.Host, device.Serial, device.AccessCode)
	if err != nil {
		return bambu.Metadata{}, err
	}
	defer ftps.Close()
	return ftps.JobMetadata(snapshot)
}
