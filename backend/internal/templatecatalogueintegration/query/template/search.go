package template

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	templatecatalogueintegration "digital-contracting-service/gen/template_catalogue_integration"
	"digital-contracting-service/internal/templatecatalogueintegration/client"
)

type SearchQry struct {
	DID            string
	DocumentNumber string // ignored for thin catalogue entries
	Version        int
	Name           string
	Description    string
	Offset         int
	Limit          int
}

type SearchHandler struct {
	Ctx      context.Context
	FCClient *client.FederatedCatalogueClient
}

const searchTemplatesCountStatementTemplate = `
MATCH (ct:ContractTemplate)
WHERE head(ct.claimsGraphUri) IS NOT NULL
%s
RETURN count(ct) AS total
`

const searchTemplatesStatementTemplate = `
MATCH (ct:ContractTemplate)
WHERE head(ct.claimsGraphUri) IS NOT NULL
%s
RETURN {
  did: head(ct.claimsGraphUri),
  name: ct.name,
  description: ct.description,
  version: ct.version,
  state: ct.state,
  template_uuid: ct.templateUuid
} AS n
SKIP %d
LIMIT %d
`

func (h *SearchHandler) Handle(qry SearchQry) (*templatecatalogueintegration.TemplateCatalogueRetrieveResponse, error) {
	if h.FCClient == nil {
		return nil, client.ErrFederatedCatalogueNotConfigured
	}
	if qry.Offset < 0 {
		return nil, fmt.Errorf("offset must be >= 0")
	}

	whereClause, params := buildSearchWhereClause(qry)
	where := formatSearchWhereSection(whereClause)

	countStatement := fmt.Sprintf(searchTemplatesCountStatementTemplate, where)
	countResp, err := h.FCClient.Query(h.Ctx, client.QueryRequest{
		Statement:  countStatement,
		Parameters: params,
	})
	if err != nil {
		return nil, err
	}

	totalCount := countResp.TotalCount

	limit := qry.Limit
	if limit < 1 {
		limit = totalCount
	}

	statement := fmt.Sprintf(searchTemplatesStatementTemplate, where, qry.Offset, limit)
	dataResp, err := h.FCClient.Query(h.Ctx, client.QueryRequest{
		Statement:  statement,
		Parameters: params,
	})
	if err != nil {
		return nil, err
	}

	items := make([]*templatecatalogueintegration.TemplateCatalogueItem, 0, len(dataResp.Items))
	for _, item := range dataResp.Items {
		if ct := projectionMap(item); ct != nil {
			if mapped := mapCatalogueItem(ct); mapped != nil {
				items = append(items, mapped)
			}
		}
	}

	return &templatecatalogueintegration.TemplateCatalogueRetrieveResponse{
		TotalCount: totalCount,
		Items:      items,
	}, nil
}

func formatSearchWhereSection(whereClause string) string {
	if whereClause == "" {
		return ""
	}
	return "AND " + whereClause
}

func buildSearchWhereClause(qry SearchQry) (string, map[string]string) {
	conditions := make([]string, 0, 4)
	params := make(map[string]string)

	if value := strings.TrimSpace(qry.DID); value != "" {
		conditions = append(conditions, "toLower(head(ct.claimsGraphUri)) CONTAINS toLower($did)")
		params["did"] = value
	}
	if qry.Version > 0 {
		conditions = append(conditions, "ct.version = toString($version)")
		params["version"] = strconv.Itoa(qry.Version)
	}
	if value := strings.TrimSpace(qry.Name); value != "" {
		conditions = append(conditions, "toLower(coalesce(ct.name, '')) CONTAINS toLower($name)")
		params["name"] = value
	}
	if value := strings.TrimSpace(qry.Description); value != "" {
		conditions = append(conditions, "toLower(coalesce(ct.description, '')) CONTAINS toLower($description)")
		params["description"] = value
	}

	return strings.Join(conditions, " AND "), params
}
