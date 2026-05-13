package omnilogic

import (
	"fmt"
	"strconv"
	"strings"
)

// GetSiteList lists every site (backyard) registered to the account.
func (c *Client) GetSiteList() ([]Site, error) {
	if err := c.EnsureToken(); err != nil {
		return nil, err
	}
	state := c.AuthState()
	params := map[string]any{"Token": state.Token, "UserID": state.UserID}
	body, err := c.callOp("GetSiteList", params)
	if err != nil {
		return nil, err
	}
	if strings.Contains(body, "You don't have permission") || strings.Contains(body, "The message format is wrong") {
		return nil, fmt.Errorf("GetSiteList rejected: %s", truncate(body, 120))
	}
	return parseSiteList(body)
}

// GetMspConfig fetches the equipment inventory tree (XML) for one site.
func (c *Client) GetMspConfig(mspSystemID int) (*MspConfig, error) {
	if err := c.EnsureToken(); err != nil {
		return nil, err
	}
	state := c.AuthState()
	params := map[string]any{"Token": state.Token, "MspSystemID": mspSystemID, "Version": 0}
	body, err := c.callOp("GetMspConfigFile", params)
	if err != nil {
		return nil, err
	}
	cfg, err := parseMspConfig(body)
	if err != nil {
		return nil, err
	}
	cfg.MspSystemID = mspSystemID
	cfg.RawXML = body
	return cfg, nil
}

// GetAlarmList fetches the current alarm set for one site.
func (c *Client) GetAlarmList(mspSystemID int) ([]Alarm, error) {
	if err := c.EnsureToken(); err != nil {
		return nil, err
	}
	state := c.AuthState()
	params := map[string]any{"Token": state.Token, "MspSystemID": mspSystemID, "Version": "0"}
	body, err := c.callOp("GetAlarmList", params)
	if err != nil {
		return nil, err
	}
	return parseAlarmList(body), nil
}

// GetTelemetry fetches the current state snapshot for one site.
func (c *Client) GetTelemetry(mspSystemID int) (*Telemetry, error) {
	if err := c.EnsureToken(); err != nil {
		return nil, err
	}
	state := c.AuthState()
	params := map[string]any{"Token": state.Token, "MspSystemID": mspSystemID}
	body, err := c.callOp("GetTelemetryData", params)
	if err != nil {
		return nil, err
	}
	t, err := parseTelemetry(body)
	if err != nil {
		return nil, err
	}
	t.MspSystemID = mspSystemID
	t.RawXML = body
	return t, nil
}

// SetHeaterEnable turns a heater on (enable=true) or off (enable=false).
// Order matches the Python reference wrapper's dict insertion order.
func (c *Client) SetHeaterEnable(mspSystemID, poolID, heaterID int, enable bool) (*CommandResult, error) {
	ordered := []orderedParam{
		{"MspSystemID", "int", mspSystemID},
		{"Version", "string", "0"},
		{"PoolID", "int", poolID},
		{"HeaterID", "int", heaterID},
		{"Enabled", "bool", enable},
	}
	return c.runSetOpOrdered("SetHeaterEnable", ordered, mspSystemID, fmt.Sprintf("heater %d", heaterID))
}

// SetHeaterTemp sets a heater setpoint in °F.
func (c *Client) SetHeaterTemp(mspSystemID, poolID, heaterID, tempF int) (*CommandResult, error) {
	ordered := []orderedParam{
		{"MspSystemID", "int", mspSystemID},
		{"Version", "string", "0"},
		{"PoolID", "int", poolID},
		{"HeaterID", "int", heaterID},
		{"Temp", "int", tempF},
	}
	return c.runSetOpOrdered("SetUIHeaterCmd", ordered, mspSystemID, fmt.Sprintf("heater %d", heaterID))
}

// SetPumpSpeed sets a VSP pump's running speed (0-100%). Speed=0 stops the pump.
//
// Hayward overloads the IsOn parameter on SetUIEquipmentCmd: when sent with
// dataType="int" and a value 0-100, it sets the VSP's running speed; when
// sent with dataType="bool" (True/False) it's a simple on/off toggle for
// non-variable equipment. There is NO separate "Speed" param — verified
// against the canonical Python wrapper (djtimca/omnilogic-api 0.6.1) via
// live packet capture against the real .ashx endpoint. Sending Speed as a
// separate param triggers "Input string was not in a correct format".
//
// Field order matches the Python wrapper's dict insertion order verbatim,
// because Hayward's .NET handler is also order-sensitive.
// PATCH (fix-vsp-pump-ison-type-overload): send IsOn as dataType=int (0-100 speed) — Hayward overloads IsOn on SetUIEquipmentCmd, there is no separate Speed param. Field order matches the Python wrapper insertion order because the .NET handler is order-sensitive.
func (c *Client) SetPumpSpeed(mspSystemID, poolID, pumpID, speed int) (*CommandResult, error) {
	ordered := []orderedParam{
		{"MspSystemID", "int", mspSystemID},
		{"Version", "string", "0"},
		{"PoolID", "int", poolID},
		{"EquipmentID", "int", pumpID},
		{"IsOn", "int", speed},
		{"IsCountDownTimer", "bool", false},
		{"StartTimeHours", "int", 0},
		{"StartTimeMinutes", "int", 0},
		{"EndTimeHours", "int", 0},
		{"EndTimeMinutes", "int", 0},
		{"DaysActive", "int", 0},
		{"Recurring", "bool", false},
	}
	return c.runSetOpOrdered("SetUIEquipmentCmd", ordered, mspSystemID, fmt.Sprintf("pump %d", pumpID))
}

// SetEquipment turns a non-variable-speed equipment item on/off, optionally
// for a bounded duration. For VSP pumps, callers must use SetPumpSpeed
// instead — Hayward overloads the IsOn parameter and an IsOn=bool against
// a VSP pump returns "Input string was not in a correct format".
//
// durationMinutes=0 means run indefinitely; >0 schedules a countdown timer.
// Order matches Hayward's "you should input following parameters" list.
func (c *Client) SetEquipment(mspSystemID, poolID, equipmentID int, isOn bool, durationMinutes int) (*CommandResult, error) {
	ordered := []orderedParam{
		{"MspSystemID", "int", mspSystemID},
		{"Version", "string", "0"},
		{"PoolID", "int", poolID},
		{"EquipmentID", "int", equipmentID},
		{"IsOn", "bool", isOn},
		{"IsCountDownTimer", "bool", durationMinutes > 0},
		{"StartTimeHours", "int", 0},
		{"StartTimeMinutes", "int", 0},
		{"EndTimeHours", "int", durationMinutes / 60},
		{"EndTimeMinutes", "int", durationMinutes % 60},
		{"DaysActive", "int", 0},
		{"Recurring", "bool", false},
	}
	return c.runSetOpOrdered("SetUIEquipmentCmd", ordered, mspSystemID, fmt.Sprintf("equipment %d", equipmentID))
}

// DefaultVSPOnSpeed is the speed used when `equipment on` is invoked against
// a VSP pump without an explicit --speed. 100% (max) is the safe pick: it
// matches "this pump should be running" intent without making the agent
// guess at a calibration value. Users can call `pump set-speed --speed N`
// for a specific RPM/%.
const DefaultVSPOnSpeed = 100

// SetSpillover sets the spillover speed.
func (c *Client) SetSpillover(mspSystemID, poolID, speed, durationMinutes int) (*CommandResult, error) {
	ordered := []orderedParam{
		{"MspSystemID", "int", mspSystemID},
		{"Version", "string", "0"},
		{"PoolID", "int", poolID},
		{"Speed", "int", speed},
		{"IsCountDownTimer", "bool", durationMinutes > 0},
		{"StartTimeHours", "int", 0},
		{"StartTimeMinutes", "int", 0},
		{"EndTimeHours", "int", durationMinutes / 60},
		{"EndTimeMinutes", "int", durationMinutes % 60},
		{"DaysActive", "int", 0},
		{"Recurring", "bool", false},
	}
	return c.runSetOpOrdered("SetUISpilloverCmd", ordered, mspSystemID, fmt.Sprintf("pool %d", poolID))
}

// SetSuperchlor toggles superchlorination on a salt chlorinator.
func (c *Client) SetSuperchlor(mspSystemID, poolID, chlorID int, isOn bool) (*CommandResult, error) {
	ordered := []orderedParam{
		{"MspSystemID", "int", mspSystemID},
		{"Version", "string", "0"},
		{"PoolID", "int", poolID},
		{"ChlorID", "int", chlorID},
		{"IsOn", "bool", isOn},
	}
	return c.runSetOpOrdered("SetUISuperCHLORCmd", ordered, mspSystemID, fmt.Sprintf("chlor %d", chlorID))
}

// SetLightShow sets a ColorLogic light show (V1).
func (c *Client) SetLightShow(mspSystemID, poolID, lightID, showID int) (*CommandResult, error) {
	ordered := []orderedParam{
		{"MspSystemID", "int", mspSystemID},
		{"Version", "string", "0"},
		{"PoolID", "int", poolID},
		{"LightID", "int", lightID},
		{"Show", "int", showID},
		{"IsCountDownTimer", "bool", false},
		{"StartTimeHours", "int", 0},
		{"StartTimeMinutes", "int", 0},
		{"EndTimeHours", "int", 0},
		{"EndTimeMinutes", "int", 0},
		{"DaysActive", "int", 0},
		{"Recurring", "bool", false},
	}
	return c.runSetOpOrdered("SetStandAloneLightShow", ordered, mspSystemID, fmt.Sprintf("light %d", lightID))
}

// SetLightShowV2 sets a ColorLogic light show with speed + brightness (V2 lights only).
func (c *Client) SetLightShowV2(mspSystemID, poolID, lightID, showID, speed, brightness int) (*CommandResult, error) {
	ordered := []orderedParam{
		{"MspSystemID", "int", mspSystemID},
		{"Version", "string", "0"},
		{"PoolID", "int", poolID},
		{"LightID", "int", lightID},
		{"Show", "int", showID},
		{"Speed", "int", speed},
		{"Brightness", "int", brightness},
		{"IsCountDownTimer", "bool", false},
		{"StartTimeHours", "int", 0},
		{"StartTimeMinutes", "int", 0},
		{"EndTimeHours", "int", 0},
		{"EndTimeMinutes", "int", 0},
		{"DaysActive", "int", 0},
		{"Recurring", "bool", false},
	}
	return c.runSetOpOrdered("SetStandAloneLightShowV2", ordered, mspSystemID, fmt.Sprintf("light %d", lightID))
}

// SetChlorParams writes chlorinator configuration. Pass nil for any field to
// keep its existing value (caller is responsible for reading current values
// from MSP config and supplying them — there's no merge in the client).
type ChlorParams struct {
	MspSystemID  int
	PoolID       int
	ChlorID      int
	CfgState     *int
	OpMode       *int
	BOWType      *int
	CellType     *int
	TimedPercent *int
	SCTimeout    *int
	ORPTimeout   *int
}

func (c *Client) SetChlorParams(p ChlorParams) (*CommandResult, error) {
	params := map[string]any{
		"MspSystemID": p.MspSystemID,
		"PoolID":      p.PoolID,
		"ChlorID":     p.ChlorID,
	}
	if p.CfgState != nil {
		params["CfgState"] = *p.CfgState
	}
	if p.OpMode != nil {
		params["OpMode"] = *p.OpMode
	}
	if p.BOWType != nil {
		params["BOWType"] = *p.BOWType
	}
	if p.CellType != nil {
		params["CellType"] = *p.CellType
	}
	if p.TimedPercent != nil {
		params["TimedPercent"] = *p.TimedPercent
	}
	if p.SCTimeout != nil {
		params["SCTimeout"] = *p.SCTimeout
	}
	if p.ORPTimeout != nil {
		params["ORPTimout"] = *p.ORPTimeout // Hayward typo preserved
	}
	return c.runSetOp("SetCHLORParams", params, fmt.Sprintf("chlor %d", p.ChlorID))
}

// runSetOpOrdered is the ordered-params variant of runSetOp. Use this for
// Set* operations against the .ashx endpoint where Hayward's .NET handler
// requires a specific parameter order (SetUIEquipmentCmd, SetUIHeaterCmd,
// SetUISpilloverCmd, SetStandAloneLightShow, SetUISuperCHLORCmd, etc.).
func (c *Client) runSetOpOrdered(op string, ordered []orderedParam, mspSystemID int, target string) (*CommandResult, error) {
	if err := c.EnsureToken(); err != nil {
		return nil, err
	}
	body, err := c.callOpOrdered(op, ordered, mspSystemID)
	if err != nil {
		return nil, err
	}
	return interpretSetResponse(body, op, target), nil
}

// runSetOp shares the boilerplate for Set* operations: token, call, parse
// Status, wrap as CommandResult.
func (c *Client) runSetOp(op string, params map[string]any, target string) (*CommandResult, error) {
	if err := c.EnsureToken(); err != nil {
		return nil, err
	}
	body, err := c.callOp(op, params)
	if err != nil {
		return nil, err
	}
	return interpretSetResponse(body, op, target), nil
}

// interpretSetResponse parses a Set* XML response into a CommandResult.
// Status == 0 means success; any other status surfaces the StatusMessage
// (or a generic detail when Hayward omits one).
func interpretSetResponse(body, op, target string) *CommandResult {
	status, msg, hasStatus := statusFromResponse(body)
	result := &CommandResult{Operation: op, Target: target}
	if hasStatus {
		result.StatusCode = status
		if status == 0 {
			result.Status = "ok"
		} else {
			result.Status = "error"
			result.Detail = msg
			if result.Detail == "" {
				result.Detail = "non-zero Status from Hayward"
			}
		}
	} else {
		// No Status param — treat as success (mirrors the Python wrapper).
		result.Status = "ok"
	}
	return result
}

// ParseDuration accepts strings like "30m", "1h", "2h30m" and returns whole
// minutes. Empty string returns 0 (run indefinitely).
func ParseDuration(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n, nil
	}
	var minutes int
	var num strings.Builder
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			num.WriteRune(r)
		case r == 'h':
			if num.Len() == 0 {
				return 0, fmt.Errorf("invalid duration %q", s)
			}
			n, _ := strconv.Atoi(num.String())
			minutes += n * 60
			num.Reset()
		case r == 'm':
			if num.Len() == 0 {
				return 0, fmt.Errorf("invalid duration %q", s)
			}
			n, _ := strconv.Atoi(num.String())
			minutes += n
			num.Reset()
		case r == ' ':
			// allowed separator
		default:
			return 0, fmt.Errorf("invalid duration %q", s)
		}
	}
	if num.Len() > 0 {
		// trailing number with no unit -> treat as minutes
		n, _ := strconv.Atoi(num.String())
		minutes += n
	}
	return minutes, nil
}
