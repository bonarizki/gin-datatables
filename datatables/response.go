package datatables

import (
	"net/http"

	"github.com/bonarizki-dat/Datatables-Gin/v2/datatables/dto"
	"github.com/gin-gonic/gin"
)

// JSON sends the DataTables result as the HTTP response body with draw,
// recordsTotal, recordsFiltered, and data at the JSON root — the shape a
// stock jQuery DataTables client expects from a serverSide ajax source.
//
// Example:
//
//	result, err := datatables.OfReturn(c, query, &users, searchable, orderable, opts)
//	if err != nil {
//	    datatables.JSONError(c, http.StatusInternalServerError, "failed to load users")
//	    return
//	}
//	datatables.JSON(c, result)
func JSON(c *gin.Context, res dto.Datatables) {
	c.JSON(http.StatusOK, res)
}

// JSONError sends a DataTables-compatible error response: a root-level
// "error" string.
//
// message should be a safe, user-facing description. Do not pass
// err.Error() directly — database and schema details should not reach the
// client; log the underlying error server-side instead.
func JSONError(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, dto.ErrorResponse{Error: message})
}
