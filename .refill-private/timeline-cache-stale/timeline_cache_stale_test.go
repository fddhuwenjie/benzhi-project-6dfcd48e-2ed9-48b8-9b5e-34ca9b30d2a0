package timelinecachestale

import (
	"context"
	"testing"
	"time"

	"tape-preservation-incident-api/internal/preservation"
	"tape-preservation-incident-api/internal/store"
)

func TestTimelineRefreshesAfterCommittedRevision(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2025, time.January, 2, 10, 0, 0, 0, time.UTC)
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	incident, err := preservation.CreateIncident(preservation.NewIncident{
		IncidentID: "INC-timeline-cache", BatchCode: "BATCH-cache", Symptoms: []string{"异味"},
		Environment: preservation.EnvironmentSnapshot{TemperatureCelsius: 20, RelativeHumidity: 50, StorageLocation: "vault", CapturedAt: now}, CreatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := repository.Create(ctx, incident, "req-create", "incident.created", "actor-1", now, map[string]any{"incident": incident}); err != nil {
		t.Fatal(err)
	}
	first, err := repository.Timeline(ctx, incident.IncidentID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 1 {
		t.Fatalf("初始时间线长度=%d", len(first.Items))
	}
	if _, _, err := repository.Update(ctx, incident.IncidentID, "req-assess", "assessment.recorded", "actor-1", 1, now.Add(time.Minute), func(current *preservation.PreservationIncident) (any, error) {
		err := current.AddAssessment(preservation.MediaAssessment{AssessmentID: "assessment-1", MediaID: "TAPE-1", SampleRole: "target", VisualGrade: 2, ObservedBy: "actor-1", ObservedAt: now}, now.Add(time.Minute))
		return map[string]any{"incident": current}, err
	}); err != nil {
		t.Fatal(err)
	}
	second, err := repository.Timeline(ctx, incident.IncidentID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 2 {
		t.Fatalf("提交新修订后时间线未刷新：长度=%d，期望 2", len(second.Items))
	}
}
