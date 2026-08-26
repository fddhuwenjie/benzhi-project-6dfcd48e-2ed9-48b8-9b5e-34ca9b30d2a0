package app

import (
	"context"
	"encoding/json"
	"time"

	"tape-preservation-incident-api/internal/preservation"
	"tape-preservation-incident-api/internal/store"
)

type Repository interface {
	Create(context.Context, *preservation.PreservationIncident, string, string, string, time.Time, any) (json.RawMessage, bool, error)
	Update(context.Context, string, string, string, string, int64, time.Time, store.Mutator) (json.RawMessage, bool, error)
	Load(context.Context, string) (*preservation.PreservationIncident, error)
	Timeline(context.Context, string, int64, int) (store.TimelinePage, error)
	Verify(context.Context, string) (store.IntegrityResult, error)
}

type ArchiveManifestReader interface {
	ArchiveManifest(context.Context, string, string) (preservation.ArchiveManifest, error)
}

type Clock interface{ Now() time.Time }

type UTCClock struct{}

func (UTCClock) Now() time.Time { return time.Now().UTC() }
