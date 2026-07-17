package dependency

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/templaterepository/datatype/contracttemplatestate"
	"digital-contracting-service/internal/templaterepository/datatype/contracttemplatetype"
	"digital-contracting-service/internal/templaterepository/db"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var (
	ErrInvalidDID          = errors.New("invalid template DID")
	ErrTemplateNotFound    = errors.New("dependency template not found")
	ErrCycle               = errors.New("template dependency cycle")
	ErrTemplateNotReusable = errors.New("dependency template is not reusable")
)

var didPattern = regexp.MustCompile(`^did:[a-z0-9]+:[A-Za-z0-9._:%-]+(?::[A-Za-z0-9._:%-]+)*$`)

func ValidateIdentifier(identifier string) error {
	identifier = strings.TrimSpace(identifier)
	if _, err := uuid.Parse(identifier); err == nil {
		return nil
	}
	if didPattern.MatchString(identifier) {
		return nil
	}
	return fmt.Errorf("%w: %q", ErrInvalidDID, identifier)
}

// ValidateReference verifies existence and rejects a path from referenceDID
// back to sourceDID. It performs no mutation and reads the whole dependency
// graph from authoritative template snapshots.
func ValidateReference(ctx context.Context, tx *sqlx.Tx, repo db.ContractTemplateRepo, sourceDID, referenceDID string) (*db.ContractTemplate, error) {
	return validateReference(ctx, tx, repo.ReadDataByID, sourceDID, referenceDID)
}

// ValidateReusableReference validates a picker reference without taking row locks.
// Mutation paths use ValidateTemplateData and its transaction-scoped share locks.
func ValidateReusableReference(ctx context.Context, tx *sqlx.Tx, repo db.ContractTemplateRepo, sourceDID, referenceDID string) (*db.ContractTemplate, error) {
	template, err := ValidateReference(ctx, tx, repo, sourceDID, referenceDID)
	if err != nil {
		return nil, err
	}
	if err := validateReusableDependency(template); err != nil {
		return nil, err
	}
	return template, nil
}

type templateReader func(context.Context, *sqlx.Tx, string) (*db.ContractTemplate, error)

func validateReference(ctx context.Context, tx *sqlx.Tx, readTemplate templateReader, sourceDID, referenceDID string) (*db.ContractTemplate, error) {
	if err := ValidateIdentifier(referenceDID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(sourceDID) == strings.TrimSpace(referenceDID) {
		return nil, fmt.Errorf("%w: %s references itself", ErrCycle, sourceDID)
	}

	visited := make(map[string]bool)
	var root *db.ContractTemplate
	var visit func(string) error
	visit = func(current string) error {
		if current == sourceDID {
			return fmt.Errorf("%w: dependency path reaches %s", ErrCycle, sourceDID)
		}
		if visited[current] {
			return nil
		}
		visited[current] = true
		template, err := readTemplate(ctx, tx, current)
		if err != nil {
			if errors.Is(err, db.ErrContractTemplateNotFound) {
				return fmt.Errorf("%w: %s", ErrTemplateNotFound, current)
			}
			return err
		}
		if current == referenceDID {
			root = template
		}
		for _, child := range References(template.TemplateData) {
			if err := visit(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(strings.TrimSpace(referenceDID)); err != nil {
		return nil, err
	}
	return root, nil
}

// ValidateTemplateData gates create/update mutations for every declared
// dependency in the candidate document.
func ValidateTemplateData(ctx context.Context, tx *sqlx.Tx, repo db.ContractTemplateRepo, sourceDID string, data *datatype.JSON) error {
	for _, reference := range References(data) {
		template, err := validateReference(ctx, tx, repo.ReadDataByIDForShare, sourceDID, reference)
		if err != nil {
			return err
		}
		if err := validateReusableDependency(template); err != nil {
			return err
		}
	}
	return nil
}

func validateReusableDependency(template *db.ContractTemplate) error {
	if template == nil {
		return ErrTemplateNotReusable
	}

	templateType, err := contracttemplatetype.NewContractTemplateType(template.TemplateType)
	if err != nil || templateType != contracttemplatetype.Component {
		return fmt.Errorf("%w: template %s must have type %s", ErrTemplateNotReusable, template.DID, contracttemplatetype.Component)
	}

	state, err := contracttemplatestate.NewContractTemplateState(template.State)
	if err != nil || (state != contracttemplatestate.Registered && state != contracttemplatestate.Published) {
		return fmt.Errorf(
			"%w: component %s must be in state %s or %s",
			ErrTemplateNotReusable,
			template.DID,
			contracttemplatestate.Registered,
			contracttemplatestate.Published,
		)
	}

	return nil
}

func References(data *datatype.JSON) []string {
	if data == nil || !data.IsNotNullValue() {
		return nil
	}
	var document any
	if err := json.Unmarshal(*data, &document); err != nil {
		return nil
	}
	seen := make(map[string]bool)
	var result []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	var walk func(any)
	walk = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			if reference, ok := typed["dcs:templateDid"].(string); ok {
				add(reference)
			}
			if snapshots, ok := typed["dcs:subTemplates"].([]any); ok {
				for _, snapshot := range snapshots {
					if object, ok := snapshot.(map[string]any); ok {
						if reference, ok := object["@id"].(string); ok {
							add(reference)
						}
					}
				}
			}
			for key, child := range typed {
				if key == "dcs:template" {
					continue
				}
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(document)
	sort.Strings(result)
	return result
}
