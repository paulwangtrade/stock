package entitlement

import "errors"

// ErrNotFound is returned when no entitlement row exists for user×feature.
var ErrNotFound = errors.New("entitlement: not found")
