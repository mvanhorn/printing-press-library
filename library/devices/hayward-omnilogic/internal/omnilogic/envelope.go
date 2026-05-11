package omnilogic

import (
	"encoding/xml"
	"fmt"
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
// body — it travels in the HTTP header instead.
func buildRequest(opName string, params map[string]any) (string, error) {
	req := envRequest{Name: opName}
	for k, v := range params {
		if k == "Token" {
			continue
		}
		dt, val, err := paramRepr(v)
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
		if x {
			return "bool", "True", nil
		}
		return "bool", "False", nil
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
