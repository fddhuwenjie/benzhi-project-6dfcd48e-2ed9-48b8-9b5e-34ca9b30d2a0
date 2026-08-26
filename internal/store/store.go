package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"tape-preservation-incident-api/internal/preservation"
)

type Store struct {
	root        string
	guard       sync.Mutex
	locks       map[string]*sync.Mutex
	coordinator *rootCoordinator
}

type rootCoordinator struct {
	guard sync.Mutex
	locks map[string]chan struct{}
}

var processCoordinators = struct {
	sync.Mutex
	byRoot map[string]*rootCoordinator
}{byRoot: make(map[string]*rootCoordinator)}

func coordinatorFor(root string) *rootCoordinator {
	processCoordinators.Lock()
	defer processCoordinators.Unlock()
	coordinator, ok := processCoordinators.byRoot[root]
	if !ok {
		coordinator = &rootCoordinator{locks: make(map[string]chan struct{})}
		processCoordinators.byRoot[root] = coordinator
	}
	return coordinator
}

func (c *rootCoordinator) acquire(ctx context.Context, id string) (func(), error) {
	c.guard.Lock()
	semaphore, ok := c.locks[id]
	if !ok {
		semaphore = make(chan struct{}, 1)
		semaphore <- struct{}{}
		c.locks[id] = semaphore
	}
	c.guard.Unlock()
	select {
	case <-semaphore:
		return func() { semaphore <- struct{}{} }, nil
	default:
	}
	select {
	case <-semaphore:
		return func() { semaphore <- struct{}{} }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func Open(root string) (*Store, error) {
	if root == "" {
		return nil, fmt.Errorf("数据目录不能为空")
	}
	info, err := os.Stat(root)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(root, 0o750); err != nil {
			return nil, fmt.Errorf("创建数据目录: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("检查数据目录: %w", err)
	} else if !info.IsDir() {
		return nil, fmt.Errorf("数据路径不是目录")
	}
	return &Store{root: root, locks: make(map[string]*sync.Mutex), coordinator: coordinatorFor(root)}, nil
}

func (s *Store) incidentLock(id string) *sync.Mutex {
	s.guard.Lock()
	defer s.guard.Unlock()
	lock, ok := s.locks[id]
	if !ok {
		lock = &sync.Mutex{}
		s.locks[id] = lock
	}
	return lock
}

func (s *Store) lockIncident(ctx context.Context, id string) (func(), error) {
	local := s.incidentLock(id)
	local.Lock()
	releaseRoot, err := s.coordinator.acquire(ctx, id)
	if err != nil {
		local.Unlock()
		return nil, err
	}
	return func() {
		releaseRoot()
		local.Unlock()
	}, nil
}

func (s *Store) path(id string) string { return filepath.Join(s.root, id+".json") }

func (s *Store) Load(ctx context.Context, id string) (*preservation.PreservationIncident, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := preservation.ValidateIdentifier("incident_id", id); err != nil {
		return nil, err
	}
	unlock, err := s.lockIncident(ctx, id)
	if err != nil {
		return nil, err
	}
	defer unlock()
	envelope, err := s.readUnlocked(id)
	if err != nil {
		return nil, err
	}
	return preservation.Clone(envelope.Incident)
}

func (s *Store) readUnlocked(id string) (*incidentFile, error) {
	data, err := os.ReadFile(s.path(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil, &preservation.DomainError{Code: preservation.CodeNotFound, Message: "事件不存在"}
	}
	if err != nil {
		return nil, fmt.Errorf("读取事件: %w", err)
	}
	var envelope incidentFile
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, &preservation.DomainError{Code: preservation.CodeIntegrity, Message: "事件文件 JSON 损坏"}
	}
	if err := verifyEnvelope(&envelope); err != nil {
		return nil, err
	}
	return &envelope, nil
}

func (s *Store) writeUnlocked(id string, envelope *incidentFile) error {
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化事件文件: %w", err)
	}
	temp, err := os.CreateTemp(s.root, ".incident-*.tmp")
	if err != nil {
		return fmt.Errorf("创建临时事件文件: %w", err)
	}
	tempName := temp.Name()
	cleanup := func() { _ = temp.Close(); _ = os.Remove(tempName) }
	if err := temp.Chmod(0o640); err != nil {
		cleanup()
		return fmt.Errorf("设置临时文件权限: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("写入临时事件文件: %w", err)
	}
	if err := temp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("同步临时事件文件: %w", err)
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempName)
		return fmt.Errorf("关闭临时事件文件: %w", err)
	}
	if err := os.Rename(tempName, s.path(id)); err != nil {
		_ = os.Remove(tempName)
		return fmt.Errorf("原子替换事件文件: %w", err)
	}
	directory, err := os.Open(s.root)
	if err != nil {
		return fmt.Errorf("打开数据目录: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("同步数据目录: %w", err)
	}
	return nil
}
