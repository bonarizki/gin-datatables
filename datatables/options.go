package datatables

import (
	"maps"
	"slices"

	"github.com/bonarizki/gin-datatables/v2/datatables/dto"
)

// Options provides customization similar to Yajra DataTables.
// It allows adding, editing, and removing columns dynamically,
// as well as controlling the row index column, default ordering, and pagination limits.
type Options struct {
	// IndexColumn is the name of the index column to be added (e.g., "DT_RowIndex")
	IndexColumn string

	// ResetIndex determines whether to reset index numbering to start from 1
	ResetIndex bool

	// DefaultOrders specifies the ordering to apply when the request does not
	// provide a valid order. Populated via WithDefaultOrder; each entry's
	// Column is a literal database column, validated the same way as
	// searchable/orderable columns.
	DefaultOrders []dto.Order

	// StrictOrder controls what happens when the request's order key is not
	// found in the orderable map. When false (default), the mismatch is
	// logged and the query falls back to DefaultOrders. When true, OfReturn
	// returns ErrInvalidOrderColumn instead, surfacing frontend/backend
	// wiring mistakes immediately instead of leaving them unnoticed.
	StrictOrder bool

	// DefaultPageSize is used when the request omits "length" or sends a
	// value that cannot be parsed as a positive page size. Defaults to 10.
	DefaultPageSize int

	// MaxPageSize caps the "length" a client may request. Defaults to 500.
	MaxPageSize int

	// UnlimitedPageSize allows a client to request length=-1 for "all rows".
	// Defaults to false: an unrecognized or unbounded length silently falls
	// back to DefaultPageSize instead of dumping the entire table.
	UnlimitedPageSize bool

	// AddColumns contains custom columns to add, computed from existing row data
	AddColumns map[string]func(row map[string]interface{}) interface{}

	// EditColumns contains custom transformations for existing columns
	EditColumns map[string]func(value interface{}, row map[string]interface{}) interface{}

	// RemoveColumns is a list of columns to be removed from the final output
	RemoveColumns []string
}

// NewOptions returns a new Options instance with sensible defaults.
// By default, it creates an index column named "DT_RowIndex" without resetting
// indices, a page size of 10 capped at 500, and no unlimited-page escape hatch.
// normalized fills in default pagination bounds when DefaultPageSize or
// MaxPageSize are zero or inconsistent, so a zero-value Options cannot
// disable LIMIT and return every row.
func (o Options) normalized() Options {
	def := o.DefaultPageSize
	max := o.MaxPageSize
	if def <= 0 {
		def = 10
	}
	if max <= 0 {
		max = 500
	}
	if def > max {
		def = max
	}
	o.DefaultPageSize = def
	o.MaxPageSize = max
	return o
}

func NewOptions() Options {
	return Options{
		IndexColumn:       "DT_RowIndex",
		ResetIndex:        false,
		DefaultOrders:     nil,
		StrictOrder:       false,
		DefaultPageSize:   10,
		MaxPageSize:       500,
		UnlimitedPageSize: false,
		AddColumns:        make(map[string]func(row map[string]interface{}) interface{}),
		EditColumns:       make(map[string]func(value interface{}, row map[string]interface{}) interface{}),
		RemoveColumns:     []string{},
	}
}

// WithIndex configures the name and behavior of the index column.
//
// Parameters:
//   - col: The column name for the index (e.g., "DT_RowIndex", "row_number")
//   - reset: If true, index starts from 1 on each page; if false, continues from previous pages
//
// Example:
//
//	opts.WithIndex("row_num", true)
func (o Options) WithIndex(col string, reset bool) Options {
	o.IndexColumn = col
	o.ResetIndex = reset
	return o
}

// WithDefaultOrder appends a column/direction pair to use when no valid
// order is provided in the request. Call it multiple times for a
// multi-column default order; clauses are applied in call order.
//
// Unlike the v1 API, this does not accept a free-form SQL fragment: col must
// pass the same identifier validation as searchable/orderable columns, and
// dir must be "asc" or "desc" (case-insensitive). OfReturn returns an error
// if either check fails, instead of concatenating unvalidated SQL.
//
// Example:
//
//	opts.WithDefaultOrder("created_at", "desc")
func (o Options) WithDefaultOrder(col string, dir string) Options {
	o.DefaultOrders = append(slices.Clip(o.DefaultOrders), dto.Order{Column: col, Dir: dir})
	return o
}

// WithStrictOrder makes OfReturn return ErrInvalidOrderColumn when the
// request's order key is not present in the orderable map, instead of
// silently falling back to DefaultOrders. Recommended during development so
// frontend/backend wiring mistakes fail loudly.
func (o Options) WithStrictOrder(strict bool) Options {
	o.StrictOrder = strict
	return o
}

// WithPageSize sets the default and maximum page sizes. def is used when the
// request omits "length" or sends an unparseable/non-positive value; max
// caps whatever the client requests.
//
// Example:
//
//	opts.WithPageSize(25, 200)
func (o Options) WithPageSize(def int, max int) Options {
	o.DefaultPageSize = def
	o.MaxPageSize = max
	return o
}

// WithUnlimitedPageSize allows the client to request length=-1 to fetch all
// matching rows, bypassing MaxPageSize. Off by default, since any client can
// otherwise force a full table dump with a single query parameter.
func (o Options) WithUnlimitedPageSize(allow bool) Options {
	o.UnlimitedPageSize = allow
	return o
}

// Add registers a new column to be added dynamically using a callback function.
// The callback receives the row data built so far (including any earlier
// AddColumns/index columns) and should return the value for the new column.
//
// Parameters:
//   - col: The name of the new column
//   - fn: A function that computes the column value from row data
//
// Example:
//
//	opts.Add("full_name", func(row map[string]interface{}) interface{} {
//	    return row["first_name"].(string) + " " + row["last_name"].(string)
//	})
func (o Options) Add(col string, fn func(row map[string]interface{}) interface{}) Options {
	m := maps.Clone(o.AddColumns)
	if m == nil {
		m = make(map[string]func(row map[string]interface{}) interface{})
	}
	m[col] = fn
	o.AddColumns = m
	return o
}

// Edit registers a callback function to modify an existing column's value.
// The callback receives both the current value and the entire row data.
//
// Parameters:
//   - col: The name of the column to edit
//   - fn: A function that transforms the column value
//
// Example:
//
//	opts.Edit("email", func(value interface{}, row map[string]interface{}) interface{} {
//	    return strings.ToLower(value.(string))
//	})
func (o Options) Edit(col string, fn func(value interface{}, row map[string]interface{}) interface{}) Options {
	m := maps.Clone(o.EditColumns)
	if m == nil {
		m = make(map[string]func(value interface{}, row map[string]interface{}) interface{})
	}
	m[col] = fn
	o.EditColumns = m
	return o
}

// Remove specifies one or more columns to be removed from the final output.
// This is useful for hiding sensitive data or reducing payload size.
//
// Parameters:
//   - cols: One or more column names to remove
//
// Example:
//
//	opts.Remove("password", "internal_id", "deleted_at")
func (o Options) Remove(cols ...string) Options {
	o.RemoveColumns = append(slices.Clip(o.RemoveColumns), cols...)
	return o
}
