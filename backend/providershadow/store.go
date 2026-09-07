package providershadow

import (
	"encoding/json"
	"os"
	"sync"
)

// RecordStore persists ShadowComparisonRecord rows independently of trade_plans.
type RecordStore interface {
	Append(ShadowComparisonRecord) error
	List() []ShadowComparisonRecord
}

// MemoryStore is the in-process append-only implementation used by tests.
type MemoryStore struct {
	mu      sync.Mutex
	records []ShadowComparisonRecord
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{records: []ShadowComparisonRecord{}}
}

func (s *MemoryStore) Append(rec ShadowComparisonRecord) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, rec)
	return nil
}

func (s *MemoryStore) List() []ShadowComparisonRecord {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ShadowComparisonRecord, len(s.records))
	copy(out, s.records)
	return out
}

// FileStore appends one JSON object per line. It is a test/ops sink, not a trade table.
type FileStore struct {
	mu   sync.Mutex
	Path string
	mem  []ShadowComparisonRecord
}

func NewFileStore(path string) *FileStore {
	return &FileStore{Path: path, mem: []ShadowComparisonRecord{}}
}

func (s *FileStore) Append(rec ShadowComparisonRecord) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mem = append(s.mem, rec)
	if s.Path == "" {
		return nil
	}
	f, err := os.OpenFile(s.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	return enc.Encode(rec)
}

func (s *FileStore) List() []ShadowComparisonRecord {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ShadowComparisonRecord, len(s.mem))
	copy(out, s.mem)
	return out
}

// DiscardStore ignores records. Used when Enabled is false or persist is optional.
type DiscardStore struct{}

func (DiscardStore) Append(ShadowComparisonRecord) error { return nil }
func (DiscardStore) List() []ShadowComparisonRecord      { return nil }
