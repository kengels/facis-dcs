package template

import (
	"context"
	"errors"
	"testing"

	templatecatalogueintegration "digital-contracting-service/gen/template_catalogue_integration"
	"digital-contracting-service/internal/fcasset"
)

func TestEnrichCatalogueDetailSkipsDocumentFetchForUUIDMetadata(t *testing.T) {
	templateType := "COMPONENT"
	result := &templatecatalogueintegration.TemplateCatalogueRetrieveByIDResponse{
		Did:          "6a1d6d73-1d6f-44f7-a20f-45c88ff529d9",
		TemplateType: &templateType,
	}
	fetchCalled := false

	got, err := enrichCatalogueDetailWithDocument(
		context.Background(),
		result.Did,
		result,
		func(context.Context, string) (map[string]any, error) {
			fetchCalled = true
			return nil, errors.New("unexpected fetch")
		},
	)

	if err != nil {
		t.Fatalf("enrichCatalogueDetailWithDocument() error = %v", err)
	}
	if fetchCalled {
		t.Fatal("document fetch was called for UUID catalogue metadata")
	}
	if got != result || got.TemplateType == nil || *got.TemplateType != "COMPONENT" {
		t.Fatalf("detail = %#v, want mapped component metadata", got)
	}
}

func TestEnrichCatalogueDetailKeepsDIDWebFetchBehavior(t *testing.T) {
	did := "did:web:templates.example:component-1"
	result := &templatecatalogueintegration.TemplateCatalogueRetrieveByIDResponse{Did: did}

	t.Run("attaches fetched document", func(t *testing.T) {
		document := map[string]any{"@id": did}
		got, err := enrichCatalogueDetailWithDocument(
			context.Background(),
			did,
			result,
			func(_ context.Context, fetchedDID string) (map[string]any, error) {
				if fetchedDID != did {
					t.Fatalf("fetched DID = %q, want %q", fetchedDID, did)
				}
				return document, nil
			},
		)
		if err != nil {
			t.Fatalf("enrichCatalogueDetailWithDocument() error = %v", err)
		}
		if got.TemplateData == nil {
			t.Fatal("fetched template document was not attached")
		}
	})

	t.Run("keeps metadata when remote document is missing", func(t *testing.T) {
		metadata := &templatecatalogueintegration.TemplateCatalogueRetrieveByIDResponse{Did: did}
		got, err := enrichCatalogueDetailWithDocument(
			context.Background(),
			did,
			metadata,
			func(context.Context, string) (map[string]any, error) {
				return nil, fcasset.ErrRemoteTemplateNotFound
			},
		)
		if err != nil || got != metadata {
			t.Fatalf("detail = %#v, error = %v; want unchanged metadata", got, err)
		}
	})

	t.Run("propagates fetch errors", func(t *testing.T) {
		fetchErr := errors.New("remote DCS unavailable")
		_, err := enrichCatalogueDetailWithDocument(
			context.Background(),
			did,
			result,
			func(context.Context, string) (map[string]any, error) {
				return nil, fetchErr
			},
		)
		if !errors.Is(err, fetchErr) {
			t.Fatalf("error = %v, want %v", err, fetchErr)
		}
	})
}
