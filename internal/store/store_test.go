package store

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"tape-preservation-incident-api/internal/preservation"
)

func TestAtomicCommitIdempotencyAndIntegrity(t *testing.T) {
	repository, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Add(-time.Hour)
	incident, err := preservation.CreateIncident(preservation.NewIncident{IncidentID: "INC-store-test", BatchCode: "BATCH-store", Symptoms: []string{"异味"}, Environment: preservation.EnvironmentSnapshot{TemperatureCelsius: 22, RelativeHumidity: 55, StorageLocation: "vault", CapturedAt: at}, CreatedAt: at})
	if err != nil {
		t.Fatal(err)
	}
	result := map[string]any{"incident": incident}
	raw, replayed, err := repository.Create(context.Background(), incident, "request-create", "incident.created", "actor-01", at, result)
	if err != nil || replayed {
		t.Fatalf("首次提交失败: replayed=%v err=%v", replayed, err)
	}
	if !json.Valid(raw) {
		t.Fatal("命令结果不是有效 JSON")
	}
	replayedRaw, replayed, err := repository.Create(context.Background(), incident, "request-create", "incident.created", "actor-01", at, result)
	var firstResult, secondResult any
	firstDecodeErr := json.Unmarshal(raw, &firstResult)
	secondDecodeErr := json.Unmarshal(replayedRaw, &secondResult)
	firstCanonical, _ := json.Marshal(firstResult)
	secondCanonical, _ := json.Marshal(secondResult)
	if err != nil || !replayed || firstDecodeErr != nil || secondDecodeErr != nil || string(firstCanonical) != string(secondCanonical) {
		t.Fatalf("幂等重放异常: replayed=%v err=%v", replayed, err)
	}
	raw, replayed, err = repository.Update(context.Background(), incident.IncidentID, "request-update", "assessment.recorded", "actor-01", 1, at, func(current *preservation.PreservationIncident) (any, error) {
		err := current.AddAssessment(preservation.MediaAssessment{AssessmentID: "assessment-store", MediaID: "TAPE-store", SampleRole: "target", VisualGrade: 2, ObservedBy: "actor-01", ObservedAt: at}, at)
		return map[string]any{"incident": current}, err
	})
	if err != nil || replayed {
		t.Fatalf("更新失败: replayed=%v err=%v", replayed, err)
	}
	integrity, err := repository.Verify(context.Background(), incident.IncidentID)
	if err != nil {
		t.Fatal(err)
	}
	if !integrity.Valid || integrity.Revision != 2 || integrity.AuditEvents != 2 {
		t.Fatalf("完整性结果异常: %+v", integrity)
	}
	page, err := repository.Timeline(context.Background(), incident.IncidentID, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].PreviousDigest == "" {
		t.Fatalf("分页审计链异常: %+v", page)
	}
}

func TestTamperedAuditChainIsRejected(t *testing.T) {
	repository, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Add(-time.Hour)
	incident, err := preservation.CreateIncident(preservation.NewIncident{IncidentID: "INC-tamper-test", BatchCode: "BATCH-tamper", Symptoms: []string{"黏连"}, Environment: preservation.EnvironmentSnapshot{TemperatureCelsius: 20, RelativeHumidity: 50, StorageLocation: "vault", CapturedAt: at}, CreatedAt: at})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := repository.Create(context.Background(), incident, "request-tamper", "incident.created", "actor-01", at, map[string]any{"incident": incident}); err != nil {
		t.Fatal(err)
	}
	path := repository.path(incident.IncidentID)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var envelope incidentFile
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatal(err)
	}
	envelope.Audit[0].ActorID = "forged-actor"
	data, err = json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Load(context.Background(), incident.IncidentID); err == nil {
		t.Fatal("被篡改审计链仍能加载")
	}
}
