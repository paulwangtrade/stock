package user

import "time"

// UserProfile holds non-auth preferences and settings for a User.
// It does not store trading positions, plans, passwords, or billing state.
type UserProfile struct {
	UserID       string            `json:"user_id"`
	DisplayName  string            `json:"display_name,omitempty"`
	Preferences  map[string]string `json:"preferences,omitempty"`
	Settings     map[string]string `json:"settings,omitempty"`
	Email        string            `json:"email,omitempty"` // optional contact; not required for local
	AvatarURL    string            `json:"avatar_url,omitempty"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// Pref returns a preference value or empty string.
func (p *UserProfile) Pref(key string) string {
	if p == nil || p.Preferences == nil {
		return ""
	}
	return p.Preferences[key]
}

// Setting returns a settings value or empty string.
func (p *UserProfile) Setting(key string) string {
	if p == nil || p.Settings == nil {
		return ""
	}
	return p.Settings[key]
}
