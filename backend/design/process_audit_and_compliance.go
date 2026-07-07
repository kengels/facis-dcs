package design

import (
	. "goa.design/goa/v3/dsl"
)

var PACAuditRequest = Type("PACAuditRequest", func() {
	Description("Process audit request")

	Token("token", String, "JWT token")

	Attribute("scope", String, "Scope that should be audited")

	Required("scope")
})

var PACResourceAuditTrailEntry = Type("PACResourceAuditTrailEntry", func() {
	Description("Resource audit trails entry")

	Attribute("id", Int64, "Identifier for the outbox event")
	Attribute("component", String, "Name of the component")
	Attribute("event_type", String, "Type of the event")
	Attribute("event_data", Any, "Data of the event")
	Attribute("did", String, "Decentralized Identifier of the resource")
	Attribute("created_at", String, "The creation date of the event")
	Attribute("res_log_pred_cid", String, "Resource audit trail predecessor on the IPFS chain")
	Attribute("global_log_pred_cid", String, "Global audit trail predecessor on the IPFS chain")

	Required("id", "component", "event_type", "event_data", "created_at")
})

var PACAuditResponse = Type("PACAuditResponse", func() {
	Description("Resource audit trail")

	Attribute("did", String, "Decentralized Identifier of the resource")
	Attribute("component", String, "Name of the component")
	Attribute("created_at", String, "Creation date of the audit response")
	Attribute("audit_trail", ArrayOfRequired(PACResourceAuditTrailEntry), "Resource audit trails entries")

	Required("did", "component", "created_at", "audit_trail")
})

var PACAuditFinding = Type("PACAuditFinding", func() {
	Description("Stored PACM audit finding")

	Attribute("id", String, "Stable finding identifier")
	Attribute("auditRunId", String, "AuditRun identifier")
	Attribute("scope", String, "Audit scope")
	Attribute("check", String, "Technical check name")
	Attribute("status", String, "Finding status: PASS, WARNING, FAIL, SKIPPED")
	Attribute("title", String, "Finding title")
	Attribute("message", String, "Finding message")
	Attribute("component", String, "Audited component")
	Attribute("did", String, "Audited resource DID")
	Attribute("evidence", Any, "Evidence backing the finding")
	Attribute("createdAt", String, "Creation timestamp")

	Required("id", "auditRunId", "scope", "check", "status", "title", "message", "component", "evidence", "createdAt")
})

var PACAuditEvent = Type("PACAuditEvent", func() {
	Description("Stored PACM audit event")

	Attribute("eventType", String, "PACM audit event type")
	Attribute("message", String, "Event message")
	Attribute("createdAt", String, "Event timestamp")

	Required("eventType", "message", "createdAt")
})

var PACAuditRun = Type("PACAuditRun", func() {
	Description("Stored synchronous PACM AuditRun")

	Attribute("id", String, "Stable AuditRun identifier")
	Attribute("scope", String, "Audit scope")
	Attribute("status", String, "AuditRun status: PENDING, RUNNING, COMPLETED, FAILED")
	Attribute("resultStatus", String, "Finished AuditRun result status")
	Attribute("createdAt", String, "Creation timestamp")
	Attribute("startedAt", String, "Start timestamp")
	Attribute("completedAt", String, "Completion timestamp")
	Attribute("auditedBy", String, "Participant that started the audit")
	Attribute("findings", ArrayOfRequired(PACAuditFinding), "Audit findings")
	Attribute("events", ArrayOfRequired(PACAuditEvent), "Audit lifecycle events")
	Attribute("resources", ArrayOfRequired(PACAuditResponse), "Legacy resource audit trail resources")

	Required("id", "scope", "status", "resultStatus", "createdAt", "startedAt", "auditedBy", "findings")
})

// Process Audit & Compliance Management Service  (/pac/...)
var _ = Service("ProcessAuditAndCompliance", func() {
	Description("Process Audit & Compliance Management APIs (/pac/...)")

	Method("audit", func() {
		Description("trigger an audit on selected scope.")
		Meta("dcs:requirements", "DCS-IR-PACM-01")
		Meta("dcs:ui", "Auditing Tool")
		Meta("dcs:pacm:components", "")

		Security(JWTAuth, func() {
			Scope("Auditor")
			Scope("Compliance Officer")
		})

		Payload(PACAuditRequest)
		Result(PACAuditRun)

		Error("bad_request", ErrorResult, "Bad request")
		Error("internal_error", ErrorResult, "Internal server error")

		HTTP(func() {
			POST("/pac/audit")
			Response(StatusOK)
			Response("bad_request", StatusBadRequest)
			Response("internal_error", StatusInternalServerError)
		})
	})

	Method("audit_report", func() {
		Description("generate and retrieve audit reports.")
		Meta("dcs:requirements", "DCS-IR-PACM-02")
		Meta("dcs:ui", "Auditing Tool")
		Meta("dcs:pacm:components", "")
		Security(JWTAuth, func() {
			Scope("Auditor")
		})
		Error("bad_request", ErrorResult, "Bad request")

		Payload(func() {
			Token("token", String, "JWT token")
			Attribute("auditRunId", String, "Stored AuditRun identifier")
			Attribute("scope", String, "Scope that should be reported")
			Attribute("format", String, "Report format: json, csv, or pdf")
			Attribute("did", String, "Optional resource DID filter")
			Attribute("create", String, "Command-guard flag; GET queries must not create report data")
		})
		HTTP(func() {
			GET("/pac/report")
			Param("auditRunId")
			Param("scope")
			Param("format")
			Param("did")
			Param("create")
			Response(StatusOK)
			Response("bad_request", StatusBadRequest)
		})
		Result(Any)
	})

	Method("monitor", func() {
		Description("continuous monitoring and event retrieval.")
		Meta("dcs:requirements", "DCS-IR-PACM-03")
		Meta("dcs:ui", "Non-Compliance Investigation")
		Meta("dcs:pacm:components", "")
		Security(JWTAuth, func() {
			Scope("Auditor")
			Scope("Compliance Officer")
		})
		Payload(func() {
			Token("token", String, "JWT token")
			Attribute("scope", String, "Audit scope filter")
			Attribute("status", String, "AuditRun status filter")
			Attribute("from", String, "Inclusive created-at date/time lower bound")
			Attribute("to", String, "Inclusive created-at date/time upper bound")
			Attribute("auditRunId", String, "AuditRun identifier filter")
			Attribute("includeEvents", String, "Include stored lifecycle events")
		})
		HTTP(func() {
			GET("/pac/monitor")
			Param("scope")
			Param("status")
			Param("from")
			Param("to")
			Param("auditRunId")
			Param("includeEvents")
			Response(StatusOK)
		})
		Result(Any)
	})

	Method("incident_report", func() {
		Description("submit non-compliance findings as case records.")
		Meta("dcs:requirements", "DCS-IR-PACM-04")
		Meta("dcs:ui", "Non-Compliance Investigation")
		Meta("dcs:pacm:components", "")
		Security(JWTAuth, func() {
			Scope("Auditor")
			Scope("Compliance Officer")
		})
		Error("bad_request", ErrorResult, "Bad request")
		Error("internal_error", ErrorResult, "Internal server error")

		Payload(func() {
			Token("token", String, "JWT token")
			Attribute("auditRunId", String, "Stored AuditRun identifier")
			Attribute("format", String, "Report format: json, csv, or pdf")
			Required("auditRunId")
		})
		HTTP(func() {
			POST("/pac/report")
			Response(StatusOK)
			Response("bad_request", StatusBadRequest)
			Response("internal_error", StatusInternalServerError)
		})
		Result(Any)
	})
})
