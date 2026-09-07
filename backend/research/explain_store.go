package research

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	runtimeutil "go-stock/backend/runtime"
)

const explainStoreFile = "research_explains.json"

// ExplainOverlay is the persisted manual layer over derived Explain (sidecar / memory).
type ExplainOverlay struct {
	Summary            string          `json:"summary,omitempty"`
	HasSummary         bool            `json:"has_summary,omitempty"`
	ResearchReason     *ResearchReason `json:"research_reason,omitempty"`
	RiskNote           *RiskNote       `json:"risk_note,omitempty"`
	HasRiskNote        bool            `json:"has_risk_note,omitempty"`
	ClearRiskNote      bool            `json:"clear_risk_note,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

type explainFile struct {
	Version  int                       `json:"version"`
	Overlays map[string]ExplainOverlay `json:"overlays"` // key = candidate_id
}

// ExplainStore persists manual explain edits without a formal DB table.
type ExplainStore interface {
	Get(candidateID string) (ExplainOverlay, bool)
	Patch(candidateID string, patch ExplainPatch) (ExplainOverlay, error)
	Reset()
}

type explainMemoryStore struct {
	mu   sync.RWMutex
	data map[string]ExplainOverlay
}

func newExplainMemoryStore() *explainMemoryStore {
	return &explainMemoryStore{data: map[string]ExplainOverlay{}}
}

func (s *explainMemoryStore) Get(candidateID string) (ExplainOverlay, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.data[candidateID]
	return o, ok
}

func (s *explainMemoryStore) Patch(candidateID string, patch ExplainPatch) (ExplainOverlay, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur := s.data[candidateID]
	applyExplainOverlay(&cur, patch)
	s.data[candidateID] = cur
	return cur, nil
}

func (s *explainMemoryStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = map[string]ExplainOverlay{}
}

type explainFileStore struct {
	mu   sync.Mutex
	path string
	mem  map[string]ExplainOverlay
}

func newExplainFileStore(path string) *explainFileStore {
	return &explainFileStore{path: path, mem: map[string]ExplainOverlay{}}
}

func (s *explainFileStore) loadLocked() error {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.mem = map[string]ExplainOverlay{}
			return nil
		}
		return err
	}
	if len(raw) == 0 {
		s.mem = map[string]ExplainOverlay{}
		return nil
	}
	var f explainFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return err
	}
	if f.Overlays == nil {
		f.Overlays = map[string]ExplainOverlay{}
	}
	s.mem = f.Overlays
	return nil
}

func (s *explainFileStore) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	f := explainFile{Version: 1, Overlays: s.mem}
	if f.Overlays == nil {
		f.Overlays = map[string]ExplainOverlay{}
	}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o644)
}

func (s *explainFileStore) Get(candidateID string) (ExplainOverlay, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.loadLocked()
	o, ok := s.mem[candidateID]
	return o, ok
}

func (s *explainFileStore) Patch(candidateID string, patch ExplainPatch) (ExplainOverlay, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.loadLocked(); err != nil {
		return ExplainOverlay{}, err
	}
	cur := s.mem[candidateID]
	applyExplainOverlay(&cur, patch)
	s.mem[candidateID] = cur
	if err := s.saveLocked(); err != nil {
		return ExplainOverlay{}, err
	}
	return cur, nil
}

func (s *explainFileStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mem = map[string]ExplainOverlay{}
	_ = os.Remove(s.path)
}

func applyExplainOverlay(cur *ExplainOverlay, patch ExplainPatch) {
	now := time.Now().UTC()
	if cur.CreatedAt.IsZero() {
		cur.CreatedAt = now
	}
	if patch.Summary != nil {
		cur.Summary = strings.TrimSpace(*patch.Summary)
		cur.HasSummary = true
	}
	if patch.ResearchReason != nil {
		rr := *patch.ResearchReason
		rr.Kind = strings.TrimSpace(rr.Kind)
		rr.Text = strings.TrimSpace(rr.Text)
		if rr.Kind == "" {
			rr.Kind = ReasonKindAnalystNote
		}
		cur.ResearchReason = &rr
	}
	if patch.ClearRiskNote {
		cur.RiskNote = nil
		cur.HasRiskNote = true
		cur.ClearRiskNote = true
	} else if patch.RiskNote != nil {
		rn := *patch.RiskNote
		rn.Severity = strings.TrimSpace(rn.Severity)
		rn.Text = strings.TrimSpace(rn.Text)
		if rn.Severity == "" {
			rn.Severity = RiskSeverityInfo
		}
		cur.RiskNote = &rn
		cur.HasRiskNote = true
		cur.ClearRiskNote = false
	}
	cur.UpdatedAt = now
}

var (
	explainStoreMu       sync.RWMutex
	defaultExplainStore  ExplainStore
	explainStoreOverride ExplainStore
)

func explainStorePath() string {
	return runtimeutil.GetConfigPath(explainStoreFile)
}

// DefaultExplainStore returns file sidecar under data/research_explains.json.
func DefaultExplainStore() ExplainStore {
	explainStoreMu.RLock()
	if explainStoreOverride != nil {
		s := explainStoreOverride
		explainStoreMu.RUnlock()
		return s
	}
	if defaultExplainStore != nil {
		s := defaultExplainStore
		explainStoreMu.RUnlock()
		return s
	}
	explainStoreMu.RUnlock()

	explainStoreMu.Lock()
	defer explainStoreMu.Unlock()
	if explainStoreOverride != nil {
		return explainStoreOverride
	}
	if defaultExplainStore == nil {
		defaultExplainStore = newExplainFileStore(explainStorePath())
	}
	return defaultExplainStore
}

// SetExplainStoreForTest replaces explain overlay store.
func SetExplainStoreForTest(s ExplainStore) {
	explainStoreMu.Lock()
	defer explainStoreMu.Unlock()
	explainStoreOverride = s
}

// ResetExplainStoreForTest clears override / default explain store.
func ResetExplainStoreForTest() {
	explainStoreMu.Lock()
	defer explainStoreMu.Unlock()
	if explainStoreOverride != nil {
		explainStoreOverride.Reset()
		explainStoreOverride = nil
	}
	if defaultExplainStore != nil {
		defaultExplainStore.Reset()
	}
}

// NewExplainMemoryStoreForTest exposes in-memory explain store.
func NewExplainMemoryStoreForTest() ExplainStore {
	return newExplainMemoryStore()
}

// ExplainStorePath is cwd-relative sidecar path.
func ExplainStorePath() string { return explainStorePath() }
