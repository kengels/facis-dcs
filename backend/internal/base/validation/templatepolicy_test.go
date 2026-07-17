package validation

import (
	"encoding/json"
	"strings"
	"testing"

	"digital-contracting-service/internal/base/datatype"

	"github.com/stretchr/testify/require"
)

func TestAuditTemplatePoliciesReturnsWarningsForMissingFinishedTemplateFields(t *testing.T) {
	findings, err := AuditTemplatePolicies(canonicalTemplateData(t), TemplatePolicyAuditMetadata{
		DID:          "did:example:template",
		TemplateType: "CONTRACT_TEMPLATE",
		State:        "APPROVED",
	})
	require.NoError(t, err)

	require.NotEmpty(t, findings)
	require.Contains(t, policyFindingRuleIDs(findings), "FACIS-TPL-LEGAL-001")
	require.NotContains(t, policyFindingRuleIDs(findings), "FACIS-TPL-PARTY-001")
}

func TestAuditTemplatePoliciesAcceptsCanonicalTemplateWithoutErrors(t *testing.T) {
	findings, err := AuditTemplatePolicies(canonicalTemplateData(t), TemplatePolicyAuditMetadata{
		DID:          "did:example:template",
		TemplateType: "CONTRACT_TEMPLATE",
		State:        "APPROVED",
	})
	require.NoError(t, err)

	for _, finding := range findings {
		require.NotEqual(t, "error", finding.Severity, finding.Message)
	}
}

func TestAuditTemplatePoliciesFlagsCanonicalClauseWithoutContractDataBinding(t *testing.T) {
	data := canonicalTemplateData(t)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(*data, &decoded))
	structure := decoded["dcs:documentStructure"].(map[string]any)
	blocks := structure["dcs:blocks"].(map[string]any)["@list"].([]any)
	clause := blocks[0].(map[string]any)
	content := clause["dcs:content"].(map[string]any)["@list"].([]any)
	placeholder := content[1].(map[string]any)
	placeholder["dcs:bindsTo"] = map[string]any{"@id": "urn:uuid:missing-field"}
	raw, err := datatype.NewJSON(decoded)
	require.NoError(t, err)

	findings, err := AuditTemplatePolicies(&raw, TemplatePolicyAuditMetadata{
		DID:          "did:example:template",
		TemplateType: "CONTRACT_TEMPLATE",
		State:        "APPROVED",
	})
	require.NoError(t, err)

	require.True(t, hasFindingSeverity(findings, "FACIS-TPL-CLAUSE-001", "error"))
}

func TestAuditTemplatePoliciesFlagsPolicyOperandOutsideContractData(t *testing.T) {
	data := canonicalTemplateData(t)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(*data, &decoded))
	policy := firstPolicyDuty(decoded)
	constraint := policy["odrl:constraint"].(map[string]any)
	constraint["odrl:leftOperand"] = map[string]any{"@id": "urn:uuid:missing-field"}
	raw, err := datatype.NewJSON(decoded)
	require.NoError(t, err)

	findings, err := AuditTemplatePolicies(&raw, TemplatePolicyAuditMetadata{
		DID:          "did:example:template",
		TemplateType: "CONTRACT_TEMPLATE",
		State:        "APPROVED",
	})
	require.NoError(t, err)

	require.True(t, hasFindingSeverity(findings, "FACIS-TPL-POLICY-001", "error"))
}

func TestAuditTemplatePoliciesAcceptsRequiredDomainFields(t *testing.T) {
	data := canonicalTemplateData(t)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(*data, &decoded))
	requirement := decoded["dcs:contractData"].([]any)[0].(map[string]any)
	fields := requirement["dcs:fields"].([]any)
	requirement["dcs:fields"] = append(fields,
		canonicalRequirementField("jurisdiction"),
		canonicalRequirementField("signature-level"),
	)
	raw, err := datatype.NewJSON(decoded)
	require.NoError(t, err)

	findings, err := AuditTemplatePolicies(&raw, TemplatePolicyAuditMetadata{
		DID:          "did:example:template",
		TemplateType: "CONTRACT_TEMPLATE",
		State:        "APPROVED",
	})
	require.NoError(t, err)

	ruleIDs := policyFindingRuleIDs(findings)
	require.NotContains(t, ruleIDs, "FACIS-TPL-LEGAL-001")
	require.NotContains(t, ruleIDs, "FACIS-TPL-PARTY-001")
	require.NotContains(t, ruleIDs, "FACIS-TPL-SIGN-001")
}

func TestAuditTemplatePoliciesDoesNotApplyCompletenessRulesToComponents(t *testing.T) {
	data := canonicalTemplateData(t)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(*data, &decoded))
	delete(decoded, "dcs:contractData")
	delete(decoded, "dcs:policies")
	structure := decoded["dcs:documentStructure"].(map[string]any)
	delete(structure, "dcs:layout")
	raw, err := datatype.NewJSON(decoded)
	require.NoError(t, err)

	findings, err := AuditTemplatePolicies(&raw, TemplatePolicyAuditMetadata{
		DID:          "did:example:component",
		TemplateType: "COMPONENT",
		State:        "DRAFT",
	})
	require.NoError(t, err)

	ruleIDs := policyFindingRuleIDs(findings)
	require.NotContains(t, ruleIDs, "FACIS-TPL-STRUCT-001")
	require.NotContains(t, ruleIDs, "FACIS-TPL-DATA-001")
	require.NotContains(t, ruleIDs, "FACIS-TPL-POLICY-001")
	require.NotContains(t, ruleIDs, "FACIS-TPL-LIFECYCLE-001")
	require.NotContains(t, ruleIDs, "FACIS-TPL-CLAUSE-001")
	require.NotContains(t, ruleIDs, "FACIS-TPL-LEGAL-001")
	require.NotContains(t, ruleIDs, "FACIS-TPL-PARTY-001")
	require.NotContains(t, ruleIDs, "FACIS-TPL-SIGN-001")
	for _, finding := range findings {
		require.NotEqual(t, "error", finding.Severity, finding.Message)
	}
}

func TestAuditTemplatePoliciesFlagsComponentInternalPolicyReferences(t *testing.T) {
	data := canonicalTemplateData(t)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(*data, &decoded))
	policy := firstPolicyDuty(decoded)
	constraint := policy["odrl:constraint"].(map[string]any)
	constraint["odrl:leftOperand"] = map[string]any{"@id": "urn:uuid:missing-field"}
	raw, err := datatype.NewJSON(decoded)
	require.NoError(t, err)

	findings, err := AuditTemplatePolicies(&raw, TemplatePolicyAuditMetadata{
		DID:          "did:example:component",
		TemplateType: "COMPONENT",
		State:        "DRAFT",
	})
	require.NoError(t, err)

	require.True(t, hasFindingSeverity(findings, "FACIS-COMP-POLICY-001", "error"))
	require.False(t, hasFindingSeverity(findings, "FACIS-TPL-POLICY-001", "error"))
}

func TestAuditTemplatePoliciesUsesImmediateComponentDataAndClauseSemantics(t *testing.T) {
	root := decodedCanonicalTemplate(t)
	root["dcs:contractData"] = []any{}
	root["dcs:policies"] = []any{}
	unbindFirstClause(root)
	component := decodedCanonicalTemplate(t)
	attachImmediateComponent(root, component)

	findings := auditContractTemplateData(t, root)

	ruleIDs := policyFindingRuleIDs(findings)
	require.NotContains(t, ruleIDs, "FACIS-TPL-DATA-001")
	require.NotContains(t, ruleIDs, "FACIS-TPL-CLAUSE-001")
}

func TestAuditTemplatePoliciesReportsMissingDataAndClauseAcrossComposition(t *testing.T) {
	root := decodedCanonicalTemplate(t)
	root["dcs:contractData"] = []any{}
	unbindFirstClause(root)
	component := decodedCanonicalTemplate(t)
	component["dcs:contractData"] = []any{}
	unbindFirstClause(component)
	attachImmediateComponent(root, component)

	findings := auditContractTemplateData(t, root)

	require.True(t, hasFindingSeverity(findings, "FACIS-TPL-DATA-001", "error"))
	require.True(t, hasFindingSeverity(findings, "FACIS-TPL-CLAUSE-001", "error"))
}

func TestAuditTemplatePoliciesDoesNotHideMalformedDataInEitherCompositionSource(t *testing.T) {
	for _, testCase := range []struct {
		name               string
		malformedComponent bool
		expectedPathPrefix string
	}{
		{name: "root", expectedPathPrefix: "dcs:contractData"},
		{name: "component", malformedComponent: true, expectedPathPrefix: "dcs:metadata.dcs:subTemplates[0].dcs:template.dcs:contractData"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			root := decodedCanonicalTemplate(t)
			component := decodedCanonicalTemplate(t)
			target := root
			if testCase.malformedComponent {
				target = component
			}
			field := firstRequirementField(target)
			delete(field, "dcs:parameterName")
			delete(field, "dcs:domainField")
			attachImmediateComponent(root, component)

			findings := auditContractTemplateData(t, root)

			finding := requireTemplatePolicyFinding(t, findings, "FACIS-TPL-DATA-001", "error")
			require.True(t, strings.HasPrefix(finding.Path, testCase.expectedPathPrefix), finding.Path)
		})
	}
}

func TestAuditTemplatePoliciesAuditsEffectiveComponentPolicyAndDomainContent(t *testing.T) {
	root := decodedCanonicalTemplate(t)
	component := decodedCanonicalTemplate(t)
	componentField := firstRequirementField(component)
	componentField["dcs:domainField"] = map[string]any{"@id": "https://example.invalid/ontology#unknown-field"}
	componentPolicy := firstPolicyDuty(component)
	componentPolicy["odrl:constraint"].(map[string]any)["odrl:leftOperand"] = map[string]any{"@id": "urn:uuid:missing-field"}
	attachImmediateComponent(root, component)

	findings := auditContractTemplateData(t, root)

	for _, ruleID := range []string{"FACIS-TPL-POLICY-001", "FACIS-TPL-DOMAIN-001"} {
		finding := requireTemplatePolicyFinding(t, findings, ruleID, "error")
		require.True(t, strings.HasPrefix(finding.Path, "dcs:metadata.dcs:subTemplates[0].dcs:template."), finding.Path)
	}
}

func TestAuditTemplatePoliciesAggregatesRequiredFieldsFromImmediateComponent(t *testing.T) {
	root := decodedCanonicalTemplate(t)
	root["dcs:contractData"] = []any{}
	root["dcs:policies"] = []any{}
	component := decodedCanonicalTemplate(t)
	requirement := component["dcs:contractData"].([]any)[0].(map[string]any)
	requirement["dcs:fields"] = []any{
		canonicalRequirementField("jurisdiction"),
		canonicalRequirementField("country"),
		canonicalRequirementField("signature-level"),
	}
	firstPolicyDuty(component)["odrl:constraint"].(map[string]any)["odrl:leftOperand"] = map[string]any{"@id": "urn:uuid:field-country"}
	attachImmediateComponent(root, component)

	findings := auditContractTemplateData(t, root)

	ruleIDs := policyFindingRuleIDs(findings)
	for _, ruleID := range []string{
		"FACIS-TPL-POLICY-001",
		"FACIS-TPL-CONSTRAINT-001",
		"FACIS-TPL-LEGAL-001",
		"FACIS-TPL-PARTY-001",
		"FACIS-TPL-SIGN-001",
	} {
		require.NotContains(t, ruleIDs, ruleID)
	}
}

func TestAuditTemplatePoliciesKeepsStructureMetadataAndLifecycleRootScoped(t *testing.T) {
	root := decodedCanonicalTemplate(t)
	component := decodedCanonicalTemplate(t)
	component["dcs:documentStructure"].(map[string]any)["dcs:layout"] = []any{}
	delete(component, "dcs:metadata")
	component["state"] = "DRAFT"
	attachImmediateComponent(root, component)

	findings := auditContractTemplateData(t, root)

	ruleIDs := policyFindingRuleIDs(findings)
	require.NotContains(t, ruleIDs, "FACIS-TPL-STRUCT-001")
	require.NotContains(t, ruleIDs, "FACIS-TPL-AUDIT-001")
	require.NotContains(t, ruleIDs, "FACIS-TPL-LIFECYCLE-001")
}

func TestAuditTemplatePoliciesUsesOnlyImmutableImmediateSnapshots(t *testing.T) {
	root := decodedCanonicalTemplate(t)
	component := decodedCanonicalTemplate(t)
	nested := decodedCanonicalTemplate(t)
	firstRequirementField(nested)["dcs:domainField"] = map[string]any{"@id": "https://example.invalid/ontology#unknown-field"}
	attachImmediateComponent(component, nested)
	attachImmediateComponent(root, component)
	before, err := json.Marshal(root)
	require.NoError(t, err)

	findings := auditContractTemplateData(t, root)
	after, err := json.Marshal(root)
	require.NoError(t, err)

	require.NotContains(t, policyFindingRuleIDs(findings), "FACIS-TPL-DOMAIN-001")
	require.JSONEq(t, string(before), string(after))
}

func canonicalRequirementField(id string) map[string]any {
	ontologyIRIs := map[string]string{
		"jurisdiction":    "https://w3id.org/facis/dcs/taxonomy/v1#field-contract-jurisdiction",
		"country":         "https://w3id.org/facis/dcs/taxonomy/v1#field-company-location-country",
		"signature-level": "https://w3id.org/facis/dcs/taxonomy/v1#field-signature-requiredLevel",
	}
	return map[string]any{
		"@id":               "urn:uuid:field-" + id,
		"@type":             "dcs:RequirementField",
		"dcs:parameterName": id,
		"dcs:domainField":   map[string]any{"@id": ontologyIRIs[id]},
		"dcs:required":      true,
	}
}

func decodedCanonicalTemplate(t *testing.T) map[string]any {
	t.Helper()
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(*canonicalTemplateData(t), &decoded))
	return decoded
}

func attachImmediateComponent(root map[string]any, component map[string]any) {
	metadata := root["dcs:metadata"].(map[string]any)
	metadata["dcs:subTemplates"] = []any{
		map[string]any{
			"@id":          "did:example:component",
			"dcs:version":  1,
			"dcs:template": component,
		},
	}
}

func firstRequirementField(data map[string]any) map[string]any {
	requirement := data["dcs:contractData"].([]any)[0].(map[string]any)
	return requirement["dcs:fields"].([]any)[0].(map[string]any)
}

func unbindFirstClause(data map[string]any) {
	structure := data["dcs:documentStructure"].(map[string]any)
	blocks := structure["dcs:blocks"].(map[string]any)["@list"].([]any)
	clause := blocks[0].(map[string]any)
	clause["dcs:content"] = map[string]any{"@list": []any{"No binding"}}
}

func auditContractTemplateData(t *testing.T, data map[string]any) []PolicyFinding {
	t.Helper()
	raw, err := datatype.NewJSON(data)
	require.NoError(t, err)
	findings, err := AuditTemplatePolicies(&raw, TemplatePolicyAuditMetadata{
		DID:          "did:example:template",
		TemplateType: "CONTRACT_TEMPLATE",
		State:        "APPROVED",
	})
	require.NoError(t, err)
	return findings
}

func requireTemplatePolicyFinding(t *testing.T, findings []PolicyFinding, ruleID string, severity string) PolicyFinding {
	t.Helper()
	for _, finding := range findings {
		if finding.RuleID == ruleID && finding.Severity == severity {
			return finding
		}
	}
	require.FailNow(t, "required policy finding missing", "rule=%s severity=%s findings=%+v", ruleID, severity, findings)
	return PolicyFinding{}
}

func policyFindingRuleIDs(findings []PolicyFinding) []string {
	ruleIDs := make([]string, len(findings))
	for i, finding := range findings {
		ruleIDs[i] = finding.RuleID
	}
	return ruleIDs
}
