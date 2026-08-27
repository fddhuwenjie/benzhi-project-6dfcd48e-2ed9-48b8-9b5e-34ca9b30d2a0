package archive_manifest_cache_alias

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"tape-preservation-incident-api/internal/app"
	"tape-preservation-incident-api/internal/preservation"
	"tape-preservation-incident-api/internal/store"
)

type fakeRepository struct {
	manifest preservation.ArchiveManifest
	calls    int
}

func (f *fakeRepository) Create(context.Context, *preservation.PreservationIncident, string, string, string, time.Time, any) (json.RawMessage, bool, error) {
	return nil, false, nil
}
func (f *fakeRepository) Update(context.Context, string, string, string, string, int64, time.Time, store.Mutator) (json.RawMessage, bool, error) {
	return nil, false, nil
}
func (f *fakeRepository) Load(context.Context, string) (*preservation.PreservationIncident, error) {
	return nil, nil
}
func (f *fakeRepository) Timeline(context.Context, string, int64, int) (store.TimelinePage, error) {
	return store.TimelinePage{}, nil
}
func (f *fakeRepository) Verify(context.Context, string) (store.IntegrityResult, error) {
	return store.IntegrityResult{}, nil
}
func (f *fakeRepository) ArchiveManifest(_ context.Context, _ string, expected string) (preservation.ArchiveManifest, error) {
	f.calls++
	if expected != "" && expected != f.manifest.ArchiveDigest {
		return preservation.ArchiveManifest{}, &preservation.DomainError{Code: preservation.CodeConflict, Message: "期望档案摘要与实际摘要不一致", ActualDigest: f.manifest.ArchiveDigest}
	}
	return f.manifest, nil
}

func TestArchiveManifestCacheClonesReturnedManifest(t *testing.T) {
	repo := &fakeRepository{manifest: preservation.ArchiveManifest{IncidentID: "INC-archive", ArchiveDigest: "digest-good", AffectedMediaIDs: []string{"TAPE-001"}}}
	service := app.NewService(repo, app.UTCClock{})
	manifest, err := service.ArchiveManifest(context.Background(), "INC-archive", "digest-good")
	if err != nil {
		t.Fatalf("首个清单读取失败: %v", err)
	}
	manifest.AffectedMediaIDs[0] = "CORRUPTED"
	replayed, err := service.ArchiveManifest(context.Background(), "INC-archive", "digest-good")
	if err != nil {
		t.Fatalf("缓存清单读取失败: %v", err)
	}
	if len(replayed.AffectedMediaIDs) != 1 || replayed.AffectedMediaIDs[0] != "TAPE-001" {
		t.Fatalf("调用方修改返回清单污染了缓存: %+v (存储调用 %d 次)", replayed.AffectedMediaIDs, repo.calls)
	}
}
