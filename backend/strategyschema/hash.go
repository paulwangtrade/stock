package strategyschema

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// contentFingerprint is the canonical payload hashed into revision_hash / params_hash.
type contentFingerprint struct {
	Universe       UniverseSpec   `json:"universe"`
	Signals        SignalsSpec    `json:"signals"`
	Filters        FiltersSpec    `json:"filters"`
	Ranking        RankingSpec    `json:"ranking"`
	RiskProfileRef RiskProfileRef `json:"risk_profile_ref"`
	Knobs          map[string]any `json:"knobs"`
}

// ComputeParamsHash hashes normalized knobs (stable pin for parameters).
func ComputeParamsHash(knobs map[string]any) string {
	if knobs == nil {
		knobs = map[string]any{}
	}
	return hashCanonical(knobs)
}

// ComputeRevisionHash hashes full rule content (universe…knobs) for freeze / pin.
func ComputeRevisionHash(r Revision) string {
	fp := contentFingerprint{
		Universe:       r.Universe,
		Signals:        r.Signals,
		Filters:        r.Filters,
		Ranking:        r.Ranking,
		RiskProfileRef: r.RiskProfileRef,
		Knobs:          r.Parameters.Knobs,
	}
	if fp.Knobs == nil {
		fp.Knobs = map[string]any{}
	}
	return hashCanonical(fp)
}

// ApplyContentHashes sets Parameters.ParamsHash and RevisionHash from content.
func ApplyContentHashes(r *Revision) {
	if r == nil {
		return
	}
	r.Parameters.ParamsHash = ComputeParamsHash(r.Parameters.Knobs)
	r.RevisionHash = ComputeRevisionHash(*r)
}

// VerifyContentHashes ensures stored hashes match recomputed content hashes.
func VerifyContentHashes(r Revision) error {
	wantParams := ComputeParamsHash(r.Parameters.Knobs)
	if r.Parameters.ParamsHash == "" {
		return errHashRequired("params_hash")
	}
	if r.Parameters.ParamsHash != wantParams {
		return errHashMismatch("params_hash")
	}
	wantRev := ComputeRevisionHash(r)
	if r.RevisionHash == "" {
		return errHashRequired("revision_hash")
	}
	if r.RevisionHash != wantRev {
		return errHashMismatch("revision_hash")
	}
	return nil
}

func hashCanonical(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		sum := sha256.Sum256([]byte("marshal_error"))
		return hex.EncodeToString(sum[:])
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
