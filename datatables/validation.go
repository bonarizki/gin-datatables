package datatables

import (
	"regexp"
	"strings"

	"github.com/bonarizki/gin-datatables/v2/datatables/dto"
)

// columnNamePattern defines the allowed pattern for column names
// Allows: alphanumeric characters, underscores, and dots (for table.column notation)
var columnNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_.]+$`)

// isValidColumnName checks if a column name is safe to use in SQL queries.
// This helps prevent SQL injection attacks by validating column names
// before they are used in dynamic queries.
//
// Valid column names must contain only:
//   - Alphanumeric characters (a-z, A-Z, 0-9)
//   - Underscores (_)
//   - Dots (.) for table.column notation
//
// Returns true if the column name is valid, false otherwise.
func isValidColumnName(name string) bool {
	if name == "" {
		return false
	}
	return columnNamePattern.MatchString(name)
}

// validateSearchableColumns validates all searchable column names
// to ensure they are safe for SQL queries.
//
// Returns an error if any column name is invalid.
func validateSearchableColumns(columns []string) error {
	for _, col := range columns {
		if !isValidColumnName(col) {
			return &ValidationError{
				Field:   col,
				Message: "searchable column name contains invalid characters",
				Err:     ErrInvalidColumnName,
			}
		}
	}
	return nil
}

// validateOrderableColumns validates all orderable column mappings
// to ensure both keys and values are safe for SQL queries.
//
// Returns an error if any column name is invalid.
func validateOrderableColumns(columns map[string]string) error {
	for key, val := range columns {
		if !isValidColumnName(key) {
			return &ValidationError{
				Field:   key,
				Message: "orderable column key contains invalid characters",
				Err:     ErrInvalidColumnName,
			}
		}
		if !isValidColumnName(val) {
			return &ValidationError{
				Field:   val,
				Message: "orderable column value contains invalid characters",
				Err:     ErrInvalidColumnName,
			}
		}
	}
	return nil
}

// isValidOrderDirection reports whether dir is "asc" or "desc"
// (case-insensitive).
func isValidOrderDirection(dir string) bool {
	return strings.EqualFold(dir, "asc") || strings.EqualFold(dir, "desc")
}

// validateOptionColumns validates the column names carried by Options
// itself: TieBreaker (unless empty or "-") and every ColumnSearch entry.
func validateOptionColumns(opts Options) error {
	if opts.TieBreaker != "" && opts.TieBreaker != "-" && !isValidColumnName(opts.TieBreaker) {
		return &ValidationError{
			Field:   opts.TieBreaker,
			Message: "tie-breaker column contains invalid characters",
			Err:     ErrInvalidColumnName,
		}
	}
	for key, val := range opts.ColumnSearch {
		if !isValidColumnName(key) || !isValidColumnName(val) {
			return &ValidationError{
				Field:   key,
				Message: "column search mapping contains invalid characters",
				Err:     ErrInvalidColumnName,
			}
		}
	}
	return nil
}

// validateDefaultOrders validates every Options.DefaultOrders entry so that
// WithDefaultOrder cannot be used to pass unvalidated SQL into query.Order(),
// unlike the v1 free-form string.
//
// Returns an error if any column name is invalid or any direction is not
// "asc"/"desc".
func validateDefaultOrders(orders []dto.Order) error {
	for _, o := range orders {
		if !isValidColumnName(o.Column) {
			return &ValidationError{
				Field:   o.Column,
				Message: "default order column contains invalid characters",
				Err:     ErrInvalidColumnName,
			}
		}
		if !isValidOrderDirection(o.Dir) {
			return &ValidationError{
				Field:   o.Dir,
				Message: "default order direction must be \"asc\" or \"desc\"",
				Err:     ErrInvalidOrderDirection,
			}
		}
	}
	return nil
}
