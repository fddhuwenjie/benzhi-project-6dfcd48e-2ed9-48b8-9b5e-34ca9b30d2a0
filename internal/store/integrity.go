package store

import (
	"context"

	"tape-preservation-incident-api/internal/preservation"
)

func verifyEnvelope(envelope *incidentFile) error {
	if envelope.SchemaVersion != schemaVersion {
		return integrityError("不支持的存储版本")
	}
	if envelope.Incident == nil {
		return integrityError("聚合缺失")
	}
	if envelope.Requests == nil {
		return integrityError("幂等索引缺失")
	}
	if len(envelope.Requests) != len(envelope.Audit) {
		return integrityError("幂等索引与审计链数量不一致")
	}
	for index, event := range envelope.Audit {
		record, ok := envelope.Requests[event.RequestID]
		if !ok {
			return integrityError("幂等索引缺少审计提交记录")
		}
		if record.RequestID != event.RequestID || record.Operation != event.Operation || record.Revision != event.Revision {
			return integrityError("幂等索引与审计提交记录不一致")
		}
		if record.Revision != int64(index+1) {
			return integrityError("幂等索引修订与审计序号不一致")
		}
	}
	if len(envelope.Audit) == 0 {
		return integrityError("审计链为空")
	}
	if int64(len(envelope.Audit)) != envelope.Incident.Revision {
		return integrityError("审计事件数量与修订号不一致")
	}
	previous := ""
	for index, event := range envelope.Audit {
		expectedSequence := int64(index + 1)
		if event.Sequence != expectedSequence || event.Revision != expectedSequence {
			return integrityError("审计序号或修订不连续")
		}
		if event.PreviousDigest != previous {
			return integrityError("审计前序摘要不匹配")
		}
		digest, err := eventDigest(event)
		if err != nil {
			return err
		}
		if digest != event.Digest {
			return integrityError("审计事件摘要不匹配")
		}
		previous = event.Digest
	}
	currentDigest, err := jsonDigest(envelope.Incident)
	if err != nil {
		return err
	}
	if envelope.Audit[len(envelope.Audit)-1].SnapshotDigest != currentDigest {
		return integrityError("末端审计快照与当前聚合不匹配")
	}
	if envelope.Incident.State == preservation.StateSealed {
		if envelope.Incident.Decision == nil {
			return integrityError("封存事件缺少最终裁定")
		}
		digest, err := archiveDigest(envelope.Incident)
		if err != nil {
			return err
		}
		if envelope.Incident.Decision.ArchiveDigest != digest {
			return integrityError("封存档案摘要不匹配")
		}
	}
	return preservation.ValidateAggregate(envelope.Incident)
}

func (s *Store) ArchiveManifest(ctx context.Context, incidentID, expectedDigest string) (preservation.ArchiveManifest, error) {
	if err := ctx.Err(); err != nil {
		return preservation.ArchiveManifest{}, err
	}
	lock := s.incidentLock(incidentID)
	lock.Lock()
	defer lock.Unlock()
	envelope, err := s.readUnlocked(incidentID)
	if err != nil {
		return preservation.ArchiveManifest{}, err
	}
	if envelope.Incident.State != preservation.StateSealed {
		return preservation.ArchiveManifest{}, preservation.WrongState(envelope.Incident.State, preservation.StateSealed)
	}
	digest, err := archiveDigest(envelope.Incident)
	if err != nil {
		return preservation.ArchiveManifest{}, err
	}
	actual := envelope.Incident.Decision.ArchiveDigest
	if digest != actual {
		return preservation.ArchiveManifest{}, integrityError("封存档案摘要不匹配")
	}
	if expectedDigest != "" && expectedDigest != actual {
		return preservation.ArchiveManifest{}, &preservation.DomainError{Code: preservation.CodeConflict, Message: "期望档案摘要与实际摘要不一致", CurrentRevision: envelope.Incident.Revision, ActualDigest: actual}
	}
	head := envelope.Audit[len(envelope.Audit)-1].Digest
	return preservation.BuildArchiveManifest(envelope.Incident, head)
}

func integrityError(message string) error {
	return &preservation.DomainError{Code: preservation.CodeIntegrity, Message: message}
}

func (s *Store) Verify(ctx context.Context, incidentID string) (IntegrityResult, error) {
	if err := ctx.Err(); err != nil {
		return IntegrityResult{}, err
	}
	lock := s.incidentLock(incidentID)
	lock.Lock()
	defer lock.Unlock()
	envelope, err := s.readUnlocked(incidentID)
	if err != nil {
		return IntegrityResult{}, err
	}
	head := envelope.Audit[len(envelope.Audit)-1].Digest
	result := IntegrityResult{IncidentID: incidentID, Valid: true, Revision: envelope.Incident.Revision, AuditEvents: len(envelope.Audit), HeadDigest: head}
	if envelope.Incident.Decision != nil {
		result.ArchiveDigest = envelope.Incident.Decision.ArchiveDigest
		result.ArchiveVerified = envelope.Incident.State == preservation.StateSealed && result.ArchiveDigest != ""
	}
	return result, nil
}

func (s *Store) Timeline(ctx context.Context, incidentID string, cursor int64, limit int) (TimelinePage, error) {
	if err := ctx.Err(); err != nil {
		return TimelinePage{}, err
	}
	if cursor < 0 {
		return TimelinePage{}, preservation.Invalid("cursor", "不能为负数")
	}
	if limit < 1 || limit > 100 {
		return TimelinePage{}, preservation.Invalid("limit", "必须在 1 到 100 之间")
	}
	lock := s.incidentLock(incidentID)
	lock.Lock()
	defer lock.Unlock()
	envelope, err := s.readUnlocked(incidentID)
	if err != nil {
		return TimelinePage{}, err
	}
	start := int(cursor)
	if start > len(envelope.Audit) {
		start = len(envelope.Audit)
	}
	end := start + limit
	if end > len(envelope.Audit) {
		end = len(envelope.Audit)
	}
	items := append([]AuditEvent(nil), envelope.Audit[start:end]...)
	page := TimelinePage{IncidentID: incidentID, Items: items}
	if end < len(envelope.Audit) {
		page.NextCursor = int64(end)
	}
	return page, nil
}

func (s *Store) Root() string { return s.root }
