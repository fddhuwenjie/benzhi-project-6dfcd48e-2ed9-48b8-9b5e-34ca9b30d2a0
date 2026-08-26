package preservation

import (
	"fmt"
	"sort"
)

func ValidateAggregate(i *PreservationIncident) error {
	if i == nil {
		return &DomainError{Code: CodeIntegrity, Message: "聚合为空"}
	}
	if err := ValidateIdentifier("incident_id", i.IncidentID); err != nil {
		return err
	}
	if i.Revision < 1 {
		return &DomainError{Code: CodeIntegrity, Message: "聚合修订号无效"}
	}
	if i.State == StateSealed && (i.SealedAt == nil || i.Decision == nil || i.Decision.ArchiveDigest == "") {
		return &DomainError{Code: CodeIntegrity, Message: "封存聚合缺少时间、裁定或摘要"}
	}
	seen := make(map[string]struct{})
	assessmentIDs := make(map[string]struct{})
	assessmentMedia := make(map[string]struct{})
	previousAssessmentMedia := ""
	for _, assessment := range i.Assessments {
		if _, exists := assessmentIDs[assessment.AssessmentID]; exists {
			return &DomainError{Code: CodeIntegrity, Message: "抽样观察编号重复"}
		}
		if _, exists := assessmentMedia[assessment.MediaID]; exists {
			return &DomainError{Code: CodeIntegrity, Message: "抽样观察介质编号重复"}
		}
		if previousAssessmentMedia != "" && assessment.MediaID < previousAssessmentMedia {
			return &DomainError{Code: CodeIntegrity, Message: "抽样观察未按介质编号排序"}
		}
		assessmentIDs[assessment.AssessmentID], assessmentMedia[assessment.MediaID] = struct{}{}, struct{}{}
		previousAssessmentMedia = assessment.MediaID
	}
	previousMedia := ""
	for _, mediaID := range i.AffectedMediaIDs {
		if _, ok := seen[mediaID]; ok {
			return &DomainError{Code: CodeIntegrity, Message: "冻结清单含重复介质"}
		}
		seen[mediaID] = struct{}{}
		if previousMedia != "" && mediaID < previousMedia {
			return &DomainError{Code: CodeIntegrity, Message: "冻结清单未按介质编号排序"}
		}
		previousMedia = mediaID
	}
	if i.State == StateVerification || i.State == StateDecision || i.State == StateAwaitingSeal || i.State == StateSealed {
		if !i.allTreatmentsComplete() {
			return &DomainError{Code: CodeIntegrity, Message: "状态与处理完整性不一致"}
		}
	}
	if i.State == StateDecision || i.State == StateAwaitingSeal || i.State == StateSealed {
		if len(i.Verifications) != len(i.AffectedMediaIDs) {
			return &DomainError{Code: CodeIntegrity, Message: "状态与复验完整性不一致"}
		}
	}
	if i.State == StateAwaitingSeal && i.Decision == nil {
		return &DomainError{Code: CodeIntegrity, Message: "待封存状态缺少裁定"}
	}
	if i.UpdatedAt.Before(i.CreatedAt) {
		return fmt.Errorf("更新时间早于创建时间")
	}
	if err := validateRounds(i); err != nil {
		return err
	}
	return nil
}

func validateRounds(i *PreservationIncident) error {
	if i.CurrentRound == 0 {
		if len(i.RoundHistory) != 0 {
			return &DomainError{Code: CodeIntegrity, Message: "历史轮次存在但当前轮次未初始化"}
		}
		return nil
	}
	if i.CurrentRound != len(i.RoundHistory)+1 {
		return &DomainError{Code: CodeIntegrity, Message: "补治轮次编号不连续"}
	}
	ids := make(map[string]string)
	use := func(kind, id string) error {
		if id == "" {
			return &DomainError{Code: CodeIntegrity, Message: kind + " 标识缺失"}
		}
		if previous, exists := ids[id]; exists {
			return &DomainError{Code: CodeIntegrity, Message: kind + " 标识与 " + previous + " 重复"}
		}
		ids[id] = kind
		return nil
	}
	previousEnd := int64(0)
	for index, round := range i.RoundHistory {
		if round.RoundNumber != index+1 || round.StartRevision < 1 || round.EndRevision < round.StartRevision || (previousEnd > 0 && round.StartRevision <= previousEnd) {
			return &DomainError{Code: CodeIntegrity, Message: "历史轮次修订边界不连续"}
		}
		if round.Decision.Decision != DecisionRetreat || round.Plan.PlanID == "" || round.Decision.DecisionID == "" {
			return &DomainError{Code: CodeIntegrity, Message: "历史轮次缺少补治裁定证据"}
		}
		if err := validateHistoricalRound(round, i.AffectedMediaIDs); err != nil {
			return err
		}
		if round.RoundNumber > 1 && round.Plan.RetreatDecisionID != i.RoundHistory[index-1].Decision.DecisionID {
			return &DomainError{Code: CodeIntegrity, Message: "历史补治方案裁定关联无效"}
		}
		if err := use("plan_id", round.Plan.PlanID); err != nil {
			return err
		}
		if err := use("decision_id", round.Decision.DecisionID); err != nil {
			return err
		}
		for _, record := range round.Treatments {
			if err := validateTreatmentSequence(record); err != nil {
				return err
			}
			if err := use("record_id", record.RecordID); err != nil {
				return err
			}
		}
		for _, verification := range round.Verifications {
			if err := use("verification_id", verification.VerificationID); err != nil {
				return err
			}
		}
		previousEnd = round.EndRevision
	}
	if i.Plan != nil {
		if i.CurrentRoundStartRevision < 1 || (previousEnd > 0 && i.CurrentRoundStartRevision <= previousEnd) {
			return &DomainError{Code: CodeIntegrity, Message: "当前轮次起始修订无效"}
		}
		if err := use("plan_id", i.Plan.PlanID); err != nil {
			return err
		}
		if i.CurrentRound > 1 {
			previous := i.RoundHistory[len(i.RoundHistory)-1]
			if i.Plan.RetreatDecisionID != previous.Decision.DecisionID {
				return &DomainError{Code: CodeIntegrity, Message: "当前补治方案未关联上一轮裁定"}
			}
		}
	}
	for _, record := range i.Treatments {
		if err := validateTreatmentSequence(record); err != nil {
			return err
		}
		if err := use("record_id", record.RecordID); err != nil {
			return err
		}
	}
	for _, verification := range i.Verifications {
		if err := use("verification_id", verification.VerificationID); err != nil {
			return err
		}
	}
	if i.Decision != nil {
		if err := use("decision_id", i.Decision.DecisionID); err != nil {
			return err
		}
	}
	return nil
}

func validateHistoricalRound(round TreatmentRoundEvidence, affected []string) error {
	if round.Plan.Status != "approved" || round.Plan.ApprovedAt == nil {
		return &DomainError{Code: CodeIntegrity, Message: "历史轮次方案未审批"}
	}
	if len(round.Treatments) != len(affected) || len(round.Verifications) != len(affected) {
		return &DomainError{Code: CodeIntegrity, Message: "历史轮次处理或复验证据不完整"}
	}
	expected := append([]string(nil), affected...)
	sort.Strings(expected)
	for index := range expected {
		if round.Treatments[index].MediaID != expected[index] || round.Verifications[index].MediaID != expected[index] {
			return &DomainError{Code: CodeIntegrity, Message: "历史轮次介质证据未确定性排序或引用错误"}
		}
		if round.Treatments[index].Status != TreatmentCompleted {
			return &DomainError{Code: CodeIntegrity, Message: "历史轮次包含未完成处理"}
		}
	}
	return nil
}
