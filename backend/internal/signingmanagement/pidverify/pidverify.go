// Package pidverify re-verifies a PID SD-JWT VC + KB-JWT presentation for a
// signing ceremony (UC-04-02, UC-04-03). It proves the presentation is
// internally consistent: the holder binding key (cnf.jwk) signs the KB-JWT, the
// KB-JWT carries the correct sd_hash for the disclosed credential and its
// audience is the ceremony audience. The signer DID is derived from cnf.jwk;
// when a credential also declares sub, it must identify that same holder key.
package pidverify

import (
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"digital-contracting-service/internal/auth/oid4vp/sdjwt"
)

// Audience is the fixed OID4VP audience/client_id bound into the KB-JWT of a
// PID presentation for a signing ceremony.
const Audience = "dcs-signature-ceremony"

// Verify validates the presentation and returns the signer DID (credential
// subject) and the KB-JWT sd_hash.
func Verify(vpToken string) (signerDID, sdHash string, err error) {
	return verify(vpToken, "")
}

// VerifyForNonce additionally binds a live presentation to the nonce issued
// for the signing ceremony.
func VerifyForNonce(vpToken, expectedNonce string) (signerDID, sdHash string, err error) {
	if strings.TrimSpace(expectedNonce) == "" {
		return "", "", fmt.Errorf("expected nonce is required")
	}
	return verify(vpToken, expectedNonce)
}

func verify(vpToken, expectedNonce string) (signerDID, sdHash string, err error) {
	presentation, err := sdjwt.ParsePresentation(vpToken)
	if err != nil {
		return "", "", err
	}

	issuerClaims := jwt.MapClaims{}
	if _, _, perr := jwt.NewParser().ParseUnverified(presentation.IssuerJWT, &issuerClaims); perr != nil {
		return "", "", fmt.Errorf("parse issuer jwt: %w", perr)
	}

	cnfJWK, err := sdjwt.CNFJWKFromClaims(issuerClaims)
	if err != nil {
		return "", "", err
	}
	sub, _ := issuerClaims["sub"].(string)
	sub = strings.TrimSpace(sub)
	expectedSub, err := sdjwt.DIDJWKFromPublicJWK(cnfJWK)
	if err != nil {
		return "", "", fmt.Errorf("credential cnf.jwk: %w", err)
	}
	if sub != "" && sub != expectedSub {
		return "", "", fmt.Errorf("credential sub does not match cnf.jwk holder binding")
	}
	sub = expectedSub

	// The ceremony webhook does not carry the wallet nonce, so KB verification
	// checks the holder signature, sd_hash and audience against the nonce
	// actually present in the KB-JWT (self-consistency), not a server nonce.
	kbNonce, err := kbJWTNonce(presentation.KBJWT)
	if err != nil {
		return "", "", err
	}
	if expectedNonce != "" && kbNonce != expectedNonce {
		return "", "", fmt.Errorf("kb jwt nonce does not match signing ceremony")
	}
	if err := sdjwt.VerifyKB(presentation.KBJWT, presentation.SDHash, cnfJWK, sub, kbNonce, Audience); err != nil {
		return "", "", err
	}

	return sub, presentation.SDHash, nil
}

func kbJWTNonce(kbJWT string) (string, error) {
	claims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(kbJWT, &claims); err != nil {
		return "", fmt.Errorf("parse kb jwt: %w", err)
	}
	nonce, _ := claims["nonce"].(string)
	if strings.TrimSpace(nonce) == "" {
		return "", fmt.Errorf("kb jwt missing nonce")
	}
	return nonce, nil
}
