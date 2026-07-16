package command

import (
	"context"
	"errors"
	"testing"
)

func TestIncidentReporterRejectsMissingAffectedResourcesBeforePersistence(t *testing.T) {
	_, err := (&IncidentReporter{}).Handle(context.Background(), ReportIncidentCmd{})
	if !errors.Is(err, ErrInvalidIncident) {
		t.Fatalf("expected ErrInvalidIncident, got %v", err)
	}
}

func TestIncidentReporterRejectsBlankAffectedDIDBeforePersistence(t *testing.T) {
	_, err := (&IncidentReporter{}).Handle(context.Background(), ReportIncidentCmd{
		AffectedContractDIDs: []string{"  "},
	})
	if !errors.Is(err, ErrInvalidIncident) {
		t.Fatalf("expected ErrInvalidIncident, got %v", err)
	}
}
