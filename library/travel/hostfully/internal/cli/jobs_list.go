// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Narrow generated-tree extension: live Jobs evidence for invoice review.
// Keep provider transport here; the business CLI maps properties to units.
type jobRecord struct {
	UID         string `json:"uid"`
	Status      string `json:"status"`
	PropertyUID string `json:"propertyUid"`
	ServiceUID  string `json:"serviceUid"`
	LeadUID     string `json:"leadUid"`
	Start       string `json:"startLocalDateTime"`
	End         string `json:"endLocalDateTime"`
	Notes       string `json:"notes"`
}
type jobService struct {
	UID         string `json:"uid"`
	ProviderUID string `json:"serviceProviderUid"`
	Type        string `json:"type"`
	Title       string `json:"title"`
}
type jobProvider struct {
	UID     string `json:"uid"`
	First   string `json:"firstName"`
	Last    string `json:"lastName"`
	Company string `json:"companyName"`
}
type jobsResult struct {
	Source    string                 `json:"source"`
	Complete  bool                   `json:"complete"`
	From      string                 `json:"from"`
	To        string                 `json:"to"`
	Jobs      []jobRecord            `json:"jobs"`
	Services  map[string]jobService  `json:"services"`
	Providers map[string]jobProvider `json:"providers"`
}

func newNovelJobsListCmd(flags *rootFlags) *cobra.Command {
	var from, to, properties string
	var mock bool
	list := &cobra.Command{Use: "list", Short: "List live jobs with services and providers; includes cancelled jobs", Args: cobra.NoArgs,
		Example:     "  hostfully-pp-cli jobs list --from 2026-10-01 --to 2026-10-31 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJobsList(cmd, flags, from, to, properties, mock)
		}}
	list.Flags().StringVar(&from, "from", "", "First job date (YYYY-MM-DD)")
	list.Flags().StringVar(&to, "to", "", "Last job date (YYYY-MM-DD); maximum 90-day span")
	list.Flags().StringVar(&properties, "property-uids", "", "Comma-separated property UIDs")
	list.Flags().BoolVar(&mock, "mock", false, "Return a no-network example (same as the root --mock flag)")
	return list
}

func runJobsList(cmd *cobra.Command, flags *rootFlags, from, to, properties string, mock bool) error {
	if flags.dryRun && (from == "" || to == "") {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"method": "GET", "path": "/jobs", "from": from, "to": to, "propertiesUids": properties})
	}
	start, err := time.Parse("2006-01-02", from)
	if err != nil {
		return fmt.Errorf("--from must be YYYY-MM-DD")
	}
	end, err := time.Parse("2006-01-02", to)
	if err != nil || end.Before(start) || end.Sub(start) > 90*24*time.Hour {
		return fmt.Errorf("--to must be on/after --from and within 90 days")
	}
	if flags.dataSource == "local" {
		return fmt.Errorf("jobs requires live data")
	}
	if flags.dryRun {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"method": "GET", "path": "/jobs", "from": from, "to": to, "propertiesUids": properties})
	}
	result := jobsResult{Source: "hostfully_live", Complete: true, From: from, To: to, Jobs: []jobRecord{}, Services: map[string]jobService{}, Providers: map[string]jobProvider{}}
	if mock || mockMode() {
		result.Source = "mock"
		return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	c.NoCache = true
	params := map[string]string{"from": from, "to": to}
	if properties != "" {
		params["propertiesUids"] = properties
	}
	raw, err := c.Get(cmd.Context(), "/jobs", params)
	if err != nil {
		return err
	}
	var response struct {
		Jobs     *[]jobRecord `json:"jobs"`
		Metadata struct {
			Count *int `json:"count"`
			Total *int `json:"totalCount"`
		} `json:"_metadata"`
	}
	if err = json.Unmarshal(raw, &response); err != nil {
		return err
	}
	if response.Jobs == nil {
		return fmt.Errorf("Hostfully response missing jobs array")
	}
	result.Jobs = *response.Jobs
	if (response.Metadata.Count != nil && *response.Metadata.Count != len(result.Jobs)) || (response.Metadata.Total != nil && *response.Metadata.Total != len(result.Jobs)) {
		return fmt.Errorf("Hostfully returned incomplete jobs; narrow the date range")
	}
	seen := map[string]bool{}
	for _, job := range result.Jobs {
		if job.UID == "" || seen[job.UID] || job.PropertyUID == "" || job.ServiceUID == "" {
			return fmt.Errorf("Hostfully returned an invalid or duplicate job")
		}
		seen[job.UID] = true
		if _, err = time.Parse("2006-01-02T15:04:05", job.Start); err != nil {
			return fmt.Errorf("job %s has invalid local start", job.UID)
		}
		if _, ok := result.Services[job.ServiceUID]; !ok {
			raw, err = c.Get(cmd.Context(), "/services/"+url.PathEscape(job.ServiceUID), nil)
			if err != nil {
				return err
			}
			var value struct {
				Service jobService `json:"service"`
			}
			if err = json.Unmarshal(raw, &value); err != nil {
				return err
			}
			if value.Service.UID != job.ServiceUID || value.Service.Type == "" {
				return fmt.Errorf("invalid service for job %s", job.UID)
			}
			result.Services[job.ServiceUID] = value.Service
		}
	}
	for _, service := range result.Services {
		if service.ProviderUID == "" {
			continue
		}
		if _, ok := result.Providers[service.ProviderUID]; ok {
			continue
		}
		raw, err = c.Get(cmd.Context(), "/service-providers/"+url.PathEscape(service.ProviderUID), nil)
		if err != nil {
			return err
		}
		var value struct {
			Provider jobProvider `json:"serviceProvider"`
		}
		if err = json.Unmarshal(raw, &value); err != nil {
			return err
		}
		if value.Provider.UID != service.ProviderUID {
			return fmt.Errorf("invalid provider for service %s", service.UID)
		}
		result.Providers[service.ProviderUID] = value.Provider
	}
	sort.Slice(result.Jobs, func(i, j int) bool {
		a, b := result.Jobs[i], result.Jobs[j]
		return strings.Join([]string{a.Start, a.PropertyUID, a.UID}, "/") < strings.Join([]string{b.Start, b.PropertyUID, b.UID}, "/")
	})
	return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
}
