package datatables

import "errors"

// Common errors returned by the datatables package. All are reachable
// through errors.Is, including indirectly via *ValidationError.Unwrap.
var (
	// ErrInvalidColumnName is returned when a column name contains invalid characters
	ErrInvalidColumnName = errors.New("invalid column name: must contain only alphanumeric characters, underscores, and dots")

	// ErrInvalidData is returned when the destination passed to OfReturn is
	// not a pointer to a slice of structs.
	ErrInvalidData = errors.New("invalid data: expected a pointer to a slice of structs")

	// ErrInvalidOrderColumn is returned by OfReturn when Options.StrictOrder is
	// true and the request's order key is not present in the orderable map.
	ErrInvalidOrderColumn = errors.New("order column not found in orderable map")

	// ErrInvalidOrderDirection is returned when an order direction is not
	// "asc" or "desc".
	ErrInvalidOrderDirection = errors.New("order direction must be \"asc\" or \"desc\"")
)

// ValidationError represents a validation error with additional context.
// It wraps one of the sentinel errors above, so callers can use
// errors.Is(err, datatables.ErrInvalidColumnName) instead of parsing the
// message string.
type ValidationError struct {
	Field   string
	Message string
	Err     error
}

func (e *ValidationError) Error() string {
	return "validation error on field '" + e.Field + "': " + e.Message
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}
