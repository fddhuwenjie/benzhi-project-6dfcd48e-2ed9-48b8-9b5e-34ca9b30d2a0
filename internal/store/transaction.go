package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"tape-preservation-incident-api/internal/preservation"
)

type Mutator func(*preservation.PreservationIncident) (any, error)

func (s *Store) Create(ctx context.Context, incident *preservation.PreservationIncident, requestID, operation, actor string, now time.Time, result any) (json.RawMessage, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	lock := s.incidentLock(incident.IncidentID)
	lock.Lock()
	defer lock.Unlock()
	existing, err := s.readUnlocked(incident.IncidentID)
	if err == nil {
		return replay(existing, requestID, operation)
	}
	var de *preservation.DomainError
	if !errors.As(err, &de) || de.Code != preservation.CodeNotFound {
		return nil, false, err
	}
	clone, err := preservation.Clone(incident)
	if err != nil {
		return nil, false, err
	}
	clone.Revision = 1
	clone.Touch(now)
	response, err := json.Marshal(result)
	if err != nil {
		return nil, false, fmt.Errorf("序列化命令结果: %w", err)
	}
	// The public result must expose the committed revision.
	response, err = replaceIncidentResult(response, clone)
	if err != nil {
		return nil, false, err
	}
	envelope := &incidentFile{SchemaVersion: schemaVersion, Incident: clone, Requests: make(map[string]IdempotencyRecord), Audit: []AuditEvent{}}
	if err := appendCommit(envelope, requestID, operation, actor, now, response); err != nil {
		return nil, false, err
	}
	if err := s.writeUnlocked(clone.IncidentID, envelope); err != nil {
		return nil, false, err
	}
	return append(json.RawMessage(nil), response...), false, nil
}

func (s *Store) Update(ctx context.Context, incidentID, requestID, operation, actor string, expectedRevision int64, now time.Time, mutate Mutator) (json.RawMessage, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	lock := s.incidentLock(incidentID)
	lock.Lock()
	defer lock.Unlock()
	envelope, err := s.readUnlocked(incidentID)
	if err != nil {
		return nil, false, err
	}
	if record, ok := envelope.Requests[requestID]; ok {
		if record.Operation != operation {
			return nil, false, preservation.Invalid("request_id", "已用于其他操作")
		}
		return append(json.RawMessage(nil), record.Response...), true, nil
	}
	if expectedRevision != envelope.Incident.Revision {
		return nil, false, &preservation.DomainError{Code: preservation.CodeConflict, Message: "expected_revision 与当前修订不一致", CurrentRevision: envelope.Incident.Revision}
	}
	clone, err := preservation.Clone(envelope.Incident)
	if err != nil {
		return nil, false, err
	}
	result, err := mutate(clone)
	if err != nil {
		return nil, false, err
	}
	clone.Revision++
	clone.Touch(now)
	if clone.State == preservation.StateSealed {
		digest, digestErr := archiveDigest(clone)
		if digestErr != nil {
			return nil, false, digestErr
		}
		clone.Decision.ArchiveDigest = digest
	}
	if err := preservation.ValidateAggregate(clone); err != nil {
		return nil, false, err
	}
	response, err := json.Marshal(result)
	if err != nil {
		return nil, false, fmt.Errorf("序列化命令结果: %w", err)
	}
	response, err = replaceIncidentResult(response, clone)
	if err != nil {
		return nil, false, err
	}
	envelope.Incident = clone
	if err := appendCommit(envelope, requestID, operation, actor, now, response); err != nil {
		return nil, false, err
	}
	if err := s.writeUnlocked(incidentID, envelope); err != nil {
		return nil, false, err
	}
	return append(json.RawMessage(nil), response...), false, nil
}

func replay(envelope *incidentFile, requestID, operation string) (json.RawMessage, bool, error) {
	record, ok := envelope.Requests[requestID]
	if !ok {
		return nil, false, &preservation.DomainError{Code: preservation.CodeConflict, Message: "确定性事件编号已存在但 request_id 不匹配", CurrentRevision: envelope.Incident.Revision}
	}
	if record.Operation != operation {
		return nil, false, preservation.Invalid("request_id", "已用于其他操作")
	}
	return append(json.RawMessage(nil), record.Response...), true, nil
}

func replaceIncidentResult(response []byte, incident *preservation.PreservationIncident) ([]byte, error) {
	var object map[string]any
	if err := json.Unmarshal(response, &object); err != nil {
		return nil, err
	}
	object["incident"] = incident
	return json.Marshal(object)
}

func appendCommit(envelope *incidentFile, requestID, operation, actor string, now time.Time, response json.RawMessage) error {
	if requestID == "" || operation == "" || actor == "" {
		return fmt.Errorf("审计提交元数据不完整")
	}
	snapshotDigest, err := jsonDigest(envelope.Incident)
	if err != nil {
		return err
	}
	previous := ""
	if len(envelope.Audit) > 0 {
		previous = envelope.Audit[len(envelope.Audit)-1].Digest
	}
	event := AuditEvent{Sequence: int64(len(envelope.Audit) + 1), Revision: envelope.Incident.Revision, Operation: operation, ActorID: actor, RequestID: requestID, OccurredAt: now.UTC(), PreviousDigest: previous, SnapshotDigest: snapshotDigest}
	event.Digest, err = eventDigest(event)
	if err != nil {
		return err
	}
	envelope.Audit = append(envelope.Audit, event)
	envelope.Requests = ensureRequestIndex(envelope)
	envelope.Requests[requestID] = IdempotencyRecord{RequestID: requestID, Operation: operation, Revision: envelope.Incident.Revision, Response: append(json.RawMessage(nil), response...), CommittedAt: now.UTC()}
	return nil
}

// ensureRequestIndex returns a non-nil idempotency map so that appending a new
// commit never panics on a nil map assignment. Normal snapshots reuse the
// existing index; a recovered envelope whose index is missing is initialized
// fresh, but such envelopes are rejected by verifyEnvelope before any commit.
func ensureRequestIndex(envelope *incidentFile) map[string]IdempotencyRecord {
	if envelope.Requests != nil {
		return envelope.Requests
	}
	return make(map[string]IdempotencyRecord)
}
