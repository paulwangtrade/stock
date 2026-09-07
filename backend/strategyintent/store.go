package strategyintent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	runtimeutil "go-stock/backend/runtime"
)

const storeFileName = "strategy_intents.json"

type storeFile struct {
	Version int               `json:"version"`
	Intents map[string]Intent `json:"intents"` // key = intent id
}

// Store is sidecar-backed intent storage (B1-B adds Mutate with atomic write).
type Store interface {
	List() ([]Intent, error)
	Get(id string) (Intent, bool, error)
	Mutate(mut func(f *storeFile) error) error
	Reset()
}

type memoryStore struct {
	mu   sync.RWMutex
	file storeFile
}

func newMemoryStore(seed storeFile) *memoryStore {
	if seed.Intents == nil {
		seed.Intents = map[string]Intent{}
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

func (s *memoryStore) List() ([]Intent, error) {
	f := s.snapshot()
	return listFromFile(f), nil
}

func (s *memoryStore) Get(id string) (Intent, bool, error) {
	f := s.snapshot()
	in, ok := f.Intents[strings.TrimSpace(id)]
	return in, ok, nil
}

func (s *memoryStore) Mutate(mut func(f *storeFile) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := cloneStoreFile(s.file)
	if err := mut(&next); err != nil {
		return err
	}
	s.file = next
	return nil
}

func (s *memoryStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.file = storeFile{Version: 1, Intents: map[string]Intent{}}
}

type fileStore struct {
	mu   sync.Mutex
	path string
	mem  storeFile
}

func newFileStore(path string) *fileStore {
	return &fileStore{
		path: path,
		mem:  storeFile{Version: 1, Intents: map[string]Intent{}},
	}
}

func (s *fileStore) loadLocked() error {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.mem = storeFile{Version: 1, Intents: map[string]Intent{}}
			return nil
		}
		return err
	}
	if len(raw) == 0 {
		s.mem = storeFile{Version: 1, Intents: map[string]Intent{}}
		return nil
	}
	var f storeFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return err
	}
	if f.Intents == nil {
		f.Intents = map[string]Intent{}
	}
	if f.Version == 0 {
		f.Version = 1
	}
	s.mem = f
	return nil
}

func (s *fileStore) List() ([]Intent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return nil, err
	}
	return listFromFile(s.mem), nil
}

func (s *fileStore) Get(id string) (Intent, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return Intent{}, false, err
	}
	in, ok := s.mem.Intents[strings.TrimSpace(id)]
	return in, ok, nil
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
	if err := atomicWriteStoreFile(s.path, next); err != nil {
		return err
	}
	s.mem = next
	return nil
}

func (s *fileStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mem = storeFile{Version: 1, Intents: map[string]Intent{}}
	_ = os.Remove(s.path)
	_ = os.Remove(s.path + ".bak")
	_ = os.Remove(s.path + ".tmp")
}

func listFromFile(f storeFile) []Intent {
	out := make([]Intent, 0, len(f.Intents))
	for _, in := range f.Intents {
		out = append(out, in)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func cloneStoreFile(in storeFile) storeFile {
	out := storeFile{
		Version: in.Version,
		Intents: map[string]Intent{},
	}
	for k, v := range in.Intents {
		out.Intents[k] = v
	}
	if out.Version == 0 {
		out.Version = 1
	}
	return out
}

func atomicWriteStoreFile(path string, f storeFile) error {
	if f.Version == 0 {
		f.Version = 1
	}
	if f.Intents == nil {
		f.Intents = map[string]Intent{}
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
	if _, err := os.Stat(path); err == nil {
		bak := path + ".bak"
		_ = os.Remove(bak)
		if cpErr := copyFile(path, bak); cpErr != nil {
			_ = os.Remove(tmp)
			return cpErr
		}
	}
	_ = os.Remove(path)
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

func storePath() string {
	return runtimeutil.GetConfigPath(storeFileName)
}

var (
	storeMu       sync.RWMutex
	defaultStore  Store
	storeOverride Store
)

// DefaultStore returns file-backed store under data/strategy_intents.json.
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

// SetStoreForTest replaces the intent store.
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

// NewMemoryStoreForTest builds an in-memory store from intents.
func NewMemoryStoreForTest(intents map[string]Intent) Store {
	return newMemoryStore(storeFile{Version: 1, Intents: intents})
}

// NewFileStoreForTest builds a file-backed store at an explicit path.
func NewFileStoreForTest(path string) Store {
	return newFileStore(path)
}

// LoadStoreFileJSON unmarshals raw sidecar JSON (tests).
func LoadStoreFileJSON(raw []byte) (storeFile, error) {
	var f storeFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return storeFile{}, err
	}
	if f.Intents == nil {
		f.Intents = map[string]Intent{}
	}
	return f, nil
}
