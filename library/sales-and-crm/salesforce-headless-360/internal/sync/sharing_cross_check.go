package sync

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/client"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/store"
)

type SharingAuditor interface {
	RecordSharingDrop(sobject, sobjectID, reason, accountID string) error
}

func FilterVisibleRecords(c HeaderGetClient, auditor SharingAuditor, accountID, sobject string, records []json.RawMessage, gate *Gate) ([]json.RawMessage, error) {
	if len(records) == 0 {
		return nil, nil
	}

	var visible []json.RawMessage
	for start := 0; start < len(records); start += 25 {
		end := start + 25
		if end > len(records) {
			end = len(records)
		}
		for _, record := range records[start:end] {
			id := ExtractRecordID(record)
			if id == "" {
				visible = append(visible, record)
				continue
			}
			if err := gate.Check(); err != nil {
				return visible, &PartialSyncError{Stage: "sync.sharing_cross_check", Err: err}
			}
			_, headers, err := c.GetWithResponseHeaders("/services/data/"+APIVersion+"/ui-api/records/"+id, map[string]string{"fields": "Id"})
			if updateErr := gate.UpdateFromHeaders(headers); updateErr != nil {
				return visible, updateErr
			}
			if err == nil {
				visible = append(visible, record)
				if err := gate.Check(); err != nil {
					return visible, &PartialSyncError{Stage: "sync.sharing_cross_check", Err: err}
				}
				continue
			}
			var apiErr *client.APIError
			if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusForbidden {
				if auditor != nil {
					if auditErr := auditor.RecordSharingDrop(sobject, id, "ui_api_403", accountID); auditErr != nil {
						return visible, auditErr
					}
				}
				continue
			}
			if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
				visible = append(visible, record)
				continue
			}
			return visible, fmt.Errorf("ui api sharing probe %s/%s: %w", sobject, id, err)
		}
	}
	return visible, nil
}

func ExtractRecordID(record json.RawMessage) string {
	var obj map[string]any
	if err := json.Unmarshal(record, &obj); err != nil {
		return ""
	}
	for _, key := range []string{"Id", "id", "ID"} {
		if v, ok := obj[key]; ok {
			s := strings.TrimSpace(fmt.Sprintf("%v", v))
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

var _ SharingAuditor = (*store.Store)(nil)
