package obsfeedback

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Store persists Observation Records outside paper_sim_* tables.
type Store interface {
	Save(records []Record) error
	List() ([]Record, error)
}

// MemoryStore is the test / process-local store.
type MemoryStore struct {
	mu      sync.Mutex
	byKey   map[string]Record
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{byKey: map[string]Record{}}
}

func recordKey(r Record) string {
	return strings.TrimSpace(r.ObservationDate) + "|" + strings.TrimSpace(r.Symbol)
}

func (s *MemoryStore) Save(records []Record) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byKey == nil {
		s.byKey = map[string]Record{}
	}
	for _, r := range records {
		if strings.TrimSpace(r.Symbol) == "" || strings.TrimSpace(r.ObservationDate) == "" {
			continue
		}
		s.byKey[recordKey(r)] = r
	}
	return nil
}

func (s *MemoryStore) List() ([]Record, error) {
	if s == nil {
		return []Record{}, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Record, 0, len(s.byKey))
	for _, r := range s.byKey {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ObservationDate == out[j].ObservationDate {
			return out[i].Symbol < out[j].Symbol
		}
		return out[i].ObservationDate < out[j].ObservationDate
	})
	return out, nil
}

// JSONStore is the E.4 production persistence (not a trading table).
type JSONStore struct {
	Path string
	mu   sync.Mutex
}

func NewJSONStore(path string) *JSONStore {
	if strings.TrimSpace(path) == "" {
		path = filepath.Join("data", "paper_observation_history.json")
	}
	return &JSONStore{Path: path}
}

func (s *JSONStore) Save(records []Record) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, _ := s.readUnlocked()
	byKey := map[string]Record{}
	for _, r := range existing {
		byKey[recordKey(r)] = r
	}
	for _, r := range records {
		if strings.TrimSpace(r.Symbol) == "" || strings.TrimSpace(r.ObservationDate) == "" {
			continue
		}
		byKey[recordKey(r)] = r
	}
	out := make([]Record, 0, len(byKey))
	for _, r := range byKey {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ObservationDate == out[j].ObservationDate {
			return out[i].Symbol < out[j].Symbol
		}
		return out[i].ObservationDate < out[j].ObservationDate
	})
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.Path, raw, 0o644)
}

func (s *JSONStore) List() ([]Record, error) {
	if s == nil {
		return []Record{}, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readUnlocked()
}

func (s *JSONStore) readUnlocked() ([]Record, error) {
	raw, err := os.ReadFile(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Record{}, nil
		}
		return nil, err
	}
	var recs []Record
	if err := json.Unmarshal(raw, &recs); err != nil {
		return []Record{}, nil
	}
	return recs, nil
}

var (
	activeMu    sync.Mutex
	activeStore Store = NewMemoryStore()
)

func ActiveStore() Store {
	activeMu.Lock()
	defer activeMu.Unlock()
	return activeStore
}

func SetStore(s Store) Store {
	activeMu.Lock()
	defer activeMu.Unlock()
	prev := activeStore
	if s == nil {
		s = NewMemoryStore()
	}
	activeStore = s
	return prev
}

// SetStoreForTest swaps the process store; caller should defer restore.
func SetStoreForTest(s Store) func() {
	prev := SetStore(s)
	return func() { SetStore(prev) }
}

// RunningUnderTest reports go test (avoid writing JSON / capturing into global store).
func RunningUnderTest() bool {
	return flag.Lookup("test.v") != nil
}

// PersistCapture writes records to the active store (and JSON outside tests).
func PersistCapture(records []Record) error {
	if len(records) == 0 {
		return nil
	}
	if err := ActiveStore().Save(records); err != nil {
		return err
	}
	if RunningUnderTest() {
		return nil
	}
	return NewJSONStore("").Save(records)
}

// LoadRecords returns memory store ∪ JSON file (production restart).
func LoadRecords() ([]Record, error) {
	mem, err := ActiveStore().List()
	if err != nil {
		return nil, err
	}
	if RunningUnderTest() {
		return mem, nil
	}
	file, err := NewJSONStore("").List()
	if err != nil || len(file) == 0 {
		return mem, nil
	}
	merged := NewMemoryStore()
	_ = merged.Save(file)
	_ = merged.Save(mem)
	return merged.List()
}
