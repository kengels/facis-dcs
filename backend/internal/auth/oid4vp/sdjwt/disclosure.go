package sdjwt

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

const defaultSDAlg = "sha-256"

// MergeDisclosedClaims merges selectively disclosed claims into issuer-signed payload claims.
func MergeDisclosedClaims(issuerClaims jwt.MapClaims, disclosures []string) (jwt.MapClaims, error) {
	out := make(jwt.MapClaims, len(issuerClaims)+len(disclosures))

	for k, v := range issuerClaims {
		out[k] = v
	}

	delete(out, "_sd")
	delete(out, "_sd_alg")

	for _, encoded := range disclosures {
		arr, err := decodeDisclosure(encoded)
		if err != nil {
			return nil, err
		}
		claimName, ok := arr[1].(string)
		if !ok || strings.TrimSpace(claimName) == "" {
			return nil, fmt.Errorf("disclosure claim name must be a non-empty string")
		}
		out[claimName] = arr[2]
	}

	return out, nil
}

// VerifyDisclosures checks that every disclosure is anchored either directly
// in the credential _sd array or in a nested _sd array exposed by another
// anchored disclosure. Wallets may send nested disclosures before their
// containing disclosure, so verification resolves the graph to a fixed point.
func VerifyDisclosures(issuerClaims jwt.MapClaims, disclosures []string) error {
	sdAlg, _ := issuerClaims["_sd_alg"].(string)

	if strings.TrimSpace(sdAlg) == "" {
		sdAlg = defaultSDAlg
	}

	if sdAlg != defaultSDAlg {
		return fmt.Errorf("unsupported _sd_alg %q", sdAlg)
	}

	rawSD, ok := issuerClaims["_sd"]

	if !ok {
		return fmt.Errorf("credential missing _sd")
	}

	sdHashes, err := stringSliceFromAny(rawSD)

	if err != nil {
		return err
	}

	if len(sdHashes) == 0 {
		return fmt.Errorf("credential _sd is empty")
	}

	available := make(map[string]struct{}, len(sdHashes))
	for _, digest := range sdHashes {
		available[digest] = struct{}{}
	}
	pending := make(map[string]string, len(disclosures))
	for _, encoded := range disclosures {
		digest := disclosureDigest(encoded)
		if _, dup := pending[digest]; dup {
			return fmt.Errorf("duplicate disclosure digest")
		}
		pending[digest] = encoded
	}

	for len(pending) > 0 {
		progress := false
		for digest, encoded := range pending {
			if _, ok := available[digest]; !ok {
				continue
			}
			decoded, err := decodeDisclosure(encoded)
			if err != nil {
				return err
			}
			collectNestedSDHashes(decoded[2], available)
			delete(pending, digest)
			progress = true
		}
		if !progress {
			return fmt.Errorf("disclosure digest is not listed in credential _sd")
		}
	}

	return nil
}

func collectNestedSDHashes(value any, hashes map[string]struct{}) {
	switch typed := value.(type) {
	case map[string]any:
		if raw, ok := typed["_sd"]; ok {
			if nested, err := stringSliceFromAny(raw); err == nil {
				for _, digest := range nested {
					hashes[digest] = struct{}{}
				}
			}
		}
		for key, child := range typed {
			if key != "_sd" {
				collectNestedSDHashes(child, hashes)
			}
		}
	case []any:
		for _, child := range typed {
			collectNestedSDHashes(child, hashes)
		}
	}
}

func disclosureDigest(encodedDisclosure string) string {
	return sha256Base64URL(encodedDisclosure)
}

func decodeDisclosure(encoded string) ([]any, error) {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)

	if err != nil {
		return nil, fmt.Errorf("decode disclosure: %w", err)
	}

	var arr []any
	err = json.Unmarshal(raw, &arr)

	if err != nil {
		return nil, fmt.Errorf("parse disclosure json: %w", err)
	}

	if len(arr) != 3 {
		return nil, fmt.Errorf("property disclosure must be a three-element array")
	}

	return arr, nil
}

func stringSliceFromAny(raw any) ([]string, error) {
	arr, ok := raw.([]any)

	if !ok {
		return nil, fmt.Errorf("expected json array")
	}
	out := make([]string, 0, len(arr))

	for _, item := range arr {
		s, ok := item.(string)
		if !ok || strings.TrimSpace(s) == "" {
			return nil, fmt.Errorf("expected string array")
		}
		out = append(out, s)
	}

	return out, nil
}

func containsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}

	return false
}
