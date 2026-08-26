package preservation

import (
	"sort"
	"time"
)

func validateParameters(field string, parameters map[string]string) error {
	if len(parameters) == 0 {
		return Invalid(field, "必须记录实际处理参数")
	}
	for key, value := range parameters {
		if err := requireText(field+".key", key, 100); err != nil {
			return err
		}
		if err := requireText(field+".value", value, 300); err != nil {
			return err
		}
	}
	return nil
}

func validateDeviations(deviations []Deviation) error {
	for _, deviation := range deviations {
		if err := requireText("deviations.description", deviation.Description, 500); err != nil {
			return err
		}
		if err := requireText("deviations.explanation", deviation.Explanation, 1000); err != nil {
			return Invalid("deviations.explanation", "每项偏差必须闭环解释")
		}
	}
	return nil
}

func (r *TreatmentRecord) validateStart() error {
	if err := ValidateIdentifier("record_id", r.RecordID); err != nil {
		return err
	}
	if err := ValidateIdentifier("media_id", r.MediaID); err != nil {
		return err
	}
	if err := ValidateActor(r.OperatorID); err != nil {
		return err
	}
	if err := ValidateTimestamp("started_at", r.StartedAt); err != nil {
		return err
	}
	return validateParameters("actual_parameters", r.ActualParameters)
}

// Validate保留对原有“一次登记完成结果”入口的兼容校验。
func (r *TreatmentRecord) Validate() error {
	if err := r.validateStart(); err != nil {
		return err
	}
	if r.CompletedAt == nil {
		return Invalid("completed_at", "处理记录必须明确完成时间")
	}
	if err := ValidateTimestamp("completed_at", *r.CompletedAt); err != nil {
		return err
	}
	if r.CompletedAt.Before(r.StartedAt) {
		return Invalid("completed_at", "不能早于开始时间")
	}
	if err := validateDeviations(r.Deviations); err != nil {
		return err
	}
	if r.StopEvent != "" {
		return Invalid("stop_event", "存在停止事件的处理不能标记为完成")
	}
	if r.Outcome != TreatmentCompleted {
		return Invalid("outcome", "只有 completed 结果可以进入复验")
	}
	return nil
}

func (i *PreservationIncident) AddTreatment(record TreatmentRecord, now time.Time) error {
	if err := EnsureMutable(i); err != nil {
		return err
	}
	if i.State != StateReadyTreatment && i.State != StateTreating {
		return WrongState(i.State, StateReadyTreatment, StateTreating)
	}
	if i.Plan == nil || i.Plan.Status != "approved" {
		return Invalid("plan", "方案尚未审批")
	}
	record.IncidentID = i.IncidentID
	if !i.HasAffectedMedia(record.MediaID) {
		return Invalid("media_id", "不在冻结的受影响介质清单中")
	}
	if err := record.validateStart(); err != nil {
		return err
	}
	if i.treatmentRecordIDUsed(record.RecordID) {
		return Invalid("record_id", "已经在当前或历史轮次中存在")
	}
	for _, existing := range i.Treatments {
		if existing.MediaID == record.MediaID {
			return Invalid("media_id", "该介质已经登记处理记录")
		}
	}
	if record.CompletedAt != nil || record.Outcome != "" {
		if len(record.Interruptions) > 0 {
			return Invalid("interruptions", "一次性完成入口不能携带中止事件，请使用中止与恢复端点")
		}
		if record.Status != "" && record.Status != TreatmentCompleted {
			return Invalid("status", "完成记录状态必须为 completed")
		}
		record.Status = TreatmentCompleted
		if err := record.Validate(); err != nil {
			return err
		}
		record.CompletedBy = record.OperatorID
		record.Interruptions = []TreatmentInterruption{}
	} else {
		record.Status = TreatmentInProgress
		record.CompletedAt = nil
		record.CompletedBy = ""
		record.Deviations = []Deviation{}
		record.Interruptions = []TreatmentInterruption{}
	}
	i.Treatments = append(i.Treatments, record)
	sort.Slice(i.Treatments, func(x, y int) bool { return i.Treatments[x].MediaID < i.Treatments[y].MediaID })
	i.State = StateTreating
	if i.allTreatmentsComplete() {
		i.State = StateVerification
	}
	i.Touch(now)
	return nil
}

func (i *PreservationIncident) InterruptTreatment(recordID string, event TreatmentInterruption, now time.Time) error {
	if err := EnsureMutable(i); err != nil {
		return err
	}
	if i.State != StateTreating {
		return WrongState(i.State, StateTreating)
	}
	record, err := i.treatmentByID(recordID)
	if err != nil {
		return err
	}
	if record.Status != TreatmentInProgress && record.Status != TreatmentResumed {
		return Invalid("record_id", "当前处理状态不允许中止")
	}
	if err := ValidateIdentifier("interruption_id", event.InterruptionID); err != nil {
		return err
	}
	if err := ValidateActor(event.ReportedBy); err != nil {
		return err
	}
	if err := ValidateTimestamp("occurred_at", event.OccurredAt); err != nil {
		return err
	}
	if event.OccurredAt.Before(record.StartedAt) {
		return Invalid("occurred_at", "不能早于处理开始时间")
	}
	if len(record.Interruptions) > 0 {
		last := record.Interruptions[len(record.Interruptions)-1]
		baseline := last.OccurredAt
		if last.ResumedAt != nil {
			baseline = *last.ResumedAt
		}
		if event.OccurredAt.Before(baseline) {
			return Invalid("occurred_at", "不能早于上一处理事件")
		}
	}
	if err := requireText("stop_condition", event.StopCondition, 300); err != nil {
		return err
	}
	matched := false
	for _, condition := range i.Plan.StopConditions {
		if condition == event.StopCondition {
			matched = true
			break
		}
	}
	if !matched {
		return Invalid("stop_condition", "必须引用已审批方案中的停止条件")
	}
	if err := requireText("onsite_observation", event.OnsiteObservation, 1000); err != nil {
		return err
	}
	if err := requireText("immediate_action", event.ImmediateAction, 1000); err != nil {
		return err
	}
	for _, treatment := range i.Treatments {
		for _, existing := range treatment.Interruptions {
			if existing.InterruptionID == event.InterruptionID {
				return Invalid("interruption_id", "已经存在")
			}
		}
	}
	event.ResumedAt, event.ResumedBy, event.RiskDisposition, event.ParameterAdjustments = nil, "", "", nil
	record.Interruptions = append(record.Interruptions, event)
	record.Status = TreatmentPendingDisposition
	i.Touch(now)
	return nil
}

func (i *PreservationIncident) ResumeTreatment(recordID, interruptionID, actor, riskDisposition string, adjustments map[string]string, resumedAt, now time.Time) error {
	if err := EnsureMutable(i); err != nil {
		return err
	}
	if i.State != StateTreating {
		return WrongState(i.State, StateTreating)
	}
	if err := ValidateActor(actor); err != nil {
		return err
	}
	if err := ValidateTimestamp("resumed_at", resumedAt); err != nil {
		return err
	}
	if err := requireText("risk_disposition", riskDisposition, 1500); err != nil {
		return err
	}
	if err := validateParameters("parameter_adjustments", adjustments); err != nil {
		return err
	}
	record, err := i.treatmentByID(recordID)
	if err != nil {
		return err
	}
	if record.Status != TreatmentPendingDisposition {
		return Invalid("record_id", "处理记录不处于待处置状态")
	}
	for index := range record.Interruptions {
		event := &record.Interruptions[index]
		if event.InterruptionID != interruptionID {
			continue
		}
		if event.ResumedAt != nil {
			return Invalid("interruption_id", "中止事件已经闭环")
		}
		if resumedAt.Before(event.OccurredAt) {
			return Invalid("resumed_at", "不能早于中止发生时间")
		}
		t := resumedAt.UTC()
		event.ResumedAt, event.ResumedBy, event.RiskDisposition = &t, actor, riskDisposition
		event.ParameterAdjustments = cloneStringMap(adjustments)
		record.Status = TreatmentResumed
		i.Touch(now)
		return nil
	}
	return Invalid("interruption_id", "未找到该处理记录的未闭环中止事件")
}

func (i *PreservationIncident) CompleteTreatment(recordID, actor string, completedAt time.Time, deviations []Deviation, outcome string, now time.Time) error {
	if err := EnsureMutable(i); err != nil {
		return err
	}
	if i.State != StateTreating {
		return WrongState(i.State, StateTreating)
	}
	if err := ValidateActor(actor); err != nil {
		return err
	}
	if err := ValidateTimestamp("completed_at", completedAt); err != nil {
		return err
	}
	record, err := i.treatmentByID(recordID)
	if err != nil {
		return err
	}
	if record.Status != TreatmentInProgress && record.Status != TreatmentResumed {
		return Invalid("record_id", "待处置或已完成的处理不能提交完成")
	}
	if completedAt.Before(record.StartedAt) {
		return Invalid("completed_at", "不能早于开始时间")
	}
	for _, event := range record.Interruptions {
		if event.ResumedAt == nil {
			return Invalid("interruptions", "存在未闭环中止事件")
		}
		if completedAt.Before(*event.ResumedAt) {
			return Invalid("completed_at", "不能早于恢复时间")
		}
	}
	if err := validateDeviations(deviations); err != nil {
		return err
	}
	if outcome != TreatmentCompleted {
		return Invalid("outcome", "只有 completed 结果可以进入复验")
	}
	t := completedAt.UTC()
	record.CompletedAt, record.CompletedBy = &t, actor
	record.Deviations = append([]Deviation(nil), deviations...)
	record.Outcome, record.Status = TreatmentCompleted, TreatmentCompleted
	i.State = StateTreating
	if i.allTreatmentsComplete() {
		i.State = StateVerification
	}
	i.Touch(now)
	return nil
}

func (i *PreservationIncident) treatmentByID(recordID string) (*TreatmentRecord, error) {
	if err := ValidateIdentifier("record_id", recordID); err != nil {
		return nil, err
	}
	for index := range i.Treatments {
		if i.Treatments[index].RecordID == recordID {
			return &i.Treatments[index], nil
		}
	}
	return nil, Invalid("record_id", "处理记录不存在")
}

func (i *PreservationIncident) treatmentRecordIDUsed(recordID string) bool {
	for _, record := range i.Treatments {
		if record.RecordID == recordID {
			return true
		}
	}
	for _, round := range i.RoundHistory {
		for _, record := range round.Treatments {
			if record.RecordID == recordID {
				return true
			}
		}
	}
	return false
}

func (i *PreservationIncident) allTreatmentsComplete() bool {
	if len(i.Treatments) != len(i.AffectedMediaIDs) {
		return false
	}
	seen := make(map[string]bool, len(i.Treatments))
	for _, record := range i.Treatments {
		if record.CompletedAt == nil || record.Outcome != TreatmentCompleted || record.Status != TreatmentCompleted {
			return false
		}
		for _, event := range record.Interruptions {
			if event.ResumedAt == nil {
				return false
			}
		}
		for _, deviation := range record.Deviations {
			if normalizeText(deviation.Explanation) == "" {
				return false
			}
		}
		seen[record.MediaID] = true
	}
	for _, mediaID := range i.AffectedMediaIDs {
		if !seen[mediaID] {
			return false
		}
	}
	return true
}

func validateTreatmentSequence(record TreatmentRecord) error {
	if record.Status != TreatmentInProgress && record.Status != TreatmentPendingDisposition && record.Status != TreatmentResumed && record.Status != TreatmentCompleted {
		return &DomainError{Code: CodeIntegrity, Message: "处理记录状态无效"}
	}
	if record.Status == TreatmentCompleted {
		if record.CompletedAt == nil || record.Outcome != TreatmentCompleted {
			return &DomainError{Code: CodeIntegrity, Message: "已完成处理缺少完成证据"}
		}
	} else if record.CompletedAt != nil || record.Outcome != "" {
		return &DomainError{Code: CodeIntegrity, Message: "未完成处理包含完成结果"}
	}
	pending := 0
	last := record.StartedAt
	for _, event := range record.Interruptions {
		if event.OccurredAt.Before(last) {
			return &DomainError{Code: CodeIntegrity, Message: "处理中止事件时间顺序无效"}
		}
		if event.ResumedAt == nil {
			pending++
			last = event.OccurredAt
		} else {
			if event.ResumedAt.Before(event.OccurredAt) || event.RiskDisposition == "" || len(event.ParameterAdjustments) == 0 {
				return &DomainError{Code: CodeIntegrity, Message: "处理恢复事件证据不完整"}
			}
			last = *event.ResumedAt
		}
	}
	if (record.Status == TreatmentPendingDisposition) != (pending == 1) || pending > 1 {
		return &DomainError{Code: CodeIntegrity, Message: "处理状态与中止事件序列不一致"}
	}
	if record.Status == TreatmentInProgress && len(record.Interruptions) != 0 {
		return &DomainError{Code: CodeIntegrity, Message: "进行中处理包含已发生中止事件"}
	}
	if record.Status == TreatmentResumed && (len(record.Interruptions) == 0 || pending != 0) {
		return &DomainError{Code: CodeIntegrity, Message: "已恢复状态缺少闭环中止事件"}
	}
	if record.Status == TreatmentCompleted && pending != 0 {
		return &DomainError{Code: CodeIntegrity, Message: "已完成处理仍有未闭环中止事件"}
	}
	if record.CompletedAt != nil && record.CompletedAt.Before(last) {
		return &DomainError{Code: CodeIntegrity, Message: "处理完成时间早于事件序列"}
	}
	return nil
}

func cloneStringMap(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
