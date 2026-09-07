package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/decision/semantic"
	"go-stock/backend/models"
)

// Conflict kinds (Phase3-C integrity).
const (
	ConflictKindIDContent  = "id_content_conflict"  // same DecisionID, different semantic hash
	ConflictKindActionCode = "action_code_conflict" // same code+asOf, different Action.Code
)

// Conflict recorded integrity / semantic conflict (does not mutate Candidate Rank/Score).
type Conflict struct {
	Kind            string `json:"kind"`
	DecisionID      string `json:"decisionId"`
	OtherDecisionID string `json:"otherDecisionId,omitempty"`
	Code            string `json:"code,omitempty"`
	AsOf            string `json:"asOf,omitempty"`
	LeftActionCode  string `json:"leftActionCode,omitempty"`
	RightActionCode string `json:"rightActionCode,omitempty"`
	LeftHash        string `json:"leftHash,omitempty"`
	RightHash       string `json:"rightHash,omitempty"`
	Summary         string `json:"summary"`
	At              string `json:"at"`
}

// Snapshot sealed immutable Decision view.
type Snapshot struct {
	DecisionID string                `json:"decisionId"`
	Hash       string                `json:"hash"`
	SealedAt   string                `json:"sealedAt"`
	Decision   *models.QuantDecision `json:"decision"`
}

// PutResult outcome of immutable Put.
type PutResult struct {
	DecisionID string    `json:"decisionId"`
	Hash       string    `json:"hash"`
	Created    bool      `json:"created"`
	Idempotent bool      `json:"idempotent"`
	Conflict   *Conflict `json:"conflict,omitempty"`
}

// IntegrityResult Verify result for a DecisionID.
type IntegrityResult struct {
	DecisionID  string `json:"decisionId"`
	Found       bool   `json:"found"`
	Intact      bool   `json:"intact"`
	StoredHash  string `json:"storedHash,omitempty"`
	CurrentHash string `json:"currentHash,omitempty"`
	Summary     string `json:"summary"`
}

// SemanticContentHash hashes Phase2-B0 semantic normalization (immutable content identity).
func SemanticContentHash(d *models.QuantDecision) string {
	if d == nil {
		return ""
	}
	n := semantic.NormalizeDecision(d)
	b, err := json.Marshal(n)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func codeAsOfKey(d *models.QuantDecision) string {
	if d == nil {
		return ""
	}
	code := normalizeCode(d.Instrument.StockCode)
	asOf := strings.TrimSpace(formatAsOf(d.AsOf))
	if code == "" || asOf == "" {
		return ""
	}
	return code + "|" + asOf
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// PutImmutable seals a Decision snapshot: same ID + same hash is idempotent;
// same ID + different hash keeps the original and records a conflict.
// Same code+asOf with a different Action.Code also records a conflict (first wins).
// Never mutates CandidatePoolItem Score/Rank; never touches TradePlan/Execution.
func (r *Registry) PutImmutable(d *models.QuantDecision) (PutResult, error) {
	if r == nil {
		return PutResult{}, fmt.Errorf("registry: nil")
	}
	if d == nil {
		return PutResult{}, fmt.Errorf("registry: nil decision")
	}
	id := EnsureDecisionID(d)
	if id == "" {
		return PutResult{}, fmt.Errorf("registry: empty decision id")
	}
	hash := SemanticContentHash(d)
	if hash == "" {
		return PutResult{}, fmt.Errorf("registry: empty content hash")
	}
	clone := cloneDecision(d)
	clone.ID = id
	clone.Meta.DecisionHash = hash
	key := codeAsOfKey(clone)
	at := nowRFC3339()

	r.mu.Lock()
	defer r.mu.Unlock()
	r.ensureMaps()

	if prev, ok := r.byID[id]; ok && prev != nil {
		prevHash := r.hashByID[id]
		if prevHash == hash {
			return PutResult{DecisionID: id, Hash: hash, Created: false, Idempotent: true}, nil
		}
		c := Conflict{
			Kind:       ConflictKindIDContent,
			DecisionID: id,
			Code:       normalizeCode(prev.Instrument.StockCode),
			AsOf:       formatAsOf(prev.AsOf),
			LeftHash:   prevHash,
			RightHash:  hash,
			LeftActionCode:  prev.Action.Code,
			RightActionCode: clone.Action.Code,
			Summary:    "IMMUTABLE: refuse overwrite; same decisionId different content hash",
			At:         at,
		}
		r.appendConflictLocked(c)
		cp := c
		return PutResult{DecisionID: id, Hash: prevHash, Created: false, Idempotent: false, Conflict: &cp}, nil
	}

	// action.code conflict: same code+asOf, different action, different id
	if key != "" {
		if existingID, ok := r.byCodeAsOf[key]; ok && existingID != id {
			if existing, ok2 := r.byID[existingID]; ok2 && existing != nil {
				if existing.Action.Code != clone.Action.Code {
					c := Conflict{
						Kind:              ConflictKindActionCode,
						DecisionID:        existingID,
						OtherDecisionID:   id,
						Code:              normalizeCode(clone.Instrument.StockCode),
						AsOf:              formatAsOf(clone.AsOf),
						LeftActionCode:    existing.Action.Code,
						RightActionCode:   clone.Action.Code,
						LeftHash:          r.hashByID[existingID],
						RightHash:         hash,
						Summary:           "CONFLICT: same code+asOf different action.code",
						At:                at,
					}
					r.appendConflictLocked(c)
					// still store the new snapshot (immutable archive); conflict is observational
					r.byID[id] = clone
					r.hashByID[id] = hash
					r.sealedAt[id] = at
					cp := c
					return PutResult{DecisionID: id, Hash: hash, Created: true, Conflict: &cp}, nil
				}
			}
		} else {
			r.byCodeAsOf[key] = id
		}
	}

	r.byID[id] = clone
	r.hashByID[id] = hash
	r.sealedAt[id] = at
	if key != "" {
		if _, exists := r.byCodeAsOf[key]; !exists {
			r.byCodeAsOf[key] = id
		}
	}
	return PutResult{DecisionID: id, Hash: hash, Created: true}, nil
}

// GetSnapshot returns a sealed clone + content hash (read-only view).
func (r *Registry) GetSnapshot(decisionID string) (Snapshot, bool) {
	if r == nil || decisionID == "" {
		return Snapshot{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.byID[decisionID]
	if !ok || d == nil {
		return Snapshot{}, false
	}
	return Snapshot{
		DecisionID: decisionID,
		Hash:       r.hashByID[decisionID],
		SealedAt:   r.sealedAt[decisionID],
		Decision:   cloneDecision(d),
	}, true
}

// Verify recomputes semantic hash and compares to sealed hash.
func (r *Registry) Verify(decisionID string) IntegrityResult {
	out := IntegrityResult{DecisionID: decisionID}
	snap, ok := r.GetSnapshot(decisionID)
	if !ok {
		out.Found = false
		out.Intact = false
		out.Summary = "MISSING: decisionId not in registry"
		return out
	}
	out.Found = true
	out.StoredHash = snap.Hash
	out.CurrentHash = SemanticContentHash(snap.Decision)
	out.Intact = out.StoredHash != "" && out.StoredHash == out.CurrentHash
	if out.Intact {
		out.Summary = "INTACT: content hash matches sealed snapshot"
	} else {
		out.Summary = "DRIFT: content hash mismatch (should not happen if clones are sealed)"
	}
	return out
}

// Conflicts returns a copy of recorded integrity conflicts.
func (r *Registry) Conflicts() []Conflict {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Conflict, len(r.conflicts))
	copy(out, r.conflicts)
	return out
}

// ConflictCount helper.
func (r *Registry) ConflictCount() int {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.conflicts)
}

func (r *Registry) ensureMaps() {
	if r.byID == nil {
		r.byID = map[string]*models.QuantDecision{}
	}
	if r.hashByID == nil {
		r.hashByID = map[string]string{}
	}
	if r.sealedAt == nil {
		r.sealedAt = map[string]string{}
	}
	if r.byCodeAsOf == nil {
		r.byCodeAsOf = map[string]string{}
	}
}

func (r *Registry) appendConflictLocked(c Conflict) {
	// de-dupe identical kind+ids+hashes
	for _, e := range r.conflicts {
		if e.Kind == c.Kind && e.DecisionID == c.DecisionID && e.OtherDecisionID == c.OtherDecisionID &&
			e.LeftHash == c.LeftHash && e.RightHash == c.RightHash &&
			e.LeftActionCode == c.LeftActionCode && e.RightActionCode == c.RightActionCode {
			return
		}
	}
	r.conflicts = append(r.conflicts, c)
}
