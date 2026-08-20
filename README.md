# 🧮 Datatables-Gin

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Tests](https://img.shields.io/badge/tests-passing-brightgreen.svg)](datatables/)

A **DataTables-style server-side processor** for **Gin + GORM** in Go.
Inspired by [Yajra DataTables](https://github.com/yajra/laravel-datatables) (Laravel), designed for clean, composable data handling in APIs.

> **v2.0.0 is a breaking change from v1.x.** It fixes a cross-tenant data leak in
> global search, spec-compliant ordering, unbounded pagination, and a response
> shape the stock DataTables client couldn't read. See
> [MIGRATING.md](MIGRATING.md) before upgrading, and
> [CHANGELOG.md](CHANGELOG.md) for the full list.

---

## ✨ Features

- 🔁 **Automatic Processing**: Handles pagination, ordering, and search, bounded by default
- 🛡️ **Search isolation**: Global search is grouped so it cannot escape a caller's own `WHERE` clause, and LIKE wildcards are escaped
- 🧱 **Simple API**: Similar to Yajra DataTables, easy to learn and use
- ⚙️ **Highly Customizable**: Add, edit, or remove columns dynamically
- 📊 **Gin + GORM**: Works seamlessly with Gin framework and GORM ORM
- 💡 **Type-Safe**: Generic implementation (`OfReturn[T any]`) for compile-time safety
- 🧪 **Tested**: 92.8% coverage as of v2.0.0, including SQL-level tests for the search and ordering logic
- 📦 **Column validation**: Searchable/orderable/default-order columns are checked against an identifier whitelist before reaching SQL

---

## 📦 Installation

```bash
go get github.com/bonarizki-dat/Datatables-Gin/v2
```

**Requirements:**
- Go 1.23 or higher
- Gin Web Framework
- GORM v2

---

## 🚀 Quick Start

### Basic Example

```go
package main

import (
    "log"
    "net/http"
    "time"

    "github.com/bonarizki-dat/Datatables-Gin/v2/datatables"
    "github.com/gin-gonic/gin"
    "gorm.io/gorm"
)

type User struct {
    ID        uint   `json:"id"`
    Name      string `json:"name"`
    Email     string `json:"email"`
    CreatedAt time.Time `json:"created_at"`
}

func GetUsers(c *gin.Context, db *gorm.DB) {
    var users []User

    // Define searchable columns
    searchable := []string{"name", "email"}

    // Map frontend column names to database columns
    orderable := map[string]string{
        "name":    "name",
        "email":   "email",
        "created": "created_at",
    }

    // Configure options
    opts := datatables.NewOptions().
        WithDefaultOrder("created_at", "desc")

    // Process DataTables request
    result, err := datatables.OfReturn(
        c,
        db.Model(&User{}),
        &users,
        searchable,
        orderable,
        opts,
    )

    if err != nil {
        // Log the real error server-side; don't send err.Error() to the
        // client, since it can contain database/schema details.
        log.Printf("GetUsers: %v", err)
        datatables.JSONError(c, http.StatusInternalServerError, "failed to load users")
        return
    }

    datatables.JSON(c, result)
}
```

### Frontend Integration

`datatables.JSON()` writes `draw`, `recordsTotal`, `recordsFiltered`, and `data`
at the root of the response body, so a stock `serverSide: true` DataTables
config works with no `ajax.dataSrc` override:

```html
<table id="users-table">
    <thead>
        <tr>
            <th data-data="DT_RowIndex">No</th>
            <th data-data="name">Name</th>
            <th data-data="email">Email</th>
            <th data-data="created">Created</th>
        </tr>
    </thead>
</table>

<script>
$('#users-table').DataTable({
    processing: true,
    serverSide: true,
    ajax: '/api/users',
    columns: [
        { data: 'DT_RowIndex', orderable: false, searchable: false },
        { data: 'name' },
        { data: 'email' },
        { data: 'created', name: 'created' }
    ]
});
</script>
```

---

## 🎯 Advanced Usage

### Custom Columns

Add computed columns dynamically:

```go
opts := datatables.NewOptions().
    Add("full_name", func(row map[string]interface{}) interface{} {
        return fmt.Sprintf("%s %s",
            row["first_name"],
            row["last_name"])
    }).
    Add("status_badge", func(row map[string]interface{}) interface{} {
        status := row["status"].(string)
        if status == "active" {
            return `<span class="badge badge-success">Active</span>`
        }
        return `<span class="badge badge-danger">Inactive</span>`
    })
```

### Transform Existing Columns

Edit column values before sending to frontend:

```go
opts := datatables.NewOptions().
    Edit("email", func(value interface{}, row map[string]interface{}) interface{} {
        // Lowercase all emails
        return strings.ToLower(value.(string))
    }).
    Edit("created_at", func(value interface{}, row map[string]interface{}) interface{} {
        // Format timestamps
        t := value.(time.Time)
        return t.Format("2006-01-02 15:04:05")
    })
```

### Remove Sensitive Data

Hide columns from the response:

```go
opts := datatables.NewOptions().
    Remove("password", "internal_id", "deleted_at")
```

### Custom Index Column

Configure row numbering:

```go
// Continuous numbering across pages
opts := datatables.NewOptions().
    WithIndex("row_number", false)

// Reset numbering on each page
opts := datatables.NewOptions().
    WithIndex("row_number", true)
```

### Complete Example

```go
func GetProducts(c *gin.Context, db *gorm.DB) {
    var products []Product

    searchable := []string{"name", "category", "sku"}

    orderable := map[string]string{
        "name":     "name",
        "price":    "price",
        "stock":    "stock",
        "category": "category",
        "created":  "created_at",
    }

    opts := datatables.NewOptions().
        WithIndex("row_num", true).
        WithDefaultOrder("created_at", "desc").
        Add("price_formatted", func(row map[string]interface{}) interface{} {
            price := row["price"].(float64)
            return fmt.Sprintf("$%.2f", price)
        }).
        Add("stock_status", func(row map[string]interface{}) interface{} {
            stock := row["stock"].(int)
            if stock > 10 {
                return "In Stock"
            } else if stock > 0 {
                return "Low Stock"
            }
            return "Out of Stock"
        }).
        Edit("category", func(value interface{}, row map[string]interface{}) interface{} {
            return strings.ToUpper(value.(string))
        }).
        Remove("internal_notes", "cost_price")

    result, err := datatables.OfReturn(
        c,
        db.Model(&Product{}),
        &products,
        searchable,
        orderable,
        opts,
    )

    if err != nil {
        log.Printf("GetProducts: %v", err)
        datatables.JSONError(c, http.StatusInternalServerError, "failed to load products")
        return
    }

    datatables.JSON(c, result)
}
```

---

## 🔒 Security Features

### SQL Injection Prevention

Column names passed as `searchable`, `orderable`, or via `WithDefaultOrder` are
validated against a whitelist pattern before being used in queries:

```go
// ✅ Valid column names
"user_id"
"users.name"
"created_at"

// ❌ Blocked - prevents SQL injection
"id; DROP TABLE users--"
"name' OR '1'='1"
"id--"
```

`WithDefaultOrder(col, dir string)` validates both arguments the same way —
unlike v1, it no longer accepts a free-form SQL fragment.

### Validation Errors

The package returns errors wrapping one of the sentinel values in
`errors.go` (`ErrInvalidColumnName`, `ErrInvalidOrderColumn`,
`ErrInvalidOrderDirection`, `ErrInvalidData`), so callers can branch on the
failure kind with `errors.Is` instead of parsing the message string:

```go
result, err := datatables.OfReturn(c, query, &users, searchable, orderable, opts)
if err != nil {
    log.Printf("DataTables error: %v", err) // log the real error server-side
    if errors.Is(err, datatables.ErrInvalidColumnName) {
        datatables.JSONError(c, http.StatusBadRequest, "invalid column configuration")
        return
    }
    datatables.JSONError(c, http.StatusInternalServerError, "failed to load data")
    return
}
```

### Search isolation and safe LIKE patterns

Global search is grouped into its own parenthesized clause, so it cannot
escape a `WHERE` the caller already applied (e.g. tenant scoping), and `%`,
`_`, and `\` are escaped in the search value before it reaches the query:

```go
// db.Model(&User{}).Where("tenant_id = ?", tenantID)
// search value: "x"
// generates:
// WHERE tenant_id = ? AND (LOWER(name) LIKE LOWER(?) ESCAPE '\' OR LOWER(code) LIKE LOWER(?) ESCAPE '\')
// vars: [tenantID, "%x%", "%x%"]
```

### Bounded pagination

`length` is always clamped between 1 and `Options.MaxPageSize` (500 by
default) unless the request omits it or sends a value that can't be parsed,
in which case `Options.DefaultPageSize` (10 by default) is used. A client
cannot force an unbounded result set with `length=0` or `length=-1` unless
the server has explicitly opted in via `WithUnlimitedPageSize(true)`.

---

## 📚 API Reference

### Core Functions

#### `OfReturn[T any]()`

Main function for processing DataTables requests.

```go
func OfReturn[T any](
    c *gin.Context,
    query *gorm.DB,
    dest *[]T,
    searchable []string,
    orderable map[string]string,
    opts Options,
) (dto.Datatables, error)
```

**Parameters:**
- `c` - Gin context containing request parameters
- `query` - GORM query builder (can include WHERE, JOIN, etc.)
- `dest` - Pointer to slice where results will be stored
- `searchable` - Columns that support global search
- `orderable` - Map of frontend column names to database columns
- `opts` - Configuration options

**Returns:**
- `dto.Datatables` - `{Draw, RecordsTotal, RecordsFiltered, Data}`, written at
  the JSON root by `JSON()`
- `error` - Validation or database errors, wrapping a sentinel from `errors.go`

#### `JSON()`

Writes `dto.Datatables` at the root of the HTTP response body — the shape a
stock jQuery DataTables client expects from `serverSide: true`.

```go
datatables.JSON(c, result)
```

#### `JSONError()`

Sends a `{"error": "..."}` response. Pass a safe, user-facing message —
never `err.Error()` directly, since it can leak database/schema details.

```go
datatables.JSONError(c, http.StatusInternalServerError, "failed to load users")
```

### Options Builder

#### `NewOptions()`

Creates a new Options instance with defaults: `DefaultPageSize` 10,
`MaxPageSize` 500, `UnlimitedPageSize` false, `StrictOrder` false, no
`DefaultOrders`.

```go
opts := datatables.NewOptions()
```

#### `WithIndex(col string, reset bool)`

Configures the index column.

```go
opts.WithIndex("DT_RowIndex", false)
```

#### `WithDefaultOrder(col string, dir string)`

Appends a column/direction pair applied when the request has no valid
order. Call it multiple times for a multi-column default order. Unlike v1,
`col` and `dir` are validated the same way as searchable/orderable columns
instead of accepting a free-form SQL fragment.

```go
opts.WithDefaultOrder("created_at", "desc")
```

#### `WithStrictOrder(strict bool)`

When `true`, `OfReturn` returns `ErrInvalidOrderColumn` if the request's
order key is not in the orderable map, instead of logging a warning and
falling back to `DefaultOrders`. Recommended in development so
frontend/backend wiring mistakes fail loudly.

```go
opts.WithStrictOrder(true)
```

#### `WithPageSize(def int, max int)`

Sets the default and maximum page sizes.

```go
opts.WithPageSize(25, 200)
```

#### `WithUnlimitedPageSize(allow bool)`

Allows a client to request `length=-1` for "all rows", bypassing
`MaxPageSize`. Off by default.

```go
opts.WithUnlimitedPageSize(true)
```

#### `Add(col string, fn func)`

Adds a custom computed column.

```go
opts.Add("full_name", func(row map[string]interface{}) interface{} {
    return row["first_name"].(string) + " " + row["last_name"].(string)
})
```

#### `Edit(col string, fn func)`

Transforms an existing column value.

```go
opts.Edit("email", func(value interface{}, row map[string]interface{}) interface{} {
    return strings.ToLower(value.(string))
})
```

#### `Remove(cols ...string)`

Removes columns from the response.

```go
opts.Remove("password", "internal_id")
```

---

## 🧪 Testing

Run the test suite:

```bash
go test ./datatables/... -v
```

Run with coverage (92.8% as of v2.0.0):

```bash
go test ./datatables/... -cover
```

`parser_test.go` and `processor_test.go` cover the request parsing,
search-grouping, and ordering logic with `gorm.Config{DryRun: true}`,
asserting on the generated SQL directly — no database required.

---

## 📂 Project Structure

```
datatables/
├── datatables.go      # Package documentation
├── processor.go       # Core OfReturn logic
├── options.go         # Options builder
├── parser.go          # Request parameter parsing
├── converter.go       # Struct to map conversion
├── transformer.go     # Column transformations
├── validation.go      # Security validation
├── errors.go          # Error types
├── response.go        # JSON response helpers
├── parser_test.go     # ParseParams table-driven tests
├── processor_test.go  # DryRun SQL assertions (search, ordering, OfReturn)
└── dto/
    ├── request.go     # Request DTOs
    └── response.go    # Response DTOs
```

---

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## 🙏 Acknowledgments

- Inspired by [Yajra DataTables](https://github.com/yajra/laravel-datatables) for Laravel
- Built with [Gin](https://github.com/gin-gonic/gin) and [GORM](https://gorm.io)

---

## 📞 Support

If you encounter any issues or have questions:

- 📝 [Open an issue](https://github.com/bonarizki-dat/Datatables-Gin/issues)
- 💬 [Discussions](https://github.com/bonarizki-dat/Datatables-Gin/discussions)

---

<p align="center">Made with ❤️ for the Go community</p>
