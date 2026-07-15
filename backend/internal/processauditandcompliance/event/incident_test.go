package event

import "testing"

func TestIncidentReportedEventAnchorsAffectedResource(t *testing.T) {
	evt := IncidentReportedEvent{
		IncidentID:  "incident-1",
		ResourceDID: "did:web:example.test:contracts:1",
		Reason:      "Missing approval",
	}

	if got := evt.EventType(); got != "PAC_INCIDENT_REPORTED" {
		t.Fatalf("unexpected event type %q", got)
	}
	if got := evt.GetDID(); got != evt.ResourceDID {
		t.Fatalf("expected resource DID %q, got %q", evt.ResourceDID, got)
	}
}
