package dependency

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/templaterepository/db"

	"github.com/jmoiron/sqlx"
)

func TestReferencesExtractsCanonicalDependencyLocations(t *testing.T) {
	data := datatype.JSON(`{"dcs:metadata":{"dcs:subTemplates":[{"@id":"c30a36a2-a19f-4a03-bfe5-78f456118c42"}]},"dcs:documentStructure":{"dcs:blocks":{"@list":[{"@type":"dcs:ApprovedTemplate","dcs:templateDid":"did:web:example.test:templates:component"}]}}}`)
	want := []string{"c30a36a2-a19f-4a03-bfe5-78f456118c42", "did:web:example.test:templates:component"}
	if got := References(&data); !reflect.DeepEqual(got, want) {
		t.Fatalf("references = %v, want %v", got, want)
	}
}

func TestValidateReferenceClassifiesMissingAndCycle(t *testing.T) {
	const (
		source = "11111111-1111-4111-8111-111111111111"
		child  = "22222222-2222-4222-8222-222222222222"
	)
	missingRepo := &dependencyRepoFake{templates: map[string]*db.ContractTemplate{}}
	if _, err := ValidateReference(context.Background(), nil, missingRepo, source, child); !errors.Is(err, ErrTemplateNotFound) {
		t.Fatalf("missing reference error = %v, want ErrTemplateNotFound", err)
	}

	data := datatype.JSON(fmt.Sprintf(`{"dcs:metadata":{"dcs:subTemplates":[{"@id":%q}]}}`, source))
	cycleRepo := &dependencyRepoFake{templates: map[string]*db.ContractTemplate{
		child: {DID: child, TemplateData: &data},
	}}
	if _, err := ValidateReference(context.Background(), nil, cycleRepo, source, child); !errors.Is(err, ErrCycle) {
		t.Fatalf("cycle error = %v, want ErrCycle", err)
	}
}

func TestValidateIdentifierClassifiesInvalidValues(t *testing.T) {
	for _, valid := range []string{"c30a36a2-a19f-4a03-bfe5-78f456118c42", "did:web:example.test:templates:component"} {
		if err := ValidateIdentifier(valid); err != nil {
			t.Fatalf("valid identifier %q rejected: %v", valid, err)
		}
	}
	if err := ValidateIdentifier("not a template identifier"); err == nil {
		t.Fatal("invalid identifier accepted")
	}
}

type dependencyRepoFake struct {
	db.ContractTemplateRepo
	templates map[string]*db.ContractTemplate
}

func (r *dependencyRepoFake) ReadDataByID(_ context.Context, _ *sqlx.Tx, did string) (*db.ContractTemplate, error) {
	template := r.templates[did]
	if template == nil {
		return nil, fmt.Errorf("%w: %s", db.ErrContractTemplateNotFound, did)
	}
	return template, nil
}
