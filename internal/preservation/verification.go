package preservation

import (
	"sort"
	"time"
)

const (
	RecommendationPass          = "pass"
	RecommendationRetreat       = "retreat"
	RecommendationIrrecoverable = "irrecoverable"
)

func CalculateRecommendation(errorRate float64, readableSeconds int64, calibrationRef string) (string, error) {
	if normalizeText(calibrationRef) == "" {
		return "", Invalid("calibration_ref", "必须提供校准标识")
	}
	if errorRate < 0 || errorRate > 1 {
		return "", Invalid("error_rate", "必须在 0 到 1 之间")
	}
	if readableSeconds < 0 {
		return "", Invalid("readable_duration_seconds", "不能为负数")
	}
	if errorRate <= 0.01 && readableSeconds >= 60 {
		return RecommendationPass, nil
	}
	if errorRate <= 0.05 && readableSeconds > 0 {
		return RecommendationRetreat, nil
	}
	return RecommendationIrrecoverable, nil
}

func (v *ReadabilityVerification) Validate() error {
	if err := ValidateIdentifier("verification_id", v.VerificationID); err != nil {
		return err
	}
	if err := ValidateIdentifier("media_id", v.MediaID); err != nil {
		return err
	}
	if err := ValidateIdentifier("device_id", v.DeviceID); err != nil {
		return err
	}
	if err := ValidateActor(v.VerifiedBy); err != nil {
		return err
	}
	if err := requireText("sample_digest", v.SampleDigest, 256); err != nil {
		return err
	}
	if err := ValidateTimestamp("verified_at", v.VerifiedAt); err != nil {
		return err
	}
	recommendation, err := CalculateRecommendation(v.ErrorRate, v.ReadableDurationSecs, v.CalibrationRef)
	if err != nil {
		return err
	}
	v.Recommendation = recommendation
	return nil
}

func (i *PreservationIncident) AddVerification(v ReadabilityVerification, now time.Time) error {
	if err := EnsureMutable(i); err != nil {
		return err
	}
	if i.State != StateVerification {
		return WrongState(i.State, StateVerification)
	}
	if !i.allTreatmentsComplete() {
		return Invalid("treatments", "处理记录未完整闭环")
	}
	v.IncidentID = i.IncidentID
	if !i.HasAffectedMedia(v.MediaID) {
		return Invalid("media_id", "不在冻结清单中")
	}
	if err := v.Validate(); err != nil {
		return err
	}
	for _, existing := range i.Verifications {
		if existing.VerificationID == v.VerificationID {
			return Invalid("verification_id", "已经存在")
		}
		if existing.MediaID == v.MediaID {
			return Invalid("media_id", "该介质已经完成复验")
		}
	}
	for _, round := range i.RoundHistory {
		for _, existing := range round.Verifications {
			if existing.VerificationID == v.VerificationID {
				return Invalid("verification_id", "已经在历史轮次中存在")
			}
		}
	}
	i.Verifications = append(i.Verifications, v)
	sort.Slice(i.Verifications, func(x, y int) bool { return i.Verifications[x].MediaID < i.Verifications[y].MediaID })
	if len(i.Verifications) == len(i.AffectedMediaIDs) {
		i.State = StateDecision
	}
	i.Touch(now)
	return nil
}

func (i *PreservationIncident) BatchRecommendation() string {
	hasRetreat := false
	for _, verification := range i.Verifications {
		if verification.Recommendation == RecommendationIrrecoverable {
			return RecommendationIrrecoverable
		}
		if verification.Recommendation == RecommendationRetreat {
			hasRetreat = true
		}
	}
	if hasRetreat {
		return RecommendationRetreat
	}
	return RecommendationPass
}
