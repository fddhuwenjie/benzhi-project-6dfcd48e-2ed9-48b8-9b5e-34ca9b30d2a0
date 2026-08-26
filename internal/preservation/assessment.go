package preservation

import (
	"sort"
	"time"
)

const MaxAssessmentBatchSize = 100

type BoundaryPreflightResult struct {
	AffectedMediaIDs   []string `json:"affected_media_ids"`
	MissingRoles       []string `json:"missing_roles"`
	UnobservedMediaIDs []string `json:"unobserved_media_ids"`
	CanFreeze          bool     `json:"can_freeze"`
}

func (a *MediaAssessment) Validate() error {
	if err := ValidateIdentifier("assessment_id", a.AssessmentID); err != nil {
		return err
	}
	if err := ValidateIdentifier("media_id", a.MediaID); err != nil {
		return err
	}
	if err := ValidateActor(a.ObservedBy); err != nil {
		return err
	}
	if a.SampleRole != "target" && a.SampleRole != "neighbor" && a.SampleRole != "control" {
		return Invalid("sample_role", "必须为 target、neighbor 或 control")
	}
	grades := map[string]int{"visual_grade": a.VisualGrade, "odor_grade": a.OdorGrade, "adhesion_grade": a.AdhesionGrade, "shedding_grade": a.SheddingGrade}
	for field, grade := range grades {
		if grade < 0 || grade > 3 {
			return Invalid(field, "必须在 0 到 3 之间")
		}
	}
	return ValidateTimestamp("observed_at", a.ObservedAt)
}

func ComputeAffected(a MediaAssessment) bool {
	severe := a.VisualGrade >= 2 || a.OdorGrade >= 2 || a.AdhesionGrade >= 2 || a.SheddingGrade >= 2
	cumulative := a.VisualGrade+a.OdorGrade+a.AdhesionGrade+a.SheddingGrade >= 4
	return severe || cumulative || a.DriveContamination
}

func (i *PreservationIncident) AddAssessment(a MediaAssessment, now time.Time) error {
	return i.AddAssessmentBatch([]MediaAssessment{a}, now)
}

func (i *PreservationIncident) AddAssessmentBatch(assessments []MediaAssessment, now time.Time) error {
	if err := EnsureMutable(i); err != nil {
		return err
	}
	if i.State != StateScoping {
		return WrongState(i.State, StateScoping)
	}
	if len(assessments) == 0 {
		return Invalid("assessments", "至少包含一条抽样观察")
	}
	if len(assessments) > MaxAssessmentBatchSize {
		return Invalid("assessments", "单批最多登记 100 条抽样观察")
	}
	assessmentIDs := make(map[string]struct{}, len(i.Assessments)+len(assessments))
	mediaIDs := make(map[string]struct{}, len(i.Assessments)+len(assessments))
	for _, existing := range i.Assessments {
		assessmentIDs[existing.AssessmentID] = struct{}{}
		mediaIDs[existing.MediaID] = struct{}{}
	}
	validated := make([]MediaAssessment, len(assessments))
	for index, assessment := range assessments {
		assessment.IncidentID = i.IncidentID
		if err := assessment.Validate(); err != nil {
			return Invalid("assessments["+itoa(index)+"]", err.Error())
		}
		if _, exists := assessmentIDs[assessment.AssessmentID]; exists {
			return Invalid("assessments["+itoa(index)+"].assessment_id", "请求内或事件内已经存在")
		}
		if _, exists := mediaIDs[assessment.MediaID]; exists {
			return Invalid("assessments["+itoa(index)+"].media_id", "请求内或事件内已经完成抽样观察")
		}
		assessmentIDs[assessment.AssessmentID] = struct{}{}
		mediaIDs[assessment.MediaID] = struct{}{}
		assessment.Affected = ComputeAffected(assessment)
		validated[index] = assessment
	}
	i.Assessments = append(i.Assessments, validated...)
	sort.Slice(i.Assessments, func(x, y int) bool { return i.Assessments[x].MediaID < i.Assessments[y].MediaID })
	i.Touch(now)
	return nil
}

func (i *PreservationIncident) BoundaryPreflight(expected []string) (BoundaryPreflightResult, error) {
	result := BoundaryPreflightResult{AffectedMediaIDs: []string{}, MissingRoles: []string{}, UnobservedMediaIDs: []string{}}
	if i.State != StateScoping {
		return result, WrongState(i.State, StateScoping)
	}
	if len(expected) == 0 {
		return result, Invalid("expected_media_ids", "至少包含一个待圈定介质")
	}
	if err := uniqueNonEmpty("expected_media_ids", expected); err != nil {
		return result, err
	}
	observed := make(map[string]MediaAssessment, len(i.Assessments))
	roles := make(map[string]bool)
	for _, assessment := range i.Assessments {
		observed[assessment.MediaID] = assessment
		roles[assessment.SampleRole] = true
	}
	for _, role := range []string{"control", "target"} {
		if !roles[role] {
			result.MissingRoles = append(result.MissingRoles, role)
		}
	}
	for _, mediaID := range expected {
		assessment, ok := observed[mediaID]
		if !ok {
			result.UnobservedMediaIDs = append(result.UnobservedMediaIDs, mediaID)
			continue
		}
		if assessment.Affected {
			result.AffectedMediaIDs = append(result.AffectedMediaIDs, mediaID)
		}
	}
	sort.Strings(result.AffectedMediaIDs)
	sort.Strings(result.UnobservedMediaIDs)
	result.CanFreeze = len(result.MissingRoles) == 0 && len(result.UnobservedMediaIDs) == 0 && len(result.AffectedMediaIDs) > 0
	return result, nil
}

func (i *PreservationIncident) FreezeBoundary(expected []string, now time.Time) error {
	if err := EnsureMutable(i); err != nil {
		return err
	}
	if i.State != StateScoping {
		return WrongState(i.State, StateScoping)
	}
	if len(expected) == 0 {
		return Invalid("expected_media_ids", "至少包含一个待圈定介质")
	}
	if err := uniqueNonEmpty("expected_media_ids", expected); err != nil {
		return err
	}
	preflight, err := i.BoundaryPreflight(expected)
	if err != nil {
		return err
	}
	if len(preflight.MissingRoles) > 0 {
		return Invalid("assessments", "完整抽样必须同时包含 target 与 control")
	}
	if len(preflight.UnobservedMediaIDs) > 0 {
		return Invalid("expected_media_ids", "存在尚未观察的介质 "+preflight.UnobservedMediaIDs[0])
	}
	if len(preflight.AffectedMediaIDs) == 0 {
		return Invalid("assessments", "抽样结果未圈定任何受影响介质")
	}
	i.AffectedMediaIDs = preflight.AffectedMediaIDs
	i.State = StatePlanning
	i.Touch(now)
	return nil
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := make([]byte, 0, 3)
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
