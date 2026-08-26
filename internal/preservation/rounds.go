package preservation

import "sort"

func sortRoundEvidence(round *TreatmentRoundEvidence) {
	sort.Slice(round.Treatments, func(a, b int) bool { return round.Treatments[a].MediaID < round.Treatments[b].MediaID })
	sort.Slice(round.Verifications, func(a, b int) bool { return round.Verifications[a].MediaID < round.Verifications[b].MediaID })
}

func (i *PreservationIncident) CurrentRoundEvidence() (TreatmentRoundEvidence, error) {
	if i.CurrentRound < 1 || i.Plan == nil {
		return TreatmentRoundEvidence{}, Invalid("round_number", "当前尚无处理轮次")
	}
	round := TreatmentRoundEvidence{
		RoundNumber: i.CurrentRound, StartRevision: i.CurrentRoundStartRevision, EndRevision: i.CurrentRoundEndRevision,
		Plan: *i.Plan, Treatments: append([]TreatmentRecord(nil), i.Treatments...),
		Verifications: append([]ReadabilityVerification(nil), i.Verifications...),
	}
	if i.Decision != nil {
		round.Decision = *i.Decision
	}
	sortRoundEvidence(&round)
	return round, nil
}

func (i *PreservationIncident) RoundDetail(number int) (TreatmentRoundEvidence, error) {
	if number < 1 {
		return TreatmentRoundEvidence{}, Invalid("round_number", "必须为正整数")
	}
	for _, round := range i.RoundHistory {
		if round.RoundNumber == number {
			copy := round
			copy.Treatments = append([]TreatmentRecord(nil), round.Treatments...)
			copy.Verifications = append([]ReadabilityVerification(nil), round.Verifications...)
			sortRoundEvidence(&copy)
			return copy, nil
		}
	}
	if number == i.CurrentRound {
		return i.CurrentRoundEvidence()
	}
	return TreatmentRoundEvidence{}, &DomainError{Code: CodeNotFound, Message: "补治轮次不存在"}
}

func (i *PreservationIncident) RoundSummaries() []TreatmentRoundSummary {
	summaries := make([]TreatmentRoundSummary, 0, len(i.RoundHistory)+1)
	appendSummary := func(round TreatmentRoundEvidence) {
		summaries = append(summaries, TreatmentRoundSummary{
			RoundNumber: round.RoundNumber, StartRevision: round.StartRevision, EndRevision: round.EndRevision,
			PlanID: round.Plan.PlanID, DecisionID: round.Decision.DecisionID, Decision: round.Decision.Decision,
			TreatmentCount: len(round.Treatments), VerificationCount: len(round.Verifications),
		})
	}
	for _, round := range i.RoundHistory {
		appendSummary(round)
	}
	if round, err := i.CurrentRoundEvidence(); err == nil {
		appendSummary(round)
	}
	sort.Slice(summaries, func(a, b int) bool { return summaries[a].RoundNumber < summaries[b].RoundNumber })
	return summaries
}

func BuildArchiveManifest(i *PreservationIncident, auditHead string) (ArchiveManifest, error) {
	if i.State != StateSealed || i.SealedAt == nil {
		return ArchiveManifest{}, WrongState(i.State, StateSealed)
	}
	if i.Decision == nil || i.Decision.ArchiveDigest == "" {
		return ArchiveManifest{}, &DomainError{Code: CodeIntegrity, Message: "封存档案缺少最终裁定或摘要"}
	}
	if normalizeText(auditHead) == "" {
		return ArchiveManifest{}, &DomainError{Code: CodeIntegrity, Message: "封存档案缺少审计链头"}
	}
	rounds := make([]TreatmentRoundEvidence, 0, len(i.RoundHistory)+1)
	for _, round := range i.RoundHistory {
		copy := round
		copy.Treatments = append([]TreatmentRecord(nil), round.Treatments...)
		copy.Verifications = append([]ReadabilityVerification(nil), round.Verifications...)
		sortRoundEvidence(&copy)
		rounds = append(rounds, copy)
	}
	current, err := i.CurrentRoundEvidence()
	if err != nil {
		return ArchiveManifest{}, &DomainError{Code: CodeIntegrity, Message: "封存档案缺少当前轮次"}
	}
	rounds = append(rounds, current)
	sort.Slice(rounds, func(a, b int) bool { return rounds[a].RoundNumber < rounds[b].RoundNumber })
	assessments := append([]MediaAssessment(nil), i.Assessments...)
	sort.Slice(assessments, func(a, b int) bool { return assessments[a].MediaID < assessments[b].MediaID })
	affected := append([]string(nil), i.AffectedMediaIDs...)
	sort.Strings(affected)
	return ArchiveManifest{
		IncidentID: i.IncidentID, BatchCode: i.BatchCode, State: i.State, Revision: i.Revision, CreatedAt: i.CreatedAt,
		Symptoms:            append([]string(nil), i.Symptoms...),
		EnvironmentSnapshot: i.EnvironmentSnapshot, AffectedMediaIDs: affected, Assessments: assessments,
		Rounds: rounds, FinalDecision: *i.Decision, SealedAt: i.SealedAt.UTC(), AuditHead: auditHead,
		ArchiveDigest: i.Decision.ArchiveDigest,
	}, nil
}
