package app

import (
	"context"

	"tape-preservation-incident-api/internal/preservation"
)

func (s *Service) AddTreatment(ctx context.Context, incidentID string, cmd AddTreatmentCommand) (CommandResult, error) {
	now := s.clock.Now()
	return s.update(ctx, incidentID, "treatment.recorded", cmd.CommandMeta, func(incident *preservation.PreservationIncident) (any, error) {
		cmd.Treatment.OperatorID = cmd.ActorID
		if err := incident.AddTreatment(cmd.Treatment, now); err != nil {
			return nil, err
		}
		return CommandResult{Incident: incident}, nil
	})
}

func (s *Service) InterruptTreatment(ctx context.Context, incidentID, recordID string, cmd InterruptTreatmentCommand) (CommandResult, error) {
	now := s.clock.Now()
	return s.update(ctx, incidentID, "treatment.interrupted", cmd.CommandMeta, func(incident *preservation.PreservationIncident) (any, error) {
		cmd.Interruption.ReportedBy = cmd.ActorID
		if err := incident.InterruptTreatment(recordID, cmd.Interruption, now); err != nil {
			return nil, err
		}
		return CommandResult{Incident: incident}, nil
	})
}

func (s *Service) ResumeTreatment(ctx context.Context, incidentID, recordID string, cmd ResumeTreatmentCommand) (CommandResult, error) {
	now := s.clock.Now()
	if cmd.ResumedAt.IsZero() {
		cmd.ResumedAt = now
	}
	return s.update(ctx, incidentID, "treatment.resumed", cmd.CommandMeta, func(incident *preservation.PreservationIncident) (any, error) {
		if err := incident.ResumeTreatment(recordID, cmd.InterruptionID, cmd.ActorID, cmd.RiskDisposition, cmd.ParameterAdjustments, cmd.ResumedAt, now); err != nil {
			return nil, err
		}
		return CommandResult{Incident: incident}, nil
	})
}

func (s *Service) CompleteTreatment(ctx context.Context, incidentID, recordID string, cmd CompleteTreatmentCommand) (CommandResult, error) {
	now := s.clock.Now()
	if cmd.CompletedAt.IsZero() {
		cmd.CompletedAt = now
	}
	return s.update(ctx, incidentID, "treatment.completed", cmd.CommandMeta, func(incident *preservation.PreservationIncident) (any, error) {
		if err := incident.CompleteTreatment(recordID, cmd.ActorID, cmd.CompletedAt, cmd.Deviations, cmd.Outcome, now); err != nil {
			return nil, err
		}
		return CommandResult{Incident: incident}, nil
	})
}

func (s *Service) AddVerification(ctx context.Context, incidentID string, cmd AddVerificationCommand) (CommandResult, error) {
	now := s.clock.Now()
	return s.update(ctx, incidentID, "verification.recorded", cmd.CommandMeta, func(incident *preservation.PreservationIncident) (any, error) {
		cmd.Verification.VerifiedBy = cmd.ActorID
		if err := incident.AddVerification(cmd.Verification, now); err != nil {
			return nil, err
		}
		return CommandResult{Incident: incident}, nil
	})
}
