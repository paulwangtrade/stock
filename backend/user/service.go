package user

import "context"

// Service is the identity application port for Shell / commercial composition.
// Implementations must not call Trading Engine APIs (TradePlan / Execution / Broker).
type Service interface {
	// EnsureLocalUser returns the default local user, creating it if absent.
	EnsureLocalUser(ctx context.Context) (*User, error)

	// GetUser loads by id.
	GetUser(ctx context.Context, userID string) (*User, error)

	// GetProfile loads the profile for userID (creates empty profile if user exists).
	GetProfile(ctx context.Context, userID string) (*UserProfile, error)

	// UpdateProfile patches mutable profile fields for an existing user.
	UpdateProfile(ctx context.Context, userID string, patch ProfilePatch) (*UserProfile, error)
}

// ProfilePatch is a partial update for UserProfile.
// Nil pointer / nil map fields mean "leave unchanged".
type ProfilePatch struct {
	DisplayName *string
	Email       *string
	AvatarURL   *string
	Preferences map[string]string // merge into profile.Preferences when non-nil
	Settings    map[string]string // merge into profile.Settings when non-nil
}

// Store is the persistence port (memory in C5; no cloud database).
type Store interface {
	GetUser(ctx context.Context, userID string) (*User, error)
	SaveUser(ctx context.Context, u *User) error
	GetProfile(ctx context.Context, userID string) (*UserProfile, error)
	SaveProfile(ctx context.Context, p *UserProfile) error
}
