package template

import (
	"context"
	"fmt"
	"strings"

	templatecatalogueintegration "digital-contracting-service/gen/template_catalogue_integration"
	"digital-contracting-service/internal/templatecatalogueintegration/client"
	"digital-contracting-service/internal/templatecatalogueintegration/internal/ptr"
)

type GetAllMetadataQry struct {
	Offset int
	Limit  int
}

type GetAllMetadataHandler struct {
	Ctx      context.Context
	FCClient *client.FederatedCatalogueClient
}

const retrieveTemplatesCountStatement = `
MATCH (ct:ContractTemplate)
WHERE head(ct.claimsGraphUri) IS NOT NULL
RETURN count(ct) AS total
`

const retrieveTemplatesStatementTemplate = `
MATCH (ct:ContractTemplate)
WHERE head(ct.claimsGraphUri) IS NOT NULL
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

func (h *GetAllMetadataHandler) Handle(qry GetAllMetadataQry) (*templatecatalogueintegration.TemplateCatalogueRetrieveResponse, error) {
	if h.FCClient == nil {
		return nil, client.ErrFederatedCatalogueNotConfigured
	}
	if qry.Offset < 0 {
		return nil, fmt.Errorf("offset must be >= 0")
	}

	countResp, err := h.FCClient.Query(h.Ctx, client.QueryRequest{
		Statement: retrieveTemplatesCountStatement,
	})
	if err != nil {
		return nil, err
	}

	totalCount := countResp.TotalCount

	limit := qry.Limit
	if limit < 1 {
		limit = totalCount
	}

	statement := fmt.Sprintf(retrieveTemplatesStatementTemplate, qry.Offset, limit)
	dataResp, err := h.FCClient.Query(h.Ctx, client.QueryRequest{
		Statement: statement,
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

func projectionMap(row map[string]interface{}) map[string]interface{} {
	if row == nil {
		return nil
	}
	for _, value := range row {
		if mapped, ok := value.(map[string]interface{}); ok {
			return mapped
		}
	}
	return nil
}

func mapCatalogueItem(ct map[string]interface{}) *templatecatalogueintegration.TemplateCatalogueItem {
	if ct == nil {
		return nil
	}

	did := ptr.StringFromMap(ct, "did")
	if strings.TrimSpace(did) == "" {
		return nil
	}

	return &templatecatalogueintegration.TemplateCatalogueItem{
		Did:         did,
		Version:     ptr.Ref(ptr.IntFromMap(ct, "version")),
		Name:        ptr.Ref(ptr.StringFromMap(ct, "name")),
		Description: ptr.Ref(ptr.StringFromMap(ct, "description")),
	}
}
