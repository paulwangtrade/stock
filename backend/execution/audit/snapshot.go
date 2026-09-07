package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ReplayContractVersion identifies the fold/serialize contract for recovery
// checkpoints. Changing this requires a full replay; incremental resume is refused.
const ReplayContractVersion = "audit.replay.v1"

var (
	// ErrCheckpointCorrupted is returned when snapshot_json does not match snapshot_hash
	// or cannot be decoded into a ReplaySnapshot.
	ErrCheckpointCorrupted = errors.New("audit replay: checkpoint snapshot corrupted")
	// ErrCheckpointContract is returned when a stored checkpoint uses a different
	// replay contract version than this binary.
	ErrCheckpointContract = errors.New("audit replay: replay contract version mismatch")
	// ErrCheckpointMissing is returned by LoadVerified when no row exists for the scope.
	ErrCheckpointMissing = errors.New("audit replay: checkpoint not found")
)

// ReplaySnapshot is the durable fold state attached to a recovery checkpoint.
// It is derived only from audit_events replay and never writes business tables.
type ReplaySnapshot struct {
	Order         ReplayOrder  `json:"order"`
	Fills         []ReplayFill `json:"fills"`
	FillQty       int64        `json:"fillQty"`
	PositionDelta int64        `json:"positionDelta"`
	SpecHash      string       `json:"specHash"`
	SubmitOutcome string       `json:"submitOutcome"`
	EventsApplied int          `json:"eventsApplied"`
	EventsSkipped int          `json:"eventsSkipped"`
	Divergences   []Divergence `json:"divergences"`
}

// SnapshotFromResult extracts the durable snapshot from a ReplayResult.
func SnapshotFromResult(res ReplayResult) ReplaySnapshot {
	fills := res.Fills
	if fills == nil {
		fills = []ReplayFill{}
	}
	divs := res.Divergences
	if divs == nil {
		divs = []Divergence{}
	}
	return ReplaySnapshot{
		Order:         res.Order,
		Fills:         append([]ReplayFill(nil), fills...),
		FillQty:       res.FillQty,
		PositionDelta: res.PositionDelta,
		SpecHash:      res.SpecHash,
		SubmitOutcome: res.SubmitOutcome,
		EventsApplied: res.EventsApplied,
		EventsSkipped: res.EventsSkipped,
		Divergences:   append([]Divergence(nil), divs...),
	}
}

// ResultFromSnapshot rebuilds a ReplayResult from a durable snapshot.
func ResultFromSnapshot(snap ReplaySnapshot) ReplayResult {
	fills := snap.Fills
	if fills == nil {
		fills = []ReplayFill{}
	}
	divs := snap.Divergences
	if divs == nil {
		divs = []Divergence{}
	}
	return ReplayResult{
		Order:         snap.Order,
		Fills:         append([]ReplayFill(nil), fills...),
		FillQty:       snap.FillQty,
		PositionDelta: snap.PositionDelta,
		SpecHash:      snap.SpecHash,
		SubmitOutcome: snap.SubmitOutcome,
		EventsApplied: snap.EventsApplied,
		EventsSkipped: snap.EventsSkipped,
		Divergences:   append([]Divergence(nil), divs...),
	}
}

// EncodeSnapshot marshals a canonical snapshot and returns JSON + SHA-256 hex hash.
func EncodeSnapshot(snap ReplaySnapshot) (jsonText string, hash string, err error) {
	if snap.Fills == nil {
		snap.Fills = []ReplayFill{}
	}
	if snap.Divergences == nil {
		snap.Divergences = []Divergence{}
	}
	raw, err := json.Marshal(snap)
	if err != nil {
		return "", "", fmt.Errorf("encode replay snapshot: %w", err)
	}
	return string(raw), HashSnapshotBytes(raw), nil
}

// HashSnapshotBytes returns the SHA-256 hex digest of canonical snapshot bytes.
func HashSnapshotBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// DecodeSnapshot verifies hash (when provided) and decodes SnapshotJSON.
func DecodeSnapshot(jsonText, expectHash string) (ReplaySnapshot, error) {
	raw := []byte(strings.TrimSpace(jsonText))
	if len(raw) == 0 {
		return ReplaySnapshot{}, fmt.Errorf("%w: empty snapshot_json", ErrCheckpointCorrupted)
	}
	if expectHash != "" {
		got := HashSnapshotBytes(raw)
		if !strings.EqualFold(got, strings.TrimSpace(expectHash)) {
			return ReplaySnapshot{}, fmt.Errorf("%w: snapshot_hash mismatch", ErrCheckpointCorrupted)
		}
	}
	var snap ReplaySnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return ReplaySnapshot{}, fmt.Errorf("%w: %v", ErrCheckpointCorrupted, err)
	}
	if snap.Fills == nil {
		snap.Fills = []ReplayFill{}
	}
	if snap.Divergences == nil {
		snap.Divergences = []Divergence{}
	}
	return snap, nil
}

// AttachSnapshot encodes the replay result onto a checkpoint cursor.
func AttachSnapshot(cp Checkpoint, res ReplayResult) (Checkpoint, error) {
	jsonText, hash, err := EncodeSnapshot(SnapshotFromResult(res))
	if err != nil {
		return cp, err
	}
	cp.ReplayContractVersion = ReplayContractVersion
	cp.SnapshotJSON = jsonText
	cp.SnapshotHash = hash
	cp.EventsSeen = res.EventsApplied
	cp.FillCount = res.FillCount()
	cp.DivergenceCount = len(res.Divergences)
	cp.SubmitOutcom = res.SubmitOutcome
	if cp.Version <= 0 {
		cp.Version = 1
	}
	return cp, nil
}

// VerifyCheckpointIntegrity checks contract version and snapshot hash.
func VerifyCheckpointIntegrity(cp Checkpoint) error {
	if strings.TrimSpace(cp.Scope) == "" {
		return fmt.Errorf("%w: empty scope", ErrCheckpointCorrupted)
	}
	version := strings.TrimSpace(cp.ReplayContractVersion)
	if version == "" {
		return fmt.Errorf("%w: missing replay_contract_version", ErrCheckpointCorrupted)
	}
	if version != ReplayContractVersion {
		return fmt.Errorf("%w: stored %q current %q", ErrCheckpointContract, version, ReplayContractVersion)
	}
	if strings.TrimSpace(cp.SnapshotHash) == "" {
		return fmt.Errorf("%w: missing snapshot_hash", ErrCheckpointCorrupted)
	}
	_, err := DecodeSnapshot(cp.SnapshotJSON, cp.SnapshotHash)
	return err
}
