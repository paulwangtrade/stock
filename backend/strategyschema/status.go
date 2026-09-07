package strategyschema

// ValidRevisionStatuses for display / future transitions.
var ValidRevisionStatuses = map[string]bool{
	RevisionStatusDraft:     true,
	RevisionStatusReviewing: true,
	RevisionStatusActive:    true,
	RevisionStatusRetired:   true,
	RevisionStatusDiscarded: true,
}

// ValidDefinitionStatuses for definition shell.
var ValidDefinitionStatuses = map[string]bool{
	DefinitionStatusActive:    true,
	DefinitionStatusArchived:  true,
	DefinitionStatusDraftOnly: true,
}

// CanTransitionRevision reports whether from→to is allowed (B3-B will enforce writes).
// B3-A exposes this for documentation/tests only.
func CanTransitionRevision(from, to string) bool {
	if from == to {
		return true
	}
	switch from {
	case RevisionStatusDraft:
		return to == RevisionStatusReviewing || to == RevisionStatusDiscarded
	case RevisionStatusReviewing:
		return to == RevisionStatusActive || to == RevisionStatusDraft || to == RevisionStatusDiscarded
	case RevisionStatusActive:
		return to == RevisionStatusRetired
	case RevisionStatusRetired, RevisionStatusDiscarded:
		return false
	default:
		return false
	}
}
