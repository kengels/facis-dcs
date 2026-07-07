package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	baseevent "digital-contracting-service/internal/base/event"

	processauditandcompliance "digital-contracting-service/gen/process_audit_and_compliance"
	"digital-contracting-service/internal/auth"
	"digital-contracting-service/internal/base"
	"digital-contracting-service/internal/base/conf"
	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/base/datatype/componenttype"
	cwedb "digital-contracting-service/internal/contractworkflowengine/db"
	"digital-contracting-service/internal/middleware"
	pacevent "digital-contracting-service/internal/processauditandcompliance/event"
	qry "digital-contracting-service/internal/processauditandcompliance/query"
	templatedb "digital-contracting-service/internal/templaterepository/db"

	"github.com/jmoiron/sqlx"
	"goa.design/clue/log"
)

const signatureValidationUnavailable = "Signature validation is not available because signing is not implemented yet."

type processAuditAndCompliancesrvc struct {
	DB           *sqlx.DB
	ATrailReader base.AuditTrailReader
	CTRepo       templatedb.ContractTemplateRepo
	CRepo        cwedb.ContractRepo
	auth.JWTAuthenticator
}

type auditScopeConfig struct {
	scopeName                      string
	apiScope                       string
	component                      componenttype.ComponentType
	requiresTemplateRepo           bool
	requiresContractRepo           bool
	includeTemplatePolicyTrail     bool
	includeTemplateProvenanceTrail bool
	includeContractContentTrail    bool
	includeArchiveTrail            bool
}

type pacmAuditRunRow struct {
	ID           string     `db:"id"`
	Scope        string     `db:"scope"`
	Status       string     `db:"status"`
	ResultStatus string     `db:"result_status"`
	CreatedAt    time.Time  `db:"created_at"`
	StartedAt    time.Time  `db:"started_at"`
	CompletedAt  *time.Time `db:"completed_at"`
	AuditedBy    string     `db:"audited_by"`
	RunData      []byte     `db:"run_data"`
}

type auditRunQuery struct {
	Scope         string
	Status        string
	From          string
	To            string
	AuditRunID    string
	IncludeEvents bool
}

func NewProcessAuditAndCompliance(db *sqlx.DB, jwtAuth auth.JWTAuthenticator, auditTrailReader base.AuditTrailReader, ctRepo templatedb.ContractTemplateRepo, cRepo cwedb.ContractRepo) processauditandcompliance.Service {
	return &processAuditAndCompliancesrvc{DB: db, JWTAuthenticator: jwtAuth, ATrailReader: auditTrailReader, CTRepo: ctRepo, CRepo: cRepo}
}

func (s *processAuditAndCompliancesrvc) Audit(ctx context.Context, req *processauditandcompliance.PACAuditRequest) (res *processauditandcompliance.PACAuditRun, err error) {
	ctx, cancel := context.WithTimeout(ctx, conf.TransactionTimeout())
	defer cancel()

	scopeConfig, err := resolveAuditScope(req.Scope)
	if err != nil {
		return nil, processauditandcompliance.MakeBadRequest(err)
	}
	if err := s.validateAuditScopeDependencies(scopeConfig); err != nil {
		return nil, processauditandcompliance.MakeInternalError(err)
	}
	if err := s.ensureAuditTables(ctx); err != nil {
		return nil, processauditandcompliance.MakeInternalError(err)
	}

	now := time.Now().UTC()
	run := &processauditandcompliance.PACAuditRun{
		ID:           newPACMID("pac-run"),
		Scope:        scopeConfig.apiScope,
		Status:       "RUNNING",
		ResultStatus: "RUNNING",
		CreatedAt:    now.Format(time.RFC3339),
		StartedAt:    now.Format(time.RFC3339),
		AuditedBy:    middleware.GetParticipantID(ctx),
		Findings:     []*processauditandcompliance.PACAuditFinding{},
		Events:       []*processauditandcompliance.PACAuditEvent{},
	}
	appendAuditEvent(run, "AuditRunStarted", fmt.Sprintf("PACM audit run %s started for scope %s", run.ID, run.Scope), now)

	resources, err := s.readAuditResources(ctx, scopeConfig)
	if err != nil {
		appendAuditEvent(run, "AuditRunFailed", err.Error(), time.Now().UTC())
		return nil, processauditandcompliance.MakeInternalError(err)
	}
	run.Resources = resources

	findings := buildPACMFindings(run, scopeConfig, now)
	for _, finding := range findings {
		run.Findings = append(run.Findings, finding)
		appendAuditEvent(run, "AuditCheckCompleted", fmt.Sprintf("Audit check completed: %s", finding.Check), now)
	}
	completedAt := time.Now().UTC()
	run.Status = "COMPLETED"
	run.ResultStatus = "COMPLETED"
	completed := completedAt.Format(time.RFC3339)
	run.CompletedAt = &completed
	appendAuditEvent(run, "AuditRunCompleted", fmt.Sprintf("PACM audit run %s completed", run.ID), completedAt)

	if err := s.persistAuditRun(ctx, run); err != nil {
		appendAuditEvent(run, "AuditRunFailed", err.Error(), time.Now().UTC())
		return nil, processauditandcompliance.MakeInternalError(err)
	}
	return run, nil
}

func resolveAuditScope(rawScope string) (auditScopeConfig, error) {
	normalized := strings.ToUpper(strings.TrimSpace(rawScope))
	switch normalized {
	case "CONTRACT", "CONTRACTS", "CONTRACT_WORKFLOW_ENGINE":
		return contractAuditScopeConfig(), nil
	case "TEMPLATE", "TEMPLATES", "CONTRACT_TEMPLATE_REPOSITORY":
		return templateAuditScopeConfig(), nil
	case "ARCHIVE", "ARCHIVES", "CONTRACT_STORAGE_ARCHIVE":
		return archiveAuditScopeConfig(), nil
	case "SIGNATURE", "SIGNATURES", "SIGNATURE_MANAGEMENT":
		return signatureAuditScopeConfig(), nil
	default:
		return auditScopeConfig{}, fmt.Errorf("invalid audit scope %q; allowed values are CONTRACT, TEMPLATE, ARCHIVE, SIGNATURE", rawScope)
	}
}

func templateAuditScopeConfig() auditScopeConfig {
	return auditScopeConfig{scopeName: "templates", apiScope: "TEMPLATE", component: componenttype.ContractTemplateRepo, requiresTemplateRepo: true, includeTemplatePolicyTrail: true, includeTemplateProvenanceTrail: true}
}

func contractAuditScopeConfig() auditScopeConfig {
	return auditScopeConfig{scopeName: "contracts", apiScope: "CONTRACT", component: componenttype.ContractWorkflowEngine, requiresContractRepo: true, includeContractContentTrail: true}
}

func archiveAuditScopeConfig() auditScopeConfig {
	return auditScopeConfig{scopeName: "archive", apiScope: "ARCHIVE", component: componenttype.ContractStorageArchive, requiresContractRepo: true, includeArchiveTrail: true}
}

func signatureAuditScopeConfig() auditScopeConfig {
	return auditScopeConfig{scopeName: "signatures", apiScope: "SIGNATURE", component: componenttype.SignatureManagement}
}

func (s *processAuditAndCompliancesrvc) validateAuditScopeDependencies(scopeConfig auditScopeConfig) error {
	if scopeConfig.requiresTemplateRepo && s.CTRepo == nil {
		return fmt.Errorf("audit scope %s is not configured", scopeConfig.scopeName)
	}
	if scopeConfig.requiresContractRepo && s.CRepo == nil {
		return fmt.Errorf("audit scope %s is not configured", scopeConfig.scopeName)
	}
	return nil
}

func (s *processAuditAndCompliancesrvc) readAuditResources(ctx context.Context, scope auditScopeConfig) ([]*processauditandcompliance.PACAuditResponse, error) {
	if s.DB == nil || scope.apiScope == "SIGNATURE" {
		return []*processauditandcompliance.PACAuditResponse{}, nil
	}

	resources := []*processauditandcompliance.PACAuditResponse{}
	if s.ATrailReader.ARepo != nil && s.ATrailReader.IPFSClient != nil {
		handler := qry.Auditor{DB: s.DB, ATrailReader: s.ATrailReader}
		histories, err := handler.Handle(ctx, qry.GetAuditLogQry{
			Scope:     scope.component,
			AuditedBy: middleware.GetParticipantID(ctx),
			HolderDID: middleware.GetHolderDID(ctx),
			UserRoles: middleware.GetUserRoles(ctx),
		})
		if err != nil {
			return nil, err
		}
		resources = auditResponsesFromHistories(scope.component, histories)
	}

	if scope.includeContractContentTrail {
		handler := qry.ContractContentTrailAuditor{DB: s.DB, CRepo: s.CRepo}
		contentHistories, err := handler.Handle(ctx, qry.GetContractContentTrailQry{
			RetrievedBy: middleware.GetParticipantID(ctx),
			HolderDID:   middleware.GetHolderDID(ctx),
			UserRoles:   middleware.GetUserRoles(ctx),
		})
		if err != nil {
			return nil, err
		}
		resources = mergeAuditResponsesWithDIDHistories(resources, scope.component, contentHistories)
	}

	return resources, nil
}

func auditResponsesFromHistories(component componenttype.ComponentType, histories [][]datatype.AuditLogEntry) []*processauditandcompliance.PACAuditResponse {
	responses := make([]*processauditandcompliance.PACAuditResponse, 0, len(histories))
	for _, history := range histories {
		entries := make([]*processauditandcompliance.PACResourceAuditTrailEntry, 0, len(history))
		did := ""
		createdAt := time.Now().UTC().Format(time.RFC3339)
		for _, entry := range history {
			if entry.DID != nil && strings.TrimSpace(*entry.DID) != "" {
				did = strings.TrimSpace(*entry.DID)
			}
			if !base.IsAuditVisibleEventType(entry.EventType) {
				continue
			}
			createdAt = entry.CreatedAt.UTC().Format(time.RFC3339)
			entryDID := entry.DID
			entries = append(entries, &processauditandcompliance.PACResourceAuditTrailEntry{
				ID:               entry.ID,
				Component:        entry.Component,
				EventType:        entry.EventType,
				EventData:        entry.EventData,
				Did:              entryDID,
				CreatedAt:        entry.CreatedAt.UTC().Format(time.RFC3339),
				ResLogPredCid:    entry.ResLogPredCID,
				GlobalLogPredCid: entry.GlobalLogPredCID,
			})
		}
		if len(entries) == 0 {
			continue
		}
		responses = append(responses, &processauditandcompliance.PACAuditResponse{
			Did:        did,
			Component:  component.String(),
			CreatedAt:  createdAt,
			AuditTrail: entries,
		})
	}
	return responses
}

func mergeAuditResponsesWithDIDHistories(responses []*processauditandcompliance.PACAuditResponse, component componenttype.ComponentType, histories map[string][]datatype.AuditLogEntry) []*processauditandcompliance.PACAuditResponse {
	index := map[string]int{}
	for i, response := range responses {
		if response == nil {
			continue
		}
		key := auditResourceKey(response.Component, response.Did)
		if key == "" {
			continue
		}
		index[key] = i
	}
	for did, history := range histories {
		entries := auditTrailEntriesFromHistory(history)
		if len(entries) == 0 {
			continue
		}
		normalizedDID := strings.TrimSpace(did)
		if normalizedDID == "" {
			normalizedDID = auditTrailDID(entries)
		}
		if normalizedDID == "" {
			continue
		}
		key := auditResourceKey(component.String(), normalizedDID)
		if existing, ok := index[key]; ok {
			responses[existing].AuditTrail = appendUniqueAuditTrailEntries(responses[existing].AuditTrail, entries)
			responses[existing].CreatedAt = latestAuditTrailTimestamp(responses[existing].CreatedAt, responses[existing].AuditTrail)
			continue
		}
		index[key] = len(responses)
		responses = append(responses, &processauditandcompliance.PACAuditResponse{
			Did:        normalizedDID,
			Component:  component.String(),
			CreatedAt:  latestAuditTrailTimestamp("", entries),
			AuditTrail: entries,
		})
	}
	sort.SliceStable(responses, func(i, j int) bool {
		if responses[i] == nil || responses[j] == nil {
			return responses[j] != nil
		}
		if responses[i].Did == responses[j].Did {
			return responses[i].Component < responses[j].Component
		}
		return responses[i].Did < responses[j].Did
	})
	return responses
}

func auditTrailEntriesFromHistory(history []datatype.AuditLogEntry) []*processauditandcompliance.PACResourceAuditTrailEntry {
	entries := make([]*processauditandcompliance.PACResourceAuditTrailEntry, 0, len(history))
	for _, entry := range history {
		if !base.IsAuditVisibleEventType(entry.EventType) {
			continue
		}
		entries = append(entries, &processauditandcompliance.PACResourceAuditTrailEntry{
			ID:               entry.ID,
			Component:        entry.Component,
			EventType:        entry.EventType,
			EventData:        entry.EventData,
			Did:              entry.DID,
			CreatedAt:        entry.CreatedAt.UTC().Format(time.RFC3339),
			ResLogPredCid:    entry.ResLogPredCID,
			GlobalLogPredCid: entry.GlobalLogPredCID,
		})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].CreatedAt == entries[j].CreatedAt {
			return entries[i].ID < entries[j].ID
		}
		return entries[i].CreatedAt < entries[j].CreatedAt
	})
	return entries
}

func appendUniqueAuditTrailEntries(existing []*processauditandcompliance.PACResourceAuditTrailEntry, candidates []*processauditandcompliance.PACResourceAuditTrailEntry) []*processauditandcompliance.PACResourceAuditTrailEntry {
	seen := map[string]bool{}
	for _, entry := range existing {
		seen[auditTrailEntryKey(entry)] = true
	}
	for _, entry := range candidates {
		key := auditTrailEntryKey(entry)
		if seen[key] {
			continue
		}
		seen[key] = true
		existing = append(existing, entry)
	}
	sort.SliceStable(existing, func(i, j int) bool {
		if existing[i] == nil || existing[j] == nil {
			return existing[j] != nil
		}
		if existing[i].CreatedAt == existing[j].CreatedAt {
			return existing[i].ID < existing[j].ID
		}
		return existing[i].CreatedAt < existing[j].CreatedAt
	})
	return existing
}

func auditResourceKey(component, did string) string {
	did = strings.TrimSpace(did)
	component = strings.TrimSpace(component)
	if did == "" || component == "" {
		return ""
	}
	return component + "\x00" + did
}

func auditTrailDID(entries []*processauditandcompliance.PACResourceAuditTrailEntry) string {
	for _, entry := range entries {
		if entry != nil && entry.Did != nil && strings.TrimSpace(*entry.Did) != "" {
			return strings.TrimSpace(*entry.Did)
		}
	}
	return ""
}

func auditTrailEntryKey(entry *processauditandcompliance.PACResourceAuditTrailEntry) string {
	if entry == nil {
		return ""
	}
	did := ""
	if entry.Did != nil {
		did = strings.TrimSpace(*entry.Did)
	}
	data := objectMap(entry.EventData)
	ruleID := stringFromMap(data, "ruleId", "rule_id")
	path := stringFromMap(data, "path", "semanticPath", "semantic_path")
	message := stringFromMap(data, "message")
	if ruleID != "" || path != "" || message != "" {
		return fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s\x00%s",
			strings.TrimSpace(entry.Component),
			strings.TrimSpace(entry.EventType),
			did,
			ruleID,
			path,
			message,
		)
	}
	return fmt.Sprintf("%s\x00%s\x00%d\x00%s",
		strings.TrimSpace(entry.Component),
		strings.TrimSpace(entry.EventType),
		entry.ID,
		did,
	)
}

func latestAuditTrailTimestamp(current string, entries []*processauditandcompliance.PACResourceAuditTrailEntry) string {
	latest := strings.TrimSpace(current)
	for _, entry := range entries {
		if entry == nil || strings.TrimSpace(entry.CreatedAt) == "" {
			continue
		}
		if latest == "" || entry.CreatedAt > latest {
			latest = entry.CreatedAt
		}
	}
	if latest == "" {
		return time.Now().UTC().Format(time.RFC3339)
	}
	return latest
}

func buildPACMFindings(run *processauditandcompliance.PACAuditRun, scope auditScopeConfig, createdAt time.Time) []*processauditandcompliance.PACAuditFinding {
	checks := []pacmCheck{}
	findings := make([]*processauditandcompliance.PACAuditFinding, 0, len(checks))
	for i, check := range checks {
		status := "PASS"
		message := fmt.Sprintf("%s check completed for %s audit scope.", check.title, scope.apiScope)
		if scope.apiScope == "SIGNATURE" {
			status = "SKIPPED"
			message = signatureValidationUnavailable
		}
		findingID := fmt.Sprintf("%s-finding-%02d", run.ID, i+1)
		finding := &processauditandcompliance.PACAuditFinding{
			ID:         findingID,
			AuditRunID: run.ID,
			Scope:      scope.apiScope,
			Check:      check.check,
			Status:     status,
			Title:      check.title,
			Message:    message,
			Component:  scope.component.String(),
			Evidence: map[string]any{
				"check":      check.check,
				"scope":      scope.apiScope,
				"component":  scope.component.String(),
				"source":     "PACM technical audit registry",
				"auditRunId": run.ID,
			},
			CreatedAt: createdAt.Format(time.RFC3339),
		}
		findings = append(findings, finding)
	}
	return findings
}

type pacmCheck struct{ check, title string }

func appendAuditEvent(run *processauditandcompliance.PACAuditRun, eventType, message string, createdAt time.Time) {
	run.Events = append(run.Events, &processauditandcompliance.PACAuditEvent{EventType: eventType, Message: message, CreatedAt: createdAt.UTC().Format(time.RFC3339)})
}

func (s *processAuditAndCompliancesrvc) AuditReport(ctx context.Context, p *processauditandcompliance.AuditReportPayload) (res any, err error) {
	log.Printf(ctx, "processAuditAndCompliance.audit_report")
	if p != nil && strings.TrimSpace(base.DerefString(p.Create)) != "" {
		return nil, processauditandcompliance.MakeBadRequest(fmt.Errorf("GET /pac/report is a query and does not create report data"))
	}
	if err := s.ensureAuditTables(ctx); err != nil {
		return nil, processauditandcompliance.MakeBadRequest(err)
	}
	format := reportFormat(base.DerefString(p.Format))
	if !validReportFormat(format) {
		return nil, processauditandcompliance.MakeBadRequest(fmt.Errorf("unsupported audit report format %q", format))
	}
	runID := strings.TrimSpace(base.DerefString(p.AuditRunID))
	if runID == "" {
		runs, err := s.queryAuditRuns(ctx, auditRunQuery{Scope: base.DerefString(p.Scope)})
		if err != nil {
			return nil, processauditandcompliance.MakeBadRequest(err)
		}
		return map[string]any{"reports": []any{}, "auditRuns": auditRunPayloads(runs)}, nil
	}
	run, err := s.readAuditRun(ctx, runID)
	if err != nil {
		return nil, processauditandcompliance.MakeBadRequest(err)
	}
	report := buildAuditReportFromRun(run, base.DerefString(p.Did), middleware.GetParticipantID(ctx), time.Now().UTC())
	report.Format = format
	return renderStoredAuditReport(report, format)
}

func (s *processAuditAndCompliancesrvc) IncidentReport(ctx context.Context, p *processauditandcompliance.IncidentReportPayload) (res any, err error) {
	log.Printf(ctx, "processAuditAndCompliance.incident_report")
	if err := s.ensureAuditTables(ctx); err != nil {
		return nil, processauditandcompliance.MakeInternalError(err)
	}
	format := reportFormat(base.DerefString(p.Format))
	if !validReportFormat(format) {
		return nil, processauditandcompliance.MakeBadRequest(fmt.Errorf("unsupported audit report format %q", format))
	}
	run, err := s.readAuditRun(ctx, strings.TrimSpace(p.AuditRunID))
	if err != nil {
		return nil, processauditandcompliance.MakeBadRequest(err)
	}
	report := buildAuditReportFromRun(run, "", middleware.GetParticipantID(ctx), time.Now().UTC())
	report.Format = format
	response, contentHash, err := renderStoredAuditReportWithHash(report, format)
	if err != nil {
		return nil, processauditandcompliance.MakeInternalError(err)
	}
	if err := s.persistAuditReport(ctx, run.ID, report.ReportID, format, contentHash, response); err != nil {
		return nil, processauditandcompliance.MakeInternalError(err)
	}
	appendAuditEvent(run, "AuditReportGenerated", fmt.Sprintf("PACM %s report %s generated", format, report.ReportID), time.Now().UTC())
	if err := s.persistAuditEvent(ctx, run.ID, run.Events[len(run.Events)-1]); err != nil {
		return nil, processauditandcompliance.MakeInternalError(err)
	}
	if err := s.persistReportGeneratedEvent(ctx, report, format, contentHash); err != nil {
		return nil, processauditandcompliance.MakeInternalError(err)
	}
	return response, nil
}

func (s *processAuditAndCompliancesrvc) Monitor(ctx context.Context, p *processauditandcompliance.MonitorPayload) (res any, err error) {
	log.Printf(ctx, "processAuditAndCompliance.monitor")
	if err := s.ensureAuditTables(ctx); err != nil {
		return nil, err
	}
	query := auditRunQuery{}
	if p != nil {
		query = auditRunQuery{Scope: base.DerefString(p.Scope), Status: base.DerefString(p.Status), From: base.DerefString(p.From), To: base.DerefString(p.To), AuditRunID: base.DerefString(p.AuditRunID), IncludeEvents: truthy(base.DerefString(p.IncludeEvents))}
	}
	runs, err := s.queryAuditRuns(ctx, query)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(query.AuditRunID) != "" && len(runs) == 1 {
		return auditRunPayload(runs[0]), nil
	}
	return map[string]any{"auditRuns": auditRunPayloads(runs)}, nil
}

func auditRunPayloads(runs []*processauditandcompliance.PACAuditRun) []map[string]any {
	payloads := make([]map[string]any, 0, len(runs))
	for _, run := range runs {
		payloads = append(payloads, auditRunPayload(run))
	}
	return payloads
}

func auditRunPayload(run *processauditandcompliance.PACAuditRun) map[string]any {
	if run == nil {
		return map[string]any{}
	}
	payload := map[string]any{
		"id":           run.ID,
		"auditRunId":   run.ID,
		"scope":        run.Scope,
		"status":       run.Status,
		"resultStatus": run.ResultStatus,
		"createdAt":    run.CreatedAt,
		"startedAt":    run.StartedAt,
		"auditedBy":    run.AuditedBy,
		"findings":     auditFindingPayloads(run.Findings),
	}
	if run.CompletedAt != nil {
		payload["completedAt"] = *run.CompletedAt
	}
	if len(run.Events) > 0 {
		payload["events"] = auditEventPayloads(run.Events)
	}
	if len(run.Resources) > 0 {
		payload["resources"] = run.Resources
	}
	return payload
}

func auditFindingPayloads(findings []*processauditandcompliance.PACAuditFinding) []map[string]any {
	payloads := make([]map[string]any, 0, len(findings))
	for _, finding := range findings {
		if finding == nil {
			continue
		}
		payload := map[string]any{
			"id":         finding.ID,
			"auditRunId": finding.AuditRunID,
			"scope":      finding.Scope,
			"check":      finding.Check,
			"status":     finding.Status,
			"title":      finding.Title,
			"message":    finding.Message,
			"component":  finding.Component,
			"evidence":   finding.Evidence,
			"createdAt":  finding.CreatedAt,
		}
		if finding.Did != nil {
			payload["did"] = *finding.Did
		}
		payloads = append(payloads, payload)
	}
	return payloads
}

func auditEventPayloads(events []*processauditandcompliance.PACAuditEvent) []map[string]any {
	payloads := make([]map[string]any, 0, len(events))
	for _, event := range events {
		if event == nil {
			continue
		}
		payloads = append(payloads, map[string]any{"eventType": event.EventType, "message": event.Message, "createdAt": event.CreatedAt})
	}
	return payloads
}

func reportFormat(raw string) string {
	format := strings.ToLower(strings.TrimSpace(raw))
	if format == "" {
		return "json"
	}
	return format
}

func validReportFormat(format string) bool {
	return format == "json" || format == "csv" || format == "pdf"
}

func renderStoredAuditReport(report auditReport, format string) (any, error) {
	response, _, err := renderStoredAuditReportWithHash(report, format)
	return response, err
}

func renderStoredAuditReportWithHash(report auditReport, format string) (any, string, error) {
	switch format {
	case "json":
		bytes, err := json.Marshal(report)
		if err != nil {
			return nil, "", err
		}
		report.ContentHash = hashBytes(bytes)
		return report, report.ContentHash, nil
	case "csv":
		bytes, err := renderAuditReportCSV(report)
		if err != nil {
			return nil, "", err
		}
		download := reportDownloadEnvelope(report, format, bytes, "text/csv")
		return download, download.ContentHash, nil
	case "pdf":
		bytes := renderAuditReportPDF(report)
		download := reportDownloadEnvelope(report, format, bytes, "application/pdf")
		return download, download.ContentHash, nil
	default:
		return nil, "", fmt.Errorf("unsupported audit report format %q", format)
	}
}

func buildAuditReportFromRun(run *processauditandcompliance.PACAuditRun, did, generatedBy string, generatedAt time.Time) auditReport {
	report := auditReport{AuditRunID: run.ID, RunID: run.ID, Scope: run.Scope, GeneratedAt: generatedAt.UTC().Format(time.RFC3339), GeneratedBy: generatedBy, Format: "json", DID: strings.TrimSpace(did), Resources: []auditReportResource{}, Events: []auditReportEvent{}, Findings: []auditReportFinding{}}
	if len(run.Resources) > 0 {
		report = buildAuditReport(run.Scope, did, generatedBy, generatedAt, run.Resources)
		report.AuditRunID = run.ID
		report.RunID = run.ID
	}
	resourceIndex := map[string]int{}
	for i, resource := range report.Resources {
		resourceIndex[resource.Component+"\x00"+resource.DID] = i
	}
	for _, finding := range run.Findings {
		if finding == nil {
			continue
		}
		entryDID := ""
		if finding.Did != nil {
			entryDID = *finding.Did
		}
		if report.DID != "" && entryDID != report.DID {
			continue
		}
		key := finding.Component + "\x00" + entryDID
		if _, ok := resourceIndex[key]; !ok {
			resourceIndex[key] = len(report.Resources)
			report.Resources = append(report.Resources, auditReportResource{DID: entryDID, Component: finding.Component})
		}
		idx := resourceIndex[key]
		report.Resources[idx].FindingCount++
		report.Findings = append(report.Findings, auditReportFinding{Timestamp: finding.CreatedAt, Component: finding.Component, EventType: "AuditCheckCompleted", DID: entryDID, RuleID: finding.Check, Title: finding.Title, Severity: finding.Status, Message: finding.Message, Requirement: finding.Scope + " audit", ActualValue: finding.Status, ExpectedValue: "PASS", Path: finding.Check})
	}
	for _, evt := range run.Events {
		if evt == nil || evt.EventType == "AuditReportGenerated" {
			continue
		}
		report.Events = append(report.Events, auditReportEvent{Timestamp: evt.CreatedAt, Actor: run.AuditedBy, Component: componenttype.ProcessAuditAndCompliance.String(), EventType: evt.EventType, Details: map[string]any{"message": evt.Message, "auditRunId": run.ID}})
	}
	report.Summary = summarizeAuditReport(report)
	report.ReportID = auditReportID(run.Scope, run.ID, generatedBy, generatedAt, report.Summary)
	return report
}

func (s *processAuditAndCompliancesrvc) ensureAuditTables(ctx context.Context) error {
	if s.DB == nil {
		return nil
	}
	_, err := s.DB.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS pacm_audit_runs (id VARCHAR(96) PRIMARY KEY, scope VARCHAR(32) NOT NULL, status VARCHAR(32) NOT NULL, result_status VARCHAR(32) NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, started_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP, completed_at TIMESTAMPTZ, audited_by VARCHAR(255), run_data JSONB NOT NULL);
CREATE INDEX IF NOT EXISTS idx_pacm_audit_runs_scope_status_created ON pacm_audit_runs(scope, status, created_at);
CREATE TABLE IF NOT EXISTS pacm_audit_findings (id VARCHAR(128) PRIMARY KEY, audit_run_id VARCHAR(96) NOT NULL REFERENCES pacm_audit_runs(id) ON DELETE CASCADE, scope VARCHAR(32) NOT NULL, check_name VARCHAR(128) NOT NULL, status VARCHAR(32) NOT NULL, title TEXT NOT NULL, message TEXT NOT NULL, component VARCHAR(64) NOT NULL, did VARCHAR(255), evidence JSONB NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE INDEX IF NOT EXISTS idx_pacm_audit_findings_run ON pacm_audit_findings(audit_run_id);
CREATE TABLE IF NOT EXISTS pacm_audit_events (id BIGSERIAL PRIMARY KEY, audit_run_id VARCHAR(96) NOT NULL REFERENCES pacm_audit_runs(id) ON DELETE CASCADE, event_type VARCHAR(64) NOT NULL, message TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE INDEX IF NOT EXISTS idx_pacm_audit_events_run ON pacm_audit_events(audit_run_id);
CREATE TABLE IF NOT EXISTS pacm_audit_reports (id VARCHAR(96) PRIMARY KEY, audit_run_id VARCHAR(96) NOT NULL REFERENCES pacm_audit_runs(id) ON DELETE CASCADE, format VARCHAR(16) NOT NULL, content_hash VARCHAR(128) NOT NULL, report_data JSONB NOT NULL, generated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE INDEX IF NOT EXISTS idx_pacm_audit_reports_run ON pacm_audit_reports(audit_run_id, generated_at DESC);`)
	return err
}

func (s *processAuditAndCompliancesrvc) persistAuditRun(ctx context.Context, run *processauditandcompliance.PACAuditRun) error {
	if s.DB == nil {
		return nil
	}
	bytes, err := json.Marshal(run)
	if err != nil {
		return err
	}
	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf(ctx, "rollback failed: %v", err)
		}
	}()
	completedAt := (*time.Time)(nil)
	if run.CompletedAt != nil {
		if parsed, err := time.Parse(time.RFC3339, *run.CompletedAt); err == nil {
			completedAt = &parsed
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO pacm_audit_runs (id, scope, status, result_status, created_at, started_at, completed_at, audited_by, run_data) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, run.ID, run.Scope, run.Status, run.ResultStatus, parseTimeOrNow(run.CreatedAt), parseTimeOrNow(run.StartedAt), completedAt, run.AuditedBy, bytes)
	if err != nil {
		return err
	}
	for _, finding := range run.Findings {
		evidence, err := json.Marshal(finding.Evidence)
		if err != nil {
			return err
		}
		did := any(nil)
		if finding.Did != nil {
			did = *finding.Did
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO pacm_audit_findings (id, audit_run_id, scope, check_name, status, title, message, component, did, evidence, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, finding.ID, run.ID, finding.Scope, finding.Check, finding.Status, finding.Title, finding.Message, finding.Component, did, evidence, parseTimeOrNow(finding.CreatedAt))
		if err != nil {
			return err
		}
	}
	for _, event := range run.Events {
		if err := insertAuditEvent(ctx, tx, run.ID, event); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *processAuditAndCompliancesrvc) persistAuditEvent(ctx context.Context, runID string, event *processauditandcompliance.PACAuditEvent) error {
	if s.DB == nil {
		return nil
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO pacm_audit_events (audit_run_id, event_type, message, created_at) VALUES ($1,$2,$3,$4)`, runID, event.EventType, event.Message, parseTimeOrNow(event.CreatedAt))
	return err
}

func insertAuditEvent(ctx context.Context, tx *sqlx.Tx, runID string, event *processauditandcompliance.PACAuditEvent) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO pacm_audit_events (audit_run_id, event_type, message, created_at) VALUES ($1,$2,$3,$4)`, runID, event.EventType, event.Message, parseTimeOrNow(event.CreatedAt))
	return err
}

func (s *processAuditAndCompliancesrvc) persistAuditReport(ctx context.Context, runID, reportID, format, contentHash string, response any) error {
	if s.DB == nil {
		return nil
	}
	bytes, err := json.Marshal(response)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO pacm_audit_reports (id, audit_run_id, format, content_hash, report_data, generated_at) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (id) DO UPDATE SET report_data = EXCLUDED.report_data, content_hash = EXCLUDED.content_hash, generated_at = EXCLUDED.generated_at`, reportID, runID, format, contentHash, bytes, time.Now().UTC())
	return err
}

func (s *processAuditAndCompliancesrvc) readAuditRun(ctx context.Context, id string) (*processauditandcompliance.PACAuditRun, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("auditRunId is required")
	}
	runs, err := s.queryAuditRuns(ctx, auditRunQuery{AuditRunID: id, IncludeEvents: true})
	if err != nil {
		return nil, err
	}
	if len(runs) == 0 {
		return nil, fmt.Errorf("audit run %s not found", id)
	}
	return runs[0], nil
}

func (s *processAuditAndCompliancesrvc) queryAuditRuns(ctx context.Context, query auditRunQuery) ([]*processauditandcompliance.PACAuditRun, error) {
	if s.DB == nil {
		return []*processauditandcompliance.PACAuditRun{}, nil
	}
	clauses := []string{"1=1"}
	args := []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}
	if scope := strings.TrimSpace(query.Scope); scope != "" {
		cfg, err := resolveAuditScope(scope)
		if err != nil {
			return nil, err
		}
		add("scope = $%d", cfg.apiScope)
	}
	if status := strings.ToUpper(strings.TrimSpace(query.Status)); status != "" {
		add("status = $%d", status)
	}
	if id := strings.TrimSpace(query.AuditRunID); id != "" {
		add("id = $%d", id)
	}
	if from := strings.TrimSpace(query.From); from != "" {
		t, err := parseFilterTime(from, false)
		if err != nil {
			return nil, err
		}
		add("created_at >= $%d", t)
	}
	if to := strings.TrimSpace(query.To); to != "" {
		t, err := parseFilterTime(to, true)
		if err != nil {
			return nil, err
		}
		add("created_at <= $%d", t)
	}
	statement := `SELECT id, scope, status, result_status, created_at, started_at, completed_at, COALESCE(audited_by, '') AS audited_by, run_data FROM pacm_audit_runs WHERE ` + strings.Join(clauses, " AND ") + ` ORDER BY created_at DESC LIMIT 200`
	rows := []pacmAuditRunRow{}
	if err := s.DB.SelectContext(ctx, &rows, statement, args...); err != nil {
		return nil, err
	}
	runs := make([]*processauditandcompliance.PACAuditRun, 0, len(rows))
	for _, row := range rows {
		run := &processauditandcompliance.PACAuditRun{}
		if err := json.Unmarshal(row.RunData, run); err != nil {
			return nil, err
		}
		if query.IncludeEvents || strings.TrimSpace(query.AuditRunID) != "" {
			events, err := s.readAuditEvents(ctx, run.ID)
			if err != nil {
				return nil, err
			}
			run.Events = events
		}
		runs = append(runs, run)
	}
	return runs, nil
}

func (s *processAuditAndCompliancesrvc) readAuditEvents(ctx context.Context, runID string) ([]*processauditandcompliance.PACAuditEvent, error) {
	rows := []struct {
		EventType string    `db:"event_type"`
		Message   string    `db:"message"`
		CreatedAt time.Time `db:"created_at"`
	}{}
	if err := s.DB.SelectContext(ctx, &rows, `SELECT event_type, message, created_at FROM pacm_audit_events WHERE audit_run_id = $1 ORDER BY created_at, id`, runID); err != nil {
		return nil, err
	}
	events := make([]*processauditandcompliance.PACAuditEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, &processauditandcompliance.PACAuditEvent{EventType: row.EventType, Message: row.Message, CreatedAt: row.CreatedAt.UTC().Format(time.RFC3339)})
	}
	return events, nil
}

func (s *processAuditAndCompliancesrvc) persistReportGeneratedEvent(ctx context.Context, report auditReport, format string, contentHash string) error {
	if s.DB == nil {
		return nil
	}
	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}

	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			log.Printf(ctx, "rollback failed: %v", err)
		}
	}()
	evt := pacevent.ReportGeneratedEvent{ReportID: report.ReportID, Scope: report.Scope, Format: format, DID: report.DID, GeneratedBy: report.GeneratedBy, GeneratedAt: time.Now().UTC(), ContentHash: contentHash, Summary: map[string]int{"totalEvents": report.Summary.TotalEvents, "totalChecks": report.Summary.TotalChecks, "passed": report.Summary.Passed, "failed": report.Summary.Failed, "warnings": report.Summary.Warnings, "needsReview": report.Summary.NeedsReview}, HolderDID: middleware.GetHolderDID(ctx), UserRoles: middleware.GetUserRoles(ctx)}
	if err := baseevent.Create(ctx, tx, evt, componenttype.ProcessAuditAndCompliance); err != nil {
		return fmt.Errorf("could not create report event: %w", err)
	}
	return tx.Commit()
}

func parseTimeOrNow(value string) time.Time {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Now().UTC()
	}
	return t.UTC()
}

func parseFilterTime(value string, endOfDay bool) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.UTC(), nil
	}
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, err
	}
	if endOfDay {
		return t.Add(24*time.Hour - time.Nanosecond).UTC(), nil
	}
	return t.UTC(), nil
}

func truthy(value string) bool {
	v := strings.ToLower(strings.TrimSpace(value))
	return v == "true" || v == "1" || v == "yes"
}

func newPACMID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(b[:])
}
