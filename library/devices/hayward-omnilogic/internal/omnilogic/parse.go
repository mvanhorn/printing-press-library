package omnilogic

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// parseSiteList parses the GetSiteList XML response into []Site.
//
// Shape:
//
//	<Response>
//	  <Parameters>
//	    <Parameter name="List" dataType="...">
//	      <Item>
//	        <Property name="MspSystemID" dataType="int">12345</Property>
//	        <Property name="BackyardName" dataType="string">Backyard 1</Property>
//	      </Item>
//	      ...
//	    </Parameter>
//	  </Parameters>
//	</Response>
//
// Different Hayward releases have used both <Item> and direct child Items;
// we accept either.
func parseSiteList(xmlText string) ([]Site, error) {
	dec := xml.NewDecoder(strings.NewReader(xmlText))
	var sites []Site
	var inList bool
	var curSite *Site
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parsing site list: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "Parameter":
				for _, a := range t.Attr {
					if a.Name.Local == "name" && a.Value == "List" {
						inList = true
					}
				}
			case "Item":
				if inList {
					curSite = &Site{}
				}
			}
			if curSite != nil && (t.Name.Local == "Property" || t.Name.Local == "Parameter") {
				// Read this property's name and chardata.
				attrName := ""
				for _, a := range t.Attr {
					if a.Name.Local == "name" {
						attrName = a.Value
					}
				}
				var charVal string
				for {
					inner, err := dec.Token()
					if err != nil {
						break
					}
					if cd, ok := inner.(xml.CharData); ok {
						charVal += string(cd)
					}
					if end, ok := inner.(xml.EndElement); ok && (end.Name.Local == "Property" || end.Name.Local == "Parameter") {
						break
					}
				}
				charVal = strings.TrimSpace(charVal)
				switch attrName {
				case "MspSystemID":
					if n, err := strconv.Atoi(charVal); err == nil {
						curSite.MspSystemID = n
					}
				case "BackyardName":
					curSite.BackyardName = charVal
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "Item":
				if curSite != nil {
					sites = append(sites, *curSite)
					curSite = nil
				}
			case "Parameter":
				if inList {
					inList = false
				}
			}
		}
	}
	return sites, nil
}

// parseAlarmList parses the GetAlarmList XML response.
//
// Shape (per Python wrapper):
//
//	<Response><Parameters><Parameter name="List"><Item><Property name="K">V</Property>...</Item>...</Parameter></Parameters></Response>
func parseAlarmList(xmlText string) []Alarm {
	dec := xml.NewDecoder(strings.NewReader(xmlText))
	var alarms []Alarm
	var inList bool
	var cur *Alarm
	for {
		tok, err := dec.Token()
		if err == io.EOF || err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "Parameter" {
				for _, a := range t.Attr {
					if a.Name.Local == "name" && a.Value == "List" {
						inList = true
					}
				}
			}
			if t.Name.Local == "Item" && inList {
				cur = &Alarm{Raw: map[string]string{}}
			}
			if cur != nil && (t.Name.Local == "Property" || t.Name.Local == "Parameter") {
				attrName := ""
				for _, a := range t.Attr {
					if a.Name.Local == "name" {
						attrName = a.Value
					}
				}
				var charVal string
				for {
					inner, err := dec.Token()
					if err != nil {
						break
					}
					if cd, ok := inner.(xml.CharData); ok {
						charVal += string(cd)
					}
					if end, ok := inner.(xml.EndElement); ok && (end.Name.Local == "Property" || end.Name.Local == "Parameter") {
						break
					}
				}
				charVal = strings.TrimSpace(charVal)
				if attrName != "" {
					cur.Raw[attrName] = charVal
					switch attrName {
					case "EquipmentID":
						cur.EquipmentID = charVal
					case "BowID":
						cur.BowID = charVal
					case "AlarmCode", "Code":
						cur.Code = charVal
					case "AlarmType":
						cur.Severity = charVal
					case "Message", "DisplayText":
						cur.Message = charVal
					case "FirstOccurrence", "FirstSeen":
						cur.FirstSeen = charVal
					}
				}
			}
		case xml.EndElement:
			if t.Name.Local == "Item" && cur != nil {
				if cur.EquipmentID != "" || len(cur.Raw) > 0 {
					alarms = append(alarms, *cur)
				}
				cur = nil
			}
			if t.Name.Local == "Parameter" && inList {
				inList = false
			}
		}
	}
	return alarms
}

// parseMspConfig parses GetMspConfigFile XML into MspConfig.
//
// MSP shape is deeply nested. We extract:
//   - Backyard name (top-level)
//   - Body-of-water list (each has Name, Type, Shared-Type, System-Id, equipment lists)
//   - Equipment within each BoW (Pump, Heater, ColorLogic-Light, Relay, Chlorinator, Filter, CSAD, Valve)
//   - Backyard-level Relay list
func parseMspConfig(xmlText string) (*MspConfig, error) {
	cfg := &MspConfig{FetchedAt: time.Now().UTC()}
	var raw mspRaw
	if err := xml.Unmarshal([]byte(xmlText), &raw); err != nil {
		return nil, fmt.Errorf("parsing MSP config: %w", err)
	}
	cfg.BackyardName = raw.MSPConfig.Backyard.Name
	for _, r := range raw.MSPConfig.Backyard.Relay {
		cfg.Relays = append(cfg.Relays, Equipment{
			SystemID: r.SystemID,
			Name:     r.Name,
			Type:     r.Type,
			Function: r.Function,
		})
	}
	for _, bow := range raw.MSPConfig.Backyard.BOW {
		out := BodyOfWater{
			SystemID:          bow.SystemID,
			Name:              bow.Name,
			Type:              bow.Type,
			SharedType:        bow.SharedType,
			SharedEquipID:     bow.SharedEquipID,
			SupportsSpillover: bow.SupportsSpillover,
		}
		// Pumps: BoW may have Filter (always one) + Pump (zero or more)
		if bow.Filter != nil {
			out.Filter = &Equipment{
				SystemID: bow.Filter.SystemID, Name: bow.Filter.Name, Type: bow.Filter.Type,
				Function: bow.Filter.Function, MinSpeed: bow.Filter.MinSpeed, MaxSpeed: bow.Filter.MaxSpeed,
			}
			// Treat the BoW's filter (which is its primary pump on most systems)
			// as a controllable pump too.
			out.Pumps = append(out.Pumps, *out.Filter)
		}
		for _, p := range bow.Pump {
			out.Pumps = append(out.Pumps, Equipment{
				SystemID: p.SystemID, Name: p.Name, Type: p.Type, Function: p.Function,
				MinSpeed: p.MinSpeed, MaxSpeed: p.MaxSpeed,
			})
		}
		for _, h := range bow.Heater {
			for _, op := range h.Operation {
				out.Heaters = append(out.Heaters, Heater{
					Name:                 op.HeaterEquipment.Name,
					SystemID:             h.SystemID,
					Enabled:              h.Enabled,
					CurrentSetPoint:      h.CurrentSetPoint,
					MaxWaterTemp:         h.MaxWaterTemp,
					MinSettableWaterTemp: h.MinSettableWaterTemp,
					MaxSettableWaterTemp: h.MaxSettableWaterTemp,
					SharedType:           h.SharedType,
					HeaterType:           op.HeaterEquipment.Type,
				})
			}
			// Fallback when only a single heater is present and Operation isn't a list
			if len(h.Operation) == 0 && h.SystemID != "" {
				out.Heaters = append(out.Heaters, Heater{
					Name:                 "Heater",
					SystemID:             h.SystemID,
					Enabled:              h.Enabled,
					CurrentSetPoint:      h.CurrentSetPoint,
					MaxWaterTemp:         h.MaxWaterTemp,
					MinSettableWaterTemp: h.MinSettableWaterTemp,
					MaxSettableWaterTemp: h.MaxSettableWaterTemp,
					SharedType:           h.SharedType,
				})
			}
		}
		for _, l := range bow.Light {
			v2 := "no"
			if l.V2Active != "" {
				v2 = "yes"
			}
			out.Lights = append(out.Lights, Equipment{
				SystemID: l.SystemID, Name: l.Name, Type: l.Type, V2Active: v2,
			})
		}
		for _, r := range bow.Relay {
			out.Relays = append(out.Relays, Equipment{
				SystemID: r.SystemID, Name: r.Name, Type: r.Type, Function: r.Function,
			})
		}
		if bow.Chlor != nil {
			out.Chlorinator = &Equipment{
				SystemID: bow.Chlor.SystemID, Name: bow.Chlor.Name, Type: bow.Chlor.Type,
				CellType: bow.Chlor.CellType,
			}
		}
		if bow.CSAD != nil {
			out.CSAD = &Equipment{
				SystemID: bow.CSAD.SystemID, Name: bow.CSAD.Name, Type: bow.CSAD.Type,
			}
		}
		cfg.BodiesOfWater = append(cfg.BodiesOfWater, out)
	}
	return cfg, nil
}

// parseTelemetry parses GetTelemetryData XML into a Telemetry snapshot.
// The shape uses element attributes (not nested Parameter elements) so we
// have to walk it more loosely than the other XML responses.
func parseTelemetry(xmlText string) (*Telemetry, error) {
	t := &Telemetry{SampledAt: time.Now().UTC()}
	dec := xml.NewDecoder(strings.NewReader(xmlText))
	var curBOW *TelemetryBOW
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parsing telemetry: %w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch se.Name.Local {
		case "Backyard":
			if v := attr(se, "airTemp"); v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					t.AirTemp = &n
				}
			}
			if v := attr(se, "status"); v != "" {
				t.Status = v
			}
		case "BodyOfWater":
			if curBOW != nil {
				t.BodiesOfWater = append(t.BodiesOfWater, *curBOW)
			}
			curBOW = &TelemetryBOW{
				SystemID: attr(se, "systemId"),
				Attrs:    attrsExcept(se, "systemId", "waterTemp"),
			}
			if v := attr(se, "waterTemp"); v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					curBOW.WaterTemp = &n
				}
			}
		case "Filter", "Pump":
			st := equipState(se)
			if curBOW != nil {
				curBOW.Pumps = append(curBOW.Pumps, st)
			}
		case "Heater":
			st := equipState(se)
			if curBOW != nil {
				curBOW.Heaters = append(curBOW.Heaters, st)
			}
		case "ColorLogic-Light":
			st := equipState(se)
			if curBOW != nil {
				curBOW.Lights = append(curBOW.Lights, st)
			}
		case "Relay":
			st := equipState(se)
			if curBOW == nil {
				t.Relays = append(t.Relays, st)
			} else {
				curBOW.Relays = append(curBOW.Relays, st)
			}
		case "Chlorinator":
			if curBOW != nil {
				if v := attr(se, "Salt"); v != "" {
					if n, err := strconv.Atoi(v); err == nil {
						curBOW.SaltPPM = &n
					}
				}
				if v := attr(se, "instantSaltLevel"); v != "" && curBOW.SaltPPM == nil {
					if n, err := strconv.Atoi(v); err == nil {
						curBOW.SaltPPM = &n
					}
				}
				if v := attr(se, "chlrOutPct"); v != "" {
					if n, err := strconv.Atoi(v); err == nil {
						curBOW.ChlorOutputPct = &n
					}
				}
				if v := attr(se, "Timed-Percent"); v != "" && curBOW.ChlorOutputPct == nil {
					if n, err := strconv.Atoi(v); err == nil {
						curBOW.ChlorOutputPct = &n
					}
				}
			}
		case "CSAD":
			if curBOW != nil {
				if v := attr(se, "ph"); v != "" {
					if f, err := strconv.ParseFloat(v, 64); err == nil {
						curBOW.PH = &f
					}
				}
				if v := attr(se, "orp"); v != "" {
					if n, err := strconv.Atoi(v); err == nil {
						curBOW.ORP = &n
					}
				}
			}
		}
	}
	if curBOW != nil {
		t.BodiesOfWater = append(t.BodiesOfWater, *curBOW)
	}
	return t, nil
}

// equipState extracts the common equipment state attributes from a
// telemetry XML element (Pump, Heater, Light, Relay, etc).
func equipState(se xml.StartElement) TelemetryEquipmentState {
	st := TelemetryEquipmentState{
		SystemID: attr(se, "systemId"),
		Attrs:    attrsExcept(se, "systemId", "valveActuator", "filter-state", "lastTemp"),
	}
	if v := attr(se, "valveActuator"); v != "" {
		b := (v == "1" || v == "true" || v == "True" || v == "on" || v == "On")
		st.IsOn = &b
	}
	if v := attr(se, "filterSpeed"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			st.Speed = &n
		}
	}
	if v := attr(se, "pumpSpeed"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			st.Speed = &n
		}
	}
	if v := attr(se, "speed"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			st.Speed = &n
		}
	}
	if v := attr(se, "filter-state"); v != "" {
		on := v != "0" && v != "off" && v != "Off"
		st.IsOn = &on
	}
	if v := attr(se, "showId"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			st.ShowID = &n
		}
	}
	if v := attr(se, "brightness"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			st.Brightness = &n
		}
	}
	if v := attr(se, "lightState"); v != "" {
		on := v != "0" && v != "off"
		st.IsOn = &on
	}
	if v := attr(se, "heaterState"); v != "" {
		on := v == "1" || v == "On" || v == "on"
		st.Enabled = &on
	}
	if v := attr(se, "currentSetPoint"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			st.SetPoint = &n
		}
	}
	if v := attr(se, "Current-Set-Point"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			st.SetPoint = &n
		}
	}
	return st
}

func attr(se xml.StartElement, name string) string {
	for _, a := range se.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func attrsExcept(se xml.StartElement, excluded ...string) map[string]string {
	ex := map[string]bool{}
	for _, e := range excluded {
		ex[e] = true
	}
	out := map[string]string{}
	for _, a := range se.Attr {
		if ex[a.Name.Local] {
			continue
		}
		out[a.Name.Local] = a.Value
	}
	return out
}

// ----- minimal XML structs for MSP config unmarshal -----

type mspRaw struct {
	XMLName   xml.Name `xml:"Response"`
	MSPConfig mspRoot  `xml:"MSPConfig"`
}

type mspRoot struct {
	Backyard mspBackyard `xml:"Backyard"`
}

type mspBackyard struct {
	Name  string     `xml:"Name"`
	Relay []mspRelay `xml:"Relay"`
	BOW   []mspBOW   `xml:"Body-of-water"`
}

type mspRelay struct {
	SystemID string `xml:"System-Id"`
	Name     string `xml:"Name"`
	Type     string `xml:"Type"`
	Function string `xml:"Function"`
}

type mspBOW struct {
	SystemID          string      `xml:"System-Id"`
	Name              string      `xml:"Name"`
	Type              string      `xml:"Type"`
	SharedType        string      `xml:"Shared-Type"`
	SharedEquipID     string      `xml:"Shared-Equipment-System-ID"`
	SupportsSpillover string      `xml:"Supports-Spillover"`
	Filter            *mspFilter  `xml:"Filter"`
	Pump              []mspPump   `xml:"Pump"`
	Heater            []mspHeater `xml:"Heater"`
	Light             []mspLight  `xml:"ColorLogic-Light"`
	Relay             []mspRelay  `xml:"Relay"`
	Chlor             *mspChlor   `xml:"Chlorinator"`
	CSAD              *mspCSAD    `xml:"CSAD"`
}

type mspFilter struct {
	SystemID string `xml:"System-Id"`
	Name     string `xml:"Name"`
	Type     string `xml:"Filter-Type"`
	Function string `xml:"Function"`
	MinSpeed string `xml:"Min-Pump-Speed"`
	MaxSpeed string `xml:"Max-Pump-Speed"`
}

type mspPump struct {
	SystemID string `xml:"System-Id"`
	Name     string `xml:"Name"`
	Type     string `xml:"Type"`
	Function string `xml:"Function"`
	MinSpeed string `xml:"Min-Pump-Speed"`
	MaxSpeed string `xml:"Max-Pump-Speed"`
}

type mspHeater struct {
	SystemID             string        `xml:"System-Id"`
	Enabled              string        `xml:"Enabled"`
	CurrentSetPoint      string        `xml:"Current-Set-Point"`
	MaxWaterTemp         string        `xml:"Max-Water-Temp"`
	MinSettableWaterTemp string        `xml:"Min-Settable-Water-Temp"`
	MaxSettableWaterTemp string        `xml:"Max-Settable-Water-Temp"`
	SharedType           string        `xml:"Shared-Type"`
	Operation            []mspHeaterOp `xml:"Operation"`
}

type mspHeaterOp struct {
	HeaterEquipment mspHeaterEq `xml:"Heater-Equipment"`
}

type mspHeaterEq struct {
	Name string `xml:"Name"`
	Type string `xml:"Heater-Type"`
}

type mspLight struct {
	SystemID string `xml:"System-Id"`
	Name     string `xml:"Name"`
	Type     string `xml:"Type"`
	V2Active string `xml:"V2-Active"`
}

type mspChlor struct {
	SystemID string `xml:"System-Id"`
	Name     string `xml:"Name"`
	Type     string `xml:"Type"`
	CellType string `xml:"Cell-Type"`
}

type mspCSAD struct {
	SystemID string `xml:"System-Id"`
	Name     string `xml:"Name"`
	Type     string `xml:"Type"`
}
