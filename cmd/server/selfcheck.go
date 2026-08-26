package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"tape-preservation-incident-api/internal/app"
	"tape-preservation-incident-api/internal/preservation"
	"tape-preservation-incident-api/internal/store"
)

func runSelfCheck(parent context.Context, cfg config) error {
	tempDir, err := os.MkdirTemp("", "tape-preservation-self-check-*")
	if err != nil {
		return fmt.Errorf("创建自检临时目录: %w", err)
	}
	defer os.RemoveAll(tempDir)
	cfg.dataDir = tempDir
	rt, err := buildRuntime(cfg)
	if err != nil {
		return err
	}
	serveErrors := make(chan error, 1)
	go rt.serve(serveErrors)
	ctx, cancel := context.WithTimeout(parent, cfg.selfCheckTimeout)
	defer cancel()
	flowErr := exerciseWorkflow(ctx, cfg.addr)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	shutdownErr := rt.shutdown(shutdownCtx)
	shutdownCancel()
	serveErr := <-serveErrors
	if flowErr != nil {
		return flowErr
	}
	if shutdownErr != nil {
		return fmt.Errorf("关闭自检服务: %w", shutdownErr)
	}
	if serveErr != nil {
		return fmt.Errorf("自检服务运行: %w", serveErr)
	}
	return nil
}

func exerciseWorkflow(ctx context.Context, address string) error {
	client := newCheckClient(address)
	if err := client.request(ctx, http.MethodGet, "/healthz", nil, http.StatusOK, nil); err != nil {
		return err
	}
	baseTime := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	create := app.CreateIncidentCommand{RequestID: "self-create-001", ActorID: "engineer-001", BatchCode: "BATCH-2026-001", Symptoms: []string{"磁带表面黏连", "驱动器出现污染"}, EnvironmentSnapshot: preservation.EnvironmentSnapshot{TemperatureCelsius: 22.5, RelativeHumidity: 61, StorageLocation: "vault-A", CapturedAt: baseTime}}
	var created app.CommandResult
	if err := client.request(ctx, http.MethodPost, "/api/v1/incidents", create, http.StatusCreated, &created); err != nil {
		return err
	}
	if created.Incident == nil || created.Incident.Revision != 1 {
		return fmt.Errorf("建档响应缺少首个修订")
	}
	incidentID := created.Incident.IncidentID
	var replay app.CommandResult
	if err := client.request(ctx, http.MethodPost, "/api/v1/incidents", create, http.StatusOK, &replay); err != nil {
		return fmt.Errorf("幂等建档校验失败: %w", err)
	}
	if !replay.Replayed || replay.Incident.Revision != 1 {
		return fmt.Errorf("重复 request_id 未返回原始结果")
	}
	conflict := app.AddAssessmentCommand{CommandMeta: app.CommandMeta{RequestID: "self-conflict-001", ActorID: "engineer-001", ExpectedRevision: 99}, Assessment: sampleAssessment("assessment-conflict", "TAPE-X", "target", "engineer-001", baseTime)}
	if err := client.request(ctx, http.MethodPost, incidentPath(incidentID, "/assessments"), conflict, http.StatusConflict, nil); err != nil {
		return fmt.Errorf("修订冲突校验失败: %w", err)
	}
	revision := int64(1)
	assessments := []preservation.MediaAssessment{
		sampleAssessment("assessment-target", "TAPE-001", "target", "engineer-001", baseTime.Add(10*time.Minute)),
		sampleAssessment("assessment-control", "TAPE-CTRL", "control", "engineer-001", baseTime.Add(11*time.Minute)),
	}
	batch := app.AddAssessmentBatchCommand{CommandMeta: meta("self-assessment-batch", "engineer-001", revision), Assessments: assessments}
	result, err := postResult(ctx, client, incidentPath(incidentID, "/assessments/batch"), batch)
	if err != nil {
		return err
	}
	revision = result.Incident.Revision
	var preflight app.BoundaryPreflightResponse
	preflightCmd := app.BoundaryPreflightCommand{ActorID: "engineer-001", ExpectedRevision: revision, ExpectedMediaIDs: []string{"TAPE-001", "TAPE-CTRL"}}
	if err := client.request(ctx, http.MethodPost, incidentPath(incidentID, "/boundary/preflight"), preflightCmd, http.StatusOK, &preflight); err != nil {
		return err
	}
	if !preflight.CanFreeze || preflight.Revision != revision {
		return fmt.Errorf("圈定预检未返回可冻结且保持修订")
	}
	boundary := app.FreezeBoundaryCommand{CommandMeta: meta("self-boundary-001", "engineer-001", revision), ExpectedMediaIDs: []string{"TAPE-001", "TAPE-CTRL"}}
	result, err = postResult(ctx, client, incidentPath(incidentID, "/boundary"), boundary)
	if err != nil {
		return err
	}
	revision = result.Incident.Revision
	plan := app.SubmitPlanCommand{CommandMeta: meta("self-plan-001", "engineer-001", revision), Plan: preservation.StabilizationPlan{PlanID: "plan-001", EnvironmentTarget: "20°C、相对湿度 40%", CleaningMethod: "无纺布低张力表面清洁", BakingLimit: "不超过 48°C 且不超过 8 小时", StopConditions: []string{"出现卷曲或异味加重立即停止"}}}
	result, err = postResult(ctx, client, incidentPath(incidentID, "/plans"), plan)
	if err != nil {
		return err
	}
	revision = result.Incident.Revision
	approval := app.ApprovePlanCommand{CommandMeta: meta("self-approve-001", "approver-002", revision), ApprovedAt: baseTime.Add(30 * time.Minute)}
	result, err = postResult(ctx, client, incidentPath(incidentID, "/plans/approval"), approval)
	if err != nil {
		return err
	}
	revision = result.Incident.Revision
	started := baseTime.Add(40 * time.Minute)
	completed := baseTime.Add(70 * time.Minute)
	treatment := app.AddTreatmentCommand{CommandMeta: meta("self-treatment-start-001", "operator-003", revision), Treatment: preservation.TreatmentRecord{RecordID: "record-001", MediaID: "TAPE-001", StartedAt: started, ActualParameters: map[string]string{"temperature": "42°C", "duration": "30m"}}}
	result, err = postResult(ctx, client, incidentPath(incidentID, "/treatments"), treatment)
	if err != nil {
		return err
	}
	revision = result.Incident.Revision
	interruption := app.InterruptTreatmentCommand{CommandMeta: meta("self-treatment-stop-001", "operator-003", revision), Interruption: preservation.TreatmentInterruption{InterruptionID: "interruption-001", OccurredAt: started.Add(10 * time.Minute), StopCondition: "出现卷曲或异味加重立即停止", OnsiteObservation: "边缘出现轻微卷曲", ImmediateAction: "停止升温并隔离观察"}}
	result, err = postResult(ctx, client, incidentPath(incidentID, "/treatments/record-001/interruptions"), interruption)
	if err != nil {
		return err
	}
	revision = result.Incident.Revision
	resume := app.ResumeTreatmentCommand{CommandMeta: meta("self-treatment-resume-001", "engineer-006", revision), InterruptionID: "interruption-001", RiskDisposition: "确认卷曲稳定，降低温度继续", ParameterAdjustments: map[string]string{"temperature": "38°C"}, ResumedAt: started.Add(15 * time.Minute)}
	result, err = postResult(ctx, client, incidentPath(incidentID, "/treatments/record-001/resume"), resume)
	if err != nil {
		return err
	}
	revision = result.Incident.Revision
	complete := app.CompleteTreatmentCommand{CommandMeta: meta("self-treatment-complete-001", "operator-003", revision), CompletedAt: completed, Deviations: []preservation.Deviation{{Description: "中止后降温", Explanation: "按风险处置降至 38°C 并完成观察"}}, Outcome: "completed"}
	result, err = postResult(ctx, client, incidentPath(incidentID, "/treatments/record-001/complete"), complete)
	if err != nil {
		return err
	}
	revision = result.Incident.Revision
	verification := app.AddVerificationCommand{CommandMeta: meta("self-verify-001", "verifier-004", revision), Verification: preservation.ReadabilityVerification{VerificationID: "verification-001", MediaID: "TAPE-001", DeviceID: "drive-001", CalibrationRef: "CAL-2026-008", ErrorRate: 0.005, ReadableDurationSecs: 120, SampleDigest: "sha256:sample-evidence-001", VerifiedAt: baseTime.Add(90 * time.Minute)}}
	result, err = postResult(ctx, client, incidentPath(incidentID, "/verifications"), verification)
	if err != nil {
		return err
	}
	revision = result.Incident.Revision
	decision := app.DecideCommand{CommandMeta: meta("self-decision-001", "reviewer-005", revision), Decision: preservation.DispositionDecision{DecisionID: "decision-001", Decision: "release", Rationale: "所有受影响介质均完成稳定化且复验通过", EvidenceChecks: map[string]bool{"responsibility_separation": true, "treatments_complete": true, "calibration_evidence": true, "verification_complete": true, "audit_reviewed": true}, SignedAt: baseTime.Add(100 * time.Minute)}}
	result, err = postResult(ctx, client, incidentPath(incidentID, "/decisions"), decision)
	if err != nil {
		return err
	}
	revision = result.Incident.Revision
	if err := client.request(ctx, http.MethodGet, incidentPath(incidentID, "/archive/manifest"), nil, http.StatusConflict, nil); err != nil {
		return fmt.Errorf("未封存清单门禁失败: %w", err)
	}
	seal := app.SealCommand{CommandMeta: meta("self-seal-001", "reviewer-005", revision), SealedAt: baseTime.Add(110 * time.Minute)}
	result, err = postResult(ctx, client, incidentPath(incidentID, "/seal"), seal)
	if err != nil {
		return err
	}
	if result.Incident.State != preservation.StateSealed || result.Incident.Decision.ArchiveDigest == "" {
		return fmt.Errorf("封存未生成档案摘要")
	}
	var integrity store.IntegrityResult
	if err := client.request(ctx, http.MethodGet, incidentPath(incidentID, "/archive/verify"), nil, http.StatusOK, &integrity); err != nil {
		return err
	}
	if !integrity.Valid || !integrity.ArchiveVerified || integrity.AuditEvents != 12 {
		return fmt.Errorf("档案完整性结果不符合预期: %+v", integrity)
	}
	var manifest preservation.ArchiveManifest
	headers, err := client.requestWithHeaders(ctx, http.MethodGet, incidentPath(incidentID, "/archive/manifest"), nil, http.StatusOK, &manifest, nil)
	if err != nil {
		return err
	}
	if manifest.ArchiveDigest == "" || headers.Get("ETag") != "\""+manifest.ArchiveDigest+"\"" {
		return fmt.Errorf("规范档案清单缺少匹配的 ETag")
	}
	if _, err := client.requestWithHeaders(ctx, http.MethodGet, incidentPath(incidentID, "/archive/manifest"), nil, http.StatusNotModified, nil, map[string]string{"If-None-Match": headers.Get("ETag")}); err != nil {
		return err
	}
	if err := client.request(ctx, http.MethodGet, incidentPath(incidentID, "/archive/manifest")+"?expected_digest=wrong-digest", nil, http.StatusConflict, nil); err != nil {
		return err
	}
	var round struct {
		Round preservation.TreatmentRoundEvidence `json:"round"`
	}
	if err := client.request(ctx, http.MethodGet, incidentPath(incidentID, "/rounds/1"), nil, http.StatusOK, &round); err != nil {
		return err
	}
	if round.Round.RoundNumber != 1 || len(round.Round.Treatments) != 1 {
		return fmt.Errorf("轮次明细不完整")
	}
	var timeline store.TimelinePage
	if err := client.request(ctx, http.MethodGet, incidentPath(incidentID, "/timeline")+"?limit=3", nil, http.StatusOK, &timeline); err != nil {
		return err
	}
	if len(timeline.Items) != 3 || timeline.NextCursor != 3 {
		return fmt.Errorf("时间线分页结果不符合预期")
	}
	sealedMutation := app.SubmitPlanCommand{CommandMeta: meta("self-after-seal", "engineer-001", result.Incident.Revision), Plan: preservation.StabilizationPlan{PlanID: "plan-after-seal"}}
	if err := client.request(ctx, http.MethodPost, incidentPath(incidentID, "/plans"), sealedMutation, http.StatusConflict, nil); err != nil {
		return fmt.Errorf("封存后拒绝修改校验失败: %w", err)
	}
	return nil
}

func sampleAssessment(id, mediaID, role, actor string, at time.Time) preservation.MediaAssessment {
	grade := 0
	contamination := false
	if role == "target" {
		grade = 2
		contamination = true
	}
	return preservation.MediaAssessment{AssessmentID: id, MediaID: mediaID, SampleRole: role, VisualGrade: grade, OdorGrade: grade, AdhesionGrade: grade, SheddingGrade: grade, DriveContamination: contamination, ObservedBy: actor, ObservedAt: at}
}

func meta(requestID, actor string, revision int64) app.CommandMeta {
	return app.CommandMeta{RequestID: requestID, ActorID: actor, ExpectedRevision: revision}
}

func incidentPath(incidentID, suffix string) string {
	return "/api/v1/incidents/" + url.PathEscape(incidentID) + suffix
}

func postResult(ctx context.Context, client *checkClient, path string, payload any) (app.CommandResult, error) {
	var result app.CommandResult
	err := client.request(ctx, http.MethodPost, path, payload, http.StatusOK, &result)
	return result, err
}
