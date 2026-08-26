package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"tape-preservation-incident-api/internal/preservation"
)

func jsonDigest(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("序列化摘要载荷: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

type auditHashMaterial struct {
	Sequence       int64  `json:"sequence"`
	Revision       int64  `json:"revision"`
	Operation      string `json:"operation"`
	ActorID        string `json:"actor_id"`
	RequestID      string `json:"request_id"`
	OccurredAt     string `json:"occurred_at"`
	PreviousDigest string `json:"previous_digest"`
	SnapshotDigest string `json:"snapshot_digest"`
}

func eventDigest(event AuditEvent) (string, error) {
	material := auditHashMaterial{
		Sequence: event.Sequence, Revision: event.Revision, Operation: event.Operation,
		ActorID: event.ActorID, RequestID: event.RequestID,
		OccurredAt:     event.OccurredAt.UTC().Format("2006-01-02T15:04:05.000000000Z"),
		PreviousDigest: event.PreviousDigest, SnapshotDigest: event.SnapshotDigest,
	}
	return jsonDigest(material)
}

func archiveDigest(incident *preservation.PreservationIncident) (string, error) {
	clone, err := preservation.Clone(incident)
	if err != nil {
		return "", err
	}
	if clone.Decision == nil {
		return "", fmt.Errorf("封存快照缺少裁定")
	}
	clone.Decision.ArchiveDigest = ""
	return jsonDigest(clone)
}
