// Package host provides bounded, allowlisted remote probes.
package host

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/results"
)

type ProbeError struct{ msg string }

func (e *ProbeError) Error() string { return e.msg }

func probeErr(msg string) error { return &ProbeError{msg: msg} }

// Result is a bounded runner result.
type Result struct {
	Code   int
	Stdout string
}

// Runner is an argv-only executor.
type Runner interface {
	Run(context.Context, []string, time.Duration) (Result, error)
}

// Profile mirrors Python HostProbeProfile.
type Profile struct {
	Service string
	DRMNode string
	Timeout time.Duration
}

var (
	nameRE       = regexp.MustCompile(`^[A-Za-z0-9_.@-]+$`)
	nodeRE       = regexp.MustCompile(`^/dev/dri/(card|renderD|controlD)[0-9]+$`)
	secretRE     = regexp.MustCompile(`(?i)(password|passwd|token|secret|private[_ -]?key)\s*[:=]`)
	hostnameRE   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]{0,252}$`)
	drmNodeRE    = regexp.MustCompile(`^(?:card\d+|renderD\d+|controlD\d+)$`)
	pciRE        = regexp.MustCompile(`^(?P<address>[0-9a-fA-F:.]+) (?P<class>[^\[]+) \[[0-9a-fA-F]{4}\]: (?P<description>.+?) \[[0-9a-fA-F]{4}:[0-9a-fA-F]{4}\]$`)
	pciLookingRE = regexp.MustCompile(`^[0-9a-fA-F:.]+\s+[^[]+\[[0-9a-fA-F]{4}\]:`)
	driverRE     = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	keyRE        = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
	allowedLine  = regexp.MustCompile(`^\s*(?:Subsystem:|Flags:|Kernel modules:|IOMMU group |NUMA node:|Memory at |Capabilities:|Physical Slot:|Rev:|Latency:|Region )`)
)

// invoke runs runner with a bounded timeout via goroutine, mirroring Python's thread+queue.
func invoke(ctx context.Context, r Runner, argv []string, timeout time.Duration) (Result, error) {
	type res struct {
		v   Result
		err error
	}
	ch := make(chan res, 1)
	go func() {
		v, err := r.Run(ctx, argv, timeout)
		ch <- res{v, err}
	}()
	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	case got := <-ch:
		if got.err != nil {
			return Result{}, probeErr(fmt.Sprintf("probe command failed: %s", argv[0]))
		}
		return got.v, nil
	case <-time.After(timeout):
		return Result{}, probeErr(fmt.Sprintf("probe command timed out: %s", argv[0]))
	}
}

func output(ctx context.Context, r Runner, argv []string, limit int, timeout time.Duration) (string, error) {
	res, err := invoke(ctx, r, argv, timeout)
	if err != nil {
		return "", err
	}
	if res.Code != 0 {
		return "", probeErr(fmt.Sprintf("probe command failed: %s", argv[0]))
	}
	value := res.Stdout
	if len([]byte(value)) > limit {
		return "", probeErr("malformed probe output: output exceeds bound")
	}
	if strings.Contains(value, "\x00") {
		return "", probeErr("malformed probe output: unsafe characters")
	}
	for _, c := range value {
		if c < 9 && c != '\r' {
			return "", probeErr("malformed probe output: unsafe characters")
		}
	}
	if secretRE.MatchString(value) {
		return "", probeErr("malformed probe output: sensitive value")
	}
	return value, nil
}

func identityProbe(ctx context.Context, r Runner, limit int, timeout time.Duration) (map[string]any, error) {
	hostnameRaw, err := output(ctx, r, []string{"hostname"}, limit, timeout)
	if err != nil {
		return nil, err
	}
	hostname := strings.TrimSpace(hostnameRaw)
	if !hostnameRE.MatchString(hostname) {
		return nil, probeErr("malformed identity output")
	}
	release, err := output(ctx, r, []string{"cat", "/etc/os-release"}, limit, timeout)
	if err != nil {
		return nil, err
	}
	fields := map[string]string{}
	for _, line := range strings.Split(release, "\n") {
		if line == "" || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		key := parts[0]
		val := parts[1]
		if !keyRE.MatchString(key) {
			return nil, probeErr("malformed identity output")
		}
		fields[key] = strings.Trim(strings.TrimSpace(val), `"`)
	}
	for _, k := range []string{"NAME", "VERSION_ID", "PRETTY_NAME"} {
		if fields[k] == "" {
			return nil, probeErr("malformed identity output")
		}
	}
	return map[string]any{"probe": "host.identity.inspect", "hostname": hostname, "os": map[string]string{"name": fields["NAME"], "version_id": fields["VERSION_ID"], "pretty_name": fields["PRETTY_NAME"]}}, nil
}

func graphicsProbe(ctx context.Context, r Runner, limit int, timeout time.Duration) (map[string]any, error) {
	pci, err := output(ctx, r, []string{"lspci", "-nnk"}, limit, timeout)
	if err != nil {
		return nil, err
	}
	drm, err := output(ctx, r, []string{"find", "/dev/dri", "-maxdepth", "1", "-type", "c", "-printf", "%f\\n"}, limit, timeout)
	if err != nil {
		return nil, err
	}
	allowedClasses := map[string]bool{"VGA compatible controller": true, "3D controller": true, "Display controller": true}
	var devices []map[string]any
	var current map[string]any
	for _, line := range strings.Split(pci, "\n") {
		// keep raw line for leading whitespace detection; stripped for matching
		stripped := strings.TrimSpace(line)
		if stripped == "" {
			continue
		}
		if pciLookingRE.MatchString(stripped) && !pciRE.MatchString(stripped) {
			return nil, probeErr("malformed graphics output")
		}
		m := pciRE.FindStringSubmatch(stripped)
		if m != nil {
			idxAddr := pciRE.SubexpIndex("address")
			idxClass := pciRE.SubexpIndex("class")
			idxDesc := pciRE.SubexpIndex("description")
			cls := strings.TrimSpace(m[idxClass])
			if !allowedClasses[cls] {
				current = nil
				continue
			}
			current = map[string]any{"address": m[idxAddr], "class": cls, "description": strings.TrimSpace(m[idxDesc]), "driver": nil}
			devices = append(devices, current)
			continue
		}
		if current != nil && (strings.HasPrefix(line, "\t") || strings.HasPrefix(line, " ")) && strings.HasPrefix(strings.TrimSpace(line), "Kernel driver in use:") {
			driver := strings.TrimSpace(strings.SplitN(strings.TrimSpace(line), ":", 2)[1])
			if !driverRE.MatchString(driver) {
				return nil, probeErr("malformed graphics output")
			}
			current["driver"] = driver
			continue
		}
		if stripped != "" {
			if !strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, " ") {
				return nil, probeErr("malformed graphics output")
			}
			if !allowedLine.MatchString(line) {
				return nil, probeErr("malformed graphics output")
			}
		}
	}
	var nodes []string
	for _, line := range strings.Split(drm, "\n") {
		n := strings.TrimSpace(line)
		if n == "" {
			continue
		}
		if !drmNodeRE.MatchString(n) {
			return nil, probeErr("malformed graphics output")
		}
		nodes = append(nodes, n)
	}
	if devices == nil {
		devices = []map[string]any{}
	}
	if nodes == nil {
		nodes = []string{}
	}
	return map[string]any{"probe": "host.graphics.inspect", "devices": devices, "drm_nodes": nodes}, nil
}

func statusCheck(ctx context.Context, r Runner, argv []string, limit int, timeout time.Duration, success map[int]bool, failure map[int]bool, legacy map[string]bool) (bool, error) {
	res, err := invoke(ctx, r, argv, timeout)
	if err != nil {
		return false, err
	}
	if res.Stdout == "" {
		if !success[res.Code] && !failure[res.Code] {
			return false, probeErr("malformed render access output")
		}
		return success[res.Code], nil
	}
	if res.Code != 0 {
		return false, probeErr("malformed render access output")
	}
	v := strings.TrimSpace(res.Stdout)
	if !legacy[v] {
		return false, probeErr("malformed render access output")
	}
	return v == "active" || v == "readable" || v == "writable", nil
}

func renderAccessProbe(ctx context.Context, r Runner, limit int, p Profile, timeout time.Duration) (map[string]any, error) {
	service := p.Service
	node := p.DRMNode
	active, err := statusCheck(ctx, r, []string{"systemctl", "is-active", "--quiet", service}, limit, timeout, map[int]bool{0: true}, map[int]bool{3: true}, map[string]bool{"active": true, "inactive": true})
	if err != nil {
		return nil, err
	}
	readable, err := statusCheck(ctx, r, []string{"test", "-r", node}, limit, timeout, map[int]bool{0: true}, map[int]bool{1: true}, map[string]bool{"readable": true, "not-readable": true})
	if err != nil {
		return nil, err
	}
	writable, err := statusCheck(ctx, r, []string{"test", "-w", node}, limit, timeout, map[int]bool{0: true}, map[int]bool{1: true}, map[string]bool{"writable": true, "not-writable": true})
	if err != nil {
		return nil, err
	}
	return map[string]any{"probe": "service.render_access.inspect", "service": service, "active": active, "node": node, "readable": readable, "writable": writable}, nil
}

// RunProbe dispatches named probes with bounds and profile validation.
func RunProbe(ctx context.Context, name string, r Runner, maxOutputBytes int, profile Profile) (map[string]any, error) {
	valid := map[string]bool{"host.identity.inspect": true, "host.graphics.inspect": true, "service.render_access.inspect": true}
	if !valid[name] {
		return nil, probeErr(fmt.Sprintf("unknown probe: %s", name))
	}
	if maxOutputBytes <= 0 {
		return nil, probeErr("invalid output bound")
	}
	if profile.Service == "" {
		profile.Service = "kvm-render"
	}
	if profile.DRMNode == "" {
		profile.DRMNode = "/dev/dri/renderD128"
	}
	if profile.Timeout <= 0 {
		profile.Timeout = 10 * time.Second
	}
	if !nameRE.MatchString(profile.Service) || !nodeRE.MatchString(profile.DRMNode) {
		return nil, probeErr("invalid host probe profile")
	}
	timeout := profile.Timeout
	switch name {
	case "host.identity.inspect":
		return identityProbe(ctx, r, maxOutputBytes, timeout)
	case "host.graphics.inspect":
		return graphicsProbe(ctx, r, maxOutputBytes, timeout)
	case "service.render_access.inspect":
		return renderAccessProbe(ctx, r, maxOutputBytes, profile, timeout)
	}
	return nil, probeErr("unknown probe")
}

// Probe is the legacy single-probe entry (identity) for backward compat.
func Probe(ctx context.Context, r Runner, p Profile) (map[string]any, error) {
	// default bound 65536 like Python
	return RunProbe(ctx, "host.identity.inspect", r, 65536, p)
}

// RebootConfirmation returns the hash bound to target+operation.
func RebootConfirmation(target string, operation string) string {
	if operation == "" {
		operation = "host.reboot"
	}
	m := map[string]string{"operation": operation, "target": target}
	b, _ := json.Marshal(m)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// Journal is the minimal journal interface.
type Journal interface {
	Append(map[string]any) error
}

// checkpointJournal is optionally implemented by journal.Journal.
type checkpointJournal interface {
	Checkpoint(operation, target, transition string, details map[string]any) error
}

// Adapter provides host operations with checkpoint/journal integration.
type Adapter struct {
	runner  Runner
	max     int
	journal Journal
	profile Profile
}

// NewAdapter creates a host adapter.
func NewAdapter(r Runner, maxOutputBytes int, j Journal, p Profile) *Adapter {
	if maxOutputBytes <= 0 {
		maxOutputBytes = 65536
	}
	if p.Service == "" {
		p.Service = "kvm-render"
	}
	if p.DRMNode == "" {
		p.DRMNode = "/dev/dri/renderD128"
	}
	if p.Timeout <= 0 {
		p.Timeout = 10 * time.Second
	}
	return &Adapter{runner: r, max: maxOutputBytes, journal: j, profile: p}
}

func (a *Adapter) checkpoint(transition, target string, details map[string]any) {
	if a.journal == nil {
		return
	}
	if cj, ok := a.journal.(checkpointJournal); ok {
		_ = cj.Checkpoint("host.reboot", target, transition, details)
		return
	}
	rec := map[string]any{"operation": "host.reboot", "target": target, "transition": transition}
	for k, v := range details {
		rec[k] = v
	}
	_ = a.journal.Append(rec)
}

// Identity runs the identity probe.
func (a *Adapter) Identity(ctx context.Context) (map[string]any, error) {
	return RunProbe(ctx, "host.identity.inspect", a.runner, a.max, a.profile)
}

// Reboot performs a confirmed reboot with polling.
func (a *Adapter) Reboot(ctx context.Context, target, confirmation string, writeEnabled bool) (results.Operation, error) {
	if !writeEnabled {
		return results.Operation{}, fmt.Errorf("policy refused: host.reboot requires write authorization")
	}
	if confirmation != RebootConfirmation(target, "host.reboot") {
		return results.Operation{}, fmt.Errorf("host.reboot requires explicit confirmation bound to target and operation")
	}
	preflight, err := a.Identity(ctx)
	if err != nil {
		return results.Operation{}, err
	}
	hostname, _ := preflight["hostname"].(string)
	a.checkpoint("preflight", target, map[string]any{"hostname": hostname})
	if hostname != target {
		return results.Build("host.reboot", "host", false, target, false, false, "mismatch", map[string]any{"preflight": map[string]any{"hostname": hostname}}, &results.Error{Code: "host_identity_mismatch", Retryable: false, RequiresHuman: true}), nil
	}
	// invoke reboot
	res, err := a.runner.Run(ctx, []string{"systemctl", "reboot"}, a.profile.Timeout)
	if err != nil {
		a.checkpoint("reboot_failed", target, nil)
		return results.Build("host.reboot", "host", false, target, false, false, "unknown", map[string]any{"preflight": map[string]any{"hostname": target}}, &results.Error{Code: "host_reboot_failed", Retryable: true}), nil
	}
	if res.Code != 0 {
		a.checkpoint("reboot_failed", target, map[string]any{"return_code": res.Code})
		return results.Build("host.reboot", "host", false, target, false, false, "unknown", map[string]any{"preflight": map[string]any{"hostname": target}}, &results.Error{Code: "host_reboot_failed", Retryable: true}), nil
	}
	a.checkpoint("reboot_requested", target, nil)

	const attempts = 5
	const delay = time.Second
	disappeared := false
	for i := 0; i < attempts; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return results.Operation{}, ctx.Err()
			case <-time.After(delay):
			}
		}
		ret, err := a.Identity(ctx)
		if err != nil {
			if !disappeared {
				disappeared = true
				a.checkpoint("disappeared", target, nil)
			}
			continue
		}
		if disappeared {
			rh, _ := ret["hostname"].(string)
			if rh != target {
				a.checkpoint("mismatch", target, map[string]any{"hostname": rh})
				return results.Build("host.reboot", "host", false, target, false, true, "mismatch", map[string]any{"preflight": map[string]any{"hostname": target}, "post_return": map[string]any{"hostname": rh}}, &results.Error{Code: "host_identity_mismatch", Retryable: false, RequiresHuman: true}), nil
			}
			a.checkpoint("ready", target, map[string]any{"hostname": target})
			return results.Build("host.reboot", "host", false, target, true, true, "ready", map[string]any{"preflight": map[string]any{"hostname": target}, "disappeared": true, "post_return": map[string]any{"hostname": target}}, nil), nil
		}
	}
	a.checkpoint("timeout", target, map[string]any{"disappeared": disappeared})
	return results.Build("host.reboot", "host", false, target, false, disappeared, "timeout", map[string]any{"preflight": map[string]any{"hostname": target}, "disappeared": disappeared}, &results.Error{Code: "host_reboot_timeout", Retryable: true}), nil
}
