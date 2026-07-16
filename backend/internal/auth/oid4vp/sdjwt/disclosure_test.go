package sdjwt

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func encodedDisclosure(t *testing.T, value []any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func TestVerifyDisclosuresAcceptsNestedDisclosureGraphInWalletOrder(t *testing.T) {
	nested := encodedDisclosure(t, []any{"nested-salt", "street_address", "Main Street 1"})
	container := encodedDisclosure(t, []any{"container-salt", "address", map[string]any{
		"_sd": []any{disclosureDigest(nested)},
	}})
	claims := jwt.MapClaims{"_sd": []any{disclosureDigest(container)}, "_sd_alg": defaultSDAlg}

	if err := VerifyDisclosures(claims, []string{nested, container}); err != nil {
		t.Fatalf("expected nested disclosure graph to verify: %v", err)
	}
}

func TestVerifyDisclosuresRejectsUnanchoredNestedDisclosure(t *testing.T) {
	unanchored := encodedDisclosure(t, []any{"salt", "street_address", "Main Street 1"})
	root := encodedDisclosure(t, []any{"root-salt", "given_name", "Alice"})
	claims := jwt.MapClaims{"_sd": []any{disclosureDigest(root)}, "_sd_alg": defaultSDAlg}

	if err := VerifyDisclosures(claims, []string{unanchored, root}); err == nil {
		t.Fatal("expected unanchored disclosure to be rejected")
	}
}
