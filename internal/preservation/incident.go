package preservation

import (
	"sort"
	"strings"
	"time"
)

type NewIncident struct {
	IncidentID  string
	BatchCode   string
	Symptoms    []string
	Environment EnvironmentSnapshot
	CreatedAt   time.Time
}

func CreateIncident(cmd NewIncident) (*PreservationIncident, error) {
	if err := ValidateIdentifier("incident_id", cmd.IncidentID); err != nil {
		return nil, err
	}
	if err := ValidateIdentifier("batch_code", cmd.BatchCode); err != nil {
		return nil, err
	}
	if len(cmd.Symptoms) == 0 {
		return nil, Invalid("symptoms", "至少登记一个发现症状")
	}
	for _, symptom := range cmd.Symptoms {
		if err := requireText("symptoms", symptom, 300); err != nil {
			return nil, err
		}
	}
	if cmd.Environment.TemperatureCelsius < -20 || cmd.Environment.TemperatureCelsius > 60 {
		return nil, Invalid("environment_snapshot.temperature_celsius", "超出合理范围")
	}
	if cmd.Environment.RelativeHumidity < 0 || cmd.Environment.RelativeHumidity > 100 {
		return nil, Invalid("environment_snapshot.relative_humidity", "必须在 0 到 100 之间")
	}
	if err := requireText("environment_snapshot.storage_location", cmd.Environment.StorageLocation, 200); err != nil {
		return nil, err
	}
	if err := ValidateTimestamp("environment_snapshot.captured_at", cmd.Environment.CapturedAt); err != nil {
		return nil, err
	}
	if cmd.CreatedAt.IsZero() {
		cmd.CreatedAt = time.Now().UTC()
	}
	symptoms := append([]string(nil), cmd.Symptoms...)
	sort.Strings(symptoms)
	return &PreservationIncident{
		IncidentID: cmd.IncidentID, BatchCode: cmd.BatchCode, Symptoms: symptoms,
		EnvironmentSnapshot: cmd.Environment, State: StateScoping, Revision: 0,
		CreatedAt: cmd.CreatedAt.UTC(), UpdatedAt: cmd.CreatedAt.UTC(),
		AffectedMediaIDs: []string{}, Assessments: []MediaAssessment{}, Treatments: []TreatmentRecord{}, Verifications: []ReadabilityVerification{}, RoundHistory: []TreatmentRoundEvidence{},
	}, nil
}

func (i *PreservationIncident) Touch(at time.Time) {
	i.UpdatedAt = at.UTC()
}

func (i *PreservationIncident) HasAffectedMedia(mediaID string) bool {
	index := sort.SearchStrings(i.AffectedMediaIDs, mediaID)
	return index < len(i.AffectedMediaIDs) && i.AffectedMediaIDs[index] == mediaID
}

func (i *PreservationIncident) Participants() map[string]struct{} {
	people := make(map[string]struct{})
	if i.Plan != nil {
		people[i.Plan.AuthorID] = struct{}{}
		if i.Plan.ApproverID != "" {
			people[i.Plan.ApproverID] = struct{}{}
		}
	}
	for _, record := range i.Treatments {
		addTreatmentParticipants(people, record)
	}
	for _, round := range i.RoundHistory {
		people[round.Plan.AuthorID] = struct{}{}
		if round.Plan.ApproverID != "" {
			people[round.Plan.ApproverID] = struct{}{}
		}
		for _, record := range round.Treatments {
			addTreatmentParticipants(people, record)
		}
		for _, verification := range round.Verifications {
			people[verification.VerifiedBy] = struct{}{}
		}
	}
	for _, verification := range i.Verifications {
		people[verification.VerifiedBy] = struct{}{}
	}
	return people
}

func addTreatmentParticipants(people map[string]struct{}, record TreatmentRecord) {
	people[record.OperatorID] = struct{}{}
	if record.CompletedBy != "" {
		people[record.CompletedBy] = struct{}{}
	}
	for _, interruption := range record.Interruptions {
		people[interruption.ReportedBy] = struct{}{}
		if interruption.ResumedBy != "" {
			people[interruption.ResumedBy] = struct{}{}
		}
	}
}

func (i *PreservationIncident) TreatmentParticipants() map[string]struct{} {
	people := make(map[string]struct{})
	for _, record := range i.Treatments {
		addTreatmentParticipants(people, record)
	}
	for _, round := range i.RoundHistory {
		for _, record := range round.Treatments {
			addTreatmentParticipants(people, record)
		}
	}
	return people
}

func normalizeText(value string) string { return strings.TrimSpace(value) }
