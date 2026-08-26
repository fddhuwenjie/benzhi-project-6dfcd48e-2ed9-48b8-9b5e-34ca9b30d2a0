package app

import (
	"context"

	"tape-preservation-incident-api/internal/preservation"
)

func (s *Service) Decide(ctx context.Context, incidentID string, cmd DecideCommand) (CommandResult, error) {
	now := s.clock.Now()
	return s.update(ctx, incidentID, "decision.signed", cmd.CommandMeta, func(incident *preservation.PreservationIncident) (any, error) {
		cmd.Decision.ReviewerID = cmd.ActorID
		if cmd.Decision.SignedAt.IsZero() {
			cmd.Decision.SignedAt = now
		}
		if err := incident.Decide(cmd.Decision, now); err != nil {
			return nil, err
		}
		return CommandResult{Incident: incident}, nil
	})
}

func (s *Service) Seal(ctx context.Context, incidentID string, cmd SealCommand) (CommandResult, error) {
	now := cmd.SealedAt
	if now.IsZero() {
		now = s.clock.Now()
	}
	return s.update(ctx, incidentID, "incident.sealed", cmd.CommandMeta, func(incident *preservation.PreservationIncident) (any, error) {
		if err := incident.MarkSealed(now); err != nil {
			return nil, err
		}
		return CommandResult{Incident: incident}, nil
	})
}
