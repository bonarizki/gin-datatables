# 🧮 gin-datatables

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![CI](https://github.com/bonarizki/gin-datatables/actions/workflows/ci.yml/badge.svg)](https://github.com/bonarizki/gin-datatables/actions/workflows/ci.yml)

A **DataTables-style server-side processor** for **Gin + GORM** in Go
(`github.com/bonarizki/gin-datatables/v2`).
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
- 🧪 **Tested**: SQL-level DryRun tests plus an in-memory SQLite integration suite
- 📦 **Column validation**: Searchable/orderable/default-order columns are checked against an identifier whitelist before reaching SQL

---

## 📦 Installation

```bash
go get github.com/bonarizki/gin-datatables/v2
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

    "github.com/bonarizki/gin-datatables/v2/datatables"
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
        "name":       "name",
        "email":      "email",
        "created_at": "created_at",
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
            <th data-data="created_at">Created</th>
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
        { data: 'created_at' }
    ]
});
</script>
```

Keep three names identical for every sortable column: the JSON key your
struct produces (`created_at`), the frontend `columns[i].data`, and the
key in the `orderable` map. The map's value is the database column, which
may differ (for example `"created_at": "users.created_at"`).

---

## 🎯 Advanced Usage

### Custom Columns

Add computed columns dynamically:

```go
opts := datatables.NewOptions().
    // Assumes a struct with `json:"name"`, `json:"email"` and `json:"status"` fields.
    Add("label", func(row map[string]interface{}) interface{} {
        return fmt.Sprintf("%v <%v>", row["name"], row["email"])
    }).
    Add("status_badge", func(row map[string]interface{}) interface{} {
        status, _ := row["status"].(string)
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
        return strings.ToLower(fmt.Sprint(value))
    }).
    Edit("created_at", func(value interface{}, row map[string]interface{}) interface{} {
        // Format timestamps
        t, ok := value.(time.Time)
        if !ok {
            return value
        }
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

### Per-Column Search

When a DataTables column has its own search box (`columns.searchable: true`
with individual column filtering enabled on the frontend), the client sends
`columns[i][data]` and `columns[i][search][value]` for every column.
`OfReturn` reads it automatically. Declare which keys may be filtered with
`WithColumnSearch`; pass an empty map to turn per-column search off:

```go
opts := datatables.NewOptions().
    WithColumnSearch(map[string]string{"name": "users.name"})

// Request: columns[0][data]=name&columns[0][search][value]=john
// generates an additional AND-ed condition (Postgres shown):
// WHERE ... AND CAST(users.name AS TEXT) ILIKE ? ESCAPE '!'
```

If `WithColumnSearch` is not set, keys are resolved through the `orderable`
map instead. That fallback is deprecated, because it makes every sortable
column filterable, and will become opt-in in v3.

Per-column search values combine with global search and any `WHERE` the
caller already applied — all as independent `AND` conditions, so they
narrow the result set further rather than replacing each other.

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
        "created_at": "created_at",
    }

    opts := datatables.NewOptions().
        WithIndex("row_num", true).
        WithDefaultOrder("created_at", "desc").
        Add("price_formatted", func(row map[string]interface{}) interface{} {
            price, _ := row["price"].(float64)
            return fmt.Sprintf("$%.2f", price)
        }).
        Add("stock_status", func(row map[string]interface{}) interface{} {
            stock, _ := row["stock"].(int)
            if stock > 10 {
                return "In Stock"
            } else if stock > 0 {
                return "Low Stock"
            }
            return "Out of Stock"
        }).
        Edit("category", func(value interface{}, row map[string]interface{}) interface{} {
            return strings.ToUpper(fmt.Sprint(value))
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
escape a `WHERE` the caller already applied (e.g. tenant scoping). `%`, `_`
and `!` in the search value are escaped with `!` (`ESCAPE '!'`), which works
in MySQL's default `sql_mode` where a backslash escape would not:

```go
// db.Model(&User{}).Where("tenant_id = ?", tenantID)
// search value: "x"
// generates (SQLite shown; Postgres uses ILIKE, MySQL CAST(... AS CHAR) LIKE):
// WHERE tenant_id = ? AND (LOWER(CAST(name AS TEXT)) LIKE LOWER(?) ESCAPE '!'
//                       OR LOWER(CAST(code AS TEXT)) LIKE LOWER(?) ESCAPE '!')
// vars: [tenantID, "%x%", "%x%"]
```

Columns are cast to text first, so searching a numeric or timestamp column
does not fail on Postgres. Search values are cut to
`Options.MaxSearchLength` runes (256 by default).

### Bounded pagination

A requested `length` above `Options.MaxPageSize` (500 by default) is cut to
that maximum. A missing, zero, negative or unparseable `length` uses
`Options.DefaultPageSize` (10 by default). Zero or inconsistent values in
`Options` itself also fall back to those defaults, so a zero-value `Options`
never disables `LIMIT`. Only `WithUnlimitedPageSize(true)` lets a client
request `length=-1` for every row.

### Stable paging and GROUP BY

After the requested (or default) order, `OfReturn` appends the model's
primary key so rows with equal sort values do not move between pages. It
is skipped for `GROUP BY` and `DISTINCT` queries and when the key is already
ordered. Use `WithTieBreaker("users.id")` to choose another column, or
`WithTieBreaker("-")` to turn it off.

### Joins and aliases

`searchable`, `orderable` values and `WithColumnSearch` values are used in
`WHERE` and `ORDER BY`, so they must be real column expressions such as
`users.name`, not aliases defined in your `SELECT`.

### Not supported by design

A few parameters from the DataTables protocol are intentionally not read,
rather than being gaps:

- **`search[regex]` / `columns[i][search][regex]`** — treating the search
  value as a regular expression is not implemented. Regex-to-SQL
  translation is not portable across database engines (MySQL `REGEXP`,
  Postgres `~`, SQLite has no native equivalent), and an untrusted regex is
  a known denial-of-service vector (catastrophic backtracking).
- **`columns[i][searchable]` / `columns[i][orderable]`** — these are
  client-sent hints and are never read. Trusting them would let a client
  widen its own search/order surface; the `searchable`/`orderable`
  arguments passed to `OfReturn` remain the only source of truth for what
  is allowed.

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
- `orderable` - Map of frontend column names to database columns; also used
  to resolve per-column search keys when `WithColumnSearch` is not set
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
`DefaultOrders`, primary-key tie-breaker, `MaxSearchLength` 256, and no
logger.

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
order key is not in the orderable map, instead of skipping it and
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

#### `WithTieBreaker(col string)`

Column appended last to `ORDER BY`: `""` for the primary key (default),
`"-"` to disable.

#### `WithColumnSearch(cols map[string]string)`

Frontend keys allowed in `columns[i][search][value]`, mapped to database
columns. An empty map disables per-column search.

#### `WithLogger(l *slog.Logger)`

Receives debug records for ignored order and search keys. Nothing is
logged by default.

#### `WithMaxSearchLength(n int)`

Maximum runes used from each search value (default 256).

#### `Add(col string, fn func)`

Adds a custom computed column.

```go
opts.Add("label", func(row map[string]interface{}) interface{} {
    return fmt.Sprintf("%v <%v>", row["name"], row["email"])
})
```

#### `Edit(col string, fn func)`

Transforms an existing column value.

```go
opts.Edit("email", func(value interface{}, row map[string]interface{}) interface{} {
    return strings.ToLower(fmt.Sprint(value))
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

Run with coverage:

```bash
go test ./datatables/... -cover
```

`parser_test.go` and `processor_test.go` cover the request parsing,
search-grouping, and ordering logic with `gorm.Config{DryRun: true}`,
asserting on the generated SQL directly. `integration_sqlite_test.go` runs
`OfReturn` against an in-memory SQLite database (pure Go, no cgo), and
`example_test.go` keeps the documented usage compiling.

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
├── *_test.go          # DryRun SQL, SQLite integration and example tests
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

- 📝 [Open an issue](https://github.com/bonarizki/gin-datatables/issues)
- 💬 [Discussions](https://github.com/bonarizki/gin-datatables/discussions)

---

<p align="center">Made with ❤️ for the Go community</p>
