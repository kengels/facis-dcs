package dependency

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/templaterepository/datatype/contracttemplatestate"
	"digital-contracting-service/internal/templaterepository/datatype/contracttemplatetype"
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

func TestValidateTemplateDataAcceptsOnlyReusableDirectComponents(t *testing.T) {
	const (
		source = "11111111-1111-4111-8111-111111111111"
		child  = "22222222-2222-4222-8222-222222222222"
	)
	data := datatype.JSON(fmt.Sprintf(`{"dcs:metadata":{"dcs:subTemplates":[{"@id":%q}]}}`, child))

	for _, state := range []contracttemplatestate.ContractTemplateState{
		contracttemplatestate.Registered,
		contracttemplatestate.Published,
	} {
		repo := &dependencyRepoFake{templates: map[string]*db.ContractTemplate{
			child: {
				DID:          child,
				TemplateType: contracttemplatetype.Component.String(),
				State:        state.String(),
			},
		}}
		if err := ValidateTemplateData(context.Background(), nil, repo, source, &data); err != nil {
			t.Fatalf("reusable component in state %s rejected: %v", state, err)
		}
	}

	for name, template := range map[string]*db.ContractTemplate{
		"contract template": {
			DID:          child,
			TemplateType: contracttemplatetype.ContractTemplate.String(),
			State:        contracttemplatestate.Registered.String(),
		},
		"approved component": {
			DID:          child,
			TemplateType: contracttemplatetype.Component.String(),
			State:        contracttemplatestate.Approved.String(),
		},
	} {
		t.Run(name, func(t *testing.T) {
			repo := &dependencyRepoFake{templates: map[string]*db.ContractTemplate{child: template}}
			err := ValidateTemplateData(context.Background(), nil, repo, source, &data)
			if !errors.Is(err, ErrTemplateNotReusable) {
				t.Fatalf("error = %v, want ErrTemplateNotReusable", err)
			}
		})
	}
}

func TestValidateTemplateDataIgnoresEmbeddedSnapshotReferencesForAvailability(t *testing.T) {
	const (
		source = "11111111-1111-4111-8111-111111111111"
		root   = "22222222-2222-4222-8222-222222222222"
		child  = "33333333-3333-4333-8333-333333333333"
	)
	candidate := datatype.JSON(fmt.Sprintf(
		`{"dcs:metadata":{"dcs:subTemplates":[{"@id":%q,"dcs:template":{"dcs:metadata":{"dcs:subTemplates":[{"@id":%q}]}}}]}}`,
		root,
		child,
	))
	repo := &dependencyRepoFake{templates: map[string]*db.ContractTemplate{
		root: {
			DID:          root,
			TemplateType: contracttemplatetype.Component.String(),
			State:        contracttemplatestate.Registered.String(),
		},
		child: {
			DID:          child,
			TemplateType: contracttemplatetype.Component.String(),
			State:        contracttemplatestate.Deprecated.String(),
		},
	}}

	if err := ValidateTemplateData(context.Background(), nil, repo, source, &candidate); err != nil {
		t.Fatalf("embedded historical child blocked reusable direct root: %v", err)
	}
	if repo.reads[child] != 0 {
		t.Fatalf("embedded immutable child was read %d times, want 0", repo.reads[child])
	}
}

func TestValidateTemplateDataUsesShareLockedReads(t *testing.T) {
	const (
		source = "11111111-1111-4111-8111-111111111111"
		child  = "22222222-2222-4222-8222-222222222222"
	)
	data := datatype.JSON(fmt.Sprintf(`{"dcs:metadata":{"dcs:subTemplates":[{"@id":%q}]}}`, child))
	repo := &dependencyRepoFake{templates: map[string]*db.ContractTemplate{
		child: {
			DID:          child,
			TemplateType: contracttemplatetype.Component.String(),
			State:        contracttemplatestate.Published.String(),
		},
	}}

	if err := ValidateTemplateData(context.Background(), nil, repo, source, &data); err != nil {
		t.Fatalf("ValidateTemplateData() error = %v", err)
	}
	if repo.shareReads[child] == 0 {
		t.Fatal("dependency row was not read through ReadDataByIDForShare")
	}
	if repo.reads[child] != 0 {
		t.Fatalf("dependency row used unlocked reader %d times", repo.reads[child])
	}
}

func TestValidateReusableReferenceChecksAvailabilityWithoutMutationLock(t *testing.T) {
	const (
		source = "11111111-1111-4111-8111-111111111111"
		child  = "22222222-2222-4222-8222-222222222222"
	)
	repo := &dependencyRepoFake{templates: map[string]*db.ContractTemplate{
		child: {
			DID:          child,
			TemplateType: contracttemplatetype.Component.String(),
			State:        contracttemplatestate.Approved.String(),
		},
	}}

	_, err := ValidateReusableReference(context.Background(), nil, repo, source, child)
	if !errors.Is(err, ErrTemplateNotReusable) {
		t.Fatalf("error = %v, want ErrTemplateNotReusable", err)
	}
	if repo.reads[child] == 0 {
		t.Fatal("query validation did not use the unlocked reader")
	}
	if repo.shareReads[child] != 0 {
		t.Fatalf("query validation acquired mutation lock %d times", repo.shareReads[child])
	}
}

type dependencyRepoFake struct {
	db.ContractTemplateRepo
	templates  map[string]*db.ContractTemplate
	reads      map[string]int
	shareReads map[string]int
}

func (r *dependencyRepoFake) ReadDataByID(_ context.Context, _ *sqlx.Tx, did string) (*db.ContractTemplate, error) {
	if r.reads == nil {
		r.reads = make(map[string]int)
	}
	r.reads[did]++
	return r.read(did)
}

func (r *dependencyRepoFake) ReadDataByIDForShare(_ context.Context, _ *sqlx.Tx, did string) (*db.ContractTemplate, error) {
	if r.shareReads == nil {
		r.shareReads = make(map[string]int)
	}
	r.shareReads[did]++
	return r.read(did)
}

func (r *dependencyRepoFake) read(did string) (*db.ContractTemplate, error) {
	template := r.templates[did]
	if template == nil {
		return nil, fmt.Errorf("%w: %s", db.ErrContractTemplateNotFound, did)
	}
	return template, nil
}
