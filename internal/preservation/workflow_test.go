package preservation

import (
	"testing"
	"time"
)

func TestCompleteReleaseWorkflow(t *testing.T) {
	at := time.Now().UTC().Add(-time.Hour)
	incident, err := CreateIncident(NewIncident{IncidentID: "INC-test-release", BatchCode: "BATCH-test", Symptoms: []string{"黏连"}, Environment: EnvironmentSnapshot{TemperatureCelsius: 21, RelativeHumidity: 58, StorageLocation: "vault-test", CapturedAt: at}, CreatedAt: at})
	if err != nil {
		t.Fatal(err)
	}
	addAssessment(t, incident, MediaAssessment{AssessmentID: "assessment-target", MediaID: "TAPE-01", SampleRole: "target", VisualGrade: 2, ObservedBy: "observer-01", ObservedAt: at}, at)
	addAssessment(t, incident, MediaAssessment{AssessmentID: "assessment-control", MediaID: "TAPE-02", SampleRole: "control", ObservedBy: "observer-01", ObservedAt: at}, at)
	if err := incident.FreezeBoundary([]string{"TAPE-01", "TAPE-02"}, at); err != nil {
		t.Fatal(err)
	}
	if len(incident.AffectedMediaIDs) != 1 || incident.AffectedMediaIDs[0] != "TAPE-01" {
		t.Fatalf("意外冻结清单: %v", incident.AffectedMediaIDs)
	}
	plan := StabilizationPlan{PlanID: "plan-test", EnvironmentTarget: "20°C", CleaningMethod: "低张力清洁", BakingLimit: "48°C/8h", StopConditions: []string{"出现卷曲"}, AuthorID: "author-01"}
	if err := incident.SubmitPlan(plan, at); err != nil {
		t.Fatal(err)
	}
	if err := incident.ApprovePlan("approver-02", at); err != nil {
		t.Fatal(err)
	}
	completed := at.Add(20 * time.Minute)
	record := TreatmentRecord{RecordID: "record-test", MediaID: "TAPE-01", OperatorID: "operator-03", StartedAt: at.Add(10 * time.Minute), CompletedAt: &completed, ActualParameters: map[string]string{"temperature": "42°C"}, Outcome: "completed"}
	if err := incident.AddTreatment(record, completed); err != nil {
		t.Fatal(err)
	}
	if incident.State != StateVerification {
		t.Fatalf("处理后状态 = %s", incident.State)
	}
	verification := ReadabilityVerification{VerificationID: "verification-test", MediaID: "TAPE-01", DeviceID: "drive-test", CalibrationRef: "CAL-test", ErrorRate: 0.009, ReadableDurationSecs: 60, SampleDigest: "sha256:test", VerifiedBy: "verifier-04", VerifiedAt: completed}
	if err := incident.AddVerification(verification, completed); err != nil {
		t.Fatal(err)
	}
	decision := DispositionDecision{DecisionID: "decision-test", ReviewerID: "reviewer-05", Decision: DecisionRelease, Rationale: "复验满足固定规则", EvidenceChecks: completeEvidence(), SignedAt: completed}
	if err := incident.Decide(decision, completed); err != nil {
		t.Fatal(err)
	}
	if err := incident.MarkSealed(completed); err != nil {
		t.Fatal(err)
	}
	if incident.State != StateSealed || incident.SealedAt == nil {
		t.Fatalf("未正确封存: %+v", incident)
	}
	if err := incident.SubmitPlan(plan, completed); err == nil {
		t.Fatal("封存后修改未被拒绝")
	}
}

func TestDecisionAndSeparationGates(t *testing.T) {
	incident := readyForDecision(t, RecommendationRetreat)
	release := DispositionDecision{DecisionID: "decision-release", ReviewerID: "reviewer-09", Decision: DecisionRelease, Rationale: "尝试错误放行", EvidenceChecks: completeEvidence(), SignedAt: time.Now().UTC()}
	if err := incident.Decide(release, time.Now().UTC()); err == nil {
		t.Fatal("补治建议被错误放行")
	}
	retreat := release
	retreat.DecisionID = "decision-retreat"
	retreat.Decision = DecisionRetreat
	if err := incident.Decide(retreat, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if incident.State != StateRemediation {
		t.Fatalf("补治裁定状态 = %s", incident.State)
	}

	incident = readyForDecision(t, RecommendationPass)
	badReviewer := release
	badReviewer.DecisionID = "decision-bad-reviewer"
	badReviewer.ReviewerID = "operator-03"
	if err := incident.Decide(badReviewer, time.Now().UTC()); err == nil {
		t.Fatal("参与处理者被允许作最终裁定")
	}
	missingEvidence := release
	missingEvidence.DecisionID = "decision-missing-evidence"
	missingEvidence.EvidenceChecks = map[string]bool{"responsibility_separation": true}
	if err := incident.Decide(missingEvidence, time.Now().UTC()); err == nil {
		t.Fatal("证据不完整仍被允许裁定")
	}
}

func TestRecommendationThresholds(t *testing.T) {
	tests := []struct {
		name      string
		errorRate float64
		duration  int64
		want      string
	}{
		{"通过边界", 0.01, 60, RecommendationPass}, {"时长不足", 0.01, 59, RecommendationRetreat},
		{"补治边界", 0.05, 1, RecommendationRetreat}, {"不可恢复", 0.051, 120, RecommendationIrrecoverable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := CalculateRecommendation(test.errorRate, test.duration, "CAL-test")
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("建议 = %s，期望 %s", got, test.want)
			}
		})
	}
	if _, err := CalculateRecommendation(0.001, 100, ""); err == nil {
		t.Fatal("缺少校准标识未被拒绝")
	}
}

func addAssessment(t *testing.T, incident *PreservationIncident, assessment MediaAssessment, at time.Time) {
	t.Helper()
	if err := incident.AddAssessment(assessment, at); err != nil {
		t.Fatal(err)
	}
}

func completeEvidence() map[string]bool {
	return map[string]bool{"responsibility_separation": true, "treatments_complete": true, "calibration_evidence": true, "verification_complete": true, "audit_reviewed": true}
}

func readyForDecision(t *testing.T, recommendation string) *PreservationIncident {
	t.Helper()
	at := time.Now().UTC().Add(-time.Hour)
	incident, err := CreateIncident(NewIncident{IncidentID: "INC-ready-" + recommendation, BatchCode: "BATCH-ready", Symptoms: []string{"脱粉"}, Environment: EnvironmentSnapshot{TemperatureCelsius: 20, RelativeHumidity: 50, StorageLocation: "vault", CapturedAt: at}, CreatedAt: at})
	if err != nil {
		t.Fatal(err)
	}
	addAssessment(t, incident, MediaAssessment{AssessmentID: "assessment-target-" + recommendation, MediaID: "TAPE-ready", SampleRole: "target", SheddingGrade: 3, ObservedBy: "observer-01", ObservedAt: at}, at)
	addAssessment(t, incident, MediaAssessment{AssessmentID: "assessment-control-" + recommendation, MediaID: "TAPE-control", SampleRole: "control", ObservedBy: "observer-01", ObservedAt: at}, at)
	if err := incident.FreezeBoundary([]string{"TAPE-ready", "TAPE-control"}, at); err != nil {
		t.Fatal(err)
	}
	if err := incident.SubmitPlan(StabilizationPlan{PlanID: "plan-" + recommendation, EnvironmentTarget: "20°C", CleaningMethod: "清洁", BakingLimit: "48°C", StopConditions: []string{"停止"}, AuthorID: "author-01"}, at); err != nil {
		t.Fatal(err)
	}
	if err := incident.ApprovePlan("approver-02", at); err != nil {
		t.Fatal(err)
	}
	completed := at.Add(20 * time.Minute)
	if err := incident.AddTreatment(TreatmentRecord{RecordID: "record-" + recommendation, MediaID: "TAPE-ready", OperatorID: "operator-03", StartedAt: at, CompletedAt: &completed, ActualParameters: map[string]string{"temperature": "40°C"}, Outcome: "completed"}, completed); err != nil {
		t.Fatal(err)
	}
	errorRate, duration := 0.005, int64(120)
	if recommendation == RecommendationRetreat {
		errorRate, duration = 0.03, 120
	}
	if recommendation == RecommendationIrrecoverable {
		errorRate, duration = 0.2, 0
	}
	if err := incident.AddVerification(ReadabilityVerification{VerificationID: "verify-" + recommendation, MediaID: "TAPE-ready", DeviceID: "drive-01", CalibrationRef: "CAL-01", ErrorRate: errorRate, ReadableDurationSecs: duration, SampleDigest: "sha256:test", VerifiedBy: "verifier-04", VerifiedAt: completed}, completed); err != nil {
		t.Fatal(err)
	}
	return incident
}
