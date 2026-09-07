package strategyintent

// ValidStatuses for Intent lifecycle.
var ValidStatuses = map[string]bool{
	StatusDraft:     true,
	StatusReviewing: true,
	StatusApproved:  true,
	StatusExpired:   true,
	StatusDiscarded: true,
}

// ValidIntentTypes for intent_type field.
var ValidIntentTypes = map[string]bool{
	IntentTypeManual:        true,
	IntentTypeRuleSuggested: true,
	IntentTypeAIDraft:       true,
	IntentTypeHybrid:        true,
}

// CanTransition reports whether from→to is allowed (B1 write waves will enforce).
func CanTransition(from, to string) bool {
	if from == to {
		return true
	}
	switch from {
	case StatusDraft:
		return to == StatusReviewing || to == StatusDiscarded
	case StatusReviewing:
		return to == StatusApproved || to == StatusDraft || to == StatusDiscarded
	case StatusApproved:
		return to == StatusExpired || to == StatusDiscarded
	case StatusExpired, StatusDiscarded:
		return false
	default:
		return false
	}
}
