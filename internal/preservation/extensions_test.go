package preservation

import (
	"testing"
	"time"
)

func TestAssessmentBatchAtomicityAndPreflight(t *testing.T) {
	at := time.Now().UTC().Add(-time.Hour)
	incident, err := CreateIncident(NewIncident{IncidentID: "INC-batch-test", BatchCode: "BATCH-batch", Symptoms: []string{"异味"}, Environment: EnvironmentSnapshot{TemperatureCelsius: 20, RelativeHumidity: 50, StorageLocation: "vault", CapturedAt: at}, CreatedAt: at})
	if err != nil {
		t.Fatal(err)
	}
	batch := []MediaAssessment{
		{AssessmentID: "assessment-z", MediaID: "TAPE-Z", SampleRole: "control", ObservedBy: "observer-01", ObservedAt: at},
		{AssessmentID: "assessment-a", MediaID: "TAPE-A", SampleRole: "target", VisualGrade: 2, ObservedBy: "observer-01", ObservedAt: at},
		{AssessmentID: "assessment-bad", MediaID: "TAPE-B", SampleRole: "neighbor", VisualGrade: 4, ObservedBy: "observer-01", ObservedAt: at},
	}
	if err := incident.AddAssessmentBatch(batch, at); err == nil {
		t.Fatal("含越界等级的批次未被拒绝")
	}
	if len(incident.Assessments) != 0 {
		t.Fatalf("失败批次写入了观察: %+v", incident.Assessments)
	}
	if err := incident.AddAssessmentBatch(batch[:2], at); err != nil {
		t.Fatal(err)
	}
	if incident.Assessments[0].MediaID != "TAPE-A" {
		t.Fatalf("批次结果未确定性排序: %+v", incident.Assessments)
	}
	preflight, err := incident.BoundaryPreflight([]string{"TAPE-Z", "TAPE-A", "TAPE-MISSING"})
	if err != nil {
		t.Fatal(err)
	}
	if preflight.CanFreeze || len(preflight.UnobservedMediaIDs) != 1 || preflight.UnobservedMediaIDs[0] != "TAPE-MISSING" {
		t.Fatalf("圈定缺口异常: %+v", preflight)
	}
	preflight, err = incident.BoundaryPreflight([]string{"TAPE-Z", "TAPE-A"})
	if err != nil || !preflight.CanFreeze || len(preflight.AffectedMediaIDs) != 1 || preflight.AffectedMediaIDs[0] != "TAPE-A" {
		t.Fatalf("圈定预检异常: %+v err=%v", preflight, err)
	}
}

func TestTreatmentInterruptionResumeAndCompletion(t *testing.T) {
	at := time.Now().UTC().Add(-2 * time.Hour)
	incident := readyForTreatment(t, at)
	start := at.Add(20 * time.Minute)
	if err := incident.AddTreatment(TreatmentRecord{RecordID: "record-interrupted", MediaID: "TAPE-A", OperatorID: "operator-01", StartedAt: start, ActualParameters: map[string]string{"temperature": "40C"}}, start); err != nil {
		t.Fatal(err)
	}
	stopAt := start.Add(10 * time.Minute)
	event := TreatmentInterruption{InterruptionID: "stop-01", OccurredAt: stopAt, StopCondition: "出现卷曲", OnsiteObservation: "边缘轻微卷曲", ImmediateAction: "停止加热并隔离", ReportedBy: "operator-01"}
	if err := incident.InterruptTreatment("record-interrupted", event, stopAt); err != nil {
		t.Fatal(err)
	}
	if incident.Treatments[0].Status != TreatmentPendingDisposition {
		t.Fatalf("中止状态异常: %+v", incident.Treatments[0])
	}
	if err := incident.CompleteTreatment("record-interrupted", "operator-01", stopAt.Add(time.Minute), nil, TreatmentCompleted, stopAt); err == nil {
		t.Fatal("未闭环中止后错误允许完成")
	}
	resumeAt := stopAt.Add(5 * time.Minute)
	if err := incident.ResumeTreatment("record-interrupted", "stop-01", "engineer-02", "确认卷曲未扩展，降低温度继续", map[string]string{"temperature": "35C"}, resumeAt, resumeAt); err != nil {
		t.Fatal(err)
	}
	completedAt := resumeAt.Add(10 * time.Minute)
	if err := incident.CompleteTreatment("record-interrupted", "operator-01", completedAt, []Deviation{{Description: "温度下调", Explanation: "依据风险处置改为 35C"}}, TreatmentCompleted, completedAt); err != nil {
		t.Fatal(err)
	}
	if incident.State != StateVerification {
		t.Fatalf("完整闭环后状态 = %s", incident.State)
	}
}

func TestRetreatmentRoundAndCrossRoundSeparation(t *testing.T) {
	at := time.Now().UTC().Add(-3 * time.Hour)
	incident := readyForTreatment(t, at)
	complete := at.Add(30 * time.Minute)
	if err := incident.AddTreatment(TreatmentRecord{RecordID: "record-round-1", MediaID: "TAPE-A", OperatorID: "operator-round-1", StartedAt: at.Add(20 * time.Minute), CompletedAt: &complete, ActualParameters: map[string]string{"temperature": "40C"}, Outcome: TreatmentCompleted}, complete); err != nil {
		t.Fatal(err)
	}
	if err := incident.AddVerification(ReadabilityVerification{VerificationID: "verification-round-1", MediaID: "TAPE-A", DeviceID: "drive-01", CalibrationRef: "CAL-01", ErrorRate: .03, ReadableDurationSecs: 120, SampleDigest: "sha256:first", VerifiedBy: "verifier-01", VerifiedAt: complete}, complete); err != nil {
		t.Fatal(err)
	}
	decision := DispositionDecision{DecisionID: "decision-retreat-1", ReviewerID: "reviewer-01", Decision: DecisionRetreat, Rationale: "复验建议补治", EvidenceChecks: completeEvidence(), SignedAt: complete}
	if err := incident.Decide(decision, complete); err != nil {
		t.Fatal(err)
	}
	plan := StabilizationPlan{PlanID: "plan-round-2", RetreatDecisionID: decision.DecisionID, EnvironmentTarget: "20C", CleaningMethod: "低张力清洁", BakingLimit: "40C", StopConditions: []string{"出现卷曲"}, AuthorID: "author-02"}
	if err := incident.SubmitPlan(plan, complete.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if len(incident.RoundHistory) != 1 || incident.RoundHistory[0].Treatments[0].RecordID != "record-round-1" || len(incident.Treatments) != 0 {
		t.Fatalf("首轮证据未正确固化: %+v", incident)
	}
	if err := incident.ApprovePlan("approver-02", complete.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	complete2 := complete.Add(20 * time.Minute)
	if err := incident.AddTreatment(TreatmentRecord{RecordID: "record-round-2", MediaID: "TAPE-A", OperatorID: "operator-round-2", StartedAt: complete.Add(3 * time.Minute), CompletedAt: &complete2, ActualParameters: map[string]string{"temperature": "35C"}, Outcome: TreatmentCompleted}, complete2); err != nil {
		t.Fatal(err)
	}
	if err := incident.AddVerification(ReadabilityVerification{VerificationID: "verification-round-2", MediaID: "TAPE-A", DeviceID: "drive-01", CalibrationRef: "CAL-02", ErrorRate: .005, ReadableDurationSecs: 120, SampleDigest: "sha256:second", VerifiedBy: "verifier-02", VerifiedAt: complete2}, complete2); err != nil {
		t.Fatal(err)
	}
	release := DispositionDecision{DecisionID: "decision-release-2", ReviewerID: "operator-round-1", Decision: DecisionRelease, Rationale: "全部通过", EvidenceChecks: completeEvidence(), SignedAt: complete2}
	if err := incident.Decide(release, complete2); err == nil {
		t.Fatal("首轮处理人员被允许签发第二轮最终裁定")
	}
	release.ReviewerID = "reviewer-final"
	if err := incident.Decide(release, complete2); err != nil {
		t.Fatal(err)
	}
}

func readyForTreatment(t *testing.T, at time.Time) *PreservationIncident {
	t.Helper()
	incident, err := CreateIncident(NewIncident{IncidentID: "INC-treatment-extension", BatchCode: "BATCH-treatment", Symptoms: []string{"黏连"}, Environment: EnvironmentSnapshot{TemperatureCelsius: 20, RelativeHumidity: 50, StorageLocation: "vault", CapturedAt: at}, CreatedAt: at})
	if err != nil {
		t.Fatal(err)
	}
	if err := incident.AddAssessmentBatch([]MediaAssessment{{AssessmentID: "assessment-target-x", MediaID: "TAPE-A", SampleRole: "target", AdhesionGrade: 2, ObservedBy: "observer-01", ObservedAt: at}, {AssessmentID: "assessment-control-x", MediaID: "TAPE-C", SampleRole: "control", ObservedBy: "observer-01", ObservedAt: at}}, at); err != nil {
		t.Fatal(err)
	}
	if err := incident.FreezeBoundary([]string{"TAPE-A", "TAPE-C"}, at); err != nil {
		t.Fatal(err)
	}
	if err := incident.SubmitPlan(StabilizationPlan{PlanID: "plan-round-1", EnvironmentTarget: "20C", CleaningMethod: "清洁", BakingLimit: "45C", StopConditions: []string{"出现卷曲"}, AuthorID: "author-01"}, at); err != nil {
		t.Fatal(err)
	}
	if err := incident.ApprovePlan("approver-01", at.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	return incident
}
