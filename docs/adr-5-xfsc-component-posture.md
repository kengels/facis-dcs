# ADR-5: XFSC component posture — what is retained, what is substituted

## Context

DCS is built against the XFSC (GAIA-X Federated Secure Computing) stack.
Appendix D describes the OCM W-Stack deployment, whose "essential crypto
parts are defined in the Crypto Provider Service (formerly TSA Signer
Service)." Appendix E describes the Status List Service, deployable
standalone or together with the Crypto Provider Service. IR-SI-02 requires
Node-RED-webhook-compatible orchestration integration. Not every XFSC
component fits DCS's actual key-custody decision (ADR-1), and this ADR
records, component by component, which are used as specified and which are
substituted.

## Decision

| XFSC component | Posture |
|---|---|
| Federated Catalogue | Retained behind the DCS catalogue adapter (`backend/internal/templatecatalogueintegration/`). Contract templates and component templates use the shared RDF class `dcs:ContractTemplate`; `dcs:templateType` preserves the product-level distinction. |
| XFSC Status List Service | Retained, integrated as specified — credential revocation status (`backend/internal/auth/oid4vp/status_list*.go`) and C2PA lifecycle status publication both resolve to it. |
| ORCE (orchestration engine) | Retained; IR-SI-02's Node-RED-webhook compatibility is satisfied by the shipped example flow (`deployment/helm/charts/orce/flows/contract-target-flow.json`). |
| Crypto Provider Service (Appendix D's "essential crypto parts") | **Substituted** by PKCS#11/HSM key custody (ADR-1). IR-HI-01's "standardized interfaces" wording is read as satisfied by PKCS#11 itself being the standardized interface — DCS does not require the specific Crypto Provider Service REST API to satisfy that requirement, since PKCS#11 already is one. |
| OCM W-Stack (issuance/verification/retrieval/well-known) | Not used for VC signing — see [adr-ocmw-vc-signing.md](adr-ocmw-vc-signing.md) for the detailed evaluation of why the OCM-W protocol (OID4VCI wallet-pull) does not fit DCS's synchronous in-process signing requirement. Remains the natural integration point if DCS later issues credentials *to* wallet holders (a different feature than what it does today). |

### Catalogue typing for reusable template components

- Catalogue assets for both `CONTRACT_TEMPLATE` and `COMPONENT` retain the
  existing RDF class `dcs:ContractTemplate`. The publish payload carries the
  normalized repository type separately as `dcs:templateType`
  (`backend/internal/fcasset/asset.go:61-78`,
  `backend/internal/templaterepository/catalogueasset/payload.go:25-49`).
- The catalogue adapter projects that discriminator for list, detail, and
  search responses (`backend/internal/templatecatalogueintegration/query/template/queryall.go:43-59`,
  `backend/internal/templatecatalogueintegration/query/template/querybyid.go:37-49`,
  `backend/internal/templatecatalogueintegration/query/template/search.go:49-64`).
- The existing catalogue response contract already exposes `template_type`,
  so this decision requires no new endpoint or wire field
  (`backend/design/template_catalogue_integration.go:7-21`,
  `backend/design/template_catalogue_integration.go:70-86`).

A separate RDF class for components was rejected because components share the
same catalogue resource semantics and lifecycle as other contract templates.
A parallel component-specific API or persistence model was also rejected: it
would duplicate the existing type-neutral lifecycle and catalogue interfaces
without adding a distinct behavior.

## Consequences

- The substitution is documented rather than silently made, so a reviewer
  checking Appendix D compliance finds the reasoning here instead of
  discovering an unexplained gap.
- The swap-back path (re-adopting the Crypto Provider Service instead of
  PKCS#11 directly) stays open: DCS's signer interfaces (`VCSigner`,
  the HSM `crypto.Signer`) are narrow enough that a Crypto-Provider-backed
  implementation could be substituted without touching call sites.
- Template Managers can apply the existing `APPROVED → REGISTERED → PUBLISHED`
  lifecycle to both template kinds; action visibility depends on role and state,
  not on template type
  (`frontend/ClientApp/src/modules/template-repository/utils/template-manager-lifecycle.ts:4-10`).
- Consumers can distinguish components after a catalogue roundtrip without
  introducing an additional ontology class or database migration. Missing or
  unsupported repository types are rejected at the catalogue adapter boundary
  (`backend/internal/templaterepository/catalogueasset/payload.go:18-28`).
