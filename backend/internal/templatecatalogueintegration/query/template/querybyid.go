package template

import (
	"context"
	"errors"
	"fmt"
	"strings"

	templatecatalogueintegration "digital-contracting-service/gen/template_catalogue_integration"
	"digital-contracting-service/internal/fcasset"
	"digital-contracting-service/internal/templatecatalogueintegration/client"
	"digital-contracting-service/internal/templatecatalogueintegration/internal/ptr"
)

type GetByIDQry struct {
	DID     string
	Version int
}

type GetByIDHandler struct {
	Ctx      context.Context
	FCClient *client.FederatedCatalogueClient
}

const retrieveTemplateByIDStatement = `
MATCH (ct:ContractTemplate)
WHERE head(ct.claimsGraphUri) = $did
RETURN {
  did: head(ct.claimsGraphUri),
  name: ct.name,
  description: ct.description,
  version: ct.version,
  state: ct.state,
  template_uuid: ct.templateUuid
} AS n
LIMIT 1
`

func (h *GetByIDHandler) Handle(qry GetByIDQry) (*templatecatalogueintegration.TemplateCatalogueRetrieveByIDResponse, error) {
	if h.FCClient == nil {
		return nil, client.ErrFederatedCatalogueNotConfigured
	}
	if qry.DID == "" {
		return nil, fmt.Errorf("did is empty")
	}
	if qry.Version < 1 {
		return nil, fmt.Errorf("version must be greater than 0")
	}

	resp, err := h.FCClient.Query(h.Ctx, client.QueryRequest{
		Statement: retrieveTemplateByIDStatement,
		Parameters: map[string]string{
			"did": qry.DID,
		},
	})
	if err != nil {
		return nil, err
	}
	if resp.TotalCount == 0 || len(resp.Items) == 0 {
		return nil, nil
	}

	n := projectionMap(resp.Items[0])
	if n == nil {
		return nil, fmt.Errorf("query projection missing projected map for did=%s", qry.DID)
	}

	result := mapCatalogueDetail(n)
	if result == nil {
		return nil, nil
	}

	if result.Version == nil || *result.Version != qry.Version {
		return nil, nil
	}

	templateData, err := fcasset.FetchDocument(h.Ctx, qry.DID)
	if errors.Is(err, fcasset.ErrRemoteTemplateNotFound) {
		return result, nil
	}

	if err != nil {
		return nil, err
	}

	result.TemplateData = templateData
	return result, nil
}

func mapCatalogueDetail(n map[string]interface{}) *templatecatalogueintegration.TemplateCatalogueRetrieveByIDResponse {
	if n == nil {
		return nil
	}

	did := ptr.StringFromMap(n, "did")
	if strings.TrimSpace(did) == "" {
		return nil
	}

	return &templatecatalogueintegration.TemplateCatalogueRetrieveByIDResponse{
		Did:         did,
		Version:     ptr.Ref(ptr.IntFromMap(n, "version")),
		Name:        ptr.Ref(ptr.StringFromMap(n, "name")),
		Description: ptr.Ref(ptr.StringFromMap(n, "description")),
	}
}
