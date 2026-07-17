package validation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"digital-contracting-service/internal/base/datatype"
)

const defaultTemplatePolicyFile = "docs/policies/facis-template-audit-policies.json"

type TemplatePolicyAuditMetadata struct {
	DID          string
	TemplateType string
	State        string
}

type PolicyFinding struct {
	PolicySetID    string `json:"policySetId"`
	PolicyVersion  string `json:"policyVersion"`
	RuleID         string `json:"ruleId"`
	Title          string `json:"title"`
	Severity       string `json:"severity"`
	Message        string `json:"message"`
	Path           string `json:"path,omitempty"`
	FieldIri       string `json:"fieldIri,omitempty"`
	OntologyTerm   string `json:"ontologyTerm,omitempty"`
	Requirement    string `json:"requirement,omitempty"`
	ActualValue    any    `json:"actualValue,omitempty"`
	ExpectedValue  any    `json:"expectedValue,omitempty"`
	ExpectedValues []any  `json:"expectedValues,omitempty"`
	Operator       string `json:"operator,omitempty"`
	// ShapesVersion is the Semantic Hub SHACL shapes version (kind="shapes")
	// this finding was produced against — set only for findings produced by
	// validateAgainstHubShapes (shaclengine.go, ADR-8/ADR-9); zero for
	// findings from other audit sources (ODRL, the SLA validation profile).
	ShapesVersion int `json:"shapesVersion,omitempty"`
}

type templatePolicySet struct {
	PolicySetID string               `json:"policySetId"`
	Version     string               `json:"version"`
	Ontology    string               `json:"ontology"`
	Rules       []templatePolicyRule `json:"rules"`
}

type templatePolicyRule struct {
	ID                     string         `json:"id"`
	Title                  string         `json:"title"`
	Severity               string         `json:"severity"`
	Builtin                string         `json:"builtin"`
	OntologyTerm           string         `json:"ontologyTerm"`
	AppliesToTemplateTypes []string       `json:"appliesToTemplateTypes"`
	Parameters             map[string]any `json:"parameters"`
}

type templatePolicyContentSource struct {
	data       documentData
	pathPrefix string
}

type templatePolicyEvaluation struct {
	root             documentData
	effectiveContent []templatePolicyContentSource
}

func AuditTemplatePolicies(raw *datatype.JSON, metadata TemplatePolicyAuditMetadata) ([]PolicyFinding, error) {
	data, err := decodeDocumentData(raw)
	if err != nil {
		return []PolicyFinding{templateStructureDecodeFinding(err)}, nil
	}
	policySet, err := loadTemplatePolicySet()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ontology, err := requireDomainOntology(ctx)
	if err != nil {
		return nil, err
	}

	findings := []PolicyFinding{}
	if !isCanonicalEnvelope(data) {
		findings = append(findings, templateStructureFinding(policySet, "template data must use the canonical dcs:documentStructure envelope"))
	}
	evaluation := newTemplatePolicyEvaluation(data, metadata.TemplateType)
	for _, rule := range policySet.Rules {
		if !templatePolicyRuleApplies(rule, metadata.TemplateType) {
			continue
		}
		findings = append(findings, evaluateTemplatePolicyRule(policySet, rule, evaluation, metadata, ontology)...)
	}
	return findings, nil
}

func newTemplatePolicyEvaluation(root documentData, templateType string) templatePolicyEvaluation {
	evaluation := templatePolicyEvaluation{
		root:             root,
		effectiveContent: []templatePolicyContentSource{{data: root}},
	}
	if normalizeTemplatePolicyType(templateType) != "CONTRACT_TEMPLATE" {
		return evaluation
	}
	metadata, _ := topLevelValue(root, "metadata").(map[string]any)
	rawSnapshots, ok := metadata["dcs:subTemplates"]
	if !ok {
		rawSnapshots = metadata["subTemplates"]
	}
	snapshots, ok := asArray(rawSnapshots)
	if !ok {
		return evaluation
	}
	for index, rawSnapshot := range snapshots {
		snapshot, ok := rawSnapshot.(map[string]any)
		if !ok {
			continue
		}
		rawTemplate, ok := snapshot["dcs:template"]
		if !ok {
			rawTemplate = snapshot["template"]
		}
		template, ok := rawTemplate.(map[string]any)
		if !ok {
			continue
		}
		evaluation.effectiveContent = append(evaluation.effectiveContent, templatePolicyContentSource{
			data:       documentData(template),
			pathPrefix: fmt.Sprintf("dcs:metadata.dcs:subTemplates[%d].dcs:template", index),
		})
	}
	return evaluation
}

func loadTemplatePolicySet() (*templatePolicySet, error) {
	path, err := resolveTemplatePolicyFile()
	if err != nil {
		return nil, err
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read template policy file %q: %w", path, err)
	}
	var policySet templatePolicySet
	if err := json.Unmarshal(bytes, &policySet); err != nil {
		return nil, fmt.Errorf("decode template policy file %q: %w", path, err)
	}
	if strings.TrimSpace(policySet.PolicySetID) == "" || strings.TrimSpace(policySet.Version) == "" {
		return nil, fmt.Errorf("template policy file %q requires policySetId and version", path)
	}
	return &policySet, nil
}

func resolveTemplatePolicyFile() (string, error) {
	if path := strings.TrimSpace(os.Getenv("FACIS_TEMPLATE_POLICY_FILE")); path != "" {
		return path, nil
	}
	candidates := []string{
		defaultTemplatePolicyFile,
		filepath.Join("..", defaultTemplatePolicyFile),
		filepath.Join("..", "..", defaultTemplatePolicyFile),
		filepath.Join("..", "..", "..", defaultTemplatePolicyFile),
		filepath.Join("..", "..", "..", "..", defaultTemplatePolicyFile),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", errors.New("template policy file not found")
}

func evaluateTemplatePolicyRule(policySet *templatePolicySet, rule templatePolicyRule, evaluation templatePolicyEvaluation, metadata TemplatePolicyAuditMetadata, ontology *domainOntology) []PolicyFinding {
	switch rule.Builtin {
	case "canonical_document_structure":
		return auditCanonicalDocumentStructure(policySet, rule, evaluation.root)
	case "canonical_contract_data":
		return auditCompositionContractData(policySet, rule, evaluation.effectiveContent)
	case "canonical_policy_operands":
		return auditCompositionPolicyOperands(policySet, rule, evaluation.effectiveContent)
	case "component_document_structure_integrity":
		return auditComponentDocumentStructureIntegrity(policySet, rule, evaluation.root)
	case "component_contract_data_integrity":
		return auditComponentContractDataIntegrity(policySet, rule, evaluation.root)
	case "component_policy_operand_integrity":
		return auditComponentPolicyOperandIntegrity(policySet, rule, evaluation.root)
	case "finished_template_has_clause_semantics":
		return auditCompositionHasClauseSemantics(policySet, rule, evaluation.effectiveContent)
	case "finished_template_state":
		return auditFinishedTemplateState(policySet, rule, metadata)
	case "canonical_domain_fields":
		return auditCompositionDomainFields(policySet, rule, evaluation.effectiveContent, ontology)
	case "constrained_parameters_use_value_constraint":
		return auditCompositionConstrainedParameters(policySet, rule, evaluation.effectiveContent, ontology)
	case "required_domain_fields":
		return auditCompositionRequiredDomainFields(policySet, rule, evaluation.effectiveContent, ontology)
	case "audit_metadata_complete":
		return auditMetadataComplete(policySet, rule, evaluation.root, metadata)
	default:
		return []PolicyFinding{newPolicyFinding(policySet, rule, "Unsupported policy builtin "+rule.Builtin, "rules."+rule.ID, "")}
	}
}

func templatePolicyRuleApplies(rule templatePolicyRule, templateType string) bool {
	if len(rule.AppliesToTemplateTypes) == 0 {
		return true
	}
	normalizedTemplateType := normalizeTemplatePolicyType(templateType)
	for _, candidate := range rule.AppliesToTemplateTypes {
		if normalizeTemplatePolicyType(candidate) == normalizedTemplateType {
			return true
		}
	}
	return false
}

func normalizeTemplatePolicyType(templateType string) string {
	normalized := strings.ToUpper(strings.TrimSpace(templateType))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	return normalized
}

func auditCanonicalDocumentStructure(policySet *templatePolicySet, rule templatePolicyRule, data documentData) []PolicyFinding {
	findings := []PolicyFinding{}
	structure, ok := topLevelValue(data, "documentStructure").(map[string]any)
	if !ok {
		return []PolicyFinding{newPolicyFinding(policySet, rule, "dcs:documentStructure must be an object", "dcs:documentStructure", "")}
	}
	blocks, ok := canonicalBlocks(structure)
	if !ok || len(blocks) == 0 {
		findings = append(findings, newPolicyFinding(policySet, rule, "dcs:documentStructure.dcs:blocks must contain at least one block", "dcs:documentStructure.dcs:blocks", ""))
	}
	layout, ok := structure["dcs:layout"].([]any)
	if !ok || len(layout) == 0 {
		findings = append(findings, newPolicyFinding(policySet, rule, "dcs:documentStructure.dcs:layout must contain at least one layout node", "dcs:documentStructure.dcs:layout", ""))
	}
	blockIDs := map[string]bool{}
	for index, rawBlock := range blocks {
		block, ok := rawBlock.(map[string]any)
		if !ok {
			findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("dcs:blocks item %d must be an object", index), "dcs:documentStructure.dcs:blocks", ""))
			continue
		}
		blockID, _ := block["@id"].(string)
		if strings.TrimSpace(blockID) == "" {
			findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("dcs:blocks item %d requires @id", index), "dcs:documentStructure.dcs:blocks", ""))
			continue
		}
		blockIDs[blockID] = true
		if strings.TrimSpace(stringMapValue(block, "@type")) == "" {
			findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("document block %q requires @type", blockID), blockID, ""))
		}
	}
	rootCount := 0
	for index, rawNode := range layout {
		node, ok := rawNode.(map[string]any)
		if !ok {
			findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("dcs:layout item %d must be an object", index), "dcs:documentStructure.dcs:layout", ""))
			continue
		}
		nodeID, _ := node["@id"].(string)
		if isTrueValue(node["dcs:isRoot"]) {
			rootCount++
		} else if strings.TrimSpace(nodeID) != "" && !blockIDs[nodeID] {
			findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("layout node %q references no declared block", nodeID), "dcs:documentStructure.dcs:layout", ""))
		}
		children, ok := jsonLDList(node["dcs:children"])
		if !ok {
			findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("layout node %q must declare dcs:children as @list", nodeID), "dcs:documentStructure.dcs:layout.dcs:children", ""))
			continue
		}
		for _, rawChild := range children {
			child, _ := rawChild.(map[string]any)
			childID, _ := child["@id"].(string)
			if !blockIDs[childID] {
				findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("layout references nonexistent block %q", childID), "dcs:documentStructure.dcs:layout.dcs:children", ""))
			}
		}
	}
	if rootCount != 1 {
		findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("dcs:layout must contain exactly one root node, got %d", rootCount), "dcs:documentStructure.dcs:layout", ""))
	}
	return findings
}

func auditComponentDocumentStructureIntegrity(policySet *templatePolicySet, rule templatePolicyRule, data documentData) []PolicyFinding {
	structure, ok := topLevelValue(data, "documentStructure").(map[string]any)
	if !ok {
		return nil
	}
	return auditDocumentStructureIntegrity(policySet, rule, structure, false)
}

func auditComponentContractDataIntegrity(policySet *templatePolicySet, rule templatePolicyRule, data documentData) []PolicyFinding {
	if _, exists := topLevelValueExists(data, "contractData"); !exists {
		return nil
	}
	return auditContractDataIntegrity(policySet, rule, data, false)
}

func auditComponentPolicyOperandIntegrity(policySet *templatePolicySet, rule templatePolicyRule, data documentData) []PolicyFinding {
	if _, exists := topLevelValueExists(data, "policies"); !exists {
		return nil
	}
	return auditPolicyOperandIntegrity(policySet, rule, data, false)
}

func auditCanonicalContractData(policySet *templatePolicySet, rule templatePolicyRule, data documentData) []PolicyFinding {
	return auditContractDataIntegrity(policySet, rule, data, true)
}

func auditCompositionContractData(policySet *templatePolicySet, rule templatePolicyRule, sources []templatePolicyContentSource) []PolicyFinding {
	if len(sources) == 1 {
		return auditCanonicalContractData(policySet, rule, sources[0].data)
	}
	findings := []PolicyFinding{}
	fieldIDs := map[string]bool{}
	for _, source := range sources {
		if _, exists := topLevelValueExists(source.data, "contractData"); !exists {
			continue
		}
		findings = append(findings, prefixPolicyFindings(
			auditContractDataIntegrity(policySet, rule, source.data, false), source.pathPrefix)...)
		for fieldID := range canonicalContractDataFieldIDs(source.data) {
			fieldIDs[fieldID] = true
		}
	}
	if len(fieldIDs) == 0 {
		findings = append(findings, newPolicyFinding(policySet, rule, "dcs:contractData must contain at least one dcs:DataRequirement with a requirement field @id", "dcs:contractData", ""))
	}
	return findings
}

func auditContractDataIntegrity(policySet *templatePolicySet, rule templatePolicyRule, data documentData, requireAtLeastOne bool) []PolicyFinding {
	requirements, ok := topLevelValue(data, "contractData").([]any)
	if !ok || len(requirements) == 0 {
		if !requireAtLeastOne && ok {
			return nil
		}
		return []PolicyFinding{newPolicyFinding(policySet, rule, "dcs:contractData must contain at least one dcs:DataRequirement", "dcs:contractData", "")}
	}
	findings := []PolicyFinding{}
	fieldIDs := map[string]bool{}
	for reqIndex, rawRequirement := range requirements {
		requirement, ok := rawRequirement.(map[string]any)
		if !ok {
			findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("dcs:contractData item %d must be an object", reqIndex), "dcs:contractData", ""))
			continue
		}
		conditionID, _ := requirement["dcs:conditionId"].(string)
		if strings.TrimSpace(conditionID) == "" {
			findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("dcs:contractData item %d requires dcs:conditionId", reqIndex), "dcs:contractData.dcs:conditionId", ""))
		}
		fields, ok := requirement["dcs:fields"].([]any)
		if !ok || len(fields) == 0 {
			findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("data requirement %q must contain at least one dcs:RequirementField", conditionID), "dcs:contractData.dcs:fields", ""))
			continue
		}
		for fieldIndex, rawField := range fields {
			field, ok := rawField.(map[string]any)
			if !ok {
				findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("field %d in requirement %q must be an object", fieldIndex, conditionID), "dcs:contractData.dcs:fields", ""))
				continue
			}
			fieldID, _ := field["@id"].(string)
			if strings.TrimSpace(fieldID) == "" {
				findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("field %d in requirement %q requires @id", fieldIndex, conditionID), "dcs:contractData.dcs:fields.@id", ""))
			} else {
				fieldIDs[fieldID] = true
			}
			if strings.TrimSpace(stringMapValue(field, "dcs:parameterName")) == "" {
				findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("field %q requires dcs:parameterName", fieldID), "dcs:contractData.dcs:fields.dcs:parameterName", ""))
			}
			domainField, ok := field["dcs:domainField"].(map[string]any)
			domainFieldID, _ := domainField["@id"].(string)
			if !ok || strings.TrimSpace(domainFieldID) == "" {
				findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("field %q requires dcs:domainField @id", fieldID), "dcs:contractData.dcs:fields.dcs:domainField", ""))
			}
		}
	}
	if len(fieldIDs) == 0 {
		findings = append(findings, newPolicyFinding(policySet, rule, "dcs:contractData must declare at least one requirement field @id", "dcs:contractData.dcs:fields", ""))
	}
	return findings
}

func auditCanonicalPolicyOperands(policySet *templatePolicySet, rule templatePolicyRule, data documentData) []PolicyFinding {
	return auditPolicyOperandIntegrity(policySet, rule, data, true)
}

func auditCompositionPolicyOperands(policySet *templatePolicySet, rule templatePolicyRule, sources []templatePolicyContentSource) []PolicyFinding {
	if len(sources) == 1 {
		return auditCanonicalPolicyOperands(policySet, rule, sources[0].data)
	}
	fieldIDs := compositionContractDataFieldIDs(sources)
	findings := []PolicyFinding{}
	hasPolicies := false
	for _, source := range sources {
		if _, exists := topLevelValueExists(source.data, "policies"); !exists {
			continue
		}
		hasPolicies = true
		findings = append(findings, prefixPolicyFindings(
			auditPolicyOperandIntegrityWithFieldIDs(policySet, rule, source.data, false, fieldIDs), source.pathPrefix)...)
	}
	if !hasPolicies {
		findings = append(findings, newPolicyFinding(policySet, rule, "dcs:policies must be an odrl:Set object (or an empty array)", "dcs:policies", ""))
	}
	return findings
}

func auditPolicyOperandIntegrity(policySet *templatePolicySet, rule templatePolicyRule, data documentData, requirePoliciesArray bool) []PolicyFinding {
	return auditPolicyOperandIntegrityWithFieldIDs(policySet, rule, data, requirePoliciesArray, canonicalContractDataFieldIDs(data))
}

func auditPolicyOperandIntegrityWithFieldIDs(policySet *templatePolicySet, rule templatePolicyRule, data documentData, requirePoliciesArray bool, fieldIDs map[string]bool) []PolicyFinding {
	rawPolicies, exists := topLevelValueExists(data, "policies")
	if !exists {
		if !requirePoliciesArray {
			return nil
		}
		return []PolicyFinding{newPolicyFinding(policySet, rule, "dcs:policies must be an odrl:Set object (or an empty array)", "dcs:policies", "")}
	}
	// dcs:policies is the canonical enclosing odrl:Set (or an empty array);
	// collectODRLPolicyRules flattens its rule buckets for this advisory
	// audit the same way the enforcement path does.
	policies := collectODRLPolicyRules(rawPolicies)
	findings := []PolicyFinding{}
	for _, policy := range policies {
		policyID, _ := policy["@id"].(string)
		constraints := policyConstraints(policy["odrl:constraint"])
		if len(constraints) == 0 {
			findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("policy %q requires odrl:constraint", policyID), "dcs:policies.odrl:constraint", ""))
			continue
		}
		for _, constraint := range constraints {
			leftOperand, ok := constraint["odrl:leftOperand"].(map[string]any)
			fieldID, _ := leftOperand["@id"].(string)
			if !ok || strings.TrimSpace(fieldID) == "" {
				findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("policy %q requires odrl:leftOperand @id", policyID), "dcs:policies.odrl:leftOperand", ""))
				continue
			}
			// A context operand (spatial, dateTime, …) is use-time access
			// context, not a document data field.
			if !isODRLContextOperandTerm(fieldID) && !fieldIDs[fieldID] {
				findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("policy %q references nonexistent contract data field %q", policyID, fieldID), fieldID, fieldID))
			}
			if _, ok := constraint["odrl:operator"].(map[string]any); !ok {
				findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("policy %q requires odrl:operator @id", policyID), "dcs:policies.odrl:operator", ""))
			}
		}
	}
	return findings
}

func auditDocumentStructureIntegrity(policySet *templatePolicySet, rule templatePolicyRule, structure map[string]any, requireCompleteLayout bool) []PolicyFinding {
	findings := []PolicyFinding{}
	blocks, ok := canonicalBlocks(structure)
	if !ok || len(blocks) == 0 {
		if requireCompleteLayout {
			findings = append(findings, newPolicyFinding(policySet, rule, "dcs:documentStructure.dcs:blocks must contain at least one block", "dcs:documentStructure.dcs:blocks", ""))
		}
		return findings
	}
	blockIDs := map[string]bool{}
	for index, rawBlock := range blocks {
		block, ok := rawBlock.(map[string]any)
		if !ok {
			findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("dcs:blocks item %d must be an object", index), "dcs:documentStructure.dcs:blocks", ""))
			continue
		}
		blockID, _ := block["@id"].(string)
		if strings.TrimSpace(blockID) == "" {
			findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("dcs:blocks item %d requires @id", index), "dcs:documentStructure.dcs:blocks", ""))
			continue
		}
		if blockIDs[blockID] {
			findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("duplicate document block @id %q", blockID), "dcs:documentStructure.dcs:blocks.@id", blockID))
		}
		blockIDs[blockID] = true
		if strings.TrimSpace(stringMapValue(block, "@type")) == "" {
			findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("document block %q requires @type", blockID), blockID, ""))
		}
	}
	layout, ok := structure["dcs:layout"].([]any)
	if !ok {
		if requireCompleteLayout {
			findings = append(findings, newPolicyFinding(policySet, rule, "dcs:documentStructure.dcs:layout must contain at least one layout node", "dcs:documentStructure.dcs:layout", ""))
		}
		return findings
	}
	rootCount := 0
	for index, rawNode := range layout {
		node, ok := rawNode.(map[string]any)
		if !ok {
			findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("dcs:layout item %d must be an object", index), "dcs:documentStructure.dcs:layout", ""))
			continue
		}
		nodeID, _ := node["@id"].(string)
		if isTrueValue(node["dcs:isRoot"]) {
			rootCount++
		} else if strings.TrimSpace(nodeID) != "" && !blockIDs[nodeID] {
			findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("layout node %q references no declared block", nodeID), "dcs:documentStructure.dcs:layout", ""))
		}
		children, ok := jsonLDList(node["dcs:children"])
		if !ok {
			findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("layout node %q must declare dcs:children as @list", nodeID), "dcs:documentStructure.dcs:layout.dcs:children", ""))
			continue
		}
		for _, rawChild := range children {
			child, _ := rawChild.(map[string]any)
			childID, _ := child["@id"].(string)
			if !blockIDs[childID] {
				findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("layout references nonexistent block %q", childID), "dcs:documentStructure.dcs:layout.dcs:children", ""))
			}
		}
	}
	if requireCompleteLayout && rootCount != 1 {
		findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("dcs:layout must contain exactly one root node, got %d", rootCount), "dcs:documentStructure.dcs:layout", ""))
	}
	return findings
}

func auditFinishedTemplateHasClauseSemantics(policySet *templatePolicySet, rule templatePolicyRule, data documentData) []PolicyFinding {
	structure, _ := topLevelValue(data, "documentStructure").(map[string]any)
	blocks, _ := canonicalBlocks(structure)
	fieldIDs := canonicalContractDataFieldIDs(data)
	for _, rawBlock := range blocks {
		block, ok := rawBlock.(map[string]any)
		if !ok || block["@type"] != "dcs:Clause" {
			continue
		}
		if clauseHasContractDataBinding(block, fieldIDs) {
			return nil
		}
	}
	return []PolicyFinding{newPolicyFinding(policySet, rule, "finished templates should contain at least one dcs:Clause with a placeholder bound to dcs:contractData", "dcs:documentStructure.dcs:blocks", "")}
}

func auditCompositionHasClauseSemantics(policySet *templatePolicySet, rule templatePolicyRule, sources []templatePolicyContentSource) []PolicyFinding {
	if len(sources) == 1 {
		return auditFinishedTemplateHasClauseSemantics(policySet, rule, sources[0].data)
	}
	fieldIDs := compositionContractDataFieldIDs(sources)
	for _, source := range sources {
		structure, _ := topLevelValue(source.data, "documentStructure").(map[string]any)
		blocks, _ := canonicalBlocks(structure)
		for _, rawBlock := range blocks {
			block, ok := rawBlock.(map[string]any)
			if ok && block["@type"] == "dcs:Clause" && clauseHasContractDataBinding(block, fieldIDs) {
				return nil
			}
		}
	}
	return []PolicyFinding{newPolicyFinding(policySet, rule, "finished templates should contain at least one dcs:Clause with a placeholder bound to dcs:contractData", "dcs:documentStructure.dcs:blocks", "")}
}

func auditFinishedTemplateState(policySet *templatePolicySet, rule templatePolicyRule, metadata TemplatePolicyAuditMetadata) []PolicyFinding {
	states := stringSliceParameter(rule.Parameters, "states")
	if len(states) == 0 || containsString(states, metadata.State) {
		return nil
	}
	return []PolicyFinding{newPolicyFinding(policySet, rule, fmt.Sprintf("template state %q is not one of the finished states: %s", metadata.State, strings.Join(states, ", ")), "state", "")}
}

func auditCanonicalDomainFields(policySet *templatePolicySet, rule templatePolicyRule, data documentData, ontology *domainOntology) []PolicyFinding {
	findings := []PolicyFinding{}
	forEachSemanticParameter(data, ontology, func(conditionID string, index int, param map[string]any) {
		fieldIri, _ := param["fieldIri"].(string)
		if _, ok := ontology.fields[fieldIri]; !ok {
			findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("semantic condition %q uses unknown domain field %q", conditionID, fieldIri), fmt.Sprintf("semanticConditions.%s.parameters.%d", conditionID, index), fieldIri))
		}
	})
	return findings
}

func auditCompositionDomainFields(policySet *templatePolicySet, rule templatePolicyRule, sources []templatePolicyContentSource, ontology *domainOntology) []PolicyFinding {
	findings := []PolicyFinding{}
	for _, source := range sources {
		findings = append(findings, prefixPolicyFindings(
			auditCanonicalDomainFields(policySet, rule, source.data, ontology), source.pathPrefix)...)
	}
	return findings
}

func auditConstrainedParameters(policySet *templatePolicySet, rule templatePolicyRule, data documentData, ontology *domainOntology) []PolicyFinding {
	findings := []PolicyFinding{}
	forEachSemanticParameter(data, ontology, func(conditionID string, index int, param map[string]any) {
		fieldIri, _ := param["fieldIri"].(string)
		field, ok := ontology.fields[fieldIri]
		if !ok || field.Constraint == nil {
			return
		}
		if _, ok := param["valueConstraint"].(map[string]any); !ok {
			findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("semantic field %q requires valueConstraint metadata", fieldIri), fmt.Sprintf("semanticConditions.%s.parameters.%d.valueConstraint", conditionID, index), fieldIri))
		}
	})
	return findings
}

func auditCompositionConstrainedParameters(policySet *templatePolicySet, rule templatePolicyRule, sources []templatePolicyContentSource, ontology *domainOntology) []PolicyFinding {
	findings := []PolicyFinding{}
	for _, source := range sources {
		findings = append(findings, prefixPolicyFindings(
			auditConstrainedParameters(policySet, rule, source.data, ontology), source.pathPrefix)...)
	}
	return findings
}

func auditRequiredDomainFields(policySet *templatePolicySet, rule templatePolicyRule, data documentData, ontology *domainOntology) []PolicyFinding {
	required := stringSliceParameter(rule.Parameters, "fieldIris")
	if len(required) == 0 {
		return nil
	}
	seen := map[string]bool{}
	forEachSemanticParameter(data, ontology, func(_ string, _ int, param map[string]any) {
		fieldIri, _ := param["fieldIri"].(string)
		seen[fieldIri] = true
	})
	findings := []PolicyFinding{}
	for _, fieldIri := range required {
		if !seen[fieldIri] {
			findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("required semantic field %q is missing", fieldIri), "semanticConditions.parameters", fieldIri))
		}
	}
	return findings
}

func auditCompositionRequiredDomainFields(policySet *templatePolicySet, rule templatePolicyRule, sources []templatePolicyContentSource, ontology *domainOntology) []PolicyFinding {
	if len(sources) == 1 {
		return auditRequiredDomainFields(policySet, rule, sources[0].data, ontology)
	}
	required := stringSliceParameter(rule.Parameters, "fieldIris")
	seen := map[string]bool{}
	for _, source := range sources {
		forEachSemanticParameter(source.data, ontology, func(_ string, _ int, param map[string]any) {
			fieldIri, _ := param["fieldIri"].(string)
			seen[fieldIri] = true
		})
	}
	findings := []PolicyFinding{}
	for _, fieldIri := range required {
		if !seen[fieldIri] {
			findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("required semantic field %q is missing", fieldIri), "semanticConditions.parameters", fieldIri))
		}
	}
	return findings
}

func auditMetadataComplete(policySet *templatePolicySet, rule templatePolicyRule, data documentData, metadata TemplatePolicyAuditMetadata) []PolicyFinding {
	findings := []PolicyFinding{}
	if strings.TrimSpace(metadata.DID) == "" {
		findings = append(findings, newPolicyFinding(policySet, rule, "template DID is missing", "did", ""))
	}
	if strings.TrimSpace(metadata.State) == "" {
		findings = append(findings, newPolicyFinding(policySet, rule, "template state is missing", "state", ""))
	}
	templateType := normalizeTemplatePolicyType(metadata.TemplateType)
	if templateType != "CONTRACT_TEMPLATE" && templateType != "COMPONENT" {
		findings = append(findings, newPolicyFinding(policySet, rule, fmt.Sprintf("template type %q is not supported for audit", metadata.TemplateType), "template_type", ""))
	}
	if _, ok := topLevelValue(data, "metadata").(map[string]any); !ok {
		findings = append(findings, newPolicyFinding(policySet, rule, "template metadata is missing", "dcs:metadata", ""))
	}
	return findings
}

func forEachSemanticParameter(data documentData, ontology *domainOntology, visit func(conditionID string, index int, param map[string]any)) {
	requirements, _ := topLevelValue(data, "contractData").([]any)
	for _, rawRequirement := range requirements {
		requirement, ok := rawRequirement.(map[string]any)
		if !ok {
			continue
		}
		conditionID, _ := requirement["dcs:conditionId"].(string)
		fields, _ := requirement["dcs:fields"].([]any)
		for index, rawField := range fields {
			field, ok := rawField.(map[string]any)
			if !ok {
				continue
			}
			domainField, _ := field["dcs:domainField"].(map[string]any)
			domainFieldID, _ := domainField["@id"].(string)
			visit(conditionID, index, map[string]any{
				"fieldIri":        domainFieldID,
				"valueConstraint": domainFieldValueConstraint(ontology, domainFieldID),
			})
		}
	}
}

func canonicalBlocks(structure map[string]any) ([]any, bool) {
	blocks, ok := jsonLDList(structure["dcs:blocks"])
	if !ok {
		blocks, ok = structure["dcs:blocks"].([]any)
	}
	return blocks, ok
}

func canonicalContractDataFieldIDs(data documentData) map[string]bool {
	fieldIDs := map[string]bool{}
	requirements, _ := topLevelValue(data, "contractData").([]any)
	for _, rawRequirement := range requirements {
		requirement, ok := rawRequirement.(map[string]any)
		if !ok {
			continue
		}
		fields, _ := requirement["dcs:fields"].([]any)
		for _, rawField := range fields {
			field, ok := rawField.(map[string]any)
			if !ok {
				continue
			}
			fieldID, _ := field["@id"].(string)
			if strings.TrimSpace(fieldID) != "" {
				fieldIDs[fieldID] = true
			}
		}
	}
	return fieldIDs
}

func compositionContractDataFieldIDs(sources []templatePolicyContentSource) map[string]bool {
	fieldIDs := map[string]bool{}
	for _, source := range sources {
		for fieldID := range canonicalContractDataFieldIDs(source.data) {
			fieldIDs[fieldID] = true
		}
	}
	return fieldIDs
}

func prefixPolicyFindings(findings []PolicyFinding, prefix string) []PolicyFinding {
	if prefix == "" {
		return findings
	}
	for index := range findings {
		if findings[index].Path == "" {
			findings[index].Path = prefix
		} else {
			findings[index].Path = prefix + "." + findings[index].Path
		}
	}
	return findings
}

func clauseHasContractDataBinding(block map[string]any, fieldIDs map[string]bool) bool {
	content, ok := jsonLDList(block["dcs:content"])
	if !ok {
		return false
	}
	for _, rawSegment := range content {
		segment, ok := rawSegment.(map[string]any)
		if !ok || segment["@type"] != "dcs:Placeholder" {
			continue
		}
		bindsTo, _ := segment["dcs:bindsTo"].(map[string]any)
		fieldID, _ := bindsTo["@id"].(string)
		if fieldIDs[fieldID] {
			return true
		}
	}
	return false
}

func domainFieldValueConstraint(ontology *domainOntology, domainFieldID string) map[string]any {
	field, ok := ontology.fields[domainFieldID]
	if !ok || field.Constraint == nil {
		return nil
	}
	return field.Constraint.asMap()
}

func stringMapValue(values map[string]any, key string) string {
	value, ok := values[key]
	if !ok || value == nil {
		return ""
	}
	text, _ := value.(string)
	return text
}

func templateStructureDecodeFinding(err error) PolicyFinding {
	return PolicyFinding{
		PolicySetID:   "facis.dcs.template.audit",
		PolicyVersion: "v1",
		RuleID:        "FACIS-TPL-STRUCT-000",
		Title:         "Template data must be valid JSON",
		Severity:      "error",
		Message:       err.Error(),
		Path:          "template_data",
		OntologyTerm:  "dcs:DocumentStructure",
	}
}

func templateStructureFinding(policySet *templatePolicySet, message string) PolicyFinding {
	return PolicyFinding{
		PolicySetID:   policySet.PolicySetID,
		PolicyVersion: policySet.Version,
		RuleID:        "FACIS-TPL-STRUCT-000",
		Title:         "Template data must use canonical JSON-LD",
		Severity:      "error",
		Message:       message,
		Path:          "template_data",
		OntologyTerm:  "dcs:DocumentStructure",
	}
}

func stringSliceParameter(parameters map[string]any, key string) []string {
	if parameters == nil {
		return nil
	}
	raw, ok := parameters[key]
	if !ok {
		return nil
	}
	items, ok := asArray(raw)
	if !ok {
		return nil
	}
	values := []string{}
	for _, item := range items {
		value, ok := item.(string)
		if ok && strings.TrimSpace(value) != "" {
			values = append(values, value)
		}
	}
	return values
}

func newPolicyFinding(policySet *templatePolicySet, rule templatePolicyRule, message string, path string, fieldIri string) PolicyFinding {
	severity := rule.Severity
	if severity == "" {
		severity = "warning"
	}
	return PolicyFinding{
		PolicySetID:   policySet.PolicySetID,
		PolicyVersion: policySet.Version,
		RuleID:        rule.ID,
		Title:         rule.Title,
		Severity:      severity,
		Message:       message,
		Path:          path,
		FieldIri:      fieldIri,
		OntologyTerm:  rule.OntologyTerm,
	}
}
