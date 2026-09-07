package projection

import "errors"

var (
	// ErrInvalidStockCode indicates an empty or unnormalizable stock code.
	ErrInvalidStockCode = errors.New("projection: invalid stock code")
	// ErrNotFound indicates no projection data for the requested stock.
	ErrNotFound = errors.New("projection: not found")
)
