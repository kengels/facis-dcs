package pg

import (
	"strings"
	"testing"

	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/contractworkflowengine/db"
)

func TestCreateSearchConditionsIncludesArchivedFilter(t *testing.T) {
	archived := true

	conditions, params, err := createSearchConditions(db.SearchValues{Archived: &archived})
	if err != nil {
		t.Fatalf("create search conditions: %v", err)
	}
	if conditions == nil || *conditions != " archived = $1" {
		t.Fatalf("conditions = %q, want archived condition", *conditions)
	}
	if len(params) != 1 || params[0] != true {
		t.Fatalf("params = %#v, want [true]", params)
	}
}

func TestReadAllMetaDataByFilterQueryArchivedTrueWithPagination(t *testing.T) {
	archived := true
	query, params, err := readAllMetaDataByFilterQuery(db.SearchValues{Archived: &archived}, datatype.Pagination{Limit: 25, Offset: 50})
	if err != nil {
		t.Fatalf("build filter query: %v", err)
	}

	assertContains(t, query, "FROM (", "metadata filter query must filter over derived metadata subquery")
	assertContains(t, query, "AS archived", "metadata filter query must derive archived flag")
	assertContains(t, query, "WHERE  archived = $1", "metadata filter query must filter by archived flag")
	assertContains(t, query, "ORDER BY created_at DESC LIMIT $2 OFFSET $3", "pagination placeholders must follow filter params")
	assertNotContains(t, query, "SELECT *", "metadata filter query must not return unmapped helper columns")

	want := []interface{}{true, 25, 50}
	assertParams(t, params, want)
}

func TestReadAllMetaDataByFilterQueryArchivedFalseAndContractData(t *testing.T) {
	archived := false
	query, params, err := readAllMetaDataByFilterQuery(db.SearchValues{ContractData: "service level", Archived: &archived}, datatype.Pagination{Limit: 10, Offset: 20})
	if err != nil {
		t.Fatalf("build filter query: %v", err)
	}

	assertContains(t, query, "search_vector AS search_vector", "contract_data search must expose search_vector inside subquery")
	assertContains(t, query, "search_vector @@ plainto_tsquery('english', $1)", "contract_data search must use first parameter")
	assertContains(t, query, "archived = $2", "archived filter must follow contract_data parameter")
	assertContains(t, query, "LIMIT $3 OFFSET $4", "pagination placeholders must follow search and archived params")

	want := []interface{}{"service level", false, 10, 20}
	assertParams(t, params, want)
}

func TestReadAllMetaDataQueryUsesOffsetAsStartIndex(t *testing.T) {
	query, params := readAllMetaDataQuery(datatype.Pagination{Limit: 50, Offset: 100})

	assertContains(t, query, "ORDER BY created_at DESC LIMIT $1 OFFSET $2", "pagination must use limit and start-index offset")
	want := []any{50, 100}
	if len(params) != len(want) {
		t.Fatalf("params = %#v, want %#v", params, want)
	}
	for i := range want {
		if params[i] != want[i] {
			t.Fatalf("params[%d] = %#v, want %#v", i, params[i], want[i])
		}
	}
}

func TestContractMetadataSelectQuerySearchVectorOnlyWhenRequested(t *testing.T) {
	plainQuery := contractMetadataSelectQuery(false)
	assertNotContains(t, plainQuery, "search_vector AS search_vector", "plain metadata query must not return search_vector")

	searchableQuery := contractMetadataSelectQuery(true)
	assertContains(t, searchableQuery, "search_vector AS search_vector", "searchable metadata query must expose search_vector for filtering")
	assertContains(t, searchableQuery, "AS archived", "metadata query must expose archived flag")
}

func TestArchiveEntryExistsQueryIncludesVersionAndActiveEntry(t *testing.T) {
	query := archiveEntryExistsQuery()
	assertContains(t, query, "contract_version = $2", "archive existence check must be scoped to contract version")
	assertContains(t, query, "deleted_at IS NULL", "archive existence check must ignore deleted archive entries")
}

func assertContains(t *testing.T, value string, substring string, message string) {
	t.Helper()
	if !strings.Contains(value, substring) {
		t.Fatalf("%s: expected %q to contain %q", message, value, substring)
	}
}

func assertNotContains(t *testing.T, value string, substring string, message string) {
	t.Helper()
	if strings.Contains(value, substring) {
		t.Fatalf("%s: expected %q not to contain %q", message, value, substring)
	}
}

func assertParams(t *testing.T, got []interface{}, want []interface{}) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("params = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("params[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}
