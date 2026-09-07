package research

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	runtimeutil "go-stock/backend/runtime"
)

const annotationStoreFile = "research_candidates.json"

// Annotation is the persisted research-only overlay (sidecar / memory).
// It never writes Trade Candidate / TradePlan / Broker state.
type Annotation struct {
	Status    string    `json:"status,omitempty"`
	Note      string    `json:"note,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	HasTags   bool      `json:"has_tags,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type annotationFile struct {
	Version     int                    `json:"version"`
	Annotations map[string]Annotation  `json:"annotations"`
}

// AnnotationStore persists research status/note/tags without a formal DB table.
type AnnotationStore interface {
	Get(id string) (Annotation, bool)
	Patch(id string, patch UpdatePatch) (Annotation, error)
	Reset() // tests
}

type memoryStore struct {
	mu   sync.RWMutex
	data map[string]Annotation
}

func newMemoryStore() *memoryStore {
	return &memoryStore{data: map[string]Annotation{}}
}

func (s *memoryStore) Get(id string) (Annotation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.data[id]
	return a, ok
}

func (s *memoryStore) Patch(id string, patch UpdatePatch) (Annotation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur := s.data[id]
	applyPatch(&cur, patch)
	s.data[id] = cur
	return cur, nil
}

func (s *memoryStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = map[string]Annotation{}
}

type fileStore struct {
	mu   sync.Mutex
	path string
	mem  map[string]Annotation
}

func newFileStore(path string) *fileStore {
	return &fileStore{path: path, mem: map[string]Annotation{}}
}

func (s *fileStore) loadLocked() error {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.mem = map[string]Annotation{}
			return nil
		}
		return err
	}
	if len(raw) == 0 {
		s.mem = map[string]Annotation{}
		return nil
	}
	var f annotationFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return err
	}
	if f.Annotations == nil {
		f.Annotations = map[string]Annotation{}
	}
	s.mem = f.Annotations
	return nil
}

func (s *fileStore) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	f := annotationFile{Version: 1, Annotations: s.mem}
	if f.Annotations == nil {
		f.Annotations = map[string]Annotation{}
	}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o644)
}

func (s *fileStore) Get(id string) (Annotation, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.loadLocked()
	a, ok := s.mem[id]
	return a, ok
}

func (s *fileStore) Patch(id string, patch UpdatePatch) (Annotation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return Annotation{}, err
	}
	cur := s.mem[id]
	applyPatch(&cur, patch)
	s.mem[id] = cur
	if err := s.saveLocked(); err != nil {
		return Annotation{}, err
	}
	return cur, nil
}

func (s *fileStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mem = map[string]Annotation{}
	_ = os.Remove(s.path)
}

func applyPatch(cur *Annotation, patch UpdatePatch) {
	if patch.Status != nil {
		cur.Status = *patch.Status
	}
	if patch.Note != nil {
		cur.Note = *patch.Note
	}
	if patch.Tags != nil {
		tags := append([]string(nil), (*patch.Tags)...)
		cur.Tags = tags
		cur.HasTags = true
	}
	cur.UpdatedAt = time.Now().UTC()
}

var (
	storeMu       sync.RWMutex
	defaultStore  AnnotationStore
	storeOverride AnnotationStore
)

func annotationStorePath() string {
	return runtimeutil.GetConfigPath(annotationStoreFile)
}

// DefaultStore returns the process annotation store (file sidecar under data/).
func DefaultStore() AnnotationStore {
	storeMu.RLock()
	if storeOverride != nil {
		s := storeOverride
		storeMu.RUnlock()
		return s
	}
	if defaultStore != nil {
		s := defaultStore
		storeMu.RUnlock()
		return s
	}
	storeMu.RUnlock()

	storeMu.Lock()
	defer storeMu.Unlock()
	if storeOverride != nil {
		return storeOverride
	}
	if defaultStore == nil {
		defaultStore = newFileStore(annotationStorePath())
	}
	return defaultStore
}

// SetStoreForTest replaces the annotation store (memory recommended).
func SetStoreForTest(s AnnotationStore) {
	storeMu.Lock()
	defer storeMu.Unlock()
	storeOverride = s
}

// ResetStoreForTest clears override and resets default file store if present.
func ResetStoreForTest() {
	storeMu.Lock()
	defer storeMu.Unlock()
	if storeOverride != nil {
		storeOverride.Reset()
		storeOverride = nil
	}
	if defaultStore != nil {
		defaultStore.Reset()
	}
}

// NewMemoryStoreForTest exposes an in-memory store for unit/API tests.
func NewMemoryStoreForTest() AnnotationStore {
	return newMemoryStore()
}

// AnnotationStorePath is the cwd-relative sidecar path.
func AnnotationStorePath() string { return annotationStorePath() }
