package user

import "errors"

// ErrNotFound is returned when a user or profile is missing from the Store.
var ErrNotFound = errors.New("user: not found")
