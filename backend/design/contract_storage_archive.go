package design

import (
	. "goa.design/goa/v3/dsl"
)

var ArchiveRetrieveRequest = Type("ArchiveRetrieveRequest", func() {
	Description("Archive retrieve request")

	Token("token", String, "JWT token")
})
var ArchiveRetrieveResponse = Type("ArchiveRetrieveResponse", func() {
	Description("Result for retrieving the archive")

	Attribute("contracts", ArrayOf(ContractItem), "A list of contracts")
})

var ArchiveSearchRequest = Type("ArchiveSearchRequest", func() {
	Description("Archive search request")

	Token("token", String, "JWT token")

	Attribute("did", String, "Decentralized Identifier of the contract")
	Attribute("contract_version", Int, "The version number of the contract")
	Attribute("state", String, "The state of the contract")
	Attribute("name", String, "The name of the contract")
	Attribute("description", String, "A description for that contract")
	Attribute("contract_data", String, "Search value for full text search in contract data")
	Attribute("tag", String, "Return only archive entries carrying this annotation tag (DCS-FR-CSA-11)")
	Attribute("party", String, "Return entries involving this party DID")
	Attribute("contract_type", String, "Return entries of this ontology-derived contract type")
	Attribute("jurisdiction", String, "Return entries for this ontology-derived jurisdiction")
	Attribute("parent_did", String, "Return archived children of this parent contract DID")
	Attribute("valid_from", String, "Inclusive lower validity bound (RFC3339)")
	Attribute("valid_to", String, "Inclusive upper validity bound (RFC3339)")
})

var ArchiveAlert = Type("ArchiveAlert", func() {
	Attribute("id", String)
	Attribute("did", String)
	Attribute("contract_version", Int)
	Attribute("alert_type", String)
	Attribute("due_at", String)
	Attribute("message", String)
	Attribute("created_at", String)
	Attribute("acknowledged_at", String)
	Attribute("acknowledged_by", String)
	Required("id", "did", "contract_version", "alert_type", "message", "created_at")
})

var ArchiveSavedQuery = Type("ArchiveSavedQuery", func() {
	Attribute("id", String)
	Attribute("name", String)
	Attribute("filters", Any)
	Attribute("created_at", String)
	Attribute("updated_at", String)
	Required("id", "name", "filters", "created_at", "updated_at")
})

var ArchiveComponent = Type("ArchiveComponent", func() {
	Attribute("id", String)
	Attribute("did", String)
	Attribute("contract_version", Int)
	Attribute("component_iri", String)
	Attribute("party_ids", ArrayOf(String))
	Attribute("component_snapshot", Any)
	Attribute("content_hash", String)
	Required("id", "did", "contract_version", "component_iri", "party_ids", "component_snapshot", "content_hash")
})

var ArchiveRetention = Type("ArchiveRetention", func() {
	Attribute("did", String)
	Attribute("retention_until", String)
	Attribute("archive_status", String)
	Required("did", "archive_status")
})

var ArchiveMonitoringPreferences = Type("ArchiveMonitoringPreferences", func() {
	Attribute("enabled", Boolean)
	Attribute("notice_days", Int)
	Attribute("updated_at", String)
	Required("enabled", "notice_days", "updated_at")
})

var ArchiveAnnotationResponse = Type("ArchiveAnnotationResponse", func() {
	Description("The archive entry annotation after an annotate call (DCS-FR-CSA-11)")

	Attribute("did", String, "Decentralized Identifier of the annotated contract")
	Attribute("summary", String, "The stored summary (caller-provided, or system-generated from the contract metadata when none was supplied)")
	Attribute("tags", ArrayOf(String), "The stored tag set")

	Required("did", "summary")
})

var ArchiveDashboardRecentAction = Type("ArchiveDashboardRecentAction", func() {
	Description("A persisted archive-domain action shown by the archive dashboard")
	Attribute("id", Int64, "Persistent outbox event identifier")
	Attribute("did", String, "Affected contract DID")
	Attribute("event_type", String, "Archive event type")
	Attribute("occurred_at", String, "Event creation time (RFC3339)")
	Required("id", "did", "event_type", "occurred_at")
})

var ArchiveDashboardComplianceSummary = Type("ArchiveDashboardComplianceSummary", func() {
	Description("Server-derived archive compliance counts")
	Attribute("archive_entries", Int, "Number of non-deleted archive entries")
	Attribute("entries_with_proof", Int, "Number of non-deleted entries with a content hash and snapshot CID")
	Attribute("entries_without_proof", Int, "Number of non-deleted entries lacking a content hash or snapshot CID")
	Attribute("non_compliant_entries", Int)
	Required("archive_entries", "entries_with_proof", "entries_without_proof", "non_compliant_entries")
})

var ArchiveDashboardResponse = Type("ArchiveDashboardResponse", func() {
	Description("Authoritative server read model for the contract archive dashboard")
	Attribute("recent_actions", ArrayOfRequired(ArchiveDashboardRecentAction), "Persisted archive actions, newest first")
	Attribute("expiring_contracts", ArrayOfRequired(ContractItem), "Archived contracts with a future expiration date, soonest first")
	Attribute("compliance", ArchiveDashboardComplianceSummary, "Server-derived archive proof coverage")
	Attribute("storage_bytes", Int64, "Persisted bytes occupied by archive snapshots and evidence")
	Attribute("open_alerts", Int, "Unacknowledged archive alerts")
	Attribute("renewal_due", Int, "Open renewal-due alerts")
	Attribute("source", String, "Origin of the dashboard values")
	Attribute("generated_at", String, "Read-model generation time (RFC3339)")
	Required("recent_actions", "expiring_contracts", "compliance", "storage_bytes", "open_alerts", "renewal_due", "source", "generated_at")
})

// Contract Storage & Archive Service  (/archive/...)
var _ = Service("ContractStorageArchive", func() {
	Description("Contract Storage & Archive APIs (/archive/...)")

	Method("retrieve", func() {
		Description("retrieve archived items.")
		Meta("dcs:requirements", "DCS-IR-CSA-01", "DCS-IR-CSA-05")
		Meta("dcs:ui", "Archive Manager Dashboard", "Archive Access")
		Meta("dcs:csa:components", "Signed Contract Archive")

		Security(JWTAuth, func() {
			Scope("Archive Manager")
			Scope("Contract Observer")
		})

		Payload(ArchiveRetrieveRequest)
		Result(ArchiveRetrieveResponse)

		Error("bad_request", ErrorResult, "Bad request")
		Error("internal_error", ErrorResult, "Internal server error")

		HTTP(func() {
			GET("/archive/retrieve")
			Response(StatusOK)
			Response("bad_request", StatusBadRequest)
			Response("internal_error", StatusInternalServerError)
		})
	})

	Method("search", func() {
		Description("search archived records. search records by criteria.")
		Meta("dcs:requirements", "DCS-IR-CSA-01", "DCS-IR-CSA-05")
		Meta("dcs:ui", "Archive Manager Dashboard", "Archive Access")
		Meta("dcs:csa:components", "Signed Contract Archive")
		Security(JWTAuth, func() {
			Scope("Archive Manager")
			Scope("Contract Observer")
		})
		Payload(ArchiveSearchRequest)
		Result(ArrayOfRequired(ContractItem))

		Error("bad_request", ErrorResult, "Bad request")
		Error("internal_error", ErrorResult, "Internal server error")

		HTTP(func() {
			GET("/archive/search")
			Param("did")
			Param("contract_version")
			Param("state")
			Param("name")
			Param("description")
			Param("contract_data")
			Param("tag")
			Param("party")
			Param("contract_type")
			Param("jurisdiction")
			Param("parent_did")
			Param("valid_from")
			Param("valid_to")
			Response(StatusOK)
			Response("bad_request", StatusBadRequest)
			Response("internal_error", StatusInternalServerError)
		})
	})

	Method("dashboard", func() {
		Description("Return authoritative archive dashboard data derived from persisted archive entries and archive audit events.")
		Meta("dcs:requirements", "DCS-FR-CSA-21", "DCS-IR-CSA-01")
		Meta("dcs:ui", "Archive Manager Dashboard")
		Meta("dcs:csa:components", "Signed Contract Archive")
		Security(JWTAuth, func() {
			Scope("Archive Manager")
			Scope("Contract Observer")
		})
		Payload(ArchiveRetrieveRequest)
		Result(ArchiveDashboardResponse)
		Error("internal_error", ErrorResult, "Internal server error")
		HTTP(func() {
			GET("/archive/dashboard")
			Response(StatusOK)
			Response("internal_error", StatusInternalServerError)
		})
	})

	Method("store", func() {
		Description("store new contract or evidence.")
		Meta("dcs:requirements", "DCS-IR-CSA-02", "DCS-IR-CSA-06")
		Meta("dcs:ui", "Archive Manager Dashboard")
		Meta("dcs:csa:components", "Signed Contract Archive")
		Security(JWTAuth, func() {
			Scope("Archive Manager")
		})
		Payload(func() {
			Token("token", String, "JWT token")
		})
		HTTP(func() {
			POST("/archive/store")
			Response(StatusOK)
		})
		Result(String)
	})

	Method("delete", func() {
		Description("Permanently delete an archived contract entry (DCS-FR-CSA-17). This is a soft delete: the archive entry is marked deleted_at/deleted_by/deletion_reason rather than physically removed, so evidence remains discoverable for compliance/dispute resolution, and requires a justification that is logged with the deletion's audit event.")
		Meta("dcs:requirements", "DCS-IR-CSA-03", "DCS-IR-CSA-06", "DCS-FR-CSA-17")
		Meta("dcs:ui", "Archive Manager Dashboard")
		Meta("dcs:csa:components", "Signed Contract Archive", "Automated Alerts")
		Security(JWTAuth, func() {
			Scope("Archive Manager")
		})
		Payload(func() {
			Token("token", String, "JWT token")
			Attribute("did", String, "Decentralized Identifier of the archived contract to delete")
			Attribute("justification", String, "Justification for the deletion (DCS-FR-CSA-17); logged with the deletion audit event")
			Required("did", "justification")
		})

		Error("bad_request", ErrorResult, "Bad request")
		Error("internal_error", ErrorResult, "Internal server error")

		HTTP(func() {
			DELETE("/archive/delete")
			Param("did")
			Param("justification")
			Response(StatusOK)
			Response("bad_request", StatusBadRequest)
			Response("internal_error", StatusInternalServerError)
		})
		Result(Int)
	})

	Method("annotate", func() {
		Description("Annotate an archived contract with a summary and tags (DCS-FR-CSA-11). The summary may be supplied by the caller or, when omitted, is generated from the archived contract's metadata; tags replace the entry's tag set when provided. Only the annotation is mutable — the archive entry's snapshot and evidence stay immutable.")
		Meta("dcs:requirements", "DCS-FR-CSA-11")
		Meta("dcs:ui", "Archive Manager Dashboard")
		Meta("dcs:csa:components", "Signed Contract Archive")
		Security(JWTAuth, func() {
			Scope("Archive Manager")
		})
		Payload(func() {
			Token("token", String, "JWT token")
			Attribute("did", String, "Decentralized Identifier of the archived contract to annotate")
			Attribute("summary", String, "Manual summary; when omitted (and none is stored yet) a summary is generated from the contract metadata")
			Attribute("tags", ArrayOf(String), "Tags for thematic categorization and discovery; replaces the entry's tag set")
			Required("did")
		})
		Result(ArchiveAnnotationResponse)

		Error("bad_request", ErrorResult, "Bad request")
		Error("internal_error", ErrorResult, "Internal server error")

		HTTP(func() {
			POST("/archive/annotate")
			Response(StatusOK)
			Response("bad_request", StatusBadRequest)
			Response("internal_error", StatusInternalServerError)
		})
	})

	Method("audit", func() {
		Description("Retrieve the archive audit log: actor, timestamp, operation, and contract DID for every recorded archive-affecting event (store/retrieve/search/delete) — DCS-IR-CSA-04, UC-07-03.")
		Meta("dcs:requirements", "DCS-IR-CSA-04")
		Meta("dcs:ui", "Archive Manager Dashboard")
		Meta("dcs:csa:components", "")
		Security(JWTAuth, func() {
			Scope("Auditor")
			Scope("Archive Manager")
		})
		Payload(func() {
			Token("token", String, "JWT token")
			Attribute("did", String, "Optional archived contract DID filter")
			Attribute("justification", String, "Required audit justification", func() { MinLength(1) })
			Required("justification")
		})

		Error("bad_request", ErrorResult, "Bad request")
		Error("internal_error", ErrorResult, "Internal server error")

		HTTP(func() {
			GET("/archive/audit")
			Param("did")
			Param("justification")
			Response(StatusOK)
			Response("bad_request", StatusBadRequest)
			Response("internal_error", StatusInternalServerError)
		})
		Result(ArrayOfRequired(PACAuditResponse))
	})

	Method("alerts", func() {
		Description("List persisted archive expiry, retention and compliance alerts.")
		Meta("dcs:requirements", "DCS-FR-CSA-04", "DCS-FR-CSA-20", "DCS-FR-CSA-23")
		Security(JWTAuth, func() { Scope("Archive Manager"); Scope("Contract Observer") })
		Payload(func() { Token("token", String); Attribute("include_acknowledged", Boolean) })
		Result(ArrayOfRequired(ArchiveAlert))
		Error("internal_error", ErrorResult)
		HTTP(func() {
			GET("/archive/alerts")
			Param("include_acknowledged")
			Response(StatusOK)
			Response("internal_error", StatusInternalServerError)
		})
	})

	Method("acknowledge_alert", func() {
		Description("Acknowledge one archive alert without deleting its audit evidence.")
		Meta("dcs:requirements", "DCS-FR-CSA-20", "DCS-FR-CSA-23")
		Security(JWTAuth, func() { Scope("Archive Manager") })
		Payload(func() { Token("token", String); Attribute("id", String); Required("id") })
		Result(ArchiveAlert)
		Error("bad_request", ErrorResult)
		Error("internal_error", ErrorResult)
		HTTP(func() {
			POST("/archive/alerts/{id}/acknowledge")
			Response(StatusOK)
			Response("bad_request", StatusBadRequest)
			Response("internal_error", StatusInternalServerError)
		})
	})

	Method("monitoring_preferences", func() {
		Description("Read the current user's archive alert preferences.")
		Meta("dcs:requirements", "DCS-FR-CSA-20", "DCS-FR-CSA-23")
		Security(JWTAuth, func() { Scope("Archive Manager"); Scope("Contract Observer") })
		Payload(func() { Token("token", String) })
		Result(ArchiveMonitoringPreferences)
		Error("internal_error", ErrorResult)
		HTTP(func() {
			GET("/archive/monitoring-preferences")
			Response(StatusOK)
			Response("internal_error", StatusInternalServerError)
		})
	})

	Method("set_monitoring_preferences", func() {
		Description("Enable or disable UI archive alerts and configure the personal notice window.")
		Meta("dcs:requirements", "DCS-FR-CSA-20", "DCS-FR-CSA-23")
		Security(JWTAuth, func() { Scope("Archive Manager"); Scope("Contract Observer") })
		Payload(func() {
			Token("token", String)
			Attribute("enabled", Boolean)
			Attribute("notice_days", Int, func() { Minimum(0); Maximum(3650) })
			Required("enabled", "notice_days")
		})
		Result(ArchiveMonitoringPreferences)
		Error("internal_error", ErrorResult)
		HTTP(func() {
			PUT("/archive/monitoring-preferences")
			Response(StatusOK)
			Response("internal_error", StatusInternalServerError)
		})
	})

	Method("set_retention", func() {
		Description("Set or extend an archive retention deadline with an audited justification.")
		Meta("dcs:requirements", "DCS-FR-CSA-14", "DCS-FR-CSA-19")
		Security(JWTAuth, func() { Scope("Archive Manager") })
		Payload(func() {
			Token("token", String)
			Attribute("did", String)
			Attribute("retention_until", String)
			Attribute("justification", String, func() { MinLength(1) })
			Required("did", "justification")
		})
		Result(ArchiveRetention)
		Error("bad_request", ErrorResult)
		Error("internal_error", ErrorResult)
		HTTP(func() {
			PUT("/archive/retention")
			Response(StatusOK)
			Response("bad_request", StatusBadRequest)
			Response("internal_error", StatusInternalServerError)
		})
	})

	Method("saved_queries", func() {
		Description("List the current user's saved archive searches.")
		Meta("dcs:requirements", "DCS-FR-CSA-22")
		Security(JWTAuth, func() { Scope("Archive Manager"); Scope("Contract Observer") })
		Payload(func() { Token("token", String) })
		Result(ArrayOfRequired(ArchiveSavedQuery))
		Error("internal_error", ErrorResult)
		HTTP(func() {
			GET("/archive/saved-queries")
			Response(StatusOK)
			Response("internal_error", StatusInternalServerError)
		})
	})

	Method("save_query", func() {
		Description("Persist a named archive search for the current user.")
		Meta("dcs:requirements", "DCS-FR-CSA-22")
		Security(JWTAuth, func() { Scope("Archive Manager"); Scope("Contract Observer") })
		Payload(func() {
			Token("token", String)
			Attribute("name", String, func() { MinLength(1) })
			Attribute("filters", Any)
			Required("name", "filters")
		})
		Result(ArchiveSavedQuery)
		Error("bad_request", ErrorResult)
		Error("internal_error", ErrorResult)
		HTTP(func() {
			POST("/archive/saved-queries")
			Response(StatusOK)
			Response("bad_request", StatusBadRequest)
			Response("internal_error", StatusInternalServerError)
		})
	})

	Method("delete_saved_query", func() {
		Description("Delete one of the current user's saved archive searches.")
		Meta("dcs:requirements", "DCS-FR-CSA-22")
		Security(JWTAuth, func() { Scope("Archive Manager"); Scope("Contract Observer") })
		Payload(func() { Token("token", String); Attribute("id", String); Required("id") })
		Result(Boolean)
		Error("bad_request", ErrorResult)
		Error("internal_error", ErrorResult)
		HTTP(func() {
			DELETE("/archive/saved-queries/{id}")
			Response(StatusOK)
			Response("bad_request", StatusBadRequest)
			Response("internal_error", StatusInternalServerError)
		})
	})

	Method("components", func() {
		Description("Return archived contract components visible to the caller.")
		Meta("dcs:requirements", "DCS-FR-CSA-05", "DCS-FR-CSA-26")
		Security(JWTAuth, func() { Scope("Archive Manager"); Scope("Contract Observer"); Scope("Auditor") })
		Payload(func() { Token("token", String); Attribute("did", String); Required("did") })
		Result(ArrayOfRequired(ArchiveComponent))
		Error("internal_error", ErrorResult)
		HTTP(func() {
			GET("/archive/components/{did}")
			Response(StatusOK)
			Response("internal_error", StatusInternalServerError)
		})
	})

})
