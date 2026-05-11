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
func (c *Client) SetHeaterEnable(mspSystemID, poolID, heaterID int, enable bool) (*CommandResult, error) {
	params := map[string]any{
		"MspSystemID": mspSystemID,
		"PoolID":      poolID,
		"HeaterID":    heaterID,
		"Version":     "0",
		"Enabled":     enable,
	}
	return c.runSetOp("SetHeaterEnable", params, fmt.Sprintf("heater %d", heaterID))
}

// SetHeaterTemp sets a heater setpoint in °F.
func (c *Client) SetHeaterTemp(mspSystemID, poolID, heaterID, tempF int) (*CommandResult, error) {
	params := map[string]any{
		"MspSystemID": mspSystemID,
		"PoolID":      poolID,
		"HeaterID":    heaterID,
		"Version":     "0",
		"Temp":        tempF,
	}
	return c.runSetOp("SetUIHeaterCmd", params, fmt.Sprintf("heater %d", heaterID))
}

// SetPumpSpeed sets a pump's running speed.
func (c *Client) SetPumpSpeed(mspSystemID, poolID, pumpID, speed int) (*CommandResult, error) {
	params := map[string]any{
		"MspSystemID":      mspSystemID,
		"PoolID":           poolID,
		"EquipmentID":      pumpID,
		"Version":          "0",
		"Speed":            speed,
		"IsCountDownTimer": false,
		"StartTimeHours":   0,
		"StartTimeMinutes": 0,
		"EndTimeHours":     0,
		"EndTimeMinutes":   0,
		"DaysActive":       0,
		"Recurring":        false,
	}
	return c.runSetOp("SetUIEquipmentCmd", params, fmt.Sprintf("pump %d", pumpID))
}

// SetEquipment turns an equipment item on/off, optionally for a bounded
// duration. dur of 0 means run indefinitely.
func (c *Client) SetEquipment(mspSystemID, poolID, equipmentID int, isOn bool, durationMinutes int) (*CommandResult, error) {
	params := map[string]any{
		"MspSystemID":      mspSystemID,
		"PoolID":           poolID,
		"EquipmentID":      equipmentID,
		"Version":          "0",
		"IsOn":             isOn,
		"IsCountDownTimer": durationMinutes > 0,
		"StartTimeHours":   0,
		"StartTimeMinutes": 0,
		"EndTimeHours":     durationMinutes / 60,
		"EndTimeMinutes":   durationMinutes % 60,
		"DaysActive":       0,
		"Recurring":        false,
	}
	return c.runSetOp("SetUIEquipmentCmd", params, fmt.Sprintf("equipment %d", equipmentID))
}

// SetSpillover sets the spillover speed.
func (c *Client) SetSpillover(mspSystemID, poolID, speed, durationMinutes int) (*CommandResult, error) {
	params := map[string]any{
		"MspSystemID":      mspSystemID,
		"PoolID":           poolID,
		"Version":          "0",
		"Speed":            speed,
		"IsCountDownTimer": durationMinutes > 0,
		"StartTimeHours":   0,
		"StartTimeMinutes": 0,
		"EndTimeHours":     durationMinutes / 60,
		"EndTimeMinutes":   durationMinutes % 60,
		"DaysActive":       0,
		"Recurring":        false,
	}
	return c.runSetOp("SetUISpilloverCmd", params, fmt.Sprintf("pool %d", poolID))
}

// SetSuperchlor toggles superchlorination on a salt chlorinator.
func (c *Client) SetSuperchlor(mspSystemID, poolID, chlorID int, isOn bool) (*CommandResult, error) {
	params := map[string]any{
		"MspSystemID": mspSystemID,
		"PoolID":      poolID,
		"ChlorID":     chlorID,
		"Version":     "0",
		"IsOn":        isOn,
	}
	return c.runSetOp("SetUISuperCHLORCmd", params, fmt.Sprintf("chlor %d", chlorID))
}

// SetLightShow sets a ColorLogic light show (V1).
func (c *Client) SetLightShow(mspSystemID, poolID, lightID, showID int) (*CommandResult, error) {
	params := map[string]any{
		"MspSystemID":      mspSystemID,
		"PoolID":           poolID,
		"LightID":          lightID,
		"Version":          "0",
		"Show":             showID,
		"IsCountDownTimer": false,
		"StartTimeHours":   0,
		"StartTimeMinutes": 0,
		"EndTimeHours":     0,
		"EndTimeMinutes":   0,
		"DaysActive":       0,
		"Recurring":        false,
	}
	return c.runSetOp("SetStandAloneLightShow", params, fmt.Sprintf("light %d", lightID))
}

// SetLightShowV2 sets a ColorLogic light show with speed + brightness (V2 lights only).
func (c *Client) SetLightShowV2(mspSystemID, poolID, lightID, showID, speed, brightness int) (*CommandResult, error) {
	params := map[string]any{
		"MspSystemID":      mspSystemID,
		"PoolID":           poolID,
		"LightID":          lightID,
		"Version":          "0",
		"Show":             showID,
		"Speed":            speed,
		"Brightness":       brightness,
		"IsCountDownTimer": false,
		"StartTimeHours":   0,
		"StartTimeMinutes": 0,
		"EndTimeHours":     0,
		"EndTimeMinutes":   0,
		"DaysActive":       0,
		"Recurring":        false,
	}
	return c.runSetOp("SetStandAloneLightShowV2", params, fmt.Sprintf("light %d", lightID))
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
	return result, nil
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
