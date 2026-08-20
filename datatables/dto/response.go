package dto

// ========================
// DataTables Response Format
// ========================

// Datatables represents the standard response structure used by the
// jQuery DataTables plugin. It includes pagination metadata and data rows.
// datatables.JSON writes this at the root of the HTTP response body, since
// that is the shape a serverSide DataTables client requires.
type Datatables struct {
	Draw            int64       `json:"draw"`            // Draw counter to synchronize client-side and server-side data
	RecordsTotal    int64       `json:"recordsTotal"`    // Total number of records available
	RecordsFiltered int64       `json:"recordsFiltered"` // Number of records after applying filters
	Data            interface{} `json:"data"`            // Actual data rows to be displayed in the DataTable
}

// ========================
// Error Response Format
// ========================

// ErrorResponse is the shape DataTables expects on a failed request: a
// root-level "error" string. datatables.JSONError writes this.
type ErrorResponse struct {
	Error string `json:"error"`
}
