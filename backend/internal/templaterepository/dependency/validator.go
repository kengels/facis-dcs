package dependency

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/templaterepository/db"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

var (
	ErrInvalidDID       = errors.New("invalid template DID")
	ErrTemplateNotFound = errors.New("dependency template not found")
	ErrCycle            = errors.New("template dependency cycle")
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
		template, err := repo.ReadDataByID(ctx, tx, current)
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
		if _, err := ValidateReference(ctx, tx, repo, sourceDID, reference); err != nil {
			return err
		}
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
			for _, child := range typed {
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(document)
	return result
}
