package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
)

type linearPromotedGraphQLSpec struct {
	Singular   string
	Connection string
	List       string
	Singleton  string
	Selection  string
}

var linearPromotedGraphQLSpecs = map[string]linearPromotedGraphQLSpec{
	"attachments":                      {Singular: "attachment", Connection: "attachments", Selection: "id title subtitle url source sourceType createdAt updatedAt"},
	"audit-entry-types":                {List: "auditEntryTypes", Selection: "type description"},
	"authentication-session-responses": {List: "authenticationSessions", Selection: "id name ip location createdAt updatedAt lastActiveAt isCurrentSession"},
	"email-intake-addresses":           {Singular: "emailIntakeAddress", Selection: "id address enabled senderName createdAt updatedAt team { id key name }"},
	"favorites":                        {Singular: "favorite", Connection: "favorites", Selection: "id type title url color icon createdAt updatedAt"},
	"initiative-relations":             {Singular: "initiativeRelation", Connection: "initiativeRelations", Selection: "id sortOrder createdAt updatedAt"},
	"initiative-to-projects":           {Singular: "initiativeToProject", Connection: "initiativeToProjects", Selection: "id sortOrder createdAt updatedAt"},
	"initiatives":                      {Singular: "initiative", Connection: "initiatives", Selection: "id name description content slugId status url createdAt updatedAt"},
	"issue-priority-values":            {List: "issuePriorityValues", Selection: "priority label"},
	"organizations":                    {Singleton: "organization", Selection: "id name urlKey createdAt updatedAt"},
	"project-labels":                   {Singular: "projectLabel", Connection: "projectLabels", Selection: "id name description color isGroup createdAt updatedAt"},
	"project-milestones":               {Singular: "projectMilestone", Connection: "projectMilestones", Selection: "id name description targetDate sortOrder progress createdAt updatedAt"},
	"project-relations":                {Singular: "projectRelation", Connection: "projectRelations", Selection: "id type anchorType relatedAnchorType createdAt updatedAt"},
	"project-statuses":                 {Singular: "projectStatus", Connection: "projectStatuses", Selection: "id name color description position indefinite createdAt updatedAt"},
	"projects":                         {Singular: "project", Connection: "projects", Selection: "id name description state slugId targetDate startDate progress url createdAt updatedAt teams { nodes { id name key } }"},
	"release-notes":                    {Singular: "releaseNote", Selection: "id title slugId createdAt updatedAt"},
	"release-pipelines":                {Connection: "releasePipelines", Selection: "id name slugId isProduction approximateReleaseCount url createdAt updatedAt teams { nodes { id name key } }"},
	"release-stages":                   {Singular: "releaseStage", Connection: "releaseStages", Selection: "id name color position frozen createdAt updatedAt"},
	"releases":                         {Singular: "release", Connection: "releases", Selection: "id name description version slugId startDate targetDate progress url createdAt updatedAt"},
	"roadmap-to-projects":              {Singular: "roadmapToProject", Selection: "id sortOrder createdAt updatedAt"},
	"roadmaps":                         {Singular: "roadmap", Selection: "id name description slugId color url createdAt updatedAt"},
	"teams":                            {Singular: "team", Connection: "teams", Selection: "id name key description color createdAt updatedAt"},
	"templates":                        {Singular: "template", List: "templates", Selection: "id type name description icon color createdAt updatedAt team { id key name }"},
	"user-settingses":                  {Singleton: "userSettings", Selection: "id calendarHash showFullUserNames autoAssignToSelf createdAt updatedAt"},
	"users":                            {Singular: "user", Connection: "users", Selection: "id name displayName email active admin url createdAt updatedAt"},
}

func isLinearPromotedGraphQLRead(path string) bool {
	return path == "/graphql"
}

func linearPromotedGraphQLReadIsList(resourceType string, params map[string]string) bool {
	spec, ok := linearPromotedGraphQLSpecs[resourceType]
	if !ok {
		return false
	}
	if params != nil && params["id"] != "" {
		return false
	}
	return spec.Connection != "" || spec.List != ""
}

// resolveLinearPromotedGraphQLRead issues the mapped read and returns its raw
// payload. In dry-run mode the request is still printed by the transport, but
// the query yields no body. Rather than let downstream printing see a nil
// payload, return an empty value shaped like the real one: [] for the list and
// connection reads, {} for the singular and singleton reads. Mapping errors
// still surface on the dry-run path because they are configuration faults, not
// missing data.
func resolveLinearPromotedGraphQLRead(c *client.Client, resourceType string, params map[string]string) (json.RawMessage, error) {
	spec, ok := linearPromotedGraphQLSpecs[resourceType]
	if !ok {
		return nil, fmt.Errorf("promoted Linear GraphQL read %q is not mapped; use a first-class command or local sync until this generated endpoint is patched", resourceType)
	}
	id := ""
	if params != nil {
		id = params["id"]
	}
	raw, err := queryLinearPromotedGraphQLRead(c, spec, resourceType, id)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		if linearPromotedGraphQLReadIsList(resourceType, params) {
			return json.RawMessage("[]"), nil
		}
		return json.RawMessage("{}"), nil
	}
	return raw, nil
}

func queryLinearPromotedGraphQLRead(c *client.Client, spec linearPromotedGraphQLSpec, resourceType, id string) (json.RawMessage, error) {
	switch {
	case id != "" && spec.Singular != "":
		query := fmt.Sprintf(`query($id: String!) { %s(id: $id) { %s } }`, spec.Singular, spec.Selection)
		return queryPromotedGraphQLField(c, query, map[string]any{"id": id}, spec.Singular)
	case id != "":
		return nil, fmt.Errorf("promoted Linear GraphQL read %q does not support lookup by id", resourceType)
	case spec.Connection != "":
		query := fmt.Sprintf(`query($first: Int!, $after: String) { %s(first: $first, after: $after) { nodes { %s } pageInfo { hasNextPage endCursor } } }`, spec.Connection, spec.Selection)
		nodes, err := c.PaginatedQuery(query, nil, spec.Connection, 100)
		if err != nil {
			return nil, err
		}
		if c.DryRun {
			return nil, nil
		}
		// PaginatedQuery hands back a nil slice for an empty connection, and
		// json.Marshal turns that into null, but --json consumers want an array.
		if nodes == nil {
			nodes = []json.RawMessage{}
		}
		return json.Marshal(nodes)
	case spec.List != "":
		query := fmt.Sprintf(`query { %s { %s } }`, spec.List, spec.Selection)
		return queryPromotedGraphQLField(c, query, nil, spec.List)
	case spec.Singleton != "":
		query := fmt.Sprintf(`query { %s { %s } }`, spec.Singleton, spec.Selection)
		return queryPromotedGraphQLField(c, query, nil, spec.Singleton)
	default:
		return nil, fmt.Errorf("promoted Linear GraphQL read %q has no usable GraphQL field mapping", resourceType)
	}
}

func queryPromotedGraphQLField(c *client.Client, query string, variables map[string]any, field string) (json.RawMessage, error) {
	data, err := c.Query(query, variables)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		// Dry run: the request was printed and no body came back. Leave the
		// empty-payload shaping to the caller, which knows list from object.
		return nil, nil
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parsing GraphQL data: %w", err)
	}
	raw, ok := root[field]
	if !ok || string(raw) == "null" {
		return nil, notFoundErr(fmt.Errorf("%s not found", field))
	}
	return raw, nil
}
