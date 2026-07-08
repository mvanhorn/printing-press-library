package datacloud

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	applog "github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/log"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/security"
)

type FetchOptions struct {
	HTTPClient *http.Client
	Filter     security.Filter
	DMOMap     DMOMap
}

type ProfileResult struct {
	Profile      map[string]any
	DMO          string
	Availability Availability
	Provenance   *security.Provenance
}

func Fetch(ctx context.Context, c CoreClient, accountID string, opts FetchOptions) (ProfileResult, error) {
	token, availability, err := Exchange(ctx, c)
	result := ProfileResult{Availability: availability, Provenance: &security.Provenance{Redactions: map[string]int{}}}
	if err != nil || !availability.Available {
		return result, err
	}
	for _, dmo := range CandidateDMOs("account", opts.DMOMap) {
		profile, status, err := fetchProfile(ctx, token, dmo, accountID, opts.HTTPClient)
		if status == http.StatusNotFound {
			continue
		}
		if err != nil {
			return result, err
		}
		if profile == nil {
			continue
		}
		filtered, provenance, err := applyFilter(ctx, opts.Filter, dmo, profile)
		if err != nil {
			return result, err
		}
		result.Profile = filtered
		result.DMO = dmo
		result.Availability.Available = true
		result.Availability.Reason = ""
		result.Provenance = provenance
		return result, nil
	}
	result.Availability.Available = false
	result.Availability.Reason = "dmo_not_found"
	return result, nil
}

func fetchProfile(ctx context.Context, token OffcoreToken, dmo, id string, client *http.Client) (map[string]any, int, error) {
	if client == nil {
		client = http.DefaultClient
	}
	base := strings.TrimRight(token.InstanceURL, "/")
	path := "/api/v1/profile/" + url.PathEscape(dmo) + "/" + url.PathEscape(id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("User-Agent", "github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/1.0.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("data cloud profile %s: %w", dmo, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return nil, resp.StatusCode, nil
	}
	if resp.StatusCode >= 400 {
		return nil, resp.StatusCode, fmt.Errorf("data cloud profile %s returned HTTP %d: %s", dmo, resp.StatusCode, applog.Redact(string(body)))
	}
	var profile map[string]any
	if err := unmarshalEnvelope(json.RawMessage(body), &profile); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("parse data cloud profile: %w", err)
	}
	return profile, resp.StatusCode, nil
}

func applyFilter(ctx context.Context, filter security.Filter, sobject string, fields map[string]any) (map[string]any, *security.Provenance, error) {
	provenance := &security.Provenance{Redactions: map[string]int{}}
	if filter == nil {
		return fields, provenance, nil
	}
	record := &security.Record{SObject: sobject, Fields: fields, Provenance: &security.Provenance{Redactions: map[string]int{}}}
	record = filter.Apply(ctx, record)
	if record == nil {
		return nil, provenance, nil
	}
	security.MergeProvenance(provenance, record.Provenance)
	return record.Fields, provenance, nil
}
