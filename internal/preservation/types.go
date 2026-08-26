package preservation

import "time"

type IncidentState string

const (
	StateScoping        IncidentState = "awaiting_scoping"
	StatePlanning       IncidentState = "awaiting_plan"
	StatePlanApproval   IncidentState = "awaiting_plan_approval"
	StateReadyTreatment IncidentState = "ready_for_treatment"
	StateTreating       IncidentState = "treating"
	StateVerification   IncidentState = "readability_verification"
	StateDecision       IncidentState = "awaiting_independent_decision"
	StateRemediation    IncidentState = "remediation_required"
	StateAwaitingSeal   IncidentState = "awaiting_seal"
	StateSealed         IncidentState = "sealed"
)

type EnvironmentSnapshot struct {
	TemperatureCelsius float64   `json:"temperature_celsius"`
	RelativeHumidity   float64   `json:"relative_humidity"`
	StorageLocation    string    `json:"storage_location"`
	CapturedAt         time.Time `json:"captured_at"`
}

type PreservationIncident struct {
	IncidentID                string                    `json:"incident_id"`
	BatchCode                 string                    `json:"batch_code"`
	Symptoms                  []string                  `json:"symptoms"`
	EnvironmentSnapshot       EnvironmentSnapshot       `json:"environment_snapshot"`
	State                     IncidentState             `json:"state"`
	AffectedMediaIDs          []string                  `json:"affected_media_ids"`
	Assessments               []MediaAssessment         `json:"assessments"`
	Plan                      *StabilizationPlan        `json:"plan,omitempty"`
	Treatments                []TreatmentRecord         `json:"treatments"`
	Verifications             []ReadabilityVerification `json:"verifications"`
	Decision                  *DispositionDecision      `json:"decision,omitempty"`
	CurrentRound              int                       `json:"current_round"`
	CurrentRoundStartRevision int64                     `json:"current_round_start_revision,omitempty"`
	CurrentRoundEndRevision   int64                     `json:"current_round_end_revision,omitempty"`
	RoundHistory              []TreatmentRoundEvidence  `json:"round_history"`
	Revision                  int64                     `json:"revision"`
	CreatedAt                 time.Time                 `json:"created_at"`
	UpdatedAt                 time.Time                 `json:"updated_at"`
	SealedAt                  *time.Time                `json:"sealed_at,omitempty"`
}

type MediaAssessment struct {
	AssessmentID       string    `json:"assessment_id"`
	IncidentID         string    `json:"incident_id"`
	MediaID            string    `json:"media_id"`
	SampleRole         string    `json:"sample_role"`
	VisualGrade        int       `json:"visual_grade"`
	OdorGrade          int       `json:"odor_grade"`
	AdhesionGrade      int       `json:"adhesion_grade"`
	SheddingGrade      int       `json:"shedding_grade"`
	DriveContamination bool      `json:"drive_contamination"`
	Affected           bool      `json:"affected"`
	ObservedBy         string    `json:"observed_by"`
	ObservedAt         time.Time `json:"observed_at"`
}

type StabilizationPlan struct {
	PlanID            string     `json:"plan_id"`
	IncidentID        string     `json:"incident_id"`
	EnvironmentTarget string     `json:"environment_target"`
	CleaningMethod    string     `json:"cleaning_method"`
	BakingLimit       string     `json:"baking_limit"`
	StopConditions    []string   `json:"stop_conditions"`
	AuthorID          string     `json:"author_id"`
	ApproverID        string     `json:"approver_id,omitempty"`
	ApprovedAt        *time.Time `json:"approved_at,omitempty"`
	Status            string     `json:"status"`
	RetreatDecisionID string     `json:"retreat_decision_id,omitempty"`
}

type Deviation struct {
	Description string `json:"description"`
	Explanation string `json:"explanation"`
}

type TreatmentRecord struct {
	RecordID         string                  `json:"record_id"`
	IncidentID       string                  `json:"incident_id"`
	MediaID          string                  `json:"media_id"`
	OperatorID       string                  `json:"operator_id"`
	StartedAt        time.Time               `json:"started_at"`
	CompletedAt      *time.Time              `json:"completed_at,omitempty"`
	ActualParameters map[string]string       `json:"actual_parameters"`
	Deviations       []Deviation             `json:"deviations"`
	StopEvent        string                  `json:"stop_event,omitempty"`
	Outcome          string                  `json:"outcome"`
	Status           string                  `json:"status"`
	Interruptions    []TreatmentInterruption `json:"interruptions"`
	CompletedBy      string                  `json:"completed_by,omitempty"`
}

const (
	TreatmentInProgress         = "in_progress"
	TreatmentPendingDisposition = "pending_disposition"
	TreatmentResumed            = "resumed"
	TreatmentCompleted          = "completed"
)

type TreatmentInterruption struct {
	InterruptionID       string            `json:"interruption_id"`
	OccurredAt           time.Time         `json:"occurred_at"`
	StopCondition        string            `json:"stop_condition"`
	OnsiteObservation    string            `json:"onsite_observation"`
	ImmediateAction      string            `json:"immediate_action"`
	ReportedBy           string            `json:"reported_by"`
	RiskDisposition      string            `json:"risk_disposition,omitempty"`
	ParameterAdjustments map[string]string `json:"parameter_adjustments,omitempty"`
	ResumedAt            *time.Time        `json:"resumed_at,omitempty"`
	ResumedBy            string            `json:"resumed_by,omitempty"`
}

type ReadabilityVerification struct {
	VerificationID       string    `json:"verification_id"`
	IncidentID           string    `json:"incident_id"`
	MediaID              string    `json:"media_id"`
	DeviceID             string    `json:"device_id"`
	CalibrationRef       string    `json:"calibration_ref"`
	ErrorRate            float64   `json:"error_rate"`
	ReadableDurationSecs int64     `json:"readable_duration_seconds"`
	SampleDigest         string    `json:"sample_digest"`
	Recommendation       string    `json:"recommendation"`
	VerifiedBy           string    `json:"verified_by"`
	VerifiedAt           time.Time `json:"verified_at"`
}

type DispositionDecision struct {
	DecisionID     string          `json:"decision_id"`
	IncidentID     string          `json:"incident_id"`
	ReviewerID     string          `json:"reviewer_id"`
	Decision       string          `json:"decision"`
	Rationale      string          `json:"rationale"`
	EvidenceChecks map[string]bool `json:"evidence_checks"`
	SignedAt       time.Time       `json:"signed_at"`
	ArchiveDigest  string          `json:"archive_digest,omitempty"`
}

type TreatmentRoundEvidence struct {
	RoundNumber   int                       `json:"round_number"`
	StartRevision int64                     `json:"start_revision"`
	EndRevision   int64                     `json:"end_revision"`
	Plan          StabilizationPlan         `json:"plan"`
	Treatments    []TreatmentRecord         `json:"treatments"`
	Verifications []ReadabilityVerification `json:"verifications"`
	Decision      DispositionDecision       `json:"decision"`
}

type TreatmentRoundSummary struct {
	RoundNumber       int    `json:"round_number"`
	StartRevision     int64  `json:"start_revision"`
	EndRevision       int64  `json:"end_revision"`
	PlanID            string `json:"plan_id"`
	DecisionID        string `json:"decision_id"`
	Decision          string `json:"decision"`
	TreatmentCount    int    `json:"treatment_count"`
	VerificationCount int    `json:"verification_count"`
}

type ArchiveManifest struct {
	IncidentID          string                   `json:"incident_id"`
	BatchCode           string                   `json:"batch_code"`
	State               IncidentState            `json:"state"`
	Revision            int64                    `json:"revision"`
	CreatedAt           time.Time                `json:"created_at"`
	Symptoms            []string                 `json:"symptoms"`
	EnvironmentSnapshot EnvironmentSnapshot      `json:"environment_snapshot"`
	AffectedMediaIDs    []string                 `json:"affected_media_ids"`
	Assessments         []MediaAssessment        `json:"assessments"`
	Rounds              []TreatmentRoundEvidence `json:"rounds"`
	FinalDecision       DispositionDecision      `json:"final_decision"`
	SealedAt            time.Time                `json:"sealed_at"`
	AuditHead           string                   `json:"audit_head"`
	ArchiveDigest       string                   `json:"archive_digest"`
}
