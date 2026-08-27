package recoveredidempotencypanic_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tape-preservation-incident-api/internal/preservation"
	"tape-preservation-incident-api/internal/store"
)

func TestRecoveredSnapshotWithEmptyIdempotencyIndexDoesNotPanic(t *testing.T) {
	root := t.TempDir()
	repository, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Add(-time.Hour)
	incident, err := preservation.CreateIncident(preservation.NewIncident{
		IncidentID: "INC-recovered-index",
		BatchCode:  "BATCH-recovered-index",
		Symptoms:   []string{"异味"},
		Environment: preservation.EnvironmentSnapshot{
			TemperatureCelsius: 20,
			RelativeHumidity:   50,
			StorageLocation:    "vault",
			CapturedAt:         at,
		},
		CreatedAt: at,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := repository.Create(context.Background(), incident, "create-recovered", "incident.created", "actor-01", at, map[string]any{"incident": incident}); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(root, incident.IncidentID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatal(err)
	}
	envelope["requests"] = json.RawMessage(`{}`)
	corrupted, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, corrupted, 0o640); err != nil {
		t.Fatal(err)
	}

	recoveredRepository, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _ = recoveredRepository.Update(context.Background(), incident.IncidentID, "update-after-recovery", "incident.touch", "actor-02", 1, at.Add(time.Minute), func(current *preservation.PreservationIncident) (any, error) {
		return map[string]string{"status": "unchanged"}, nil
	})
}
