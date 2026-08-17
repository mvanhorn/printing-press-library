// Hand-written addition: shared CKAN datastore_search response parsing — preserve on regeneration.

package cli

import (
	"encoding/json"
	"fmt"
)

// parseCKANDatastore unmarshals a CKAN /api/3/action/datastore_search response.
//
// CKAN always replies HTTP 200, even for API-level errors (e.g. a stale or
// invalid resource_id), signalling failure with {"success": false, "error":
// {...}} and no "result" field. Without inspecting "success" the caller would
// see records==nil / total==0 and mistake an API error for a genuine empty
// result set. Surface the CKAN error instead.
func parseCKANDatastore(raw []byte) (records []map[string]any, total int, err error) {
	var resp struct {
		Success bool `json:"success"`
		Error   struct {
			Type    string `json:"__type"`
			Message string `json:"message"`
		} `json:"error"`
		Result struct {
			Records []map[string]any `json:"records"`
			Total   int              `json:"total"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, 0, fmt.Errorf("parsing CKAN response: %w", err)
	}
	if !resp.Success {
		msg := resp.Error.Message
		if msg == "" {
			msg = resp.Error.Type
		}
		if msg == "" {
			msg = "risposta CKAN con success:false senza dettagli"
		}
		return nil, 0, fmt.Errorf("errore API CKAN: %s", msg)
	}
	return resp.Result.Records, resp.Result.Total, nil
}
