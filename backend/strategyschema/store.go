package strategyschema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	runtimeutil "go-stock/backend/runtime"
)

const storeFileName = "strategy_schemas.json"

type storeFile struct {
	Version     int                   `json:"version"`
	Definitions map[string]Definition `json:"definitions"`
	Revisions   map[string]Revision   `json:"revisions"` // key = revision_id
}

// Store is sidecar-backed schema storage (B3-B adds Mutate with atomic write).
type Store interface {
	ListDefinitions() ([]Definition, error)
	GetDefinition(strategyID string) (Definition, bool, error)
	ListRevisions(strategyID string) ([]Revision, error)
	GetRevision(strategyID, revision string) (Revision, bool, error)
	GetRevisionByID(revisionID string) (Revision, bool, error)
	// Mutate loads, applies mutator, validates ≤1 active, then persists atomically.
	Mutate(mut func(f *storeFile) error) error
	Reset() // tests
}

type memoryStore struct {
	mu   sync.RWMutex
	file storeFile
}

func newMemoryStore(seed storeFile) *memoryStore {
	if seed.Definitions == nil {
		seed.Definitions = map[string]Definition{}
	}
	if seed.Revisions == nil {
		seed.Revisions = map[string]Revision{}
	}
	if seed.Version == 0 {
		seed.Version = 1
	}
	return &memoryStore{file: seed}
}

func (s *memoryStore) snapshot() storeFile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneStoreFile(s.file)
}

func (s *memoryStore) ListDefinitions() ([]Definition, error) {
	f := s.snapshot()
	out := make([]Definition, 0, len(f.Definitions))
	for _, d := range f.Definitions {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].StrategyID < out[j].StrategyID
	})
	return out, nil
}

func (s *memoryStore) GetDefinition(strategyID string) (Definition, bool, error) {
	f := s.snapshot()
	d, ok := f.Definitions[strings.TrimSpace(strategyID)]
	return d, ok, nil
}

func (s *memoryStore) ListRevisions(strategyID string) ([]Revision, error) {
	f := s.snapshot()
	strategyID = strings.TrimSpace(strategyID)
	out := make([]Revision, 0)
	for _, r := range f.Revisions {
		if r.StrategyID == strategyID {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return revisionLess(out[j], out[i]) // newer first
	})
	return out, nil
}

func (s *memoryStore) GetRevision(strategyID, revision string) (Revision, bool, error) {
	f := s.snapshot()
	strategyID = strings.TrimSpace(strategyID)
	revision = strings.TrimSpace(revision)
	for _, r := range f.Revisions {
		if r.StrategyID == strategyID && r.Revision == revision {
			return r, true, nil
		}
	}
	return Revision{}, false, nil
}

func (s *memoryStore) GetRevisionByID(revisionID string) (Revision, bool, error) {
	f := s.snapshot()
	r, ok := f.Revisions[strings.TrimSpace(revisionID)]
	return r, ok, nil
}

func (s *memoryStore) Mutate(mut func(f *storeFile) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := cloneStoreFile(s.file)
	if err := mut(&next); err != nil {
		return err
	}
	if err := validateStoreInvariants(next); err != nil {
		return err
	}
	s.file = next
	return nil
}

func (s *memoryStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.file = storeFile{Version: 1, Definitions: map[string]Definition{}, Revisions: map[string]Revision{}}
}

type fileStore struct {
	mu   sync.Mutex
	path string
	mem  storeFile
}

func newFileStore(path string) *fileStore {
	return &fileStore{path: path, mem: storeFile{Version: 1, Definitions: map[string]Definition{}, Revisions: map[string]Revision{}}}
}

func (s *fileStore) loadLocked() error {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.mem = storeFile{Version: 1, Definitions: map[string]Definition{}, Revisions: map[string]Revision{}}
			return nil
		}
		return err
	}
	if len(raw) == 0 {
		s.mem = storeFile{Version: 1, Definitions: map[string]Definition{}, Revisions: map[string]Revision{}}
		return nil
	}
	var f storeFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return err
	}
	if f.Definitions == nil {
		f.Definitions = map[string]Definition{}
	}
	if f.Revisions == nil {
		f.Revisions = map[string]Revision{}
	}
	if f.Version == 0 {
		f.Version = 1
	}
	s.mem = f
	return nil
}

func (s *fileStore) ListDefinitions() ([]Definition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return nil, err
	}
	ms := newMemoryStore(s.mem)
	return ms.ListDefinitions()
}

func (s *fileStore) GetDefinition(strategyID string) (Definition, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return Definition{}, false, err
	}
	d, ok := s.mem.Definitions[strings.TrimSpace(strategyID)]
	return d, ok, nil
}

func (s *fileStore) ListRevisions(strategyID string) ([]Revision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return nil, err
	}
	ms := newMemoryStore(s.mem)
	return ms.ListRevisions(strategyID)
}

func (s *fileStore) GetRevision(strategyID, revision string) (Revision, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return Revision{}, false, err
	}
	ms := newMemoryStore(s.mem)
	return ms.GetRevision(strategyID, revision)
}

func (s *fileStore) GetRevisionByID(revisionID string) (Revision, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return Revision{}, false, err
	}
	r, ok := s.mem.Revisions[strings.TrimSpace(revisionID)]
	return r, ok, nil
}

func (s *fileStore) Mutate(mut func(f *storeFile) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return err
	}
	next := cloneStoreFile(s.mem)
	if err := mut(&next); err != nil {
		return err
	}
	if err := validateStoreInvariants(next); err != nil {
		return err
	}
	if err := atomicWriteStoreFile(s.path, next); err != nil {
		return err
	}
	s.mem = next
	return nil
}

func (s *fileStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mem = storeFile{Version: 1, Definitions: map[string]Definition{}, Revisions: map[string]Revision{}}
	_ = os.Remove(s.path)
	_ = os.Remove(s.path + ".bak")
	_ = os.Remove(s.path + ".tmp")
}

func validateStoreInvariants(f storeFile) error {
	activeByDef := map[string]int{}
	for _, r := range f.Revisions {
		if r.Status == RevisionStatusActive {
			activeByDef[r.StrategyID]++
		}
	}
	for id, n := range activeByDef {
		if n > 1 {
			return WriteError{
				Code:    CodeDraftConflict,
				Message: fmt.Sprintf("invariant violated: definition %s has %d active revisions", id, n),
			}
		}
	}
	return nil
}

// atomicWriteStoreFile writes via tmp + backup + rename (Windows-safe).
func atomicWriteStoreFile(path string, f storeFile) error {
	if f.Version == 0 {
		f.Version = 1
	}
	if f.Definitions == nil {
		f.Definitions = map[string]Definition{}
	}
	if f.Revisions == nil {
		f.Revisions = map[string]Revision{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	// rolling backup of previous good file
	if _, err := os.Stat(path); err == nil {
		bak := path + ".bak"
		_ = os.Remove(bak)
		if cpErr := copyFile(path, bak); cpErr != nil {
			_ = os.Remove(tmp)
			return cpErr
		}
	}
	_ = os.Remove(path) // required on Windows before rename onto existing
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func copyFile(src, dst string) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, raw, 0o644)
}

func cloneStoreFile(in storeFile) storeFile {
	out := storeFile{
		Version:     in.Version,
		Definitions: map[string]Definition{},
		Revisions:   map[string]Revision{},
	}
	for k, v := range in.Definitions {
		out.Definitions[k] = v
	}
	for k, v := range in.Revisions {
		out.Revisions[k] = v
	}
	return out
}

func revisionLess(a, b Revision) bool {
	// true if a < b (older)
	if !a.UpdatedAt.Equal(b.UpdatedAt) {
		return a.UpdatedAt.Before(b.UpdatedAt)
	}
	return a.Revision < b.Revision
}

func storePath() string {
	return runtimeutil.GetConfigPath(storeFileName)
}

var (
	storeMu      sync.RWMutex
	defaultStore Store
	storeOverride Store
)

// DefaultStore returns file-backed store under data/strategy_schemas.json.
func DefaultStore() Store {
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
		defaultStore = newFileStore(storePath())
	}
	return defaultStore
}

// SetStoreForTest replaces the schema store.
func SetStoreForTest(s Store) {
	storeMu.Lock()
	defer storeMu.Unlock()
	storeOverride = s
}

// ResetStoreForTest clears override.
func ResetStoreForTest() {
	storeMu.Lock()
	defer storeMu.Unlock()
	if storeOverride != nil {
		storeOverride.Reset()
		storeOverride = nil
	}
}

// NewMemoryStoreForTest builds an in-memory store from a seed file struct.
func NewMemoryStoreForTest(defs map[string]Definition, revs map[string]Revision) Store {
	return newMemoryStore(storeFile{Version: 1, Definitions: defs, Revisions: revs})
}

// NewFileStoreForTest builds a file-backed store at an explicit path (sidecar reload tests).
func NewFileStoreForTest(path string) Store {
	return newFileStore(path)
}

// StorePath is the cwd-relative sidecar path.
func StorePath() string { return storePath() }

// LoadStoreFileJSON unmarshals raw sidecar JSON (tests / seed helpers).
func LoadStoreFileJSON(raw []byte) (storeFile, error) {
	var f storeFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return storeFile{}, err
	}
	if f.Definitions == nil {
		f.Definitions = map[string]Definition{}
	}
	if f.Revisions == nil {
		f.Revisions = map[string]Revision{}
	}
	return f, nil
}
