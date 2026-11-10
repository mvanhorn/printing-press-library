// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"github.com/spf13/cobra"
	"io"
)

const platformOutputCaptureLimit = 64 << 20

type platformTerminalWriter struct {
	io.Writer
	terminal bool
}

func (w platformTerminalWriter) IsTerminal() bool { return w.terminal }

type platformOutputCapture struct {
	buffer            bytes.Buffer
	overflow          bool
	terminal          bool
	command           *cobra.Command
	analyticsObserved bool
}

func (c *platformOutputCapture) IsTerminal() bool { return c.terminal }

func (c *platformOutputCapture) Write(p []byte) (int, error) {
	if c.overflow || c.buffer.Len()+len(p) > platformOutputCaptureLimit {
		c.overflow = true
		return len(p), nil
	}
	return c.buffer.Write(p)
}

func wrapPlatformCommandOutputs(root *cobra.Command, flags *rootFlags) {
	previous := root.PersistentPreRunE
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		terminal := isTerminal(cmd.OutOrStdout())
		if previous != nil {
			if err := previous(cmd, args); err != nil {
				return err
			}
		}
		if terminal && !isTerminal(cmd.OutOrStdout()) {
			cmd.SetOut(platformTerminalWriter{Writer: cmd.OutOrStdout(), terminal: terminal})
		}
		return nil
	}
	for _, command := range root.Commands() {
		wrapPlatformCommandOutput(command, flags)
	}
}

func wrapPlatformCommandOutput(command *cobra.Command, flags *rootFlags) {
	for _, child := range command.Commands() {
		wrapPlatformCommandOutput(child, flags)
	}
	originalRunE := command.RunE
	originalRun := command.Run
	if originalRunE == nil && originalRun == nil {
		return
	}
	command.Run = nil
	command.RunE = func(cmd *cobra.Command, args []string) error {
		if !platformCommandNeedsGate(cmd) {
			if originalRunE != nil {
				return originalRunE(cmd, args)
			}
			originalRun(cmd, args)
			return nil
		}
		if flags != nil && flags.platformSession != nil && flags.platformSession.GateOutcome == platform.GateVerified {
			restore, err := bindLearningState(flags.platformSession)
			if err != nil {
				return err
			}
			defer restore()
		}
		originalOut := cmd.OutOrStdout()
		capture := &platformOutputCapture{terminal: isTerminal(originalOut), command: cmd}
		cmd.SetOut(capture)
		var runErr error
		if originalRunE != nil {
			runErr = originalRunE(cmd, args)
		} else {
			originalRun(cmd, args)
		}
		cmd.SetOut(originalOut)
		if capture.overflow {
			if flags != nil && flags.platformSession != nil {
				flags.platformSession.MarkOutputTruncated(platform.TruncationReason{Kind: "output_size", Configured: platformOutputCaptureLimit, Observed: ">limit", MoreAvailable: true})
			}
			if runErr == nil {
				runErr = fmt.Errorf("structured output exceeded the %d-byte safety limit", platformOutputCaptureLimit)
			}
			return runErr
		}
		raw := capture.buffer.Bytes()
		trimmed := bytes.TrimSpace(raw)
		if len(trimmed) == 0 {
			var analyticsFailure *platform.AnalyticsFailure
			if errors.As(runErr, &analyticsFailure) {
				_ = emitPlatformAnalyticsFailure(cmd, originalOut, flags, analyticsFailure)
			}
			return runErr
		}
		observePlatformCommandOutput(cmd, trimmed, flags)
		payload := append([]byte(nil), raw...)
		if runErr == nil && !capture.analyticsObserved {
			autoDeclarePlatformAnalytics(cmd, trimmed, flags)
		}
		if json.Valid(trimmed) {
			transformed, metadataErr := applyPlatformOutputMetadata(originalOut, json.RawMessage(trimmed), flags)
			if metadataErr != nil {
				if runErr == nil {
					runErr = metadataErr
				}
			} else {
				payload = transformed
				if platformCommandIsDoctor(cmd) && bytes.Contains(trimmed, []byte("\n")) {
					var indented bytes.Buffer
					if err := json.Indent(&indented, payload, "", "  "); err == nil {
						payload = indented.Bytes()
					}
				}
				if len(raw) > 0 && raw[len(raw)-1] == '\n' {
					payload = append(payload, '\n')
				}
			}
		} else if analyticsErr := validatePlatformAnalytics(flags); analyticsErr != nil && runErr == nil {
			runErr = analyticsErr
		}
		if runErr != nil {
			var analyticsFailure *platform.AnalyticsFailure
			if errors.As(runErr, &analyticsFailure) {
				_ = emitPlatformAnalyticsFailure(cmd, originalOut, flags, analyticsFailure)
				return runErr
			}
		}
		if _, err := io.Copy(originalOut, bytes.NewReader(payload)); err != nil && runErr == nil {
			runErr = err
		}
		return runErr
	}
}

func emitPlatformAnalyticsFailure(cmd *cobra.Command, output io.Writer, flags *rootFlags, failure *platform.AnalyticsFailure) error {
	if cmd == nil || failure == nil {
		return nil
	}
	diagnostic := failure.Diagnostic
	diagnostic.TenantIdentity = nil
	if flags != nil && flags.platformSession != nil && len(flags.platformSession.ObservedIdentity) > 0 {
		identityJSON, err := json.Marshal(flags.platformSession.ObservedIdentity)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(identityJSON)
		diagnostic.TenantIdentity = map[string]string{"verified_identity_sha256": fmt.Sprintf("%x", digest[:])}
	}
	event := map[string]any{
		"error": map[string]any{
			"code":       "analytics_result_rejected",
			"reason":     failure.Reason,
			"diagnostic": diagnostic,
		},
	}
	if flags != nil && flags.platformSession != nil {
		event["meta"] = flags.platformSession.OutputMetadata()
	}
	target := cmd.ErrOrStderr()
	if flags != nil && (flags.asJSON || flags.agent) {
		target = output
	}
	return json.NewEncoder(target).Encode(event)
}
