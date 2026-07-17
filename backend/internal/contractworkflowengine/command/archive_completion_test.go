package command

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"digital-contracting-service/internal/base/datatype"
)

func TestArchiveIndexUsesFrozenJSONLDFacets(t *testing.T) {
	raw, err := datatype.NewJSON(map[string]any{
		"@id":                "did:example:child",
		"@type":              "dcs:SubContract",
		"dcs:parentContract": map[string]any{"@id": "did:example:frame"},
		"dcs:parties": []any{
			map[string]any{"@id": "did:example:buyer"},
			map[string]any{"@id": "did:example:seller"},
		},
		"dcs:fields": []any{map[string]any{
			"dcs:parameterName":  "contract.jurisdiction",
			"dcs:parameterValue": "DEU",
		}},
	})
	require.NoError(t, err)

	index, err := archiveIndex(&raw)
	require.NoError(t, err)
	require.Equal(t, "did:example:frame", *index.parentDID)
	require.Equal(t, "dcs:SubContract", *index.contractType)
	require.Equal(t, "DEU", *index.jurisdiction)

	var parties []map[string]any
	require.NoError(t, json.Unmarshal(*index.parties, &parties))
	require.Len(t, parties, 2)
}

func TestBuildArchiveComponentsHashesAndScopesSections(t *testing.T) {
	raw, err := datatype.NewJSON(map[string]any{
		"@id": "did:example:contract",
		"dcs:sections": map[string]any{"@list": []any{
			map[string]any{
				"@id":           "did:example:contract#payment",
				"dcs:title":     "Payment",
				"odrl:assignee": map[string]any{"@id": "did:example:buyer"},
			},
			map[string]any{
				"@id":       "did:example:contract#general",
				"dcs:title": "General",
			},
		}},
	})
	require.NoError(t, err)

	components, err := BuildArchiveComponents(&raw)
	require.NoError(t, err)
	require.Len(t, components, 2)
	require.Equal(t, "did:example:contract#payment", components[0].IRI)
	require.Regexp(t, `^sha256:[a-f0-9]{64}$`, components[0].ContentHash)

	var partyIDs []string
	require.NoError(t, json.Unmarshal(components[0].PartyIDs, &partyIDs))
	require.Equal(t, []string{"did:example:buyer"}, partyIDs)
	require.JSONEq(t, `[]`, string(components[1].PartyIDs))
}
