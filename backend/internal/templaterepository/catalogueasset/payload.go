package catalogueasset

import (
	"fmt"

	"digital-contracting-service/internal/fcasset"
	"digital-contracting-service/internal/templaterepository/datatype/contracttemplatetype"
	"digital-contracting-service/internal/templaterepository/db"
)

// BuildTemplatePayload validates repository metadata at the catalogue adapter boundary.
func BuildTemplatePayload(
	did string,
	issuer string,
	processData *db.ContractTemplateProcessData,
	fullTemplate *db.ContractTemplate,
) (map[string]any, error) {
	if processData == nil {
		return nil, fmt.Errorf("template process data is nil")
	}
	if fullTemplate == nil {
		return nil, fmt.Errorf("template data is nil")
	}

	templateType, err := contracttemplatetype.NewContractTemplateType(fullTemplate.TemplateType)
	if err != nil {
		return nil, fmt.Errorf("invalid template type for catalogue payload: %w", err)
	}

	name := ""
	description := ""
	if fullTemplate.Name != nil {
		name = *fullTemplate.Name
	}
	if fullTemplate.Description != nil {
		description = *fullTemplate.Description
	}

	return fcasset.BuildPayload(fcasset.BuildInput{
		Issuer:    issuer,
		ValidFrom: fullTemplate.UpdatedAt,
		Subject: fcasset.CatalogueSubjectFromRepository(
			did,
			processData.Version,
			processData.State,
			templateType.String(),
			name,
			description,
		),
	})
}
