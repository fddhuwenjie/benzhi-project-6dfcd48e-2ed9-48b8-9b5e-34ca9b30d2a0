package app

import (
	"context"

	"tape-preservation-incident-api/internal/preservation"
)

func (s *Service) SubmitPlan(ctx context.Context, incidentID string, cmd SubmitPlanCommand) (CommandResult, error) {
	now := s.clock.Now()
	return s.update(ctx, incidentID, "plan.submitted", cmd.CommandMeta, func(incident *preservation.PreservationIncident) (any, error) {
		cmd.Plan.AuthorID = cmd.ActorID
		if err := incident.SubmitPlan(cmd.Plan, now); err != nil {
			return nil, err
		}
		return CommandResult{Incident: incident}, nil
	})
}

func (s *Service) ApprovePlan(ctx context.Context, incidentID string, cmd ApprovePlanCommand) (CommandResult, error) {
	approvedAt := cmd.ApprovedAt
	if approvedAt.IsZero() {
		approvedAt = s.clock.Now()
	}
	return s.update(ctx, incidentID, "plan.approved", cmd.CommandMeta, func(incident *preservation.PreservationIncident) (any, error) {
		if err := incident.ApprovePlan(cmd.ActorID, approvedAt); err != nil {
			return nil, err
		}
		return CommandResult{Incident: incident}, nil
	})
}
