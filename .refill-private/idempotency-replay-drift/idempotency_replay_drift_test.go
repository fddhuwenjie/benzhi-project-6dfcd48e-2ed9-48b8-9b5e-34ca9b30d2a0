package idempotency_replay_drift_test

import (
	"context"
	"testing"
	"time"

	"tape-preservation-incident-api/internal/app"
	"tape-preservation-incident-api/internal/preservation"
	"tape-preservation-incident-api/internal/store"
)

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time {
	return c.now
}

func TestDelayedIdempotencyReplayReturnsOriginalRevision(t *testing.T) {
	at := time.Now().UTC().Add(-time.Minute)
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("打开存储: %v", err)
	}
	service := app.NewService(repository, fixedClock{now: at})
	created, err := service.CreateIncident(context.Background(), app.CreateIncidentCommand{
		RequestID: "create-replay-drift",
		ActorID:   "engineer-replay",
		BatchCode: "BATCH-replay-drift",
		Symptoms:  []string{"黏连"},
		EnvironmentSnapshot: preservation.EnvironmentSnapshot{
			TemperatureCelsius: 20,
			RelativeHumidity:   50,
			StorageLocation:    "vault-replay",
			CapturedAt:         at,
		},
	})
	if err != nil {
		t.Fatalf("创建事件: %v", err)
	}

	first := app.AddAssessmentCommand{
		CommandMeta: app.CommandMeta{RequestID: "assessment-original", ActorID: "observer-replay", ExpectedRevision: 1},
		Assessment: preservation.MediaAssessment{
			AssessmentID: "assessment-target-replay",
			MediaID:      "TAPE-target-replay",
			SampleRole:   "target",
			VisualGrade:  2,
			ObservedBy:   "observer-replay",
			ObservedAt:   at,
		},
	}
	original, err := service.AddAssessment(context.Background(), created.Incident.IncidentID, first)
	if err != nil {
		t.Fatalf("提交首个抽样: %v", err)
	}
	if original.Incident.Revision != 2 {
		t.Fatalf("首个抽样修订 = %d，期望 2", original.Incident.Revision)
	}

	_, err = service.AddAssessment(context.Background(), created.Incident.IncidentID, app.AddAssessmentCommand{
		CommandMeta: app.CommandMeta{RequestID: "assessment-later", ActorID: "observer-replay", ExpectedRevision: 2},
		Assessment: preservation.MediaAssessment{
			AssessmentID: "assessment-control-replay",
			MediaID:      "TAPE-control-replay",
			SampleRole:   "control",
			ObservedBy:   "observer-replay",
			ObservedAt:   at,
		},
	})
	if err != nil {
		t.Fatalf("提交后续抽样: %v", err)
	}

	replayed, err := service.AddAssessment(context.Background(), created.Incident.IncidentID, first)
	if err != nil {
		t.Fatalf("延迟重放首个抽样: %v", err)
	}
	if !replayed.Replayed {
		t.Fatal("延迟请求未按幂等记录重放")
	}
	if replayed.Incident.Revision != original.Incident.Revision || len(replayed.Incident.Assessments) != len(original.Incident.Assessments) {
		t.Fatalf("延迟重放结果漂移：修订 = %d，原始修订 = %d；抽样数 = %d，原始抽样数 = %d", replayed.Incident.Revision, original.Incident.Revision, len(replayed.Incident.Assessments), len(original.Incident.Assessments))
	}
}
