// Package datatables provides a reusable DataTables-style server-side processor for Gin + GORM.
//
// Inspired by Yajra DataTables (Laravel), this package offers a clean and powerful API
// for handling server-side DataTables operations including pagination, searching, ordering,
// and dynamic column manipulation.
//
// Key Features:
//   - Automatic pagination, ordering, and search handling
//   - Simple API similar to Yajra DataTables
//   - Fully customizable columns (add, edit, remove)
//   - Type-safe implementation with Go generics
//   - SQL injection prevention with column name validation
//   - Configurable default ordering
//
// Basic Usage:
//
//	var users []User
//	result, err := datatables.OfReturn(
//	    c,
//	    db.Model(&User{}),
//	    &users,
//	    []string{"name", "email"},
//	    map[string]string{
//	        "name":       "name",
//	        "email":      "email",
//	        "created_at": "created_at",
//	    },
//	    datatables.NewOptions().WithDefaultOrder("created_at", "desc"),
//	)
//	if err != nil {
//	    log.Printf("datatables: %v", err)
//	    datatables.JSONError(c, http.StatusInternalServerError, "failed to load data")
//	    return
//	}
//	datatables.JSON(c, result)
//
// Advanced Usage with Column Customization:
//
//	opts := datatables.NewOptions().
//	    WithIndex("row_num", true).
//	    WithDefaultOrder("id", "desc").
//	    Add("label", func(row map[string]any) any {
//	        return fmt.Sprintf("%v <%v>", row["name"], row["email"])
//	    }).
//	    Edit("email", func(value any, row map[string]any) any {
//	        return strings.ToLower(fmt.Sprint(value))
//	    }).
//	    Remove("password", "internal_id")
//
// Security:
//   - All column names are validated against a whitelist pattern
//   - Search queries use parameterized statements
//   - Page size is capped by Options.MaxPageSize (default 500); a missing,
//     zero, negative or unparseable length falls back to
//     Options.DefaultPageSize (default 10)
//
// For more information, visit: https://github.com/bonarizki/gin-datatables
//
// Note: starting with v2, the module path includes the major version suffix:
//
//	import "github.com/bonarizki/gin-datatables/v2/datatables"
package datatables
