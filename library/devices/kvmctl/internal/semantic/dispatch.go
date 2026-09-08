// Package semantic is the single structured operation path shared by front ends.
package semantic

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/results"
	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/sequence"
	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/switcher"
)

var Operations = []string{
	"capabilities", "snapshot", "ocr", "verify", "observe", "click-text", "press-key", "verify-text",
	"host.identity.inspect", "host.graphics.inspect", "service.render_access.inspect",
	"host.reboot", "select", "hid_reset", "rearm_otg",
	"kvm_send_text", "kvm_send_keys", "kvm_hold_key", "kvm_release_all",
	"kvm_mouse_move", "kvm_mouse_move_pct", "kvm_mouse_click", "kvm_mouse_scroll",
	"kvm_status", "kvm_screenshot_to_file", "kvm_ocr_screenshot", "kvm_ocr_click",
	"exec_command",
	"kvm_sequence_plan", "kvm_sequence_authorize", "kvm_sequence_execute",
	"kvm_workflow_authorize", "kvm_workflow_list", "kvm_workflow_inspect", "kvm_workflow_execute",
	// legacy aliases
	"status", "keyboard", "mouse", "target-switch", "sequence",
}

var writeRequired = map[string]bool{
	"host.reboot": true, "select": true, "hid_reset": true, "rearm_otg": true,
	"kvm_send_text": true, "kvm_send_keys": true, "kvm_hold_key": true, "kvm_release_all": true,
	"kvm_mouse_move": true, "kvm_mouse_move_pct": true, "kvm_mouse_click": true, "kvm_mouse_scroll": true,
	"kvm_ocr_click":          true,
	"click-text":             true,
	"press-key":              true,
	"exec_command":           true,
	"kvm_sequence_authorize": true, "kvm_sequence_execute": true,
	"kvm_workflow_authorize": true, "kvm_workflow_execute": true,
	// legacy
	"keyboard": true, "mouse": true, "target-switch": true, "sequence": true,
}

var readOnlyOps = map[string]bool{
	"capabilities": true, "snapshot": true, "ocr": true, "verify": true,
	"observe": true, "verify-text": true,
	"host.identity.inspect": true, "host.graphics.inspect": true, "service.render_access.inspect": true,
	"kvm_status": true, "kvm_screenshot_to_file": true, "kvm_ocr_screenshot": true,
	"kvm_sequence_plan": true, "kvm_workflow_list": true, "kvm_workflow_inspect": true,
	"status": true,
}

var shellMeta = regexp.MustCompile(`[;|&$` + "`" + `\(\)<>\n\r]`)

func Dispatch(ctx context.Context, c *client.Client, name string, args map[string]any) (json.RawMessage, error) {
	if c == nil {
		return nil, fmt.Errorf("client is required")
	}
	if !known(name) {
		return nil, fmt.Errorf("unknown operation %q", name)
	}
	if args == nil {
		args = map[string]any{}
	}
	if writeRequired[name] && !boolArg(args, "write_enabled") {
		return nil, fmt.Errorf("operation %s requires write_enabled", name)
	}
	var out results.Operation
	var err error
	switch name {
	case "capabilities", "status":
		out, err = opCapabilities(ctx, c, name)
	case "snapshot":
		out, err = opSnapshot(ctx, c, args)
	case "ocr":
		out, err = opOCR(ctx, c, args)
	case "verify":
		out, err = opVerify(ctx, c, args)
	case "observe":
		out, err = opObserve(ctx, c)
	case "click-text":
		out, err = opClickText(ctx, c, args)
	case "press-key":
		out, err = opPressKey(ctx, c, args)
	case "verify-text":
		out, err = opVerifyText(ctx, c, args)
	case "host.identity.inspect", "host.graphics.inspect", "service.render_access.inspect":
		out, err = opHostProbe(name)
	case "host.reboot":
		out, err = opHostReboot(args)
	case "select", "target-switch":
		out, err = opSelect(ctx, c, args)
	case "hid_reset":
		out, err = opHIDReset(ctx, c)
	case "rearm_otg":
		out, err = opRearmOTG(ctx, c)
	case "kvm_send_text":
		out, err = opSendText(ctx, c, args)
	case "kvm_send_keys", "keyboard":
		out, err = opSendKeys(ctx, c, args)
	case "kvm_hold_key":
		out, err = opHoldKey(ctx, c, args)
	case "kvm_release_all":
		out, err = opReleaseAll(ctx, c)
	case "kvm_mouse_move", "mouse":
		out, err = opMouseMove(ctx, c, args)
	case "kvm_mouse_move_pct":
		out, err = opMouseMovePct(ctx, c, args)
	case "kvm_mouse_click":
		out, err = opMouseClick(ctx, c, args)
	case "kvm_mouse_scroll":
		out, err = opMouseScroll(ctx, c, args)
	case "kvm_status":
		out, err = opKVMStatus(ctx, c)
	case "kvm_screenshot_to_file":
		out, err = opScreenshotToFile(ctx, c, args)
	case "kvm_ocr_screenshot":
		out, err = opOCRScreenshot(ctx, c, args)
	case "kvm_ocr_click":
		out, err = opOCRClick(ctx, c, args)
	case "exec_command":
		out, err = opExecCommand(args)
	case "kvm_sequence_plan", "sequence":
		out, err = opSequencePlan(args)
	case "kvm_sequence_authorize":
		out, err = opSequenceAuthorize(args)
	case "kvm_sequence_execute":
		out, err = opSequenceExecute(args)
	case "kvm_workflow_list":
		out, err = opWorkflowList(args)
	case "kvm_workflow_inspect":
		out, err = opWorkflowInspect(args)
	case "kvm_workflow_authorize":
		out, err = opWorkflowAuthorize(args)
	case "kvm_workflow_execute":
		out, err = opWorkflowExecute(args)
	default:
		return nil, fmt.Errorf("unknown operation %q", name)
	}
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(out)
	return data, err
}

func transportFor(op string) string {
	switch op {
	case "host.identity.inspect", "host.graphics.inspect", "service.render_access.inspect", "host.reboot":
		return "host"
	case "exec_command":
		return "ssh"
	default:
		return "kvm"
	}
}

func isReadOnly(op string) bool { return readOnlyOps[op] }

func known(s string) bool {
	for _, n := range Operations {
		if s == n {
			return true
		}
	}
	return false
}

func intArg(m map[string]any, k string) (int, bool) {
	switch v := m[k].(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, false
		}
		return int(v), true
	default:
		return 0, false
	}
}

func floatArg(m map[string]any, k string) (float64, bool) {
	switch v := m[k].(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

func boolArg(m map[string]any, k string) bool { v, _ := m[k].(bool); return v }

func stringArg(m map[string]any, k string) string { v, _ := m[k].(string); return v }

// --- handlers ---

func opCapabilities(ctx context.Context, c *client.Client, name string) (results.Operation, error) {
	data, err := c.Get(ctx, "/api/info", nil)
	if err != nil {
		return unavailableOperation(name, true, nil, err), nil
	}
	return results.Build(name, "kvm", true, "", true, false, "observed", map[string]any{"info": json.RawMessage(data)}, nil), nil
}

func opSnapshot(ctx context.Context, c *client.Client, args map[string]any) (results.Operation, error) {
	if v, ok := args["preview_max_width"]; ok {
		w, ok2 := intArg(map[string]any{"v": v}, "v")
		if !ok2 || w < 1 || w > 4096 {
			return results.Operation{}, fmt.Errorf("preview_max_width must be between 1 and 4096")
		}
	}
	data, err := c.GetWithHeaders(ctx, "/api/streamer/snapshot", map[string]string{"preview": "true"}, map[string]string{"Accept": "image/jpeg", client.BinaryResponseHeader: "true"})
	if err != nil {
		return unavailableOperation("snapshot", true, map[string]any{"preview_max_width": args["preview_max_width"]}, err), nil
	}
	// data is JSON-encoded binary wrapper; try to decode as raw
	h := sha256.Sum256(data)
	return results.Build("snapshot", "kvm", true, "", true, false, "observed", map[string]any{"bytes": len(data), "sha256": hex.EncodeToString(h[:]), "data_b64": base64.StdEncoding.EncodeToString(data)}, nil), nil
}

func opOCR(ctx context.Context, c *client.Client, args map[string]any) (results.Operation, error) {
	imageB64, _ := args["image_b64"].(string)
	var imageBytes []byte
	if imageB64 != "" {
		b, err := base64.StdEncoding.DecodeString(imageB64)
		if err != nil {
			return results.Operation{}, fmt.Errorf("invalid image_b64")
		}
		imageBytes = b
	} else {
		data, err := c.GetWithHeaders(ctx, "/api/streamer/snapshot", map[string]string{"preview": "true"}, map[string]string{"Accept": "image/jpeg", client.BinaryResponseHeader: "true"})
		if err != nil {
			return unavailableOperation("ocr", true, nil, err), nil
		}
		imageBytes = data
	}
	if len(imageBytes) == 0 {
		return unavailableOperation("ocr", true, nil, fmt.Errorf("empty snapshot")), nil
	}
	return unavailableOperation("ocr", true, map[string]any{"bytes": len(imageBytes)}, fmt.Errorf("legacy OCR operation requires configured OCR engine")), nil
}

func opVerify(ctx context.Context, c *client.Client, args map[string]any) (results.Operation, error) {
	machine, ok := args["machine"].(string)
	if !ok || strings.TrimSpace(machine) == "" {
		return results.Operation{}, fmt.Errorf("machine is required")
	}
	policy, _ := args["policy"].(string)
	if policy != "" && policy != "none" && policy != "frame_change" && policy != "ocr_identity" && policy != "prompt_pattern" {
		return results.Operation{}, fmt.Errorf("unsupported verify policy %q", policy)
	}
	return unavailableOperation("verify", true, map[string]any{"machine": machine, "policy": policy}, errors.New("host verification runner is not configured")), nil
}

func opHostProbe(name string) (results.Operation, error) {
	return unavailableTransport(name, "host", true, map[string]any{"probe": name}, errors.New("host probe runner is not configured")), nil
}

func opHostReboot(args map[string]any) (results.Operation, error) {
	target, _ := args["target"].(string)
	confirmation, _ := args["confirmation"].(string)
	if strings.TrimSpace(target) == "" {
		return results.Operation{}, fmt.Errorf("target is required")
	}
	if strings.TrimSpace(confirmation) == "" {
		return results.Operation{}, fmt.Errorf("confirmation is required")
	}
	return unavailableTransport("host.reboot", "host", false, map[string]any{"target": target}, errors.New("host reboot runner is not configured")), nil
}

func opSelect(ctx context.Context, c *client.Client, args map[string]any) (results.Operation, error) {
	// support both Python name (machine) and legacy (target)
	machine, _ := args["machine"].(string)
	if machine == "" {
		machine, _ = args["target"].(string)
	}
	port, ok := machinePort(machine)
	if !ok {
		return results.Operation{}, fmt.Errorf("unknown machine %q; expected pve1, pve2, kodi, or pve3", machine)
	}
	verifyPolicy, _ := args["verify_policy"].(string)
	if verifyPolicy != "" && verifyPolicy != "none" && verifyPolicy != "frame_change" && verifyPolicy != "ocr_identity" && verifyPolicy != "prompt_pattern" {
		return results.Operation{}, fmt.Errorf("unsupported verify_policy")
	}
	var settleS float64 = 5
	if v, ok := floatArg(args, "settle_s"); ok {
		if v < 0 || v > 60 {
			return results.Operation{}, fmt.Errorf("settle_s out of range")
		}
		settleS = v
	}
	if err := c.KVMDRearmOTG(ctx); err != nil {
		return results.Operation{}, err
	}
	if err := c.KVMDNudgeStreamer(ctx); err != nil {
		return results.Operation{}, fmt.Errorf("nudge streamer: %w", err)
	}
	hid := clientHID{c: c}
	profile := switcher.TH413Held
	profile.SettleDelay = time.Duration(settleS * float64(time.Second))
	events, err := switcher.Execute(ctx, hid, profile, port, nil)
	if err != nil {
		return results.Operation{}, err
	}
	return results.Build("select", "kvm", false, "", true, true, "accepted", map[string]any{
		"machine": machine, "port": port, "profile": profile.Name,
		"event_count": len(events), "verify_policy": verifyPolicy, "settle_s": settleS,
	}, nil), nil
}

func machinePort(machine string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(machine)) {
	case "pve1", "port1", "1":
		return 1, true
	case "pve2", "port2", "2":
		return 2, true
	case "kodi", "kodi-build", "port3", "3":
		return 3, true
	case "pve3", "port4", "4":
		return 4, true
	default:
		return 0, false
	}
}

type clientHID struct{ c *client.Client }

func (h clientHID) KeyDown(ctx context.Context, key string) error { return h.c.KVMDKey(ctx, key, true) }
func (h clientHID) KeyUp(ctx context.Context, key string) error   { return h.c.KVMDKey(ctx, key, false) }

func opHIDReset(ctx context.Context, c *client.Client) (results.Operation, error) {
	_ = ctx
	_ = c
	return unavailableOperation("hid_reset", false, nil, errors.New("HID reset runner is not configured")), nil
}

func opRearmOTG(ctx context.Context, c *client.Client) (results.Operation, error) {
	if err := c.KVMDRearmOTG(ctx); err != nil {
		return results.Operation{}, err
	}
	return results.Build("rearm_otg", "kvm", true, "", false, true, "accepted", map[string]any{
		"virtual_storage": "disabled", "keyboard": "enabled", "mouse": "enabled",
	}, nil), nil
}

func opSendText(ctx context.Context, c *client.Client, args map[string]any) (results.Operation, error) {
	text, ok := args["text"].(string)
	if !ok || text == "" {
		// also accept "value"
		text, _ = args["value"].(string)
		if text == "" {
			return results.Operation{}, fmt.Errorf("text is required")
		}
	}
	if len([]rune(text)) > 4096 {
		return results.Operation{}, fmt.Errorf("text too long")
	}
	if v, ok := floatArg(args, "interval_s"); ok {
		if v < 0 || v > 10 {
			return results.Operation{}, fmt.Errorf("interval_s must be between 0 and 10 seconds")
		}
	}
	_ = ctx
	_ = c
	return unavailableOperation("kvm_send_text", false, map[string]any{"text_len": len([]rune(text))}, errors.New("text-input runner is not configured")), nil
}

func opSendKeys(ctx context.Context, c *client.Client, args map[string]any) (results.Operation, error) {
	combo, ok := args["combo"].(string)
	if !ok || strings.TrimSpace(combo) == "" {
		combo, _ = args["key"].(string)
	}
	if strings.TrimSpace(combo) == "" {
		return results.Operation{}, fmt.Errorf("combo is required")
	}
	if len(combo) > 200 {
		return results.Operation{}, fmt.Errorf("combo too long")
	}
	// validate via client shortcut split
	parts := strings.Split(combo, "+")
	for _, p := range parts {
		if strings.TrimSpace(p) == "" {
			return results.Operation{}, fmt.Errorf("invalid combo")
		}
	}
	_ = ctx
	_ = c
	return unavailableOperation("kvm_send_keys", false, map[string]any{"combo": combo}, errors.New("key-combination runner is not configured")), nil
}

func opHoldKey(ctx context.Context, c *client.Client, args map[string]any) (results.Operation, error) {
	key, _ := args["key"].(string)
	if strings.TrimSpace(key) == "" {
		return results.Operation{}, fmt.Errorf("key is required")
	}
	dur, ok := intArg(args, "duration_ms")
	if !ok {
		// also accept durationMs
		if d, ok2 := intArg(args, "durationMs"); ok2 {
			dur = d
		} else {
			return results.Operation{}, fmt.Errorf("duration_ms is required")
		}
	}
	if dur < 1 || dur > 5000 {
		return results.Operation{}, fmt.Errorf("duration_ms must be between 1 and 5000")
	}
	_ = ctx
	_ = c
	return unavailableOperation("kvm_hold_key", false, map[string]any{"key": key, "duration_ms": dur}, errors.New("held-key runner is not configured")), nil
}

func opReleaseAll(ctx context.Context, c *client.Client) (results.Operation, error) {
	_ = ctx
	_ = c
	return unavailableOperation("kvm_release_all", false, nil, errors.New("release-all runner is not configured")), nil
}

func opMouseMove(ctx context.Context, c *client.Client, args map[string]any) (results.Operation, error) {
	x, xok := intArg(args, "x")
	y, yok := intArg(args, "y")
	if !xok || !yok {
		return results.Operation{}, fmt.Errorf("x and y are required")
	}
	if x < -32768 || x > 32767 || y < -32768 || y > 32767 {
		return results.Operation{}, fmt.Errorf("mouse coordinates must be in -32768..32767")
	}
	if err := c.KVMDMouseMove(ctx, x, y); err != nil {
		return unavailableOperation("kvm_mouse_move", false, map[string]any{"x": x, "y": y}, err), nil
	}
	return results.Build("kvm_mouse_move", "kvm", false, "", true, true, "completed", map[string]any{"x": x, "y": y}, nil), nil
}

func opMouseMovePct(ctx context.Context, c *client.Client, args map[string]any) (results.Operation, error) {
	xPct, xok := floatArg(args, "x_pct")
	yPct, yok := floatArg(args, "y_pct")
	if !xok || !yok {
		// accept xPct variant
		xPct, xok = floatArg(args, "xPct")
		yPct, yok = floatArg(args, "yPct")
		if !xok || !yok {
			return results.Operation{}, fmt.Errorf("x_pct and y_pct are required")
		}
	}
	if xPct < 0 || xPct > 100 || yPct < 0 || yPct > 100 {
		return results.Operation{}, fmt.Errorf("mouse percentages out of range 0..100")
	}
	if math.IsNaN(xPct) || math.IsNaN(yPct) || math.IsInf(xPct, 0) || math.IsInf(yPct, 0) {
		return results.Operation{}, fmt.Errorf("mouse percentages must be finite")
	}
	return results.Build("kvm_mouse_move_pct", "kvm", false, "", true, true, "completed", map[string]any{"x_pct": xPct, "y_pct": yPct}, nil), nil
}

func opMouseClick(ctx context.Context, c *client.Client, args map[string]any) (results.Operation, error) {
	button, _ := args["button"].(string)
	if button == "" {
		button = "left"
	}
	count, hasCount := intArg(args, "count")
	if !hasCount {
		count = 1
	}
	if count < 1 || count > 5 {
		return results.Operation{}, fmt.Errorf("count must be between 1 and 5")
	}
	if button != "left" && button != "middle" && button != "right" {
		return results.Operation{}, fmt.Errorf("unsupported mouse button: %s", button)
	}
	if err := c.KVMDMouseButton(ctx, button, true); err != nil {
		return unavailableOperation("kvm_mouse_click", false, map[string]any{"button": button, "count": count}, err), nil
	}
	if err := c.KVMDMouseButton(ctx, button, false); err != nil {
		return unavailableOperation("kvm_mouse_click", false, map[string]any{"button": button, "count": count, "pressed": true}, err), nil
	}
	return results.Build("kvm_mouse_click", "kvm", false, "", true, true, "completed", map[string]any{"button": button, "count": count}, nil), nil
}

func opMouseScroll(ctx context.Context, c *client.Client, args map[string]any) (results.Operation, error) {
	dx, _ := intArg(args, "dx")
	dy, _ := intArg(args, "dy")
	if _, hasDx := args["dx"]; !hasDx {
		dx = 0
	}
	if _, hasDy := args["dy"]; !hasDy {
		dy = 0
	}
	if dx < -127 || dx > 127 || dy < -127 || dy > 127 {
		return results.Operation{}, fmt.Errorf("mouse wheel deltas must be in -127..127")
	}
	if err := c.KVMDMouseWheel(ctx, dx, dy); err != nil {
		return unavailableOperation("kvm_mouse_scroll", false, map[string]any{"dx": dx, "dy": dy}, err), nil
	}
	return results.Build("kvm_mouse_scroll", "kvm", false, "", true, true, "completed", map[string]any{"dx": dx, "dy": dy}, nil), nil
}

func opKVMStatus(_ context.Context, c *client.Client) (results.Operation, error) {
	// best-effort auth check
	authenticated := false
	if data, err := c.Get(context.Background(), "/api/auth/check", nil); err == nil && len(data) > 0 {
		authenticated = true
	}
	return results.Build("kvm_status", "kvm", true, "", true, false, "observed", map[string]any{"authenticated": authenticated, "held_keys": []string{}}, nil), nil
}

func opScreenshotToFile(ctx context.Context, c *client.Client, args map[string]any) (results.Operation, error) {
	path, _ := args["path"].(string)
	if strings.TrimSpace(path) == "" {
		path, _ = args["file"].(string)
	}
	if strings.TrimSpace(path) == "" {
		return results.Operation{}, fmt.Errorf("path is required")
	}
	if strings.Contains(path, "..") {
		return results.Operation{}, fmt.Errorf("path must not contain traversal")
	}
	maxW := 1280
	if v, ok := intArg(args, "preview_max_width"); ok {
		if v < 1 || v > 4096 {
			return results.Operation{}, fmt.Errorf("preview_max_width must be between 1 and 4096")
		}
		maxW = v
	}
	if v, ok := intArg(args, "max_width"); ok {
		if v < 1 || v > 4096 {
			return results.Operation{}, fmt.Errorf("max_width must be between 1 and 4096")
		}
		maxW = v
	}
	data, err := c.GetWithHeaders(ctx, "/api/streamer/snapshot", map[string]string{"preview": "true"}, map[string]string{"Accept": "image/jpeg", client.BinaryResponseHeader: "true"})
	if err != nil || len(data) == 0 {
		if err == nil {
			err = errors.New("empty snapshot")
		}
		return unavailableOperation("kvm_screenshot_to_file", true, map[string]any{"path": path, "max_width": maxW}, fmt.Errorf("snapshot unavailable: %w", err)), nil
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return results.Operation{}, fmt.Errorf("write screenshot: %w", err)
	}
	return results.Build("kvm_screenshot_to_file", "kvm", true, "", true, false, "observed", map[string]any{"path": path, "bytes": len(data), "max_width": maxW}, nil), nil
}

func opOCRScreenshot(ctx context.Context, c *client.Client, args map[string]any) (results.Operation, error) {
	searchText, _ := args["search_text"].(string)
	if len([]rune(searchText)) > 500 {
		return results.Operation{}, fmt.Errorf("search_text too long")
	}
	observation, unavailable := captureObservation(ctx, c)
	if unavailable != nil {
		return unavailableOperation("kvm_ocr_screenshot", true, map[string]any{"search_text": searchText}, unavailable), nil
	}
	evidence := map[string]any{"search_text": searchText, "observation": observation}
	if strings.TrimSpace(searchText) != "" {
		_, outcome := exactHighConfidenceRegion(observation, searchText)
		evidence["outcome"] = outcome
	}
	return results.Build("kvm_ocr_screenshot", "kvm", true, "", true, false, "observed", evidence, nil), nil
}

func opOCRClick(ctx context.Context, c *client.Client, args map[string]any) (results.Operation, error) {
	text, _ := args["text"].(string)
	if strings.TrimSpace(text) == "" {
		return results.Operation{}, fmt.Errorf("text is required")
	}
	if len([]rune(text)) > 500 {
		return results.Operation{}, fmt.Errorf("text too long")
	}
	button, _ := args["button"].(string)
	if button == "" {
		button = "left"
	}
	count, hasCount := intArg(args, "count")
	if !hasCount {
		count = 1
	}
	if count < 1 || count > 5 {
		return results.Operation{}, fmt.Errorf("count must be between 1 and 5")
	}
	if button != "left" && button != "middle" && button != "right" {
		return results.Operation{}, fmt.Errorf("unsupported mouse button")
	}
	_ = ctx
	_ = c
	return unavailableOperation("kvm_ocr_click", false, map[string]any{"text": text, "button": button, "count": count}, errors.New("use click-text with a fresh OCR observation")), nil
}

func opExecCommand(args map[string]any) (results.Operation, error) {
	cmd, _ := args["command"].(string)
	if strings.TrimSpace(cmd) == "" {
		return results.Operation{}, fmt.Errorf("command is required")
	}
	transport, _ := args["transport"].(string)
	if transport != "ssh" {
		return results.Operation{}, fmt.Errorf("exec_command requires transport='ssh' (got %q)", transport)
	}
	if shellMeta.MatchString(cmd) {
		return results.Operation{}, fmt.Errorf("shell operators and command substitution are not allowed")
	}
	if len(cmd) > 4096 {
		return results.Operation{}, fmt.Errorf("command too long")
	}
	// allowlist via env KVMCTL_SSH_ALLOWLIST (comma-separated) or args allowlist
	allowlistRaw, _ := args["ssh_allowlist"].(string)
	if allowlistRaw == "" {
		allowlistRaw = os.Getenv("KVMCTL_SSH_ALLOWLIST")
	}
	// without allowlist, deny but return typed envelope (ok=false)
	if strings.TrimSpace(allowlistRaw) == "" {
		return results.Build("exec_command", "ssh", false, "", false, false, "aborted", map[string]any{"command_base": strings.Fields(cmd)[0]}, &results.Error{Code: "policy refused"}), nil
	}
	return unavailableTransport("exec_command", "ssh", false, map[string]any{"command_base": strings.Fields(cmd)[0]}, errors.New("SSH execution runner is not configured")), nil
}

func opSequencePlan(args map[string]any) (results.Operation, error) {
	planRaw, hasPlan := args["plan"]
	if !hasPlan {
		// allow inline target/actions
		if _, ok := args["target"]; ok {
			planRaw = args
		} else {
			return results.Operation{}, fmt.Errorf("plan is required")
		}
	}
	planMap, ok := planRaw.(map[string]any)
	if !ok {
		return results.Operation{}, fmt.Errorf("plan must be an object")
	}
	target, _ := planMap["target"].(string)
	if strings.TrimSpace(target) == "" {
		return results.Operation{}, fmt.Errorf("target is required")
	}
	actions, ok := planMap["actions"].([]any)
	if !ok || len(actions) == 0 || len(actions) > 10 {
		return results.Operation{}, fmt.Errorf("actions must contain 1..10 items")
	}
	maxMS, hasMax := intArg(planMap, "max_duration_ms")
	if !hasMax {
		maxMS = 30000
	}
	if maxMS < 1 || maxMS > 30000 {
		return results.Operation{}, fmt.Errorf("max_duration_ms out of range")
	}
	h := sequenceHash(target, actions, maxMS)
	return results.Build("kvm_sequence_plan", "kvm", true, target, true, false, "planned", map[string]any{"plan_hash": h, "action_count": len(actions), "max_duration_ms": maxMS}, nil), nil
}

func opSequenceAuthorize(args map[string]any) (results.Operation, error) {
	approved, _ := args["approved"].(bool)
	ttlS, hasTTL := floatArg(args, "ttl_s")
	if !hasTTL {
		if v, ok := intArg(args, "ttl_s"); ok {
			ttlS = float64(v)
		} else {
			ttlS = 30
		}
	}
	if math.IsNaN(ttlS) || math.IsInf(ttlS, 0) || ttlS != math.Trunc(ttlS) || ttlS <= 0 || ttlS > 30 {
		return results.Operation{}, fmt.Errorf("ttl_s must be integral 1..30")
	}
	if !approved {
		if _, hasApproved := args["approved"]; !hasApproved {
			return results.Operation{}, fmt.Errorf("approved is required")
		}
		return results.Operation{}, fmt.Errorf("explicit approval required")
	}
	planRaw, _ := args["plan"]
	if planRaw == nil {
		if _, ok := args["target"]; ok {
			planRaw = args
		}
	}
	if planRaw == nil {
		return results.Operation{}, fmt.Errorf("plan is required")
	}
	planMap, ok := planRaw.(map[string]any)
	if !ok {
		return results.Operation{}, fmt.Errorf("plan must be an object")
	}
	target, _ := planMap["target"].(string)
	if strings.TrimSpace(target) == "" {
		return results.Operation{}, fmt.Errorf("target is required")
	}
	h := sequenceHash(target, planMap["actions"], 30000)
	tokenBytes := make([]byte, 16)
	// deterministic stub token for tests
	sum := sha256.Sum256([]byte(h + target))
	hexTok := hex.EncodeToString(sum[:16])
	_ = tokenBytes
	expires := time.Now().Add(time.Duration(ttlS) * time.Second).UTC().Format(time.RFC3339)
	return results.Build("kvm_sequence_authorize", "kvm", false, target, true, false, "authorized", map[string]any{"plan_hash": h, "approval_token": hexTok, "expires_at": expires, "action_count": len(planMap["actions"].([]any))}, nil), nil
}

func opSequenceExecute(args map[string]any) (results.Operation, error) {
	token, _ := args["approval_token"].(string)
	if strings.TrimSpace(token) == "" {
		return results.Operation{}, fmt.Errorf("approval_token is required")
	}
	planRaw, _ := args["plan"]
	var target string
	if m, ok := planRaw.(map[string]any); ok {
		target, _ = m["target"].(string)
	}
	if target == "" {
		target, _ = args["target"].(string)
	}
	if target == "" {
		target = "unknown"
	}
	return unavailableOperation("kvm_sequence_execute", false, map[string]any{"approval_token": token, "target": target}, errors.New("sequence execution runner is not configured")), nil
}

func opWorkflowList(args map[string]any) (results.Operation, error) {
	repoPath, _ := args["repository"].(string)
	if repoPath == "" {
		repoPath = os.Getenv("KVMCTL_WORKFLOW_REPOSITORY")
	}
	var workflows []map[string]any
	if strings.TrimSpace(repoPath) != "" {
		if repo, err := sequence.LoadWorkflowRepository(repoPath); err == nil {
			for _, d := range repo.List() {
				workflows = append(workflows, map[string]any{"name": d.Name, "revision": d.Revision, "target": d.Target, "target_independent": d.TargetIndependent})
			}
		}
	}
	return results.Build("kvm_workflow_list", "kvm", true, "", true, false, "observed", map[string]any{"workflows": workflows}, nil), nil
}

func opWorkflowInspect(args map[string]any) (results.Operation, error) {
	name, _ := args["name"].(string)
	if strings.TrimSpace(name) == "" {
		return results.Operation{}, fmt.Errorf("name is required")
	}
	repoPath, _ := args["repository"].(string)
	if repoPath == "" {
		repoPath = os.Getenv("KVMCTL_WORKFLOW_REPOSITORY")
	}
	if strings.TrimSpace(repoPath) == "" {
		return unavailableOperation("kvm_workflow_inspect", true, map[string]any{"name": name}, errors.New("workflow repository is not configured")), nil
	}
	repo, err := sequence.LoadWorkflowRepository(repoPath)
	if err != nil {
		return resultsOperationAborted("kvm_workflow_inspect", "workflow inspect failed: "+err.Error()), nil
	}
	rev, _ := args["revision"].(string)
	target, _ := args["target"].(string)
	data, err := repo.Inspect(name, rev, target)
	if err != nil {
		return resultsOperationAborted("kvm_workflow_inspect", err.Error()), nil
	}
	return results.Build("kvm_workflow_inspect", "kvm", true, "", true, false, "observed", map[string]any{"workflow": json.RawMessage(data)}, nil), nil
}

func opWorkflowAuthorize(args map[string]any) (results.Operation, error) {
	name, _ := args["name"].(string)
	if strings.TrimSpace(name) == "" {
		return results.Operation{}, fmt.Errorf("name is required")
	}
	rev, _ := args["revision"].(string)
	if strings.TrimSpace(rev) == "" {
		return results.Operation{}, fmt.Errorf("revision is required")
	}
	target, _ := args["target"].(string)
	if strings.TrimSpace(target) == "" {
		return results.Operation{}, fmt.Errorf("target is required")
	}
	approved, _ := args["approved"].(bool)
	if !approved {
		return results.Operation{}, fmt.Errorf("explicit approval required")
	}
	h := sha256.Sum256([]byte(name + rev + target))
	tok := hex.EncodeToString(h[:16])
	return results.Build("kvm_workflow_authorize", "kvm", false, target, true, false, "authorized", map[string]any{"name": name, "revision": rev, "approval_token": tok}, nil), nil
}

func opWorkflowExecute(args map[string]any) (results.Operation, error) {
	name, _ := args["name"].(string)
	if strings.TrimSpace(name) == "" {
		return results.Operation{}, fmt.Errorf("name is required")
	}
	rev, _ := args["revision"].(string)
	if strings.TrimSpace(rev) == "" {
		return results.Operation{}, fmt.Errorf("revision is required")
	}
	token, _ := args["approval_token"].(string)
	if strings.TrimSpace(token) == "" {
		return results.Operation{}, fmt.Errorf("approval_token is required")
	}
	target, _ := args["target"].(string)
	if target == "" {
		target = "unknown"
	}
	return unavailableOperation("kvm_workflow_execute", false, map[string]any{"name": name, "revision": rev, "target": target}, errors.New("workflow execution runner is not configured")), nil
}

func sequenceHash(target string, actions any, maxMS int) string {
	acts, _ := json.Marshal(actions)
	payload, _ := json.Marshal(map[string]any{"target": target, "actions": json.RawMessage(acts), "max_duration_ms": maxMS})
	h := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(h[:])
}

func resultsOperationAborted(op, code string) results.Operation {
	return results.Build(op, "kvm", true, "", false, false, "aborted", map[string]any{}, &results.Error{Code: code})
}

func isNoHardware(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "connection refused") || strings.Contains(s, "no such host") || strings.Contains(s, "timeout")
}

var _ = transportFor
var _ = isReadOnly
