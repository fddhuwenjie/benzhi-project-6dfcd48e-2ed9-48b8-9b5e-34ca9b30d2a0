package app

import (
	"time"

	"tape-preservation-incident-api/internal/preservation"
)

type CommandMeta struct {
	RequestID        string `json:"request_id"`
	ActorID          string `json:"actor_id"`
	ExpectedRevision int64  `json:"expected_revision"`
}

type CreateIncidentCommand struct {
	RequestID           string                           `json:"request_id"`
	ActorID             string                           `json:"actor_id"`
	BatchCode           string                           `json:"batch_code"`
	Symptoms            []string                         `json:"symptoms"`
	EnvironmentSnapshot preservation.EnvironmentSnapshot `json:"environment_snapshot"`
}

type AddAssessmentCommand struct {
	CommandMeta
	Assessment preservation.MediaAssessment `json:"assessment"`
}

type AddAssessmentBatchCommand struct {
	CommandMeta
	Assessments []preservation.MediaAssessment `json:"assessments"`
}

type BoundaryPreflightCommand struct {
	ActorID          string   `json:"actor_id"`
	ExpectedRevision int64    `json:"expected_revision"`
	ExpectedMediaIDs []string `json:"expected_media_ids"`
}

type BoundaryPreflightResponse struct {
	IncidentID string `json:"incident_id"`
	Revision   int64  `json:"revision"`
	preservation.BoundaryPreflightResult
}

type FreezeBoundaryCommand struct {
	CommandMeta
	ExpectedMediaIDs []string `json:"expected_media_ids"`
}

type SubmitPlanCommand struct {
	CommandMeta
	Plan preservation.StabilizationPlan `json:"plan"`
}

type ApprovePlanCommand struct {
	CommandMeta
	ApprovedAt time.Time `json:"approved_at"`
}

type AddTreatmentCommand struct {
	CommandMeta
	Treatment preservation.TreatmentRecord `json:"treatment"`
}

type InterruptTreatmentCommand struct {
	CommandMeta
	Interruption preservation.TreatmentInterruption `json:"interruption"`
}

type ResumeTreatmentCommand struct {
	CommandMeta
	InterruptionID       string            `json:"interruption_id"`
	RiskDisposition      string            `json:"risk_disposition"`
	ParameterAdjustments map[string]string `json:"parameter_adjustments"`
	ResumedAt            time.Time         `json:"resumed_at"`
}

type CompleteTreatmentCommand struct {
	CommandMeta
	CompletedAt time.Time                `json:"completed_at"`
	Deviations  []preservation.Deviation `json:"deviations"`
	Outcome     string                   `json:"outcome"`
}

type AddVerificationCommand struct {
	CommandMeta
	Verification preservation.ReadabilityVerification `json:"verification"`
}

type DecideCommand struct {
	CommandMeta
	Decision preservation.DispositionDecision `json:"decision"`
}

type SealCommand struct {
	CommandMeta
	SealedAt time.Time `json:"sealed_at"`
}

type CommandResult struct {
	Incident    *preservation.PreservationIncident `json:"incident"`
	Assessments []preservation.MediaAssessment     `json:"assessments,omitempty"`
	Replayed    bool                               `json:"replayed"`
}
