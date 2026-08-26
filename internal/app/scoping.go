package app

import (
	"context"
	"sort"

	"tape-preservation-incident-api/internal/preservation"
)

func (s *Service) AddAssessmentBatch(ctx context.Context, incidentID string, cmd AddAssessmentBatchCommand) (CommandResult, error) {
	now := s.clock.Now()
	return s.update(ctx, incidentID, "assessment.batch_recorded", cmd.CommandMeta, func(incident *preservation.PreservationIncident) (any, error) {
		if err := incident.AddAssessmentBatch(cmd.Assessments, now); err != nil {
			return nil, err
		}
		ids := make(map[string]struct{}, len(cmd.Assessments))
		for _, assessment := range cmd.Assessments {
			ids[assessment.AssessmentID] = struct{}{}
		}
		registered := make([]preservation.MediaAssessment, 0, len(ids))
		for _, assessment := range incident.Assessments {
			if _, ok := ids[assessment.AssessmentID]; ok {
				registered = append(registered, assessment)
			}
		}
		sort.Slice(registered, func(a, b int) bool { return registered[a].MediaID < registered[b].MediaID })
		return CommandResult{Incident: incident, Assessments: registered}, nil
	})
}

func (s *Service) BoundaryPreflight(ctx context.Context, incidentID string, cmd BoundaryPreflightCommand) (BoundaryPreflightResponse, error) {
	if err := preservation.ValidateIdentifier("incident_id", incidentID); err != nil {
		return BoundaryPreflightResponse{}, err
	}
	if err := preservation.ValidateActor(cmd.ActorID); err != nil {
		return BoundaryPreflightResponse{}, err
	}
	if cmd.ExpectedRevision < 1 {
		return BoundaryPreflightResponse{}, preservation.Invalid("expected_revision", "必须为正整数")
	}
	incident, err := s.repository.Load(ctx, incidentID)
	if err != nil {
		return BoundaryPreflightResponse{}, err
	}
	if incident.Revision != cmd.ExpectedRevision {
		return BoundaryPreflightResponse{}, &preservation.DomainError{Code: preservation.CodeConflict, Message: "expected_revision 与当前修订不一致", CurrentRevision: incident.Revision}
	}
	result, err := incident.BoundaryPreflight(cmd.ExpectedMediaIDs)
	if err != nil {
		return BoundaryPreflightResponse{}, err
	}
	return BoundaryPreflightResponse{IncidentID: incidentID, Revision: incident.Revision, BoundaryPreflightResult: result}, nil
}

func (s *Service) AddAssessment(ctx context.Context, incidentID string, cmd AddAssessmentCommand) (CommandResult, error) {
	now := s.clock.Now()
	return s.update(ctx, incidentID, "assessment.recorded", cmd.CommandMeta, func(incident *preservation.PreservationIncident) (any, error) {
		if err := incident.AddAssessment(cmd.Assessment, now); err != nil {
			return nil, err
		}
		return CommandResult{Incident: incident}, nil
	})
}

func (s *Service) FreezeBoundary(ctx context.Context, incidentID string, cmd FreezeBoundaryCommand) (CommandResult, error) {
	now := s.clock.Now()
	return s.update(ctx, incidentID, "boundary.frozen", cmd.CommandMeta, func(incident *preservation.PreservationIncident) (any, error) {
		if err := incident.FreezeBoundary(cmd.ExpectedMediaIDs, now); err != nil {
			return nil, err
		}
		return CommandResult{Incident: incident}, nil
	})
}
