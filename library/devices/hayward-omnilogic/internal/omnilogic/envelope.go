package omnilogic

import (
	"encoding/xml"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// envParam is one <Parameter name="X" dataType="T">value</Parameter> entry.
type envParam struct {
	Name     string `xml:"name,attr"`
	DataType string `xml:"dataType,attr"`
	Value    string `xml:",chardata"`
}

// envRequest is the top-level <Request><Name>Op</Name><Parameters>...</Parameters></Request>.
type envRequest struct {
	XMLName    xml.Name   `xml:"Request"`
	Name       string     `xml:"Name"`
	Parameters envParamsW `xml:"Parameters"`
}

type envParamsW struct {
	Params []envParam `xml:"Parameter"`
}

// buildRequest mirrors the Python wrapper's buildRequest. Each param dataType
// is inferred from the Go type: int/int32/int64 -> "int", bool -> "bool",
// float -> "double", string -> "string". The Token key is dropped from the
// body — it travels in the HTTP header instead. Params are sorted by name
// to keep the wire format deterministic: Go map iteration is randomized,
// and Hayward's .NET handler is order-sensitive for at least one Set*
// operation (SetCHLORParams). Deterministic alphabetical order means the
// same logical request emits byte-identical XML across processes, which
// makes troubleshooting and regression-testing tractable. Order-sensitive
// Set* operations should use buildOrderedRequest with a hand-authored slice
// instead.
func buildRequest(opName string, params map[string]any) (string, error) {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "Token" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	req := envRequest{Name: opName}
	for _, k := range keys {
		dt, val, err := paramRepr(params[k])
		if err != nil {
			return "", fmt.Errorf("param %q: %w", k, err)
		}
		req.Parameters.Params = append(req.Parameters.Params, envParam{Name: k, DataType: dt, Value: val})
	}
	out, err := xml.Marshal(req)
	if err != nil {
		return "", err
	}
	return xml.Header + string(out), nil
}

// orderedParam describes one Parameter element in canonical order. Hayward's
// .NET backend can return "Input string was not in a correct format" on
// otherwise-valid XML when the parameter order differs from what the
// SetUIEquipmentCmd handler expects, so callers that hit the .ashx endpoint
// build their request via this typed list rather than a map.
type orderedParam struct {
	Name     string
	DataType string
	Value    any
}

// buildOrderedRequest is the deterministic-order variant of buildRequest:
// callers pass an []orderedParam in the exact order Hayward's handler
// expects. The Token param is filtered out (it travels in the HTTP header,
// not the body). Entries with nil Value are skipped so callers can include
// optional params in canonical position without sending blanks.
func buildOrderedRequest(opName string, params []orderedParam) (string, error) {
	var b strings.Builder
	b.WriteString(xml.Header)
	b.WriteString("<Request><Name>")
	b.WriteString(opName)
	b.WriteString("</Name><Parameters>")
	for _, p := range params {
		if p.Name == "Token" || p.Value == nil {
			continue
		}
		_, val, err := paramRepr(p.Value)
		if err != nil {
			return "", fmt.Errorf("param %q: %w", p.Name, err)
		}
		fmt.Fprintf(&b, `<Parameter name="%s" dataType="%s">%s</Parameter>`, p.Name, p.DataType, val)
	}
	b.WriteString("</Parameters></Request>")
	return b.String(), nil
}

// buildChlorRequest constructs SetCHLORParams using the exact parameter order
// and dataTypes the Hayward backend requires. The "ORPTimout" key intentionally
// preserves Hayward's typo.
func buildChlorRequest(params map[string]any) string {
	order := []struct {
		Name string
		Type string
	}{
		{"MspSystemID", "int"}, {"PoolID", "int"}, {"ChlorID", "int"},
		{"CfgState", "byte"}, {"OpMode", "byte"}, {"BOWType", "byte"},
		{"CellType", "byte"}, {"TimedPercent", "byte"},
		{"SCTimeout", "byte"}, {"ORPTimout", "byte"},
	}
	var b strings.Builder
	b.WriteString(xml.Header)
	b.WriteString("<Request><Name>SetCHLORParams</Name><Parameters>")
	for _, p := range order {
		if v, ok := params[p.Name]; ok {
			_, val, err := paramRepr(v)
			if err == nil && val != "" {
				fmt.Fprintf(&b, `<Parameter name="%s" dataType="%s">%s</Parameter>`, p.Name, p.Type, val)
				continue
			}
		}
		// Hayward's defaulting rule: missing SCTimeout / ORPTimout fall back to 4 hours.
		if p.Name == "SCTimeout" || p.Name == "ORPTimout" {
			fmt.Fprintf(&b, `<Parameter name="%s" dataType="%s">4</Parameter>`, p.Name, p.Type)
		}
	}
	b.WriteString("</Parameters></Request>")
	return b.String()
}

func paramRepr(v any) (datatype, value string, err error) {
	switch x := v.(type) {
	case int:
		return "int", strconv.Itoa(x), nil
	case int32:
		return "int", strconv.FormatInt(int64(x), 10), nil
	case int64:
		return "int", strconv.FormatInt(x, 10), nil
	case bool:
		// Hayward's .NET handler is strict about bool casing — it expects
		// lowercase "true"/"false", not Python's str(bool) "True"/"False".
		if x {
			return "bool", "true", nil
		}
		return "bool", "false", nil
	case float64:
		return "double", strconv.FormatFloat(x, 'f', -1, 64), nil
	case float32:
		return "double", strconv.FormatFloat(float64(x), 'f', -1, 32), nil
	case string:
		return "string", x, nil
	default:
		return "", "", fmt.Errorf("unsupported param type %T", v)
	}
}

// envResponse models a generic OmniLogic XML response: <Response><Parameters><Parameter name="X" dataType="T">V</Parameter>...</Parameters></Response>.
type envResponse struct {
	XMLName    xml.Name   `xml:"Response"`
	Parameters envParamsR `xml:"Parameters"`
}

type envParamsR struct {
	Params []envParam `xml:"Parameter"`
	// Some responses carry richer Item-based lists; we capture them with InnerXML
	// and let operation-specific parsers handle them.
	InnerXML string `xml:",innerxml"`
}

// statusFromResponse extracts the "Status" parameter (0 = success) and
// "StatusMessage" if present. Returns (0, "") if no Status was found
// (which is the case for GetMspConfigFile/GetTelemetryData responses).
func statusFromResponse(xmlText string) (status int, message string, ok bool) {
	var resp envResponse
	if err := xml.Unmarshal([]byte(xmlText), &resp); err != nil {
		return 0, "", false
	}
	for _, p := range resp.Parameters.Params {
		if p.Name == "Status" {
			if n, err := strconv.Atoi(strings.TrimSpace(p.Value)); err == nil {
				status = n
				ok = true
			}
		}
		if p.Name == "StatusMessage" {
			message = p.Value
		}
	}
	return status, message, ok
}
