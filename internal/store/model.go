package store

import (
	"encoding/json"
	"time"

	"tape-preservation-incident-api/internal/preservation"
)

const schemaVersion = 1

type IdempotencyRecord struct {
	RequestID   string          `json:"request_id"`
	Operation   string          `json:"operation"`
	Revision    int64           `json:"revision"`
	Response    json.RawMessage `json:"response"`
	CommittedAt time.Time       `json:"committed_at"`
}

type AuditEvent struct {
	Sequence       int64     `json:"sequence"`
	Revision       int64     `json:"revision"`
	Operation      string    `json:"operation"`
	ActorID        string    `json:"actor_id"`
	RequestID      string    `json:"request_id"`
	OccurredAt     time.Time `json:"occurred_at"`
	PreviousDigest string    `json:"previous_digest"`
	SnapshotDigest string    `json:"snapshot_digest"`
	Digest         string    `json:"digest"`
}

type incidentFile struct {
	SchemaVersion int                                `json:"schema_version"`
	Incident      *preservation.PreservationIncident `json:"incident"`
	Requests      map[string]IdempotencyRecord       `json:"requests"`
	Audit         []AuditEvent                       `json:"audit"`
}

type TimelinePage struct {
	IncidentID string       `json:"incident_id"`
	Items      []AuditEvent `json:"items"`
	NextCursor int64        `json:"next_cursor,omitempty"`
}

type IntegrityResult struct {
	IncidentID      string `json:"incident_id"`
	Valid           bool   `json:"valid"`
	Revision        int64  `json:"revision"`
	AuditEvents     int    `json:"audit_events"`
	HeadDigest      string `json:"head_digest"`
	ArchiveDigest   string `json:"archive_digest,omitempty"`
	ArchiveVerified bool   `json:"archive_verified"`
}
