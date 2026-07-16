package validation

import (
	"encoding/json"
	"strings"
)

// SignatureFieldDeclaration is one authoritative signing step declared by
// the contract document. Order is one-based and Dependency names the
// immediately preceding field, if any.
type SignatureFieldDeclaration struct {
	Name       string
	Order      int
	Dependency *string
}

// DeclaredSignatureFields returns unique signature fields in document order.
func DeclaredSignatureFields(contractData []byte) []SignatureFieldDeclaration {
	var doc struct {
		SignatureFields []struct {
			SignatoryName string `json:"signatoryName"`
		} `json:"signatureFields"`
	}
	if err := json.Unmarshal(contractData, &doc); err != nil {
		return nil
	}
	fields := make([]SignatureFieldDeclaration, 0, len(doc.SignatureFields))
	seen := map[string]bool{}
	for _, sf := range doc.SignatureFields {
		name := strings.TrimSpace(sf.SignatoryName)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		declaration := SignatureFieldDeclaration{Name: name, Order: len(fields) + 1}
		if len(fields) > 0 {
			dependency := fields[len(fields)-1].Name
			declaration.Dependency = &dependency
		}
		fields = append(fields, declaration)
	}
	return fields
}

// RequiredSignatureFields returns the contract's declared signature-field
// names (dcs:SignatureField nodes' signatoryName, the AcroForm field name
// pdf-core renders and /sign targets — see pdf-core/compiler/dcs_schema.go).
// An empty result means the contract declares no explicit signature fields
// and follows the single-signature flow (DCS-FR-SM-07/-17: contracts that
// require multiple signatures declare one field per signatory).
func RequiredSignatureFields(contractData []byte) []string {
	declarations := DeclaredSignatureFields(contractData)
	if declarations == nil {
		return nil
	}
	fields := make([]string, 0, len(declarations))
	for _, declaration := range declarations {
		fields = append(fields, declaration.Name)
	}
	return fields
}
