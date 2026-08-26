package preservation

import (
	"time"
)

const (
	DecisionRelease = "release"
	DecisionRetreat = "retreat"
	DecisionDiscard = "discard"
)

var requiredEvidenceChecks = []string{"responsibility_separation", "treatments_complete", "calibration_evidence", "verification_complete", "audit_reviewed"}

func (i *PreservationIncident) Decide(d DispositionDecision, now time.Time) error {
	if err := EnsureMutable(i); err != nil {
		return err
	}
	if i.State != StateDecision {
		return WrongState(i.State, StateDecision)
	}
	d.IncidentID = i.IncidentID
	if err := ValidateIdentifier("decision_id", d.DecisionID); err != nil {
		return err
	}
	if err := ValidateActor(d.ReviewerID); err != nil {
		return err
	}
	if err := requireText("rationale", d.Rationale, 2000); err != nil {
		return err
	}
	if err := ValidateTimestamp("signed_at", d.SignedAt); err != nil {
		return err
	}
	if _, participated := i.TreatmentParticipants()[d.ReviewerID]; participated {
		return Invalid("reviewer_id", "独立复核员不得参与当前或历史轮次的任何处理事件")
	}
	if i.decisionIDUsed(d.DecisionID) {
		return Invalid("decision_id", "已经在当前或历史轮次中存在")
	}
	for _, key := range requiredEvidenceChecks {
		if !d.EvidenceChecks[key] {
			return Invalid("evidence_checks."+key, "必须核验并确认为 true")
		}
	}
	if len(i.Verifications) != len(i.AffectedMediaIDs) {
		return Invalid("verifications", "复验数据不完整")
	}
	recommendation := i.BatchRecommendation()
	switch d.Decision {
	case DecisionRelease:
		if recommendation != RecommendationPass {
			return Invalid("decision", "存在未通过建议时不能放行")
		}
		i.State = StateAwaitingSeal
	case DecisionRetreat:
		if recommendation == RecommendationPass {
			return Invalid("decision", "全部通过时不能无依据退回补治")
		}
		i.State = StateRemediation
	case DecisionDiscard:
		if recommendation != RecommendationIrrecoverable {
			return Invalid("decision", "仅不可恢复建议允许报废")
		}
		i.State = StateAwaitingSeal
	default:
		return Invalid("decision", "必须为 release、retreat 或 discard")
	}
	d.ArchiveDigest = ""
	i.Decision = &d
	i.CurrentRoundEndRevision = i.Revision + 1
	i.Touch(now)
	return nil
}

func (i *PreservationIncident) decisionIDUsed(decisionID string) bool {
	if i.Decision != nil && i.Decision.DecisionID == decisionID {
		return true
	}
	for _, round := range i.RoundHistory {
		if round.Decision.DecisionID == decisionID {
			return true
		}
	}
	return false
}

func (i *PreservationIncident) MarkSealed(at time.Time) error {
	if err := EnsureMutable(i); err != nil {
		return err
	}
	if i.State != StateAwaitingSeal {
		return WrongState(i.State, StateAwaitingSeal)
	}
	if i.Decision == nil || (i.Decision.Decision != DecisionRelease && i.Decision.Decision != DecisionDiscard) {
		return Invalid("decision", "只有放行或报废裁定可封存")
	}
	i.State = StateSealed
	t := at.UTC()
	i.SealedAt = &t
	i.Touch(at)
	return nil
}
