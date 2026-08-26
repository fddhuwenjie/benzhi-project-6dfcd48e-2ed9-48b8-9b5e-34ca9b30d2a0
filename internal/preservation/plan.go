package preservation

import "time"

func (p *StabilizationPlan) ValidateDraft() error {
	if err := ValidateIdentifier("plan_id", p.PlanID); err != nil {
		return err
	}
	if err := ValidateActor(p.AuthorID); err != nil {
		return err
	}
	if err := requireText("environment_target", p.EnvironmentTarget, 500); err != nil {
		return err
	}
	if err := requireText("cleaning_method", p.CleaningMethod, 500); err != nil {
		return err
	}
	if err := requireText("baking_limit", p.BakingLimit, 300); err != nil {
		return err
	}
	if len(p.StopConditions) == 0 {
		return Invalid("stop_conditions", "至少设置一个停止条件")
	}
	for _, condition := range p.StopConditions {
		if err := requireText("stop_conditions", condition, 300); err != nil {
			return err
		}
	}
	return nil
}

func (i *PreservationIncident) SubmitPlan(plan StabilizationPlan, now time.Time) error {
	if err := EnsureMutable(i); err != nil {
		return err
	}
	if i.State != StatePlanning && i.State != StateRemediation {
		return WrongState(i.State, StatePlanning, StateRemediation)
	}
	plan.IncidentID = i.IncidentID
	plan.Status = "pending_approval"
	plan.ApproverID = ""
	plan.ApprovedAt = nil
	if err := plan.ValidateDraft(); err != nil {
		return err
	}
	if i.planIDUsed(plan.PlanID) {
		return Invalid("plan_id", "已经在当前或历史轮次中存在")
	}
	remediation := i.State == StateRemediation
	if remediation {
		if i.Decision == nil || i.Decision.Decision != DecisionRetreat {
			return Invalid("retreat_decision_id", "当前事件缺少上一轮补治裁定")
		}
		if plan.RetreatDecisionID == "" || plan.RetreatDecisionID != i.Decision.DecisionID {
			return Invalid("retreat_decision_id", "必须关联上一轮补治裁定")
		}
		if i.Plan == nil || i.CurrentRound < 1 || i.CurrentRoundEndRevision < i.CurrentRoundStartRevision {
			return Invalid("round", "上一轮证据不完整")
		}
		round := TreatmentRoundEvidence{
			RoundNumber: i.CurrentRound, StartRevision: i.CurrentRoundStartRevision, EndRevision: i.CurrentRoundEndRevision,
			Plan: *i.Plan, Treatments: append([]TreatmentRecord(nil), i.Treatments...),
			Verifications: append([]ReadabilityVerification(nil), i.Verifications...), Decision: *i.Decision,
		}
		sortRoundEvidence(&round)
		i.RoundHistory = append(i.RoundHistory, round)
		i.CurrentRound++
		i.CurrentRoundStartRevision = i.Revision + 1
		i.CurrentRoundEndRevision = 0
		i.Treatments = []TreatmentRecord{}
		i.Verifications = []ReadabilityVerification{}
		i.Decision = nil
	} else {
		if plan.RetreatDecisionID != "" {
			return Invalid("retreat_decision_id", "首轮方案不能关联补治裁定")
		}
		if i.CurrentRound == 0 {
			i.CurrentRound = 1
			i.CurrentRoundStartRevision = i.Revision + 1
		}
	}
	i.Plan = &plan
	i.State = StatePlanApproval
	i.Touch(now)
	return nil
}

func (i *PreservationIncident) planIDUsed(planID string) bool {
	if i.Plan != nil && i.Plan.PlanID == planID {
		return true
	}
	for _, round := range i.RoundHistory {
		if round.Plan.PlanID == planID {
			return true
		}
	}
	return false
}

func (i *PreservationIncident) ApprovePlan(approver string, approvedAt time.Time) error {
	if err := EnsureMutable(i); err != nil {
		return err
	}
	if i.State != StatePlanApproval {
		return WrongState(i.State, StatePlanApproval)
	}
	if i.Plan == nil {
		return Invalid("plan", "稳定化方案不存在")
	}
	if err := ValidateActor(approver); err != nil {
		return err
	}
	if approver == i.Plan.AuthorID {
		return Invalid("approver_id", "审批人与方案编制人必须不同")
	}
	if err := ValidateTimestamp("approved_at", approvedAt); err != nil {
		return err
	}
	i.Plan.ApproverID = approver
	t := approvedAt.UTC()
	i.Plan.ApprovedAt = &t
	i.Plan.Status = "approved"
	i.State = StateReadyTreatment
	i.Touch(approvedAt)
	return nil
}
