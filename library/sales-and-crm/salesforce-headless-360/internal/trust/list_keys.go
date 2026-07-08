package trust

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

type KeyListOptions struct {
	OrgAlias string
	Live     bool
}

type KeyListRow struct {
	KID          string     `json:"kid"`
	OrgAlias     string     `json:"org"`
	Algorithm    string     `json:"algorithm"`
	Status       string     `json:"status"`
	Source       string     `json:"source"`
	RegisteredAt time.Time  `json:"registered_at"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
	RetiredAt    *time.Time `json:"retired_at,omitempty"`
}

func ListKeys(c OrgClient, opts KeyListOptions) ([]KeyListRow, error) {
	records, err := ListKeyRecords()
	if err != nil {
		return nil, err
	}
	if opts.Live && c != nil {
		liveRecords, err := listOrgCertificateKeys(c, opts.OrgAlias)
		if err == nil {
			records = mergeKeyRecords(records, liveRecords)
		}
	}
	rows := make([]KeyListRow, 0, len(records))
	for _, record := range records {
		if opts.OrgAlias != "" && record.OrgAlias != opts.OrgAlias {
			continue
		}
		status := "active"
		if record.RetiredAt != nil {
			status = "retired"
		}
		algorithm := record.Algorithm
		if algorithm == "" {
			algorithm = "Ed25519"
		}
		rows = append(rows, KeyListRow{
			KID:          record.KID,
			OrgAlias:     record.OrgAlias,
			Algorithm:    algorithm,
			Status:       status,
			Source:       record.Source,
			RegisteredAt: record.RegisteredAt,
			LastUsedAt:   record.LastUsedAt,
			RetiredAt:    record.RetiredAt,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].RegisteredAt.After(rows[j].RegisteredAt)
	})
	return rows, nil
}

func listOrgCertificateKeys(c OrgClient, orgAlias string) ([]KeyRecord, error) {
	q := "SELECT Id, DeveloperName, MasterLabel, ExpirationDate FROM Certificate WHERE DeveloperName LIKE 'SF360_Bundle_Key_%'"
	raw, err := c.Get("/services/data/"+APIVersion+"/tooling/query", map[string]string{"q": q})
	if err != nil {
		return nil, err
	}
	var payload struct {
		Records []struct {
			ID            string `json:"Id"`
			DeveloperName string `json:"DeveloperName"`
		} `json:"records"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("parse certificate list: %w", err)
	}
	now := time.Now().UTC()
	records := make([]KeyRecord, 0, len(payload.Records))
	for _, cert := range payload.Records {
		kid := cert.DeveloperName
		kid = trimCertificatePrefix(kid)
		records = append(records, KeyRecord{
			KID:           kid,
			OrgAlias:      orgAlias,
			Algorithm:     "Ed25519",
			RegisteredAt:  now,
			Source:        "certificate",
			CertificateID: cert.ID,
		})
	}
	return records, nil
}

func mergeKeyRecords(local, live []KeyRecord) []KeyRecord {
	seen := map[string]bool{}
	out := make([]KeyRecord, 0, len(local)+len(live))
	for _, record := range local {
		seen[record.KID] = true
		out = append(out, record)
	}
	for _, record := range live {
		if seen[record.KID] {
			continue
		}
		out = append(out, record)
	}
	return out
}

func trimCertificatePrefix(name string) string {
	const prefix = "SF360_Bundle_Key_"
	if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
		return name[len(prefix):]
	}
	return name
}
