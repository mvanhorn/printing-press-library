// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

// Package capture extracts ordertogo.com payment configuration from a real
// captured checkout — a browser/proxy HAR or the postmicmeshorder request body
// itself. It mirrors the Printing Press browsersniff capture parser: load a
// HAR or enriched JSON, find the order POST, and read the saved-card fields out
// of its body so they can be written to config without a manual DevTools step.
package capture

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// PaymentConfig is the set of stable, per-user fields the postmicmeshorder body
// carries that `orders place` needs from config.
type PaymentConfig struct {
	StripeCustomerID  string `json:"stripe_customer_id"`
	StripeDefaultCard string `json:"stripe_default_card"`
	CustomerFirstName string `json:"customer_firstname"`
	CustomerLastName  string `json:"customer_lastname"`
	CustomerPhone     string `json:"customer_phone"`
	// Newer request-shape fields the live client sends (2026-06); without them
	// the server rejects/times out CLI-built orders.
	DeviceID         string `json:"device_id"`
	MobileID         string `json:"mobile_id"`
	OrderContextJSON string `json:"order_context_json"`
}

// Minimal HAR 1.2 subset, matching cli-printing-press/internal/browsersniff.
type har struct {
	Log struct {
		Entries []struct {
			Request struct {
				Method   string `json:"method"`
				URL      string `json:"url"`
				PostData *struct {
					Text string `json:"text"`
				} `json:"postData"`
			} `json:"request"`
		} `json:"entries"`
	} `json:"log"`
}

const orderPath = "postmicmeshorder"

// LoadPaymentConfig reads a capture file and extracts the payment config. It
// accepts a raw HAR (containing a postmicmeshorder POST), the captured-order
// JSON artifact (with an actual_observed_body field), or a file that is the
// postmicmeshorder request body itself.
func LoadPaymentConfig(path string) (*PaymentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading capture: %w", err)
	}
	body, err := orderBody(data)
	if err != nil {
		return nil, err
	}
	return ExtractPaymentConfig([]byte(body))
}

// orderBody locates the postmicmeshorder request body string within a capture.
func orderBody(data []byte) (string, error) {
	// HAR: find the postmicmeshorder POST and return its postData text.
	if bytesContains(data, `"log"`) {
		var h har
		if err := json.Unmarshal(data, &h); err == nil {
			for _, e := range h.Log.Entries {
				if strings.Contains(e.Request.URL, orderPath) && e.Request.PostData != nil {
					return e.Request.PostData.Text, nil
				}
			}
			return "", fmt.Errorf("no %s POST with a request body found in HAR", orderPath)
		}
	}
	// Captured-order artifact: prefer a full observed body, fall back to a
	// truncated prefix (the tolerant extractor handles partial JSON).
	var artifact struct {
		ActualObservedBody       string `json:"actual_observed_body"`
		ActualObservedBodyPrefix string `json:"actual_observed_body_prefix_2000"`
	}
	if err := json.Unmarshal(data, &artifact); err == nil {
		if artifact.ActualObservedBody != "" {
			return artifact.ActualObservedBody, nil
		}
		if artifact.ActualObservedBodyPrefix != "" {
			return artifact.ActualObservedBodyPrefix, nil
		}
	}
	// Otherwise assume the file is the request body itself.
	return string(data), nil
}

// ExtractPaymentConfig reads the saved-card fields from a postmicmeshorder body,
// tolerating truncated/partial JSON via field-level extraction.
func ExtractPaymentConfig(body []byte) (*PaymentConfig, error) {
	var parsed struct {
		Param struct {
			CustomerName  string          `json:"customername"`
			CustomerPhone string          `json:"customerphone"`
			DeviceID      string          `json:"deviceId"`
			MobileID      string          `json:"mobileId"`
			Context       json.RawMessage `json:"context"`
			PaymentCard   struct {
				StCusID        string         `json:"st_cus_id"`
				DefaultCardMap map[string]any `json:"defaultCardMap"`
				FirstName      string         `json:"firstname"`
				LastName       string         `json:"lastname"`
				PhoneNum       string         `json:"phonenum"`
			} `json:"paymentCard"`
		} `json:"param"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil && parsed.Param.PaymentCard.StCusID != "" {
		pc := parsed.Param.PaymentCard
		key, _ := pc.DefaultCardMap["key"].(string)
		ctx := ""
		if len(parsed.Param.Context) > 0 && string(parsed.Param.Context) != "null" {
			ctx = string(parsed.Param.Context)
		}
		return &PaymentConfig{
			StripeCustomerID:  pc.StCusID,
			StripeDefaultCard: key,
			CustomerFirstName: firstNonEmpty(pc.FirstName, parsed.Param.CustomerName),
			CustomerLastName:  pc.LastName,
			CustomerPhone:     firstNonEmpty(pc.PhoneNum, parsed.Param.CustomerPhone),
			DeviceID:          parsed.Param.DeviceID,
			MobileID:          parsed.Param.MobileID,
			OrderContextJSON:  ctx,
		}, nil
	}

	// Tolerant fallback for truncated bodies.
	cfg := &PaymentConfig{
		StripeCustomerID:  jsonStringField(body, "st_cus_id"),
		StripeDefaultCard: cardKey(body),
		CustomerFirstName: firstNonEmpty(jsonStringField(body, "firstname"), jsonStringField(body, "customername")),
		CustomerLastName:  jsonStringField(body, "lastname"),
		CustomerPhone:     firstNonEmpty(jsonStringField(body, "phonenum"), jsonStringField(body, "customerphone")),
	}
	if cfg.StripeCustomerID == "" {
		return nil, fmt.Errorf("no postmicmeshorder payment fields found in capture")
	}
	return cfg, nil
}

// cardKey extracts defaultCardMap.key, scanning from the defaultCardMap marker
// so it does not collide with other "key" fields.
func cardKey(body []byte) string {
	s := string(body)
	if i := strings.Index(s, "defaultCardMap"); i >= 0 {
		return jsonStringField([]byte(s[i:]), "key")
	}
	return ""
}

// jsonStringField returns the string value of "field":"value" in body, or "".
// It is intentionally small and truncation-tolerant rather than a full parser.
func jsonStringField(body []byte, field string) string {
	s := string(body)
	marker := `"` + field + `"`
	idx := strings.Index(s, marker)
	if idx < 0 {
		return ""
	}
	rest := s[idx+len(marker):]
	colon := strings.IndexByte(rest, ':')
	if colon < 0 {
		return ""
	}
	rest = strings.TrimLeft(rest[colon+1:], " \t")
	if len(rest) == 0 || rest[0] != '"' {
		return ""
	}
	rest = rest[1:]
	var out strings.Builder
	for i := 0; i < len(rest); i++ {
		c := rest[i]
		if c == '\\' && i+1 < len(rest) {
			i++
			out.WriteByte(rest[i])
			continue
		}
		if c == '"' {
			return out.String()
		}
		out.WriteByte(c)
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func bytesContains(data []byte, sub string) bool {
	return strings.Contains(string(data), sub)
}
