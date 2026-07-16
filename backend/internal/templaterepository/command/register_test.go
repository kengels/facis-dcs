package command

import (
	"testing"

	"digital-contracting-service/internal/templaterepository/datatype/contracttemplatestate"
	"digital-contracting-service/internal/templaterepository/db"
)

func TestValidateRegistrationStateRequiresApprovedTemplate(t *testing.T) {
	if err := validateRegistrationState(&db.ContractTemplate{State: contracttemplatestate.Approved.String()}); err != nil {
		t.Fatalf("approved template rejected: %v", err)
	}
	for _, state := range []contracttemplatestate.ContractTemplateState{
		contracttemplatestate.Draft,
		contracttemplatestate.Registered,
		contracttemplatestate.Deprecated,
	} {
		if err := validateRegistrationState(&db.ContractTemplate{State: state.String()}); err == nil {
			t.Fatalf("template in state %s accepted for registration", state)
		}
	}
}
