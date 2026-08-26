package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"

	"tape-preservation-incident-api/internal/preservation"
	"tape-preservation-incident-api/internal/store"
)

type Service struct {
	repository   Repository
	clock        Clock
	archiveMu    sync.RWMutex
	archiveCache map[string]preservation.ArchiveManifest
}

func NewService(repository Repository, clock Clock) *Service {
	if clock == nil {
		clock = UTCClock{}
	}
	return &Service{repository: repository, clock: clock, archiveCache: make(map[string]preservation.ArchiveManifest)}
}

func incidentID(requestID string) string {
	sum := sha256.Sum256([]byte(requestID))
	return "INC-" + hex.EncodeToString(sum[:10])
}

func validateRequest(requestID, actor string) error {
	if err := preservation.ValidateIdentifier("request_id", requestID); err != nil {
		return err
	}
	return preservation.ValidateActor(actor)
}

func validateMeta(meta CommandMeta) error {
	if err := validateRequest(meta.RequestID, meta.ActorID); err != nil {
		return err
	}
	if meta.ExpectedRevision < 1 {
		return preservation.Invalid("expected_revision", "必须为正整数")
	}
	return nil
}

func decodeResult(raw json.RawMessage, replayed bool) (CommandResult, error) {
	var result CommandResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return CommandResult{}, fmt.Errorf("解析持久化命令结果: %w", err)
	}
	result.Replayed = replayed
	return result, nil
}

func (s *Service) CreateIncident(ctx context.Context, cmd CreateIncidentCommand) (CommandResult, error) {
	if err := validateRequest(cmd.RequestID, cmd.ActorID); err != nil {
		return CommandResult{}, err
	}
	now := s.clock.Now()
	incident, err := preservation.CreateIncident(preservation.NewIncident{
		IncidentID: incidentID(cmd.RequestID), BatchCode: cmd.BatchCode, Symptoms: cmd.Symptoms,
		Environment: cmd.EnvironmentSnapshot, CreatedAt: now,
	})
	if err != nil {
		return CommandResult{}, err
	}
	raw, replayed, err := s.repository.Create(ctx, incident, cmd.RequestID, "incident.created", cmd.ActorID, now, CommandResult{Incident: incident})
	if err != nil {
		return CommandResult{}, err
	}
	return decodeResult(raw, replayed)
}

func (s *Service) GetIncident(ctx context.Context, incidentID string) (*preservation.PreservationIncident, error) {
	if err := preservation.ValidateIdentifier("incident_id", incidentID); err != nil {
		return nil, err
	}
	return s.repository.Load(ctx, incidentID)
}

func (s *Service) Timeline(ctx context.Context, incidentID string, cursor int64, limit int) (store.TimelinePage, error) {
	if err := preservation.ValidateIdentifier("incident_id", incidentID); err != nil {
		return store.TimelinePage{}, err
	}
	return s.repository.Timeline(ctx, incidentID, cursor, limit)
}

func (s *Service) VerifyArchive(ctx context.Context, incidentID string) (store.IntegrityResult, error) {
	if err := preservation.ValidateIdentifier("incident_id", incidentID); err != nil {
		return store.IntegrityResult{}, err
	}
	return s.repository.Verify(ctx, incidentID)
}

func (s *Service) GetRound(ctx context.Context, incidentID string, number int) (preservation.TreatmentRoundEvidence, error) {
	if err := preservation.ValidateIdentifier("incident_id", incidentID); err != nil {
		return preservation.TreatmentRoundEvidence{}, err
	}
	incident, err := s.repository.Load(ctx, incidentID)
	if err != nil {
		return preservation.TreatmentRoundEvidence{}, err
	}
	return incident.RoundDetail(number)
}

func (s *Service) ArchiveManifest(ctx context.Context, incidentID, expectedDigest string) (preservation.ArchiveManifest, error) {
	if err := preservation.ValidateIdentifier("incident_id", incidentID); err != nil {
		return preservation.ArchiveManifest{}, err
	}
	reader, ok := s.repository.(ArchiveManifestReader)
	if !ok {
		return preservation.ArchiveManifest{}, fmt.Errorf("存储未提供规范档案清单读取能力")
	}
	s.archiveMu.RLock()
	if cached, exists := s.archiveCache[incidentID]; exists {
		s.archiveMu.RUnlock()
		if expectedDigest != "" && expectedDigest != cached.ArchiveDigest {
			return preservation.ArchiveManifest{}, &preservation.DomainError{
				Code: preservation.CodeConflict, Message: "期望档案摘要与实际摘要不一致",
				ActualDigest: cached.ArchiveDigest,
			}
		}
		return cached, nil
	}
	s.archiveMu.RUnlock()
	manifest, err := reader.ArchiveManifest(ctx, incidentID, expectedDigest)
	if err != nil {
		return preservation.ArchiveManifest{}, err
	}
	s.archiveMu.Lock()
	if cached, exists := s.archiveCache[incidentID]; exists {
		manifest = cached
	} else {
		s.archiveCache[incidentID] = manifest
	}
	s.archiveMu.Unlock()
	return manifest, nil
}

func (s *Service) update(ctx context.Context, incidentID, operation string, meta CommandMeta, mutate store.Mutator) (CommandResult, error) {
	if err := preservation.ValidateIdentifier("incident_id", incidentID); err != nil {
		return CommandResult{}, err
	}
	if err := validateMeta(meta); err != nil {
		return CommandResult{}, err
	}
	now := s.clock.Now()
	raw, replayed, err := s.repository.Update(ctx, incidentID, meta.RequestID, operation, meta.ActorID, meta.ExpectedRevision, now, mutate)
	if err != nil {
		return CommandResult{}, err
	}
	return decodeResult(raw, replayed)
}
