package authority

// DecisionConsumerRole who may interact with a Decision snapshot.
type DecisionConsumerRole string

const (
	RoleUICard                   DecisionConsumerRole = "UI_CARD"
	RoleAlert                    DecisionConsumerRole = "ALERT"
	RoleKline                    DecisionConsumerRole = "KLINE"
	RoleObservability            DecisionConsumerRole = "OBSERVABILITY"
	RoleShadowCompare            DecisionConsumerRole = "SHADOW_COMPARE"
	RoleTradePlanDraftFuture     DecisionConsumerRole = "TRADEPLAN_DRAFT_FUTURE"
	RoleTradePlanCandidateShadow DecisionConsumerRole = "TRADEPLAN_CANDIDATE_SHADOW"
	RoleExecutionForbidden       DecisionConsumerRole = "EXECUTION_FORBIDDEN"
)

// DecisionPermission capability bits for a consumer role.
type DecisionPermission struct {
	ReadDecision    bool `json:"readDecision"`
	WriteDecision   bool `json:"writeDecision"`
	ModifyAction    bool `json:"modifyAction"`
	CreateTrade     bool `json:"createTrade"`
	CreateCandidate bool `json:"createCandidate"` // Phase5-B: shadow TradePlanCandidate only
}

// AccessDecision result of Authorize.
type AccessDecision struct {
	Role       DecisionConsumerRole `json:"role"`
	Allowed    bool                 `json:"allowed"`
	Permission DecisionPermission   `json:"permission"`
	Reason     string               `json:"reason"`
}

// PermissionFor returns the frozen permission matrix entry for a role.
func PermissionFor(role DecisionConsumerRole) DecisionPermission {
	switch role {
	case RoleUICard, RoleAlert, RoleKline, RoleObservability, RoleShadowCompare:
		return DecisionPermission{
			ReadDecision: true,
		}
	case RoleTradePlanDraftFuture:
		return DecisionPermission{
			ReadDecision: true,
		}
	case RoleTradePlanCandidateShadow:
		// Phase5-B: read Decision snapshot + create Candidate shadow; never Execute.
		return DecisionPermission{
			ReadDecision:    true,
			CreateCandidate: true,
		}
	case RoleExecutionForbidden:
		return DecisionPermission{}
	default:
		return DecisionPermission{}
	}
}

// Authorize checks whether role may perform a read of Decision (primary gate).
// Write / ModifyAction / CreateTrade are always denied in Phase3-D+ matrix.
func Authorize(role DecisionConsumerRole) AccessDecision {
	perm := PermissionFor(role)
	if role == RoleExecutionForbidden || (!perm.ReadDecision && !perm.WriteDecision && !perm.CreateTrade) {
		if role == RoleExecutionForbidden || !perm.ReadDecision {
			return AccessDecision{
				Role:       role,
				Allowed:    false,
				Permission: perm,
				Reason:     "DENIED: execution/trade path must not use Decision as authority",
			}
		}
	}
	if perm.ReadDecision && !perm.WriteDecision && !perm.ModifyAction && !perm.CreateTrade {
		reason := "ALLOWED: read-only Decision consumer"
		if perm.CreateCandidate {
			reason = "ALLOWED: read Decision + create TradePlanCandidate shadow (no execute)"
		}
		return AccessDecision{
			Role:       role,
			Allowed:    true,
			Permission: perm,
			Reason:     reason,
		}
	}
	if !perm.ReadDecision {
		return AccessDecision{
			Role:       role,
			Allowed:    false,
			Permission: perm,
			Reason:     "DENIED: no Decision read permission",
		}
	}
	return AccessDecision{
		Role:       role,
		Allowed:    false,
		Permission: perm,
		Reason:     "DENIED: write/modify/createTrade not granted",
	}
}

// AuthorizeCreateTrade always false — Decision.Action is never execution authorization.
func AuthorizeCreateTrade(role DecisionConsumerRole) AccessDecision {
	perm := PermissionFor(role)
	return AccessDecision{
		Role:       role,
		Allowed:    false,
		Permission: perm,
		Reason:     "DENIED: Decision.Action is not execution authorization",
	}
}

// AuthorizeCreateCandidate true only for TRADEPLAN_CANDIDATE_SHADOW (shadow only).
func AuthorizeCreateCandidate(role DecisionConsumerRole) AccessDecision {
	perm := PermissionFor(role)
	if perm.CreateCandidate && !perm.CreateTrade {
		return AccessDecision{
			Role:       role,
			Allowed:    true,
			Permission: perm,
			Reason:     "ALLOWED: create TradePlanCandidate shadow",
		}
	}
	return AccessDecision{
		Role:       role,
		Allowed:    false,
		Permission: perm,
		Reason:     "DENIED: cannot create TradePlanCandidate",
	}
}

// AllRoles frozen consumer role list (stable order for golden).
func AllRoles() []DecisionConsumerRole {
	return []DecisionConsumerRole{
		RoleUICard,
		RoleAlert,
		RoleKline,
		RoleObservability,
		RoleShadowCompare,
		RoleTradePlanDraftFuture,
		RoleTradePlanCandidateShadow,
		RoleExecutionForbidden,
	}
}

// AuthorityMatrix returns role → permission for golden / docs.
func AuthorityMatrix() map[DecisionConsumerRole]DecisionPermission {
	m := make(map[DecisionConsumerRole]DecisionPermission, len(AllRoles()))
	for _, r := range AllRoles() {
		m[r] = PermissionFor(r)
	}
	return m
}
