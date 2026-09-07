package outcome

import "errors"

var (
	ErrInvalidStockCode = errors.New("outcome: invalid stock code")
	ErrNotFound         = errors.New("outcome: not found")
)
