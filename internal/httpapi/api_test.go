package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"tape-preservation-incident-api/internal/app"
	"tape-preservation-incident-api/internal/preservation"
	"tape-preservation-incident-api/internal/store"
)

func TestCreateQueryAndProtocolErrors(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(app.NewService(repository, app.UTCClock{})).Handler())
	defer server.Close()
	command := app.CreateIncidentCommand{RequestID: "http-create-test", ActorID: "engineer-01", BatchCode: "BATCH-http", Symptoms: []string{"黏连"}, EnvironmentSnapshot: preservation.EnvironmentSnapshot{TemperatureCelsius: 21, RelativeHumidity: 55, StorageLocation: "vault", CapturedAt: time.Now().UTC().Add(-time.Hour)}}
	data, _ := json.Marshal(command)
	request, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/incidents", bytes.NewReader(data))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Correlation-ID", "test-correlation")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("创建状态码 = %d", response.StatusCode)
	}
	var result app.CommandResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	query, err := http.Get(server.URL + "/api/v1/incidents/" + result.Incident.IncidentID)
	if err != nil {
		t.Fatal(err)
	}
	query.Body.Close()
	if query.StatusCode != http.StatusOK {
		t.Fatalf("查询状态码 = %d", query.StatusCode)
	}

	bad, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v1/incidents", bytes.NewReader([]byte(`{"unknown":true}`)))
	bad.Header.Set("Content-Type", "application/json")
	bad.Header.Set("X-Correlation-ID", "bad-correlation")
	badResponse, err := http.DefaultClient.Do(bad)
	if err != nil {
		t.Fatal(err)
	}
	defer badResponse.Body.Close()
	if badResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("未知字段状态码 = %d", badResponse.StatusCode)
	}
	var protocol protocolError
	if err := json.NewDecoder(badResponse.Body).Decode(&protocol); err != nil {
		t.Fatal(err)
	}
	if protocol.Error.CorrelationID != "bad-correlation" || protocol.Error.Code != "validation_failed" {
		t.Fatalf("协议错误异常: %+v", protocol)
	}
}
