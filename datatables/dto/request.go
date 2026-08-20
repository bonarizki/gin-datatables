package dto

// ========================
// Params → standard DataTables parameters
// ========================

// Order represents a single column/direction pair from a DataTables order[i]
// request, or from Options.WithDefaultOrder. Column is either a frontend
// orderable key (for request-driven orders) or a literal database column
// (for default orders), depending on where it originated.
type Order struct {
	Column string
	Dir    string
}

// ColumnSearch represents a single columns[i][search][value] entry from a
// DataTables request. Column is the frontend key (columns[i][data]), which
// is resolved against the same orderable map used for sorting.
type ColumnSearch struct {
	Column string
	Value  string
}

// Params holds the normalized DataTables request parameters. Length and Start
// are always bounded (see Options.DefaultPageSize/MaxPageSize/UnlimitedPageSize),
// so callers never need to guard against unbounded pagination themselves.
type Params struct {
	Draw           int64
	Start          int
	Length         int
	Search         string
	Orders         []Order
	ColumnSearches []ColumnSearch
}
