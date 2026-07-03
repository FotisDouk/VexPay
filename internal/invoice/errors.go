package invoice

import "errors"

// ErrNotFound is returned when an invoice does not exist.
var ErrNotFound = errors.New("invoice not found")
